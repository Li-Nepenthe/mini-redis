package tcp

import (
	"fmt"
	"net"
	"time"
)

//func networkPrinciple() {
//	// 当我占用6739端口作为我的服务端口时
//
//	//1.像OS申请端口权限 此时os分配一个Socket并绑定在6379端口 且将该Socket设为Listen状态，此时开始留意来到6379的“客人”
//	listener, err := net.Listen("tcp", ":6379")
//	if err != nil {
//		fmt.Println("申请错误，原因为", err)
//	}
//	// 不断监听有无访问
//	for {
//		// 当有外部进程访问端口时，当操作系统底层完成三次握手时，利用accept将访问进程放进来
//		conn, err := listener.Accept()
//		if err != nil {
//			fmt.Println("连接失败，原因为", err)
//		}
//		// 拿到连接后，就可以与访客互相发送消息
//		_, err = conn.Write([]byte("Hello World"))
//		if err != nil {
//			return
//		}
//		// 构建一个字节数组接受传递过来的消息
//		buf := make([]byte, 1024)
//		_, _ = conn.Read(buf)
//		fmt.Println(string(buf))
//
//		// 关闭连接
//		err = conn.Close()
//		if err != nil {
//			return
//		}
//	}
//}

// 网络层：封装 net.Listen，处理 Accept 阻塞，并为新连接分发 Goroutine

// Server 一个Server必须包含两个，一个是监听的地址 一个是能处理的接口
type Server struct {
	Addr    string
	Handler Handler //此时是抽象的接口方法
}

// NewServer 工厂函数 传入具体的xxxHandler
func NewServer(addr string, handler Handler) *Server {
	return &Server{Addr: addr, Handler: handler}
}

type Handler interface {
	Handle(conn net.Conn)
	Close() error
}

func (s *Server) ListenAndServe() {
	// 创建监听器 监听服务端地址
	listen, _ := net.Listen("tcp", s.Addr)
	for {
		conn, err := listen.Accept() // 那些访问服务端口并建立连接的conn 会被放行
		//此时为accept出问题了 该层解决
		if err != nil {
			// 如果错误不影响整体以及其他请求 则继续 否则中断
			// 如网络错误，文件描述符用尽（这里被封装为临时网络错误）
			// 如果这里报错太频繁，可以休眠几毫秒，防止 CPU 100%
			if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
				// ok则代表确实是网络抛出的错误
				// nerr.Temporary() 会判断是否为暂时性错误并返回true
				time.Sleep(5 * time.Millisecond)
				continue
			}
			fmt.Println("Accept error:", err)
			// 其他连接正常进行
			continue
		}
		// 创建协程 把当前的conn传递给协程的hander处理
		go s.Handler.Handle(conn)
	}
}
