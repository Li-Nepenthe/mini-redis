// 本文件：setExpiration 和 clearExpiration 维护抽样索引，lockRead 做惰性删除，RunCleanup 清理无人读取的过期值。

package database

import (
	"context"
	"math/rand/v2"
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

// 续期只改期限，避免同一个 key 重复进入抽样池。
func (s *shard) setExpiration(key string, deadline time.Time) {
	if old, ok := s.expires[key]; ok {
		old.deadline = deadline
		s.expires[key] = old
		return
	}
	s.expires[key] = expiration{deadline: deadline, index: len(s.expiring)}
	s.expiring = append(s.expiring, key)
}

// 抽样池没有顺序要求，用末项补洞并修正下标就能 O(1) 删除；清空旧末项，避免底层数组继续持有已删 key。
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

// 数据和过期索引必须一起删除。清理不写日志，恢复依靠绝对期限和新建列表标记。
func (s *shard) purgeExpired(key string, now time.Time) bool {
	entry, ok := s.expires[key]
	if !ok || now.Before(entry.deadline) {
		return false
	}
	delete(s.data, key)
	s.clearExpiration(key)
	return true
}

// 读锁不能原地升级。发现过期后先放读锁，再持写锁重新检查，避免把间隙中重建的新值删掉。
// 返回时仍持有读锁，读取者必须释放；返回的时间也供 TTL 计算使用，避免同一次读取跨越两个时刻。
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

// 每个引擎由启动者运行一个清理循环并等待退出。有限抽样避免全表扫描长时间占锁。
// 只有累计删掉足够多的 key 才请求归还内存，且至少间隔五秒，避免每轮都强制 GC。
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
				e.reclaim()
				deleted = 0
				lastReclaim = time.Now()
			}
		}
	}
}

// 每个分片只检查固定数量的随机样本就放锁，让请求有机会继续执行；单轮不保证删完所有过期值。
func (e *Engine) cleanupExpired() int {
	now := e.now()
	deleted := 0
	for _, s := range e.shards {
		s.mu.Lock()
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
