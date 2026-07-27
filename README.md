# Go 实习求职与项目路线总规范：P0–P3

> **文档用途与版本说明**
>
> 本文档是 Go 求职基础轨道与三个核心项目的统一需求说明、架构设计、学习路线、开发规范和 AI 协作上下文。
>
> 可将本文档直接提供给 ChatGPT、Gemini、Codex 或其他 AI，用于：
>
> * 新对话上下文迁移；
> * 项目任务拆解；
> * 架构设计；
> * 代码审查；
> * Bug 调试；
> * 测试设计；
> * 项目阶段验收；
> * 简历与面试准备。
>
> 本版本依据 2026 年 7 月深圳及周边 Go 后端、基础架构、云原生和 AI Agent 相关岗位的公开要求进行校准。后续进入 P2、P3 或集中投递前，指导 AI 应重新联网检查当时的岗位要求，不得把本文件视为永久不变的招聘事实。
>
> 本文档不是不可修改的死方案。若后续招聘要求、开发实践、用户能力或时间安排表明某项设计收益过低，指导 AI 应直接建议删除、替换或重构，不得为了维持原方案继续堆砌无效功能。
>
> 本路线的核心原则是：**P0 全程并行，P1 完整且克制，P2 最完整且最深入，P3 先完成可靠 MVP，再按岗位和时间增加增强能力。**

---

# 1. 用户背景与总体目标

## 1.1 用户当前情况

* 用户是 Go 初学者。
* 已有一定 Java 和 Go 基础，但缺少完整的 Go 工程开发经验。
* 主要目标是寻找深圳及周边的：

  * Go 后端实习；
  * 基础架构实习；
  * 云原生相关实习；
  * AI 应用后端实习；
  * Agent 平台后端实习。
* 计划在 **2027 年 3 月前**形成成熟的项目组合和面试能力；但不等待三个项目全部完成，达到 **P1 完成 + P2 核心 MVP 可运行** 后即开始持续投递。
* 用户不是依靠单个项目投递，而是通过 P0 基础轨道和三个循序渐进的项目建立完整技能链。
* 用户会高强度使用网页版 AI 和 Codex 辅助开发，但必须保证：

  * 自己理解核心代码；
  * 自己能够调试；
  * 自己能够解释设计；
  * 自己能够回答面试追问。
* 项目不是为了追求功能数量，而是为了形成：

  * Go-native 编程能力；
  * 真实后端工程能力；
  * 网络与并发能力；
  * 数据库和中间件能力；
  * 前后端联调能力；
  * 测试和工程质量意识；
  * Docker、CI/CD 和 Kubernetes 基础认知；
  * 日志、指标和链路追踪能力；
  * AI Agent 工程化能力；
  * 持续算法、计算机基础和 Linux 排障能力；
  * 能够阅读和使用 Python Agent 生态的辅助能力。

---

## 1.2 三个核心项目

### P1：Mini-Redis

定位：

> Go 底层网络、协议、并发、数据结构和持久化训练项目。

主要训练：

* TCP；
* RESP；
* 字节流；
* goroutine；
* 锁；
* 分片存储；
* TTL；
* AOF；
* 并发测试；
* Benchmark。

---

### P2：Production ShortURL

定位：

> 完整、可部署的传统 Go 业务后端项目，也是简历主项目。

主要训练：

* HTTP API；
* 前后端联调；
* MySQL；
* Redis；
* 消息队列；
* 鉴权；
* 缓存治理；
* 测试；
* Docker 与 Docker Compose；
* CI/CD；
* 可观测性。

---

### P3：Agent Workflow Platform

定位：

> 使用 Go 构建可靠 AI Agent 应用和工作流平台的进阶项目。

主要训练：

* 一个模型 Provider 的可靠接入；
* SSE 流式输出；
* Tool Calling；
* 基础 RAG；
* 小型 Workflow 状态机；
* Human-in-the-loop；
* Agent Evaluation；
* 日志、指标和 Trace；
* Docker Compose；
* Python Agent 生态阅读与实验能力。

完成核心 MVP 后再按岗位和时间增加：

* MCP；
* 长期 Memory；
* 异步 Worker 与任务恢复；
* 多 Provider；
* gRPC；
* Kubernetes；
* 更复杂的工作流和微服务拆分。

---

## 1.3 三项目递进关系

```text
P1：理解 Go 如何处理网络、内存、并发和协议
                         ↓
P2：使用 Go 完成真实业务系统的完整工程闭环
                         ↓
P3：使用 Go 构建可靠、可评测、可观测的 AI Agent 应用平台 MVP
```

---

## 1.4 本路线的非目标

实习投递前不要求完成：

* Redis Cluster；
* Redis Sentinel；
* 完整主从复制；
* 完整 Raft 或 Paxos 实现；
* 自研 Kubernetes；
* Service Mesh；
* 大规模分库分表；
* 严格意义上的 Exactly-Once；
* 全链路 Zero-Allocation；
* 完整大模型训练；
* CUDA 编程；
* 自研向量数据库；
* 四个以上同等规模的大项目。

这些内容可以理解概念，但不应在当前阶段投入大量时间。

---

# 2. P0：Go 后端求职基础轨道

P0 不是第四个项目，而是与 P1—P3 全程并行的基础训练。项目不能替代算法、计算机基础、Linux 和数据库原理。

## 2.1 数据结构与算法

必须覆盖：

* 数组、字符串、链表；
* 栈、队列、哈希表；
* 二叉树、二叉搜索树、堆、Trie；
* 二分查找；
* 双指针、滑动窗口；
* DFS、BFS；
* 贪心、回溯；
* 基础动态规划；
* 常见排序算法；
* 时间复杂度和空间复杂度。

阶段目标：

* 累计完成约 100—150 道高频题；
* 核心题至少二刷；
* 能在没有 AI 直接给出答案的情况下独立分析并写出主要逻辑；
* 能解释复杂度、边界条件和替代方案；
* 每周持续练习，不因项目开发连续中断一个月。

## 2.2 计算机网络

必须掌握：

* TCP/IP 分层；
* 三次握手、四次挥手；
* TIME_WAIT、CLOSE_WAIT；
* TCP 流、粘包和半包；
* 阻塞 I/O、非阻塞 I/O 和 I/O 多路复用基础；
* HTTP/1.1、HTTP/2、HTTPS；
* Keep-Alive；
* DNS；
* Cookie、Session、JWT；
* SSE、WebSocket；
* RPC 和 gRPC 基础；
* 超时、重试、幂等之间的关系。

P1 负责把 TCP 和协议知识落地，P2 负责 HTTP，P3 负责 SSE 和可选 gRPC。

## 2.3 操作系统与 Linux

必须掌握：

* 进程、线程、协程；
* 用户态和内核态；
* 上下文切换；
* 虚拟内存；
* 文件描述符；
* 页缓存和文件 I/O 基础；
* 锁、死锁和竞态；
* epoll 基础；
* 信号；
* 进程退出和资源回收；
* 容器与 Namespace、Cgroup 的基础概念。

必须能够使用：

```bash
ps
top
htop
free
df
du
ss
lsof
curl
dig
ping
traceroute
grep
awk
sed
tail
journalctl
systemctl
```

需要能够排查：

* 端口占用；
* 进程异常；
* CPU 过高；
* 内存增长；
* 文件描述符耗尽；
* 服务无法连接；
* 日志错误；
* DNS 或网络超时。

## 2.4 数据库原理

必须掌握：

* B+ Tree；
* 主键和二级索引；
* 聚簇索引；
* 联合索引和最左匹配；
* 覆盖索引和回表；
* 事务 ACID；
* 隔离级别；
* MVCC；
* 行锁、间隙锁基础；
* WAL/Redo Log/Undo Log 的基本作用；
* 慢查询；
* `EXPLAIN`；
* 数据库连接池；
* 主从复制和读写分离的概念与限制。

P2 必须把这些知识落到表结构、SQL、事务和索引分析中。

## 2.5 Go 核心能力

必须持续学习：

* package、module 和可见性；
* array、slice、map、string、`[]byte`；
* 指针和方法集；
* interface；
* error wrapping 和错误分类；
* `defer`、`panic`、`recover`；
* goroutine、channel；
* Mutex、RWMutex、atomic；
* `context.Context`；
* `io.Reader`、`io.Writer`、`bufio`；
* `net`、`net/http`；
* `database/sql`；
* GMP；
* GC；
* 逃逸分析；
* testing、Benchmark、Race Detector；
* pprof。

学习 runtime 原理以能够解释和调试为目标，不要求当前阶段通读全部 runtime 源码。

## 2.6 Python 与 AI 基础轨道

Go 仍是主语言。Python 作为 P3 的辅助语言，目标是能够阅读和使用主流 Agent 生态，而不是转向模型算法岗位。

需要达到：

* 掌握 Python 基础语法；
* 使用虚拟环境和依赖管理；
* 调用 HTTP 和模型 API；
* 编写文档处理、Embedding 和评测脚本；
* 使用 Jupyter 分析评测结果；
* 阅读 LangChain、LangGraph 或同类框架示例；
* 完成一个小型 Python Agent/Workflow 实验；
* 理解 State、Node、Edge、Tool Calling、Checkpoint 和 Human-in-the-loop。

正式 P3 平台的核心后端、工作流执行、权限、存储、并发和可观测性仍优先使用 Go。

## 2.7 时间分配原则

默认长期比例：

```text
项目开发：55%
算法与计算机基础：30%
简历、文档、面试与复盘：15%
```

论文或其他任务较重时可以降低总时长，但不能完全停止 P0。

---

