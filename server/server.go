// 本文件：Server 接管监听器和连接，Serve 接收请求，Shutdown 排空当前回复，Close 强制关闭并等待回收。

package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
)

type Executor interface {
	Exec(args [][]byte) (any, error)
}

type Server struct {
	addr     string
	executor Executor
	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]bool // true 表示正在执行命令或写回复，停机等它写完；false 表示空闲，停机直接关闭。
	stopping bool
	wg       sync.WaitGroup
	shutdown sync.Once
	done     chan struct{}
	stopErr  error
	closeErr error
}

func NewServer(addr string, executor Executor) *Server {
	return &Server{addr: addr, executor: executor, conns: make(map[net.Conn]bool), done: make(chan struct{})}
}

func (s *Server) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.addr, err)
	}
	return s.Serve(listener)
}

// Serve 取得监听器所有权；停机关闭监听后，仍须等排空结果，不能抢先关闭正在发送的回复。
func (s *Server) Serve(listener net.Listener) (serveErr error) {
	s.mu.Lock()
	if s.stopping || s.listener != nil {
		stopping := s.stopping
		s.mu.Unlock()
		_ = listener.Close()
		if stopping {
			return nil
		}
		return errors.New("server is already serving")
	}
	s.listener = listener
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		stopping := s.stopping
		s.mu.Unlock()
		if stopping {
			<-s.done
			serveErr = errors.Join(serveErr, s.stopErr)
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
			// 没有可靠重试条件时直接返回，避免在永久错误上忙循环。
			return fmt.Errorf("accept: %w", err)
		}
		if !s.startConnection(conn) {
			return nil
		}
	}
}

// 登记连接与关闭入口共用一把锁，保证停机开始等待后不会再增加任务。
func (s *Server) startConnection(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping {
		_ = conn.Close()
		return false
	}
	s.conns[conn] = false
	s.wg.Add(1)
	go s.handle(conn)
	return true
}

// 先封住新连接和新命令，再取快照。网络操作放到锁外，避免它们拖住处理循环退出。
func (s *Server) stopConnections() (net.Listener, map[net.Conn]bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopping = true
	conns := make(map[net.Conn]bool, len(s.conns))
	for conn, active := range s.conns {
		conns[conn] = active
	}
	return s.listener, conns
}

// Shutdown 立即关闭空闲连接，等执行中的连接写完回复；ctx 结束时强制关闭剩余连接。
// 第一次调用确定排空期限和结果。超时只表示等待结束，不能冒充任意 Exec 或 fsync 已经停止。
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdown.Do(func() {
		listener, conns := s.stopConnections()
		s.closeErr = closeListener(listener)
		s.stopErr = s.closeErr
		for conn, active := range conns {
			if !active {
				_ = conn.Close()
			}
		}
		finished := make(chan struct{})
		go func() { s.wg.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-ctx.Done():
			for conn := range conns {
				_ = conn.Close()
			}
			s.stopErr = errors.Join(s.stopErr, ctx.Err())
		}
		close(s.done)
	})
	return s.stopErr
}

// Close 强制打断所有连接的 I/O，再等实际工作退出；若执行器仍阻塞，这里也必须继续等待。
// 等待完成后只报告资源关闭错误，过去的排空超时仍由 Shutdown 保留。
func (s *Server) Close() error {
	_, conns := s.stopConnections()
	for conn := range conns {
		_ = conn.Close()
	}
	_ = s.Shutdown(context.Background())
	s.wg.Wait()
	return s.closeErr
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
