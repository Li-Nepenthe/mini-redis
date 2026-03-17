package resp

import (
	"bufio"
	"fmt"
	"io"
	"net"
)

type RespHandler struct {
}

func (resp *RespHandler) Handle(conn net.Conn) {
	// 关闭连接
	defer conn.Close() // 无需处理 close会发生的错误
	// 拿到TCP-Server传过来的conn后 分析里面的数据

	reader := bufio.NewReader(conn)
	// buf := make([]byte, 1024) conn.Read(buf)
	//
	for {
		msg, err := reader.ReadString('\n')
		if err == io.EOF {
			fmt.Println("客户端关闭连接")
			return
		} else if err != nil {
			fmt.Println("发生错误：", err)
			return
		}

		fmt.Printf("内容为：%q\n", msg)
	}
}

func (resp *RespHandler) Close() error {
	return nil
}
