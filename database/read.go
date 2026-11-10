// 本文件：get、exists、lrange 和 ttl 实现读命令，通过过期检查后再读取分片中的值。

package database

import (
	"bytes"
	"strconv"
	"time"
)

// 读取结果会在解锁后交给调用者，因此返回副本，防止外部修改引擎持有的字节。
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
	return bytes.Clone(result), nil
}

// exists 按参数逐个计数，重复的存活 key 也重复计算；这和 DEL 只计算实际删除数不同。
// 判断存在性可能清理过期值，所以整条命令使用有序分片写锁。
func (e *Engine) exists(args [][]byte) (any, error) {
	if len(args) < 2 {
		return nil, wrongArgs("exists")
	}
	unlock := e.lockKeys(args[1:], true)
	defer unlock()
	var count int64
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

// 先把负下标和越界范围折算为实际区间，再在读锁内遍历并复制元素。
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
	list, ok := raw.(*linkedList)
	if !ok {
		return nil, ErrTypeMismatch
	}
	length := int64(list.size())
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

// TTL 按半秒四舍五入显示剩余秒数；显示 0 时 key 仍可能存活，删除只看精确期限。
func (e *Engine) ttl(args [][]byte) (any, error) {
	if len(args) != 2 {
		return nil, wrongArgs("ttl")
	}
	key := string(args[1])
	s, now := e.lockRead(key)
	defer s.mu.RUnlock()
	if _, ok := s.data[key]; !ok {
		return int64(-2), nil
	}
	entry, ok := s.expires[key]
	if !ok {
		return int64(-1), nil
	}
	remaining := entry.deadline.Sub(now)
	seconds := int64(remaining / time.Second)
	if remaining%time.Second >= 500*time.Millisecond {
		seconds++
	}
	return seconds, nil
}
