// Package tcp_test 从外部调用 tcp 公共 API，验证真实回环 TCP 下协议、命令、TTL、故障隔离和停机边界。
// 依赖 tcp、resp、database 及 Go 测试/网络标准库，不通过访问 Server 私有字段替代用户可见行为。
// 辅助 listener/Handler/Parser/执行器只控制有限故障与时序；连接、临时端口和 goroutine 由测试清理。
// 样本通过和 race 结果是已执行场景的证据，不证明任意负载的 P99 或任意阻塞执行器的有界退出。
package tcp_test
