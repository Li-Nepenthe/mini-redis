// 本文件：CommandLog 连接引擎与日志，AttachLog 绑定日志，Replay 复用写计划恢复历史记录和绝对期限。

package database

import (
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
