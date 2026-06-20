package main

import (
	"Mini-Redis/database"
	"Mini-Redis/resp"
	"Mini-Redis/tcp"
	"fmt"
)

func main() {
	fmt.Println("Mini-Redis Server 启动中....")

	// 根据业务注入 业务引擎和解析引擎
	dbEngine := database.NewRespEngine()
	rp := resp.NewRespParser()

	// 将业务引擎和解析引擎注入到handler中 形成独一无二的handler
	handler := resp.NewRespHandler(rp, dbEngine)

	// 将handler注入到server中 处理具体的server
	server := tcp.NewServer(":6379", handler)
	fmt.Println("开始监听....")
	server.ListenAndServe()
}
