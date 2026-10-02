package tcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

type Server struct {
	Addr         string
	Handler      Handler
	mu           sync.Mutex
	listener     net.Listener
	closed       bool
	draining     bool
	wg           sync.WaitGroup
	close        sync.Once
	closeErr     error
	shutdown     sync.Once
	shutdownDone chan struct{}
	shutdownErr  error
}

// NewServer 保存 addr 与 handler，并创建停机完成通道，返回尚未监听的 Server。
// 不创建连接或 goroutine；handler 必须有效并遵守连接所有权、Shutdown/Close 契约，调用者随后选择 ListenAndServe 或 Serve。
func NewServer(addr string, handler Handler) *Server {
	return &Server{Addr: addr, Handler: handler, shutdownDone: make(chan struct{})}
}

// Handler 拥有已接收的连接。Shutdown 排空当前命令，Close 则强制打断连接并等待回收。
type Handler interface {
	// Handle 接管 conn 并在返回前回收连接/解析工作；Server 负责等待该调用结束。
	Handle(conn net.Conn)
	// Shutdown 用 ctx 排空已经开始的回复并拒新命令，失败必须如实返回而非声称全部结束。
	Shutdown(ctx context.Context) error
	// Close 强制关闭连接并等待全部处理工作；未要求实现必须能中断任意业务计算。
	Close() error
}

// ListenAndServe 在 Server.Addr 上创建 TCP listener，再将其所有权交给 Serve；返回带地址的监听错误或服务结果。
// 监听失败直接返回，不在 nil listener 上执行 Close；已经取得的 listener 由 Serve/Server 停机路径回收。
func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.Addr, err)
	}
	return s.Serve(listener)
}

// Serve 接管非 nil listener 并循环 Accept，为每个连接启动 Handler goroutine，返回接受/关闭/停机的合并错误。
// 已关闭或重复 Serve 会关闭传入 listener；永久 Accept 错误返回，net.ErrClosed 视为正常停止，不对弃用 Temporary 忙循环重试。
// closed 与 wg.Add 同锁，防止 Wait 后新增任务；排空时等待 shutdownDone，不能 listener 一关就抢先强制 Close 当前回复。
func (s *Server) Serve(listener net.Listener) (serveErr error) {
	s.mu.Lock()
	if s.closed || s.listener != nil {
		closed := s.closed
		s.mu.Unlock()
		_ = listener.Close()
		if closed {
			return nil
		}
		return errors.New("server is already serving")
	}
	s.listener = listener
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		draining := s.draining
		s.mu.Unlock()
		if draining {
			// listener 提前关闭不代表排空完成，Serve 必须等停机结果，不能抢先强制 Close。
			<-s.shutdownDone
			serveErr = errors.Join(serveErr, s.shutdownErr)
		} else {
			serveErr = errors.Join(serveErr, s.Close())
		}
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			// Temporary 已弃用；没有可靠重试条件时返回，避免永久错误上的忙循环。
			return fmt.Errorf("accept: %w", err)
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return nil
		}
		// 与 closed 同锁登记，避免 Shutdown 开始 Wait 后还加入新连接。
		s.wg.Add(1)
		s.mu.Unlock()
		go func() { defer s.wg.Done(); s.Handler.Handle(conn) }()
	}
}

// closeListener 关闭 listener，nil 或 net.ErrClosed 视为成功，其他错误原样返回。
// 它只处理监听资源，不关闭已接收连接、不等待 Handler；用于合并停机错误并避免重复关闭造成虚假失败。
func closeListener(listener net.Listener) error {
	if listener == nil {
		return nil
	}
	err := listener.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// Shutdown 通过 Once 标记 closed/draining，关闭 listener，调用 Handler.Shutdown(ctx)，返回保存的合并停机错误。
// Handler 成功时等待 Server 的连接 goroutine 全部结束，再关闭 shutdownDone；失败时保留错误而不宣称已全部回收。
// 重复调用共享第一次的结果和 ctx，不能靠第二次调用延长原有排空期限；任意执行器阻塞仍由上层处理。
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdown.Do(func() {
		s.mu.Lock()
		s.closed, s.draining = true, true
		listener := s.listener
		s.mu.Unlock()
		s.shutdownErr = closeListener(listener)
		s.shutdownErr = errors.Join(s.shutdownErr, s.Handler.Shutdown(ctx))
		if s.shutdownErr == nil {
			s.wg.Wait()
		}
		close(s.shutdownDone)
	})
	return s.shutdownErr
}

// Close 通过 Once 标记停止、关闭 listener 并强制 Handler.Close，等待全部连接 goroutine，返回保存的合并错误。
// 并发/重复调用不会重复执行关闭过程；无超时预算，Handler/执行器若无法返回，本函数也不能保证有界完成。
func (s *Server) Close() error {
	s.close.Do(func() {
		s.mu.Lock()
		s.closed = true
		listener := s.listener
		s.mu.Unlock()
		s.closeErr = errors.Join(closeListener(listener), s.Handler.Close())
		s.wg.Wait()
	})
	return s.closeErr
}
