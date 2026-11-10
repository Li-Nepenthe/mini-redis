// 本文件：说明本包职责、阅读入口和各文件分工。

// Package aof 负责持久化写入确认和启动恢复。
// 从 store.go 的 Open 看恢复，再看 Append 的确认位置；引擎只在 Append 成功后修改内存。文件锁防止两个进程同时恢复或追加同一文件。
//
// 文件分工如下：
//   - doc.go：本包导航。
//   - store.go：顺序恢复、半条尾部修复、追加确认和失败回滚。
//   - lock_unix.go：Linux 和 macOS 独占文件锁。
//   - lock_windows.go：Windows 独占文件锁。
//   - lock_other.go：其余平台明确拒绝启用 AOF。
//   - store_test.go：旧记录、恢复边界、文件故障、锁和崩溃重启。
package aof
