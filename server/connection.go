// 本文件：handle 在一个连接内同步读取、执行并回复；writeReply 保证短写时仍写完全部字节。

package server

import (
	"bufio"
	"io"
	"log"
	"net"

	"github.com/Li-Nepenthe/mini-redis/resp"
)

func (s *Server) handle(conn net.Conn) {
	defer func() {
		_ = conn.Close()
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		s.wg.Done()
	}()
	reader := bufio.NewReader(conn)
	for {
		args, _, readErr := resp.ReadRequest(reader)
		if readErr == io.EOF {
			return
		}
		if readErr == nil && len(args) == 0 {
			continue
		}
		// 与停机共用这道门：预读到请求不等于已经开始执行。
		s.mu.Lock()
		if s.stopping {
			s.mu.Unlock()
			return
		}
		s.conns[conn] = true
		s.mu.Unlock()
		if readErr != nil {
			// 不把原始输入和内部网络错误放进回复，避免多行注入或泄露。
			_ = writeReply(conn, []byte("-ERR Protocol error\r\n"))
			return
		}
		result, execErr := s.executor.Exec(args)
		reply, err := resp.EncodeReply(result, execErr)
		if err != nil {
			log.Printf("encode reply: %v", err)
			return
		}
		if err := writeReply(conn, reply); err != nil {
			return
		}
		s.mu.Lock()
		s.conns[conn] = false
		stopping := s.stopping
		s.mu.Unlock()
		if stopping {
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
