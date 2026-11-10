// 本文件：Engine 和 NewEngine 建立内存引擎，Exec 是网络请求进入命令实现的入口。

package database

import (
	"bytes"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

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

// 分片数必须是非零的 2 的幂，路由才能用掩码取余；日志绑定和清理循环由启动者安排。
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

// Exec 只接受公开命令，私有 AOF 标记只能经 Replay 恢复，不能让客户端绕过命令语义。
// 统一命令名的大小写，但保留 key 和值的原始字节；调用期间参数不能被并发修改。
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
