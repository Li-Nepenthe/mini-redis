package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
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

	// 1. 包装操作系统的标准输入（你的键盘）
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("mini-redis> ")

		// 2. 阻塞等待，直到你在键盘按下 Enter 键
		text, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		// 3. 剔除末尾多余的换行符（抹平 Mac/Linux/Windows 的差异）
		text = strings.TrimSpace(text)

		if text == "exit" {
			break
		}
		if text == "" {
			continue
		}

		// 4. 🔪 核心协议构建：Redis 协议的硬性规定！
		// RESP 协议规定，不管是内联命令还是数组命令，必须以 \r\n (CRLF) 结尾！
		// 思考：你该如何把你输入的 text 变成符合规范的 payload？
		payload := []byte(text + "\r\n")
		_, err = conn.Write(payload)
		if err != nil {
			fmt.Println("发送失败:", err)
			break
		}

		// 5. 接收服务端的响应 (如果你现在没写服务端回写逻辑，这步会卡住)
		// 为了让你能一直发，我们暂时不在这里阻塞 Read，或者只打印发送成功。
		// fmt.Println("-> 已发送")
	}
}
