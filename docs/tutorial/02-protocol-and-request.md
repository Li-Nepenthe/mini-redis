# 第 2 章：字节流如何变成一条请求

## 2.1 把 SET k v 逐字节拆开

RESP 是客户端和服务端约定的编码。请求中的数组给参数个数，bulk 给每段**字节**长度。下面 `\r\n` 代表真正的回车/换行两个字节，不是输入四个反斜线字母。

```text
*3\r\n             3 个参数，4 bytes
$3\r\nSET\r\n      第一参数 SET，9 bytes
$1\r\nk\r\n        第二参数 k，7 bytes
$1\r\nv\r\n        第三参数 v，7 bytes
合计 27 bytes → [][]byte{SET,k,v}
```

随后 GET k 是 20 bytes。若客户端一次发送 47 bytes，不能把它当“一条 47 字节命令”；若 SET 被分三次收到，也不能第一段就报完整请求错误。这就是俗称粘包/半包的问题：TCP 是有序字节流，不为应用保留发送调用边界。

朴素 `Read(buf)` 后 `strings.Fields` 会错在三点：一读可能不满；多个请求可能一起到；value 可以含空格、零字节和 CRLF。`$4\r\na\r\nb\r\n` 的内容是四字节 `a CR LF b`，最后的 CRLF 才是帧分隔。字符“你”的 UTF-8 是三个字节，长度不能按屏幕字符数。

## 2.2 函数依赖与输入输出

源码：[resp/parser.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/parser.go)。按以下顺序理解：

1. `NewRespParser` 只建无状态对象，尚未读任何输入。
2. `ParseStream(ctx,reader)` 创建无缓冲通道与 goroutine，持一个 `bufio.Reader` 循环解析。缓冲器可预读并保存下一帧。
3. `readHeader` 读数组头或 bulk 头，以 CRLF 验证后返回借用缓冲切片；马上解析，不能跨下一次读保存它。
4. `readRequest` 校验 `*` 与非负个数，建参数 slice；逐段校验 `$` 与长度，**先算整帧预算，再 make 内容**，ReadFull 凑够内容+2，校验尾 CRLF。
5. 成功返回 Data 与 BytesRead；失败返回错误，ParseStream 发送一次后退出；干净 EOF 不发送错误。
6. Handler 接收 Payload。空数组在 Parser 层合法、Handler 忽略；`SET k` 是合法协议却不合法业务，由 Engine 返回参数错误。

| 边界 | 当前值/作用 | 少了会怎样 |
|---|---|---|
| 数组参数 | 1024 | `*1000000000` 可诱发巨大参数 slice |
| 单 bulk | 16MiB | 一个声明长度就诱发巨大分配 |
| 长度头 | 64 bytes，含 CRLF | 无换行头不断读取/扩容 |
| 整帧 | 32MiB，含所有头/分隔 | 1024×16MiB 每段合法仍可达约 16GiB |

`Atoi("-1")` 不一定返回解析错误，所以必须单独判负数；不能 make 后再检查。`ReadSlice` 固定缓冲在恶意长头时停下，可能预读一个 4096 字节缓冲，不代表底层一定恰好只读 64 bytes。这些是当前项目上限，协议中不存在“1024 就是所有服务器要求”的结论。

EOF 的区分：空输入在新帧开头是干净 EOF；`*1`、`*1\r\n`、bulk 缺内容是已开始帧的错误。`BytesRead` 只在完整成功帧有效，错误是 0；AOF 因此维护“最后成功记录偏移”，不能依错误帧的零长度推测其实际已读残字节。

## 2.3 实验 P：让每次 Read 最多只给三个字节

这是对真实 Parser 的局部实验，不实现整套服务。继续第 1 章的专用 lab 目录，先连接本机仓库模块（只读使用源码）：

```powershell
$repoDir = 'D:\Code\GO项目\mini-redis'
Set-Location $labDir
go mod edit -require=github.com/Li-Nepenthe/mini-redis@v0.0.0
go mod edit "-replace=github.com/Li-Nepenthe/mini-redis=$repoDir"
```

