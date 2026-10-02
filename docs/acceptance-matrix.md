# 说明书验收映射

依据根目录《项目执行说明书-MiniRedis与AI后端_1.md》§3、§4、§6。用户授权先完成工程再学习，2026-10-02 追加授权审查/测试通过后合并，并完成 P2 M1–M5。时间表是原计划；实际完成日期另记，不预填未来日期。

状态：**通过**＝已执行且有证据；**待实现**＝尚未交付；**待验**＝代码已有但该条件没有实证；**本人待答**＝学习/口述不能由代理代签。Mock 与本地 HTTP 假上游仅验证工程，不代表真实模型质量或付费成本。

## P1 必做

| 说明书要求 | 实现/备注位置 | 验证证据 | 状态 |
|---|---|---|---|
| S1 分配前数组/bulk/头/整包预算、负数、畸形输入隔离 | resp/parser.go、parser_test.go | 表驱动/TCP；两轮 10 分钟 Fuzz，最新 79,148,927 次 | 通过 |
| S1 listen/accept 错误、端口冲突、parser 取消、100 异常连接回到基线 | tcp/server.go、resp/handler.go | server/handler 测试；100 RST 后 goroutine 3→3 | 通过 |
| S1 Staticcheck、先有 Fuzz 后修复 | resp/fuzz_test.go、docs/notes.md | Staticcheck v0.8.1；基线已有 Fuzz，历史顺序有记录 | 通过 |
| S2 十条命令、大小写、多值 LPUSH、类型错误、错误不泄漏、每命令表驱动 | database/engine.go、errors.go、engine_test.go；resp/reply.go | 官方 redis-cli 8.10.2 同 TCP 连接 20 条命令；full race | 通过 |
| S3 EXPIRE/TTL、惰性+抽样主动清理、context、README 语义 | database/ttl.go、ttl_test.go、cleanup_test.go | TCP TTL 5→3→-2/-1；取消等待；full race | 通过 |
| S3 10 万 5 秒 key 不读、30 秒进程内存回落 | ttl.go；docs/performance.md | Windows 工作集 82,616,320→25,841,664 bytes/15 秒 | 通过 |
| S4 AOF 追加→sync→内存、启动重放、相对 TTL 不延长、半尾恢复/完整坏记录拒绝 | aof/store.go、database/persistence.go | 1,000 确认写后强杀；真实尾部；失败注入；Linux/Windows | 通过 |
| S4 历史 TTL 续期/过期前 LPUSH/过期后重建一致 | persistence_test.go、aof/store_test.go | 固定时钟/真实 Store、精确32MiB；full race；Linux SIGKILL恢复 | 通过 |
| S5 1/16/64 shard × 9:1/纯写/80%热点、环境、profile、README 结论 | benchmark_test.go；docs/performance.md | 9 组各 3 次，CPU pprof；原始日志路径记录 | 通过 |
| S6 信号关 listener→排空→AOF，3 秒；Docker 多阶段非 root | cmd/server；Dockerfile | Windows Ctrl+C 1.6–3.0ms；Docker SIGTERM 0.294s；官方客户端 | 通过 |
| S6 race/Staticcheck/build CI、默认徽章、仓库首页 | .github/workflows/ci.yml；README.md | fbbce11 CI36991097480、main84d3c8c CI36991251698全success；首页实证 | 通过 |
| S6 改 module/文件名、清空文件/死码/emoji、LF、gitignore、README/问答、元信息 | 全仓；docs/learning-guide.md、notes.md | 清理/格式/范围已通过；description/topics缺写入口仍空 | 待验 |
| S1–S6 各阶段“必须能回答”、S6 七点脱稿、§3.5 全特征封版 | docs/learning-guide.md | 需本人学习口述，不代答、不伪造封版 | 本人待答 |

## P2 M1–M3：必须完成的最小版本

实现放在同仓独立 Go module 的 ai-backend/，不改变 P1 网络/数据库接口；API、Worker 共用业务包，仍是说明书要求的单体部署。MySQL/Redis/Kafka 是现成 Docker 测试服务；不用 Mini-Redis 替代完整 Redis 的 Lua 能力。

