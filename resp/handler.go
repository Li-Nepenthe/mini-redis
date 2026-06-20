package resp

import (
	"fmt"
	"io"
	"net"
)

// 业务多态

type CommandExecutor interface {
	Exec(args [][]byte) []byte
}

// 协议多态

type ProtocolParser interface {
	ParseStream(reader io.Reader) <-chan *Payload
}

type RespHandler struct {
	parser   ProtocolParser
	executor CommandExecutor
}

func NewRespHandler(p ProtocolParser, exec CommandExecutor) *RespHandler {
	return &RespHandler{
		parser:   p,
		executor: exec,
	}
}

func (resp *RespHandler) Handle(conn net.Conn) {
	// 保证连接在退出时被关闭
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(conn)

	// 拿到只读管道Payload
	ch := resp.parser.ParseStream(conn)
	// 不断从管道中读取数据
	for payload := range ch {
		// 先检查底层有没有报错
		if payload.Err != nil {
			// 如果中途解析报错（比如客户端断开或传了乱码），打印并回送错误，结束循环
			fmt.Println("解析发生错误:", payload.Err)
			conn.Write([]byte("-ERR " + payload.Err.Error() + "\r\n"))
			return
		}

		if len(payload.Data) == 0 {
			continue
		}
		replyBytes := resp.executor.Exec(payload.Data)
		//将响应结果写回
		conn.Write(replyBytes)
	}
}

func (resp *RespHandler) Close() error {
	return nil
}
