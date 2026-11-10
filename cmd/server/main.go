// 本文件：main 解析启动参数，run 组装引擎、AOF 与 Server，并按顺序收束网络和后台清理。

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/Li-Nepenthe/mini-redis/aof"
	"github.com/Li-Nepenthe/mini-redis/database"
	"github.com/Li-Nepenthe/mini-redis/server"
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

// 先占端口再恢复日志，避免重复启动时触碰正在服务的 AOF。
// 启动者拥有清理循环与信号观察工作；网络停止后先等后台退出，再关闭日志，防止最后的写入落到已关闭的文件。
func run(ctx context.Context, addr, aofPath string) (runErr error) {
	if ctx.Err() != nil {
		return nil
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	fmt.Println("Mini-Redis 服务端启动中....")

	// 网络只依赖执行接口，恢复完成后才开始接收请求。
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
	// 先等后台真正退出，再关日志；只发出取消信号不足以完成回收。
	defer func() {
		cancel()
		<-cleanupDone
	}()
	// Serve 返回后仍等待信号观察工作退出，再回收后台与日志。
	network := server.NewServer(addr, dbEngine)
	watchStop, watchDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-ctx.Done():
			shutdownCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
			defer stop()
			_ = network.Shutdown(shutdownCtx)
		case <-watchStop:
		}
	}()
	fmt.Println("开始监听....")
	err = network.Serve(listener)
	close(watchStop)
	<-watchDone
	return err
}
