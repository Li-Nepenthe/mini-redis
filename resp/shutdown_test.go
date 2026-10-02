package resp

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// TestShutdownTimeoutDoesNotHideBlockedExecutor 将执行器阻塞到 release，验证 Shutdown 到期返回 DeadlineExceeded 而不假装工作结束。
// t 随后释放执行器并等待 Handle/Close，证明网络超时与最终回收是两个动作；不向外部服务发信号。
func TestShutdownTimeoutDoesNotHideBlockedExecutor(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	handler := NewRespHandler(NewRespParser(), executorFunc(func([][]byte) (any, error) {
		close(started)
		<-release
		return true, nil
	}))
	server, client := net.Pipe()
	defer client.Close()
	handled := make(chan struct{})
	go func() { handler.Handle(server); close(handled) }()
	go func() { _, _ = io.WriteString(client, "*1\r\n$4\r\nPING\r\n") }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := handler.Shutdown(ctx)
	close(release)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("shutdown error=%v elapsed=%s", err, time.Since(start))
	}
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("released executor left a parser or handler behind")
	}
	if err := handler.Close(); err != nil {
		t.Fatal(err)
	}
}
