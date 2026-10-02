# 第 5 章：过期语义、索引与内存回收

## 5.1 “看不见”与“已经释放”是两件事

期限 deadline 是“在哪个时间点开始不可见”。`SET k v; EXPIRE k 5` 在 t=0 设期限 t=5；t=4 GET 仍有值，t=5 GET 必须缺失，即使抽样 worker 还没访问 k。惰性删除在访问时检查并删除，保证可见性；主动删除负责永不再读的冷 key，否则冷数据一直占引用。

TTL 的值：缺失 -2；存活无期限 -1；否则剩余秒按本项目四舍五入，余量>=500ms 进一。固定时钟 t=0 为5；t=0.4 剩4.6仍显示5；t=1.1 剩3.9显示4；t=4.8 仍存活却显示0；t=5缺失-2。是否过期按精确期限判断，不能 `TTL==0` 就提前删。

SET 覆盖时清旧 TTL。存活 List LPUSH/非末 LPOP 保留 TTL；最后节点弹出删除 key/TTL。过期 String 后 LPUSH 创建新 List，不能因旧类型残留返回 WRONGTYPE。EXPIRE 缺失返回0；零/负秒直接删除。源码：[ttl.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go)。

## 5.2 数据结构为什么不直接随机遍历 map

每分片 `expires[key]={deadline,index}`，`expiring` 是仅含有 TTL key 的 slice。slice 随机索引 O(1)，无需为抽样扫描全 map。维护不变量：两边长度相同；slice[i]=key，则 expires[key].index=i；同 key 不重复入池。

小例子：expiring=[a,b,c]，index 分别0/1/2；删 b，拿末项 c 填1，清空旧末项字符串，再缩短到[a,c]；c.index 改1，删除 b 的 map 元素。这是 swap remove，**不保持顺序**，但抽样没有顺序需求，删除 O(1)。若漏改 c.index，下次删 c 就越界/删错位置；若不清旧末项，底层数组仍可能保留已删 key 引用。

`setExpiration` 在续期时只改 deadline；新 TTL 才 append。`clearExpiration` 只删期限/索引，不删 data；`purgeExpired` 在写锁内删 data 并调用 clear。三者都假定调用者已持写锁，不自行加锁避免重入。

## 5.3 lockRead 为什么有循环和重查

先拿 RLock，看期限：存活就返回 shard、now，**锁仍在手上**，GET/LRANGE/TTL 最后释放。过期时不能持 RLock 直接 Lock；先放读锁，再拿写锁，按新时间重新 purge，释放后重试。

原因是间隙内其他写者可能续期：A 读到过期旧期限→放锁；B 把期限延到未来→解锁；A 若用旧判断直接删，就删掉合法的新值。`purgeExpired(key,e.now())` 在写锁下重查真正当前状态，再回读路径。此处循环既处理删除，也处理续期竞争。

## 5.4 worker 的预算与资源所有权

`RunCleanup(ctx)` 是阻塞循环，启动者显式 `go` 调用且最后等待。一个 Engine 只有一个 worker。ticker 每250ms触发 `cleanupExpired`：逐分片锁住，每分片最多256次随机抽样（可以重复），然后放锁，不保证单轮删光。

朴素全表每tick扫描会长持锁；每key定时器则增加大量定时管理/闭包/取消负担，还要解决 SET/续期与旧timer竞态。当前双策略用有限主动工作换不严格的物理删除时刻，同时访问路径仍保证精确可见性。样本未删完不意味着“过期值还能GET到”。

累计主动实际删除32768 key 后，且距上次回收完成至少5秒，锁外调用 `debug.FreeOSMemory`；lastReclaim 初值为零，因此第一次达到阈值可立即回收，不要求启动满五秒。它会请求 GC/归还内存，有全局成本；不用每tick强制GC。退出取消也不能中断已经在执行的同步 GC。

## 5.5 真实故障案例：HeapAlloc 降了，工作集没有降

四层需要分开：

1. 删除 map/链接引用：业务对象不再可达，但这一步不等于内存马上回收。
2. GC 确认不可达对象：`HeapAlloc` 等活堆指标可下降；map/slice 容量可能仍保留。
3. Go 运行时将空闲页归还 OS：不一定因无后续分配马上发生。
4. OS 进程工作集：当前驻留物理页，还包含栈、代码及其他内存；Peak 是历史最大值不会因现在下降而下降。

旧实现已主动删完 key，普通显式 GC 测试通过，真实空闲进程工作集约85.8MB在30秒仍不降。修复把“大批实际过期后锁外限频归还”纳入 worker；后来独立学习实验100000 key/256字节/5秒、不AOF、不GET、不外部GC：写耗3.539秒，工作集88,051,712→28,491,776（15秒）→28,475,392（30秒）。这是这台Windows的样本，不是每台机器必须完全相同的数字，也不保证回到启动值。

若写入超过5秒，早期 key 可能写完前已过期，必须记录写入时间；不要重启进程“制造回落”。绝对期限使用当前墙上时钟，系统时间调整会影响判断，不声称具有单调时钟免疫。

## 5.6 可复制检查与结果定位

在根模块：

```powershell
go test ./database -run='TestTTLSemantics|TestExpiredKeysAreMissingForEveryCommand|TestListKeepsTTLAndExpiryIndexIsConsistent|TestExpireArgumentErrors' -count=1 -v
go test ./database -run='TestCleanupCancellation|TestLargeExpirationReclaimsOutsideShardLocks|TestExpireHundredThousandKeysWithoutReads' -count=1 -timeout=60s -v
```

第一组精确固定时钟，期望所有TTL/命令/索引子用例PASS；第二组包括取消、锁外reclaim与真实10万key清空。后者比较**显式GC后的HeapAlloc**，不会替代OS采样；`-short`会跳过十万key验收，不能宣称跑过。完整 Windows 采样脚本在[主指南第7章](https://github.com/Li-Nepenthe/mini-redis/blob/main/docs/learning-guide.md)，需要Python且只清理自己启动的无AOF实验进程。

失败排查按层走：GET还看见过期→检查deadline/lockRead；抽样池长度不等→检查swap/remove；key删完活堆未降→检查引用/GC；活堆降但工作集未降→检查页归还/当前指标，不能把Peak当当前。学习答案Q09–Q13。下一章考虑重启后期限仍正确的历史问题。