# 3. 全项目统一工程规范

以下规范从 P1 开始逐步引入。P2 必须完整执行核心规范；P3 按“核心 MVP / 增强项”分级执行。

---

## 3.1 开发优先级

所有项目统一遵循：

```text
正确性
→ 可读性
→ 可测试性
→ 可维护性
→ 性能
```

禁止在正确性和测试尚未完成时，提前追求：

* 无锁编程；
* `sync.Pool`；
* `unsafe`；
* 零拷贝；
* 零分配；
* 复杂泛型；
* 过度抽象；
* 自研框架。

所有性能结论必须由 Benchmark、压测或 Profile 支持。

---

## 3.2 Go 代码检查

每个明显阶段完成后必须运行：

```bash
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
```

根据项目情况增加：

```bash
go test -bench=. -benchmem ./...
```

P2 和 P3 可增加：

```bash
golangci-lint run
```

---

## 3.3 Git 开发流程

每个明显功能使用以下流程：

```text
Issue
→ 功能分支
→ 编码
→ 单元测试
→ 本地自测
→ Pull Request
→ Code Review
→ 修复
→ 合并
```

推荐提交信息：

```text
feat(resp): support RESP bulk string parsing
fix(engine): prevent expired keys from being returned
test(aof): add recovery test for partial command
refactor(server): separate client and server entrypoints
```

禁止长期使用：

```text
上传
更新内容
修改功能
一些改动
```

---

## 3.4 AI 分配任务的标准格式

AI 每次给用户安排开发任务时，必须包含：

1. 任务目标；
2. 为什么需要该任务；
3. 前置知识；
4. 输入；
5. 输出；
6. 推荐实现步骤；
7. 接口和数据结构约束；
8. 验收标准；
9. 必须编写的测试；
10. 常见错误；
11. 完成后用户必须回答的问题。

每次只安排一个可验证的任务，不得一次要求用户同时完成多个大型模块。

---

## 3.5 测试规范

项目测试至少包含：

* 单元测试；
* 表驱动测试；
* 边界测试；
* 错误路径测试；
* 并发测试；
* 集成测试；
* Race Detector；
* 必要时 Benchmark。

禁止只测试成功路径。

---

## 3.6 配置和密钥

* 配置通过环境变量或配置文件读取；
* 提供 `.env.example`；
* 不提交真实 `.env`；
* 密码、Token 和 API Key 不得写死在代码中；
* 日志不得直接输出：

  * 数据库密码；
  * JWT 密钥；
  * LLM API Key；
  * 用户敏感数据；
  * 完整私密 Prompt；
  * 敏感 Tool 参数。

---

## 3.7 外部请求规范

所有数据库、HTTP、RPC、MQ 和大模型调用必须考虑：

* 超时；
* Context 取消；
* 错误分类；
* 最大请求大小；
* 最大响应大小；
* 重试边界；
* 幂等；
* 资源释放；
* 服务关闭。

---

## 3.8 可观测性规范

P2 和 P3 必须包含：

### 日志

* 结构化日志；
* Request ID；
* Trace ID；
* 错误码；
* 耗时；
* 服务名；
* 关键资源 ID。

### Metrics

* 请求量；
* 错误率；
* P50、P95、P99；
* 数据库耗时；
* Redis 命中率；
* MQ 积压；
* Worker 数量；
* Goroutine 数量；
* 外部依赖错误率。

### Trace

使用 OpenTelemetry 或类似方案，追踪跨模块调用。

---

## 3.9 部署规范

### P1 必做

* Dockerfile；
* 配置；
* 优雅退出；
* AOF 持久化卷；
* GitHub Actions 执行测试。

P1 不要求 Kubernetes。

### P2 必做

* Docker Compose；
* GitHub Actions；
* 健康检查；
* 优雅停机；
* Prometheus Metrics；
* 配置与密钥分离；
* Linux 环境下能够启动和排查。

### P2 加分项

* Kubernetes Deployment；
* Service；
* ConfigMap；
* Secret；
* livenessProbe；
* readinessProbe；
* Requests/Limits；
* 滚动更新。

Kubernetes 不阻塞 P2 完成和投递。

### P3 核心 MVP 必做

* Docker Compose；
* API 服务；
* 数据库；
* Redis；
* 向量存储；
* Metrics 与 Trace；
* 如核心 MVP 暂不拆 Worker，则允许先使用模块化单体。

### P3 增强项

* API 和 Worker 独立部署；
* MQ；
* 可恢复任务；
* Kubernetes；
* Worker 独立扩缩容；
* Helm 或 HPA。

这些增强项必须在 P3 核心 MVP 完成后再决定是否实现。

---

# 4. P1：Mini-Redis

## 4.1 项目定位

使用 Go 从零实现一个兼容部分 RESP2 协议的单机并发内存数据库。

项目核心不是复刻完整 Redis，而是学习：

* TCP Socket；
* 流式协议解析；
* `[]byte`；
* 二进制安全；
* goroutine；
* Mutex 和 RWMutex；
* 分片锁；
* 内存数据结构；
* TTL；
* AOF；
* 错误处理；
* 并发测试；
* 性能分析。

---

## 4.2 最终项目描述

> 使用 Go 实现支持 RESP2 子集的单机内存键值数据库，提供字符串、列表、TTL 和基础 AOF 持久化；通过分片哈希表和读写锁保证并发安全，支持 TCP 长连接、连续命令解析、优雅停机、竞态检测和基准测试。

---

## 4.3 功能范围

### 必做命令

```text
PING
SET key value
GET key
DEL key [key ...]
EXPIRE key seconds
TTL key
LPUSH key value [value ...]
LPOP key
```

### 可选命令

```text
EXISTS key [key ...]
LRANGE key start stop
INCR
DECR
MGET
MSET
```

P1 不以命令数量为目标。必做命令、协议健壮性、并发安全、TTL、AOF、测试和可解释性完成后，应停止扩张并进入 P2。

### 明确不做

* Cluster；
* Sentinel；
* 主从复制；
* Lua；
* 完整事务；
* 完整 Pub/Sub；
* RDB；
* Redis 全部数据结构；
* Redis Module。

---

## 4.4 总体架构

```text
redis-cli / 自定义客户端
          │
          ▼
      TCP Server
          │
          ▼
  Connection Handler
          │
          ▼
      RESP Decoder
          │
          ▼
   Command Dispatcher
          │
          ▼
      Storage Engine
          │
    ┌─────┴─────┐
    ▼           ▼
String Value  List Value
          │
          ▼
      RESP Encoder
          │
          ▼
      TCP Response
```

写命令旁路：

```text
写命令执行
   │
   ├── 修改内存
   └── 追加 AOF
```

---

## 4.5 推荐目录结构

```text
mini-redis/
├── cmd/
│   ├── server/
│   │   └── main.go
│   └── client/
│       └── main.go
├── internal/
│   ├── server/
│   │   ├── server.go
│   │   └── handler.go
│   ├── resp/
│   │   ├── decoder.go
│   │   ├── encoder.go
│   │   ├── value.go
│   │   └── errors.go
│   ├── engine/
│   │   ├── engine.go
│   │   ├── shard.go
│   │   ├── command.go
│   │   ├── string.go
│   │   ├── list.go
│   │   ├── ttl.go
│   │   └── errors.go
│   └── persistence/
│       ├── aof.go
│       └── replay.go
├── tests/
├── Dockerfile
├── Makefile
├── README.md
└── go.mod
```

---

## 4.6 TCP Server

### 职责

* 监听 TCP 地址；
* 接受客户端连接；
* 为每个连接启动 goroutine；
* 把连接交给 Handler；
* 支持服务关闭；
* 处理 Listener 错误；
* 防止连接 goroutine 泄漏。

### 基本要求

* `ListenAndServe` 返回 `error`；
* 服务器保存 Listener；
* Handler 关闭当前连接；
* 支持优雅停机；
* 停止接收新连接后等待已有连接完成；
* 不忽略 `net.Listen`、`Accept` 和 `Write` 错误。

### 必须理解

* TCP 没有消息边界；
* 一次 `Read` 不一定是一个完整命令；
* 一个命令可能被拆成多次读取；
* 一次读取可能包含多个命令；
* `io.ReadFull` 解决什么问题；
* 正常 EOF 与协议错误的区别；
* goroutine 在什么情况下会泄漏。

---

## 4.7 RESP Decoder

### 请求需要支持

* Array；
* Bulk String。

### 响应需要支持

核心必做：

* Simple String；
* Error；
* Integer；
* Bulk String；
* Null Bulk String。

实现 LRANGE 等可选命令时再增加 Array 响应。

### 推荐接口

```go
type Decoder interface {
    Decode(reader *bufio.Reader) (Value, error)
}

type Encoder interface {
    Encode(value Value) ([]byte, error)
}
```

也可以先实现更直接的命令接口：

```go
ReadCommand(reader *bufio.Reader) ([][]byte, error)
```

### 必须处理

* 非法类型前缀；
* 非法数字；
* 负长度；
* 超大数组；
* 超大 Bulk String；
* 缺少 CRLF；
* 数据不完整；
* EOF；
* 连续命令；
* Bulk String 内包含 `\r\n`；
* 任意非法输入不得导致 panic。

### 输入限制

必须设置类似：

```text
MaxArrayLength = 1024
MaxBulkLength  = 16 MiB
```

具体数值可以调整，但必须防止恶意长度造成超大内存分配。

---

## 4.8 RESP Encoder

数据库层不得直接拼接 RESP 字节。

职责：

* Engine 负责执行命令；
* RESP 层负责协议编码。

Engine 可以返回：

