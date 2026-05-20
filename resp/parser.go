package resp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
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

	// Conn实现了 Read(p []byte) (n int, err error) 方法 所以满足了io.Reader的接口要求

	// 当前的reader底层是net.Conn（TCP的网络套接字） 如果调用reader.Read(buf) (即conn.Read(buf))时 Go会向操作系统申请系统调用
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
			// 如果在找到delim之前 触碰到文件末尾 会返回io.EOF错误
			// 但是无论网络波动还是合法的EOF退出  当前这连接的解析流水线就必须宣告终止
			// bufReader的缺点就是只能通过识别指定的结束符来完成读取的结束 这就带来了麻烦即如果用的传过来的内容如果包含结束符的话就会意外结束识别 造成严重问题
			// 解决方法就是 第一个传来的是resp协议根据数据内容得到的协议数据 这个是不包含用户内容的 我们可以利用bufReader读取这一个数据 得到整体的数据信息
			// 然后再利用io.ReadFull读取传来的数据
			line, err := bufReader.ReadBytes('\n')

			// bufReader.ReadBytes只读取第一个\n
			// 比如要传输hello world 通过resp协议转换为*2\r\n$5hello\r\n$5world\r\n
			// 首先通过ReadBytes读取到了*2\r\n 此时line的内容即为*2\r\n
			// 然后我们对line的内容进行分析 首先判断开头是* 则紧接着的数据就是需要创建的字节数组的大小 bytes := make([][]byte,2)
			// 接着就是利用io.ReadFull读取bufReader中剩余的元素 利用$的数字大小确定传输的内容

			if err != nil {
				//将错误通过chan传递给业务层
				channel <- &Payload{
					Err: err,
				}
				// 跳出循环 执行close
				break
			}

			/**
			Redis 的网络协议叫做 RESP（Redis Serialization Protocol，Redis序列化协议）
			redis规定第一个字符只能为以下五种
				+	简单字符串信息 比如服务器回复 +OK\r\n
				-	错误信息 比如服务器报错 -ERR Unknown Command\r\n
				:	整数 比如回复长度或计数 :1000\r\n
				$	块字符串(Bulk Strings) 二进制安全的字符串，后面会紧跟长度（如 $5\r\nhello\r\n）
				*	数组
			对于Redis  客户端发来的任何命令本质是都是一个字符串数组
			如 SET name Gemini --> *3\r\n$3\r\nSET\r\n$4\r\nname\r\n$6\r\nGemini\r\n

			第一步：看到*3\r\n 知道传过来的是一个数组 长度为3 在堆内存中分配空间 args := make([][]byte, 3) 启动接下来启动一个执行 3 次的 for 循环，去装这 3 个参数
			第二步：看到$3\r\n 知道传过来的是一个块字符串 且长度为3 于是arg[0] = SET
			第三步，第四步以此类推 arg[1] =name arg[2] = Gemini
			*/

			// 去除掉后缀的\r\n
			line = bytes.TrimSuffix(line, []byte("\r\n"))

			if len(line) == 0 {
				// 如果数据为空 则继续接受数据
				continue
			}
			if line[0] == '*' {
				// 提取出后面的长度
				length, _ := strconv.Atoi(string(line[1:]))
				msgBytes := make([][]byte, length)
				fmt.Println(msgBytes)
			}
		}
	}()
	return channel
}
