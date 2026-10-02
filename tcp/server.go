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

func NewServer(addr string, handler Handler) *Server {
	return &Server{Addr: addr, Handler: handler, shutdownDone: make(chan struct{})}
}

// Handler 拥有已接收的连接。Shutdown 排空当前命令，Close 则强制打断连接并等待回收。
type Handler interface {
	Handle(conn net.Conn)
	Shutdown(ctx context.Context) error
	Close() error
}

func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.Addr, err)
	}
	return s.Serve(listener)
}

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
