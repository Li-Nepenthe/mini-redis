# 第 0 章：先知道自己在解决什么

本书解释第一阶段实际代码。第二阶段保持暂停。这是**模拟从零构建的教学顺序**：为了理解依赖，先做能检查的小实验，再阅读最终实现。它不是 Git 提交历史，也不声称早期仓库没有后来发现的修复。

## 0.1 一个具体需求

假设两个程序要共享一个小状态：程序 A 保存 `user:7 → Alice`，程序 B 随后读取它。只在 A 进程里建 map，B 看不到；写普通文件，每次读取/解析整文件又难以支持并发、类型和过期。我们选择单独的服务进程：客户端连接，服务解释命令、持有内存、返回结果。内存让常见读取快，日志使重启能恢复已确认写。

本项目的 String 是二进制字节，不是只能存英文的文本；List 是有顺序的字节值集合。`LPUSH jobs a b` 得到 `[b,a]`，`LPOP jobs` 得到 `b`。过期可以表达“这个状态只保留五秒”。这些是学习载体，实际可交付仅 RESP2 子集：十条公开命令，没有事务、复制、Rewrite、其他数据结构或客户端增强。

学习价值在边界：网络给的是字节而非完整命令；并发不能任意读写 map；文件写成功不等于已同步；取消不等于工作结束；一组性能数字必须带负载条件。能解释这些约束，才有能力判断一个后端的失败方式，命令数不代表理解深度。

## 0.2 先区分三类知识

| 类别 | 含义 | 本书例子 |
|---|---|---|
| Go 基础 | 语言/标准库的规则 | slice 可能共享底层数组；接口按方法集隐式实现；defer 后进先出 |
| 协议规则 | 发送方/接收方的字节约定 | 数组 `*`、bulk `$`、整数 `:`、CRLF 和字节长度 |
| 项目约束 | 为这份实现选择的边界 | 1024 参数、16MiB bulk、64 字节头、32MiB 帧；16 分片；每写 Sync |

不要把“默认 16 分片”说成 Go 的要求，也不要把 32MiB 限额说成全部 Redis 的标准。源码和测试说明本项目选择；标准库说明语言行为。

## 0.3 最少前置知识，配当前代码理解

**包（package）与模块（module）**：目录里的 Go 文件组成包；模块是 `go.mod` 规定的导入路径/版本单位。根模块为 `github.com/Li-Nepenthe/mini-redis`；`ai-backend` 是另一个暂停模块。`go test ./...` 从根运行不会自动跨入它。`cmd/server` 与 `cmd/client` 都写 `package main`，因为它们各自是可执行程序，目录不同并不冲突。

**值、指针、slice、map**：`*Engine` 指向共享引擎；`[]byte` 是描述底层字节数组的切片，复制切片描述符通常不复制字节。`[][]byte` 是“多个参数，每个参数是一段字节”，例如 `[SET,k,v]`。map 做 key 查找，`map[string]any` 的 value 可能是 String 字节或 `*LinkedList`；`any` 是 `interface{}` 的别名，不表示自动知道类型。

`value.([]byte)` 是类型断言。双返回写法 `v, ok := value.([]byte)` 失败得到 `ok=false`；单返回断言失败会 panic。本项目先判断 ok，再返回 WRONGTYPE。大写首字母导出供其他包调用；小写函数也有用途与锁前提，不能因为未导出就省略说明。

