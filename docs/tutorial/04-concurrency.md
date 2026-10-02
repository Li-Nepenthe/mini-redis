# 第 4 章：多连接为什么不能直接共享 map

## 4.1 先从丢更新的时间线理解锁

两个连接都向同一 List 头插：A 读到旧 head=a，B 也读到 head=a；A 建 b→a，B 建 c→a；最后只留下 b 或 c，另一个节点不可达。即使 Go map 没报并发写错误，复合操作仍可能丢失。同一个命令内的“看类型/取节点/修改链接/改 len/设置 map”必须一起受保护。

最朴素全局 Mutex 简单正确，但 GET 一个 key 时也阻挡无关 key 的写。RWMutex 可以同锁多读，仍不能读与写同时进行。当前引擎把 key 路由到若干 shard，每个 shard 有 RWMutex、data、expires、expiring；不同分片可以独立锁，List 不再自己加另一套锁。

## 4.2 分片选择为什么依赖 2 的幂

[engine.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go) 的 `fnv32` 是 FNV-1（先乘再异或）字节散列，`getShard` 做 `hash & (N-1)`。比如 N=16、mask=15（低四位全为 1），示意 hash=18，18&15=2；这与 `%16` 一致。如果 N=10，mask=9（二进制1001），结果只可能为0/1/8/9，很多分片根本不被选中，因此 NewEngine 拒绝非 2 的幂。

这里 18 只是演算值，不声称某个真实 key 的 FNV 值就是 18。若用 `%N`，可支持任意 N，但当前协议/性能测试和配置不走那条设计。FNV 不是密码散列；不同 key 可碰撞到同分片，同一个 key 永远同一锁，增加分片不能把单热点拆开。

## 4.3 多 key 命令先排序，再去重的重要性

假设 a 在 shard2、b 在 shard9。若 `DEL a b` 先锁2再锁9，`DEL b a` 先锁9再锁2，两个请求各拿一锁再互等，叫死锁。当前 `lockKeys` 先收集所有分片编号、去重、升序获取，返回逆序 unlock 闭包。两条命令都先2后9，不能形成这个环。

去重也不能省：a、c 可能同在 shard2；同一 goroutine 对不可重入 RWMutex 再 Lock 会自己卡住。这里去重的是**锁**，业务计数仍由命令决定：EXISTS a a 计两次，DEL a a 删除一次。

`getShard` 只路由不加锁；`NewLinkedList/Len/LPush/LPop` 只管结构，不自己加锁。访问者必须知道锁责任：GET/LRANGE 使用 `lockRead` 取得 RLock 并在当前函数释放；EXISTS 虽是逻辑读，却因可能惰性清理而拿写锁；普通写使用 `prepareWrite→lockKeys`。

## 4.4 AOF 加入后，为什么有 logMu

无日志时不同分片可并发修改；有日志时所有写先获得 Engine.logMu，再拿有序分片锁，再调用 Store.Append（Store.mu）。若只让 Store 同步写，先写记录 A 的线程可能稍后才 apply，B 先 apply，则内存最终值与回放顺序不一致。logMu 把“日志确认+内存提交”作为同一写序列。

代价是真实 AOF 写被串行化，且同分片读在 fsync 期间也等待。不能拿未启 AOF 的 benchmark 成绩宣称持久化吞吐。缓存日志/提前放锁可提速，但需重设计确认与可见性；本项目不引入它们。

## 4.5 实验 C：结果数量稳定，顺序可以不稳定

专用 lab 目录 main.go：

```go
// lab-file: main.go
package main

import (
    "fmt"
    "sync"
    "github.com/Li-Nepenthe/mini-redis/database"
)

func main() {
    e := database.NewEngine(16)
    var wg sync.WaitGroup
    for worker := 0; worker < 2; worker++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for i := 0; i < 3; i++ {
                _, err := e.Exec([][]byte{[]byte("LPUSH"), []byte("jobs"), []byte(fmt.Sprintf("%d:%d", id, i))})
                if err != nil { panic(err) } // 实验固定合法输入，异常使实验明确失败。
            }
        }(worker)
    }
    wg.Wait()
    value, err := e.Exec([][]byte{[]byte("LRANGE"), []byte("jobs"), []byte("0"), []byte("-1")})
    if err != nil { panic(err) }
    fmt.Printf("elements=%d\n", len(value.([][]byte)))
}
```

```powershell
gofmt -w main.go
go run .
```

精确预期 `elements=6`。两个 worker 的跨线程相对顺序不能固定；不要把某一运行序列写成保证。每个 id 通过参数交给闭包，Add 在启动前，Done 在退出 defer，Wait 后再读完整结果。

仓库根检查：

```powershell
go test -race ./database -run='TestConcurrentListAndMultiKeyCommands|TestConcurrentExpirationAndCleanup|TestEngineOwnsStoredAndReturnedBytes' -count=1 -timeout=60s -v
go test ./aof -run=TestConcurrentWriteOrderMatchesReplay -count=1 -v
```

预期全部 PASS 且无 race 报告。race 只看本次实际执行路径，不证明永无死锁；超时检查和锁顺序推理都需要。若报 race，读报告中的两个访问栈，再找它们共享的 map/节点/输入 slice，确认保护的是同一把锁而非“每个函数都随便有锁”。

## 4.6 热点与伪共享，别混成一个原因

热点 key 让大部分请求落同一锁，64 分片的另外63把锁很闲；这是逻辑争用。缓存行伪共享是不同变量处在同一 CPU 缓存行，线程修改各自变量也导致缓存一致性流量；它是另一机制。当前 CPU profile 有锁/调度函数，但没有专门硬件证据定位伪共享，不应凭猜测加 padding 后声称解决。

mutex profile 看等待，CPU profile 看 CPU 样本，P99 看延迟分布；各自问题不同。完整取舍/依据在第 8 章与 Q06–Q08、Q27；下一章加入时间后，读操作也可能变成删除，锁规则还要继续成立。