```go
Exec(args [][]byte) (any, error)
```

或者返回明确的 Result 类型。

Encoder 将结果转换为：

```text
+OK\r\n
:1\r\n
$5\r\nhello\r\n
$-1\r\n
-ERR ...\r\n
```

---

## 4.9 Storage Engine

### 分片结构

```go
type shard struct {
    mu   sync.RWMutex
    data map[string]Entry
}

type Engine struct {
    shards []*shard
}
```

### Entry

推荐使用显式类型：

```go
type ValueType uint8

const (
    TypeString ValueType = iota + 1
    TypeList
)

type Entry struct {
    Type      ValueType
    Value     any
    ExpiresAt time.Time
}
```

TTL 也可以独立保存，但必须明确锁和一致性关系。

### 分片要求

* shard 数量使用 2 的幂；
* 根据 key 哈希定位 shard；
* 可以使用位掩码代替取模；
* 必须能解释：

  * 为什么位掩码可行；
  * 热点 key 为什么仍会竞争；
  * shard 越多为什么不一定越好；
  * 多 key 命令如何加锁；
  * 如何避免锁顺序死锁。

---

## 4.10 命令分发

命令名统一转换为大写：

```text
set
SET
Set
```

应被统一识别。

初期可以使用 `switch`，命令增加后可重构为：

```go
type CommandFunc func(e *Engine, args [][]byte) (any, error)
```

以及：

```go
map[string]CommandFunc
```

不要为了抽象而过早建立复杂命令框架。

---

## 4.11 String 数据

使用 `[]byte` 保存字符串，保持二进制安全。

需要明确：

* 是否复制客户端传入的切片；
* 返回数据时是否复制；
* 如何防止调用者修改内部数据；
* 内存复制和安全之间的权衡。

---

## 4.12 List 数据结构

可先手写双向链表。

需要处理：

* 空链表；
* 单节点；
* 多节点；
* 头插；
* 头删；
* 列表为空后删除 key；
* String 与 List 类型冲突；
* 多值 LPUSH 顺序；
* LRANGE 正负索引；
* 越界。

后续可以比较：

* 手写链表；
* `container/list`；
* 数组；
* 环形缓冲区。

---

## 4.13 TTL

必须实现两种策略。

### 惰性删除

访问 key 时检查过期时间：

```text
未过期 → 返回
已过期 → 删除并当作不存在
```

### 定期清理

后台 goroutine 定期扫描或抽样清理过期 key。

必须保证：

* 服务关闭时后台 goroutine 能退出；
* 不发生 goroutine 泄漏；
* 不长时间持有全局锁；
* 清理任务不会阻塞正常请求。

### 必须定义的语义

* key 不存在时 TTL 返回什么；
* key 没有设置过期时间时返回什么；
* key 已经过期时返回什么；
* SET 是否覆盖原有 TTL；
* List 操作是否保留 TTL；
* AOF 恢复后如何处理过期时间。

---

## 4.14 AOF

### MVP 功能

* 写命令成功后追加到 AOF；
* 使用 RESP 格式保存命令；
* 启动时顺序重放；
* 处理尾部不完整命令；
* AOF 写入失败不可静默忽略。

### 持久化策略范围

MVP 只需正确实现一种清晰、可解释的策略，例如：

```text
每条写命令追加到缓冲区，并按固定周期 Flush/Sync
```

可以研究但不要求完整实现：

```text
always
everysec
no
```

不得为了模仿 Redis 而在 P1 中实现复杂的 fsync 配置体系。

### 必须明确写入顺序

需要明确选择：

1. 先写 AOF，再更新内存；
2. 先更新内存，再写 AOF；
3. 通过统一写流程协调。

MVP 可以选择简单策略，但 README 必须说明：

* 崩溃窗口；
* 数据丢失风险；
* 内存和文件可能不一致的情况。

### 可选增强

* AOF Rewrite；
* 后台重写；
* 临时文件；
* 原子替换。

---

## 4.15 错误模型

至少定义：

```text
ErrUnknownCommand
ErrWrongArgumentCount
ErrWrongType
ErrProtocol
ErrTooLarge
ErrPersistence
```

RESP 层将内部错误转换为 Redis 风格错误响应。

不得把内部路径、堆栈或敏感信息直接返回客户端。

---

## 4.16 P1 测试要求

### RESP 测试

* 正常命令；
* 分片到达；
* 多命令连续输入；
* Bulk String 内包含 CRLF；
* 非法数组长度；
* 非法 Bulk 长度；
* 缺少 CRLF；
* 超大输入；
* EOF；
* 随机非法输入不 panic。

### Engine 测试

核心必做：

* SET/GET；
* DEL；
* 类型冲突；
* LPUSH/LPOP；
* TTL；
* 过期；
* 多 shard；
* 并发读写；
* 热点 key；
* Race Detector。

实现可选命令时再增加 EXISTS 和 LRANGE 测试。

### AOF 测试

* 正常恢复；
* 多命令恢复；
* TTL 恢复；
* 尾部半条命令；
* 写失败；
* 重启后一致性。

### Benchmark

核心 Benchmark 至少覆盖：

* 基础 SET/GET；
* 并发读多写少；
* 并发写；
* 热点 key；
* 分片存储的当前配置。

以下对照实验属于推荐增强，不阻塞 P1 完成：

* 单全局锁；
* 16 shard；
* 64 shard。

性能报告必须注明：

* CPU；
* 内存；
* Go 版本；
* 测试命令；
* key 数量；
* 并发数。

性能结论必须基于数据，不能仅凭“分片一定更快”之类的推测。

---

## 4.17 当前仓库优先修复项

现有 Mini-Redis 需要先处理：

1. `cmd/server` 目录存在两个 `main`；
2. 客户端移动到 `cmd/client`；
3. `NewRespEngine()` 与实际 Engine 构造函数不一致；
4. `CommandExecutor.Exec` 接口与 Engine 签名不一致；
5. 数据库层与 RESP 层职责混乱；
6. Parser 忽略多个读取错误；
7. Parser 忽略 `strconv.Atoi` 错误；
8. `net.Listen` 错误被忽略；
9. EOF 被当作协议错误返回；
10. 公开代码中的夸张式注释和 emoji 需要清理；
11. 文件名统一为 Go 常见小写格式；
12. 建立单元测试后再继续增加命令。

---

## 4.18 P1 里程碑

### M1：可编译骨架

* 服务端和客户端分离；
* TCP 连接可建立；
* 接口签名闭环；
* `go test ./...` 通过。

### M2：协议闭环

* PING；
* SET；
* GET；
* RESP Decoder；
* RESP Encoder；
* 连续命令；
* 错误输入测试。

### M3：存储能力

* 分片 Engine；
* DEL；
* List；
* 可选 EXISTS 或 LRANGE；
* 并发测试；
* Race Detector。

### M4：TTL 与 AOF

* EXPIRE；
* TTL；
* 惰性删除；
* 定期清理；
* AOF；
* 重启恢复。

### M5：工程化收尾

* Dockerfile；
* CI；
* README；
* 架构图；
* Benchmark；
* 面试问题整理；
* 停止继续扩展命令，准备进入 P2。

P1 的总开发周期建议控制在约 5—7 周。若超过该范围，指导 AI 应优先删减可选命令和增强型 AOF，而不是继续扩大范围。

---

## 4.19 P1 完成标准

达到以下标准后停止扩张：

* redis-cli 或自定义客户端能够稳定连接；
* 必做命令可用；
* RESP 能处理半包、连续命令、非法长度和异常 EOF；
* 非法协议不会 panic；
* 分片 Engine 不存在已知数据竞争；
* `go test -race ./...` 通过；
* 基础 AOF 能够恢复数据；
* 有基础 Benchmark 和测试环境说明；
* Dockerfile 和 CI 可用；
* README 能解释主要设计取舍；
* 用户能脱离代码讲清：

  * TCP；
  * RESP；
  * 分片锁；
  * TTL；
  * AOF；
  * goroutine 生命周期。

完成以上标准后直接进入 P2，不因缺少可选命令、AOF Rewrite 或 Kubernetes 推迟。

---

# 5. P2：Production ShortURL

## 5.1 项目定位

构建一个生产化短链接平台。

P2 负责覆盖 Go 后端实习中最常见的完整工作流程：

```text
需求分析
→ API 设计
→ 前后端联调
→ 数据库设计
→ 业务开发
→ Redis 缓存
→ 消息队列
→ 测试
→ 部署
→ 监控
→ 故障排查
```

P2 是三个项目中最重要的简历主项目。

---

## 5.2 最终项目描述

> 使用 Go 构建可部署的短链接平台，支持短码生成、跳转、链接管理、过期控制和访问统计；使用 MySQL 持久化、Redis 缓存热点链接、消息队列异步处理访问事件，并实现统一错误处理、鉴权、限流、优雅停机、Prometheus 指标、分布式追踪、Docker Compose 和 CI。Kubernetes 作为加分部署方案，不作为 P2 完成前提。

---

## 5.3 使用角色

### 普通用户

* 注册和登录；
* 创建短链接；
* 设置有效期；
* 查看链接列表；
* 禁用或启用链接；
* 删除链接；
* 查看访问统计。

### 访问者

* 通过短码访问原始地址；
* 无需登录；
* 系统记录访问事件。

### 管理员，可选

* 查看异常链接；
* 封禁恶意链接；
* 查看系统指标。

---

## 5.4 推荐技术栈

### 核心

