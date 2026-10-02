// Package main 是 MiniRedis 服务进程入口，负责参数、系统信号以及 TCP/RESP/Engine/AOF 的依赖组装。
// 依赖项目四个核心包和 Go 标准库；先监听再恢复，恢复成功绑定日志后才启动清理和连接处理。
// 入口拥有后台 worker、观察 goroutine 和日志生命周期，网络结束后先等待工作退出再关闭 AOF。
// 只启动第一阶段命令服务，默认 16 分片和 always-fsync；不启动第二阶段 HTTP/AI 后端。
package main
