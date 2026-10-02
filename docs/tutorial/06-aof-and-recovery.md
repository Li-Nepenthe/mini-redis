# 第 6 章：日志确认与历史时间

## 6.1 从“重启丢失”引出日志接口

内存 map 随进程结束消失。朴素每次保存整张表成本随总数据增长；只追加**改变状态的命令**，启动顺序回放，则每次写只处理本条记录。这里 AOF（Append Only File）是命令日志，不是事务/WAL 的完整工业实现。

先定义 `database.CommandLog.Append(args) error`：nil 必须含同步确认。数据库不导入 aof，而由 cmd/server 注入 Store，因此测试可换 failingLog，协议不需要知道文件在哪里。模拟搭建时先拆 prepare/apply 再接日志，是为了给失败留明确边界；不是伪造历史说源仓库曾按这个顺序提交。

源码：[persistence.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence.go)、[aof/store.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store.go)。

## 6.2 五个失败位置逐个推导

当前策略为 `校验/准备（持锁）→Append→完整Write→Sync→apply→回复`。Sync 请求操作系统把文件同步，普通 Write 成功只是把字节交给文件层，不能直接当持久化承诺。

| 时刻 | 可见状态与正确解释 |
|---|---|
| 参数/类型错 | 不写日志，不提交新值；准备可能清已过期旧值 |
| Write 半写/Sync 失败 | 不 apply；Store 尝试回滚确认前缀并保持失败，客户端公开存储错误 |
| Sync 成功，apply 前崩溃 | 该记录会恢复，客户端可能未收到成功 |
| apply 后，回复未完整到达 | 记录/内存已提交，但客户端仍不确定，不能当“请求绝未发生” |
| 回复确认成功后强杀 | 按当前同步/存储条件，已确认记录应恢复；测试验证此场景 |

先改内存再写日志会让失败的新值被读到，恢复却没有；先记无效记录会让启动 Replay 拒绝；只锁文件不锁完整提交序列会使回放与内存最终顺序不同。代价是 logMu 串行写和 fsync 期间占分片锁。周期 fsync 可以增吞吐，但改变丢失窗口/缓冲/关闭承诺，本项目只实现 always-fsync，没有“每秒最多丢一秒”的策略。

## 6.3 Store 的函数各负责哪一层

1. `Open(ctx,path,replay)` 检查回调、创建/打开普通文件、系统独占锁，用 Parser 按顺序调用 replay；成功定位到最后完整偏移并交出 Store。
2. Linux `lockFile` 用非阻塞 flock，Windows用LockFileEx，锁随句柄/进程释放；另一个进程失败，不等它改完再偷偷共享。
3. `encodeRequest` 只编码/检查记录预算，不判断业务命令。编码失败不标 Store 存储失败，不写文件。
4. `Append` 在 Store.mu 下拒关闭/已有失败，循环完整Write，Sync成功才增加size；size 是上次确认前缀，不是 file当前可见长度。
5. `fail(cause)` 尝试Truncate(size)+Sync，合并原因、记内部日志并持续拒写；即使回滚成功，也不冒充故障消失。
6. `Close` 与Append同锁，标关闭，必要时Sync并Close，保留旧错误；重复Close幂等，关闭后Append优先返回os.ErrClosed。

Open 的 replay 回调可已修改临时 Engine 前缀，后面失败不会回滚引擎；启动者必须丢弃这个半恢复对象。错误时会取消/关reader并等待Parser，避免恢复失败也留后台。AttachLog只能在完整恢复后、并发业务开始前调用；运行时替换日志不安全。

## 6.4 半尾恢复为何用实际 BytesRead

假设完整前缀含 `*03`、`$03`、`$01`，解析合法但比标准 `*3`/`$3`/`$1` 长。重新编码会变短；bufio可能预读后面的残尾，文件游标又会变长。两个都不是已确认前缀偏移。

当前只在完整记录 Replay 成功后累加 Payload.BytesRead；不完整尾包错误包裹EOF/UnexpectedEOF时截到这个offset并Sync。完整非法结束符或完整未知恢复命令报错，保留原字节，不删整个后缀，否则可能静默丢确认记录。没有自动备份、Rewrite或旧格式迁移。

## 6.5 两个真实 TTL 缺陷与一条新建边界

记录EXPIRE相对秒会在重启重新加五秒，延长寿命，所以写成私有 `__EXPIREATMS key 绝对毫秒期限`。但**绝对期限并不自动解决历史回放的状态依赖**。

