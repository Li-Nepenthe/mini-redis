package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

// main 启动教学交互客户端，连接固定的 127.0.0.1:6379，将标准输入按空白拆词并编码为 RESP 请求。
// 逐次发送后显示简单状态/错误/bulk 或首行原始回复，exit 或输入/网络错误结束并关闭连接；错误打印到终端，没有返回值。
// 它不支持带空格/引号的完整参数解析，也不消费完整数组回复，不应作为 List、流水线或兼容性验收工具。
func main() {
	conn, err := net.Dial("tcp", "127.0.0.1:6379")
	if err != nil {
		fmt.Println("拨号失败:", err)
		return
	}
	defer conn.Close()
	fmt.Println("=== 交互式 Mini-Redis 客户端已启动 (输入 exit 退出) ===")

	osReader := bufio.NewReader(os.Stdin) //读取键盘的输入
	netReader := bufio.NewReader(conn)    // 读取连接传来的输入

	for {
		fmt.Print("mini-redis> ")

		text, err := osReader.ReadString('\n')
		if err != nil {
			break
		}

		text = strings.TrimSpace(text)
		if text == "exit" {
			break
		}
		if text == "" {
			continue
		}

		parts := strings.Fields(text)
		length := len(parts)

		var sb strings.Builder
		sb.WriteString("*" + strconv.Itoa(length) + "\r\n")
		for _, part := range parts {
			sb.WriteString("$" + strconv.Itoa(len(part)) + "\r\n")
			sb.WriteString(part + "\r\n")
		}

		payload := []byte(sb.String())
		_, err = conn.Write(payload)
		if err != nil {
			fmt.Println("发送失败:", err)
			break
		}

		// 读出服务端回包的第一行（以 \n 结尾）
		replyLine, err := netReader.ReadBytes('\n')
		if err != nil {
			fmt.Println("读取服务端响应失败，可能连接已被断开:", err)
			break
		}

		replyLine = bytes.TrimSuffix(replyLine, []byte("\r\n"))
		if len(replyLine) == 0 {
			continue
		}

		// 该学习客户端只显示简单回复，不作为完整数组/命令兼容性验收工具。
		switch replyLine[0] {
		case '+': // 状态回复
			fmt.Printf("状态：%s\n", string(replyLine[1:]))
		case '-': // 错误回复
			fmt.Printf("（错误）%s\n", string(replyLine[1:]))
		case '$': // 块字符串回复
			length, _ := strconv.Atoi(string(replyLine[1:]))
			if length == -1 {
				fmt.Println("（空值）")
			} else {
				// bulk 以字节长度分帧，内容中的换行不能作为结束标记。
				contentBuf := make([]byte, length+2)

				// TCP 一次 Read 不保证读满，ReadFull 才能取完这一帧。
				_, err = io.ReadFull(netReader, contentBuf)
				if err != nil {
					fmt.Println("网络流读取残缺，强制中断:", err)
					break
				}

				fmt.Printf("\"%s\"\n", string(contentBuf[:length]))
			}
		default:
			fmt.Printf("原始响应：%s\n", string(replyLine))
		}
	}
}
