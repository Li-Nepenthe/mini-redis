// 本文件：用受控执行器暂停第一条命令，验证 TCP 停机排空当前回复，同时拒绝流水线里尚未开始的命令。

package server

import (
	"context"
	"github.com/Li-Nepenthe/mini-redis/database"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type delayedExecutor struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

// Exec 由 release 控制完成时刻，保证测试能在命令执行中触发停机，而不用靠等待时间猜测。
func (e *delayedExecutor) Exec([][]byte) (any, error) {
	if e.calls.Add(1) == 1 {
		close(e.started)
	}
	<-e.release
	return database.StatusReply("OK"), nil
}

// TestShutdownDrainsInFlightReplyAndRejectsPipeline 发送两条粘连请求，在第一条执行中停机，再释放执行器。
// t 验证 listener 关闭、只完成第一回复、第二命令不执行、连接 EOF 与 Serve/Shutdown 都结束；用专用通道确定时序。
func TestShutdownDrainsInFlightReplyAndRejectsPipeline(t *testing.T) {
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var listener net.Listener = &closeObservedListener{Listener: raw, closed: make(chan struct{})}
	executor := &delayedExecutor{started: make(chan struct{}), release: make(chan struct{})}
	server := NewServer(listener.Addr().String(), executor)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	var release sync.Once
	defer func() { release.Do(func() { close(executor.release) }); _ = server.Close() }()
	client := dial(t, listener.Addr().String())
	if _, err := io.WriteString(client, setRequest+setRequest); err != nil {
		t.Fatal(err)
	}
	select {
	case <-executor.started:
	case <-time.After(3 * time.Second):
		t.Fatal("first command did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stopped := make(chan error, 1)
	start := time.Now()
	go func() { stopped <- server.Shutdown(ctx) }()
	select {
	case <-listener.(*closeObservedListener).closed:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not stop new requests")
	}
	if late, err := net.DialTimeout("tcp", listener.Addr().String(), 100*time.Millisecond); err == nil {
		late.Close()
		t.Fatal("listener remained open during drain")
	}
	release.Do(func() { close(executor.release) })
	reply := make([]byte, len("+OK\r\n"))
	if _, err := io.ReadFull(client, reply); err != nil || string(reply) != "+OK\r\n" {
		t.Fatalf("in-flight reply=%q error=%v", reply, err)
	}
	if _, err := client.Read(make([]byte, 1)); err != io.EOF {
		t.Fatalf("connection after reply: %v", err)
	}
	if err := awaitError(t, stopped); err != nil {
		t.Fatal(err)
	}
	if err := awaitError(t, served); err != nil {
		t.Fatal(err)
	}
	if executor.calls.Load() != 1 {
		t.Fatal("shutdown executed a queued pipeline command")
	}
	if elapsed := time.Since(start); elapsed >= 3*time.Second {
		t.Fatalf("shutdown took %s", elapsed)
	}
	t.Logf("listener stopped, one reply drained, queued command rejected in %s", time.Since(start))
}

// 观察真正的监听器关闭，使停机与释放执行器之间有确定的先后关系。
type closeObservedListener struct {
	net.Listener
	closed chan struct{}
	once   sync.Once
}

func (l *closeObservedListener) Close() error {
	err := l.Listener.Close()
	l.once.Do(func() { close(l.closed) })
	return err
}
