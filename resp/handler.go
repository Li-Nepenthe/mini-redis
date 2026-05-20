package resp

import (
	"fmt"
	"net"
)

type RespHandler struct {
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
	ch := ParseStream(conn)

	// 不断从管道中读取数据
	for payload := range ch {
		// 先检查底层有没有报错
		if payload.Err != nil {
			fmt.Println("检测到管道流出现错误 准备断开连接")
			return
		}
		if payload.Data != nil {
			fmt.Printf("[Handler 协程] 🎉 成功从管道中读取到数据！内容为: %s\n", string(payload.Data[0]))
		}
	}
}

func (resp *RespHandler) Close() error {
	return nil
}
