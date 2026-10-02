package main

import (
	"context"
	"net"
	"testing"
	"time"
)

// TestRunCancellationStopsBackgroundWork 验证取消 ctx 后 run 能在 3 秒内返回 nil，没有返回值。
// 取消可能先于服务启动，允许 run 直接返回；不单凭这一测试证明运行中各 worker 的完整退出时序，另由停机集成测试补足。
func TestRunCancellationStopsBackgroundWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- run(ctx, "127.0.0.1:0", "") }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("canceled server did not stop")
	}
}

// TestRunListenFailureStopsBackgroundWork 先占用临时端口，验证 run 在监听冲突时返回错误。
// t 管理端口释放；当前 run 在创建引擎/worker 前就返回，测试没有独立统计 goroutine 数量。
func TestRunListenFailureStopsBackgroundWork(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := run(context.Background(), listener.Addr().String(), ""); err == nil {
		t.Fatal("run hid a listen failure")
	}
}