* Go；
* Gin 作为实际业务框架；
* 理解 Gin 底层基于 `net/http`，核心 Middleware 和 Handler 机制不能只会套模板；
* MySQL；
* Redis；
* Kafka 或 RabbitMQ 二选一；
* Docker Compose；
* Prometheus；
* OpenTelemetry；
* GitHub Actions。

### 数据访问

可选：

* `database/sql` + 手写 SQL；
* sqlc；
* GORM。

建议：

* 先掌握 `database/sql`；
* SQL 必须自己理解；
* 可以使用 sqlc；
* 不允许完全依赖 ORM 掩盖 SQL 和索引设计。

### 前端

使用轻量 Vue、React 或简单管理页面。

前端不是主学习目标，重点是：

* API 契约；
* JSON；
* 鉴权；
* CORS；
* 分页；
* 错误码；
* 联调；
* 统计展示。

---

## 5.5 总体架构

```text
Browser / API Client
        │
        ▼
     HTTP API
        │
 ┌──────┼──────────┐
 ▼      ▼          ▼
Auth  ShortURL    Admin
        │
        ▼
 Application Service
        │
 ┌──────┼─────────────┐
 ▼      ▼             ▼
MySQL  Redis       Message Queue
                      │
                      ▼
               Analytics Consumer
                      │
                      ▼
                Stats Storage
```

跳转查询：

```text
GET /{code}
    │
    ▼
查询 Redis
    │ miss
    ▼
singleflight
    │
    ▼
查询 MySQL
    │
    ├── 回填 Redis
    └── 发送 VisitEvent 到 MQ
```

---

## 5.6 推荐目录结构

```text
shorturl/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── consumer/
│       └── main.go
├── internal/
│   ├── config/
│   ├── transport/http/
│   ├── application/
│   ├── domain/
│   ├── repository/
│   ├── cache/
│   ├── messaging/
│   ├── auth/
│   ├── observability/
│   └── platform/
├── migrations/
├── api/
│   └── openapi.yaml
├── web/
├── deploy/
│   ├── docker-compose.yml
│   └── k8s/
├── tests/
├── Dockerfile
├── Makefile
├── README.md
└── go.mod
```

不要为了模仿 DDD 建立大量没有实际作用的空层。

---

## 5.7 数据模型

### users

```text
id
email
password_hash
status
created_at
updated_at
```

### short_links

```text
id
owner_id
short_code
original_url
status
expires_at
created_at
updated_at
version
```

推荐索引：

```text
UNIQUE(short_code)
INDEX(owner_id, created_at)
INDEX(status, expires_at)
```

### visit_events

```text
id
event_id
short_link_id
visited_at
ip_hash
user_agent
referer
country
device
```

### daily_stats

```text
short_link_id
stat_date
visit_count
unique_visitor_count
```

---

## 5.8 API 设计

