# 第 3 章：值、命令与函数如何配合

## 3.1 先用一个 key 讲清状态

`SET k v` 使 `data["k"]` 成为 String `[]byte("v")`。`GET k` 取副本。`SET k x` 覆盖原类型和值并清 TTL，包括把 List 覆盖成 String；`LPUSH k a` 对仍存活的 String 则拒绝 WRONGTYPE，不能偷偷转类型。

List 的 head 是左端、tail 是右端。空 List 第一次 push a：head/tail 都指向 a，len=1。再 push b：b.next=a、a.prev=b、head=b、tail=a，len=2。pop b：head 回到 a、a.prev=nil、len=1；pop a 后首尾 nil。命令层同时删 key 与 TTL，避免空链表和旧过期元数据悬挂。

朴素 slice 也能实现 List：尾端 append 简单，但头插可能移动已有数据或重新分配。当前选双向链表练首尾链接和 O(1) 头插/弹出；代价是节点分配/指针间接，LRANGE 仍需要走链表。不是“链表比所有数组都快”的结论。

## 3.2 按调用层次读核心函数

源码：[engine.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go)、[persistence.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence.go)、[errors.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/errors.go)。

1. `NewEngine(16)` 建分片、map、默认时钟与回收回调；没有 worker/文件/网络。分片如何路由下一章解释。
2. `Exec(args)` 只归一化命令名，值的大小写不改。空/未知命令返回稳定业务错误。读分到 get/exists/lrange/ttl，写分到 executeWrite。
3. `executeWrite` 可先持日志顺序锁，然后 `prepareWrite` 检查数量/整数、取得有序分片写锁、按需清理过期旧值、检查类型。
4. `prepareWrite` 返回“五件东西”：业务结果 result、将写日志的 record、未来修改 apply、释放锁 unlock、错误 err。没有日志时仍用相同路径，但省 Append；有日志时成功 Append/Sync 后才 apply。
5. 即使 prepare 报错也可能已持锁，调用者必须执行非 nil unlock。apply 引用同一 List，不允许解锁后再执行。
6. `get` 通过 lockRead 得到仍持读锁的分片，检查 String，拷贝后返回；`lrange` 同样持读锁遍历并拷贝每个元素。`exists` 使用写锁，因为检查可能清理过期 key。
7. `NewLinkedList/Len/LPush/LPop` 是数据结构层，不自己加锁或理解日志。Engine 的 LPUSH/LPOP 外部方法只是统一写路径入口。
8. `StatusReply.RESPStatus` 告诉编码器状态是什么；错误的 Error/Unwrap/RESPError 则分别处理显示、原因链、公开协议。

### SET 的五返回，用小值推导

初始 k 不存在，args=[SET,k,v]：`result=true`，`record=[SET,k,v]`，apply 捕获 v 的副本并会清 TTL/设置 data，unlock 捕获已获取分片，err=nil。prepare 返回时**尚未提交新业务值**。日志错误：执行 unlock，不执行 apply，向客户端发 persistence write failed。成功：apply，再返回 true，EncodeReply 编为 +OK。

无效果 `DEL missing`：result=0、record/apply=nil，仍需 unlock，但不会为“没删任何东西”追加记录。`LPOP missing` 同样 nil result。数量错误通常在拿锁前返回；类型错误发生在锁内，不能只检查 err 而漏 unlock。准备阶段删除已过期旧值是合法清理副作用，并不等于这次新写成功。

### 读结果与所有权

如果 GET 直接交出存储 slice，调用者 `result[0]='X'` 会绕过锁改变数据库；如果 SET 直接保留输入 slice，客户端缓冲复用也能改旧值。因此 SET/LPUSH 入库复制，GET/LRANGE 出库复制。LPOP 返回从存储脱离的值，可以转移所有权。输入在**调用期间**不得并发修改；复制不能修复正在发生的竞争。

## 3.3 公开命令的完整小数据语义

