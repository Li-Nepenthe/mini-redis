# Mini-Redis 开发记录与学习备注

## 2026-10-02 · P1-S1 安全与正确性

### 基线与当前范围

- 现有工作区：`D:\Code\GO项目\mini-redis`，分支 `main`，基线 `6daf72512bd25a1a670d0f8957a1cc66cfecf992`。
- 开始时工作树干净，`main...origin/main` 的左右提交数为 `0 / 0`；没有覆盖已有修改。未进行 commit、push、merge 或部署。
- 仓库及其上级目录没有 `AGENTS.md`，仓库没有 `.agents/skills`。按《项目执行说明书-MiniRedis与AI后端_1.md》推进。
- §6 的阶段勾选全部为空，代码仍处于 S1；不能因为日历已到 S6 目标日期而跳过验收。
- 负长度、数组数量上限、单个 bulk 上限、Fuzz 骨架在基线已经存在，本轮仅保留并扩展其验证，未记为新实现。
- 本轮只实施一个任务：补齐 S1 的输入边界和连接生命周期。S2–S6、AI 后端和封版均未实施。

### 本轮任务（沿用说明书 §0.4）

| 项 | 内容 |
|---|---|
| 1. 项目与阶段 | P1 Mini-Redis / S1 安全与正确性 |
| 2. 级别 | 核心必做 |
| 3. 目标 | 畸形输入有界失败，监听失败可报告，连接结束后 Parser 与 Handler 都退出 |
| 4. 原因 | 单参数上限无法限制整条请求；Handler 提前返回后无人接收 channel；永久 Accept 错误不能无限重试 |
| 5. 前置知识 | context 是取消信号，不能自动中断普通 Reader；连接 Close 中断网络 I/O；WaitGroup 等待已登记的工作结束 |
| 6. 输入与输出 | 输入 RESP2 数组与 bulk 字节流；输出 Payload 或一条协议错误，错误连接关闭且其他连接继续工作 |
| 7. 接口与结构约束 | `ParseStream(ctx, reader)`；保留无缓冲 channel 和现有 Engine；Server/Handler 的 Close 可并发重复调用 |
| 8. 实施步骤 | 写限额回归测试 → 修 Parser → 接入 Handler 取消和资源回收 → 修监听/Accept → 执行验收 |
| 9. 必须测试 | 长协议头、总请求限额、残缺 EOF、取消、写失败、编码失败、端口占用、永久 Accept 错误、100 条 TCP RST、并发 Close |
| 10. 验收 | 下方命令及结果表；10 分钟 Fuzz、staticcheck 和 goroutine 回到基线属于 S1 必需项 |
| 11. 常见错误 | 只取消 context 不关 socket；只关 socket 不取消发送；先 Wait 后阻止 Add；向客户端回写原始错误或请求字节 |
| 12. 必须能回答 | 为什么先校验再分配；为何需要取消与关闭两条退出路径；为何不能重试所有 Accept 错误；为什么 1024×16 MiB 的单参数上限仍不够 |

### 改动内容与原因

| 文件 | 改动 | 原因 |
|---|---|---|
| `resp/RespParser.go` | context 贯穿解析与 channel 发送；有界协议头；累计请求预算；保留已有长度校验 | 消费者提前退出不能留下阻塞发送；输入长度必须在分配前受限 |
| `resp/handler.go` | 每连接 context；登记连接；取消、关连接、等 Parser channel 关闭；实现 Close；处理短写 | 取消释放 channel 发送，关连接释放 Read/Write，等待关闭才能确认解析工作已结束 |
| `tcp/server.go` | ListenAndServe 返回包装错误；Serve 支持已建 listener；net.ErrClosed 正常结束；其他 Accept 错误返回；实现 Close 和等待 | 不再对 nil listener 调用 Accept，也不在永久错误上死循环；便于真实 TCP 和注入错误验收 |
| `cmd/server/main.go` | 检查 ListenAndServe 错误并以非零状态退出 | 端口占用必须让调用者看到启动失败 |
| `resp/safety_test.go`、`resp/handler_test.go`、`tcp/server_test.go` | 增加 S1 回归与生命周期测试 | 普通成功请求无法覆盖恶意输入和提前退出 |
| `resp/parser_test.go`、`resp/fuzz_test.go` | 更新 context 参数与 Fuzz 种子 | 保留已有测试，并覆盖新接口和长协议头 |

代码注释放在约束、取消、锁与资源关闭的位置，解释原因；学习说明集中写在本记录，避免逐行重复代码。

### 输入与生命周期约束

- 数组最多 1024 个参数，单个 bulk 最多 16 MiB，协议头最多 64 字节（含 CRLF），整条请求最多 32 MiB（含数组头、bulk 头和 CRLF）。累计预算在下一次 bulk 分配前检查。
- 协议头读取使用 `ReadSlice`，reader 缓冲区固定为 4096 字节。超长头即便没有换行也不会触发 `ReadBytes` 的持续扩容。64 字节是合法头上限，4096 字节是提前读取的缓冲上限，两者不同。
- 空输入的 EOF 是正常结束；半条数组头、bulk 头或 bulk 内容必须生成错误，不能误当正常 EOF。
- `ParseStream(ctx, reader)` 不接管 Reader 所有权。调用者停止消费时必须取消 context；Reader 可能阻塞时还必须关闭它。Handler 已完成这两步，并等到 Parser 的 channel 关闭。
- `Server.Close()` 与 `RespHandler.Close()` 用于中断并回收现有连接；关闭登记与 WaitGroup.Add 使用同一把锁，防止关闭过程中加入新工作。
- 协议错误只回一条 `-ERR Protocol error\r\n`，不拼接客户端字节或内部网络错误。业务错误的 Redis 风格化属于 S2，尚未更改。
- 非 `net.ErrClosed` 的 Accept 错误立即向上返回，没有实现额外重试策略。

### 验证命令与结果

工具只放在本次任务目录，不改系统 PATH 或仓库依赖。原始输出保存在 `C:\Users\Nepenthe\Documents\Codex\2026-10-01\task\work\validation`（普通复验见 `original-tests.txt`，竞争检测见 `race.txt`，Fuzz 见 `fuzz-10m.txt`，TCP 验收见 `tcp-s1.txt`）。

```powershell
$validationRoot = 'C:\Users\Nepenthe\Documents\Codex\2026-10-01\task'
$env:GOCACHE = Join-Path $validationRoot '.cache\go-build'
$env:GOTMPDIR = Join-Path $env:TEMP 'mini-redis-go'
$env:STATICCHECK_CACHE = Join-Path $validationRoot '.cache\staticcheck'
$env:CC = Join-Path $validationRoot '.tools\w64devkit\bin\gcc.exe'
$env:CGO_ENABLED = '1'
$env:PATH = (Split-Path -Parent $env:CC) + ';' + $env:PATH
Set-Location -LiteralPath 'D:\Code\GO项目\mini-redis'

go test ./... -count=1 -timeout=30s
go vet ./...
& (Join-Path $validationRoot '.tools\bin\staticcheck.exe') ./...
go test -race ./... -count=1 -timeout=60s
go test ./tcp -run 'TestListenAndServePortConflict|TestMalformedClientsDoNotAffectHealthyConnection|TestHundredAbnormalDisconnectsReturnToBaseline' -count=1 -v
go test -fuzz=FuzzParseStream -fuzztime=10m -parallel=2 ./resp
gofmt -l resp/RespParser.go resp/handler.go resp/parser_test.go resp/fuzz_test.go resp/safety_test.go resp/handler_test.go tcp/server.go tcp/server_test.go cmd/server/main.go
git diff --check
```

