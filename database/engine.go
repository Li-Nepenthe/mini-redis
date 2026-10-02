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

// RESPStatus 返回状态回复的字符串内容，使编码器通过接口识别 PONG 等简单状态。
// 不做编码或换行校验，也不修改状态值；EncodeReply 负责拒绝带 CRLF 的状态，避免与二进制 bulk 混淆。
func (s StatusReply) RESPStatus() string { return string(s) }

// NewEngine 按 shardCount 创建分片 map、默认时钟和内存回收回调，返回尚未绑定 AOF/启动清理 worker 的引擎。
// shardCount 必须是非零的 2 的幂，否则 panic；路由 hash & (N-1) 依赖这一不变量。16 是服务默认值，不保证所有负载最优。
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

// fnv32 对 key 的字节逐个计算 FNV-1 32 位散列，返回稳定的无符号路由值，无锁及存储副作用。
// 顺序为先乘常数再异或，不能随意换成 FNV-1a；散列不提供密码安全或热点均摊保证。
func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash *= 16777619
		hash ^= uint32(key[i])
	}
	return hash
}

// getShard 使用 key 的 FNV-1 散列与分片掩码选择并返回所属 shard。
// 不获取锁，也不创建或读取 key；调用者访问该分片的 map、List、TTL 时仍须遵守分片锁约束。
func (e *Engine) getShard(key string) *shard {
	return e.shards[fnv32(key)&(e.shardCount-1)]
}

// lockKeys 将 keys 映射到分片，去重并按分片编号升序获取锁；write 为 true 获取写锁，否则获取读锁。
// 返回逆序释放全部锁的闭包，调用者必须执行一次，通常 defer。参数顺序不影响锁顺序，避免 DEL a b 与 DEL b a 互相等待。
// 同分片必须去重，因为 RWMutex 不可重入；函数本身不检查 key 是否存在，不修改业务数据。
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

// NewLinkedList 创建并返回首尾为 nil、长度为零的双向链表，不启动后台工作。
// 链表自身没有锁；若纳入 Engine，所有操作由所属 shard 保护，独立使用时同步责任在调用者。
func NewLinkedList() *LinkedList { return &LinkedList{} }

// Len 返回链表当前节点数量，不遍历或修改节点，也不加锁。
// 该数量由 LPush/LPop 同步维护；并发读取须由调用者使用所属分片锁保护。
func (l *LinkedList) Len() int { return l.len }

// LPush 拷贝 value 字节并在链表头插入一个节点，更新 head/tail、前后链接及长度，没有返回值。
// 拷贝隔离调用者后续修改；不加锁，调用者必须保证链表修改互斥。多个值的命令级顺序由 Engine 逐值调用决定。
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

// LPop 移除链表头并返回其值及是否成功；空链表返回 nil、false。
// 更新首尾和长度并断开已移除节点的 next；返回字节已脱离链表，不额外拷贝。不加锁，调用者负责互斥。
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

// Exec 将 args[0] 的命令名转为大写并分发；其余参数保持二进制内容，调用期间调用者不能并发修改 args。
// 返回状态/字节/整数/字节数组或 nil，以及参数、类型、未知命令或持久化错误；响应编码由 resp 包负责。
// 写命令走统一 prepare→日志确认→apply，读命令按需惰性清理过期 key；不向客户端开放私有 AOF 命令。
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

// get 执行 GET，args 必须包含命令名和一个 key；不存在或过期返回 nil，String 返回拷贝的 []byte。
// 参数错误或 List 类型返回业务错误；lockRead 可能删除过期值，成功取得读锁后本函数负责释放。拷贝使回复离锁后不会别名修改存储。
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

// LPush 将包含命令名、key 和至少一个值的 args 交给统一 LPUSH 写路径，返回更新后的长度或错误。
// 命令名由此入口固定为 LPUSH；顺序插入每个值，后给的值位于更靠前位置。类型检查、TTL 与 AOF 副作用由 executeWrite 统一处理。
func (e *Engine) LPush(args [][]byte) (any, error) {
	return e.executeWrite(args, "LPUSH")
}

// LPop 将包含命令名和一个 key 的 args 交给统一 LPOP 写路径，返回移除的头值、缺失时 nil 或业务/日志错误。
// 非空 List 移除末节点时同时删除 key 与 TTL；参数和类型校验、先日志后修改的确认边界由 executeWrite 处理。
func (e *Engine) LPop(args [][]byte) (any, error) {
	return e.executeWrite(args, "LPOP")
}

// exists 执行 EXISTS，对 args[1:] 的每个 key 判断是否存活并返回 int 计数；重复的存活 key 重复计数。
// 无 key 返回参数错误。按有序分片写锁保护整条命令，因为存在性检查可能惰性删除过期值及 TTL，不记录这种清理到 AOF。
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

// lrange 执行 LRANGE，args 为命令名、key、start、stop，按包含 stop 的区间返回拷贝后的 [][]byte。
// 负索引从尾部换算，越界裁剪；缺失或空区间返回空数组，整数/数量/类型非法返回业务错误。
// 通过 lockRead 隔离过期处理并持读锁遍历，负责释放；拷贝每个元素避免调用者修改存储。
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
