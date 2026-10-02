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

// 调用者持 shard 写锁。续期保留索引，避免同一个 key 在抽样池里重复占位。
func (s *shard) setExpiration(key string, deadline time.Time) {
	if old, ok := s.expires[key]; ok {
		old.deadline = deadline
		s.expires[key] = old
		return
	}
	s.expires[key] = expiration{deadline: deadline, index: len(s.expiring)}
	s.expiring = append(s.expiring, key)
}

// 抽样池不要求顺序；用末项补洞并修正反向索引，删除为 O(1)。
// 清空旧末项的字符串，避免切片底层数组继续保留已删除 key 的引用；调用者持写锁。
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

func (s *shard) purgeExpired(key string, now time.Time) bool {
	entry, ok := s.expires[key]
	if !ok || now.Before(entry.deadline) {
		return false
	}
	delete(s.data, key)
	s.clearExpiration(key)
	return true
}

// RWMutex 不能原地升级；释放读锁后加写锁并重新检查，避免误删并发续期的新值。
// 成功返回时仍持 RLock，调用者必须 RUnlock；重试是为了在删/续期后读取新的状态。
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

func parseExpirySeconds(value []byte) (int64, error) {
	seconds, err := strconv.ParseInt(string(value), 10, 64)
	if err != nil || seconds > math.MaxInt64/int64(time.Second) {
		return 0, ErrInvalidInteger
	}
	return seconds, nil
}

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

// 所有者只启动一个清理 goroutine，并在取消后等待退出；不为每 key 创建定时器。
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