| 验证 | 结果 | 说明 |
|---|---|---|
| 原始基线 `go test ./...` | 通过 | 当时只有 resp 测试；不能由此推断整个 S1 完成 |
| 旧实现新增 `TestParserLimitsHeaderRead` | 失败（预期复现） | 长头读取了 16387 字节，超过一个 4096 字节 reader 缓冲；修复后通过 |
| 新实现 `go test ./... -count=1 -timeout=30s` | 通过 | resp 与 TCP 回归测试全部通过 |
| `go vet ./...` | 通过 | 无输出 |
| `staticcheck ./...` | 通过 | 设置 STATICCHECK_CACHE 后重跑，退出码 0，无输出；首次缓存权限失败保留为环境记录 |
| `go test -race ./... -count=1 -timeout=60s` | 通过 | 已对本次审查副本执行一次，resp 1.358s、tcp 1.260s；CGO=1，未检测到数据竞争 |
| 10 分钟 Fuzz | 通过 | `-parallel=2`；59440293 次执行，结束 PASS，总测试耗时 601.225s；原始输出 fuzz-10m.txt |
| 端口占用、畸形连接、100 条异常断连 | 通过 | 真实本机 TCP；100 个连接 SetLinger(0) 异常断开，goroutine 基线 3 → 回收后 3；输出 tcp-s1.txt |
| 原仓库普通测试、vet、build | 通过 | 写回后 CGO=0 复验，resp 0.196s、tcp 0.183s；vet 与 build 退出码均为 0 |
| 本次九个 Go 文件格式与 git diff --check | 通过 | gofmt 无输出；差异检查仅有既有 autocrlf 的换行提示，无空白错误 |
| 写回校验 | 通过 | 写回前 HEAD、干净工作树及全部基线哈希相符；写回后 12 个文件哈希与审查副本完全一致 |



写回工具在完成全部复制、逐项哈希校验及写回清单后，打印中文文件名时因 stdout 默认 cp1252 报 UnicodeEncodeError。未重试覆盖操作；随后独立核对全部 12 个目标文件，哈希均匹配，并完成原仓库测试。该错误属于工具输出编码，不是代码测试失败或文件写回失败。

本机工具：Go `go1.27.1 windows/amd64`；Staticcheck `2026.2.1 (v0.8.1)`；w64devkit `v2.10.0` / GCC `16.2.0`。C 编译器提供 `libsynchronization.a`，满足 Windows Go 竞争检测的要求。

