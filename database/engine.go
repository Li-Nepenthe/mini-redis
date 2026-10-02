package database

import (
	"bytes"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type shard struct {
	mu       sync.RWMutex
	data     map[string]any
	expires  map[string]expiration
	expiring []string
}

type Engine struct {
	log        CommandLog
	logMu      sync.Mutex
	shards     []*shard
	shardCount uint32
	now        func() time.Time
	reclaim    func()
}

// 状态与二进制数据使用不同类型，避免 OK/PONG 被误编码成 bulk。
type StatusReply string

func (s StatusReply) RESPStatus() string { return string(s) }

// 路由使用 hash & (N-1)，只有 N 为 2 的幂才能覆盖全部分片；拒绝非法配置，
// 避免看似有 N 个锁却只有一部分真正分担请求。16 是默认值，不是最优值保证。
func NewEngine(shardCount uint32) *Engine {
	if shardCount == 0 || shardCount&(shardCount-1) != 0 {
		panic("shardCount must be a power of 2")
	}
	e := &Engine{shards: make([]*shard, shardCount), shardCount: shardCount, now: time.Now, reclaim: debug.FreeOSMemory}
	for i := range e.shards {
		e.shards[i] = &shard{data: make(map[string]any), expires: make(map[string]expiration)}
	}
	return e
}

// 使用 FNV-1 的先乘后异或顺序；散列只负责稳定路由，不提供安全性或热点均摊保证。
func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash *= 16777619
		hash ^= uint32(key[i])
	}
	return hash
}

func (e *Engine) getShard(key string) *shard {
	return e.shards[fnv32(key)&(e.shardCount-1)]
}

// 多 key 按固定分片顺序加锁，避免另一连接用相反参数顺序时形成死锁。
// 同一分片必须去重：RWMutex 不是可重入锁。返回的闭包由调用者 defer，锁覆盖整个命令。
func (e *Engine) lockKeys(keys [][]byte, write bool) func() {
	seen := make(map[int]bool, len(keys))
	indices := make([]int, 0, len(keys))
	for _, key := range keys {
		index := int(fnv32(string(key)) & (e.shardCount - 1))
		if !seen[index] {
			seen[index] = true
			indices = append(indices, index)
		}
	}
	sort.Ints(indices)
	for _, i := range indices {
		if write {
			e.shards[i].mu.Lock()
		} else {
			e.shards[i].mu.RLock()
		}
	}
	return func() {
		for i := len(indices) - 1; i >= 0; i-- {
			if write {
				e.shards[indices[i]].mu.Unlock()
			} else {
				e.shards[indices[i]].mu.RUnlock()
			}
		}
	}
}

type Node struct {
	val        []byte
	prev, next *Node
}

// List 自身不加锁，由所属 shard 保护；否则同时维护两套锁顺序会让命令原子性更难判断。
type LinkedList struct {
	head, tail *Node
	len        int
}

func NewLinkedList() *LinkedList { return &LinkedList{} }
func (l *LinkedList) Len() int   { return l.len }

func (l *LinkedList) LPush(value []byte) {
	// 拷贝调用者持有的字节，防止网络缓冲复用或外部修改改变已存储值。
	node := &Node{val: bytes.Clone(value), next: l.head}
	if l.head == nil {
		l.tail = node
	} else {
		l.head.prev = node
	}
	l.head = node
	l.len++
}

func (l *LinkedList) LPop() ([]byte, bool) {
	if l.head == nil {
		return nil, false
	}
	node := l.head
	l.head = node.next
	if l.head == nil {
		l.tail = nil
	} else {
		l.head.prev = nil
	}
	node.next = nil
	l.len--
	return node.val, true
}

func (e *Engine) Exec(args [][]byte) (any, error) {
	if len(args) == 0 {
		return nil, ErrUnknownCmd
	}
	cmd := strings.ToUpper(string(args[0]))
	switch cmd {
	case "SET", "LPUSH", "LPOP", "DEL", "EXPIRE":
		return e.executeWrite(args, cmd)
	case "PING":
		if len(args) == 1 {
			return StatusReply("PONG"), nil
		}
		if len(args) == 2 {
			return bytes.Clone(args[1]), nil
		}
		return nil, wrongArgs("ping")

	case "GET":
		return e.get(args)

	case "EXISTS":
		return e.exists(args)
	case "LRANGE":
		return e.lrange(args)

	case "TTL":
		return e.ttl(args)
	default:
		return nil, ErrUnknownCmd
	}
}

func (e *Engine) get(args [][]byte) (any, error) {
	if len(args) != 2 {
		return nil, wrongArgs("get")
	}
	key := string(args[1])
	s, _ := e.lockRead(key)
	defer s.mu.RUnlock()
	value, exists := s.data[key]
	if !exists {
		return nil, nil
	}
	result, ok := value.([]byte)
	if !ok {
		return nil, ErrTypeMismatch
	}
	// 回复离开锁后仍会被编码/使用，所以不能把存储内部字节直接交给调用者。
	return bytes.Clone(result), nil
}

func (e *Engine) LPush(args [][]byte) (any, error) {
	return e.executeWrite(args, "LPUSH")
}

func (e *Engine) LPop(args [][]byte) (any, error) {
	return e.executeWrite(args, "LPOP")
}

func (e *Engine) exists(args [][]byte) (any, error) {
	if len(args) < 2 {
		return nil, wrongArgs("exists")
	}
	unlock := e.lockKeys(args[1:], true)
	defer unlock()
	count := 0
	now := e.now()
	for _, arg := range args[1:] {
		key := string(arg)
		s := e.getShard(key)
		s.purgeExpired(key, now)
		if _, ok := s.data[key]; ok {
			count++
		}
	}
	return count, nil
}

func (e *Engine) lrange(args [][]byte) (any, error) {
	if len(args) != 4 {
		return nil, wrongArgs("lrange")
	}
	start, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return nil, ErrInvalidInteger
	}
	stop, err := strconv.ParseInt(string(args[3]), 10, 64)
	if err != nil {
		return nil, ErrInvalidInteger
	}
	key := string(args[1])
	s, _ := e.lockRead(key)
	defer s.mu.RUnlock()
	raw, exists := s.data[key]
	result := make([][]byte, 0)
	if !exists {
		return result, nil
	}
	list, ok := raw.(*LinkedList)
	if !ok {
		return nil, ErrTypeMismatch
	}
	length := int64(list.Len())
	if start < 0 {
		start += length
	}
	if stop < 0 {
		stop += length
	}
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start >= length || stop < 0 || start > stop {
		return result, nil
	}
	node := list.head
	for i := int64(0); i < start; i++ {
		node = node.next
	}
	result = make([][]byte, 0, int(stop-start+1))
	for i := start; i <= stop; i++ {
		result = append(result, bytes.Clone(node.val))
		node = node.next
	}
	return result, nil
}
