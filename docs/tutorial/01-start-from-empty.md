# 第 1 章：从空目录到一条可核对的结果

每次只做下面一个步骤，完成该步的命令和解释再继续。这是学习实验，不覆盖真实仓库文件。先读第 0 章术语；不需要 MySQL、API key 或 P2。

## 1.1 环境与空工程

在 PowerShell 执行 `go version`、`git --version`。根 `go.mod` 要求 Go 1.26，本次验证用 Go 1.27.1；自己的机器应使用满足要求的已安装版本。后续 race 在 Windows 需要可用 C 工具链；普通测试先不依赖 race。`redis-cli` 是官方客户端，可选，教程核心 Go 实验不靠它。

```powershell
$labDir = Join-Path $env:TEMP ('mini-redis-lab-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $labDir | Out-Null
Set-Location $labDir
go mod init example.com/miniredis-lab
```

预期当前新目录含 `go.mod`，模块名是学习命名空间，不是生产模块。若 go 不识别，先检查安装路径/PATH；若目录已存在，重取新名字，不在原仓库重复 go mod init。

把下面**完整**程序保存到 `$labDir/main.go`。这一步只观察“字节参数→值→回复”的三次转换，不实现网络/TTL/AOF。

```go
// lab-file: main.go
package main

import "fmt"

func main() {
    args := [][]byte{[]byte("SET"), []byte("k"), []byte("v")}
    data := make(map[string][]byte)
    data[string(args[1])] = append([]byte(nil), args[2]...)
    args[2][0] = 'X'
    value := data["k"]
    fmt.Printf("stored=%q\n", value)
    fmt.Printf("set-reply=%q\n", []byte("+OK\r\n"))
    fmt.Printf("get-reply=%q\n", []byte(fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)))
}
```

```powershell
gofmt -w main.go
go run .
```

精确预期（引号中的反斜线是显示出来的转义形式）：

```text
stored="v"
set-reply="+OK\r\n"
get-reply="$1\r\nv\r\n"
```

如果 stored 是 X，说明保存的是别名而非字节副本；如果 GET 长度错，先数**字节**而非屏幕字符。当前程序没有检查 args 数量、并发、网络和磁盘；它的用途是隔离所有权/结果类型，不把它称作完整数据库。

## 1.2 下一步把职责拆开，而不是先堆功能

把刚才的三行职责对应到最终文件：参数拆解由 `resp/parser.go`；存储写入由 `database/persistence.go`；结果类型和编码由 `resp/reply.go`。朴素方案把网络、map、字符串拼接放在一个 main，会让“坏协议”和“错类型”互相混杂，难以注入失败，也难证明锁在何时释放。

模拟搭建顺序如下。每行说清该步新增职责、检查和下一步理由；第 2–8 章给局部代码/函数协作与失败分析。不是复制全部最终源码到空目录后假称自己从零完成。

| 步骤 | 新增文件/契约（最终位置） | 能运行的检查（在真实仓库根） | 为什么接着做下一步 |
|---|---|---|---|
| 1 | main.go 的参数/map/回复小模型 | 本节 go run，三行精确输出 | 手写参数还不是客户端输入 |
| 2 | resp/parser.go 的 ParseStream/readRequest | `go test ./resp -run='TestParser$\|TestParserFragmentedInput\|TestParserMultipleCommands' -count=1 -v` | 参数变好了，结果还需统一协议 |
| 3 | resp/reply.go 的 EncodeReply | `go test ./resp -run='TestEncodeReply\|TestCommandReplyEncoding' -count=1 -v` | 回复可测后才加入业务类型 |
| 4 | database/engine.go、errors.go 的 Exec/List | `go test ./database -run='TestCommand\|TestListRangeBoundaries\|TestEngineOwns' -count=1 -v` | 多连接共享状态需要锁 |
| 5 | Engine 分片/lockKeys；Handler/TCP 接口 | `go test ./database ./tcp -run='ConcurrentList\|TwentyCommands\|MalformedClients' -count=1 -v` | 网络可用后处理寿命和冷数据 |
| 6 | database/ttl.go 的期限/索引/worker | `go test ./database -run='TestTTLSemantics\|TestListKeepsTTL\|TestCleanupCancellation' -count=1 -v` | 内存完成还不能跨重启 |
| 7 | CommandLog、prepare/apply、aof.Store | `go test ./aof ./database -run='Replay\|PersistenceFailure\|IncompleteTail\|Sticky' -count=1 -v` | 文件与 worker 需要完整退出 |
| 8 | cmd/server.run、Shutdown/Close | `go test ./resp ./tcp ./cmd/server -run='Shutdown\|ConcurrentClose' -count=1 -v` | 正常成功不足，还要证据和讲解 |
| 9 | fuzz/benchmark/CI/说明文档 | 第 8 章检查与结果解释 | 回答第 9 章，查出理解差距 |

表中 `\|` 在 Markdown 内用来防止表格分列，实际 PowerShell 参数应写普通 `|`；更方便的原样可复制命令在各章独立代码块中。运行测试前 `Set-Location 'D:\Code\GO项目\mini-redis'`（其他机器改为自己的仓库路径）。若出现 `no tests to run`，不能记为通过，核对正则与目录。

## 1.3 不修改源码，先走通最终系统

在仓库根创建仅学习使用的 AOF 路径；两个终端分别做服务和官方客户端。与已有服务分开用 6380，端口冲突时选择另一个空闲端口，不结束别人的进程。

```powershell
$studyDir = Join-Path $env:TEMP ('mini-redis-study-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $studyDir | Out-Null
$studyAof = Join-Path $studyDir 'study.aof'
go run ./cmd/server -addr=127.0.0.1:6380 -aof=$studyAof
```

保留终端打印的专用路径，稍后恢复必须用**同一个文件**。另一个终端：

```powershell
redis-cli -p 6380 SET study:name Alice
redis-cli -p 6380 GET study:name
redis-cli -p 6380 LPUSH study:jobs a b c
redis-cli -p 6380 LRANGE study:jobs 0 -1
redis-cli -p 6380 LPOP study:jobs
redis-cli -p 6380 LPOP study:name
```

预期依次 OK、Alice、3、`[c,b,a]`、c、WRONGTYPE。具体界面引号/编号由客户端展示，准确线路字节在第 2 章。若连接拒绝，查服务是否启动/端口是否一致；若“wrong number”，核对命令参数；若数组错乱，确认用官方 redis-cli，简易 `cmd/client` 不完整消费数组。

Ctrl+C 停学习服务后原命令重启同一路径，GET 仍为 Alice，List 为 `[b,a]`。若改了文件路径或工作目录导致打开另一文件，这不是持久化丢失，先核对实际参数。没有客户端时可运行下一章完整 Go 小实验和仓库 TCP 测试，别把客户端未安装记成服务测试失败。

这里演示常规重启；真实强杀、半尾和同步故障由第 6 章测试执行，不对真实数据手工截断。下一章开始把 SET 的每个字节与函数交接对应起来。
