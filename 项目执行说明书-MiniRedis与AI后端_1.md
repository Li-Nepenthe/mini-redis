# Mini-Redis 与 AI 应用后端 · 项目执行说明书

**周期**：2026-08-22 → 2027-01-08（20 周）

---

# §0 AI 执行说明

## 0.1 你的职责

你是使用者的 Go 工程教练，做三件事：

1. **监督进度** —— 对照 §6 进度表，确认上一阶段的完成特征全部达成，才允许进入下一阶段
2. **拆解任务** —— 按 §0.4 格式，每次只给一个可验证的任务
3. **教学与 Review** —— 讲清原理，审查代码，不替他写主体代码

## 0.2 使用者背景（用于校准讲解深度）

| 项 | 情况 | 你的应对 |
|---|---|---|
| 计算机基础 | 系统学过两年 408（数据结构 / 操作系统 / 计算机组成原理 / 计算机网络），部分遗忘 | 网络与并发相关概念是唤醒不是重学。能从原理推出的，引导他推，不直接给结论 |
| 数据库 | 零基础 | MySQL 相关从最基础讲起 |
| Go | 初学者，有 Java 基础，缺完整工程经验 | 术语不默认他懂，先解释再使用 |
| 学习偏好 | 要看推理过程，不接受只给结论；权威来源也会追问 | 讲清 why 优先于讲清 how |

## 0.3 每次对话流程

```
1. 读 §6 进度表，确认当前阶段
2. 核验上一任务的完成特征 —— 未达成则不推进，先解决
3. 安排一个可验证任务（格式见 §0.4）
4. 讲清所需原理
5. 他编码
6. Code Review（检查项见 §0.5）
7. 他修复 → 跑测试 → 提交
8. 更新 §6 进度表
```

## 0.4 任务分配格式

每个任务必须包含以下 12 项：

```
1.  所属项目与阶段
2.  级别：核心必做 / 推荐增强
3.  任务目标
4.  为什么需要它
5.  前置知识（他可能不懂的术语在此解释）
6.  输入 / 输出
7.  接口与数据结构约束
8.  推荐实现步骤
9.  必须编写的测试
10. 验收标准（可执行的命令或可观察的现象）
11. 常见错误
12. 完成后他必须能回答的问题
```

**每次只安排一个任务。**不得同时要求完成多个模块。

## 0.5 Code Review 检查项

逻辑正确性 · 边界条件 · 错误处理 · 并发安全 · goroutine 泄漏 · context 传递 · 资源关闭 · 接口设计 · 包职责 · 命名 · 测试覆盖 · 性能 · Go idiom · 日志是否泄露敏感信息 · 是否过度设计 · 是否引入不必要组件 · **是否存在他解释不了的代码**

## 0.6 红线

1. 不一次性生成完整模块。可给接口、伪代码、测试骨架、局部片段，主体代码他自己写
2. 不编造性能数字。"高并发""零分配"不能代替真实 Benchmark 和 Profile
3. 不对错误方案迎合，包括他坚持的方案
4. 上一阶段完成特征未达成，不推进下一阶段
5. 不安排 §1.4 明确不做清单里的内容
6. 出现 §0.7 情形时主动提出削减

## 0.7 应主动提出削减的情形

- 功能与项目目标不匹配
- 时间成本超出阶段预算
- 只为堆技术关键词
- 架构复杂度远超需求
- 已达到 §4.9 最低可交付状态，却仍在追加可选功能

## 0.8 上下文加载

| 场景 | 加载章节 |
|---|---|
| 日常任务推进 | §0 + §1 + §2 + 当前阶段小节 + §6 |
| 阶段验收 | 上述 + 该阶段完成特征 |
| 调整排期 | 全文 + §5 |

---

# §1 目标与约束

## 1.1 目标

> 在 2027 年 1 月 8 日前完成两个项目，每个都能被深挖 40 分钟不露怯。

## 1.2 完成标准

| | 标准 |
|---|---|
| **A** | 两个项目的全部完成特征勾选（§3、§4） |
| **B** | 各能脱稿讲 3 分钟版与 15 分钟版 |
| **C** | 任意技术点被追问，能答到第二层 |

3 分钟版：做了什么 / 解决什么问题 / 技术选型理由
15 分钟版：架构 / 关键设计取舍 / 遇到的问题与解决过程

## 1.3 预算

| 项 | 值 |
|---|---|
| 周数 | 20 周（第 1—20 周） |
| 项目开发投入 | 约 19 小时 / 周 |
| 总计 | 约 380 小时 |

分配：P1 约 100 小时（6 周），P2 约 280 小时（14 周）。

## 1.4 明确不做（AI 不得安排）

**P1**：AOF Rewrite、Cluster、Sentinel、主从复制、Lua、事务、Pub/Sub、RDB、String 与 List 之外的数据结构、客户端功能增强

**P2**：Kubernetes、OpenTelemetry Trace、Refresh Token、多 Provider 故障切换、gRPC、微服务拆分、Workflow 状态机、Human-in-the-loop、长期 Memory、管理后台、死信队列、重试策略矩阵、向量数据库

**MCP**：本周期内不做。两个项目完成后作为第一个增强项，预算 2 周。

## 1.5 作品集

```
P1  Mini-Redis          底层能力：网络、协议、并发、持久化、性能分析
P2  AI 应用后端平台      工程主项目：业务工程 + AI 工程
```

