// 本文件：executeWrite 组织先日志后内存的写入，plan 函数计算各命令的变更，Replay 用同一套计划恢复历史记录。

package database

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// 惰性过期不写日志。新列表必须覆盖历史旧值和旧 TTL，否则重放 LPUSH 会混入旧元素。
	// _LNEW 与 LPUSH 同为五字节、参数个数相同，最大的合法请求落盘后仍在协议上限内。
	recordNewList = "_LNEW"
	// 记录绝对毫秒期限，避免重启时重新计算相对秒数，让 key 多活一段时间。
	recordExpireAtMS = "__EXPIREATMS"
)

// CommandLog 的成功确认包含刷盘成功；引擎收到 nil 后才允许修改内存。
type CommandLog interface {
	Append(args [][]byte) error
}

// AttachLog 只在恢复完成、并发服务开始前绑定日志；运行中更换会打破写入确认的约定。
func (e *Engine) AttachLog(log CommandLog) { e.log = log }

// writePlan 把回复、日志和内存变更放在一起，避免调用者记忆几个返回值之间的约定。
// 计算计划和执行 apply 必须持有同一批分片写锁，计划中的列表引用才不会被其他命令改动。
type writePlan struct {
	reply  any
	record [][]byte // nil 表示没有效果，不需要记日志。
	apply  func()   // 日志确认后才执行；nil 表示无需修改内存。
}

// executeWrite 先在锁内算好变更，再确认日志，最后改内存；日志失败时读者仍只能看到旧值。
// logMu 排定写入次序，分片锁一直持有到 apply 完成，保证内存顺序和 AOF 顺序一致。
func (e *Engine) executeWrite(args [][]byte, cmd string) (any, error) {
	if err := checkWriteArgs(args, cmd); err != nil {
		return nil, err
	}
	if e.log != nil {
		e.logMu.Lock()
		defer e.logMu.Unlock()
	}
	keys := writeKeys(args, cmd)
	unlock := e.lockKeys(keys, true)
	defer unlock()
	now := e.now()
	for _, key := range keys {
		e.getShard(string(key)).purgeExpired(string(key), now)
	}
	plan, err := e.planWrite(args, cmd, now)
	if err != nil || plan.apply == nil {
		return plan.reply, err
	}
	if e.log != nil {
		if err := e.log.Append(plan.record); err != nil {
			return nil, &commandError{code: "ERR", message: "persistence write failed", cause: err}
		}
	}
	plan.apply()
	return plan.reply, nil
}

// checkWriteArgs 先挡住不完整的命令，后面的取 key、加锁和计划计算才有确定的参数位置。
func checkWriteArgs(args [][]byte, cmd string) error {
	valid := len(args) == 3
	switch cmd {
	case "LPUSH", recordNewList:
		valid = len(args) >= 3
	case "LPOP":
		valid = len(args) == 2
	case "DEL":
		valid = len(args) >= 2
	}
	if !valid {
		if cmd == recordExpireAtMS {
			return errors.New("invalid persisted expiry arity")
		}
		return wrongArgs(strings.ToLower(cmd))
	}
	return nil
}

// writeKeys 让普通写入与重放使用同一组锁范围；只有 DEL 能同时涉及多个 key。
func writeKeys(args [][]byte, cmd string) [][]byte {
	if cmd == "DEL" {
		return args[1:]
	}
	return args[1:2]
}

// planWrite 统一日志中的大写命令名，另建参数切片，避免 planLPush 改写调用者的命令名。
func (e *Engine) planWrite(args [][]byte, cmd string, now time.Time) (writePlan, error) {
	record := append([][]byte{[]byte(cmd)}, args[1:]...)
	switch cmd {
	case "SET":
		return e.planSet(record)
	case "LPUSH", recordNewList:
		return e.planLPush(record)
	case "LPOP":
		return e.planLPop(record)
	case "DEL":
		return e.planDel(record)
	case "EXPIRE":
		return e.planExpire(record, now)
	case recordExpireAtMS:
		return e.planExpireAt(record)
	default:
		return writePlan{}, ErrUnknownCmd
	}
}

// planSet 留下独立的值副本；覆盖旧值时连旧 TTL 一起清掉，不能让新值继承旧的到期时间。
func (e *Engine) planSet(args [][]byte) (writePlan, error) {
	key := string(args[1])
	s := e.getShard(key)
	value := append([]byte(nil), args[2]...)
	return writePlan{reply: StatusReply("OK"), record: args, apply: func() {
		s.clearExpiration(key)
		s.data[key] = value
	}}, nil
}

