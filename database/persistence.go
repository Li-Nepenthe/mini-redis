package database

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Append 成功必须包含刷盘成功，才能保证先日志后内存的确认边界。
type CommandLog interface {
	Append(args [][]byte) error
}

// 仅启动时重放后、启动 worker 前绑定，避免重放再追加和并发更换日志。
func (e *Engine) AttachLog(log CommandLog) { e.log = log }

// AOF 模式按 logMu → 有序 shard 锁 → Store.mu 获取锁，保证内存提交与日志顺序相同。
// 读者只用 shard 锁，因而同分片读也会等待 fsync；这是确认语义的明确代价。
func (e *Engine) executeWrite(args [][]byte, cmd string) (any, error) {
	if e.log != nil {
		e.logMu.Lock()
		defer e.logMu.Unlock()
	}
	result, record, apply, unlock, err := e.prepareWrite(args, cmd, false)
	if unlock != nil {
		defer unlock()
	}
	if err != nil || apply == nil {
		return result, err
	}
	if e.log != nil {
		// 追加、刷盘与应用内存期间保持分片锁，让读者看不到刷盘失败的值；
		// 统一写入顺序也让重放结果与内存一致，代价是磁盘等待期间占锁。
		if err := e.log.Append(record); err != nil {
			return nil, &commandError{code: "ERR", message: "persistence write failed", cause: err}
		}
	}
	apply()
	return result, nil
}

// 把“检查并准备”与“修改业务值”分开：Append/Sync 失败时不调用 apply。
// apply 捕获的 List/状态必须在同一批 shard 锁内使用；即使返回错误，调用者也要执行 unlock。
// replay 跳过当前时间的惰性过期，否则历史中后续的续期/追加会基于被提前删除的状态执行。
func (e *Engine) prepareWrite(args [][]byte, cmd string, replay bool) (result any, record [][]byte, apply func(), unlock func(), err error) {
	valid := len(args) == 3
	switch cmd {
	case "LPUSH", "_LNEW":
		valid = len(args) >= 3
	case "LPOP":
		valid = len(args) == 2
	case "DEL":
		valid = len(args) >= 2
	}
	if !valid {
		return nil, nil, nil, nil, wrongArgs(strings.ToLower(cmd))
	}
	var seconds int64
	if cmd == "EXPIRE" {
		seconds, err = parseExpirySeconds(args[2])
		if err != nil {
			return nil, nil, nil, nil, err
		}
	}
	keys := args[1:2]
	if cmd == "DEL" {
		keys = args[1:]
	}
	unlock = e.lockKeys(keys, true)
	now := e.now()
	if !replay {
		for _, key := range keys {
			e.getShard(string(key)).purgeExpired(string(key), now)
		}
	}
	key := string(args[1])
	s := e.getShard(key)
	record = append([][]byte{[]byte(cmd)}, args[1:]...)
	switch cmd {
	case "SET":
		value := append([]byte(nil), args[2]...)
		return true, record, func() {
			s.clearExpiration(key)
			s.data[key] = value
		}, unlock, nil
	case "LPUSH", "_LNEW":
		list := NewLinkedList()
		raw, exists := s.data[key]
		fresh := !exists || cmd == "_LNEW"
		if !fresh {
			var ok bool
			list, ok = raw.(*LinkedList)
			if !ok {
				return nil, nil, nil, unlock, ErrTypeMismatch
			}
		}
		if fresh {
			// 过期清理不写 AOF；必须记下「新建 List」边界，否则重放时旧值/TTL
			// 会混入过期后的新 List。私有命令与 LPUSH 同为 5 字节，参数数目也不变，
			// 避免合法的最大请求在落盘时超出已有 RESP 帧预算。
			record[0] = []byte("_LNEW")
		}
		length := list.Len() + len(args) - 2
		return length, record, func() {
			if fresh {
				s.clearExpiration(key)
			}
			for _, value := range args[2:] {
				list.LPush(value)
			}
			s.data[key] = list
		}, unlock, nil
	case "LPOP":
		raw, exists := s.data[key]
		if !exists {
			return nil, nil, nil, unlock, nil
		}
		list, ok := raw.(*LinkedList)
		if !ok {
			return nil, nil, nil, unlock, ErrTypeMismatch
		}
		value := list.head.val
		return value, record, func() {
			_, _ = list.LPop()
			if list.Len() == 0 {
				delete(s.data, key)
				s.clearExpiration(key)
			}
		}, unlock, nil
	case "DEL":
		unique := make(map[string]bool, len(keys))
		for _, arg := range keys {
			key := string(arg)
			if _, exists := e.getShard(key).data[key]; exists {
				unique[key] = true
			}
		}
		if len(unique) == 0 {
			return 0, nil, nil, unlock, nil
		}
		return len(unique), record, func() {
			for key := range unique {
				s := e.getShard(key)
				delete(s.data, key)
				s.clearExpiration(key)
			}
		}, unlock, nil
	case "EXPIRE":
		if _, exists := s.data[key]; !exists {
			return 0, nil, nil, unlock, nil
		}
		if seconds <= 0 {
			record = [][]byte{[]byte("DEL"), args[1]}
			return 1, record, func() { delete(s.data, key); s.clearExpiration(key) }, unlock, nil
		}
		deadline := time.UnixMilli(now.Add(time.Duration(seconds) * time.Second).UnixMilli())
		record = [][]byte{[]byte("__EXPIREATMS"), args[1], []byte(strconv.FormatInt(deadline.UnixMilli(), 10))}
		return 1, record, func() { s.setExpiration(key, deadline) }, unlock, nil
	}
	return nil, nil, nil, unlock, ErrUnknownCmd
}

// 重放只接受持久化写命令；绝对期限只供 AOF 使用，不向网络客户端开放。
func (e *Engine) Replay(args [][]byte) error {
	if len(args) == 0 {
		return ErrUnknownCmd
	}
	cmd := strings.ToUpper(string(args[0]))
	if cmd == "__EXPIREATMS" {
		if len(args) != 3 {
			return errors.New("invalid persisted expiry arity")
		}
		millis, err := strconv.ParseInt(string(args[2]), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid persisted expiry: %w", err)
		}
		key := string(args[1])
		s := e.getShard(key)
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, exists := s.data[key]; exists {
			s.setExpiration(key, time.UnixMilli(millis))
		}
		return nil
	}
	switch cmd {
	case "SET", "LPUSH", "_LNEW", "LPOP", "DEL":
		// 重放历史操作时不能按重启时间提前删键：后面的续期仍可能有效。
		// 完整重放后，普通读/写和清理 worker 再按当前时间处理最终期限。
		_, _, apply, unlock, err := e.prepareWrite(args, cmd, true)
		if unlock != nil {
			defer unlock()
		}
		if err == nil && apply != nil {
			apply()
		}
		return err
	default:
		return fmt.Errorf("invalid persisted command: %s", cmd)
	}
}
