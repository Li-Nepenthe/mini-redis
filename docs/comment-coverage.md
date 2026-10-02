# P1 全包、全函数注释覆盖审计

基线 main@4ae13a7。使用 Go 标准库 go/parser、go/ast 遍历 Git 清单中的全部 P1 Go 文件，包含未导出函数、Linux/Windows 平台文件、所有测试/助手、fuzz 和 benchmark。最终统计不依赖当前平台 go list 的构建过滤。

| 项目 | 补齐前 | 补齐后 |
|---|---:|---:|
| 包 | 7 | 7 |
| 具名函数/方法 FuncDecl | 152 | 152 |
| 生产函数/方法 | 61 | 61 |
| 测试/基准/fuzz及辅助函数/方法 | 91 | 91 |
| 缺包级用途/依赖/边界说明 | 7 | 0 |
| 缺函数文档注释 | 133 | 0 |
| 缺少或不足中文解释候选 | 133 | 0 |
| 命名接口方法/嵌入契约（另计，不混入FuncDecl） | 11 | 11 |
| 缺接口契约 | 11 | 0 |

补齐后包含 40 个 Go 文件，原 33 个文件全部非注释代码 token 与基线一致。新增7个文件仅包注释与package声明；未新增功能或修改P2。旧有19份函数注释也重写为完整契约，不以已有WHY片段代替用途/参数/结果。

**质量边界**：AST检查Doc存在、含中文且非短名称翻译，只能筛候选。真正语义验收由独立逐函数对照实现，检查用途、输入/返回、错误/副作用、锁/资源前提、复杂不变量；结论与适用测试见docs/notes.md本轮记录。0形式缺漏不是0行为风险或本人已学会。

## 完整包范围

| 目录:包 | 函数数 | 包说明 |
|---|---:|---|
| aof:aof | 24 | [aof/doc.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/doc.go) |
| cmd/client:main | 1 | [cmd/client/doc.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/client/doc.go) |
| cmd/server:main | 5 | [cmd/server/doc.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/server/doc.go) |
| database:database | 55 | [database/doc.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/doc.go) |
| resp:resp | 39 | [resp/doc.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/doc.go) |
| tcp:tcp | 6 | [tcp/doc.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/doc.go) |
| tcp:tcp_test | 22 | [tcp/doc_external_test.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/doc_external_test.go) |

## 明确排除项

ai-backend/ 是用户明确暂停的第二阶段独立Go module，以下文件未参与P1计数或修改：

- ai-backend/cmd/api/main.go
- ai-backend/internal/auth/auth.go
- ai-backend/internal/auth/auth_test.go
- ai-backend/internal/config/config.go
- ai-backend/internal/domain/model.go
- ai-backend/internal/httpapi/api.go
- ai-backend/internal/httpapi/api_test.go
- ai-backend/internal/store/integration_test.go
- ai-backend/internal/store/store.go

P1 Git清单中没有vendor第三方源码或带Go生成标记的文件，故这两类排除数均为0。导入的Go标准库不是本仓库源码。没有省略P1子包；tcp_test虽只用于外部测试也单列。Markdown中的教学片段属于文档实验，另编译/运行，不冒充生产函数。

## 全部152个具名函数与方法

每行对应AST FuncDecl，链接定位补齐后源码。下列用途只作索引，参数/结果/错误/副作用与WHY在函数上方完整注释。

