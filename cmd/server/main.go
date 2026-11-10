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

// 先占端口再恢复日志，避免重复启动时触碰正在服务的 AOF。
// 启动者拥有清理循环与信号观察工作；网络停止后先等后台退出，再关闭日志，防止最后的写入落到已关闭的文件。
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
	// defer 后注册先执行：先 cancel+等待清理，再执行前面注册的 Store.Close。
	// 生命周期由启动者收束，不能把“发出了取消”当成“后台已经退出”。
	defer func() {
		cancel()
		<-cleanupDone
	}()
	// Serve 结束前等待停机完成；其后的 defer 才会关闭清理工作与 AOF。
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
