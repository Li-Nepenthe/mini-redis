package database

import (
	"context"
	"math"
	"math/rand/v2"
	"strconv"
	"time"
)

type expiration struct {
	deadline time.Time
	index    int
}

const (
	cleanupInterval        = 250 * time.Millisecond
	cleanupSamples         = 256
	cleanupReclaimKeys     = 32768
	cleanupReclaimInterval = 5 * time.Second
)

// setExpiration 为 key 设置 deadline，并维护 expires→expiring 的索引；已有 key 续期只改期限、不重复入池。
// 无返回值，调用者须持 shard 写锁，并保证业务上 key 已存在；本函数不主动检查 data 或删除已经过去的期限。
func (s *shard) setExpiration(key string, deadline time.Time) {
	if old, ok := s.expires[key]; ok {
		old.deadline = deadline
		s.expires[key] = old
		return
	}
	s.expires[key] = expiration{deadline: deadline, index: len(s.expiring)}
	s.expiring = append(s.expiring, key)
}

// clearExpiration 移除 key 的 TTL 和抽样索引；不存在时无操作，无返回值，调用者须持 shard 写锁。
// 用末项补洞并修正 movedKey 反向索引实现 O(1) 删除，再清空旧末项字符串避免底层数组保留已删 key 引用；抽样池无顺序语义。
func (s *shard) clearExpiration(key string) {
	old, ok := s.expires[key]
	if !ok {
		return
	}
	last := len(s.expiring) - 1
	movedKey := s.expiring[last]
	s.expiring[old.index] = movedKey
	s.expiring[last] = ""
	s.expiring = s.expiring[:last]
	delete(s.expires, key)
	if old.index < last {
		moved := s.expires[movedKey]
		moved.index = old.index
		s.expires[movedKey] = moved
	}
}

// purgeExpired 在 now 达到或超过 key 的期限时删除 data 与 TTL 索引，返回是否执行了过期删除。
// 未设置期限或尚未到期返回 false；调用者必须持分片写锁。清理不写 AOF，日志通过绝对期限和新建边界恢复最终状态。
func (s *shard) purgeExpired(key string, now time.Time) bool {
	entry, ok := s.expires[key]
	if !ok || now.Before(entry.deadline) {
		return false
	}
	delete(s.data, key)
	s.clearExpiration(key)
	return true
}

// lockRead 为 key 获取可读取当前状态的分片，返回 shard 和本轮 now；返回时仍持该 shard 的 RLock，调用者须 RUnlock。
// 遇到过期值时先放读锁，再加写锁按新时间重新检查并删除，然后重试；RWMutex 不可原地升级。
// 间隙内其他写者可能 SET 重建同名值并设置新期限，重查避免用旧快照删除新状态；普通 EXPIRE 不会续活已过期旧值。
// 该函数可能产生 TTL 清理副作用。
func (e *Engine) lockRead(key string) (*shard, time.Time) {
	s := e.getShard(key)
	for {
		s.mu.RLock()
		now := e.now()
		entry, ok := s.expires[key]
		if !ok || now.Before(entry.deadline) {
			return s, now
		}
		s.mu.RUnlock()
		s.mu.Lock()
		s.purgeExpired(key, e.now())
		s.mu.Unlock()
	}
}

// parseExpirySeconds 将 value 的十进制文字解析为 int64 秒，返回秒数或 ErrInvalidInteger。
// 非整数/超出 int64/正数换成 time.Duration 会溢出的值拒绝；零和负数保留供 EXPIRE 立即删除，不操作存储。
func parseExpirySeconds(value []byte) (int64, error) {
	seconds, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil || seconds > math.MaxInt64/int64(time.Second) {
		return 0, ErrInvalidInteger
	}
	return seconds, nil
}

// ttl 执行 TTL，args 必须为命令名和 key，返回 int64 秒数：不存在/过期为 -2，存在无期限为 -1。
// 剩余秒按本项目规则四舍五入（余量达到 500ms 进一秒）；数量非法返回业务错误。
// lockRead 可能清理过期值，返回读锁由本函数释放；显示 0 不等于已经过期，是否存活仍按精确 deadline 判断。
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

// RunCleanup 阻塞运行 TTL 清理循环，ctx 取消后停止 ticker 并返回，没有独立错误返回。
// 每 250ms 有限抽样；累计删除至少 32768 key 且距上次回收至少 5 秒时，在所有分片锁外调用 reclaim 请求归还 OS 内存。
// 启动者只能为一个 Engine 启动一个该 worker，并必须等待退出；不为每 key 创建定时器，也不在每 tick 强制 GC。回收仍有全局 GC 成本。
func (e *Engine) RunCleanup(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	var lastReclaim time.Time
	deleted := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted += e.cleanupExpired()
			if deleted >= cleanupReclaimKeys && time.Since(lastReclaim) >= cleanupReclaimInterval {
				if ctx.Err() != nil {
					return
				}
				// 空闲进程可能很久没有新分配来触发 GC；大批过期后才请求回收，且不持分片锁。
				// 最多每 5 秒一次，避免每轮抽样都强制 GC；仍有全局 GC 暂停和归还内存的成本。
				e.reclaim()
				deleted = 0
				lastReclaim = time.Now()
			}
		}
	}
}

// cleanupExpired 取一次当前时间并逐分片抽样删除已到期 key，返回本轮实际删除数量。
// 每分片持写锁最多检查 256 次，随机样本可能重复；有限预算后放锁，避免全表扫描长时间阻塞命令。
// 维护 data/TTL 索引，不追加 AOF，不调用 GC，不承诺这一轮已经删完全部过期数据。
func (e *Engine) cleanupExpired() int {
	now := e.now()
	deleted := 0
	for _, s := range e.shards {
		s.mu.Lock()
		// 索引切片让随机抽样为 O(1)；每分片最多检查 256 次就放锁，避免全表扫描阻塞请求。
		for i := 0; i < cleanupSamples && len(s.expiring) > 0; i++ {
			key := s.expiring[rand.IntN(len(s.expiring))]
			if s.purgeExpired(key, now) {
				deleted++
			}
		}
		s.mu.Unlock()
	}
	return deleted
}