### 鉴权

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
```

### 短链接

```text
POST   /api/v1/links
GET    /api/v1/links
GET    /api/v1/links/{id}
PATCH  /api/v1/links/{id}
DELETE /api/v1/links/{id}
POST   /api/v1/links/{id}/disable
POST   /api/v1/links/{id}/enable
```

### 统计

```text
GET /api/v1/links/{id}/stats
```

### 跳转

```text
GET /{shortCode}
```

### 基础设施

```text
GET /health/live
GET /health/ready
GET /metrics
```

---

## 5.9 短码生成

可以依次研究：

1. 数据库自增 ID + Base62；
2. Snowflake + Base62；
3. 随机码 + 唯一索引冲突重试；
4. 号段模式。

建议：

* MVP 先使用数据库 ID + Base62；
* 后续再实现 Snowflake 作为扩展或对照；
* 不因 Snowflake 是面试关键词而强行使用。

必须比较：

* 唯一性；
* 可预测性；
* 分布式能力；
* 冲突处理；
* 短码长度；
* 数据库依赖；
* 性能。

---

## 5.10 MySQL 学习要求

必须掌握：

* 表设计；
* 数据类型；
* 主键；
* 唯一索引；
* 联合索引；
* 最左匹配；
* 覆盖索引；
* 回表；
* `EXPLAIN`；
* 事务；
* 隔离级别；
* MVCC；
* 乐观锁；
* 悲观锁；
* 连接池；
* 慢查询；
* 数据迁移。

必须明确：

* 创建短链接哪些操作需要事务；
* 短码冲突如何处理；
* 删除是软删除还是硬删除；
* 统计数据是否要求强一致；
* 连接池如何配置；
* Context 取消如何传递到 SQL。

### SQL 专项验收材料

P2 仓库必须提供：

* 表结构设计说明；
* 索引设计说明；
* 至少三条关键 SQL；
* 对关键 SQL 的 `EXPLAIN` 分析；
* 至少一个索引优化前后对比；
* 数据库连接池参数及理由；
* 至少一个事务场景的边界与失败处理说明。

用户必须能够解释 SQL 和索引，而不能只展示 ORM 调用。

---

## 5.11 Redis 缓存

### Cache Aside

跳转时：

1. 查询 Redis；
2. 未命中则查询 MySQL；
3. 回填 Redis；
4. 返回结果。

### 缓存穿透

处理方式：

* 参数校验；
* 空值缓存；
* 可选 Bloom Filter。

### 缓存击穿

处理方式：

* `singleflight`；
* 互斥重建；
* 热点预热。

### 缓存雪崩

处理方式：

* TTL 随机抖动；
* 限流；
* 降级；
* 分批过期。

### 缓存一致性

修改链接时：

1. 更新 MySQL；
2. 删除 Redis。

必须讨论：

* 删除缓存失败怎么办；
* 并发读写造成什么问题；
* 是否需要延迟双删；
* 系统允许多长的不一致窗口；
* 为什么不追求所有情况下强一致。

---

## 5.12 消息队列

Kafka 和 RabbitMQ 二选一，不同时使用。

### 使用场景

跳转成功后发送：

```text
VisitEvent
```

Consumer 负责：

* 批量写访问记录；
* 聚合每日统计；
* 更新报表。

### 必须处理

* 重复消息；
* 消费幂等；
* 消费失败；
* 重试；
* 死信；
* 消费积压；
* Producer 发送失败；
* 服务关闭时尚未发送完成的消息；
* 消息结构版本。

### 幂等方案

可使用：

* `event_id` 唯一索引；
* 数据库唯一约束；
* Redis 去重；
* 业务幂等键。

不得宣称严格 Exactly-Once。

---

## 5.13 HTTP 工程能力

必须掌握：

* Router；
* Middleware；
* JSON 请求和响应；
* 参数校验；
* 统一错误码；
* 统一响应格式；
* JWT；
* Refresh Token；
* CORS；
* Rate Limit；
* Request ID；
* Context；
* 超时；
* 优雅停机；
* Body 大小限制；
* 分页；
* 排序；
* 筛选；
* OpenAPI；
* 前后端联调。

---

## 5.14 鉴权与授权

MVP：

* 用户注册；
* 密码哈希；
* 登录；
* JWT Access Token；
* Refresh Token；
* 用户只能修改自己的短链接。

必须避免：

* 明文密码；
* JWT 密钥提交到 Git；
* 只依赖前端隐藏按钮；
* IDOR 越权；
* 登录接口无限制暴力尝试。

---

## 5.15 日志和可观测性

### 日志字段

```text
timestamp
level
service
request_id
trace_id
user_id
route
status
duration
error_code
```

### Metrics

* 请求数；
* 错误率；
* P50；
* P95；
* P99；
* 跳转 QPS；
* Redis 命中率；
* MySQL 延迟；
* MQ 发送失败；
* Consumer 积压；
* Goroutine 数量；
* 数据库连接池。

### Trace

一次跳转应能观察：

```text
HTTP
→ Redis
→ MySQL（缓存未命中时）
→ Redis Set
→ MQ Produce
```

### 故障排查文档

README 或 `docs/troubleshooting.md` 必须说明：

* Redis 不可用时系统行为；
* MySQL 连接池耗尽时的现象和排查方式；
* MQ 发送失败和积压如何定位；
* HTTP 请求超时如何定位；
* P99 延迟上升时的排查顺序；
* Goroutine 数量持续增长时如何判断泄漏；
* 如何通过日志、Metrics、Trace 和 Linux 命令交叉定位问题。

---

## 5.16 Docker Compose

必须能够一键启动：

* API；
* Consumer；
* MySQL；
* Redis；
* Kafka 或 RabbitMQ；
* Prometheus；
* Grafana，可选。

启动命令类似：

```bash
docker compose up
```

---

## 5.17 Kubernetes 加分项

Kubernetes 不属于 P2 核心完成条件。完成 Docker Compose、CI、健康检查、优雅停机、测试和监控后，时间允许再提供：

* Deployment；
* Service；
* ConfigMap；
* Secret；
* livenessProbe；
* readinessProbe；
* Resource Requests；
* Resource Limits；
* Rolling Update；
* API 多副本；
* Consumer 独立部署；
* 优雅停机。

暂时不要求 Kubernetes Operator、Helm 或复杂集群治理。即使未完成本节，只要 P2 核心完成标准满足，也应开始投递。

---

## 5.18 P2 测试要求

### 单元测试

* Base62；
* 短码生成；
* URL 校验；
* 权限判断；
* 缓存 Key；
* 业务错误。

### Repository 集成测试

* MySQL；
* Redis；
* 事务；
* 唯一键冲突；
* 过期链接；
* 软删除。

### HTTP 测试

* 注册；
* 登录；
* 创建；
* 查询；
* 更新；
* 删除；
* 越权；
* 参数错误；
* 跳转；
* 链接过期。

### MQ 测试

* 重复消息；
* 消费失败；
* 重试；
* 幂等；
* 积压。

### 压测

关注：

* 跳转 QPS；
* P95/P99；
* Redis 命中率；
* MySQL 压力；
* singleflight 效果；
* Consumer 吞吐。

---

## 5.19 P2 里程碑

### M1：业务 MVP

* 用户注册和登录；
* 创建短链接；
* 查询短链接；
* 跳转；
* MySQL；
* 基础 API。

### M2：缓存治理

* Redis；
* Cache Aside；
* singleflight；
* 过期；
* 更新后缓存失效。

### M3：访问统计

* MQ；
* Consumer；
* 幂等；
* 统计聚合；
* 统计查询。

### M4：工程化

* Docker Compose；
* OpenAPI；
* 日志；
* Metrics；
* Trace；
* CI；
* 集成测试。

### M5：部署增强与故障演练

核心必做：

* 健康检查；
* 优雅停机；
* 故障测试；
* Linux 环境启动与排查。

加分项：

* Kubernetes；
* 多副本；
* 资源限制；
* 滚动升级。

M5 加分项不阻塞投递。

---

## 5.20 P2 完成标准

* Docker Compose 一键启动；
* 管理页面或 API Client 可完整使用；
* 核心 API 有 OpenAPI 文档；
* 数据库结构、关键 SQL、索引和 `EXPLAIN` 能够解释；
* Redis 缓存设计经过测试；
* MQ 消费具备幂等；
* 单元测试和集成测试覆盖核心路径；
* Metrics 和 Trace 能展示；
* CI 通过；
* 有明确故障排查文档；
* Kubernetes 部署属于加分项，不属于硬性完成标准；
* 用户能回答：

  * 缓存一致性；
  * 索引；
  * 事务；
  * MQ；
  * Context；
  * 优雅停机；
  * JWT；
  * 数据库连接池；
  * Redis、MySQL 或 MQ 出现故障时系统如何退化和排查。

当 P1 已完成，且 P2 完成 M1、M2、核心测试、Docker Compose 和基本 README 后，应开始投递，不等待 P2 所有增强项或 P3 完成。

---

# 6. P3：Agent Workflow Platform

## 6.1 项目定位

构建一个以 Go 为核心后端、可评测、可观测、支持 Tool Calling 和基础 RAG 的 AI Agent 应用平台。

P3 的目标不是在几个月内复刻完整 Agent 中台，而是证明用户能够把概率性、不稳定的大模型能力纳入确定性的后端工程体系。

项目核心问题：

> 如何通过超时、取消、权限、状态机、评测和可观测性，让模型调用、检索和工具执行形成可控制、可分析、可复现的应用流程。

P3 不应只是：

* 调用一次 Chat API；
* 上传 PDF 后简单提问；
* 写几个 Prompt；
* 固定 `if/else` 冒充多 Agent；
* 完全依赖框架而解释不了 Agent Loop；
* 展示少量成功截图却没有评测；
* 一开始就拆成大量微服务。

---

## 6.2 范围分级

P3 必须严格区分核心 MVP 和增强项。

### 核心 MVP：必须完成

* 一个模型 Provider；
* 普通生成和 SSE 流式输出；
* 两个安全 Tool；
* 简单 Agent Loop；
* 基础 Workflow 状态机；
* `Start`、`Model`、`Tool`、`Retriever`、`Condition`、`HumanApproval`、`End` 节点；
* 基础 RAG；
* 引用来源；
* 一个暂停与人工批准流程；
* 20—50 条固定评测 Case；
* 成功率、Tool 正确率、延迟和 Token 指标；
* 日志、Prometheus Metrics 和 OpenTelemetry Trace；
* Docker Compose；
* 单元测试和集成测试；
* 简单前端控制台或可完整操作的 API Client。

### 增强项：核心 MVP 完成后按时间选择

* MCP Client 和自定义 MCP Server；
* 长期 Memory；
* 多 Provider 与故障切换；
* 异步 Worker；
* MQ；
* 租约、心跳和任务恢复；
* gRPC；
* 微服务拆分；
* Eino 等 Go Agent 框架；
* Kubernetes；
* Helm；
* HPA；
* 多 Agent；
* 复杂 DAG、并行节点和子工作流。

增强项不得成为 P3 核心 MVP 或投递的阻塞条件。

---

## 6.3 最终项目描述

### 核心 MVP 描述

> 使用 Go 构建研究助理型 AI Agent 平台，支持单模型 Provider、SSE 流式输出、Tool Calling、基础 RAG、工作流状态机、人工审批、固定测试集评测以及 OpenTelemetry 全链路追踪，并通过 Docker Compose 完成可复现部署。

### 完成增强项后的描述

仅在实际完成对应能力后，才可以增加：

> 支持 MCP、异步 Worker、任务 Checkpoint 与恢复、多模型路由、gRPC 服务拆分和 Kubernetes 部署。

不得在简历中写入尚未实现或无法解释的能力。

---

## 6.4 默认业务场景

推荐使用：

> **研究助理 Agent**

核心功能：

* 用户创建会话；
* 上传或导入研究资料；
* 文档解析和切分；
* 知识库检索；
* 调用文档搜索、时间查询或受限 HTTP 工具；
* 生成带引用的回答；
* 对敏感 Tool 请求人工审批；
* 使用固定测试集评测回答质量和工具使用；
* 查看执行节点、耗时、Token 和错误。

可选场景：

> 运维排障 Agent

运维排障场景风险和权限边界更复杂，不作为第一版默认场景。

---

## 6.5 核心技术栈

### Go 后端

* Go；
* Gin 或标准 `net/http`；
* PostgreSQL；
* pgvector；
* Redis，可用于会话缓存、限流或事件传递；
* SSE；
* Prometheus；
* OpenTelemetry；
* Docker Compose；
* GitHub Actions。

### Python 辅助能力

正式平台后端仍以 Go 为主，但必须完成一个独立的小型 Python Agent/Workflow 实验。

Python 用于：

* 文档处理实验；
* Embedding 和检索实验；
* 快速测试 Prompt；
* 编写离线评测脚本；
* 使用 Jupyter 分析评测结果；
* 阅读 LangChain、LangGraph 或同类框架示例。

至少理解：

* State；
* Node；
* Edge；
* Tool Calling；
* Checkpoint；
* Human-in-the-loop。

Python 实验用于补齐主流 Agent 生态认知，不替代 Go 平台项目。

### Agent 框架策略

第一阶段手写最小 Agent Loop 和 Tool Registry。

核心 MVP 完成后，可以使用 Eino 等 Go Agent 框架进行重构或对比，重点回答：

* 框架替用户解决了什么；
* 框架如何表示状态和节点；
* 框架如何传递 Context；
* 框架如何处理 Tool；
* 框架如何持久化 Checkpoint；
* 框架带来了哪些限制。

禁止从第一天开始只会拼框架配置。

---

## 6.6 核心 MVP 架构

第一版采用模块化单体，不拆微服务。

```text
Web Console / API Client
          │ HTTP / SSE
          ▼
    Agent API Service
          │
   ┌──────┼───────────────┐
   ▼      ▼               ▼
Session  Workflow       Evaluation
          │
   ┌──────┼───────────────┐
   ▼      ▼               ▼
Model   Tool Registry   Retriever
   │      │               │
   └──────┼───────────────┘
          ▼
 PostgreSQL + pgvector
          │
          ├── Workflow Run
          ├── Node Run
          ├── Conversation
          ├── Document/Chunk
          └── Evaluation Result