编译器包来源为 [w64devkit 官方 v2.10.0 发布](https://github.com/skeeto/w64devkit/releases/tag/v2.10.0)，下载文件 `w64devkit-x64-2.10.0.7z.exe` 的 SHA256 已与官方发布元数据核对：`18d0a4c71a166f8401ab6305781bec5882b40b5e06ba9807c61cb5f3b3c6325e`。



补充执行约束：关于“新下载软件需额外批准”的指示到达前，本轮已在任务目录下载、校验并解压上述便携编译器，完成一次竞争检测。收到指示后未继续调用该 C 编译器；之后的原仓库复验使用 CGO=0 的普通 Go 测试。后续若要再次调用此工具链，应先获得所要求的额外批准。

### 未完成项与必要使用说明

- 当前 `Close` 是中断连接并回收工作，不是 S6 的信号驱动优雅停机；尚无信号处理、请求排空超时或 AOF flush。
- 本轮上限约束单条请求和解析缓冲，不是全局内存配额。连接数、合法写入的数据总量没有全局上限，也未增加认证或读写超时；不能宣称任意流量都不会耗尽资源。
- S1 工程验收已完成，说明书的六项可执行完成特征已勾选；学习问答仍需使用者自行解释，未将自动测试等同于口述验收。下一阶段是 S2：先验证命令大小写与多值 LPUSH，然后依说明书补命令、错误格式和测试。
- S3 TTL、S4 AOF、S5 性能对照、S6 Docker/CI/信号关闭等尚未实现，不能封版。10/03 的封版目标不能替代 §3.5 的完成门槛。
- `项目心得.md` 保留为早期学习记录，其中占位状态和 Temporary 重试描述已过时；当前状态以本记录和说明书 §6 为准。
- `ParseStream` 新增 context 参数是接口变化，现有仓库调用已同步；外部调用者需增加 `context` import，并遵守取消/关闭 Reader 的约定。
- 本次不触及 GitHub description/topics、徽章和仓库首页的发布效果，也未提交或推送。文件名与 module 名的整理留给 S6。全仓格式检查仍列出基线的 `resp/reply.go`、`resp/reply_test.go`；本次改动的九个 Go 文件均已 gofmt，未将 S6 的全仓格式要求记为完成。

### 原理来源

- [Go net.Listener](https://pkg.go.dev/net#Listener)：关闭 listener 会使阻塞的 Accept 返回。
- [Go bufio.Reader.ReadSlice](https://pkg.go.dev/bufio#Reader.ReadSlice)：未找到界定符且缓冲已满时返回 ErrBufferFull。
- [Go 竞争检测要求](https://go.dev/doc/articles/race_detector)：Windows 的 C 工具链需要包含合适版本的 mingw-w64 runtime。
- [Staticcheck 安装与运行](https://staticcheck.dev/docs/getting-started/)：本轮按说明书安装，记录实际版本以便复现。

## 2026-10-02 · P1-S2 命令完整（继续实施）

### 范围与任务备注

保留 S1 的所有修改后继续推进。所属 P1/S2，级别为核心必做；目标是大小写不敏感、多值 List 操作及基本 Redis 命令可用。原因是旧代码只识别大写四条命令、LPUSH 仅收一个值、回复编码不支持数组。

输入为 RESP2 参数数组；输出增加 PONG 简单字符串、LRANGE bulk 数组与 Redis 风格公开错误。结构保留 16 分片和双向链表，增加固定顺序的多分片锁；命令执行不依赖 resp 包。先实现分发与参数检查，再补命令、回复、测试，最后执行普通测试/vet/build。必须测试：各命令参数数量、类型冲突、缺失 key、LPUSH 顺序、LRANGE 负下标/端点/越界/整数溢出、重复 key、二进制数据、输入输出别名、并发多 key，以及同连接 20 条请求。

常见错误是把多个 LPUSH 值倒序插入、LRANGE 忘记包含 stop、EXISTS 错误地去重 key、DEL 重复计数、按参数顺序锁多个分片，以及将错误链原样回写。使用者应能解释为何命令用数组、WRONGTYPE 与 ERR 的区别、为何网络回复不能包含内部错误链。这些是学习问答，未代替用户口述验收。

### 文件与设计原因

- `database/Engine.go`：统一命令名，新增 PING/DEL/EXISTS/LRANGE，多值 LPUSH；单值存储及读取/范围结果拷贝字节，避免外部修改存储。DEL/EXISTS 按分片编号顺序锁定，重复参数只锁一次。
- `database/errors.go`：业务错误显式提供公开 RESP code/message，同时保留 errors.Is 所需原因；未知命令不拼接客户端输入。
- `resp/reply.go`：增加简单字符串、bulk 数组和 int64；只编码公开错误字段，隐藏错误链；拒绝状态行中的 CRLF，公开错误消息去除 CRLF。
- `database/engine_test.go`、`resp/command_reply_test.go`、`tcp/commands_test.go`：表驱动、字节所有权、并发与真实 TCP 测试。`resp/reply_test.go` 同步内部错误隐藏语义。

命令语义：`PING` 返回 PONG，`PING message` 返回 bulk；`LPUSH k a b c` 得到 c,b,a；LRANGE 的 stop 包含在范围内，负下标从尾部计算，越界截取或返回空数组；EXISTS 的重复 key 重复计数，DEL 对一个存在的 key 只删除计数一次；LPOP 删除最后一个元素时删除 key；SET 可替换 List。

### 验证与状态

沿用 S1 的 GOCACHE/GOTMPDIR，`CGO_ENABLED=0`，未调用新下载的 C 编译器：

```powershell
go test ./... -count=1 -timeout=30s
go vet ./...
go build ./...
go test ./tcp -run TestTwentyCommandsOnOneConnection -count=1 -v
```

| 验证 | 结果 |
|---|---|
| 普通测试 | 通过：database 0.142s、resp 0.200s、tcp 0.182s；保留 S1 回归 |
| vet / build | 通过，退出码 0 |
| 同一连接 20 条命令 | 通过，包括小写、混合大小写、PING、List、类型/参数错误及错误后的下一条请求 |
| `go test -race ./...` | 未运行：待用户明确允许使用便携编译器；S1 的旧结果不覆盖本阶段新代码 |
| 官方 redis-cli | 未运行：PATH 及已检查的程序/开发目录未找到 redis-cli，未下载或安装；已用相同 RESP2 线格式的真实 TCP 验证行为 |

原始输出：本任务 `work/validation/s2-tests.txt`、`s2-tcp.txt`。S2 核心实现与普通验收完成，阶段勾选仍为空，尚不能声称全部完成特征通过。按用户补充授权继续可实施的后续代码；未运行项独立保留，最终阶段验收不跳过。

已整理之前两个回复文件的 gofmt，因此 S1 记录中提及的 `reply.go` / `reply_test.go` 格式遗留在本阶段消除；这不表示 S6 全仓门面整理已完成。未扩展学习客户端，其数组显示能力不作为服务端兼容性保证，使用说明继续以官方 redis-cli 为主。

语义来源：[LPUSH](https://redis.io/docs/latest/commands/lpush/)、[LRANGE](https://redis.io/docs/latest/commands/lrange/)、[EXISTS](https://redis.io/docs/latest/commands/exists/)、[PING](https://redis.io/docs/latest/commands/ping/)。

## 2026-10-02 · P1-S3 TTL（继续实施）

所属 P1/S3，核心必做；目标是访问不返回过期 key、未访问的过期值也能回收。原因是单靠惰性删除会永久留下冷 key。输入为 EXPIRE/TTL 参数，输出 1/0 或 TTL 秒数/-1/-2；保留 String/List 和分片结构，过期状态与数据由同一把分片锁保护。

前置知识：惰性删除是在访问时检查，主动删除是后台检查；context 表达取消，不能代替等待 goroutine；随机抽样的索引必须在删除时保持一致。本次先加过期元数据和全部访问路径，再加有界随机采样、context 生命周期与测试，最后写 README 语义。必要测试包括缺失/永久/过期、0/负数/非法秒数、SET 清除 TTL、List 保留 TTL、索引换位删除、所有命令访问过期 key、并发清理、取消和 10 万 key 不读取回收。常见错误是无条件升级 RWMutex、只检查 GET、List 操作丢 TTL、清理全表长时间持锁或为每 key 起定时器。使用者需能解释双策略、单 key 定时器的代价和有界锁持有；口述未验收。

### 改动与使用说明

- `database/ttl.go`：EXPIRE/TTL，按最近整秒报告 TTL；缺失 -2、无 TTL -1。正数秒数受 time.Duration 范围约束，0/负数立即删除，不支持额外选项。
- `database/Engine.go`：过期 map 与可 O(1) 随机抽样的 key 索引；GET/LRANGE 用读锁，发现过期时释放读锁再取写锁重新检查，防止删除刚被另一个连接续期的值。EXISTS/DEL 在已锁定分片中清理过期 key，LPUSH/LPOP 保留存活 key 的 TTL，SET 清除原 TTL。
- 后台每 250ms、每分片最多 256 次随机抽样；删除索引时把末尾 key 换到空位并清空末尾引用。每轮释放分片锁，不做全表扫描和每 key 定时器。
- `cmd/server/main.go`：将 Ctrl+C/SIGTERM 映射到 context；信号关闭 Server，取消并等待清理工作。此时仍为中断连接；S6 的在途请求排空和 AOF flush 尚未实现。
- `database/ttl_test.go`、`tcp/ttl_test.go`、`cmd/server/main_test.go`：确定性时钟、真实时间 10 万 key、TCP 语义及启动失败/取消回收测试。
- `README.md`：新增当前状态、命令表、TTL 语义、架构、取舍和边界；无推测性能数字。S6 仍需最终门面整理和封版验收。

### 验证结果

```powershell
go test ./... -count=1 -timeout=60s -v
go test ./tcp -run TestTTLCommandsOnTCP -count=1 -v
go vet ./...
go build ./...
```

| 验证 | 结果 |
|---|---|
| 全包普通测试 | 通过；database 11.415s、resp 0.210s、tcp 0.196s、cmd/server 0.167s |
| 10 万 key，全部 EXPIRE 5 秒，无 GET | 通过；写入结束后 11.159s 内全部被后台删除，低于 30s |
| GC 后活跃 HeapAlloc | 45,941,768 → 14,741,896 bytes；测量为 Go 活跃堆，不是进程 RSS；map/slice 容量可以保留 |
| TCP EXPIRE/TTL 与 context 取消 | 通过；启动失败和取消均等待清理工作退出 |
| vet/build | 通过，退出码 0；SDK 的遥测缓存权限提示属于环境输出，不是构建失败 |
| -race | 未运行，待既有工具链许可；不得套用 S1 的旧结果 |
| 实际控制台 Ctrl+C / 进程工作集回落 | 未运行；已有 context 取消和 HeapAlloc 证据，不将二者误报为这两个人工验收 |

原始输出在任务 `work/validation/s3-tests.txt` 与 `s3-tcp.txt`。S3 阶段勾选保留为空，待补完未运行项。后续按补充授权继续 AOF 的可实施工作，各阶段最终验收分别记录。

环境检查：已有 Docker 客户端，但 `docker version` 与 `docker image ls` 均报告 `dockerDesktopLinuxEngine` 命名管道不存在，未启动服务、未拉取镜像、未安装软件。S6 Docker 验收因此待运行；官方 redis-cli 仍未找到。

原理和语义参考：[EXPIRE](https://redis.io/docs/latest/commands/expire/)、[TTL](https://redis.io/docs/latest/commands/ttl/)。


## 2026-10-02 · P1-S4 AOF（继续实施）

所属 P1/S4，核心必做。目标是进程崩溃后恢复已确认写入，同时明确未确认请求和磁盘失败的边界。输入为通过引擎参数/类型检查的写命令，输出仍为 RESP 回复；新增顺序 RESP 日志和启动重放，不增加 Rewrite、复制或其他数据结构。

### 顺序、文件与使用说明

- `database/persistence.go`：在分片锁内校验并准备改动，串行化持久化写入，**追加日志 → fsync → 改内存 → 回复**。只做 always-fsync 一种策略；说明书推荐周期刷盘但允许单一策略，本次选择前者以简化失败一致性，代价是写延迟和写吞吐。无效命令和不会改变状态的操作不记日志。追加失败返回公开 `ERR persistence write failed`，内存不应用本次改动；内部原因保留给服务日志和错误链。
- `aof/store.go`：启动顺序重放；只把文件尾部不完整记录截到最后一条完整命令。完整但损坏的记录、非法重放命令、另一进程持有的文件锁均拒绝启动，防止静默丢数据。写入/同步失败后尝试回滚至上次确认偏移，进入永久拒绝写入状态，须排除磁盘问题后重启。文件锁实现位于 `aof/lock_windows.go` / `lock_linux.go`，当前支持 Windows/Linux。
- `resp/RespParser.go`：Payload 增加实际消费的 `BytesRead`；不能用 bufio 预读量或重新编码长度当作 AOF 偏移，合法的前导零会改变原始帧长度。相应测试见 `resp/record_size_test.go`。
- EXPIRE 重写为仅内部重放使用的绝对 Unix 毫秒期限 `__EXPIREATMS`，网络端不开放该命令。重启不重新计算剩余秒数，已经过期的 key 被删除；非正数 EXPIRE 记录 DEL。
- `cmd/server/main.go`：先绑定端口再开文件；默认 `go run ./cmd/server -addr :6379 -aof data/appendonly.aof`。`-aof ""` 明确关闭持久化；数据目录与 `*.aof` 已忽略。关闭时等待 handler 后同步并关闭日志，完整的请求排空仍待 S6。

崩溃窗口：依赖操作系统 fsync 成功的保证，已经返回成功的写命令没有应用层缓冲秒数（0 秒）。日志已同步而内存尚未更新/回复尚未送达时崩溃，重启可能恢复一条客户端未确认的操作，因此重试 LPUSH/LPOP 不能假设幂等。极端存储故障可能让同步、回滚同时失败；此时文件结果不确定，服务拒绝新写入并报告错误，不能声称断电、控制器或文件系统损坏下绝对不丢数据。日志随操作增长，未实现压缩。

### 验证命令与实际结果

```powershell
go test ./aof ./database ./resp ./cmd/server -count=1 -timeout=60s -v
go test ./... -count=1 -timeout=60s -v
go vet ./...
go build ./...
go test -fuzz=FuzzParseStream -fuzztime=10m -parallel=2 ./resp
```

| 验证 | 结果 |
|---|---|
| 全包普通测试 | 通过：aof 2.871s、cmd/server 0.224s、database 11.550s、resp 0.283s、tcp 0.249s；修改写路径后重跑 10 万 key TTL 回归 |
| 真正强制退出子进程 | 通过；子进程同引擎/Store 写满 1000 个 key，未调用 Close 即被父测试终止，重开文件完整恢复；仅终止测试自己创建的进程 |
| 尾部恢复 | 通过；覆盖三种不完整尾部和前导零帧；恢复后追加并再次重放仍正确 |
| TTL/二进制/List/DEL/并发重放 | 通过；TTL 不延长，8 个写入 worker 的重放状态与内存最终状态一致 |
| 存储失败与独占锁 | 通过；模拟短写/写错误/同步错误，回滚并拒绝后续写入；第二个持有者被拒绝 |
| 完整损坏记录 | 通过；拒绝启动且不擅自截断 |
| vet / build | 通过，退出码 0 |
| 解析器 10 分钟 Fuzz | 通过，79,148,927 次执行，600.271s；BytesRead 改动已覆盖 |
| -race / 官方 redis-cli / 人工 kill 与截断 | 未运行；既有 C 编译器权限待定、redis-cli 未找到。进程恢复与截断已有自动化证据，未误记为人工验收 |

曾失败：Windows 使用 O_APPEND 打开文件后 Truncate 返回 Access is denied。原因是 Go 的 Windows 追加模式限制写权限；改为独占文件锁 + O_RDWR + 定位到最后完整偏移，保留 Store mutex 以串行追加。不是放宽系统权限；尾部截断、追加和再次重开均复验通过。失败原始输出保留在 `work/validation/s4-aof-tests.txt`，修复输出在 `s4-aof-recovery-fixed.txt`，完整结果在 `s4-tests-final.txt` / `s4-fuzz-10m.txt`。

阶段实现与普通验收完成，S4 总勾选保留待补项。必要学习问答：为何写前日志能保护内存、fsync 为何降低吞吐、为何未确认命令仍可能恢复、为何 TTL 必须存绝对期限、为何真实 Redis 需要 Rewrite。自动化没有代替用户口述。

参考：[Redis 持久化](https://redis.io/docs/latest/operate/oss_and_stack/management/persistence/)、[Redis 8.2 过期源码](https://raw.githubusercontent.com/redis/redis/8.2/src/expire.c)、[Go 1.27.1 Windows 文件打开实现](https://raw.githubusercontent.com/golang/go/go1.27.1/src/syscall/syscall_windows.go)。

## 2026-10-02 · P1-S5 性能数据

核心必做，目标为可复现数据，不预设分片一定更快。新增 `database/benchmark_test.go`，同一 Engine 用 1/16/64 分片，三种负载；预填和预建参数在计时外，每个 worker 独立随机序列，不用共享随机数锁。必要约束：只测内存路径，明确 key/value/读写比/并发数、三次重复、分配开销和 profile。

环境：Windows/amd64；AMD Ryzen 7 9700X（8 核/16 逻辑处理器）；CIM 可用物理内存 33,396,543,488 bytes（约 31.10 GiB）；Go 1.27.1；GOMAXPROCS=16，RunParallel 默认 16 个 worker。 未调用待批准的 C 编译器，也未安装软件。

```sh
go test ./database -run='^$' -bench=BenchmarkEngine -benchmem -benchtime=2s -count=3 -cpu=16
go test ./database -run='^$' -bench='BenchmarkEngine/hot80/shards16$' -benchmem -benchtime=10s -count=1 -cpu=16 -cpuprofile=cpu.pprof -o=database-profile.test.exe
go tool pprof -top -nodecount=15 database-profile.test.exe cpu.pprof
```

九格三次全部通过（71.214s），CPU profile 及 pprof top 已生成并检查。热点场景 16/64 分片反而慢，不能据均匀负载推广。完整表格、27 条原始结果、分配数据、profile 热点和解释在 [performance.md](performance.md)，README 已摘要引用。采样文件在本任务 `work/validation/s5-cpu.pprof`，复现命令会在执行目录生成自己的文件。性能结论属于该硬件/负载，不是 AOF 或网络 SLA。

S5 工程完成特征已达成；阶段勾选不代表代替用户学习问答。必须理解 2 的幂、热点锁、伪共享的含义，以及 profile 能说明什么/不能说明什么。S6 接着补关闭、构建、CI 与文档；远端徽章与 Docker 环境验收仍待处理。

## S6 注释迁移：早期 channel 学习笔记

从 `resp/parser_test.go` 移出的原始笔记保留于此；适用的生命周期约束和完整示例见学习指南。

```text
读取channel的几种方式
方法一： 读取一次
payload := <-channel 这种方法会阻塞 直到channel中出现一个值
payload, ok := <-channel 这里的ok用于判断 channel是否关闭
如果 成功读取 则ok == true
如果 channel已关闭且没有剩余数据 ok == false
if !ok {channel已经关闭}

方法二： 持续读取 直到channel关闭 等价于源源不断执行接收并在channel关闭后退出
适合生产者发送多条数据的场景

for payload := range channel {
	fmt.Println(payload)
}


方法三：使用select 适合同时等待多个 Channel、取消信号或超时。



select {
case value := <-ch1:
	// ch1 可以接收数据时运行

case value := <-ch2:
	// ch2 可以接收数据时运行

case <-ctx.Done():
	// Context 被取消或超时时运行

default:
	// 当前没有任何 Channel 可以操作时立即运行
}

但是select只选择一次 如果要多轮选择 要加for循环
```

## 2026-10-02 · P1-S6 本地工程整理与当前交付

所属 P1/S6，核心必做；只推进 MiniRedis，不启动说明书 P2 AI 应用后端。目标为可运行、可审查、可学习的工程状态，并明确外部验收门槛。输入是取消/信号及已有代码，输出是有序停机、构建/CI 配置和当前项目文档；没有推送、合并、部署或变更 GitHub 元信息。

### 改动及原因

- `tcp/server.go`：增加 Shutdown；先关 listener，Serve 等待排空结果，防止 listener 关闭后的 defer Close 抢先中断在途回复。Handler 接口新增 `Shutdown(context.Context) error`；仓库调用/测试包装器同步，外部自定义 handler 需实现此方法。
- `resp/handler.go`：在同一锁下定义请求开始边界；停机关闭闲置/半包连接，取消 parser，保留正在执行的回复，拒绝预读流水线请求。context 的网络写截止时间与强制连接关闭不等于强行取消任意 Exec/fsync；超时明确返回，Close 则强制关闭并等待。
- `cmd/server/main.go`：信号停机给连接排空 2 秒，再等待 cleanup，最后 Store.Close 同步/关闭。普通场景小于三秒已经自动测过，人工 Ctrl+C 和极端磁盘卡死不能误报为已通过；极端不可取消 fsync 可能拖延最终 flush。
- `tcp/shutdown_test.go`、`resp/shutdown_test.go`、`cmd/server/shutdown_test.go`：真正 TCP 当前请求回复、拒绝下一条流水线/新连接、超时可见、普通退出后立即获得 AOF 锁并重放已确认写入。
- `Dockerfile` / `.dockerignore`：Go 1.27.1 多阶段构建，scratch 非 root 运行，/data 命名卷；未运行。`.github/workflows/ci.yml`：Ubuntu race/vet/固定 Staticcheck v0.8.1/build/格式，checkout/setup-go v7；文件已编写，远端未执行。
- `go.mod` 与全体 import：module 改为 `github.com/Li-Nepenthe/mini-redis`。`database/Engine.go` 改为 `engine.go`，`resp/RespParser.go` 改为 `parser.go`，只删除内容为 package database 的空 `routine.go`。Windows 文件名只改大小写使用同目录临时名，最终目录项已核对；测试、编译都在改名后运行。
- `.gitattributes` 为 LF；全仓 gofmt；`.gitignore` 增加 audit.sh、*-audit.txt、ROADMAP.md、pprof/test 临时产物，移除旧 docx QA 目录条目。
- 学习客户端只删除 emoji/夸张注释，不扩展功能；旧测试注释代码删除，channel 笔记迁入上方记录。全仓图标字符清理，说明书要求和勾选文字保留。基线没有 README，因此不存在旧 README 要搬成 ROADMAP；新 README 已覆盖当前工程说明。
- 核心代码在协议预算、输入输出所有权、锁排序、TTL 重查、日志确认和停止回收处增加中文“为什么”备注；没有逐行重复说明。
- `docs/learning-guide.md` 从启动/命令到请求追踪、模块、调试、分轮练习与参考问答；README 链接它。历史 notes 段落为当时状态，当前以本段/README/§6 为准。

### 实际验证

```powershell
go test ./... -count=1 -v -timeout=60s
go test ./tcp ./resp ./cmd/server -run='Shutdown|RunCancellation|ConcurrentClose' -count=1 -v -timeout=30s
go vet ./...
go build ./...
staticcheck ./...
gofmt -l .
# 独立 shell 设置 GOOS=linux、GOARCH=amd64、CGO_ENABLED=0 后：
go build -trimpath -o mini-redis-linux ./cmd/server
```

| 项目 | 结果 |
|---|---|
| 全包普通测试（改 module/文件名与停机后） | 通过：aof 2.763s、cmd/server 0.167s、database 11.445s、resp 0.227s、tcp 0.179s |
| 停机专题 | 通过；当前回复保留、排队命令不执行、listener 先停、超时公开；AOF 立即重开恢复，约 1ms/4ms（两次样本），不宣称性能保证 |
| 100 次 RST 断连 | 新代码回归通过，goroutine 基线 3 → 3 |
| 1000 次强制退出恢复 / TTL 10 万 key | 新代码回归通过；TTL 11.158s，GC HeapAlloc 45,939,528 → 14,739,672 bytes |
| vet / build / 已有 Staticcheck | 通过，退出码 0；没有进一步下载/安装软件，没有调用待批准 C 编译器 |
| Linux/amd64 交叉构建 | 通过；验证 Linux 文件锁代码可编译，不等于在 Linux/Docker 内运行 |
| gofmt / module import / emoji / 文档链接 | 最终清理复核，详见交付复核备注 |
| 新增代码完整 race | 未运行，工具链明确许可尚待获得；不套用 S1 结果 |
| redis-cli、人工 Ctrl+C、进程 RSS | 未运行；真实 TCP、context 取消、活跃堆证据分别记录，不互相冒充 |
| Docker build/run | 未运行，已有客户端但 daemon 命名管道不存在；未启动服务、未拉取镜像 |
| GitHub CI/徽章、description/topics、首页 | 未运行/未修改，用户未要求发布；配置文件存在不代表 CI 绿 |
| 用户口述七项与学习自查 | 未验收，学习文档提供参考思路，不代替本人回答 |

原始输出在本任务 `work/validation/s6-tests.txt`、`s6-shutdown.txt`、`s6-vet.txt`、`s6-build.txt`、`s6-staticcheck.txt`、`s6-linux-build.txt`。已修复的 S4 Windows Truncate 失败仍保留失败输出与复验，未隐去失败经历。

当前 S1/S5 工程完成，S2/S3/S4 实现与普通验证通过但仍有验收缺口，S6 本地工程已完成可实施部分、外部验收尚未闭合。日期不替代门槛；不能宣布 10/03 封版。剩余工作以补验/学习验收为主，须完整 race、官方客户端、真实信号/RSS、Docker 和获授权后的远端 CI/首页验证；不扩写 P2 或其他 Redis 功能。

配置来源：[Docker 多阶段构建](https://docs.docker.com/build/building/multi-stage/)、[官方 Go 镜像标签](https://raw.githubusercontent.com/docker-library/official-images/master/library/golang)、[checkout](https://github.com/actions/checkout)、[setup-go](https://github.com/actions/setup-go)。

### 交付前静态复核

45 个工程文件的复核中，旧 module import、图标字符和 Markdown 本地失效链接均为零，`gofmt -l .` 输出为空。已有 PyYAML 解析 CI YAML 成功，核对触发器、只读权限及 race/staticcheck/build 步骤；这只是语法/结构检查，不是远端执行结果。Linux 交叉构建退出码为 0。

本轮未改 Git 索引，文件保持可审查的本地修改。Windows 的大小写不敏感会使默认 git status 继续显示旧 Engine.go 名称；目录项已经改为 engine.go，复核可用 `git -c core.ignorecase=false status --short`。之后获授权准备提交时需要显式记录该大小写改名，不能只凭默认 status 推断目录没变。

### 原工作区最终复验

已写回 `D:\Code\GO项目\mini-redis`，分支 main、HEAD `6daf72512bd25a1a670d0f8957a1cc66cfecf992` 保持不变；与本地缓存 origin/main 的提交差为 0/0。开始时没有未提交或未推送工作，实施过程中每阶段先校验旧文件哈希，未覆盖其他修改；45 个最终工程文件与已测试镜像一致。未提交、推送、合并、部署或改 Git 索引。

原目录 `go test ./... -count=1 -timeout=60s` 再次通过：aof 2.805s、cmd/server 0.187s、database 11.452s、resp 0.235s、tcp 0.204s；客户端无测试。`git diff --check` 退出码 0，gofmt 为空。最终把文字文件的实际 CRLF 换行统一为 LF，以消除 Git 换行提示；Go 源码内容没有因此改动。日志为任务 `work/validation/original-final-tests.txt`，最终阶段汇总为 `summary-final.json`。全部未运行验收继续保留，不据本次本地结果宣称封版。


## 2026-10-02 · 最终 race 授权验收与空闲进程内存修复

本段更新此前“race/真实信号/RSS 待补”的状态；上方历史记录保留当时事实。使用者明确同意后，只在本项目测试子进程使用已有官方便携 w64devkit GCC；没有安装新工具、改全局 PATH/编译器/Go 配置、推送、合并、部署或启动外部服务。P2 AI 后端继续不在范围。

### 基线与保护

原目录 `D:\Code\GO项目\mini-redis`，main，HEAD `6daf72512bd25a1a670d0f8957a1cc66cfecf992`。本次开始已有上一轮交付的 45 个本地工程文件，与哈希记录全部一致，没有新出现的用户修改；HEAD 和 Git 索引保持不变，cached origin/main 为 0/0，未据此声称已重新 fetch 远端。此次追加 2 个 Go 文件修改、1 个测试及现有文档更新，写回前再次逐文件核对，不覆盖未识别修改。

### 发现、修复及为什么

首次完整 race 通过；进一步用实际主程序写入 10 万个 256-byte value、5 秒 TTL key，停发命令并只采样操作系统进程内存。修复前：写完工作集 85,819,392 bytes，30 秒后 85,807,104，只变化约 12 KB，不能称为实质内存回落。此前 HeapAlloc 下降只证明对象释放，不能代替此项进程内存验收。

- `database/engine.go`：构造 Engine 时设置私有 reclaim 函数为 debug.FreeOSMemory，测试可注入观察函数。
- `database/ttl.go`：主动清理返回实际删除数；累计达到 32,768，且距上次回收至少 5 秒时，在全部分片锁释放后请求 GC 并尝试归还系统页。每 250ms、每分片 256 次抽样预算继续生效。中文备注解释空闲分配不足、锁外调用和限频原因。
- `database/cleanup_test.go`：`TestLargeExpirationReclaimsOutsideShardLocks` 用过期 key 和注入回收函数检查大批删除会触发回收、所有分片锁已释放、context 取消后工作结束。完整 race 覆盖这条生命周期路径。

这是达到说明书 S3 内存回落目标的局部修复。debug.FreeOSMemory 有全进程 GC/归还页成本；不是每轮清理都调用，也没有调整 GOGC。key 数门槛不等于字节门槛，map/slice 容量可能保留，不保证小批过期立即归还内存，不保证回到启动工作集。context 不能强制打断已进入的 GC/fsync；大量过期时的请求尾延迟尚未测量，原 S5 无清理基准不覆盖该成本。

### 验证命令与结果

以下从原仓库运行，便携工具和缓存只属于本任务。APPDATA、LOCALAPPDATA、GOCACHE、GOMODCACHE、GOTMPDIR 指向任务缓存/Temp；避免工具尝试写不可写的用户默认缓存。普通检查另开子进程使用 CGO_ENABLED=0；Linux 交叉构建另设 GOOS/GOARCH，均没有写系统环境配置。

```powershell
$taskRoot='C:\Users\Nepenthe\Documents\Codex\2026-10-01\task'
$env:CGO_ENABLED='1'
$env:CC="$taskRoot\.tools\w64devkit\bin\gcc.exe"
$env:PATH="$taskRoot\.tools\w64devkit\bin;"+$env:PATH
& 'D:\Code\GoSDK\go1.27.1\go\bin\go.exe' test -race ./... -count=1 -v -timeout=180s

# 普通检查子进程使用 CGO_ENABLED=0：
go test ./... -count=1 -timeout=60s
go vet ./...
staticcheck ./...
gofmt -l .
# Linux 构建子进程：GOOS=linux、GOARCH=amd64、CGO_ENABLED=0
go build -trimpath -o <task>/work/validation/mini-redis-linux ./cmd/server

# 已有 Python + 本任务原生进程验收辅助脚本，不依赖安装包：
python <task>/work/process_acceptance.py signal
python <task>/work/process_acceptance.py crash
python <task>/work/process_acceptance.py memory
```

`<task>` 指上述任务目录；原生进程脚本和原始日志属于本机验收证据，未混入仓库运行依赖。脚本运行前先用普通构建生成 `work/validation/mini-redis-acceptance.exe`。

| 检查 | 实际结果 |
|---|---|
| 修复后全量 race | 通过、退出码 0、无 DATA RACE：aof 3.877s、cmd/server 1.319s、database 15.274s、resp 1.397s、tcp 1.221s；新锁外回收测试 2.42s |
| 修复后原目录普通测试 | 通过：aof 2.766s、cmd/server 0.188s、database 13.754s、resp 0.249s、tcp 0.214s；cmd/client 无测试 |
| vet / Staticcheck | 通过、退出码 0；Staticcheck 复核首次因默认 LOCALAPPDATA 缓存无写权限而失败，改为任务子进程缓存后通过，失败日志保留 |
| 普通主程序 build / Linux amd64 交叉构建 | 通过；Linux 产物 3,772,156 bytes，不等于已在 Linux/Docker 运行 |
| 10 万 key / 无读取清理 | 普通/race 测试通过；race 中 11.07s，HeapAlloc 45,949,664 → 14,749,856 bytes，与外部进程工作集分开记录 |
| 100 次 RST 断连 | 修复后 race 回归通过，goroutine 3 → 3 |
| 实际主程序 Windows Ctrl+C | 最终 signal 复验两次约 1.617/1.701ms，退出码均为 0；恢复 String/List，TTL 不因重启延长；memory 进程退出约 3.003ms |
| 实际主程序 5 秒 TTL | TTL 5 → 3 → -2，到期 GET nil；永久 TTL -1 |
| 实际主程序强制终止/重启 | 1000 次已确认 SET 恢复 1000/1000；Windows TerminateProcess 不执行正常 Close，Linux kill -9 字面命令未运行 |
| 实际 AOF 最后一帧截半 | 重启成功并记录尾部恢复；999 key 保留，仅最后一条 nil，非完整坏记录静默容错 |
| 原始内存测量 | 修复前约 12 KB 变化，未验收通过；修复后下表实质回落，不掩盖此前差距 |

内存复验：真实服务、AOF 关闭、10 万 key、每值 256 bytes、5 秒 TTL；写入耗时 4.035s。写完不发送 GET/TTL、不从外部强制 GC、不调用 EmptyWorkingSet，仅查询 Windows WorkingSet/Private Bytes。生产代码自身的受频率限制回收生效。

| 写完后秒数 | 工作集 bytes | Private Bytes |
|---:|---:|---:|
| 0 | 82,616,320 | 91,025,408 |
| 5 | 62,046,208 | 70,389,760 |
| 10 | 62,046,208 | 70,389,760 |
| 15 | 25,841,664 | 34,123,776 |
| 30 | 25,829,376 | 34,095,104 |

启动工作集为 6,864,896 bytes；15 秒回落约 69%，没有回到启动值。该记录是本机本次运行的观测，不是每台机器/每种负载的比例保证。

Ctrl+C 辅助脚本给本次服务单独创建隐藏控制台。AttachConsole 后核对控制台 PID 恰为辅助进程和自有服务，才广播 CTRL_C_EVENT；没有影响其他服务/终端。最初重复验收发现辅助进程忽略 Ctrl+C 的属性被第二个子进程继承，导致其不能收信号；在每次 spawn 前清除继承属性后复验通过。属于辅助脚本问题，没有为此改产品信号逻辑；失败输出保留。

新证据位于任务 `work/validation/`：`final-race-after-memory-fix.txt`、`final-acceptance-tests.txt`、`final-acceptance-vet.txt`、`final-acceptance-staticcheck.txt`、`process-signal-and-native-ttl-final.txt`、`process-signal-result.json`、`process-crash-after-memory-fix.txt`、`process-crash-result.json`、`process-memory-before-fix.json`、`process-memory-result.json`。失败记录包括 `process-signal-harness-inheritance-failure.txt`、`process-crash-harness-inheritance-failure.txt`、`final-acceptance-staticcheck-cache-permission-failure.txt`。本次汇总为 `final-acceptance-summary.json`，此前 `summary-final.json` 属于前一交付，不能继续用其 race 待批准状态判断当前代码。

### 尚未闭合的验收与必要使用说明

S1/S3/S4/S5 当前本机工程验收完成；S2 只缺官方客户端，S6 还缺 Docker、远端 CI/首页和本人七项口述。官方 redis-cli 未找到；已有 Docker 客户端 29.7.2，但 dockerDesktopLinuxEngine 命名管道仍不存在，build/run 未执行。未安装新软件、启动服务或拉取镜像。GitHub 配置存在，不代表 CI 已绿；未获发布指令，不推送、不改 description/topics，也不据日期宣告封版。Linux 的实际运行与字面 kill -9 可留待该环境验收，当前交叉构建只说明可编译。

下一项为在已有或获授权可用的官方客户端/Docker 环境补验，再按用户发布授权验证远端 CI/首页；学习指南提供原理和命令，由本人完成口述，不代勾。不新增 Redis 功能或推进 P2。真实 Ctrl+C 和工作集的自动验收已经完成，不再把它们列为未运行。

实现依据：[Go debug.FreeOSMemory](https://pkg.go.dev/runtime/debug#FreeOSMemory) 说明强制 GC 并尝试归还内存；[Windows GenerateConsoleCtrlEvent](https://learn.microsoft.com/en-us/windows/console/generateconsolectrlevent)、[AttachConsole](https://learn.microsoft.com/en-us/windows/console/attachconsole) 与 [Go os/signal](https://pkg.go.dev/os/signal) 用于核对实际控制台信号路径与继承边界。


## 2026-10-02 · 官方客户端与 Docker 实测、审查发布准备

使用者明确同意启动已有 Docker Desktop、必要时下载官方镜像、运行临时验收容器，并将本任务改动提交到新的审查分支、创建草稿 PR 和跟进 CI。没有合并 main、部署、改变安全配置或安装其他软件。本段更新此前官方客户端/Docker 不可用的历史状态。

### 保护与环境

开始时原目录 main/6daf725 的 46 个任务文件与上轮交付哈希一致；远端 main 只读复核仍是同一 SHA。六个被忽略的 .idea 文件单独记录，继续保留，不加入提交。Git 推送 dry-run 通过，拟用审查分支 codex/mini-redis-p1-acceptance-20261002，未覆盖既有分支。Git 未配置提交身份，使用已连接账户 Li-Nepenthe 的 ID 134032858 对应匿名邮箱，仅对本次 commit 设置，不更改全局配置。

已有 Docker Desktop 引擎 29.7.2 启动成功。Dockerfile 使用官方 golang:1.27.1-alpine3.24（解析到 sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414）；官方 redis:alpine 本轮为 redis-cli 8.10.2，digest 为 redis@sha256:3811787313eba226a2ef38658c6ccb91cd5e110edc89c37767de373120a0e5a0。测试服务以 65532:65532 运行，命名卷保存 AOF，端口随机映射且仅绑定 127.0.0.1。未借用其他容器或数据卷。

### 实测命令与结果

```powershell
docker build --progress=plain -t mini-redis:codex-acceptance-20261002 .
docker pull redis:alpine
python <task>/work/docker_acceptance.py
```

辅助脚本在本机任务目录，只依赖已存在的 Python 标准库和 Docker。它创建随机名称/唯一标签的服务、客户端和卷；每次清理先核对所有者标签，再按容器 ID 操作。所有本轮临时容器和卷已删除，镜像与已获准启动的 Docker Desktop 保留；没有全局 prune 或停止其他容器。

| 验收 | 结果 |
|---|---|
| Docker 多阶段构建及运行 | 通过，scratch 非 root 镜像；容器启动及本机映射端口 PING 成功 |
| 官方 redis-cli 同连接 20 命令 | 通过；第 1/5/10/20 条后采样的 TCP socket inode/本地端口相同，未重连；小写、LPUSH/LRANGE 顺序、WRONGTYPE、多 key 和 TTL 均符合预期 |
| 5 秒 TTL | 同一 Linux 环境内等待 5.2s；guest uptime 增量 5.21s，TTL 5 → -2，GET nil；宿主机含容器启动计时 5.040s，未混用两侧时钟 |
| 官方客户端二进制 | redis-cli -x SET 经 docker -i 传入零字节/CRLF/中文；raw GET 精确匹配原字节与客户端输出末尾换行 |
| Linux SIGKILL/AOF 重启 | docker kill --signal KILL；退出码 137、OOM false；重启后 String/List 完整、绝对 TTL 未续命 |
| SIGTERM 干净退出/AOF | docker stop --time 3；0.294s、退出码 0，随后重开 AOF 成功；仅是本次样本，不保证极端存储等待耗时 |

首次辅助验收的 redis-cli raw 错误展示空行导致输出错位，服务器实际 RESP 帧已核对只有正确的一条 CRLF 结尾；修正展示层读取后通过。宿主机等待后 TTL 仍为 1 的失败保留，改用容器内同一 Linux 时钟并记录两侧计时，不调整 VM/系统时钟。二进制初次辅助脚本遗漏 docker -i，导致 stdin 未进入客户端、写入空值；补上 -i 后字节比对通过。这些是验收辅助脚本修正，未为此修改产品代码。辅助 cleanup 也改为处理 --rm 客户端已自行退出的情况，未掩盖失败记录。

日志位于任务 work/validation：docker-build.txt、docker-redis-pull.txt、docker-acceptance.txt、docker-acceptance-result.json、docker-acceptance-events.json。失败记录为 docker-acceptance-client-format-failure.txt、docker-acceptance-host-timer-failure-* 和 docker-acceptance-stdin-fixture-failure-*。产品 Go 源码与此前完整 race 的 46 文件基线相同，本轮仅文档更新；不重复十分钟 Fuzz 或把 Docker 通过等同于远端 CI。

### 当前进度及下一项

S1–S5 当前本机工程验收完成；S6 的 Docker/客户端与真实停机已闭合。接下来提交并推送新审查分支、创建草稿 PR，观察该提交的 race/vet/Staticcheck/build/格式检查；远端结果需实际返回后记录，不能预先勾选。main 首页/默认分支徽章仍待获准合并后验证，本人口述未代答，未宣告封版或推进 P2。

匿名邮箱格式依据：[GitHub 邮箱说明](https://docs.github.com/en/account-and-profile/reference/email-addresses-reference)。构建、协议与计时结论以本段本机实测为证据。


## 2026-10-02 · 草稿 PR 与远端 CI 通过

按已获授权把本任务 48 个差异路径提交到新审查分支 codex/mini-redis-p1-acceptance-20261002，未包含 .idea、数据、任务日志/工具或无关文件。Windows 大小写改名在索引中显式记录为旧 Engine.go 移除、新 engine.go 加入；暂存 diff --check 首次发现性能展示 CPU 行尾填充空格，文档清除该空格后通过，原始 benchmark 日志未变。gofmt、46 文件哈希、LF、图标字符与 Markdown 本地链接复核通过。提交身份仅在 commit 参数中设置为已连接账户的匿名邮箱，不改全局配置。

实现提交为 3d95ea3523566ee3fb60198bc416c5773b8dc573，main/base 仍为 6daf72512bd25a1a670d0f8957a1cc66cfecf992。执行 git push --set-upstream origin HEAD:refs/heads/codex/mini-redis-p1-acceptance-20261002 成功，未强推；创建一次 [草稿 PR #1](https://github.com/Li-Nepenthe/mini-redis/pull/1)，base main、draft true、merged false，并核对 head SHA。

[CI 运行 36974612279](https://github.com/Li-Nepenthe/mini-redis/actions/runs/36974612279) 对该实现提交成功；Ubuntu runner 的 checkout/setup-go、gofmt、完整 race、vet、Staticcheck v0.8.1、build 与收尾步骤全部 success，不是根据空 status 列表推断。来源记录为任务 work/validation/ci-run-1-jobs.json 与 ci-run-1-complete.json，推送/提交日志为 review-push.txt 与 review-commit-final.txt。本段为已完成运行的记录，同一审查分支的后续文档提交不改变代码；审查时仍以 PR 当前 HEAD 的 checks 为准。

S1–S5 工程验收已完成；S6 的本机/Docker/官方客户端与审查 PR CI 均已通过。仍未合并 main，所以默认首页/徽章保留未勾选；本人七项口述尚未验收，不替其作答，未按日期宣布封版。没有部署、启用自动合并、变更安全配置或推进 P2。临时验收容器/卷已清理，Docker Desktop 与必要镜像保留。


## 2026-10-02 · 独立复审：AOF 历史 TTL 修复

用户追加授权：完成代码审查/测试后合并 P1，再实现说明书 P2 M1–M5；先完成工程再本人学习，不把代理讲解作为本人口述通过。此前“未授权合并/P2”的文字是历史快照，本段为当前范围。逐项覆盖、必做/推荐增强/排除项见 acceptance-matrix.md。

独立只读审查在 HEAD 3382d0d 证实一处缺陷的两种表现：LPUSH a→EXPIRE5→LPUSH b→到期重开 AOF 变为 TTL=-1/List[b]；t0 SET+EXPIRE5、t4续期10、t6重放丢失仍应存活的值。根因是 Replay 以及普通写路径在历史重放中按重启当前时间提前删键。

最小修复：重放 prepareWrite 跳过当前时间的惰性删除，绝对 EXPIREATMS 只恢复期限；完整历史恢复后读写/worker 再过期。正常 LPUSH 对不存在/已过期 key 持久化 _LNEW，明确重建 List 并清旧 TTL，避免旧类型/内容混入。无新增网络命令、无 AOF Rewrite。Replay 自己应用验证后的记录，不再调用可能重新追加的 executeWrite。

增加固定时钟五场景（过期前追加/续期/过期后重建/String变List/已清理重建）、旧绝对期限兼容、不再追加、私有命令拒绝，以及真实 aof.Store 到期再重开回归。初步 go test ./database ./aof -run 'TestReplayPreservesHistorical|TestLegacyReplay|TestExpiredListWithLater' -count=1 -v 通过。全量 race/vet/Staticcheck/build、修复提交 CI 与 main 合并证据待实际完成后补记；没有把初测当作最终验收。

旧草稿日志缺“过期后创建”标记，时间信息不足不能反推，已写 README 使用限制；不自动重写或替换用户 AOF。P2 下一步：同仓独立 module ai-backend 的 M1，MySQL 手写 SQL/鉴权/分页/索引实测，不引入说明书排除的功能。


### 复审最终本机结果

内部标记最终采用与 LPUSH 同为5字节的 _LNEW。独立复审发现较长私有名会使合法32MiB帧落盘超预算，已补精确最大帧通过回归；未扩大 Parser 限额。全量 go test -race ./... -count=1 -timeout=180s 通过（aof5.220s、database15.239s，其余包亦通过），随后 go vet、Staticcheck v0.8.1、go build 通过，gofmt与diff --check为空。Parser代码未改，不无理由重复十分钟Fuzz。

已重新 docker build 修复镜像，并用官方 redis-cli +同一Linux guest等待验证：到期前追加List重启不复活；String续期仍保留（TTL15→13）；旧String过期后新List无旧值/TTL；SIGKILL137/OOMfalse重开AOF通过；SIGTERM退出0。独立只读审查额外运行200个种子×150步历史重放比较。原始证据在任务 work/validation/p1-review-fix-race-final.txt、p1-review-fix-docker-{build-final,events,result}；仅清理自有容器/卷。修复提交及main CI须后续实证，不能引用前一HEAD的绿灯作为本提交通过。


## 2026-10-02 · P1 合并/main 实证与 P2 M1

独立复审对 fbbce11 明确通过：两个历史TTL故障、重建/类型转换/删除边界、旧普通格式、精确32MiB Parser→真实Store→重开和200种子×150步历史一致。修复HEAD的CI36991097480/job110787252186全部success后，把草稿转ready并用expected_head_sha精确约束合并PR #1，GitHub返回merged=true、sha84d3c8cf7c853ff68addae3b5056c2bf80308a8c。不是默认开启自动合并或跳过检查。

main push的CI36991251698/job110787739322全部success（格式/full race/vet/Staticcheck/build）；已读远端main README并重新打开带本次提交验证参数的仓库首页，页面显示项目文档。普通网页缓存一度仍显示旧目录，本次未据旧缓存推断本地未推进；Actions/PR/Git远端提交为证据。原目录main已ff-only到该SHA，无未提交变更，未回退/覆盖他人工作。

S6 description/topics仍空。GitHub连接工具没有元信息写接口；computer-use SKILL要求先读核心guidance再控制UI，但当前委派环境取不到这些资源且没有node_repl，故该窄项保留待补，已向父会话报出；没有读取凭据绕过连接权限。本人学习问答仍不代签，P1工程就绪不等于所有封版条件已打勾。

已从main在任务work/mini-redis-p2建独立分支codex/ai-backend-p2-20261002，M1使用Gin/手写SQL/MySQL/bcrypt/AccessToken/owner鉴权，真实MySQL8.4.11的事务回滚、密码散列、分页不重、跨用户403、全量race和curl实际进程验收通过；索引三条实际EXPLAIN与对照rows有数据。细节/WHY/命令/剩余项在ai-backend/README.md、docs/notes.md、docs/explain.md及根验收矩阵。P1核心Go文件未因P2修改。


## 2026-10-02 · P2 当前小块收尾，P1 学习意义/WHY 复核

本段为当前范围，之前“下一项”是历史记录。用户最新要求P2只完成当前小块后停止，继续核验P1学习指导，特别是设计/函数为什么这样做及学习意义。M1修复8397f97独立复审+CI36995249329全成功，PR #2合并ef8817dd263435beee3f023acdb11db8307b74d8，main CI36996394132两模块全成功。原目录main快进保留已有文件；未回退/覆盖.idea或其他工作。

Provider只收尾已写的Generate/Stream、Mock/HTTP adapter及本地回归，保存本地codex/ai-backend-m2-m3-20261002@5a973b71f3c215558331eb3a4d5ba309fa534488。独立复审复现显式SSE error仍Done、TLS握手Timeout分类错误，两项最小修复后准确提交独立Provider race+真实TLS/SSE回归1.760s通过。完整本机Provider race1.674s、vet/Staticcheck/build/格式通过；未推送/合并此块，无生成业务端点/落库/页面，M2整阶段未完成，M3–M5暂停。自己未提交的Embedding/检索/额度准备已移除；自有MySQL容器按所有者标签清理，无模型费用，无其他数据库操作。

### 本轮教学改动和原因

新工作树任务work/mini-redis-p1-learning、分支codex/p1-learning-why-20261002、基线main ef8817d。docs/learning-guide.md重写为渐进六轮：命令→字节与调用链→函数契约/锁/所有权→TTL→AOF→生命周期/验证。每个关键函数明确输入输出、返回时是否持锁、状态/资源归属和失败路径；WHY附错误后果与替代取舍，不让初学者只背函数名。AOF历史续期丢值/到期List复活/过期后重建边界，以及删除引用却工作集不降，作为可重现实例。加入术语、实验预期、第二层自查提示与三分钟/十五分钟口述路线。

8个P1 Go文件共25行设计注释：掩码构造/FNV-1、锁排序去重/锁外所有权、prepare/apply与日志锁顺序、TTL索引/读锁返回契约、Parser按长度读取、公开错误、AOF确认前缀/真实偏移、WaitGroup登记门和defer等待顺序；没有应用行为改变。保持说明书“代码只写为什么，完整讲解在文档”的原则，不逐行冗余注释。README、验收矩阵、P2 README/记录与原说明书§6更新当前暂停状态，不修改原功能/验收要求或替本人答题。

### 可执行命令与已发生结果

```powershell
go test -race ./... -count=1 -timeout=180s
go vet ./...
staticcheck ./...
go build ./...
gofmt -l .
```

当前P1全部通过，无DATA RACE，格式输出为空；记录work/validation/p1-learning-{race,vet,staticcheck,build,format}.txt及p1-learning-checks.json。使用Go1.27.1、已有便携GCC、Staticcheck0.8.1，仅进程环境，不改系统配置。指南全部普通test命令匹配实际当前测试名，源码链接定位存在，Go diff逐项确认仅注释；材料receipt为p1-learning-material-checks.json。Fuzz/性能代码未改，不重复10分钟Fuzz/九格或把旧数字冒作本轮重测。

原样提取指南Python+PowerShell独立内存实验，只有将临时Python文件路径换为实际任务文件：10万个256-byte、TTL5秒、无AOF、同一自有PID、不后续GET/外部GC；写入3.539秒，工作集88,051,712→28,491,776 bytes（15秒）→28,475,392（30秒），exit0。峰值88,055,808保持不变，验证“峰值不等于当前使用”；原始输出p1-learning-memory-experiment.txt。完整P1回归与此实验同机并行，这是可复现的内存样本，不是吞吐/P99对照。按明确持有的Process对象只停止实验PID，不碰其他服务；强制清理不算优雅停机验收。

指南单独存Library时也可直接打开main源码链接，文档ZIP不需要伪装携带源码。独立教学完整性与技术准确性复核、准确文档提交CI/合并结果随后以实际检查与交付记录为准，本节记录提交前本机检查，不预填远端通过。description/topics仍因连接无写入口且computer-use必要guidance/node_repl缺失待补；没有读凭据绕过权限。个人六轮检查点、S6七项、3/15分钟口述和第二层追问仍本人待答。材料完善不等于个人封版，也不恢复P2功能推进。
