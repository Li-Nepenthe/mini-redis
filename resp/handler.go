package resp

import (
	"fmt"
	"net"
)

type RespHandler struct {
}

func (resp *RespHandler) Handle(conn net.Conn) {
	// 关闭连接
	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			fmt.Println(err)
		}
	}(conn)

}

func (resp *RespHandler) Close() error {
	return nil
}
