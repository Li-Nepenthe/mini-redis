package resp

import (
	"bufio"
	"fmt"
	"io"
)

type Payload struct {
	Data [][]byte //解析出的数据
	Err  error    //向上传递底层的网络或者解析错误
}

// chan *Payload 双向通道  既能读又能写
// <-chan *Payload 只读通道 箭头从chan射出 表示数据只流出
// chan <- 只写通道 表示数据只流入

func ParseStream(reader io.Reader) <-chan *Payload {
	channel := make(chan *Payload)
	// 当前的reader底层是net.Conn（TCP的网络套接字） 如果调用read.Read(buf)时 Go会向操作系统申请系统调用
	// 这会使操作系统从用户态转为内核态并从网卡缓冲区搬运数据然后再切换为用户态 即会发生目态 管态 目态的频繁切换 每次变态开销很大

	// 而bufReader是一个具体的结构体， bufio.NewReader(reader)在用户态下的内存中创建了一个bufio.Reader对象
	// 内部大小是一个默认大小为4096字节的切片作为缓冲区 并持有传入的io.Reader

	// 本质上就是bufio在用户态的缓冲区 利用reader一次性尽可能多的读取网络传输的数据 从而为了避免只读取几个字节还得变态访问
	bufReader := bufio.NewReader(reader)
	go func() {
		// 退出匿名函数的时关闭chan
		defer close(channel)
		for {
			// bufio会不断从底层读取数据直到遇见指定的delim界定符
			msg, err := bufReader.ReadBytes('\n')
			// 如果在找到delim之前 触碰到文件末尾 会返回io.EOF错误
			// 但是无论网络波动还是合法的EOF退出  当前这连接的解析流水线就必须宣告终止
			if err != nil {
				//将错误通过chan传递给业务层
				channel <- &Payload{
					Err: err,
				}
				// 跳出循环 执行close
				break
			}
			length := len(msg)
			//如果当前的msg长度大于2 且最后两个为界定符 则剔除
			if length >= 2 && msg[length-2] == '\r' && msg[length-1] == '\n' {
				//剔除最后两个字节
				msg = msg[:length-2]
			}
			fmt.Println("截断后的有效数据为:", msg)
		}
	}()
	return channel
}
