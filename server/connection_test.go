// 本文件：用 net.Pipe 和受控执行器检查连接提前退出、协议错误与停机后的资源回收。

package server

import (
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/database"
)

type executorFunc func([][]byte) (any, error)

func (f executorFunc) Exec(args [][]byte) (any, error) { return f(args) }

// 两种失败都发生在真实读取之后；连接退出时必须注销，且不能执行缓冲里剩下的请求。
func TestConnectionEarlyReturnReleasesResources(t *testing.T) {
	for _, name := range []string{"write failure", "encode failure"} {
		t.Run(name, func(t *testing.T) {
			conn, client := net.Pipe()
			defer client.Close()
			var calls atomic.Int32
			s := NewServer("", executorFunc(func([][]byte) (any, error) {
				calls.Add(1)
				if name == "write failure" {
					_ = client.Close()
					return database.StatusReply("OK"), nil
				}
				return "unsupported result", nil
			}))
			defer s.Close()
			s.startConnection(conn)
			if _, err := io.WriteString(client, setRequest+setRequest+setRequest); err != nil {
				t.Fatal(err)
			}
			waitFor(t, "connection work did not finish", func() bool { return connectionCount(s) == 0 })
			if calls.Load() != 1 {
				t.Fatal("failed reply executed a queued request")
			}
			if _, err := client.Write([]byte("x")); err == nil {
				t.Fatal("connection left open after failure")
			}
		})
	}
}

func TestConnectionProtocolErrorIsSingleRESPLine(t *testing.T) {
	var calls atomic.Int32
	s := NewServer("", executorFunc(func([][]byte) (any, error) { calls.Add(1); return nil, nil }))
	defer s.Close()
	conn, client := net.Pipe()
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(2 * time.Second))
	s.startConnection(conn)
	if _, err := io.WriteString(client, "unexpected\r\n+injected\r\n"); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(client)
	if err != nil || string(got) != "-ERR Protocol error\r\n" {
		t.Fatalf("protocol response = %q, err=%v", got, err)
	}
	if calls.Load() != 0 {
		t.Fatal("malformed request reached executor")
	}
}

func TestCloseInterruptsReadAndRejectsLateConnection(t *testing.T) {
	s := NewServer("", database.NewEngine(16))
	conn, client := net.Pipe()
	defer client.Close()
	s.startConnection(conn)
	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	if err := awaitError(t, closed); err != nil {
		t.Fatal(err)
	}
	if connectionCount(s) != 0 {
		t.Fatal("Close returned before connection work finished")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	late, lateClient := net.Pipe()
	defer lateClient.Close()
	if s.startConnection(late) {
		t.Fatal("accepted connection after Close")
	}
	if _, err := lateClient.Write([]byte("x")); err == nil {
		t.Fatal("late connection remained open")
	}
}
