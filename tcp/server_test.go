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

// Handle 在进入/退出 inner.Handle 时用原子计数记录正在处理的连接，并转移 conn 给内部 Handler。
// 无返回值；计数用于回收时序观察，不能单独代表内部 Parser goroutine 数量。
func (h *observedHandler) Handle(conn net.Conn) {
	h.running.Add(1)
	defer h.running.Add(-1)
	h.inner.Handle(conn)
}

// Shutdown 将 ctx 原样交给 inner.Shutdown 并返回其错误，用观察包装器保持真实排空行为。
// 不额外关闭连接或重置 running，计数由各 Handle 自己退出时减少。
func (h *observedHandler) Shutdown(ctx context.Context) error { return h.inner.Shutdown(ctx) }

// Close 转发 inner.Close 并返回其错误，保持测试包装器的强制关闭/等待语义。
// 不自行改 running 计数，避免用清零掩盖仍在运行的处理 goroutine。
func (h *observedHandler) Close() error { return h.inner.Close() }

// startServer 在回环随机端口启动真实 Server/RESP/Engine，返回服务器、listener、观察 Handler 和 Serve 结果通道。
// t 在启动失败时终止并注册清理：Close 后等待关闭与 Serve 结果；只管理本测试资源。
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

// awaitError 等待 result 中一次操作的 error 并返回；3 秒无结果由 t.Fatal 报超时。
// 不取消被等待操作，也不关闭通道；调用者仍需安排自己的回收，缓冲结果通道避免结束发送阻塞。
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

// dial 在 1 秒内连接 addr，设置 3 秒连接截止并返回 net.Conn，失败由 t.Fatal 终止。
// 注册 t.Cleanup 关闭自己的连接，供有限时 TCP 集成断言；不控制服务器生命周期。
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

// exchange 向 conn 写 request，再 ReadFull 恰好 want 长度并比较字节，由 t 报写/读/结果错误，无返回值。
// 用于已知回复的有限测试，不是通用 RESP 客户端；超时由 dial 的 deadline 限制。
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

// waitFor 轮询 condition 直到为 true，超出 3 秒由 t.Fatal 输出 description，无返回值。
// Gosched/短 sleep 让后台有机会运行；不改变被观察状态，也不把一次条件满足证明为永久不变量。
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

// TestListenAndServePortConflict 预先占端口，验证第二次监听返回可由 errors.As 识别的 net.OpError 而不 panic。
// t 清理 listener/Handler，检验监听失败路径，不创建真实业务连接。
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

// Accept 增加 calls 并返回 nil 与注入 err，模拟永久接受错误或 net.ErrClosed。
// 不实际创建连接，计数帮助断言 Server 不在错误上忙循环重试。
func (l *errorListener) Accept() (net.Conn, error) { l.calls++; return nil, l.err }

// Close 为接受故障模拟 listener 返回 nil，满足 Server 回收接口而不操作真实资源。
// 不改变注入 err 或调用计数，测试主要观察 Accept 的一次失败行为。
func (l *errorListener) Close() error { return nil }

// Addr 返回空的 TCPAddr，满足模拟 listener 的 net.Listener 地址接口。
// 不分配端口或反映真实监听位置，无状态副作用。
func (l *errorListener) Addr() net.Addr { return &net.TCPAddr{} }

// TestServeAcceptErrors 注入永久错误与 net.ErrClosed，验证前者保留原因、后者正常停止，Accept 都只调用一次。
// t 等待 Serve 返回，覆盖弃用 Temporary 之外的明确错误策略，不声称所有暂时错误都可重试。
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

// TestMalformedClientsDoNotAffectHealthyConnection 在坏连接发送非法长度/超长头，检查固定协议错误后正常连接仍能 GET。
// t 管理多个回环连接，证明有限故障隔离样本，不把异常客户端输入传播为进程崩溃。
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

// TestHundredAbnormalDisconnectsReturnToBaseline 预热后使 100 个连接停在残请求并以 TCP RST 关闭，等待处理计数和 goroutine 回到基线。
// t 仅操作自己的连接，结合全局 NumGoroutine 作有限回收证据；不向其他进程清理资源。
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

// TestConcurrentCloseInterruptsIdleConnections 建立空闲阻塞连接，让 8 个 goroutine 并发 Close Server，验证全部返回且无运行 Handler。
// t 管理回收并等待，覆盖 Once 关闭和 Read 打断，不能据此推断被任意 Exec 卡住时也有界返回。
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
