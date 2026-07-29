package resp

import (
	"fmt"
	"io"
	"net"
)

// 业务多态

type CommandExecutor interface {
	Exec(args [][]byte) (any, error)
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
			_, err := conn.Write([]byte("-ERR " + payload.Err.Error() + "\r\n"))
			if err != nil {
				return
			}
			return
		}

		if len(payload.Data) == 0 {
			continue
		}

		// 接engine解析过后传来的数据
		result, execErr := resp.executor.Exec(payload.Data)
		// 将传来的信息和错误放入标准解码器进行解构处理
		reply, encodeErr := EncodeReply(result, execErr)

		// 如果错误 则输出错误信息
		if encodeErr != nil {
			fmt.Println("编码响应失败", encodeErr)
			return
		}
		// 没有错误，reply写回客户端
		if _, writeErr := conn.Write(reply); writeErr != nil {
			fmt.Println("写回客户端失败，原因：", writeErr)
			return
		}
	}
}

func (resp *RespHandler) Close() error {
	return nil
}