| 位置 | 名称 | 用途索引 |
|---|---|---|
| [aof/lock_linux.go:10](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/lock_linux.go#L10) | lockFile | lockFile 对已打开的 file 获取 Linux 非阻塞独占 flock，返回系统锁错误或 nil。 |
| [aof/lock_windows.go:13](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/lock_windows.go#L13) | lockFile | lockFile 对已打开的 file 调用 Windows LockFileEx，非阻塞地独占整段文件范围。 |
| [aof/store.go:39](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store.go#L39) | Open | Open 创建或打开 path 指定的普通 AOF 文件、独占加锁，并依次把完整记录交给 replay 恢复内存。 |
| [aof/store.go:113](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store.go#L113) | encodeRequest | encodeRequest 把 args 的每个二进制参数编码为 RESP 数组中的 bulk，返回独立记录字节或边界错误。 |
| [aof/store.go:137](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store.go#L137) | *Store.Append | Append 将 args 编码后完整写入 AOF 并 Sync；nil 表示该记录已经通过本次同步确认。 |
| [aof/store.go:170](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store.go#L170) | *Store.fail | fail 在调用者持有 Store.mu 时处理 cause：尽力 Truncate 到确认的 size 并再次 Sync，保存合并错误并写内部日志。 |
| [aof/store.go:181](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store.go#L181) | *Store.Close | Close 在 Store.mu 下将存储标记关闭、必要时 Sync 并关闭文件，返回持久化/关闭的合并错误。 |
| [aof/store_test.go:24](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L24) | args | args 将 parts 的每个字符串转为独立字节参数并返回二维切片，便于测试直接调用 Engine/Store。 |
| [aof/store_test.go:34](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L34) | execute | execute 用 parts 执行 engine 命令，并由 t 检查无错误且结果与 want 深度相等，无返回值。 |
| [aof/store_test.go:44](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L44) | openEngine | openEngine 为 path 创建 16 分片引擎、通过 Replay 恢复并绑定 Store，返回引擎与日志。 |
| [aof/store_test.go:62](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L62) | TestReplayStringListDeleteAndTTL | TestReplayStringListDeleteAndTTL 验证 String 二进制值、DEL、List 顺序/弹出和绝对 TTL 在真实临时 AOF 重开后恢复。 |
| [aof/store_test.go:93](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L93) | TestExpiredListWithLaterPushDoesNotResurrectOnReopen | TestExpiredListWithLaterPushDoesNotResurrectOnReopen 验证期限内追加的 List 在整体过期后重开 AOF 不会丢 TTL 而复活。 |
| [aof/store_test.go:111](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L111) | TestMaximumSizedLPUSHFitsPersistedCreationRecord | TestMaximumSizedLPUSHFitsPersistedCreationRecord 构造恰好 32MiB 的合法 LPUSH，验证新建 _LNEW 落盘不扩大帧预算。 |
| [aof/store_test.go:134](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L134) | TestRecoverOnlyIncompleteTailWithActualOffsets | TestRecoverOnlyIncompleteTailWithActualOffsets 验证非规范数字头的完整前缀按真实字节保留，仅截断各种不完整尾部。 |
| [aof/store_test.go:162](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L162) | TestRejectCorruptOrInvalidCompleteRecordsWithoutTruncating | TestRejectCorruptOrInvalidCompleteRecordsWithoutTruncating 验证完整非法 RESP/非持久化命令使 Open 失败且原文件字节不被截断。 |
| [aof/store_test.go:182](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L182) | TestAOFExclusiveLock | TestAOFExclusiveLock 验证同一路径同时只能打开一个 Store，Close 后锁释放可以重新打开。 |
| [aof/store_test.go:197](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L197) | TestConcurrentWriteOrderMatchesReplay | TestConcurrentWriteOrderMatchesReplay 让 8 个 worker 并发写共享 String/List，比较内存最终值与真实 AOF 重开后的状态。 |
| [aof/store_test.go:236](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L236) | *faultFile.Write | Write 记录调用次数，正常返回 len(data)、nil；设置 writeErr 时返回半段长度与注入错误。 |
| [aof/store_test.go:246](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L246) | *faultFile.Sync | Sync 增加模拟同步次数并返回配置的 syncErr，供测试观察刷盘失败及回滚再次同步。 |
| [aof/store_test.go:250](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L250) | *faultFile.Truncate | Truncate 记录请求的 n 为 truncated 并返回 nil，用于验证回滚目标是原确认前缀。 |
| [aof/store_test.go:254](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L254) | *faultFile.Close | Close 将模拟器的 closed 标记为 true 并返回 nil，供断言失败状态也必须回收句柄。 |
| [aof/store_test.go:258](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L258) | TestWriteAndSyncFailuresAreSticky | TestWriteAndSyncFailuresAreSticky 分别注入部分写和同步失败，验证回滚到 size、后续拒写及 Close 保留原 cause。 |
| [aof/store_test.go:280](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L280) | TestCrashProcessHelper | TestCrashProcessHelper 仅在 MINIREDIS_AOF_CHILD=1 的自有子进程中写 1000 条并输出 READY，然后保持运行等待父测试强杀。 |
| [aof/store_test.go:303](https://github.com/Li-Nepenthe/mini-redis/blob/main/aof/store_test.go#L303) | TestThousandWritesSurviveForcedProcessTermination | TestThousandWritesSurviveForcedProcessTermination 启动本测试二进制的专用助手，等待 READY 后只强杀该子进程。 |
| [cmd/client/main.go:17](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/client/main.go#L17) | main | main 启动教学交互客户端，连接固定的 127.0.0.1:6379，将标准输入按空白拆词并编码为 RESP 请求。 |
| [cmd/server/main.go:23](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/server/main.go#L23) | main | main 解析 -addr/-aof 参数，订阅 Ctrl+C/SIGTERM，并将信号 context 交给 run 启动服务。 |
| [cmd/server/main.go:38](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/server/main.go#L38) | run | run 根据 addr 启动 TCP 服务，根据 aofPath 选择恢复并绑定日志；ctx 已取消时直接返回 nil。 |
| [cmd/server/main_test.go:12](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/server/main_test.go#L12) | TestRunCancellationStopsBackgroundWork | TestRunCancellationStopsBackgroundWork 验证取消 ctx 后 run 能在 3 秒内返回 nil，没有返回值。 |
| [cmd/server/main_test.go:29](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/server/main_test.go#L29) | TestRunListenFailureStopsBackgroundWork | TestRunListenFailureStopsBackgroundWork 先占用临时端口，验证 run 在监听冲突时返回错误。 |
| [cmd/server/shutdown_test.go:16](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/server/shutdown_test.go#L16) | TestRunShutdownFlushesAndReleasesAOF | TestRunShutdownFlushesAndReleasesAOF 通过真实 TCP 确认 SET，再取消服务，检查有界样本退出、AOF 锁释放及重开恢复值。 |
| [database/benchmark_test.go:12](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/benchmark_test.go#L12) | BenchmarkEngine | BenchmarkEngine 使用 b 的并行计时，比较 1/16/64 分片在约 90% 读、全写与 80% 单 key 热点（约半写）下的耗时和分配。 |
| [database/cleanup_test.go:12](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/cleanup_test.go#L12) | TestLargeExpirationReclaimsOutsideShardLocks | TestLargeExpirationReclaimsOutsideShardLocks 用可控时钟使阈值以上 key 过期，注入 reclaim 检查调用时所有分片锁都可获取。 |
| [database/engine.go:34](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L34) | StatusReply.RESPStatus | RESPStatus 返回状态回复的字符串内容，使编码器通过接口识别 PONG 等简单状态。 |
| [database/engine.go:38](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L38) | NewEngine | NewEngine 按 shardCount 创建分片 map、默认时钟和内存回收回调，返回尚未绑定 AOF/启动清理 worker 的引擎。 |
| [database/engine.go:51](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L51) | fnv32 | fnv32 对 key 的字节逐个计算 FNV-1 32 位散列，返回稳定的无符号路由值，无锁及存储副作用。 |
| [database/engine.go:62](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L62) | *Engine.getShard | getShard 使用 key 的 FNV-1 散列与分片掩码选择并返回所属 shard。 |
| [database/engine.go:69](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L69) | *Engine.lockKeys | lockKeys 将 keys 映射到分片，去重并按分片编号升序获取锁；write 为 true 获取写锁，否则获取读锁。 |
| [database/engine.go:111](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L111) | NewLinkedList | NewLinkedList 创建并返回首尾为 nil、长度为零的双向链表，不启动后台工作。 |
| [database/engine.go:115](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L115) | *LinkedList.Len | Len 返回链表当前节点数量，不遍历或修改节点，也不加锁。 |
| [database/engine.go:119](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L119) | *LinkedList.LPush | LPush 拷贝 value 字节并在链表头插入一个节点，更新 head/tail、前后链接及长度，没有返回值。 |
| [database/engine.go:133](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L133) | *LinkedList.LPop | LPop 移除链表头并返回其值及是否成功；空链表返回 nil、false。 |
| [database/engine.go:152](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L152) | *Engine.Exec | Exec 将 args[0] 的命令名转为大写并分发；其余参数保持二进制内容，调用期间调用者不能并发修改 args。 |
| [database/engine.go:186](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L186) | *Engine.get | get 执行 GET，args 必须包含命令名和一个 key；不存在或过期返回 nil，String 返回拷贝的 []byte。 |
| [database/engine.go:207](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L207) | *Engine.LPush | LPush 将包含命令名、key 和至少一个值的 args 交给统一 LPUSH 写路径，返回更新后的长度或错误。 |
| [database/engine.go:213](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L213) | *Engine.LPop | LPop 将包含命令名和一个 key 的 args 交给统一 LPOP 写路径，返回移除的头值、缺失时 nil 或业务/日志错误。 |
| [database/engine.go:219](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L219) | *Engine.exists | exists 执行 EXISTS，对 args[1:] 的每个 key 判断是否存活并返回 int 计数；重复的存活 key 重复计数。 |
| [database/engine.go:241](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine.go#L241) | *Engine.lrange | lrange 执行 LRANGE，args 为命令名、key、start、stop，按包含 stop 的区间返回拷贝后的 [][]byte。 |
| [database/engine_test.go:13](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine_test.go#L13) | command | command 将 parts 文本转为 Engine 所需的 [][]byte 并返回，供单元测试快速构造命令。 |
| [database/engine_test.go:23](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine_test.go#L23) | requireExec | requireExec 用 parts 执行 e，要求无错误且结果与 want 深度相等，由 t 记录失败，无返回值。 |
| [database/engine_test.go:33](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine_test.go#L33) | TestCommandSequence | TestCommandSequence 按顺序验证大小写命令、二进制 PING、String、List、重复 EXISTS/DEL 与缺失值返回类型。 |
| [database/engine_test.go:62](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine_test.go#L62) | TestCommandArgumentAndTypeErrors | TestCommandArgumentAndTypeErrors 表驱动验证未知命令、数量、整数溢出与 String/List 类型冲突。 |
| [database/engine_test.go:96](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine_test.go#L96) | TestListRangeBoundaries | TestListRangeBoundaries 验证 LRANGE 的闭区间、负索引、越界裁剪、反向/空区间与 int64 极端索引。 |
| [database/engine_test.go:118](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine_test.go#L118) | TestEngineOwnsStoredAndReturnedBytes | TestEngineOwnsStoredAndReturnedBytes 在 SET/LPUSH 后修改原输入，再修改 GET/LRANGE 返回字节，验证存储不被别名污染。 |
| [database/engine_test.go:141](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/engine_test.go#L141) | TestConcurrentListAndMultiKeyCommands | TestConcurrentListAndMultiKeyCommands 让 8 个 worker 同时 SET、EXISTS、反序 DEL 和共享 List LPUSH，检查 800 个元素不丢失。 |
| [database/errors.go:15](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/errors.go#L15) | *commandError.Error | Error 返回公开的错误码与消息文本，供 error 接口显示；不拼接内部 cause。 |
| [database/errors.go:19](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/errors.go#L19) | *commandError.Unwrap | Unwrap 返回内部 cause（可能为 nil），让 errors.Is/As 沿错误链匹配真实原因。 |
| [database/errors.go:23](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/errors.go#L23) | *commandError.RESPError | RESPError 返回允许公开的 code、message 两个字段，不包括内部 cause，供编码器按白名单输出。 |
| [database/errors.go:34](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/errors.go#L34) | wrongArgs | wrongArgs 根据小写 command 构造含命令名的参数数量错误，返回实现公开 RESPError 的 error。 |
| [database/persistence.go:19](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence.go#L19) | *Engine.AttachLog | AttachLog 将 log 绑定为后续写命令的确认日志；传 nil 表示不持久化，没有返回值。 |
| [database/persistence.go:24](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence.go#L24) | *Engine.executeWrite | executeWrite 执行已归一化 cmd 的写命令，对 args 校验并准备结果，然后确认日志，最后应用内存变更。 |
| [database/persistence.go:51](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence.go#L51) | *Engine.prepareWrite | prepareWrite 在 args 的参数/类型合法时，为归一化 cmd 准备 result、持久化 record、延迟修改 apply 和释放分片锁的 unlock。 |
| [database/persistence.go:174](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence.go#L174) | *Engine.Replay | Replay 将 args 作为一条历史持久化记录恢复到内存，返回非法记录/类型/参数错误，不调用 Append。 |
| [database/persistence_test.go:17](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence_test.go#L17) | *failingLog.Append | Append 增加 calls 并返回预先配置的 err，不存储参数或真实写盘。 |
| [database/persistence_test.go:21](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence_test.go#L21) | TestPersistenceFailureDoesNotMutateMemory | TestPersistenceFailureDoesNotMutateMemory 对六种有效写注入日志失败，验证 String、List 和 TTL 原状态保留且 cause 可匹配。 |
| [database/persistence_test.go:44](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence_test.go#L44) | TestInvalidAndNoOpWritesDoNotAppend | TestInvalidAndNoOpWritesDoNotAppend 验证非法参数/类型和无效果写不进入日志，Replay 拒读命令且 Exec 拒私有恢复命令。 |
| [database/persistence_test.go:69](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence_test.go#L69) | *recordingLog.Append | Append 深拷贝 args 的每个参数后加入 records 并返回 nil，记录逻辑持久化内容供回放对照。 |
| [database/persistence_test.go:80](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence_test.go#L80) | TestReplayPreservesHistoricalExpiryAndCreation | TestReplayPreservesHistoricalExpiryAndCreation 用固定时钟覆盖期限内追加、续期、过期重建、String 转新 List 和先清理再新建。 |
| [database/persistence_test.go:133](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence_test.go#L133) | TestLegacyReplayKeepsExpiryUntilFinalState | TestLegacyReplayKeepsExpiryUntilFinalState 在重启时刻 6 秒顺序回放期限 5 与后续期限 14，验证 String 仍为 v、TTL 为 8。 |
| [database/persistence_test.go:151](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/persistence_test.go#L151) | TestReplayExpirationUsesAbsoluteDeadline | TestReplayExpirationUsesAbsoluteDeadline 用固定基准时钟回放 SET 和 __EXPIREATMS，检查剩余 TTL 精确为 5。 |
| [database/ttl.go:25](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go#L25) | *shard.setExpiration | setExpiration 为 key 设置 deadline，并维护 expires→expiring 的索引；已有 key 续期只改期限、不重复入池。 |
| [database/ttl.go:37](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go#L37) | *shard.clearExpiration | clearExpiration 移除 key 的 TTL 和抽样索引；不存在时无操作，无返回值，调用者须持 shard 写锁。 |
| [database/ttl.go:57](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go#L57) | *shard.purgeExpired | purgeExpired 在 now 达到或超过 key 的期限时删除 data 与 TTL 索引，返回是否执行了过期删除。 |
| [database/ttl.go:70](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go#L70) | *Engine.lockRead | lockRead 为 key 获取可读取当前状态的分片，返回 shard 和本轮 now；返回时仍持该 shard 的 RLock，调用者须 RUnlock。 |
| [database/ttl.go:88](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go#L88) | parseExpirySeconds | parseExpirySeconds 将 value 的十进制文字解析为 int64 秒，返回秒数或 ErrInvalidInteger。 |
| [database/ttl.go:99](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go#L99) | *Engine.ttl | ttl 执行 TTL，args 必须为命令名和 key，返回 int64 秒数：不存在/过期为 -2，存在无期限为 -1。 |
| [database/ttl.go:124](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go#L124) | *Engine.RunCleanup | RunCleanup 阻塞运行 TTL 清理循环，ctx 取消后停止 ticker 并返回，没有独立错误返回。 |
| [database/ttl.go:152](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl.go#L152) | *Engine.cleanupExpired | cleanupExpired 取一次当前时间并逐分片抽样删除已到期 key，返回本轮实际删除数量。 |
| [database/ttl_test.go:16](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl_test.go#L16) | clockEngine | clockEngine 返回注入固定 UTC 基准时钟的 Engine 及原子 elapsed，测试可推进 elapsed 控制当前时间。 |
| [database/ttl_test.go:26](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl_test.go#L26) | TestTTLSemantics | TestTTLSemantics 用可控时钟验证 TTL=-2/-1、秒数取整、到期边界、SET 清期限与零/负 EXPIRE 删除。 |
| [database/ttl_test.go:54](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl_test.go#L54) | TestExpiredKeysAreMissingForEveryCommand | TestExpiredKeysAreMissingForEveryCommand 为各命令重新构造刚到期的 String，验证它们都将过期 key 按缺失处理。 |
| [database/ttl_test.go:78](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl_test.go#L78) | TestListKeepsTTLAndExpiryIndexIsConsistent | TestListKeepsTTLAndExpiryIndexIsConsistent 验证存活 List 推入/非末弹出保留 TTL，末弹出清 TTL，续期/删除维持反向索引。 |
| [database/ttl_test.go:111](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl_test.go#L111) | TestExpireArgumentErrors | TestExpireArgumentErrors 验证 EXPIRE/TTL 数量错误、非整数、int64 溢出及秒转 Duration 溢出被拒绝。 |
| [database/ttl_test.go:129](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl_test.go#L129) | TestCleanupCancellation | TestCleanupCancellation 启动 RunCleanup 后取消 ctx，并在 1 秒内等待其完成通道。 |
| [database/ttl_test.go:144](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl_test.go#L144) | TestConcurrentExpirationAndCleanup | TestConcurrentExpirationAndCleanup 并发执行 SET/EXPIRE/TTL/GET/立即删除及主动 cleanupExpired，等待全部 worker 完成。 |
| [database/ttl_test.go:167](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/ttl_test.go#L167) | TestExpireHundredThousandKeysWithoutReads | TestExpireHundredThousandKeysWithoutReads 写入 100000 个 256 字节值及 5 秒 TTL，不 GET，以内部数量观察主动过期在 30 秒预算内清空。 |
| [resp/command_reply_test.go:15](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/command_reply_test.go#L15) | testPublicError.Error | Error 返回固定内部错误文字，供公开错误测试证明编码器不直接输出 error 的文本。 |
| [resp/command_reply_test.go:19](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/command_reply_test.go#L19) | testPublicError.RESPError | RESPError 返回模拟对象的 code/message，允许测试注入换行或非法码检查公开错误边界。 |
| [resp/command_reply_test.go:23](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/command_reply_test.go#L23) | TestCommandReplyEncoding | TestCommandReplyEncoding 验证状态、数组、二进制、int64、公开/包装/内部错误及换行注入的准确 RESP 字节。 |
| [resp/fuzz_test.go:12](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/fuzz_test.go#L12) | FuzzParseStream | FuzzParseStream 用 f 注册正常、多帧、边界和损坏输入种子，再对变异字节运行 Parser。 |
| [resp/handler.go:38](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler.go#L38) | NewRespHandler | NewRespHandler 将 p 与 exec 作为协议解析器和命令执行器注入，创建连接登记表、父 context 和完成通道，返回 Handler。 |
| [resp/handler.go:48](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler.go#L48) | *RespHandler.Handle | Handle 接管 conn，启动按连接的 Parser，顺序执行每条非空请求并完整写回编码响应，没有返回值。 |
| [resp/handler.go:114](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler.go#L114) | writeReply | writeReply 循环向 conn 写完 reply 全部字节，成功返回 nil；遇写错误原样返回，零字节无错误视为 io.ErrShortWrite。 |
| [resp/handler.go:130](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler.go#L130) | *RespHandler.connectionsForStop | connectionsForStop 在 Handler.mu 下关闭新请求/连接登记门，并按 active 状态返回 idle、active 两份连接快照。 |
| [resp/handler.go:146](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler.go#L146) | *RespHandler.waitDone | waitDone 只启动一次等待 goroutine，在已登记 Handle 全部 Done 后关闭 h.done，并返回只读完成通道。 |
| [resp/handler.go:155](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler.go#L155) | *RespHandler.Shutdown | Shutdown 停止接收新命令，取消 Parser，关闭 idle 连接并允许 active 当前回复排空；ctx 的期限用于其写 deadline。 |
| [resp/handler.go:181](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler.go#L181) | *RespHandler.Close | Close 通过 Once 强制停止新请求、取消 Parser、关闭全部连接，并等待已登记 Handle/Parser 回收，重复调用可安全共享结果。 |
| [resp/handler_test.go:19](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler_test.go#L19) | executorFunc.Exec | Exec 将 args 原样转交接收者函数 f，并返回它的结果与错误，用函数注入业务故障/延迟。 |
| [resp/handler_test.go:29](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler_test.go#L29) | *observedParser.ParseStream | ParseStream 保存 ctx 和真实 Parser 返回的通道，必要时关闭 started 通知测试解析已开始，并返回原通道。 |
| [resp/handler_test.go:49](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler_test.go#L49) | *scriptedConn.Read | Read 在模拟连接锁下从 reader 复制到 buf，返回字节数/读取错误；closed 时返回 net.ErrClosed。 |
| [resp/handler_test.go:60](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler_test.go#L60) | *scriptedConn.Write | Write 在模拟连接锁下将 buf 写入 replies 并返回写入结果；配置 writeErr 时直接返回零与该错误。 |
| [resp/handler_test.go:71](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler_test.go#L71) | *scriptedConn.Close | Close 在模拟连接锁下设置 closed 并返回 nil，使后续 Read 返回 net.ErrClosed。 |
| [resp/handler_test.go:80](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler_test.go#L80) | TestHandlerEarlyReturnStopsParser | TestHandlerEarlyReturnStopsParser 注入写失败和不支持的结果类型，验证 Handler 提前返回也取消 Parser、关闭连接并等通道结束。 |
| [resp/handler_test.go:119](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler_test.go#L119) | TestHandlerProtocolErrorIsSingleRESPLine | TestHandlerProtocolErrorIsSingleRESPLine 输入恶意多行非协议数据，验证执行器未调用且只回固定一行 Protocol error。 |
| [resp/handler_test.go:134](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler_test.go#L134) | TestHandlerCloseInterruptsRead | TestHandlerCloseInterruptsRead 用 net.Pipe 制造空闲阻塞 Read，验证 Close 打断并等待 Handle，重复关闭安全且拒后来的连接。 |
| [resp/parser.go:30](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/parser.go#L30) | NewRespParser | NewRespParser 返回无共享可变状态的 RESP 请求解析器，不读取输入或启动 goroutine。 |
| [resp/parser.go:37](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/parser.go#L37) | *Parser.ParseStream | ParseStream 为 reader 启动一个解析 goroutine，返回只读 Payload 通道，依次提供完整参数或一次解析错误后关闭通道。 |
| [resp/parser.go:64](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/parser.go#L64) | readHeader | readHeader 从 bufio reader 读取一个带 CRLF 的长度头，返回去掉 CRLF 的字节或格式/长度/读取错误。 |
| [resp/parser.go:85](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/parser.go#L85) | readRequest | readRequest 按数组→bulk 长度→精确内容→CRLF 读取一条请求，返回参数、实际完整帧字节数及错误。 |
| [resp/parser_test.go:14](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/parser_test.go#L14) | TestParser | TestParser 用 parentT 的表驱动子测试验证 GET/SET、含 CRLF 内容及非数组/非法长度/残帧/结束符等解析结果。 |
| [resp/parser_test.go:140](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/parser_test.go#L140) | TestParserMultipleCommands | TestParserMultipleCommands 将两条请求粘连在同一 reader，验证按顺序得到两份参数而不吞掉后续帧。 |
| [resp/parser_test.go:201](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/parser_test.go#L201) | TestParserFragmentedInput | TestParserFragmentedInput 通过 io.Pipe 分次发送同一 GET 的头和内容，验证 Parser 等待并拼齐正确参数。 |
| [resp/record_size_test.go:11](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/record_size_test.go#L11) | TestParserReportsActualWireRecordSizes | TestParserReportsActualWireRecordSizes 解析带前导零长度头和空数组，验证 BytesRead 等于原帧长度并保留全部帧。 |
| [resp/reply.go:18](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/reply.go#L18) | EncodeReply | EncodeReply 将 result 或 execErr 编为一条 RESP 响应，返回字节及编码错误，不进行网络 I/O。 |
| [resp/reply.go:64](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/reply.go#L64) | appendBulk | appendBulk 在 reply 尾部追加 value 的 bulk 长度头、原始二进制内容与 CRLF，返回扩展后的切片。 |
| [resp/reply.go:74](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/reply.go#L74) | validErrorCode | validErrorCode 检查 code 是否为非空的纯 ASCII 大写字母，返回能否安全用作公开 RESP 错误码。 |
| [resp/reply_test.go:11](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/reply_test.go#L11) | TestEncodeReply | TestEncodeReply 用 parentT 的表驱动子测试验证 nil/空 bulk/字节/整数/true/内部错误的准确编码及不支持类型报错。 |
| [resp/safety_test.go:19](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/safety_test.go#L19) | TestParserHeaderBoundary | TestParserHeaderBoundary 构造含 CRLF 恰好 64 字节与超一字节的头，验证接受/拒绝边界。 |
| [resp/safety_test.go:39](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/safety_test.go#L39) | TestParserRequestByteBudget | TestParserRequestByteBudget 构造两个单独合法 bulk 但整帧超限，故意不提供第二段内容。 |
| [resp/safety_test.go:58](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/safety_test.go#L58) | TestParserIncompleteCommandIsNotCleanEOF | TestParserIncompleteCommandIsNotCleanEOF 验证残数组头、缺 bulk 和残内容各发送一次错误，不能误当干净 EOF 消失。 |
| [resp/safety_test.go:75](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/safety_test.go#L75) | TestParserCancellation | TestParserCancellation 用 io.Pipe 启动解析后取消并关闭 reader，等待通道在预算内关闭。 |
| [resp/safety_test.go:102](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/safety_test.go#L102) | *countingReader.Read | Read 转发到嵌入的 Reader，将实际 n 累加到 bytesRead，并返回原 n、err。 |
| [resp/safety_test.go:110](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/safety_test.go#L110) | TestParserLimitsHeaderRead | TestParserLimitsHeaderRead 输入 16KiB 无有效短头，验证错误后底层 reader 最多消费一个 4096 字节缓冲。 |
| [resp/safety_test.go:121](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/safety_test.go#L121) | TestParserRequiresCRLF | TestParserRequiresCRLF 分别给数组头与 bulk 头只带 LF，验证产生协议错误且不返回成功参数。 |
| [resp/shutdown_test.go:14](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/shutdown_test.go#L14) | TestShutdownTimeoutDoesNotHideBlockedExecutor | TestShutdownTimeoutDoesNotHideBlockedExecutor 将执行器阻塞到 release，验证 Shutdown 到期返回 DeadlineExceeded 而不假装工作结束。 |
| [tcp/commands_test.go:11](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/commands_test.go#L11) | request | request 将 parts 按字节长度编码为 RESP 数组请求并返回字符串，供 TCP 测试构造二进制/错误命令。 |
| [tcp/commands_test.go:22](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/commands_test.go#L22) | TestTwentyCommandsOnOneConnection | TestTwentyCommandsOnOneConnection 在一个真实 TCP 连接顺序执行 20 条命令，检查正常值、错误、List 和缺失值的准确回复。 |
| [tcp/server.go:28](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server.go#L28) | NewServer | NewServer 保存 addr 与 handler，并创建停机完成通道，返回尚未监听的 Server。 |
| [tcp/server.go:44](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server.go#L44) | *Server.ListenAndServe | ListenAndServe 在 Server.Addr 上创建 TCP listener，再将其所有权交给 Serve；返回带地址的监听错误或服务结果。 |
| [tcp/server.go:55](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server.go#L55) | *Server.Serve | Serve 接管非 nil listener 并循环 Accept，为每个连接启动 Handler goroutine，返回接受/关闭/停机的合并错误。 |
| [tcp/server.go:104](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server.go#L104) | closeListener | closeListener 关闭 listener，nil 或 net.ErrClosed 视为成功，其他错误原样返回。 |
| [tcp/server.go:118](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server.go#L118) | *Server.Shutdown | Shutdown 通过 Once 标记 closed/draining，关闭 listener，调用 Handler.Shutdown(ctx)，返回保存的合并停机错误。 |
| [tcp/server.go:136](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server.go#L136) | *Server.Close | Close 通过 Once 标记停止、关闭 listener 并强制 Handler.Close，等待全部连接 goroutine，返回保存的合并错误。 |
| [tcp/server_test.go:27](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L27) | *observedHandler.Handle | Handle 在进入/退出 inner.Handle 时用原子计数记录正在处理的连接，并转移 conn 给内部 Handler。 |
| [tcp/server_test.go:35](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L35) | *observedHandler.Shutdown | Shutdown 将 ctx 原样交给 inner.Shutdown 并返回其错误，用观察包装器保持真实排空行为。 |
| [tcp/server_test.go:39](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L39) | *observedHandler.Close | Close 转发 inner.Close 并返回其错误，保持测试包装器的强制关闭/等待语义。 |
| [tcp/server_test.go:43](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L43) | startServer | startServer 在回环随机端口启动真实 Server/RESP/Engine，返回服务器、listener、观察 Handler 和 Serve 结果通道。 |
| [tcp/server_test.go:68](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L68) | awaitError | awaitError 等待 result 中一次操作的 error 并返回；3 秒无结果由 t.Fatal 报超时。 |
| [tcp/server_test.go:81](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L81) | dial | dial 在 1 秒内连接 addr，设置 3 秒连接截止并返回 net.Conn，失败由 t.Fatal 终止。 |
| [tcp/server_test.go:96](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L96) | exchange | exchange 向 conn 写 request，再 ReadFull 恰好 want 长度并比较字节，由 t 报写/读/结果错误，无返回值。 |
| [tcp/server_test.go:112](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L112) | waitFor | waitFor 轮询 condition 直到为 true，超出 3 秒由 t.Fatal 输出 description，无返回值。 |
| [tcp/server_test.go:126](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L126) | TestListenAndServePortConflict | TestListenAndServePortConflict 预先占端口，验证第二次监听返回可由 errors.As 识别的 net.OpError 而不 panic。 |
| [tcp/server_test.go:150](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L150) | *errorListener.Accept | Accept 增加 calls 并返回 nil 与注入 err，模拟永久接受错误或 net.ErrClosed。 |
| [tcp/server_test.go:154](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L154) | *errorListener.Close | Close 为接受故障模拟 listener 返回 nil，满足 Server 回收接口而不操作真实资源。 |
| [tcp/server_test.go:158](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L158) | *errorListener.Addr | Addr 返回空的 TCPAddr，满足模拟 listener 的 net.Listener 地址接口。 |
| [tcp/server_test.go:162](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L162) | TestServeAcceptErrors | TestServeAcceptErrors 注入永久错误与 net.ErrClosed，验证前者保留原因、后者正常停止，Accept 都只调用一次。 |
| [tcp/server_test.go:191](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L191) | TestMalformedClientsDoNotAffectHealthyConnection | TestMalformedClientsDoNotAffectHealthyConnection 在坏连接发送非法长度/超长头，检查固定协议错误后正常连接仍能 GET。 |
| [tcp/server_test.go:211](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L211) | TestHundredAbnormalDisconnectsReturnToBaseline | TestHundredAbnormalDisconnectsReturnToBaseline 预热后使 100 个连接停在残请求并以 TCP RST 关闭，等待处理计数和 goroutine 回到基线。 |
| [tcp/server_test.go:240](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server_test.go#L240) | TestConcurrentCloseInterruptsIdleConnections | TestConcurrentCloseInterruptsIdleConnections 建立空闲阻塞连接，让 8 个 goroutine 并发 Close Server，验证全部返回且无运行 Handler。 |
| [tcp/shutdown_test.go:22](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/shutdown_test.go#L22) | *drainingParser.ParseStream | ParseStream 转发真实 Parser，并另启 goroutine 等 ctx 取消后用 Once 关闭 canceled 观察通道。 |
| [tcp/shutdown_test.go:35](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/shutdown_test.go#L35) | *delayedExecutor.Exec | Exec 增加 calls，首次关闭 started，然后等待 release 放行并返回 true、nil。 |
| [tcp/shutdown_test.go:45](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/shutdown_test.go#L45) | TestShutdownDrainsInFlightReplyAndRejectsPipeline | TestShutdownDrainsInFlightReplyAndRejectsPipeline 发送两条粘连请求，在第一条执行中停机，再释放执行器。 |
| [tcp/ttl_test.go:7](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/ttl_test.go#L7) | TestTTLCommandsOnTCP | TestTTLCommandsOnTCP 在真实连接检查缺失/无期限/有期限及 EXPIRE0 删除的 RESP 整数与空 bulk 编码。 |

## 命名接口契约

| 文件 | 接口 | 方法/嵌入 | 说明 |
|---|---|---|---|
| aof/store.go | logFile | io.Writer | Writer 的 Write 允许短写/部分写伴错误，由 Store 循环或触发确认前缀回滚。 |
| aof/store.go | logFile | Sync | Sync 返回 nil 才表示此轮同步成功；故障不能被缓存的成功状态掩盖。 |
| aof/store.go | logFile | Truncate | Truncate 将文件长度缩至确认前缀；失败由 Store.fail 合并并持续拒写。 |
| aof/store.go | logFile | Close | Close 释放文件句柄及系统独占锁，关闭错误必须传给上层。 |
| database/persistence.go | CommandLog | Append | Append 确认 args 的完整记录及同步；nil 才允许调用者提交内存，错误不得伪装成功。 |
| resp/handler.go | CommandExecutor | Exec | Exec 处理包含命令名的二进制参数，返回编码器支持的结果或业务错误；调用期间输入不并发修改。 |
| resp/handler.go | ProtocolParser | ParseStream | ParseStream 生产完整请求/错误并自行关闭通道；ctx 取消后所有者仍须关闭阻塞 reader 并等待结束。 |
| resp/reply.go | publicError | RESPError | RESPError 提供可公开的码和消息，不能携带内部 cause；编码器仍负责码格式与 CRLF 校验。 |
| tcp/server.go | Handler | Handle | Handle 接管 conn 并在返回前回收连接/解析工作；Server 负责等待该调用结束。 |
| tcp/server.go | Handler | Shutdown | Shutdown 用 ctx 排空已经开始的回复并拒新命令，失败必须如实返回而非声称全部结束。 |
| tcp/server.go | Handler | Close | Close 强制关闭连接并等待全部处理工作；未要求实现必须能中断任意业务计算。 |