---

# §2 时间线

| 周 | 日期 | 阶段 | 关键特征 |
|---|---|---|---|
| 1—2 | 08/22—09/04 | P1-S1 安全与正确性 | 恶意输入打不崩服务器 |
| 3 | 09/05—09/11 | P1-S2 命令完整 | redis-cli 能完整操作 |
| 4 | 09/12—09/18 | P1-S3 TTL | key 自动过期，内存不涨 |
| 5 | 09/19—09/25 | P1-S4 AOF | 杀进程重启，数据还在 |
| 6 | 09/26—10/02 | P1-S5/S6 数据与封版 | 有性能对照表，CI 绿 |
| — | **10/03** | **P1 永久封版** | 此后不再改动 |
| 7—9 | 10/03—10/23 | P2-M1 业务地基 | 能注册登录建会话，越权被拒 |
| 10—12 | 10/24—11/13 | P2-M2 模型接入与流式 | 文字逐字流出，关页面即停止计费 |
| 13—15 | 11/14—12/04 | P2-M3 缓存/限流/成本 | 重复提问秒回，超配额被拒 |
| — | **12/08** | **达到最低可交付状态** | M4、M5 转为增强 |
| 16—18 | 12/05—12/25 | P2-M4 异步任务与队列 | 上传大文档秒返回，杀 worker 不丢任务 |
| 19—20 | 12/26—01/08 | P2-M5 检索与评测 + 收尾 | 回答带引用，有三方案对照数据 |
| — | **01/08** | **项目开发结束** | 此后不写新功能 |

**铁律**

1. 10/03 之后不再碰 Mini-Redis，新想法记入 README 的"未来方向"
2. 01/08 之后不写任何新功能

---

# §3 项目一：Mini-Redis

## 3.1 定义

> 用 Go 从零实现一个兼容 RESP2 子集的单机并发内存数据库。

**用户视角**：启动服务后用官方 `redis-cli` 直接连接，执行 SET/GET/DEL/EXPIRE/TTL/LPUSH/LPOP/LRANGE；杀掉进程重启，数据仍在。

**目标不是命令数量，是协议健壮性、并发安全、可解释性。**

## 3.2 架构

```
                    redis-cli
                        │  TCP
                        ▼
        ┌───────────────────────────────┐
        │   tcp.Server                  │  监听、Accept、优雅关闭
        │   每连接一个 goroutine         │
        └───────────────┬───────────────┘
                        ▼
        ┌───────────────────────────────┐
        │   resp.Handler                │  连接生命周期管理
        └───────────────┬───────────────┘
                        │  ctx
            ┌───────────┴───────────┐
            ▼                       ▼
   ┌─────────────────┐    ┌──────────────────┐
   │ resp.Parser     │───▶│  chan *Payload   │
   │ (goroutine)     │    └────────┬─────────┘
   │ bufio + ReadFull│             │
   └─────────────────┘             ▼
                        ┌──────────────────────┐
                        │  database.Engine     │  命令分发
                        │   Exec(args)         │
                        └──────────┬───────────┘
                                   ▼
                    ┌──────────────────────────────┐
                    │  分片存储  shards[N]          │
                    │  ┌────────┐ ┌────────┐       │
                    │  │RWMutex │ │RWMutex │  ...  │
                    │  │ map    │ │ map    │       │
                    │  └────────┘ └────────┘       │
                    │  fnv32(key) & (N-1) 路由      │
                    └──────────┬───────────────────┘
                               │
                ┌──────────────┼──────────────┐
                ▼              ▼              ▼
          String([]byte)   List(链表)    TTL(过期时间)
                               │
                               ▼
                    ┌──────────────────────┐
                    │  AOF 追加 + 启动重放  │
                    └──────────────────────┘
                               ▲
                    ┌──────────┴───────────┐
                    │  resp.EncodeReply    │
                    └──────────────────────┘

旁路：后台 goroutine 定期抽样清理过期 key（受 ctx 控制）
```

## 3.3 技术清单

| 领域 | 具体内容 |
|---|---|
| 网络 | `net.Listener`、TCP 粘包/半包、`net.ErrClosed` |
| I/O | `bufio.Reader`、`io.ReadFull`、EOF 语义 |
| 并发 | goroutine-per-conn、channel 管道、`sync.RWMutex`、分片锁 |
| 生命周期 | `context` 取消、优雅停机、`defer` |
| 错误处理 | 哨兵错误、`errors.Is`、`%w` 包装 |
| 数据结构 | 哈希路由、双向链表、位掩码取模 |
| 持久化 | AOF 追加、启动重放、崩溃窗口 |
| 测试 | 表驱动、`-race`、Fuzz、Benchmark |
| 工程 | `staticcheck`、Dockerfile、GitHub Actions |

## 3.4 当前状态（2026-08-22，已核对源码）

**已完成**：编译通过 · `go vet` 干净 · `resp` 包有表驱动测试且 `-race` 通过 · SET/GET/LPUSH/LPOP · 分片 Engine（fnv32 + 位掩码）· RESP 数组解析 · Reply 编码 · 服务端客户端已分离 · 接口签名闭环

**未完成**：5 个致命缺陷（见 S1）· 缺 PING/DEL/EXPIRE/TTL/EXISTS/LRANGE · 无 AOF · 无优雅关闭 · 无 Fuzz · 无 Benchmark · 无 Dockerfile/CI · README 内容需替换

