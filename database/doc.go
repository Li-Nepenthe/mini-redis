// Package database 实现 MiniRedis 的 String/List 命令、分片内存状态、TTL 和日志确认前的写准备。
// 依赖 Go 标准库；通过 CommandLog 抽象使用日志，不依赖 resp、tcp 或 aof，避免协议/存储循环依赖。
// Engine 管理 shard 锁和字节所有权；List 自身无锁，map/TTL 索引只能在所属分片锁下访问。
// AOF 写顺序为 logMu→有序分片锁→日志 Append，确认后才 apply；恢复由启动者顺序调用 Replay。
// 本包不监听网络、不编码 RESP、不自动启动清理 worker，也不提供完整 Redis 数据类型或事务功能。
package database
