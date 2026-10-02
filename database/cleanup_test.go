package database

import (
	"context"
	"strconv"
	"testing"
	"time"
)

// TestLargeExpirationReclaimsOutsideShardLocks 用可控时钟使阈值以上 key 过期，注入 reclaim 检查调用时所有分片锁都可获取。
// t 等待回收信号再取消 worker 并等待退出；验证调用边界，不测 OS 工作集或真实 GC 耗时。
func TestLargeExpirationReclaimsOutsideShardLocks(t *testing.T) {
	engine, elapsed := clockEngine()
	for i := 0; i < cleanupReclaimKeys+1; i++ {
		key := strconv.Itoa(i)
		requireExec(t, engine, true, "SET", key, "value")
		requireExec(t, engine, 1, "EXPIRE", key, "1")
	}
	elapsed.Store(int64(2 * time.Second))
	reclaimed := make(chan struct{})
	engine.reclaim = func() {
		for _, shard := range engine.shards {
			if !shard.mu.TryLock() {
				t.Error("memory reclamation ran while holding a shard lock")
				continue
			}
			shard.mu.Unlock()
		}
		close(reclaimed)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { engine.RunCleanup(ctx); close(done) }()
	select {
	case <-reclaimed:
	case <-time.After(4 * time.Second):
		t.Fatal("large idle expiration did not request memory reclamation")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not stop after reclamation and cancellation")
	}
}
