package tcp_test

import (
	"context"
	"github.com/Li-Nepenthe/mini-redis/resp"
	"github.com/Li-Nepenthe/mini-redis/tcp"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type drainingParser struct {
	canceled chan struct{}
	once     sync.Once
}

func (p *drainingParser) ParseStream(ctx context.Context, r io.Reader) <-chan *resp.Payload {
	go func() { <-ctx.Done(); p.once.Do(func() { close(p.canceled) }) }()
	return resp.NewRespParser().ParseStream(ctx, r)
}

type delayedExecutor struct {
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (e *delayedExecutor) Exec([][]byte) (any, error) {
	if e.calls.Add(1) == 1 {
		close(e.started)
	}
	<-e.release
	return true, nil
}

func TestShutdownDrainsInFlightReplyAndRejectsPipeline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	parser := &drainingParser{canceled: make(chan struct{})}
	executor := &delayedExecutor{started: make(chan struct{}), release: make(chan struct{})}
	handler := resp.NewRespHandler(parser, executor)
	server := tcp.NewServer(listener.Addr().String(), handler)
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
	case <-parser.canceled:
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
