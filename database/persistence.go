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
	// Append 确认 args 的完整记录及同步；nil 才允许调用者提交内存，错误不得伪装成功。
	Append(args [][]byte) error
}

// AttachLog 将 log 绑定为后续写命令的确认日志；传 nil 表示不持久化，没有返回值。
// 不加锁，仅允许启动阶段在恢复后、并发服务/worker 开始前调用；运行时更换会产生数据竞争与确认边界歧义。
func (e *Engine) AttachLog(log CommandLog) { e.log = log }

// executeWrite 执行已归一化 cmd 的写命令，对 args 校验并准备结果，然后确认日志，最后应用内存变更。
// 返回命令结果/业务错误；日志失败包装为公开 persistence write failed 并保留内部原因，绝不调用 apply。无效果写入不追加。
// AOF 模式锁顺序为 logMu→有序 shard 锁→Store.mu，使内存提交顺序与日志一致；同分片读可能等待 fsync，这是确认语义的代价。
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

// prepareWrite 在 args 的参数/类型合法时，为归一化 cmd 准备 result、持久化 record、延迟修改 apply 和释放分片锁的 unlock。
// 错误或无效果操作可返回 nil apply/record；即使 err 非 nil，只要 unlock 非 nil 调用者仍须执行。apply 只能在同一批分片锁内调用一次。
// 正常路径可先惰性删除已过期旧值；这不表示新业务写入成功。replay 跳过按当前时间删历史值，避免续期/追加基于提前删除的状态执行。
// SET 清 TTL，List 新建记录 _LNEW，EXPIRE 记录绝对期限或 DEL；record/闭包引用的参数调用期间必须保持不变。
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
	// 以下 apply 闭包捕获已校验状态，统一由调用者在原分片锁内、日志确认后执行。
	// 不能提前运行闭包，也不能解锁后再使用捕获的共享 List；恢复路径不再次写日志。
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

// Replay 将 args 作为一条历史持久化记录恢复到内存，返回非法记录/类型/参数错误，不调用 Append。
// 接受 SET/LPUSH/_LNEW/LPOP/DEL 和私有 __EXPIREATMS；后者记录绝对毫秒期限，缺失 key 不新增数据。
// 历史回放不按重启当前时间提前过期；先还原所有记录，再由正常访问/worker 判断最终期限。调用者在服务启动前顺序调用，错误时不能把半恢复状态投入服务。
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