**2026-10-02 复核备注**：上述内容保留为 08/22 的历史快照，不作为本轮未完成项清单。实际本地为 `main@6daf725`，开始时无未提交修改、无未推送提交；已有数组/bulk 上限、负长度校验及 Fuzz 骨架。§6 尚无阶段完成记录，本轮从 S1 的连接生命周期和输入预算补齐开始，S2–S6 未实施。详细改动、验证结果、使用约束与剩余项见 [docs/notes.md](docs/notes.md)。


---

## S1 · 安全与正确性（第 1—2 周）

**本轮备注（2026-10-02）**：任务 3、4 的长度校验在基线已实现，任务 2 的 Fuzz 文件也已存在；本轮不重复记为新工作。补齐任务 5–7、协议头和整条请求的限额，并逐项执行验收。本轮 S1 工程验收已通过，六项可执行完成特征已勾选；学习问答待使用者口述确认，下一阶段为 S2。

**目标**：任意畸形输入都无法使服务崩溃或耗尽内存。

**任务**

| # | 内容 |
|---|---|
| 1 | 安装 `staticcheck` 并纳入日常流程：`go install honnef.co/go/tools/cmd/staticcheck@latest` |
| 2 | **先写 Fuzz 测试**，再修缺陷 |
| 3 | 修：`MaxArrayLength` / `MaxBulkLength` 已定义但未接入逻辑。`*1000000000\r\n` 会触发约 24GB 分配 |
| 4 | 修：负长度未校验。`Atoi("-1")` 返回 nil error，`make([][]byte, -1)` 触发 panic 终止整个进程 |
| 5 | 修：`net.Listen` 错误被 `_` 丢弃，端口占用时对 nil 接口调用 `Accept()` |
| 6 | 修：`Temporary()` 分支与永久错误分支都执行 `continue`，永久错误时死循环。该方法自 Go 1.18 起废弃，改用 `errors.Is(err, net.ErrClosed)` |
| 7 | 修：goroutine 泄漏。Handler 提前返回后，Parser 向无接收方的 channel 发送而永久阻塞。用 context 贯穿 `ParseStream` |

**Fuzz 骨架**

```go
// resp/fuzz_test.go
func FuzzParseStream(f *testing.F) {
    f.Add("*2\r\n$3\r\nGET\r\n$4\r\nname\r\n")
    f.Add("*-1\r\n")
    f.Add("*1\r\n$-1\r\n")
    f.Fuzz(func(t *testing.T, data string) {
        ch := NewRespParser().ParseStream(context.Background(), strings.NewReader(data))
        for range ch {}
    })
}
```

**完成特征**

- [x] `printf '*-1\r\n' | nc localhost 6379` 后服务器仍在运行，其他连接不受影响
- [x] `printf '*1000000000\r\n' | nc localhost 6379` 后内存不暴涨，返回协议错误
- [x] 同时启动两个 server，第二个打印 "address already in use"，而非 nil pointer 栈
- [x] `staticcheck ./...` 输出为空
- [x] `go test -fuzz=FuzzParseStream -fuzztime=10m ./resp` 无 crash
- [x] 建立 100 条连接后全部异常断开，`runtime.NumGoroutine()` 回到基线

**完成后必须能回答**：为什么校验要在分配之前 · goroutine 泄漏的常见成因有哪几类 · `Temporary()` 为什么被废弃 · Fuzz 的语料是怎么演化的

---

## S2 · 命令完整（第 3 周）

**10/02 最终验收备注**：大小写、多值 LPUSH、PING/DEL/EXISTS/LRANGE、Redis 风格错误和回复数组已实现；普通测试、真实 TCP 与最终全量 race 通过。已用官方 redis-cli 8.10.2 在 Docker 中连续执行 20 条混合命令，四次采样保持同一个 TCP socket；本阶段本机工程验收完成，学习问答仍需本人回答，详见 docs/notes.md 最新记录。

**目标**：`redis-cli` 能像操作真 Redis 一样操作它。

**任务**

1. 命令名统一转大写。当前小写命令全部落入 default 分支
2. 补 PING / DEL / EXISTS / LRANGE
3. 错误信息改 Redis 风格英文前缀：`-WRONGTYPE ...`、`-ERR wrong number of arguments for 'set' command`
4. 不再向客户端回写内部错误链
5. 每个命令补表驱动测试，含类型冲突用例（对 String 执行 LPOP）

**完成特征**

- [x] `redis-cli -p 6379 ping` 返回 PONG（官方容器客户端实测）
- [x] `redis-cli -p 6379 set foo bar`（小写）成功（官方容器客户端实测）
- [x] `lpush mylist a b c` 后 `lrange mylist 0 -1` 返回正确内容与顺序（官方容器客户端实测）
- [x] 对 String 执行 `lpop` 返回 `WRONGTYPE` 开头的错误（官方容器客户端实测）
- [x] `redis-cli` 连续执行 20 条命令不断连（官方容器客户端实测）
- [x] `go test -race ./...` 通过（10/02 内存修复后全量复验，无竞态报告）

**完成后必须能回答**：RESP 请求为什么统一用数组格式 · 类型不匹配应该返回什么错误码 · 错误响应为什么不能包含内部错误链

---

## S3 · TTL（第 4 周）

