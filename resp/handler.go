package resp

import (
	"context"
	"io"
	"log"
	"net"
	"sync"
)

type CommandExecutor interface {
	Exec(args [][]byte) (any, error)
}
type ProtocolParser interface {
	ParseStream(ctx context.Context, reader io.Reader) <-chan *Payload
}

type connectionState struct{ active bool }

type RespHandler struct {
	parser   ProtocolParser
	executor CommandExecutor
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	conns    map[net.Conn]*connectionState
	closed   bool
	wg       sync.WaitGroup
	close    sync.Once
	wait     sync.Once
	done     chan struct{}
}

func NewRespHandler(p ProtocolParser, exec CommandExecutor) *RespHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &RespHandler{parser: p, executor: exec, ctx: ctx, cancel: cancel,
		conns: make(map[net.Conn]*connectionState), done: make(chan struct{})}
}

func (h *RespHandler) Handle(conn net.Conn) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	state := &connectionState{}
	h.conns[conn] = state
	h.wg.Add(1)
	h.mu.Unlock()
	ctx, cancel := context.WithCancel(h.ctx)
	ch := h.parser.ParseStream(ctx, conn)
	defer func() {
		// 取消解除 channel 发送阻塞，关连接解除 Read 阻塞；等待 channel 关闭才算回收完成。
		cancel()
		_ = conn.Close()
		for range ch {
		}
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
		h.wg.Done()
	}()
	for payload := range ch {
		if ctx.Err() != nil {
			return
		}
		if payload.Err == nil && len(payload.Data) == 0 {
			continue
		}
		// 与停机使用同一把锁决定“已经开始”的边界，不执行停机后预读到的新请求。
		h.mu.Lock()
		if h.closed {
			h.mu.Unlock()
			return
		}
		state.active = true
		h.mu.Unlock()
		if payload.Err != nil {
			// 协议错误不包含原始输入或内部网络错误，防止泄露和 RESP 行注入。
			_ = writeReply(conn, []byte("-ERR Protocol error\r\n"))
			return
		}
		result, execErr := h.executor.Exec(payload.Data)
		reply, err := EncodeReply(result, execErr)
		if err != nil {
			log.Printf("encode reply: %v", err)
			return
		}
		if err := writeReply(conn, reply); err != nil {
			return
		}
		h.mu.Lock()
		state.active = false
		stopped := h.closed
		h.mu.Unlock()
		// 已执行的命令先把回复写完，再关闭连接；排空不意味着继续接收流水线请求。
		if stopped {
			return
		}
	}
}

func writeReply(conn net.Conn, reply []byte) error {
	for len(reply) > 0 {
		n, err := conn.Write(reply)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		reply = reply[n:]
	}
	return nil
}

func (h *RespHandler) connectionsForStop() (idle, active []net.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for conn, state := range h.conns {
		if state.active {
			active = append(active, conn)
		} else {
			idle = append(idle, conn)
		}
	}
	return
}

func (h *RespHandler) waitDone() <-chan struct{} {
	// closed 与 wg.Add 同锁，开始 Wait 后不会再加入工作。
	h.wait.Do(func() { go func() { h.wg.Wait(); close(h.done) }() })
	return h.done
}

func (h *RespHandler) Shutdown(ctx context.Context) error {
	idle, active := h.connectionsForStop()
	h.cancel()
	for _, conn := range idle {
		_ = conn.Close()
	}
	if deadline, ok := ctx.Deadline(); ok {
		for _, conn := range active {
			_ = conn.SetWriteDeadline(deadline)
		}
	}
	select {
	case <-h.waitDone():
		return nil
	case <-ctx.Done():
		// 截止时间只能打断连接 I/O，不能中断任意 Exec 或文件 fsync；把超时交给上层。
		idle, active = h.connectionsForStop()
		for _, conn := range append(idle, active...) {
			_ = conn.Close()
		}
		return ctx.Err()
	}
}

func (h *RespHandler) Close() error {
	h.close.Do(func() {
		idle, active := h.connectionsForStop()
		h.cancel()
		for _, conn := range append(idle, active...) {
			_ = conn.Close()
		}
		<-h.waitDone()
	})
	return nil
}
