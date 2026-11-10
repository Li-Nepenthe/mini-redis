// 本文件：说明 database 的职责与阅读入口，并列出命令、持久化和过期处理的文件分工。

// Package database 把命令作用到内存中的字符串和列表，并处理 key 的过期时间。
// 网络层只需调用 Engine.Exec；引擎通过 CommandLog 请求日志确认，不依赖具体文件或协议实现。
//
// 写入最重要的边界是日志确认：先算好 writePlan，日志落盘后才修改内存。
// 计划里的回复、记录和内存变更一起阅读，才能看出写失败时为何仍保留旧值。
//
// 从 engine.go 的 Exec 读起，再沿写命令进入 persistence.go 的 executeWrite。
// 文件分工如下：
//   - doc.go：本包导航。
//   - engine.go：命令入口、分片路由、有序加锁、列表结构和读命令。
//   - persistence.go：写入计划、日志确认和历史记录重放。
//   - ttl.go：过期时间、惰性删除和后台抽样清理。
//   - errors.go：区分客户端可见的错误和内部原因。
//   - engine_test.go：命令语义、列表边界、字节所有权和并发测试。
//   - persistence_test.go：日志失败、无效果写入和历史重放测试。
//   - golden_test.go：固定 AOF 记录序列。
//   - legacy_aof_test.go：固定旧 AOF 文件的读取和恢复结果。
//   - ttl_test.go：固定时钟、TTL 语义和过期清理测试。
//   - cleanup_test.go：检查内存回收发生在分片锁外。
//   - benchmark_test.go：不同分片数下的读写与热点基准。
package database