**10/02 最终验收备注**：EXPIRE/TTL、惰性删除、有界随机清理和 context 回收已实现。实际服务 5 秒 TTL 为 5 → 3 → -2，到期 GET 为 nil，永久 TTL 为 -1；10 万 key、256-byte value、5 秒 TTL、无 AOF/后续读取，修复后工作集在 15 秒内从 82,616,320 降至 25,841,664 bytes。修复为主动删除累计 32,768 后在锁外限频请求 GC，两次至少间隔 5 秒。全量 race、普通测试、真实 Windows Ctrl+C 与清理退出通过，完成当前本机工程验收；参考问答仍需本人学习，详见 docs/notes.md。

**目标**：key 按时过期，且过期 key 不会永久占用内存。

**任务**

1. `EXPIRE` / `TTL` 命令
2. 惰性删除：读取时检查过期时间
3. 定期清理：后台 goroutine 随机抽样，必须能被 context 取消
4. 语义写入 README：key 不存在返回什么 · 无 TTL 返回什么 · SET 是否覆盖 TTL · List 操作是否保留 TTL

**完成特征**

- [x] `SET k v` → `EXPIRE k 5` → `TTL k` 返回递减秒数（实际 TCP：5 → 3）
- [x] 5 秒后 `GET k` 返回 nil，`TTL k` 返回 -2（实际主程序验收）
- [x] 对无过期时间的 key 执行 `TTL` 返回 -1（实际主程序验收）
- [x] 写入 10 万个 5 秒过期的 key 且不读取，30 秒后进程内存回落（Windows 工作集实测；15 秒回落约 69%，不保证回到启动值）
- [x] `Ctrl+C` 时后台清理 goroutine 干净退出，无泄漏（真实 Windows CTRL_C_EVENT、退出码 0；取消等待与 goroutine 回归通过）
- [x] `go test -race ./...` 通过（10/02 内存修复后全量复验，无竞态报告）

**完成后必须能回答**：为什么惰性与定期两种策略都需要 · 为什么不给每个 key 起定时器 · 清理时为什么不能长时间持有锁

---

## S4 · AOF（第 5 周）

**10/02 最终验收备注**：实际主程序经 Windows 强制终止（不运行正常关闭）后恢复全部 1000 个已确认 SET；实际 AOF 文件最后一帧截去一半后重启，仅最后一个 key 缺失，其余 999 个保留。真实 Ctrl+C/重启的 String/List、绝对 TTL 与 full race 也通过。本机 Windows 强制终止与 Docker 内原生 Linux SIGKILL 路径均已通过，详见 docs/notes.md。

**目标**：崩溃后数据在明确定义的窗口内不丢失。

**任务**

1. 写命令成功后以 RESP 格式追加 AOF
2. 只做一种刷盘策略（推荐缓冲 + 定周期 fsync）
3. 启动时顺序重放
4. 处理文件尾部半条命令
5. AOF 写入失败不得静默忽略
6. README 写明崩溃窗口：写入顺序 · 最坏丢失多少秒数据 · 内存与文件何时不一致

**完成特征**

- [x] `SET k v` → `kill -9` → 重启 → `GET k` 返回 v（10/02 Windows 强制终止及 Docker 内 Linux SIGKILL 实测，不经过 Close）
- [x] 连续写 1000 条 → kill → 重启 → 数据条数正确（实际主程序 1000/1000 已确认写入恢复）
- [x] 手工截断 AOF 尾部半条命令 → 重启 → 服务正常启动，仅丢最后一条（实际文件截断的自动验收：999 保留、最后一条 nil）
- [x] 带 TTL 的 key 重启后过期语义与 README 描述一致（10/02 自动重启/过期测试通过）
- [x] README 含"崩溃窗口"一节（10/02：always-fsync、先日志后内存、TTL 与未确认写入边界）

**完成后必须能回答**：先写日志还是先改内存，各自的失败后果 · fsync 的代价 · 为什么真实 Redis 需要 Rewrite

---

## S5 · 性能数据（第 6 周前半）

**目标**：产出可复现的性能对照数据。

**任务**

跑 3 组分片配置 × 3 种负载：

| | 单全局锁 | 16 shard | 64 shard |
|---|---|---|---|
| 读多写少 (9:1) | | | |
| 纯写 | | | |
| 热点 key（80% 请求命中同一 key） | | | |

报告注明：CPU 型号 · 内存 · Go 版本 · 测试命令 · key 数量 · 并发数。

**完成特征**

- [x] 3×3 表格填满，数据来自 `go test -bench=. -benchmem`（10/02 工程验收，详见 docs/performance.md）
- [x] 能指出至少一个分片没有优势甚至更慢的场景，并解释原因（10/02 工程验收，详见 docs/performance.md）
- [x] 有一份 `pprof` CPU profile，能指出热点函数（10/02 工程验收，详见 docs/performance.md）
- [x] 结论写入 README，附完整环境说明（10/02 工程验收，详见 docs/performance.md）

**完成后必须能回答**：分片数为什么取 2 的幂 · 热点 key 场景下分片为什么可能失效 · 缓存行伪共享是什么

---

## S6 · 工程化与封版（第 6 周后半）

**任务**

1. 优雅停机：信号 → 关 listener → 等已有连接结束 → flush AOF → 退出
2. Dockerfile（多阶段构建）
3. GitHub Actions：`go test -race` + `staticcheck` + build
4. 门面清理（见下表）
5. 重写 README
6. 整理问答清单

**门面清理**

