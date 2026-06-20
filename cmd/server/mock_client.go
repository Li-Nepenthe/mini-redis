package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io" // 🫵 记得导入 io 包
	"net"
	"os"
	"strconv"
	"strings"
)

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

		// 根据 RESP 协议的第一个字符，进行外科手术式精准打印！
		switch replyLine[0] {
		case '+': // 状态回复
			fmt.Printf("Status: %s\n", string(replyLine[1:]))
		case '-': // 错误回复
			fmt.Printf("(error) %s\n", string(replyLine[1:]))
		case '$': // 块字符串回复
			length, _ := strconv.Atoi(string(replyLine[1:]))
			if length == -1 {
				fmt.Println("(nil)")
			} else {
				// 💡 【核心修复】：准备一个容纳“内容 + \r\n”的钢铁容器
				contentBuf := make([]byte, length+2)

				// 🫵 【钢铁防线】：丢掉软弱的 Read，改用 io.ReadFull 强制灌满容器，一片残余都不准留下！
				_, err = io.ReadFull(netReader, contentBuf)
				if err != nil {
					fmt.Println("网络流读取残缺，强制中断:", err)
					break
				}

				// 剥离掉最后的 \r\n，打印纯净的肉数据
				fmt.Printf("\"%s\"\n", string(contentBuf[:length]))
			}
		default:
			fmt.Printf("Raw Reply: %s\n", string(replyLine))
		}
	}
}