| 编号 | 要求（含完成特征/材料） | 交付/证据 | 状态 |
|---|---|---|---|
| M1.1 | Gin/net/http 项目结构、配置、.env.example、手写 SQL/migrations；说明书全部表/索引 | ai-backend/；schema 文档 | 通过 |
| M1.2 | 注册登录、bcrypt 密码、JWT 仅 AccessToken；curl 登录，错 token 401 | API/鉴权测试；真实 MySQL 密码检查 | 通过 |
| M1.3 | 会话 CRUD/分页；第二页不重；资源 owner；A 访问 B 会话 403 | API/Repository 集成测试 | 通过 |
| M1.4 | 统一响应/错误码、RequestID/Recover；table-driven unit+Repository DB integration | 中间件/错误测试，full race | 通过 |
| M1.5 | 每个索引 WHY、至少 3 条关键 SQL EXPLAIN、索引前后 rows；关键查询不 ALL | schema/explain 实际数据 | 通过 |
| M1.6 | 连接池参数与理由、一例事务边界/失败路径 | README/SQL 事务回滚测试 | 通过 |
| M2.1 | Mock 先行、一个 OpenAI-compatible Provider；Generate/Stream 统一接口 | Provider 单元/本地 HTTP 合约测试 | 待实现 |
| M2.2 | SSE Content-Type/Flush/心跳/事件 ID；浏览器逐字；断连取消上游 | 最小 HTML+JS；流与断连实测 | 待实现 |
| M2.3 | InvalidRequest/RateLimited/Timeout/ProviderUnavailable/StreamInterrupted | 400/429/5xx/中断测试 | 待实现 |
| M2.4 | 只重试 429（Retry-After）及确定 5xx；不重试参数/认证/已发 token/取消 | 429/400/取消/中断计数与时序 | 待实现 |
| M2.5 | 消息与 usage 落库；5 秒 context 对 10 秒上游约 5 秒取消 | 事务/超时实测、API 日志 | 待实现 |
| M2.6 | go test 只 Mock/本地假上游，无付费请求；真实模型联调 | 离线测试；真实 API 地址/key/预算未提供 | 待验 |
| M3.1 | Cache-Aside exact hash(model+prompt+params)、输入校验/负缓存、TTL jitter | 缓存 key/TTL/错误测试 | 待实现 |
| M3.2 | singleflight；50 同问模型只调用 1 次；二次问由秒→毫秒 | 并发计数/真实 Mock 延时测量 | 待实现 |
| M3.3 | 预估 quota→实际 token 结算→超额拒绝；quota100 第二次拒绝 | 真实 MySQL/Redis 并发/结算 | 待实现 |
| M3.4 | 用户+全局 Redis Lua 原子 token bucket；100 次/秒命中限流 | 真实 Redis Lua、并发 full race | 待实现 |
| M3.5 | Idempotency-Key 返回第一次结果，usage 只计一次 | 重复/并发/归属测试 | 待实现 |
| M3.6 | Prometheus cache_hit_ratio 等可解释；最小版本 Compose/README | metrics 与 Compose ready 实测 | 待实现 |

## P2 M4–M5：说明书推荐增强（本次仍要求完成）

| 编号 | 要求（含完成特征/材料） | 交付/证据 | 状态 |
|---|---|---|---|
| M4.1 | Docker Compose Kafka KRaft 单节点；上传+pending 入库+投递立即返回 task_id | Compose；10MB 返回<500ms 真实测量 | 待实现 |
| M4.2 | 独立 worker：解析 PDF/Markdown→切块→批量 embedding→chunks；pending/running/succeeded/failed | Worker；文档状态/失败测试 | 待实现 |
| M4.3 | task_events.event_id UNIQUE 消费幂等；重复事件不重 chunk；kill/restart 恢复 | 真实 Kafka/DB 重复投递与进程强杀 | 待实现 |
| M4.4 | Kafka 断开上传仍成功、pending 可恢复；不吞发布失败 | durable pending 扫描与故障验收 | 待实现 |
| M4.5 | progress SSE 从0→100有中间值；worker Ctrl+C 完成当前任务；backlog metrics | SSE/信号/Prometheus 实测 | 待实现 |
| M5.1 | 固定同一20–30样本，full context / 内存 brute-force cosine TopK / BM25或关键词三路 | eval 数据集/三路 runner | 待实现 |
| M5.2 | accuracy/citation/no-answer/TTFT/token cost，三路实际数据 | Mock 工程结果与真实模型结果分开 | 待实现 |
| M5.3 | 来源文档名/段落可追溯；无证据拒答；A 看不到 B 文档；基础 prompt injection 防护 | 检索/所有权/引用/注入测试 | 待实现 |
| M5.4 | API+Worker+MySQL+Redis+Kafka+Prometheus Compose；up 后 curl ready | 干净环境实际运行与停机 | 待实现 |
| M5.5 | CI race/Staticcheck/build、绿色徽章；structured logs request_id | Actions/main 验证与日志样例 | 待实现 |
| M5.6 | QPS/P99/error/cache hit/cost/provider error/backlog 指标；health/grace | metrics scrape/故障/停机实测 | 待实现 |
| M5.7 | 压测 QPS/P99/cache hit/singleflight on/off；README 架构/排障 | 可复现负载与实际环境/数据 | 待实现 |
| M5.8 | 真实模型联调/三路质量与费用评估 | 未提供 API 地址/凭据/费用上限；不可由 Mock 替代 | 待验 |

## 学习验收与严格排除项

M1–M5 的口述、MaxOpenConns 理由、索引/JWT/缓存/队列/检索取舍，§4.8 两条业务流程口述，均为**本人待答**。代码旁 WHY、开发记录、学习指南与演练命令用于后续学习，不作为已学会的证据。

P1 不做 AOF Rewrite、Cluster、Sentinel、复制、Lua、事务、Pub/Sub、RDB、其他数据结构或客户端增强。P2 不做 Kubernetes、OpenTelemetry Trace、RefreshToken、多 Provider failover、gRPC、微服务拆分、Workflow 引擎、Human-in-loop、长期 Memory、管理 UI、死信队列/重试矩阵、向量数据库或 MCP。新增需求必须能映射本表与原说明书；便利性功能不自动进入范围。