| 项 | 动作 |
|---|---|
| README | 现有内容移出仓库，改名 `ROADMAP.md` 并加入 `.gitignore` |
| emoji 与夸张注释 | `cmd/client/main.go` 中全部删除 |
| `go.mod` module 名 | 改为 `github.com/Li-Nepenthe/mini-redis`，同步更新所有 import |
| 文件名 | `Engine.go`→`engine.go`、`RespParser.go`→`parser.go` |
| 空文件 | 删除 `database/routine.go` |
| 换行符 | `gofmt -w .`，新增 `.gitattributes` 写 `* text=auto eol=lf` |
| 死代码 | 删除 `tcp/server.go` 开头 32 行注释块、`parser_test.go` 97—101 行 |
| 学习笔记 | 从代码注释移至 `docs/notes.md`。代码注释只写为什么，不写是什么 |
| 仓库元信息 | 补 description 与 topics |
| `.gitignore` | 增加 `audit.sh`、`*-audit.txt`；删除 `.docx_qa_resignation/` |

**README 结构**

```
# Mini-Redis
一句话定位

## 当前状态      已实现 X，未实现 Y
## 快速开始      go run + redis-cli 两行命令
## 支持的命令    表格
## 架构          Mermaid 图
## 设计取舍      3 条，每条含选择与代价
## 性能数据      S5 对照表
## 已知限制
```

**完成特征**

- [x] `docker build` + `docker run` 能起服务，`redis-cli` 能连（10/02 Docker 29.7.2、官方 redis-cli 8.10.2 实测；非 root/命名卷/本机端口通过）
- [x] GitHub Actions 徽章为绿（10/02 PR #1 已合并；main84d3c8c的CI36991251698全步骤通过，README main徽章已提供）
- [x] `Ctrl+C` 后进程 3 秒内干净退出，AOF 已 flush（10/02 真实 Windows CTRL_C_EVENT，约 1.6–3.0ms、退出码 0；AOF 重开重放通过，极端 I/O 未测）
- [x] GitHub 仓库首页显示的是项目文档（10/02 合并后页面重新读取实证；旧缓存不作为当前状态）
- [x] 全仓库搜索 emoji 结果为零（10/02 本地字符清理，最终复核见 docs/notes.md）
- [x] `gofmt -l .` 输出为空（10/02 本地复核）
- [ ] 能脱稿讲清 7 点：TCP 粘包半包 · RESP 解析 · 分片锁 · TTL 双策略 · AOF 崩溃窗口 · goroutine 生命周期 · S1 修复的两个崩溃缺陷

## 3.5 封版标准

S1—S6 全部完成特征勾选 → **10/03 封版，此后不再改动**。

---

# §4 项目二：AI 应用后端平台

## 4.1 定义

> 一个文档问答助手的后端：用户上传文档，用自然语言提问，系统检索相关内容、调用大模型生成带引用的回答，流式返回。

**工程定位**：把慢、贵、不可靠、流式的大模型调用，纳入确定性的后端工程体系。

**业务逻辑保持极简。**

## 4.2 用户旅程

```
1. 注册 / 登录                        → 获得 JWT
2. 上传 PDF / Markdown                → 页面显示"处理中"
3. 后台异步：解析 → 切块 → 向量化      → 进度条实时更新
4. 新建会话
5. 提问
6. 回答逐字流出                        → 末尾附引用："文档A 第3段、文档B 第7段"
7. 页面显示本月 token 用量与剩余配额

关键行为：关闭页面时后端立即取消对模型的调用，停止计费
```

## 4.3 架构

```
                          浏览器（原生 HTML + EventSource）
                                    │
                                    ▼
        ┌───────────────────────────────────────────────────┐
        │  API Service (Gin)                                │
        │  中间件：Request ID · JWT · 限流 · 日志 · Recover   │
        └────┬──────────────────────────────────────┬───────┘
             │                                      │
       ┌─────▼──────┐                        ┌──────▼──────┐
       │  同步路径   │                        │  异步路径    │
       │  （提问）   │                        │ （上传文档）  │
       └─────┬──────┘                        └──────┬──────┘
             │                                      │
             ▼                                      ▼
   ┌────────────────────┐              ┌──────────────────────┐
   │ 幂等 → 缓存 → 检索  │              │  写 MySQL + 发 Kafka  │
   │ → Provider.Stream  │              │  立即返回 task_id     │
   │ → SSE 推送          │              └──────────┬───────────┘
   └─────┬──────────────┘                         │
         │                                        ▼
         │                              ┌──────────────────────┐
         │                              │  Worker (独立进程)    │
         │                              │  解析→切块→Embedding  │
         │                              └──────────┬───────────┘
    ┌────┴─────────────────────────────────────────┴────┐
    ▼                    ▼                  ▼            ▼
┌────────┐        ┌───────────┐      ┌──────────┐  ┌─────────┐
│ MySQL  │        │  Redis    │      │  Kafka   │  │Provider │
│用户/会话│        │缓存/限流   │      │  任务队列 │  │(LLM API)│
│消息/文档│        │配额/幂等   │      │          │  │+ Mock   │
│chunks  │        └───────────┘      └──────────┘  └─────────┘
└────────┘
    │
    └──▶ Prometheus ◀── QPS / P99 / 缓存命中率 / token 成本 / 队列积压
```

## 4.4 两条核心数据流

**路径 A — 提问（同步、流式）**

