package tcp_test

import (
	"bufio"
	"context"
	"errors"
	"github.com/Li-Nepenthe/mini-redis/database"
	"github.com/Li-Nepenthe/mini-redis/resp"
	"github.com/Li-Nepenthe/mini-redis/tcp"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type observedHandler struct {
	inner   tcp.Handler
	running atomic.Int32
}

func (h *observedHandler) Handle(conn net.Conn) {
	h.running.Add(1)
	defer h.running.Add(-1)
	h.inner.Handle(conn)
}

func (h *observedHandler) Shutdown(ctx context.Context) error { return h.inner.Shutdown(ctx) }

func (h *observedHandler) Close() error { return h.inner.Close() }

func startServer(t *testing.T) (*tcp.Server, net.Listener, *observedHandler, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	handler := &observedHandler{inner: resp.NewRespHandler(resp.NewRespParser(), database.NewEngine(16))}
	server := tcp.NewServer(listener.Addr().String(), handler)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		closed := make(chan error, 1)
		go func() { closed <- server.Close() }()
		if err := awaitError(t, closed); err != nil {
			t.Error(err)
		}
		if err := awaitError(t, done); err != nil {
			t.Error(err)
		}
	})
	return server, listener, handler, done
}

func awaitError(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("operation did not finish in 3 seconds")
		return nil
	}
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func exchange(t *testing.T, conn net.Conn, request, want string) {
	t.Helper()
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("response = %q, want %q", got, want)
	}
}

func waitFor(t *testing.T, description string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(description)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

func TestListenAndServePortConflict(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	handler := resp.NewRespHandler(resp.NewRespParser(), database.NewEngine(16))
	defer handler.Close()
	server := tcp.NewServer(listener.Addr().String(), handler)
	err = server.ListenAndServe()
	var opErr *net.OpError
	if !errors.As(err, &opErr) || opErr.Op != "listen" {
		t.Fatalf("want wrapped listen error, got %v", err)
	}
	t.Logf("second listener returned error without panic: %v", err)
}

type errorListener struct {
	err   error
	calls int
}

func (l *errorListener) Accept() (net.Conn, error) { l.calls++; return nil, l.err }
func (l *errorListener) Close() error              { return nil }
func (l *errorListener) Addr() net.Addr            { return &net.TCPAddr{} }

func TestServeAcceptErrors(t *testing.T) {
	permanent := errors.New("permanent accept failure")
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{"permanent", permanent, permanent},
		{"closed", net.ErrClosed, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := resp.NewRespHandler(resp.NewRespParser(), database.NewEngine(16))
			listener := &errorListener{err: test.err}
			server := tcp.NewServer("", handler)
			done := make(chan error, 1)
			go func() { done <- server.Serve(listener) }()
			err := awaitError(t, done)
			if !errors.Is(err, test.want) || listener.calls != 1 {
				t.Fatalf("error=%v accept calls=%d", err, listener.calls)
			}
		})
	}
}

const setRequest = "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n"
const getRequest = "*2\r\n$3\r\nGET\r\n$1\r\nk\r\n"

func TestMalformedClientsDoNotAffectHealthyConnection(t *testing.T) {
	_, listener, _, _ := startServer(t)
	healthy := dial(t, listener.Addr().String())
	exchange(t, healthy, setRequest, "+OK\r\n")
	for _, input := range []string{"*-1\r\n", "*1000000000\r\n", "*1\r\n$-1\r\n", "*" + strings.Repeat("9", 5000)} {
		bad := dial(t, listener.Addr().String())
		if _, err := io.WriteString(bad, input); err != nil {
			t.Fatal(err)
		}
		line, err := bufio.NewReader(bad).ReadString('\n')
		if err != nil || line != "-ERR Protocol error\r\n" {
			t.Fatalf("malformed request response=%q err=%v", line, err)
		}
		_ = bad.Close()
		exchange(t, healthy, getRequest, "$1\r\nv\r\n")
	}
}

func TestHundredAbnormalDisconnectsReturnToBaseline(t *testing.T) {
	_, listener, handler, _ := startServer(t)
	warmup := dial(t, listener.Addr().String())
	exchange(t, warmup, setRequest, "+OK\r\n")
	_ = warmup.Close()
	waitFor(t, "warmup handler did not finish", func() bool { return handler.running.Load() == 0 })
	baseline := runtime.NumGoroutine()
	conns := make([]net.Conn, 100)
	for i := range conns {
		conns[i] = dial(t, listener.Addr().String())
		if _, err := io.WriteString(conns[i], "*3\r\n$3\r\nSET\r\n"); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "not all 100 connections reached handlers", func() bool { return handler.running.Load() == 100 })
	for _, conn := range conns {
		if err := conn.(*net.TCPConn).SetLinger(0); err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
	}
	waitFor(t, "handlers or parsers leaked after TCP resets", func() bool {
		return handler.running.Load() == 0 && runtime.NumGoroutine() <= baseline
	})
	t.Logf("100 abnormal disconnects: goroutines baseline=%d after=%d", baseline, runtime.NumGoroutine())
}

func TestConcurrentCloseInterruptsIdleConnections(t *testing.T) {
	server, listener, handler, _ := startServer(t)
	_ = dial(t, listener.Addr().String())
	waitFor(t, "idle connection did not reach handler", func() bool { return handler.running.Load() == 1 })
	var closers sync.WaitGroup
	closers.Add(8)
	for i := 0; i < 8; i++ {
		go func() {
			defer closers.Done()
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	done := make(chan error, 1)
	go func() { closers.Wait(); done <- nil }()
	_ = awaitError(t, done)
	if handler.running.Load() != 0 {
		t.Fatal("Close returned with running handler")
	}
}
