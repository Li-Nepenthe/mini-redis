// Package tcp 管理监听器、接受连接、连接任务登记和服务停机；不解析命令或保存业务数据。
// 生产代码依赖 Go 标准库，通过 Handler 接口交接已接收连接；协议/数据库实现由启动程序注入。
// closed 与 WaitGroup.Add 在同锁下维护，停机关闭登记门再等待；Shutdown 排空，Close 强制关闭并等待。
// 监听器由 Server 接管，连接由 Handler 接管；网络期限不能终止任意业务执行器或文件同步。
package tcp
