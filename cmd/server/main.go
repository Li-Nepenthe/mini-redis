package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/Li-Nepenthe/mini-redis/aof"
	"github.com/Li-Nepenthe/mini-redis/database"
	"github.com/Li-Nepenthe/mini-redis/resp"
	"github.com/Li-Nepenthe/mini-redis/tcp"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// main 解析 -addr/-aof 参数，订阅 Ctrl+C/SIGTERM，并将信号 context 交给 run 启动服务。
// -aof 为空关闭持久化；run 返回错误时记录并退出进程。正常退出取消信号订阅，无业务返回值。
func main() {
	addr := flag.String("addr", ":6379", "TCP listen address")
	aofPath := flag.String("aof", "data/appendonly.aof", "AOF file; empty disables persistence")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *addr, *aofPath); err != nil {
		log.Fatal(err)
	}
}

// run 根据 addr 启动 TCP 服务，根据 aofPath 选择恢复并绑定日志；ctx 已取消时直接返回 nil。
// 先获得监听端口再打开 AOF，避免重复启动时碰触在用日志；依次组装 16 分片 Engine、Parser、Handler 和 Server。
// 拥有清理 worker、信号观察 goroutine 和文件：网络结束后取消并等待 worker，再 Close AOF；defer 后进先出保障该顺序。
// 返回监听/恢复/服务/文件关闭错误。2 秒是网络排空预算，不能强行取消任意 Exec、fsync 或同步 GC。
func run(ctx context.Context, addr, aofPath string) (runErr error) {
	if ctx.Err() != nil {
		return nil
	}
	// 先占端口再开 AOF，重复启动失败时不能触碰正在运行服务的日志。
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	fmt.Println("Mini-Redis 服务端启动中....")

	// 引擎与 RESP 通过接口连接，协议测试可注入慢执行器/故障而不用复制业务实现。
	dbEngine := database.NewEngine(16)
	if aofPath != "" {
		if err := os.MkdirAll(filepath.Dir(aofPath), 0o755); err != nil {
			return fmt.Errorf("create AOF directory: %w", err)
		}
		store, err := aof.Open(ctx, aofPath, dbEngine.Replay)
		if err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return nil
			}
			return err
		}
		if store.RecoveredTailBytes != 0 {
			log.Printf("AOF recovery removed %d incomplete tail bytes", store.RecoveredTailBytes)
		}
		dbEngine.AttachLog(store)
		defer func() { runErr = errors.Join(runErr, store.Close()) }()
	}
	cleanupDone := make(chan struct{})
	go func() {
		defer close(cleanupDone)
		dbEngine.RunCleanup(ctx)
	}()
	// defer 后注册先执行：先 cancel+等待清理，再执行前面注册的 Store.Close。
	// 生命周期由启动者收束，不能把“发出了取消”当成“后台已经退出”。
	defer func() {
		cancel()
		<-cleanupDone
	}()
	rp := resp.NewRespParser()

	handler := resp.NewRespHandler(rp, dbEngine)

	// Serve 结束前等待停机完成；其后的 defer 才会关闭清理工作与 AOF。
	server := tcp.NewServer(addr, handler)
	watchStop, watchDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-ctx.Done():
			shutdownCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
			defer stop()
			_ = server.Shutdown(shutdownCtx)
		case <-watchStop:
		}
	}()
	fmt.Println("开始监听....")
	err = server.Serve(listener)
	close(watchStop)
	<-watchDone
	return err
}
