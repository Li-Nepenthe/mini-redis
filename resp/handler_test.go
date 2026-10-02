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

func (f executorFunc) Exec(args [][]byte) (any, error) { return f(args) }

type observedParser struct {
	ctx     context.Context
	ch      <-chan *Payload
	started chan struct{}
}

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

func (c *scriptedConn) Read(buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return 0, net.ErrClosed
	}
	return c.reader.Read(buf)
}

func (c *scriptedConn) Write(buf []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return c.replies.Write(buf)
}

func (c *scriptedConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

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