| 命令 | 输入约束与结果 | 状态影响 |
|---|---|---|
| PING | 无参数 PONG；一个值返回 bulk；更多参数 ERR | 无 |
| SET | key/value 两参数；true→OK | 覆盖类型/值，清 TTL |
| GET | 一个 key；String 副本或 nil；List WRONGTYPE | 可能惰性过期删除 |
| DEL | 至少一个 key；实际删的不同 key 数 | 重复 key 只删一次，删 TTL |
| EXISTS | 至少一个 key；按每个参数计数 | `EXISTS k k` 若存在返回 2；可能过期删除 |
| LPUSH | key 与至少一个值；新长度 | `a b c` 得 `[c,b,a]`；存活 List 保 TTL，新建无旧 TTL |
| LPOP | 一个 key；头值或 nil；String WRONGTYPE | 弹最后节点删除 key/TTL |
| LRANGE | key/start/stop；闭区间数组 | 负数从尾算，裁剪，空区间/缺失为空数组 |
| EXPIRE | key/整数秒；存在 1、不存在 0 | 正数设期限，零/负数删 key |
| TTL | 一个 key；-2 缺失、-1 无 TTL、否则秒数 | 到期按缺失处理；本项目秒数取整见第 5 章 |

长度和业务类型是两层检查：Parser 不知道 SET 是三个参数；Engine 也不替直接调用者执行完整网络帧预算。自写调用时应保持参数契约。

LRANGE `[c,b,a]` 的例子：`0,0→[c]`；`1,2→[b,a]`；`-2,-1→[b,a]`；`-100,100→[c,b,a]`；`2,1→[]`。索引解析失败/溢出为 ERR invalid integer，不是悄悄当成零。

## 3.4 实验 E：把业务结果接回回复

在已连接本机模块的专用 lab，将 main.go 替换为下列完整程序：

```go
// lab-file: main.go
package main

import (
    "fmt"
    "strings"
    "github.com/Li-Nepenthe/mini-redis/database"
    "github.com/Li-Nepenthe/mini-redis/resp"
)

func main() {
    e := database.NewEngine(16)
    requests := [][]string{
        {"SET","k","v"}, {"GET","k"}, {"EXISTS","k","k","missing"},
        {"LPUSH","jobs","a","b","c"}, {"LRANGE","jobs","-2","-1"},
        {"LPOP","k"}, {"SET","k"}, {"DEL","k","k"}, {"GET","k"},
    }
    for _, words := range requests {
        args := make([][]byte, len(words))
        for i, word := range words { args[i] = []byte(word) }
        value, execErr := e.Exec(args)
        reply, encodeErr := resp.EncodeReply(value, execErr)
        fmt.Printf("%s -> %q encodeErr=%v\n", strings.Join(words," "), reply, encodeErr)
    }
}
```

```powershell
gofmt -w main.go
go run .
```

精确预期：

```text
SET k v -> "+OK\r\n" encodeErr=<nil>
GET k -> "$1\r\nv\r\n" encodeErr=<nil>
EXISTS k k missing -> ":2\r\n" encodeErr=<nil>
LPUSH jobs a b c -> ":3\r\n" encodeErr=<nil>
LRANGE jobs -2 -1 -> "*2\r\n$1\r\nb\r\n$1\r\na\r\n" encodeErr=<nil>
LPOP k -> "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n" encodeErr=<nil>
SET k -> "-ERR wrong number of arguments for 'set' command\r\n" encodeErr=<nil>
DEL k k -> ":1\r\n" encodeErr=<nil>
GET k -> "$-1\r\n" encodeErr=<nil>
```

失败定位：List 顺序反了，先画每次头插；EXISTS 与 DEL 都为 1/2，检查是否错用同一去重策略；encodeErr 非 nil，检查是不是把 []byte 改成普通 string；业务错误被当编码失败，回看两个 error 的职责。

根模块测试：

```powershell
go test ./database -run='TestCommand|TestListRangeBoundaries|TestEngineOwnsStoredAndReturnedBytes' -count=1 -v
go test ./tcp -run=TestTwentyCommandsOnOneConnection -count=1 -v
```

预期全 PASS。第一个验证业务/所有权，第二个接入真实网络在一条连接持续执行 20 条含错误命令。有限样本并不证明没有任何 bug；下一章才说明多连接共享这些状态的锁。答案 Q03、Q04、Q05、Q24、Q32。
