package database

import (
	"sync"
)

// SET和GET 用最原始的map就能实现
// 但为了实现redis的LPUSH/LPOP操作 我们需要定义一个双向链表

// 原先整个engine只有一个map 一个锁 如果并发数量过高 比如十万个并发请求都要进行set操作，
// 则这十万个goroutine 会在cpu调动时被排成一条单行道 逐一执行 从而闲置多核CPU
// 现在对engine进行封装，对外暴露仍然是一个engine，宏观上无论是http或者redis 都是共享一个map
// 但是在engine内部 把空间分为了16或者64个分片，每个分片都拥有独立的锁sync.RWMutex和map
// 无论是写入还是读取 都会根据key判断属于哪个shard分组 然后自动路由过去
type shard struct {
	// 这里使用了读写锁 因为读的场景远高于写
	// 读写锁可以满足多个可读同时进行 如果有写进程则只允许一个写进程进行
	// 如果是普通的锁 无论读写 都同时只允许一个进行
	mu sync.RWMutex
	// map采用string作为key  字节数组作为value保证二进制安全
	data map[string]any
}

type Engine struct {
	shards     []*shard
	shardCount uint32
}

func NewEngine(shardCount uint32) *Engine {
	if shardCount == 0 || (shardCount&(shardCount-1)) != 0 {
		panic("shardCount must be a power of 2")
	}
	e := &Engine{shards: make([]*shard, shardCount),
		shardCount: shardCount}
	for i := range e.shards {
		e.shards[i] = &shard{
			data: make(map[string]any),
		}
	}
	return e
}

// 将任意长度的字符串 通过位异或和乘法运算 生成一个32位无符号整数
// 这里手写函数是因为内置的hash32底层要求传入的参数为字节数组 如果在这里进行转换
// string到[]byte字节的强转会在堆内存上重新开辟空间来拷贝数据并且很会被当成垃圾回收
// 高并发下会直接让垃圾回收器GC过载
func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash *= 16777619
		// 利用循环遍历string的字节 做到在纯CPU寄存器层面完成运算
		hash ^= uint32(key[i])
	}
	return hash
}

// getShard 极速路由：利用位掩码替代高昂的取模指令
func (e *Engine) getShard(key string) *shard {
	hash := fnv32(key)
	// 把哈希值的值 落在0-shardCount - 1之间
	// 核心位运算：当 shardCount 是 2^n 时， hash & (shardCount - 1) 完全等价于 hash % shardCount
	index := hash & (e.shardCount - 1)
	return e.shards[index]
}

// Node 双向链表
type Node struct {
	val  []byte
	prev *Node // 指向上一个
	next *Node // 指向下一个
}

type LinkedList struct {
	head *Node // 总是指向第一个节点
	tail *Node // 总是指向最后一个节点
	len  int   // 链表长度
}

// NewLinkedList 初始化工厂函数
func NewLinkedList() *LinkedList {
	return &LinkedList{}
}

func (l *LinkedList) Len() int {
	return l.len
}

func (l *LinkedList) LPush(value []byte) {
	// 1.先根据value值创建一个node节点
	newNode := &Node{
		val: value,
	}
	// 2.再判断当前的列表是否是一个空链表
	if l.len == 0 {
		l.head = newNode
		l.tail = newNode
	} else {
		l.head.prev = newNode
		newNode.next = l.head
		l.head = newNode
	}
	l.len++
}

func (l *LinkedList) LPop() ([]byte, bool) {

	var data []byte

	// 当前的列表为空 则直接返回错误
	if l.len == 0 {
		return nil, false
	} else if l.len == 1 {
		data = l.head.val
		l.head = nil
		l.tail = nil
	} else {
		data = l.head.val
		l.head.next.prev = nil
		l.head = l.head.next
	}
	l.len--
	return data, true
}

// Exec 接受handler传过来的参数 识别并执行命令
func (e *Engine) Exec(args [][]byte) (any, error) {
	if len(args) == 0 {
		return nil, ErrUnknownCmd
	}
	// 接下来获取具体的命令
	cmdName := string(args[0])
	switch cmdName {
	case "SET":
		// 调用SET函数 传入args
		return e.set(args)
	case "GET":
		// 调用GET函数 传入args
		return e.get(args)
	case "LPUSH":
		return e.LPush(args)
	case "LPOP":
		return e.LPop(args)
	default:
		return nil, ErrUnknownCmd
	}
}

// SET执行函数
func (e *Engine) set(args [][]byte) (any, error) {
	//格式为 SET KEY VALUE

	// 如果传来的参数不对 则直接返回err
	if len(args) != 3 {
		return nil, ErrWrongArgsNum
	}
	// 提取出key和value
	key, value := string(args[1]), args[2]
	//写之前 拿写锁
	s := e.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	return true, nil
}

// GET执行函数
func (e *Engine) get(args [][]byte) (any, error) {
	//格式为 GET KEY
	if len(args) != 2 {
		return nil, ErrWrongArgsNum
	}
	key := string(args[1])
	s := e.getShard(key)
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, exists := s.data[key]
	// 如果对应的key没有value的话 返回RESP标准解析格式
	if !exists {
		return nil, nil //找不到不代表出错
	}
	//拿到value类型后 由于map的value类型已经更改 所以这里要进行断言处理
	result, ok := value.([]byte)
	if !ok {
		return nil, ErrTypeMismatch
	}
	return result, nil
}

// LPush 执行函数
func (e *Engine) LPush(args [][]byte) (any, error) {
	// 格式LPush Key value
	// 如果参数数量不对直接报错
	if len(args) != 3 {
		return nil, ErrWrongArgsNum
	}
	// 通过key拿value之前先上写锁
	key, value := string(args[1]), args[2]
	s := e.getShard(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	// 接着检查全局map中是否存在该key对应的值
	raw, exists := s.data[key]
	// 创建一个空的LinkedList结构体 这样无论是否存在都能统一变量 从而进行统一操作
	var list *LinkedList

	if !exists {
		// 如果当前的不存在 则创建一个LinkedList
		list = NewLinkedList()
		s.data[key] = list
	} else { // 如果存在则直接取出来
		var ok bool
		list, ok = raw.(*LinkedList)
		if !ok {
			//类型对不上 直接返回报错
			return nil, ErrTypeMismatch
		}
	}
	// 如果已经有了对应的链表 则取出来在后面追加即可
	list.LPush(value)
	// 回复长度
	return list.len, nil
}

func (e *Engine) LPop(args [][]byte) (any, error) {
	// 格式 LPop Key
	if len(args) != 2 {

		return nil, ErrWrongArgsNum
	}
	key := string(args[1])
	s := e.getShard(key)
	// 依旧是写操作
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, exists := s.data[key]
	if !exists {
		return nil, nil // Key不存在，统一返回 (nil, nil)
	}
	list, ok := raw.(*LinkedList)
	if !ok {
		return nil, ErrTypeMismatch
	}
	// 从列表中取出数据
	data, ok := list.LPop()
	if !ok {
		return nil, nil //链表空了也输入空值状态
	}

	// 如果当前的list已经为空了则没有存在必要了直接删除
	if list.Len() == 0 {
		delete(s.data, key)
	}
	return data, nil
}
