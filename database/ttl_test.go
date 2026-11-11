// 本文件：用 clockEngine 控制过期时刻，验证 TTL、索引和清理行为，并用并发与实际过期测试检查完整路径。

package database

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 原子时钟让并发测试可以推进时间，同时避开真实睡眠带来的边界抖动。
func clockEngine() (*Engine, *atomic.Int64) {
	e := NewEngine(16)
	var elapsed atomic.Int64
	base := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	e.now = func() time.Time { return base.Add(time.Duration(elapsed.Load())) }
	return e, &elapsed
}

// TestTTLSemantics 直接推进时钟，区分 TTL 的显示取整和真正到期，并检查覆盖、立即删除时旧期限不会残留。
func TestTTLSemantics(t *testing.T) {
	e, elapsed := clockEngine()
	requireExec(t, e, int64(-2), "TTL", "k")
	requireExec(t, e, int64(0), "EXPIRE", "k", "5")
	requireExec(t, e, StatusReply("OK"), "SET", "k", "v")
	requireExec(t, e, int64(-1), "TTL", "k")
	requireExec(t, e, int64(1), "expire", "k", "5")
	requireExec(t, e, int64(5), "ttl", "k")
	elapsed.Store(int64(400 * time.Millisecond))
	requireExec(t, e, int64(5), "TTL", "k")
	elapsed.Store(int64(1100 * time.Millisecond))
	requireExec(t, e, int64(4), "TTL", "k")
	elapsed.Store(int64(5 * time.Second))
	requireExec(t, e, nil, "GET", "k")
	requireExec(t, e, int64(-2), "TTL", "k")
	requireExec(t, e, StatusReply("OK"), "SET", "k", "v")
	requireExec(t, e, int64(1), "EXPIRE", "k", "5")
	requireExec(t, e, StatusReply("OK"), "SET", "k", "replacement")
	requireExec(t, e, int64(-1), "TTL", "k")
	requireExec(t, e, int64(1), "EXPIRE", "k", "0")
	requireExec(t, e, int64(-2), "TTL", "k")
	requireExec(t, e, StatusReply("OK"), "SET", "k", "v")
	requireExec(t, e, int64(1), "EXPIRE", "k", "-1")
	requireExec(t, e, nil, "GET", "k")
}

// TestExpiredKeysAreMissingForEveryCommand 每次重建过期值，让每条命令自己触发惰性删除，避免前一次读取代劳。
func TestExpiredKeysAreMissingForEveryCommand(t *testing.T) {
	for _, test := range []struct {
		args []string
		want any
	}{
		{[]string{"GET", "k"}, nil},
		{[]string{"EXISTS", "k", "k"}, int64(0)},
		{[]string{"DEL", "k"}, int64(0)},
		{[]string{"LPUSH", "k", "new"}, int64(1)},
		{[]string{"LPOP", "k"}, nil},
		{[]string{"LRANGE", "k", "0", "-1"}, [][]byte{}},
		{[]string{"TTL", "k"}, int64(-2)},
		{[]string{"EXPIRE", "k", "5"}, int64(0)},
	} {
		e, elapsed := clockEngine()
		requireExec(t, e, StatusReply("OK"), "SET", "k", "old string")
		requireExec(t, e, int64(1), "EXPIRE", "k", "1")
		elapsed.Store(int64(time.Second))
		requireExec(t, e, test.want, test.args...)
	}
}