```
POST /conversations/{id}/messages   携带 Idempotency-Key
  │
  ├─ 限流          Redis 令牌桶（用户级 + 全局）
  ├─ 配额          Redis 计数 + MySQL 结算
  ├─ 幂等          key 已处理 → 返回上次结果，不重复扣费
  ├─ 精确缓存      hash(model+prompt+params) 命中 → 直接流式吐出
  │                    miss ↓
  ├─ singleflight  同时刻相同请求只穿透一个
  ├─ 检索          查 chunks → 相似度计算 → top-k
  ├─ 组装 prompt
  ├─ Provider.Stream(ctx)      ctx 绑定 HTTP 请求
  ├─ SSE 逐 token 推送
  └─ 收尾          落库 message + 结算 token + 写缓存

  客户端断开 → ctx cancel → 上游 HTTP 请求 cancel → 停止计费
```

**路径 B — 上传文档（异步）**

```
POST /documents
  ├─ 存文件 + MySQL 建记录 (status=pending)
  ├─ 发 Kafka 消息
  └─ 立即返回 task_id

Worker:
  ├─ 消费（event_id 唯一索引去重）
  ├─ 解析 → 分块 → 批量 Embedding
  ├─ 写 chunks 表
  └─ 更新 status → 前端 SSE 感知
```

## 4.5 技术栈

| 层 | 选择 |
|---|---|
| Web | Gin。须能讲清其底层为 `net/http`，以及 Middleware 的串联机制 |
| 数据库 | MySQL + `database/sql` 手写 SQL。**不使用 ORM** |
| 缓存 | Redis |
| 队列 | Kafka（KRaft 单节点）。若 Docker 环境搭建超过 1 天，改用 RabbitMQ 或 Redis Streams |
| 向量检索 | Go 内存暴力余弦相似度。**不引入向量数据库** |
| 模型 | 一个 OpenAI 兼容 Provider + Mock Provider |
| 前端 | 原生 HTML + JS + `EventSource`，预算 8 小时 |

**前置条件**：需要一个大模型 API key。M2 第一件事是完成 Mock Provider，所有单元测试走 Mock，仅集成测试与评测调用真实接口。

## 4.6 数据模型

```sql
users           id, email, password_hash, status, created_at, updated_at
user_quotas     user_id, month, token_used, token_limit
conversations   id, user_id, title, created_at, updated_at
messages        id, conversation_id, role, content, prompt_tokens,
                completion_tokens, citations(JSON), created_at
documents       id, owner_id, name, status, error_code, created_at
chunks          id, document_id, position, content, embedding(BLOB), token_count
task_events     event_id(UNIQUE), task_id, type, payload, processed_at

索引：
  UNIQUE(users.email)
  INDEX(conversations.user_id, created_at)
  INDEX(messages.conversation_id, created_at)
  INDEX(chunks.document_id, position)
  UNIQUE(task_events.event_id)
  UNIQUE(user_quotas.user_id, month)
```

## 4.7 API

```
POST   /api/v1/auth/register
POST   /api/v1/auth/login

POST   /api/v1/documents                     上传 → 返回 task_id
GET    /api/v1/documents                     列表 + 处理状态
DELETE /api/v1/documents/{id}
GET    /api/v1/tasks/{id}/events             SSE：处理进度

POST   /api/v1/conversations
GET    /api/v1/conversations                 分页
POST   /api/v1/conversations/{id}/messages   SSE：流式回答 + 引用
GET    /api/v1/conversations/{id}/messages

GET    /api/v1/usage

GET    /health/live   /health/ready   /metrics
```

---

## M1 · 业务地基（第 7—9 周）

**目标**：一个不含任何 AI 的完整业务后端。

**涉及技术**：Gin 中间件 · JWT · bcrypt · `database/sql` 手写 SQL · 事务 · 索引 · 连接池 · 统一错误码 · 表驱动测试 + Repository 集成测试

**任务**

1. 项目骨架、配置加载（环境变量 + `.env.example`）
2. 表结构 + migration
3. 注册 / 登录 / JWT 中间件。只做 Access Token
4. 会话 CRUD + 分页
5. 统一错误码、统一响应格式、Request ID 中间件、Recover 中间件
6. 权限校验：用户只能访问自己的资源
7. 单元测试 + Repository 集成测试

**必须产出的材料**

- 表结构设计说明 + 每个索引的存在理由
- 至少 3 条关键 SQL 的 `EXPLAIN` 分析
- 一个索引优化前后对比（扫描行数变化）
- 连接池参数及理由
- 一个事务场景的边界与失败处理

**完成特征**

- [ ] `curl` 能完成注册 → 登录 → 获取 JWT
- [ ] 带错误 token 访问返回 401
- [ ] 用户 A 的 token 访问用户 B 的会话返回 403
- [ ] 数据库存储的是 bcrypt 哈希
- [ ] 会话列表分页正确，第二页不重复第一页数据
- [ ] 关键查询 `EXPLAIN` 的 `type` 列不为 ALL
- [ ] 能说出 `MaxOpenConns` 的取值与理由
- [ ] `go test -race ./...` 通过

**完成后必须能回答**：聚簇索引与二级索引的区别 · 什么是回表与覆盖索引 · 最左匹配原则 · JWT 无状态的代价与登出方案

---

## M2 · 模型接入与流式（第 10—12 周）

