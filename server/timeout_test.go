// 本文件：TestShutdownTimeoutDoesNotHideBlockedExecutor 区分停机超时与执行器真正结束，防止超时掩盖未完成的工作。

package server

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/database"
)

// 超时先返回给调用者，强制 Close 仍须等执行器结束，不能用关闭连接冒充工作已经回收。
func TestShutdownTimeoutDoesNotHideBlockedExecutor(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	s := NewServer("", executorFunc(func([][]byte) (any, error) {
		close(started)
		<-release
		return database.StatusReply("OK"), nil
	}))
	var once sync.Once
	defer func() { once.Do(func() { close(release) }); _ = s.Close() }()
	conn, client := net.Pipe()
	defer client.Close()
	s.startConnection(conn)
	if _, err := io.WriteString(client, "*1\r\n$4\r\nPING\r\n"); err != nil {
		t.Fatal(err)
	}
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := s.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
		t.Fatalf("shutdown error=%v elapsed=%s", err, time.Since(start))
	}
	if connectionCount(s) != 1 {
		t.Fatal("timeout hid blocked executor")
	}
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case <-closed:
		t.Fatal("Close returned before blocked executor finished")
	case <-time.After(20 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	if err := awaitError(t, closed); err != nil {
		t.Fatal(err)
	}
	if connectionCount(s) != 0 {
		t.Fatal("released executor left connection work behind")
	}
}