// TestListKeepsTTLAndExpiryIndexIsConsistent 区分列表内容变化和 key 消失：前者保留期限，后者清掉期限与抽样索引。
func TestListKeepsTTLAndExpiryIndexIsConsistent(t *testing.T) {
	e, _ := clockEngine()
	requireExec(t, e, int64(1), "LPUSH", "list", "a")
	requireExec(t, e, int64(1), "EXPIRE", "list", "10")
	requireExec(t, e, int64(2), "LPUSH", "list", "b")
	requireExec(t, e, int64(10), "TTL", "list")
	requireExec(t, e, []byte("b"), "LPOP", "list")
	requireExec(t, e, int64(10), "TTL", "list")
	requireExec(t, e, []byte("a"), "LPOP", "list")
	if _, exists := e.getShard("list").expires["list"]; exists {
		t.Fatal("LPOP left an expiry entry after removing the last element")
	}
	requireExec(t, e, int64(-2), "TTL", "list")
	for i := 0; i < 40; i++ {
		key := strconv.Itoa(i)
		requireExec(t, e, StatusReply("OK"), "SET", key, "v")
		requireExec(t, e, int64(1), "EXPIRE", key, "10")
		requireExec(t, e, int64(1), "EXPIRE", key, "20")
		if i%2 == 0 {
			requireExec(t, e, int64(1), "DEL", key)
		}
	}
	for _, s := range e.shards {
		if len(s.expiring) != len(s.expires) {
			t.Fatal("expiry index length mismatch")
		}
		for i, key := range s.expiring {
			if s.expires[key].index != i {
				t.Fatal("expiry index did not follow swap removal")
			}
		}
	}
}

// 把整数解析失败与秒转 Duration 溢出都挡在修改期限之前。
func TestExpireArgumentErrors(t *testing.T) {
	e := NewEngine(16)
	for _, args := range [][]string{{"EXPIRE", "k"}, {"TTL"}, {"TTL", "k", "extra"}} {
		_, err := e.Exec(command(args...))
		if !errors.Is(err, ErrWrongArgsNum) {
			t.Fatalf("%q: %v", args, err)
		}
	}
	for _, value := range []string{"bad", "9223372036854775808", "9223372036854775807"} {
		_, err := e.Exec(command("EXPIRE", "k", value))
		if !errors.Is(err, ErrInvalidInteger) {
			t.Fatalf("accepted invalid expiry %q: %v", value, err)
		}
	}
}

// 取消后等待清理循环退出，防止测试结束时仍遗留后台工作。
func TestCleanupCancellation(t *testing.T) {
	e := NewEngine(16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.RunCleanup(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not stop on context cancellation")
	}
}

// 让命令与主动清理同时访问期限和数据，用 race 检查它们是否共享同一套锁。
func TestConcurrentExpirationAndCleanup(t *testing.T) {
	e := NewEngine(16)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			key := strconv.Itoa(worker)
			for i := 0; i < 100; i++ {
				for _, args := range [][][]byte{command("SET", key, "v"), command("EXPIRE", key, "1"), command("TTL", key), command("GET", key), command("EXPIRE", key, "0")} {
					if _, err := e.Exec(args); err != nil {
						t.Error(err)
					}
				}
				e.cleanupExpired()
			}
		}(worker)
	}
	workers.Wait()
}

// TestExpireHundredThousandKeysWithoutReads 不读取写入的数据，让后台清理独自处理过期。
// 清理前后都在 GC 后测活跃堆，只比较仍被引用的内存，不把它当作操作系统工作集。
func TestExpireHundredThousandKeysWithoutReads(t *testing.T) {
	if testing.Short() {
		t.Skip("real-time 100000-key active-expiry acceptance")
	}
	e := NewEngine(16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { e.RunCleanup(ctx); close(done) }()
	defer func() { cancel(); <-done }()
	value := string(make([]byte, 256))
	for i := 0; i < 100000; i++ {
		key := "expire-" + strconv.Itoa(i)
		requireExec(t, e, StatusReply("OK"), "SET", key, value)
		requireExec(t, e, int64(1), "EXPIRE", key, "5")
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	deadline := started.Add(30 * time.Second)
	for {
		remaining := 0
		for _, s := range e.shards {
			s.mu.RLock()
			remaining += len(s.data)
			s.mu.RUnlock()
		}
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("active expiry left %d keys after 30 seconds", remaining)
		}
		time.Sleep(100 * time.Millisecond)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(e)
	if after.HeapAlloc >= before.HeapAlloc {
		t.Fatalf("live heap did not fall: before=%d after=%d", before.HeapAlloc, after.HeapAlloc)
	}
	t.Logf("100000 keys with 5s TTL, no reads: all removed in %s after writes; live HeapAlloc %d -> %d bytes; map capacity may remain", time.Since(started).Round(time.Millisecond), before.HeapAlloc, after.HeapAlloc)
}
