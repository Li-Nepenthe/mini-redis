// Package resp 在 TCP 字节流与命令接口之间解析受预算限制的数组/bulk 请求并编码 RESP 回复。
// 生产代码仅依赖 Go 标准库，通过 ProtocolParser/CommandExecutor 连接协议与业务；测试可注入故障实现。
// Parser 每次调用拥有输出 goroutine/channel；Handler 拥有连接、取消及等待流程，生产者负责关闭通道。
// 业务结果/公开错误与内部原因分离；二进制长度分帧、完整写回和停机门共同保持请求/响应边界。
// 本包不持有数据库 map 或 AOF，不实现 inline 请求或全部 RESP 类型，取消不能自行打断任意 Reader。
package resp