// planLPush 区分追加和新建。重放遇到新建标记时，即使旧值仍在，也必须从空列表开始。
func (e *Engine) planLPush(args [][]byte) (writePlan, error) {
	key := string(args[1])
	s := e.getShard(key)
	raw, exists := s.data[key]
	fresh := !exists || string(args[0]) == recordNewList
	list := NewLinkedList()
	if !fresh {
		var ok bool
		list, ok = raw.(*LinkedList)
		if !ok {
			return writePlan{}, ErrTypeMismatch
		}
	}
	if fresh {
		args[0] = []byte(recordNewList)
	}
	return writePlan{reply: int64(list.Len() + len(args) - 2), record: args, apply: func() {
		if fresh {
			s.clearExpiration(key)
		}
		for _, value := range args[2:] {
			list.LPush(value)
		}
		s.data[key] = list
	}}, nil
}

// planLPop 先看头元素但不摘下它；弹出最后一个元素时，key 和 TTL 必须一起消失。
func (e *Engine) planLPop(args [][]byte) (writePlan, error) {
	key := string(args[1])
	s := e.getShard(key)
	raw, exists := s.data[key]
	if !exists {
		return writePlan{}, nil
	}
	list, ok := raw.(*LinkedList)
	if !ok {
		return writePlan{}, ErrTypeMismatch
	}
	return writePlan{reply: list.head.val, record: args, apply: func() {
		_, _ = list.LPop()
		if list.Len() == 0 {
			delete(s.data, key)
			s.clearExpiration(key)
		}
	}}, nil
}

// planDel 对实际删除的 key 去重，日志仍保留原参数，维持已有 AOF 记录格式。
func (e *Engine) planDel(args [][]byte) (writePlan, error) {
	unique := make(map[string]bool, len(args)-1)
	for _, arg := range args[1:] {
		key := string(arg)
		if _, exists := e.getShard(key).data[key]; exists {
			unique[key] = true
		}
	}
	if len(unique) == 0 {
		return writePlan{reply: int64(0)}, nil
	}
	return writePlan{reply: int64(len(unique)), record: args, apply: func() {
		for key := range unique {
			s := e.getShard(key)
			delete(s.data, key)
			s.clearExpiration(key)
		}
	}}, nil
}

// planExpire 把非正数期限当作删除；正数期限固定到毫秒，让内存和重放使用同一个到期时刻。
func (e *Engine) planExpire(args [][]byte, now time.Time) (writePlan, error) {
	seconds, err := parseExpirySeconds(args[2])
	if err != nil {
		return writePlan{}, err
	}
	if seconds <= 0 {
		return e.planDel([][]byte{[]byte("DEL"), args[1]})
	}
	key := string(args[1])
	s := e.getShard(key)
	if _, exists := s.data[key]; !exists {
		return writePlan{reply: int64(0)}, nil
	}
	deadline := time.UnixMilli(now.Add(time.Duration(seconds) * time.Second).UnixMilli())
	record := [][]byte{[]byte(recordExpireAtMS), args[1], []byte(strconv.FormatInt(deadline.UnixMilli(), 10))}
	return writePlan{reply: int64(1), record: record, apply: func() { s.setExpiration(key, deadline) }}, nil
}

// planExpireAt 还原日志里的绝对期限；即使期限早于重启时刻，也要等历史记录全部恢复后再判断过期。
func (e *Engine) planExpireAt(args [][]byte) (writePlan, error) {
	millis, err := strconv.ParseInt(string(args[2]), 10, 64)
	if err != nil {
		return writePlan{}, fmt.Errorf("invalid persisted expiry: %w", err)
	}
	key := string(args[1])
	s := e.getShard(key)
	if _, exists := s.data[key]; !exists {
		return writePlan{}, nil
	}
	return writePlan{record: args, apply: func() { s.setExpiration(key, time.UnixMilli(millis)) }}, nil
}

// Replay 恢复历史时不做惰性过期：后续记录可能续期，提前删除会丢掉本来仍有效的数据。
// 启动者在恢复完成后才开放服务；这里直接应用计划，不再次写日志。
func (e *Engine) Replay(args [][]byte) error {
	if len(args) == 0 {
		return ErrUnknownCmd
	}
	cmd := strings.ToUpper(string(args[0]))
	switch cmd {
	case "SET", "LPUSH", recordNewList, "LPOP", "DEL", recordExpireAtMS:
	default:
		return fmt.Errorf("invalid persisted command: %s", cmd)
	}
	if err := checkWriteArgs(args, cmd); err != nil {
		return err
	}
	unlock := e.lockKeys(writeKeys(args, cmd), true)
	defer unlock()
	plan, err := e.planWrite(args, cmd, e.now())
	if err == nil && plan.apply != nil {
		plan.apply()
	}
	return err
}
