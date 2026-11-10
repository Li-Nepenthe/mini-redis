// 本文件：TestRunShutdownFlushesAndReleasesAOF 贯穿真实 TCP、后台退出与 AOF 重开，检查启动者的停机顺序。

package main

import (
	"context"
	"github.com/Li-Nepenthe/mini-redis/aof"
	"github.com/Li-Nepenthe/mini-redis/database"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"
)

// 先等 SET 的确认回复，再触发停机；重开日志能恢复值并取得锁，才说明确认写和资源释放都完成了。
func TestRunShutdownFlushesAndReleasesAOF(t *testing.T) {
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := reservation.Addr().String()
	reservation.Close()
	path := filepath.Join(t.TempDir(), "shutdown.aof")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, addr, path) }()
	var client net.Conn
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		client, err = net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if client == nil {
		t.Fatalf("server did not start: %v", err)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(3 * time.Second))
	request := "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n"
	if _, err := io.WriteString(client, request); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 5)
	if _, err := io.ReadFull(client, reply); err != nil || string(reply) != "+OK\r\n" {
		t.Fatalf("SET reply=%q error=%v", reply, err)
	}
	start := time.Now()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish in 3 seconds")
	}
	engine := database.NewEngine(16)
	store, err := aof.Open(context.Background(), path, engine.Replay)
	if err != nil {
		t.Fatalf("AOF was not released: %v", err)
	}
	defer store.Close()
	value, err := engine.Exec([][]byte{[]byte("GET"), []byte("k")})
	if err != nil || string(value.([]byte)) != "v" {
		t.Fatalf("replay value=%v error=%v", value, err)
	}
	t.Logf("idle connection, cleanup and AOF closed; acknowledged write recovered in %s", time.Since(start))
}
