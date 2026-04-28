package main

import (
	"Mini-Redis/resp"
	"Mini-Redis/tcp"
	"fmt"
)

func main() {
	fmt.Println("Mini-Redis Server 启动中....")
	handler := &resp.RespHandler{}
	server := tcp.NewServer(":6379", handler)
	fmt.Println("开始监听....")
	server.ListenAndServe()
	fmt.Println("这是笔记本的更新")
}