覆盖**学习目录**中的 main.go，实际仓库 main.go 不动：

```go
// lab-file: main.go
package main

import (
    "context"
    "fmt"
    "io"
    "strings"
    "github.com/Li-Nepenthe/mini-redis/resp"
)

type threeBytes struct { io.Reader }
func (r threeBytes) Read(p []byte) (int, error) {
    if len(p) > 3 { p = p[:3] }
    return r.Reader.Read(p)
}
func main() {
    wire := "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n" +
            "*2\r\n$3\r\nGET\r\n$1\r\nk\r\n"
    i := 0
    for p := range resp.NewRespParser().ParseStream(context.Background(), threeBytes{strings.NewReader(wire)}) {
        i++
        fmt.Printf("record=%d args=%q bytes=%d err=%v\n", i, p.Data, p.BytesRead, p.Err)
    }
}
```

```powershell
gofmt -w main.go
go run .
```

精确预期：

```text
record=1 args=["SET" "k" "v"] bytes=27 err=<nil>
record=2 args=["GET" "k"] bytes=20 err=<nil>
```

顺序/大小都保持。若只有一个记录，检查你是否真的拼了 GET；若 import 找不到，检查 replace 指向根 `go.mod` 所在路径，而非 ai-backend；若一直不结束，检查是否使用有结束的 strings.Reader 或是否忘关 Pipe 写端。

在仓库根运行边界测试：

```powershell
go test ./resp -run='TestParser$|TestParserFragmentedInput|TestParserMultipleCommands|TestParserHeaderBoundary|TestParserRequestByteBudget|TestParserIncompleteCommandIsNotCleanEOF|TestParserLimitsHeaderRead|TestParserRequiresCRLF' -count=1 -v
```

预期列出的所有测试/子用例 PASS：合法 GET/SET/二进制内容、三种残帧、负数/过大声明、严格 CRLF、64 bytes 恰好边界、整帧预算先于第二段读。失败时先分类头/长度/内容/EOF，不通过改宽所有上限把坏输入放行。

## 2.4 回复是另一个方向，不混用请求解析

[resp/reply.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/reply.go) 接收业务**值**，不是再次解析客户端参数。

| Go 结果 | 线路字节 | 原因 |
|---|---|---|
| true（SET） | `+OK\r\n` | 简单成功状态 |
| StatusReply("PONG") | `+PONG\r\n` | 显式状态接口，避免当 String |
| []byte("v") | `$1\r\nv\r\n` | 二进制长度，不按行结束 |
| nil | `$-1\r\n` | 缺失，与空 value 区别 |
| []byte{} | `$0\r\n\r\n` | 存在但空字符串 |
| [][]byte{} | `*0\r\n` | 空列表结果，与缺失 bulk 区别 |
| int/int64 | `:数字\r\n` | 数量/TTL，包括负值 |

`EncodeReply` 的第二返回 error 是“编码不了”，例如普通 string、false 或含换行状态；业务 `execErr` 则通常转换为错误**回复字节**并返回 nil 编码错误。这两层不能混为“遇错误一定断连”。`appendBulk` 追加长度/内容/CRLF，不解释值文字；`validErrorCode` 仅接受大写 ASCII 字母。

朴素 `err.Error()` 会泄露文件路径/底层信息，也可能带换行注入额外响应。当前只认实现 RESPError 的公开码/消息，仍清除 CRLF；内部错误统一 `ERR internal server error`，不抹掉服务端原因链。试验运行：

```powershell
go test ./resp -run='TestEncodeReply|TestCommandReplyEncoding|TestHandlerProtocolErrorIsSingleRESPLine' -count=1 -v
```

预期 PASS，包括 `WRONGTYPE`、包装公开错误不显示包装路径、公开消息换行变空格、非法码变通用 ERR。下一章让参数真正修改值，而不是 Parser 里直接操作 map。答案对应 Q01、Q02、Q05、Q24。
