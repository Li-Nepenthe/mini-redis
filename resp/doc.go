// 本文件：说明 resp 如何连接协议与命令执行，并给出解析、回复和连接处理的阅读入口。

// Package resp 在网络字节和命令参数之间转换，按 RESP 格式把执行结果写回客户端。
// RESP 用类型标记和字节长度区分回复，因此值里含换行或零字节也不会和下一条回复混在一起。
//
// 从 handler.go 的 Handle 看一条请求怎样经过解析、执行和回复，再读 parser.go 与 reply.go。
// 连接关闭还要等解析工作结束，相关时序由 handler_test.go 和 shutdown_test.go 保护。
// 文件分工如下：
//   - doc.go：本包导航。
//   - parser.go：按长度读取请求，限制帧大小，区分完整请求和残缺输入。
//   - reply.go：执行结果与 RESP 类型的对应关系，以及公开错误的边界。
//   - handler.go：连接上的命令循环和停机处理。
//   - parser_test.go：正常请求、多条请求和分段输入。
//   - safety_test.go：长度上限、取消、换行格式和不完整输入。
//   - record_size_test.go：记录实际消耗的字节数，供 AOF 定位截断位置。
//   - fuzz_test.go：用变化的字节输入检查解析器。
//   - reply_test.go：回复类型的字节编码和不支持类型的拒绝。
//   - command_reply_test.go：状态、数组、二进制和错误信息的编码。
//   - handler_test.go：连接处理失败后的资源回收。
//   - shutdown_test.go：停机超时后仍能等待执行器结束。
package resp