案例 A：t=0 LPUSH k a、期限5；t=1 LPUSH k b（当时未过期）；t=6重启。错误回放按“现在6>旧期限5”先删，再LPUSH b创建无TTL新List，b复活。正确完整回放保留历史期限，最后k整体过期不可见。

案例 B：t=0 SET k v、期限5；t=4续期10秒，最终期限14；t=6重启。若回放期限5就删String，后续期限14没有值可续，丢了本应存活v/TTL8。正确Replay先还原全部历史，再普通访问或worker按当前时间判断最终期限。

又有案例 C：期限5的旧List在t=6真实LPUSH新值，此时应该**新建**而不混旧元素/旧TTL。主动过期不写DEL，单纯“回放期间不删”又会把新值放进旧List。因此正常LPUSH发现缺失/过期key时持久化 `_LNEW`；Replay该标记清旧类型/TTL并创建新List。普通LPUSH用于存活List保TTL。

`_LNEW` 与 LPUSH 都5字节且参数不变，避免合法32MiB边界请求因内部标记变长后无法落盘。Exec拒这两种私有命令，只有Replay接受。旧草稿缺新建边界时，日志信息不足，不能从最终记录反推全部历史；不能宣称所有旧歧义都无损自动恢复。

## 6.6 实验 A：真实临时文件恢复半尾

专用 lab main.go：

```go
// lab-file: main.go
package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "github.com/Li-Nepenthe/mini-redis/aof"
    "github.com/Li-Nepenthe/mini-redis/database"
)

func main() {
    dir, err := os.MkdirTemp("", "mini-redis-aof-lab-")
    if err != nil { panic(err) }
    defer os.Remove(dir)
    path := filepath.Join(dir, "study.aof")
    defer os.Remove(path)
    e := database.NewEngine(16)
    log, err := aof.Open(context.Background(), path, e.Replay)
    if err != nil { panic(err) }
    e.AttachLog(log)
    if _, err = e.Exec([][]byte{[]byte("SET"),[]byte("k"),[]byte("v")}); err != nil { panic(err) }
    if err = log.Close(); err != nil { panic(err) }
    tail := []byte("*3\r\n$3\r\nSET\r\n$4\r\ntail\r\n$5\r\nabc")
    f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
    if err != nil { panic(err) }
    if _, err = f.Write(tail); err != nil { panic(err) }
    if err = f.Close(); err != nil { panic(err) }
    restored := database.NewEngine(16)
    recovered, err := aof.Open(context.Background(), path, restored.Replay)
    if err != nil { panic(err) }
    defer recovered.Close()
    value, err := restored.Exec([][]byte{[]byte("GET"), []byte("k")})
    if err != nil { panic(err) }
    fmt.Printf("restored=%q\n", value.([]byte))
    fmt.Printf("recovered-tail-matches=%v\n", recovered.RecoveredTailBytes == int64(len(tail)))
}
```

```powershell
gofmt -w main.go
go run .
```

精确预期 `restored="v"`、`recovered-tail-matches=true`。只操作os.MkdirTemp生成的自己文件；不截原仓库data/appendonly.aof。文件与空目录按defer顺序清理，不递归删别人的目录。若独占锁错误，检查是否提前Close；若完整SET丢失，检查记录前缀；若尾不恢复，检查尾是否其实是完整坏帧，当前必须拒绝这种“修复”。

根模块故障/历史/强杀验证：

```powershell
go test ./database -run='TestPersistenceFailureDoesNotMutateMemory|TestInvalidAndNoOpWritesDoNotAppend|TestReplayPreservesHistoricalExpiryAndCreation|TestLegacyReplayKeepsExpiryUntilFinalState|TestReplayExpirationUsesAbsoluteDeadline' -count=1 -v
go test ./aof -run='TestRecoverOnlyIncompleteTailWithActualOffsets|TestRejectCorruptOrInvalidCompleteRecordsWithoutTruncating|TestWriteAndSyncFailuresAreSticky|TestAOFExclusiveLock|TestExpiredListWithLaterPushDoesNotResurrectOnReopen|TestMaximumSizedLPUSHFitsPersistedCreationRecord|TestThousandWritesSurviveForcedProcessTermination' -count=1 -timeout=60s -v
```

预期所有列出的测试PASS；强杀测试自己的子进程READY后恢复1000条，普通运行的CrashProcessHelper会Skip，不是失败。最大帧测试用faultFile而非真实大文件；TTL固定时钟精确比较；半尾与独占锁用真实文件。故障注入不能冒充硬件断电测试。答案Q14–Q19；下一章处理“谁最终关这些资源”。
