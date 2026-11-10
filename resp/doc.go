// 本文件：说明本包职责、阅读入口和各文件分工。

// Package resp 负责RESP 请求和回复的字节转换。
// 从 parser.go 的 ReadRequest 读起，再看 request.go 的请求编码和 reply.go 的回复编码。连接与停机由 server 包负责。
//
// 文件分工如下：
//   - doc.go：本包导航。
//   - parser.go：同步读取一条请求，保留上限、CRLF 和错误分类。
//   - request.go：AOF 与客户端共用的请求编码。
//   - reply.go：执行结果的 RESP 编码和公开错误边界。
//   - parser_test.go：合法、非法、粘连和分段输入。
//   - safety_test.go：头部与帧预算、残帧及关闭读取端。
//   - record_size_test.go：实际消费字节数。
//   - request_test.go：请求编码、前缀和预算。
//   - reply_test.go：回复类型与字节表示。
//   - command_reply_test.go：引擎结果的协议编码。
//   - fuzz_test.go：FuzzReadRequest 的结构和边界检查。
package resp