**方法与接口**：`func (e *Engine) Exec(...)` 的 e 是接收者，把方法关联到引擎；接口列出需要的方法，不要求 `implements` 关键字。Engine 有符合签名的 Exec 就能作为 `resp.CommandExecutor`。协议层因此可以注入慢执行器来测停机，而不复制整个数据库。依据见 [Go 官方接口说明](https://go.dev/doc/effective_go#interfaces)。

**error、panic、错误链**：正常可预见失败通过返回 error 传给调用者；panic 会中断正常调用流程，不能把网络坏参数作为 panic 处理。`NewEngine(3)` 的 panic 是程序配置错误，Parser 的非法长度则是可返回的协议错误。`fmt.Errorf("位置: %w", err)` 保留原因链，`errors.Is` 判断错误身份，`errors.As` 找指定类型/接口；相同显示文字不代表相同错误。`commandError` 将 cause 与公开字段分开。

**goroutine、channel、context**：goroutine 是 Go 调度的并发执行任务，不是每个都独占一个 OS 线程。channel 交接值；无缓冲发送通常等接收者。context 传取消/期限，不能杀任意函数，也不负责等待。Parser 可能卡在 reader.Read，因此还需关闭 reader。取消函数本身不等待工作结束，见 [官方 CancelFunc](https://pkg.go.dev/context#CancelFunc)。

**mutex、RWMutex、WaitGroup、Once**：锁让多个任务访问同一状态时遵守互斥；RWMutex 允许多个读者，写者独占。它不能从持有读锁直接升级为写锁。WaitGroup 是未完成任务计数器：启动前 Add，完成 Done，Wait 等到零；不自动取消。Once 只执行一次关闭流程，不会自动释放资源或重试失败。具体锁规则见 [官方 RWMutex](https://pkg.go.dev/sync#RWMutex)。

**闭包与 defer**：`prepareWrite` 返回的 apply 捕获 List/分片状态，只有调用它才修改业务值。`defer unlock()` 推迟释放锁，不是马上解锁；同函数多个 defer 后注册先执行。`run` 借此先取消并等待清理，再关日志。

**阻塞、短读、短写、EOF**：阻塞是等待条件；一次 Read 可只给半段，一次 Write 也可能没写完。EOF 在没有新帧字节时表示干净结束；一帧写到一半断开是错误。`io.ReadFull` 才按指定长度读满或报不足，见 [官方 ReadFull](https://pkg.go.dev/io#ReadFull)。

**测试词汇**：表驱动把输入/期望列成表；`t.Run` 让每项有名字。`t.Fatal` 结束当前测试 goroutine，`t.Error` 记录失败后继续；`t.Helper` 改善失败定位，`t.Cleanup` 注册测试退出清理。fuzz 变异输入，benchmark 测受控成本，race 检查已执行共享访问。三者不能相互代替，见 [官方 testing](https://pkg.go.dev/testing)。

## 0.4 全局结构先画成一条请求

```text
客户端字节
  → tcp.Server.Serve 接收连接
  → RespHandler.Handle 接管连接
  → Parser.ParseStream → readRequest → Payload
  → Engine.Exec（业务参数/类型）
  → executeWrite → prepareWrite（持分片锁）
  → Store.Append（若启用 AOF，完整 Write+Sync）
  → apply（内存提交）
  → EncodeReply → writeReply → 客户端
```

读命令 GET 不走 Append，走 `get→lockRead→bytes.Clone`。后台清理不是另一种客户端，它访问同一分片状态但不逐条记录过期删除。AOF 恢复使用同一个 Parser，但回调是 `Replay`，不会再次 Append。

## 0.5 包职责与阅读入口

| 包 | 职责/依赖 | 不负责的事 |
|---|---|---|
| [database](https://github.com/Li-Nepenthe/mini-redis/tree/main/database) | 内存、命令、分片/TTL；标准库、CommandLog 接口 | 网络与 RESP 字节 |
| [resp](https://github.com/Li-Nepenthe/mini-redis/tree/main/resp) | 解析、编码、连接处理；通过接口调用执行器 | 持有数据库或文件 |
| [tcp](https://github.com/Li-Nepenthe/mini-redis/tree/main/tcp) | 监听/Accept/任务/停机；Handler 接口 | 判断 SET 参数 |
| [aof](https://github.com/Li-Nepenthe/mini-redis/tree/main/aof) | 独占文件/同步/恢复；resp 与 replay 回调 | Rewrite 与具体数据结构 |
| [cmd/server](https://github.com/Li-Nepenthe/mini-redis/tree/main/cmd/server) | 组装四包、信号与资源生命周期 | P2 HTTP/AI |
| [cmd/client](https://github.com/Li-Nepenthe/mini-redis/tree/main/cmd/client) | 简易固定端口交互演示 | 完整数组/引号/流水线客户端 |
| tcp_test | 外部公共 API 集成测试，依赖前三核心包 | 生产服务包 |

每个包现在有包级说明，每个具名函数都有独立契约；完整函数清单见[注释覆盖审计](https://github.com/Li-Nepenthe/mini-redis/blob/main/docs/comment-coverage.md)。逐函数注释补齐不替代教程：注释解释局部承诺，教程解释为什么要连接这些局部。

下一章只做环境和第一个可验证小程序，再进入协议。自测 Q01、Q03、Q32 的完整参考答案在[第 9 章](09-answers.md)。