**目标**：把不可靠的外部调用纳入可控的生命周期管理。

**涉及技术**：`context` 传播与取消 · SSE · `http.Flusher` · 超时控制 · 错误分类 · 重试策略 · interface 抽象 · Mock 测试

```go
type Provider interface {
    Generate(ctx context.Context, req Request) (Response, error)
    Stream(ctx context.Context, req Request) (<-chan Event, error)
}
```

**任务**

1. 先完成 Mock Provider
2. 接入一个 OpenAI 兼容 Provider
3. 普通生成
4. SSE 流式：`Content-Type: text/event-stream` + Flush + 心跳 + Event ID
5. 客户端断开时取消上游调用
6. 错误分类：`InvalidRequest` / `RateLimited` / `Timeout` / `ProviderUnavailable` / `StreamInterrupted`
7. 重试边界：仅重试 429（尊重 `Retry-After`）与部分 5xx。参数错误、鉴权失败、已产生副作用的调用、ctx 已取消一律不重试
8. 消息落库 + token 用量记录
9. 最简前端：流式聊天页面

**完成特征**

- [ ] 浏览器提问时文字逐字出现，非一次性返回
- [ ] 关闭浏览器标签页，服务端日志立即打印上游取消
- [ ] Mock Provider 下 `go test ./...` 不需要网络、不产生费用
- [ ] 模拟 429 时按 `Retry-After` 重试
- [ ] 模拟 400 时不重试，直接返回错误
- [ ] 模拟流中途断开，客户端收到明确错误事件而非静默截断
- [ ] 设置 5 秒超时、Provider 拖 10 秒时，请求在 5 秒被取消

**完成后必须能回答**：客户端断开后服务在哪一层感知 · 取消信号如何传到 HTTP 客户端 · 上游 TCP 连接何时真正关闭 · SSE 与 WebSocket 的取舍

---

## M3 · 缓存、限流、成本（第 13—15 周）

**目标**：把外部调用的成本与流量纳入可控。

**涉及技术**：Cache Aside · `singleflight` · 缓存穿透/击穿/雪崩 · 令牌桶 · 幂等设计 · Redis 数据结构 · Lua 原子操作

**任务**

1. 精确缓存：`hash(model + prompt + params)` → 响应
2. 缓存穿透：空值缓存 + 参数校验
3. 缓存击穿：`singleflight`
4. 缓存雪崩：TTL 随机抖动
5. Token 计费与配额：请求前预估、响应后结算、超额拒绝
6. 限流：令牌桶，用户级 + 全局两层，Redis + Lua 保证原子
7. 幂等：`Idempotency-Key` 头，重复提交返回首次结果

**完成特征**

- [ ] 同一问题第二次提问，响应时间从秒级降到毫秒级
- [ ] 同时发起 50 个相同请求，对模型的调用次数为 1
- [ ] 配额调至 100 token 后，第二次请求返回明确的配额错误
- [ ] 1 秒内发 100 个请求，超出部分被限流，计数在 Redis 中可见
- [ ] 同一 `Idempotency-Key` 提交两次，`token_used` 只增加一次
- [ ] Prometheus 可见 `cache_hit_ratio` 且数值合理
- [ ] `go test -race ./...` 通过，含并发限流测试

**完成后必须能回答**：Cache Aside 为什么是删缓存而不是更新缓存 · 先删缓存还是先更新数据库 · `singleflight` 的实现原理 · 令牌桶与漏桶的区别 · 幂等键应该存多久

---

## M4 · 异步任务与队列（第 16—18 周）

**目标**：把耗时任务从请求路径中移出。

**涉及技术**：Kafka 生产消费 · 至少一次投递 · 消费幂等 · 任务状态机 · Worker 优雅停机 · SSE 进度推送 · 批处理

**任务**

1. Docker Compose 起 Kafka（KRaft 单节点）
2. 文档上传接口：存文件 + 建记录 + 发消息 + 立即返回 `task_id`
3. 独立 Worker 进程：消费 → 解析 → 切块 → 批量 Embedding → 写 chunks
4. 消费幂等：`event_id` 唯一索引
5. 任务状态机：`pending → running → succeeded / failed`
6. Producer 发送失败降级：不得导致用户请求失败
7. 任务进度通过 SSE 推送
8. Worker 优雅停机：不丢正在处理的任务

**完成特征**

- [ ] 上传 10MB 文档，接口 500ms 内返回 `task_id`
- [ ] 前端进度条从 0% 走到 100%，中间状态实时更新
- [ ] 杀掉 Worker 再重启，任务继续推进且结果不重复
- [ ] 手动重复投递同一条消息，chunks 表不产生重复数据
- [ ] 停掉 Kafka 后上传接口仍返回成功，文档标记为待处理
- [ ] `Ctrl+C` Worker 时，正在处理的任务完成后才退出
- [ ] Prometheus 可见队列积压指标

**完成后必须能回答**：至少一次与恰好一次的区别，为什么不宣称后者 · 消费幂等的几种实现 · 消费者组与 rebalance · 消息积压时如何定位

---

## M5 · 检索与评测 + 收尾（第 19—20 周）

**目标**：用数据回答"该用哪种检索方案"。

**任务**

**① 三条检索路线，用同一评测集对比**

| 方案 | 做法 |
|---|---|
| A. 全文直塞 | 小语料整体放入 context |
| B. 向量检索 top-k | 暴力余弦相似度 |
| C. 关键词 / BM25 | 无向量库，纯词法匹配 |

