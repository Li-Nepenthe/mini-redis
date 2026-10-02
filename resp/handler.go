package resp

import (
	"context"
	"io"
	"log"
	"net"
	"sync"
)

type CommandExecutor interface {
	// Exec 处理包含命令名的二进制参数，返回编码器支持的结果或业务错误；调用期间输入不并发修改。
	Exec(args [][]byte) (any, error)
}
type ProtocolParser interface {
	// ParseStream 生产完整请求/错误并自行关闭通道；ctx 取消后所有者仍须关闭阻塞 reader 并等待结束。
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

// NewRespHandler 将 p 与 exec 作为协议解析器和命令执行器注入，创建连接登记表、父 context 和完成通道，返回 Handler。
// 不启动网络或解析 goroutine；依赖必须有效且满足取消/通道关闭契约，调用者最后须 Shutdown/Close 回收 Handler 拥有的连接。
func NewRespHandler(p ProtocolParser, exec CommandExecutor) *RespHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &RespHandler{parser: p, executor: exec, ctx: ctx, cancel: cancel,
		conns: make(map[net.Conn]*connectionState), done: make(chan struct{})}
}

// Handle 接管 conn，启动按连接的 Parser，顺序执行每条非空请求并完整写回编码响应，没有返回值。
// 协议错误只发固定错误行并断连；业务错误可继续同连接；编码/写失败结束该连接。
// 用同一 mutex 登记任务与停机门，标记 active 的命令才允许在停机时排空；预读到的新命令不会越过停机门。
// 退出时 cancel 解除 channel 发送阻塞、Close conn 解除 Read，再排空等待 Parser 关闭通道并 Done；不能仅发取消就宣称回收完成。
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

// writeReply 循环向 conn 写完 reply 全部字节，成功返回 nil；遇写错误原样返回，零字节无错误视为 io.ErrShortWrite。
// 处理网络短写，不关闭连接或编码回复；调用者负责连接生命周期与必要写截止时间。
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

// connectionsForStop 在 Handler.mu 下关闭新请求/连接登记门，并按 active 状态返回 idle、active 两份连接快照。
// 快照用于锁外关闭或设 deadline，本函数不执行网络 I/O；active 的含义覆盖当前执行和写回复，而非连接是否曾收过数据。
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

// waitDone 只启动一次等待 goroutine，在已登记 Handle 全部 Done 后关闭 h.done，并返回只读完成通道。
// 调用前必须已通过 connectionsForStop 关闭登记门，保证 Wait 后没有新的 Add；它本身不取消工作或关闭连接。
func (h *RespHandler) waitDone() <-chan struct{} {
	// closed 与 wg.Add 同锁，开始 Wait 后不会再加入工作。
	h.wait.Do(func() { go func() { h.wg.Wait(); close(h.done) }() })
	return h.done
}

// Shutdown 停止接收新命令，取消 Parser，关闭 idle 连接并允许 active 当前回复排空；ctx 的期限用于其写 deadline。
// 任务全部结束返回 nil；ctx 取消/超时则关闭所有连接并返回 ctx.Err，不冒充执行器已停止。
// context 和连接 Close 不能强行中断任意 Exec/fsync；上层如需最终回收仍须等待或调用 Close。
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

// Close 通过 Once 强制停止新请求、取消 Parser、关闭全部连接，并等待已登记 Handle/Parser 回收，重复调用可安全共享结果。
// 当前实现忽略连接关闭错误并返回 nil；没有截止时间，若执行器永久阻塞也会一直等待，不能当成有界超时停机。
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
