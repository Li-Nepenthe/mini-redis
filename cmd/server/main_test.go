// 本文件：TestRunCancellationStopsBackgroundWork 和 TestRunListenFailureStopsBackgroundWork 检查启动取消及监听冲突的退出路径。

package main

import (
	"context"
	"net"
	"testing"
	"time"
)

// 取消可能发生在启动前，也可能发生在服务运行中，两种时序都必须让 run 返回。
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

// 端口被占用时应在建立引擎和后台工作前返回，避免失败的启动留下资源。
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
