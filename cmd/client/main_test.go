// 本文件：通过真实服务执行 LPUSH、LRANGE 后再 PING，验证客户端发送编码和完整回复读取串在一起时不会错位。

package main

import (
	"bytes"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/database"
	"github.com/Li-Nepenthe/mini-redis/server"
)

func TestClientSession(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := server.NewServer(listener.Addr().String(), database.NewEngine(16))
	done := make(chan error, 1)
	go func() { done <- s.Serve(listener) }()
	defer func() {
		_ = s.Close()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	var output bytes.Buffer
	err = runClient(conn, strings.NewReader("LPUSH list a b\nLRANGE list 0 -1\nPING\nexit\n"), &output)
	want := "mini-redis> (integer) 2\nmini-redis> 1) \"b\"\n2) \"a\"\nmini-redis> PONG\nmini-redis> "
	if err != nil || output.String() != want {
		t.Fatalf("session=%q err=%v want=%q", output.String(), err, want)
	}
}
