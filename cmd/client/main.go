// 本文件：main 选择服务器地址，runClient 顺序发送命令并显示完整回复；输入按空白拆分，不支持带空格的值或引号转义。

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/Li-Nepenthe/mini-redis/resp"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:6379", "Mini-Redis server address")
	flag.Parse()
	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		fmt.Println("拨号失败:", err)
		return
	}
	defer conn.Close()
	fmt.Println("=== 交互式 Mini-Redis 客户端已启动 (输入 exit 退出) ===")
	if err := runClient(conn, os.Stdin, os.Stdout); err != nil {
		fmt.Println("客户端结束:", err)
	}
}

// 每次发送后必须消费完整回复再接收下一条输入，否则数组元素会被误当成下一条命令的结果。
func runClient(conn net.Conn, input io.Reader, output io.Writer) error {
	stdin, replies := bufio.NewReader(input), bufio.NewReader(conn)
	for {
		fmt.Fprint(output, "mini-redis> ")
		line, err := stdin.ReadString('\n')
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		if len(parts) == 1 && parts[0] == "exit" {
			return nil
		}
		args := make([][]byte, len(parts))
		for i, part := range parts {
			args[i] = []byte(part)
		}
		request, err := resp.AppendRequest(nil, args)
		if err != nil {
			fmt.Fprintln(output, "请求无效:", err)
			continue
		}
		if _, err := conn.Write(request); err != nil {
			return fmt.Errorf("发送请求: %w", err)
		}
		reply, err := readReply(replies)
		if err != nil {
			return fmt.Errorf("读取回复: %w", err)
		}
		fmt.Fprintln(output, reply)
	}
}