```

Redis 在核心 MVP 中可以用于：

* Rate Limit；
* 短期缓存；
* SSE 事件桥接；
* 会话临时状态。

若第一版没有真实需求，可以暂不用于关键状态持久化。

---

## 6.7 推荐目录结构

```text
agent-platform/
├── cmd/
│   └── api/
│       └── main.go
├── internal/
│   ├── agent/
│   ├── workflow/
│   ├── model/
│   ├── tool/
│   ├── retrieval/
│   ├── evaluation/
│   ├── conversation/
│   ├── repository/
│   ├── transport/http/
│   ├── observability/
│   ├── auth/
│   └── config/
├── migrations/
├── api/
│   └── openapi.yaml
├── eval/
│   ├── datasets/
│   └── scripts/
├── python-experiments/
├── web/
├── deploy/
│   └── docker-compose.yml
├── tests/
├── Dockerfile
├── Makefile
├── README.md
└── go.mod
```

增强阶段再增加：

```text
cmd/worker/
internal/queue/
internal/mcp/
internal/memory/
deploy/k8s/
```

---

## 6.8 核心领域模型

### Agent

```text
id
name
description
model_name
system_prompt
tool_policy
status
version
created_at
updated_at
```

### Workflow

```text
id
name
version
definition
status
created_at
updated_at
```

### Workflow Run

```text
id
workflow_id
agent_id
status
input
output
current_node
started_at
finished_at
error_code
version
```

### Node Run

```text
id
workflow_run_id
node_id
node_type
status
input
output
attempt
started_at
finished_at
error_code
```

### Conversation

```text
id
user_id
agent_id
title
created_at
updated_at
```

### Message

```text
id
conversation_id
role
content
token_usage
created_at
```

### Tool Call

```text
id
workflow_run_id
node_run_id
tool_name
arguments_summary
status
result_summary
started_at
finished_at
error_code
```

### Document

```text
id
owner_id
name
source
status
version
created_at
updated_at
```

### Chunk

```text
id
document_id
content
embedding
metadata
position
```

### Evaluation

```text
dataset
case
evaluation_run
case_result
metric
```

核心 MVP 不要求建立复杂多租户模型，但必须保证不同用户的文档和会话不能越权访问。

---

## 6.9 Model Provider 抽象

推荐接口：

```go
type Model interface {
    Generate(
        ctx context.Context,
        req GenerateRequest,
    ) (GenerateResponse, error)

    Stream(
        ctx context.Context,
        req GenerateRequest,
    ) (<-chan StreamEvent, error)
}
```

第一阶段只接入一个 OpenAI-compatible Provider 或其他稳定 Provider。

只有在接口稳定后，才增加第二个 Provider 验证抽象。

### 必须处理

* 超时；
* Context 取消；
* 流中断；
* Rate Limit；
* Provider 4xx/5xx；
* 请求重试边界；
* Token 使用；
* 请求大小限制；
* 响应大小限制；
* 敏感信息脱敏；
* 客户端断开后停止上游模型调用。

### 错误分类

```text
InvalidRequest
Unauthorized
RateLimited
Timeout
ProviderUnavailable
ContentRejected
StreamInterrupted
Unknown
```

### 重试规则

只对明确可重试错误重试，例如：

* 临时网络错误；
* Provider 429，并尊重 Retry-After；
* 部分 5xx。

不得对以下情况无脑重试：

* 参数错误；
* 鉴权失败；
* 内容安全拒绝；
* 已经产生不可逆副作用的 Tool；
* Context 已取消。

---

## 6.10 SSE 流式输出

前端可以接收：

```text
run_started
node_started
model_delta
tool_call_started
tool_call_finished
approval_required
node_finished
run_finished
run_failed
```

必须实现：

* `Content-Type: text/event-stream`；
* Flush；
* 心跳；
* Event ID；
* 客户端断开检测；
* Context 取消；
* 敏感数据过滤；
* 流中断错误处理；
* 前端能够展示部分结果和最终状态。

核心 MVP 不要求复杂断线续传。基础重连和最终状态查询接口即可。

---

## 6.11 Tool Calling

### Tool 接口

```go
type Tool interface {
    Name() string
    Description() string
    Schema() JSONSchema

    Execute(
        ctx context.Context,
        args json.RawMessage,
    ) (ToolResult, error)
}
```

### 核心 MVP 必须有两个 Tool

推荐：

* 文档检索 Tool；
* 时间查询 Tool；
* 受限 HTTP GET Tool；
* 简单计算 Tool。

从中选择两个。

第一版禁止：

* 任意 Shell；
* 任意 SQL；
* 任意文件写入；
* 无白名单的 URL 访问；
* 直接修改外部生产资源。

### Tool 必须支持

* JSON Schema；
* 参数校验；
* 超时；
* Context；
* 权限；
* 输入大小限制；
* 结果大小限制；
* 结构化错误；
* 日志；
* 敏感字段脱敏；
* Panic Recovery；
* 对副作用和幂等性的明确声明。

---

## 6.12 Agent Loop

第一版实现简单 Tool Agent：

```text
用户请求
→ 模型判断是否调用 Tool
→ 校验 Tool 与参数
→ 执行 Tool
→ 将 Tool 结果返回模型
→ 模型继续判断
→ 输出最终回答
```

必须限制：

* 最大迭代次数；
* 最大 Tool 调用次数；
* 最大 Token；
* 最大执行时间；
* 重复调用检测；
* Tool 白名单；
* 高风险 Tool 人工审批。

必须保存：

* 每轮模型调用摘要；
* Tool 名称；
* Tool 参数摘要；
* Tool 结果摘要；
* Token；
* 耗时；
* 错误。

不得保存不必要的完整敏感数据。

---

## 6.13 Workflow Engine

### 核心节点

```text
Start
Model
Tool
Retriever
Condition
HumanApproval
End
```

### 核心控制结构

* 顺序；
* 条件分支；
* 暂停；
* 恢复；
* 取消；
* 人工批准。

以下属于增强项：

* 并行；
* 任意 DAG；
* 复杂循环；
* 动态子工作流；
* 多 Agent。

### Run 状态

```text
Pending
Running
WaitingApproval
Succeeded
Failed
Cancelled
TimedOut
```

### Checkpoint

核心 MVP 至少在重要节点完成后持久化：

* 当前节点；
* 节点输入摘要；
* 节点输出摘要；
* 状态；
* 重试次数；
* 开始时间；
* 结束时间；
* 错误。

核心要求是人工审批后可以从当前节点继续，而不是重新执行全部前置节点。

服务崩溃后的自动任务接管属于增强阶段。

### 状态迁移

必须集中定义和测试，阻止非法迁移：

```text
Succeeded → Running
Cancelled → Succeeded
Failed → Running
```

只有明确 Retry 或 Resume 操作可以重新进入执行状态。

---

## 6.14 Human-in-the-loop

至少实现一个需要审批的 Tool 或 Workflow 节点。

工作流进入：

```text
WaitingApproval
```

前端展示：

* Tool 名称；
* 参数摘要；
* 风险说明；
* 预计影响；
* 批准；
* 拒绝。

批准后从 Checkpoint 继续。

拒绝后必须进入明确的结束或替代分支。

---

## 6.15 RAG

### 核心流程

```text
文档上传
→ 格式校验
→ 文本解析
→ 清洗
→ Chunk
→ Embedding
→ pgvector
→ Retrieval
→ Prompt Assembly
→ 生成
→ 引用来源
```

### 核心 MVP 必须研究

* Chunk 大小；
* Chunk overlap；
* Top-K；
* Metadata Filter；
* 无答案处理；
* 文档版本；
* 引用来源；
* 用户文档权限；
* Prompt Injection 基础防护。

### 增强项

* Query Rewrite；
* Rerank；
* Hybrid Search；
* 多阶段检索；
* 更复杂文档解析；
* 独立 Retrieval Service。

### 输出要求

最终回答应尽量包含：

* 来源文档；
* Chunk 或段落；
* 页面或位置信息；
* 引用；
* 无依据时明确说明无法确认。

不得把模型自身知识伪装为检索来源。

---

## 6.16 Agent Evaluation

评测是 P3 核心能力，不能删除。

### 固定测试集

准备 20—50 条 Case，每条包含：

```text
input
expected_behavior
required_tools
forbidden_tools
reference_answer
source_documents
```

### 核心指标

* 任务成功率；
* Tool 选择正确率；
* Tool 参数正确率；
* 引用正确率；
* 无答案识别率；
* 响应延迟；
* Token 使用；
* 失败原因分类。

### 对比实验

至少完成一组对比，例如：

* Prompt A 与 Prompt B；
* 不同 Top-K；
* 是否加入检索；
* 不同最大迭代次数。

评测结果必须：

* 持久化；
* 记录模型版本；
* 记录 Prompt 版本；
* 记录数据集版本；
* 可重复运行；
* 能生成简单报告。

LLM-as-a-Judge 可以作为辅助，但不能成为唯一评测依据。

---

## 6.17 可观测性

一次 Agent Run 应形成 Trace：

```text
HTTP Request
→ Workflow Run
→ Node
→ Retrieval
→ Model
→ Tool
→ Model
→ Response
```

### Metrics

* Run 成功率；
* Node 失败率；
* Provider 错误率；
* Provider 延迟；
* Tool 延迟；
* Tool 错误率；
* RAG 延迟；
* Token；
* 评测成功率；
* 当前执行 Run 数量；
* Goroutine 数量。

### 日志安全

不得直接记录：

* LLM API Key；
* 完整用户隐私；
* 未脱敏 Tool 参数；
* 完整私有文档；
* 不必要的完整 Prompt；
* 不必要的完整模型响应。

---

## 6.18 安全要求

必须考虑：

* Prompt Injection；
* Tool 越权；
* SSRF；
* SQL 注入；
* 文件上传风险；
* 恶意超大文档；
* 数据隔离；
* API Key 管理；
* 用户知识库权限；
* Tool 审计日志；
* 模型输出不可信；
* URL 白名单；
* 文件类型和大小限制；
* HTML 或 Markdown 输出注入。

Tool 默认最小权限。

任何模型输出在进入数据库、HTTP 请求、文件操作或其他 Tool 前必须经过结构化校验。

---

## 6.19 前端控制台

只需支持：

* 登录；
* 选择 Agent；
* 发起会话；
* SSE 流式回答；
* 查看 Tool 调用；
* 查看 Workflow 节点；
* 人工审批；
* 上传知识库文档；
* 查看评测结果；
* 查看 Token、耗时和错误。

前端不要求复杂视觉设计。重点是接口契约、SSE、状态展示和前后端联调。

---

## 6.20 Docker Compose

核心 MVP 必须能够启动：

* Agent API；
* PostgreSQL；
* pgvector；
* Redis，如实际使用；
* Prometheus；
* Jaeger 或 Tempo，可选；
* 简单前端，可选。

启动命令类似：

```bash
docker compose up
```

必须提供：

* `.env.example`；
* 数据库迁移；
* 健康检查；
* 初始化说明；
* 模型 Provider 配置说明；
* 不包含真实 API Key 的示例配置。

---

## 6.21 P3 核心测试要求

### Model Adapter

* 正常生成；
* 流式生成；
* 超时；
* Rate Limit；
* Provider 5xx；
* 流中断；
* Context 取消；
* Mock Provider。

### Tool

* Schema；
* 参数错误；
* 超时；
* 权限；
* 大结果；
* Panic；
* 敏感字段脱敏；
* 禁止访问非白名单 URL。

### Workflow

* 顺序执行；
* 条件分支；
* 失败；
* 取消；
* 人工审批；
* 审批后恢复；
* 非法状态迁移。

### RAG

* Chunk；
* Metadata Filter；
* Top-K；
* 引用；
* 无答案；
* 权限隔离；
* Prompt Injection 基础测试。

### Evaluation

* 数据集版本；
* 指标计算；
* 对比实验；
* 结果可复现；
* 失败分类。

### HTTP 与 SSE

* 请求校验；
* 鉴权；
* SSE 事件顺序；
* 客户端断开；
* Context 取消；
* 最终状态查询。

---

## 6.22 P3 核心里程碑

### M1：最小模型服务

* 一个 Provider；
* 普通生成；
* SSE；
* 会话记录；
* 超时；
* Context 取消；
* Mock Provider 测试。

### M2：Tool Agent

* Tool 接口；
* 两个安全 Tool；
* 最小 Agent Loop；
* 最大迭代限制；
* Tool 调用记录；
* Trace。

### M3：Workflow 与人工审批

* 核心节点；
* 状态机；
* Checkpoint；
* Condition；
* HumanApproval；
* 暂停与恢复。

### M4：RAG 与 Evaluation

* 文档上传；
* Chunk；
* Embedding；
* pgvector；
* Retrieval；
* 引用；
* 20—50 条评测 Case；
* 至少一组对比实验。

### M5：工程化收尾

* Docker Compose；
* OpenAPI；
* 日志；
* Metrics；
* Trace；
* CI；
* 单元测试；
* 集成测试；
* README；
* Python Agent 实验记录。

完成 M1—M5 即视为 P3 核心 MVP 完成。

---

## 6.23 P3 增强路线

增强路线只能在核心 MVP 完成后选择。

### E1：MCP

* MCP Client；
* 接入一个 MCP Server；
* 自己实现一个简单 MCP Server；
* MCP Tool 接入统一 Tool Registry；
* 白名单、超时、权限和审计。

必须理解：

> MCP 是工具与上下文接入协议，不等于 Agent 本身。

### E2：长期 Memory

区分：

* 短期上下文；
* 会话摘要；
* 长期用户事实；
* Workflow 状态；
* Tool 结果。

必须具备：

* 写入规则；
* 读取规则；
* Token 预算；
* 用户删除；
* 隐私控制；
* 过期策略；
* 数据隔离。

不能把全部历史消息直接塞进 Prompt 后称为长期记忆。

### E3：异步 Worker 与任务恢复

增加：

* MQ；
* Worker Pool；
* 任务状态；
* 指数退避；
* 幂等键；
* 租约；
* 心跳；
* 崩溃后重新领取；
* Checkpoint 恢复；
* 优雅停机。

执行语义最多宣称：

```text
At-least-once
```

通过业务幂等降低重复副作用，不宣称严格 Exactly-Once。

### E4：多 Provider

增加第二个 Provider，用于验证：

* 抽象是否稳定；
* 模型能力差异；
* 故障切换；
* 成本和延迟策略；
* Provider 特有参数如何隔离。

### E5：gRPC 与微服务

按真实边界拆分：

* API；
* Worker；
* Retrieval；
* Evaluation。

需要学习：

* protobuf；
* Unary RPC；
* Streaming RPC；
* Deadline；
* Metadata；
* Interceptor；
* 错误码；
* 连接复用；
* 协议兼容。

禁止为了简历关键词随意拆服务。

### E6：Kubernetes

只在明确投 AI 平台、云原生或基础设施岗位时增加：

* API Deployment；
* Worker Deployment；
* Service；
* ConfigMap；
* Secret；
* Probe；
* Requests/Limits；
* Worker 独立扩容；
* 滚动发布；
* Graceful Shutdown。

Helm、HPA 和 Operator 均为更后续的加分项。

---

## 6.24 P3 完成标准

核心 MVP 完成标准：

* 一个 Provider 可稳定工作；
* 支持普通和 SSE 流式响应；
* 客户端断开可以取消上游模型请求；
* 至少两个安全 Tool；
* Agent Loop 有迭代、时间和 Token 限制；
* 支持基础 Workflow；
* 支持一个人工审批节点；
* 审批后能够从 Checkpoint 继续；
* 支持基础 RAG；
* 回答能够提供引用；
* 有 20—50 条固定评测 Case；
* 至少完成一组可复现对比实验；
* 有日志、Metrics 和完整 Trace；
* Docker Compose 可以启动；
* CI 和核心测试通过；
* 有一个 Python Agent/Workflow 辅助实验；
* 用户能够解释：

  * Agent Loop；
  * Tool Calling；
  * RAG；
  * Workflow 状态机；
  * Checkpoint；
  * Human-in-the-loop；
  * Evaluation；
  * Prompt Injection；
  * 模型超时和重试边界；
  * 为什么核心 MVP 没有一开始拆微服务。

以下能力不属于核心完成标准：

* MCP；
* 长期 Memory；
* 多 Provider；
* 异步 Worker；
* 租约恢复；
* gRPC；
* Kubernetes；
* 多 Agent；
* 复杂 DAG。

完成核心 MVP 后，应优先投入简历、面试和投递，再根据目标岗位决定增强项。

---

# 7. 三项目能力覆盖矩阵

| 能力                  | P0          | P1    | P2  | P3 核心 MVP      |
| ------------------- | ----------- | ----- | --- | -------------- |
| 数据结构与算法             | 强           | 中     | 中   | 中              |
| 操作系统与 Linux         | 强           | 中     | 中强  | 中强             |
| Go 语法、接口、错误处理       | 强           | 强     | 强   | 强              |
| goroutine、锁、channel | 中强          | 强     | 中   | 中强             |
| TCP 和协议             | 中           | 强     | 弱   | 中              |
| HTTP API            | 中           | 无     | 强   | 强              |
| 前后端联调               | 弱           | 无     | 强   | 强              |
| MySQL/PostgreSQL    | 强原理         | 无     | 强   | 强              |
| Redis               | 中原理         | 原理与自研 | 强   | 中强             |
| 消息队列                | 基础概念        | 无     | 强   | 增强项            |
| gRPC                | 基础概念        | 无     | 可选  | 增强项            |
| Docker              | 基础          | 基础    | 强   | 强              |
| Kubernetes          | 基础概念        | 无     | 加分项 | 加分项            |
| 日志、Metrics、Trace    | 基础          | 基础    | 强   | 强              |
| CI/CD               | 基础          | 基础    | 强   | 强              |
| AI Agent            | Python 辅助基础 | 无     | 无   | 强              |
| RAG                 | Python 辅助实验 | 无     | 无   | 强              |
| MCP                 | 基础概念        | 无     | 无   | 增强项            |
| 性能分析                | Go 基础       | 强     | 中   | 中              |
| 可靠执行                | 基础概念        | 中     | 中   | 核心状态机；分布式恢复为增强 |
| 面试与算法               | 强           | 项目题   | 项目题 | 项目题            |

---

# 8. 推荐时间规划

时间规划以 2026 年 7 月下旬至 2027 年 3 月为基准。P0 在所有阶段持续进行。

## 2026 年 7 月下旬—8 月

主要任务：完成 P1。

* 修复现有仓库编译闭环；
* RESP；
* Engine；
* 必做命令；
* TTL；
* 基础 AOF；
* 测试；
* Race Detector；
* README；
* Benchmark；
* Dockerfile；
* CI。

P0 同步：

* Go 核心；
* TCP；
* 数据结构与算法；
* Linux 基础。

P1 建议总周期约 5—7 周。到达完成标准后停止扩展。

## 2026 年 9—10 月

主要任务：完成 P2 M1—M2。

* Gin 和 `net/http`；
* 用户注册、登录和鉴权；
* MySQL；
* 表结构、索引和事务；
* 创建短链接；
* 查询与跳转；
* Redis；
* Cache Aside；
* singleflight；
* 前后端联调；
* 单元测试；
* Docker Compose 基础。

P0 同步：

* MySQL 原理；
* HTTP；
* 高频算法；
* Linux 排查。

达到以下条件即可制作第一版简历并开始尝试投递：

* P1 完成；
* P2 核心 API 可运行；
* MySQL 和 Redis 已接入；
* 有核心测试；
* Docker Compose 可启动；
* README 可以展示。

## 2026 年 11—12 月

主要任务：完善 P2，并持续投递。

* MQ；
* Consumer；
* 访问统计；
* 消费幂等；
* OpenAPI；
* 集成测试；
* GitHub Actions；
* Metrics；
* Trace；
* 故障排查文档；
* 简单管理前端；
* Linux 部署和故障演练；
* Kubernetes 仅在时间允许时增加。

投递和面试：

* 持续更新简历；
* 根据面试反馈补 P0；
* 准备 P1、P2 的 3 分钟和 15 分钟讲解；
* 不因 P3 尚未开始而停止投递。

## 2027 年 1 月

主要任务：P3 M1—M2。

* Python Agent 小实验；
* 一个模型 Provider；
* 普通生成；
* SSE；
* Context 取消；
* Tool 接口；
* 两个安全 Tool；
* 最小 Agent Loop；
* Trace；
* 测试。

P0 同步：

* Go 并发；
* Context；
* SSE；
* AI Agent 基本概念；
* 算法持续练习。

## 2027 年 2 月

主要任务：P3 M3—M4。

* Workflow 状态机；
* Condition；
* HumanApproval；
* Checkpoint；
* 文档上传；
* Chunk；
* Embedding；
* pgvector；
* RAG；
* 引用；
* 固定评测集；
* 对比实验。

同时继续投递和模拟面试。

## 2027 年 3 月

主要任务：P3 M5 和面试冲刺。

* Docker Compose；
* CI；
* Metrics；
* Trace；
* 集成测试；
* README；
* 评测报告；
* 简历项目描述；
* 模拟面试；
* 算法二刷；
* Go、数据库、网络和操作系统复习；
* 持续投递；
* 不启动新的大型项目。

只有在 P3 核心 MVP 已完成且目标岗位明确需要时，才选择一个增强项：

* MCP；
* 异步 Worker；
* 多 Provider；
* gRPC；
* Kubernetes。

---

# 9. 投递与完成策略

## 9.1 最早投递门槛

不等待 2027 年 3 月。

达到以下条件即开始投递：

* P1 达到完成标准；
* P2 完成 M1 和 M2；
* P2 有基本测试；
* Docker Compose 可启动；
* GitHub README 可展示；
* P0 已持续推进；
* 用户能够讲清 P1 和 P2 的核心设计。

## 9.2 项目与投递并行

投递后继续：

* 根据 JD 调整简历关键词；
* 根据面试反馈修补 P0；
* 完善 P2 工程化；
* 推进 P3；
* 记录每次面试暴露的问题。

不得采用：

```text
三个项目全部完成
→ 才开始写简历
→ 才开始投递
```

## 9.3 岗位分流

### 普通 Go 后端

优先展示：

1. P2；
2. P1；
3. P3 核心 MVP。

### Go 基础架构

优先展示：

1. P1；
2. P3 中可靠执行与可观测性；
3. P2。

可以选择 Worker、gRPC 或 Kubernetes 增强项。

### AI 应用后端或 Agent 平台

优先展示：

1. P3；
2. P2；
3. P1。

可以选择 MCP、Python Agent 实验、多 Provider 或异步 Worker 增强项。

### 云原生研发

前三个项目只能证明容器化和基础部署能力，不能自动证明 Kubernetes 平台开发能力。

明确投 Operator、PaaS、容器调度等岗位时，再单独学习：

* CRD；
* controller-runtime；
* Informer；
* Reconcile Loop；
* CSI/CNI；
* 调度和资源管理。

不在当前核心路线中强制加入第四个大型项目。

---

# 10. AI 指导开发规范

此部分是提供给所有指导 AI 的强制要求。

## 10.1 AI 的角色

AI 是严格、客观的 Go 工程教练和项目顾问。

AI 的目标不是尽快替用户完成代码，而是让用户：

* 理解设计；
* 自己编写；
* 能够调试；
* 能够测试；
* 能够解释；
* 能够应对面试追问；
* 能够判断 AI 生成代码是否正确。

## 10.2 市场校准要求

在以下节点，指导 AI 应联网检查当时深圳及周边相关岗位：

* 开始 P2 前；
* 开始 P3 前；
* 制作正式简历前；
* 集中投递前；
* 用户考虑增加大型增强项时。

检查内容包括：

* 岗位名称；
* 公司；
* 发布时间；
* Go、MySQL、Redis、MQ、Docker、Kubernetes、Agent、RAG、MCP、Python 等要求；
* 实习时长和到岗要求；
* 普通后端、基础架构和 AI 后端的差异。

不得仅凭训练知识声称“已经检索岗位”。

## 10.3 范围控制

AI 每次必须明确当前任务属于：

```text
核心必做
推荐增强
可选探索
当前不做
```

核心里程碑未完成前，不得安排增强项。

P1、P2、P3 的默认优先级：

```text
P1：完整且克制
P2：最完整且最深入
P3：先完成核心 MVP
```

## 10.4 代码生成边界

默认：

* 不一次性生成完整大型模块；
* 可以提供：

  * 接口；
  * 伪代码；
  * 小型示例；
  * 测试骨架；
  * 局部实现建议。
* 用户应自己完成主要代码。

当用户明确要求 Codex 落地完整工程任务时，可以生成完整补丁，但必须同时提供：

* 修改范围；
* 设计理由；
* 测试；
* 风险；
* 验收方式；
* 用户需要理解的核心内容。

不得让用户在不理解的情况下复制大量代码。

## 10.5 AI 分配任务的标准格式

每次开发任务必须包含：

1. 当前项目和里程碑；
2. 任务级别：核心、增强、探索或当前不做；
3. 任务目标；
4. 为什么需要；
5. 前置知识；
6. 输入；
7. 输出；
8. 接口与数据结构约束；
9. 推荐步骤；
10. 必须测试；
11. 验收标准；
12. 常见错误；
13. 完成后用户必须回答的问题。

每次只安排一个可验证任务，不得一次要求用户同时完成多个大型模块。

## 10.6 每次指导流程

```text
读取仓库和项目文档
→ 确认当前分支与里程碑
→ 检查上一任务是否通过验收
→ 安排一个可验证任务
→ 讲清核心知识
→ 用户编码或 Codex 落地
→ AI Code Review
→ 用户修复
→ 运行测试
→ 提交
→ 更新项目状态
→ 下一任务
```

## 10.7 Code Review 检查项

AI 必须检查：

* 逻辑正确性；
* 边界条件；
* 错误处理；
* 并发安全；
* goroutine 泄漏；
* Context 传递；
* 资源关闭；
* 接口设计；
* 包职责；
* 命名；
* 测试；
* 性能；
* Go idiom；
* 安全；
* 日志是否泄露敏感信息；
* 是否过度设计；
* 是否引入无必要组件；
* 是否存在明显复制生成但用户无法解释的代码。

## 10.8 禁止的指导方式

* 一次性写完整个项目；
* 只讲怎么写，不讲为什么；
* 对明显错误迎合；
* 未测试就继续增加功能；
* 为堆技术栈而增加组件；
* 把 Demo 包装成生产系统；
* 编造 QPS、延迟或性能结论；
* 使用用户尚未理解的复杂抽象；
* 同时推进三个项目，导致全部成为半成品；
* 为了维持原计划而拒绝更合理的替代方案；
* P3 核心 MVP 未完成就要求多 Agent、微服务或 Kubernetes；
* 用“零分配”“高并发”等词汇代替真实 Profile 和 Benchmark；
* 未实际联网却声称已经完成岗位检索。

## 10.9 AI 应主动调整方案的情况

出现以下情况时，AI 应直接提出删减、替换或重构：

* 功能和求职价值不匹配；
* 与其他项目重复；
* 时间成本过高；
* 用户当前基础不足；
* 技术只用于堆关键词；
* 设计难以测试；
* 架构复杂度远超业务需求；
* 招聘市场要求发生变化；
* 项目无法在 2027 年 3 月前形成完整成果；
* 项目开发明显挤压算法和计算机基础；
* 用户已经达到投递门槛，却因可选功能迟迟不投递。

---

# 11. 最终作品集目标

## P1：Mini-Redis

证明用户具备：

* Go 底层网络能力；
* 并发能力；
* 协议处理能力；
* 数据结构能力；
* 持久化基础；
* 并发测试和性能意识。

P1 的价值来自正确性、协议健壮性、竞态测试和设计解释，不来自命令数量。

## P2：Production ShortURL

证明用户具备：

* 完整业务后端开发能力；
* MySQL、SQL、索引和事务能力；
* Redis 缓存能力；
* 消息队列能力；
* 前后端协作能力；
* 测试、部署、监控和排障能力。

P2 是普通 Go 后端求职的简历主项目。

## P3：Agent Workflow Platform

证明用户具备：

* Go + AI Agent 后端能力；
* Tool Calling；
* RAG；
* Workflow 状态机；
* Human-in-the-loop；
* 模型评测；
* 可观测性；
* Python Agent 生态阅读与实验能力。

MCP、异步 Worker、多 Provider、gRPC 和 Kubernetes 只有实际完成后才作为增强能力展示。

## P0：求职基础

证明用户具备：

* 数据结构与算法；
* 操作系统；
* 计算机网络；
* 数据库原理；
* Linux；
* Go 基础；
* 持续学习和面试准备能力。

P0 不直接作为项目写入简历，但决定能否通过笔试和面试。

---

# 12. 总体完成定义

整个路线达到成熟状态后，用户应能做到：

1. 不依赖框架解释 TCP、HTTP、RESP、SSE 和网络 I/O；
2. 独立设计和实现 Go API；
3. 正确使用 MySQL、Redis 和一种消息队列；
4. 解释关键 SQL、索引、事务和连接池；
5. 编写单元测试、集成测试和并发测试；
6. 正确使用 Context、优雅停机和错误处理；
7. 使用 Docker Compose 部署服务；
8. 理解 Kubernetes 基础，并在目标岗位需要时完成加分部署；
9. 使用日志、Metrics、Trace 和 Linux 命令排查问题；
10. 解释 Agent Loop、Tool Calling、RAG、Workflow、Checkpoint、Human-in-the-loop 和 Evaluation；
11. 阅读 Python Agent 框架示例并完成一个小型实验；
12. 解释幂等、重试、缓存一致性和 At-least-once；
13. 分别用 3 分钟和 15 分钟讲清每个项目；
14. 面对项目追问时能够展示代码、测试、架构图、SQL 分析和性能数据；
15. 明确项目限制，不夸大生产能力；
16. 在项目尚未全部结束时，已经持续投递并根据反馈迭代。

---

# 13. 一句话总路线

```text
P0 持续补齐算法、计算机基础、Linux 和 Python Agent 辅助能力；
P1 用 Go 理解底层网络、协议和并发；
P2 用 Go 完成真实业务后端工程闭环；
P3 用 Go 构建可靠、可评测、可观测的 AI Agent 平台 MVP；
Git、测试、Docker、CI/CD 和可观测性贯穿全部项目；
Kubernetes、MCP、分布式 Worker 和微服务按岗位需要选做。
```
