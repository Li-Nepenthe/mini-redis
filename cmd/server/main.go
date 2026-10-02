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
