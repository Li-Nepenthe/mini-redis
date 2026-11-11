// 本文件：说明本包职责、阅读入口和各文件分工。

// Package server 负责监听、连接处理和停机。
// 从 server.go 的 Serve 进入连接循环，再读 connection.go 的 handle；Shutdown 与命令开始共用同一道入口检查。
//
// 文件分工如下：
//   - doc.go：本包导航。
//   - server.go：Server、Executor、监听和停机。
//   - connection.go：每个连接同步读取、执行并完整回复。
//   - server_test.go：监听失败、异常连接隔离和连接回收。
//   - connection_test.go：回复失败、协议错误与停机后的连接拒绝。
//   - shutdown_test.go：排空当前回复并拒绝预读流水线。
//   - timeout_test.go：超时与执行器真正退出的区别。
//   - commands_test.go：同一连接上的连续命令。
//   - ttl_test.go：过期命令的 TCP 回复。
//   - golden_test.go：固定完整会话字节，读取 testdata/session.golden。
package server
