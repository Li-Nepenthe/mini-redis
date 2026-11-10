// 本文件：TestShutdownTimeoutDoesNotHideBlockedExecutor 区分停机超时与执行器真正结束，防止超时掩盖未完成的工作。

package resp

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/database"
)

// TestShutdownTimeoutDoesNotHideBlockedExecutor 故意阻塞执行器，先观察停机超时，再放行并等待回收，区分超时返回和工作结束。
func TestShutdownTimeoutDoesNotHideBlockedExecutor(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	handler := NewRespHandler(NewRespParser(), executorFunc(func([][]byte) (any, error) {
		close(started)
		<-release
		return database.StatusReply("OK"), nil
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