**② 评测集**：20—30 条固定 Case，每条含 `input` / 期望行为 / 参考答案 / 来源文档

**③ 指标**：准确率 · 引用正确率 · 无答案识别率 · 首字延迟 · token 成本

**④ 生成侧**
- 回答带引用来源
- 检索不到依据时明确说明，不得用模型自身知识冒充检索结果
- 用户文档权限隔离
- Prompt Injection 基础防护

**⑤ 工程化收尾**
- Docker Compose 一键起全套（API + Worker + MySQL + Redis + Kafka + Prometheus）
- GitHub Actions：`go test -race` + `staticcheck` + build
- 结构化日志，含 `request_id`
- Prometheus 指标：QPS · P99 · 错误率 · 缓存命中率 · token 成本 · Provider 错误率 · 队列积压
- 健康检查 + 优雅停机
- 压测：QPS · P99 · 缓存命中率 · singleflight 开关对照
- README + 架构图 + 故障排查文档

**完成特征**

- [ ] 回答末尾带具体引用（文档名 + 段落），可溯源到原文
- [ ] 对文档中无答案的问题，系统明确回答检索不到依据
- [ ] 用户 A 无法检索到用户 B 的文档内容
- [ ] 三方案对照表填满准确率 / 延迟 / 成本三列真实数据
- [ ] 能说出当前语料规模下哪个方案更优、以及规模变化后结论如何反转
- [ ] `docker compose up` 一条命令起全套，另开终端 `curl` 立即可用
- [ ] Prometheus 可见 token 成本随请求累加
- [ ] GitHub Actions 徽章为绿
- [ ] README 含架构图、快速开始、已知限制、评测结果

**完成后必须能回答**：长上下文与检索的适用边界 · 检索失败时如何定位是召回问题还是生成问题 · 评测集如何避免过拟合

## 4.8 完成标准

- [ ] M1—M5 全部完成特征勾选
- [ ] 能画出 §4.4 两条数据流图并逐步讲解
- [ ] 能回答：缓存一致性 · 索引设计 · 事务边界 · MQ 幂等 · context 传播 · 优雅停机 · JWT · 连接池 · Redis 或 Kafka 故障时的降级路径

## 4.9 最低可交付状态

> P1 封版 + P2 完成 M1、M2、M3 + 核心测试 + Docker Compose 可启动 + README 可展示

预计 12 月上旬达成。达成后 M4、M5 转为增强项，**不得因其未完成而阻塞项目对外展示**。

---

# §5 降级顺序

进度落后时按固定顺序削减：

```
第 1 刀   M5 的方案 A 与 C（保留 B + 评测集）
第 2 刀   M4 的 SSE 进度推送 + Worker 优雅停机
第 3 刀   前端界面（改用 curl 演示 SSE）
第 4 刀   M5 整块
第 5 刀   M4 整块（此时为 M1—M3，仍达到最低可交付状态）
```

**不可削减**：P1 的封版标准 · 每个阶段的完成特征

---

# §6 进度表

当前2026-10-02用户要求：P2只收尾已实施小块后暂停，优先核验P1学习指导、设计/函数WHY与真实缺陷实验。此前“先完成工程再学习”的范围已收束。下面勾选只记录工程实证，口述/个人学习仍单独待答。完整映射见 docs/acceptance-matrix.md。

> 每完成一阶段，将 `[ ]` 改为 `[x]` 并填写实际日期。AI 每次对话先读此表。

## P1 · Mini-Redis

- [x] S1 安全与正确性　　目标 09/04　实际 2026-10-02（工程验收完成；见 docs/notes.md；学习问答待口述确认）
- [x] S2 命令完整　　　　目标 09/11　实际 2026-10-02（官方客户端同 socket 20 命令、普通/TCP/全量 race 通过；学习问答待口述）
- [x] S3 TTL　　　　　　目标 09/18　实际 2026-10-02（工程验收完成；真实 TTL/10 万 key 工作集/信号/全量 race 通过，学习问答待口述）
- [x] S4 AOF　　　　　　目标 09/25　实际 2026-10-02（本机 Windows 工程验收完成；真实强制终止/半尾文件/TTL 恢复及全量 race 通过，学习问答待口述）
- [x] S5 性能数据　　　　目标 09/29　实际 2026-10-02（工程数据/分析完成，学习问答未代答）
- [ ] S6 工程化与封版　　目标 10/02　实际 ______（10/02 工程/复审修复/main合并与CI/首页通过；description/topics缺可用写入口、本人口述待答）

## P2 · AI 应用后端

- [x] M1 业务地基　　　　目标 10/23　实际 2026-10-02（仅工程：MySQL/事务/索引/curl/全量race通过；学习口述未代签；PR #2合并ef8817d、准确CI全成功）
- [ ] M2 模型接入与流式　目标 11/13　实际 ______（暂停：Provider小块5a973b7仅本地复核通过；业务SSE/落库/页面未实施，不作阶段完成）
- [ ] M3 缓存/限流/成本　目标 12/04　实际 ______（未实施；按用户要求暂停）
- [ ] **达到最低可交付状态**　目标 12/08
- [ ] M4 异步任务与队列　目标 12/25　实际 ______（未实施；暂停）
- [ ] M5 检索与评测 + 收尾　目标 01/08　实际 ______（未实施；暂停）
