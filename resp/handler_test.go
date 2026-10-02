package resp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type executorFunc func([][]byte) (any, error)

// Exec 将 args 原样转交接收者函数 f，并返回它的结果与错误，用函数注入业务故障/延迟。
// 适配 CommandExecutor 接口，不额外校验、加锁或改变输入；副作用完全由测试闭包定义。
func (f executorFunc) Exec(args [][]byte) (any, error) { return f(args) }

type observedParser struct {
	ctx     context.Context
	ch      <-chan *Payload
	started chan struct{}
}

// ParseStream 保存 ctx 和真实 Parser 返回的通道，必要时关闭 started 通知测试解析已开始，并返回原通道。
// 只为观察取消与通道结束，不替代真实解析行为；每个 observedParser 按测试设计只启动一次，否则 started 重关会 panic。
func (p *observedParser) ParseStream(ctx context.Context, reader io.Reader) <-chan *Payload {
	p.ctx = ctx
	p.ch = NewRespParser().ParseStream(ctx, reader)
	if p.started != nil {
		close(p.started)
	}
	return p.ch
}

type scriptedConn struct {
	net.Conn
	mu       sync.Mutex
	reader   *strings.Reader
	replies  bytes.Buffer
	writeErr error
	closed   bool
}

// Read 在模拟连接锁下从 reader 复制到 buf，返回字节数/读取错误；closed 时返回 net.ErrClosed。
// 用于有限内存输入，不模拟任意真实网络阻塞；保护测试的关闭状态与 reader 游标。
func (c *scriptedConn) Read(buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	return c.reader.Read(buf)
}

// Write 在模拟连接锁下将 buf 写入 replies 并返回写入结果；配置 writeErr 时直接返回零与该错误。
// 只记录内存回复，没有网络发送；便于断言错误编码与提前退出，当前实现不单独检查 closed。
func (c *scriptedConn) Write(buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return c.replies.Write(buf)
}

// Close 在模拟连接锁下设置 closed 并返回 nil，使后续 Read 返回 net.ErrClosed。
// 不关闭真实 socket，不等待 Parser；模拟器只为检查 Handler 是否履行连接回收动作。
func (c *scriptedConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// TestHandlerEarlyReturnStopsParser 注入写失败和不支持的结果类型，验证 Handler 提前返回也取消 Parser、关闭连接并等通道结束。
// t 用多个已预读帧触发潜在发送阻塞；不只检查发出了 cancel，而检查解析 goroutine 结束证据。
func TestHandlerEarlyReturnStopsParser(t *testing.T) {
	for _, test := range []struct {
		name     string
		result   any
		writeErr error
	}{
		{"write failure", true, errors.New("broken pipe")},
		{"encode failure", "unsupported result", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			parser := &observedParser{}
			handler := NewRespHandler(parser, executorFunc(func([][]byte) (any, error) { return test.result, nil }))
			defer handler.Close()
			conn := &scriptedConn{
				reader:   strings.NewReader(strings.Repeat("*1\r\n$3\r\nGET\r\n", 3)),
				writeErr: test.writeErr,
			}
			done := make(chan struct{})
			go func() { handler.Handle(conn); close(done) }()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatal("handler did not return")
			}
			if parser.ctx.Err() != context.Canceled {
				t.Fatal("handler did not cancel its parser context")
			}
			if _, ok := <-parser.ch; ok {
				t.Fatal("handler returned before parser channel closed")
			}
			if !conn.closed {
				t.Fatal("handler left connection open")
			}
		})
	}
}

// TestHandlerProtocolErrorIsSingleRESPLine 输入恶意多行非协议数据，验证执行器未调用且只回固定一行 Protocol error。
// t 比较模拟连接回复，防止原始输入/内部错误造成泄漏和 RESP 行注入，无真实网络资源。
func TestHandlerProtocolErrorIsSingleRESPLine(t *testing.T) {
	handler := NewRespHandler(NewRespParser(), executorFunc(func([][]byte) (any, error) {
		t.Fatal("malformed request reached executor")
		return nil, nil
	}))
	defer handler.Close()
	conn := &scriptedConn{reader: strings.NewReader("unexpected\r\n+injected\r\n")}
	handler.Handle(conn)
	if got := conn.replies.String(); got != "-ERR Protocol error\r\n" {
		t.Fatalf("protocol response = %q", got)
	}
}

// TestHandlerCloseInterruptsRead 用 net.Pipe 制造空闲阻塞 Read，验证 Close 打断并等待 Handle，重复关闭安全且拒后来的连接。
// t 通过 started/完成通道协调时序，不以任意 sleep 猜测回收是否完成。
func TestHandlerCloseInterruptsRead(t *testing.T) {
	parser := &observedParser{started: make(chan struct{})}
	handler := NewRespHandler(parser, executorFunc(func([][]byte) (any, error) { return true, nil }))
	server, client := net.Pipe()
	defer client.Close()
	handled := make(chan struct{})
	go func() { handler.Handle(server); close(handled) }()
	<-parser.started
	closed := make(chan struct{})
	go func() { _ = handler.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not interrupt an idle connection")
	}
	<-handled
	if err := handler.Close(); err != nil {
		t.Fatal(err)
	}
	lateServer, lateClient := net.Pipe()
	defer lateClient.Close()
	handler.Handle(lateServer)
	if _, err := lateClient.Write([]byte("x")); err == nil {
		t.Fatal("handler accepted connection after Close")
	}
}
