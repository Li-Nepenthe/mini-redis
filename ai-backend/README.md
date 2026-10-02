# AI 文档问答后端（P2）

说明书 P2 的独立 Go module，与 P1 Mini-Redis 同仓。当前 M1 注册/登录、会话 CRUD/分页、资源鉴权、手写 MySQL SQL/迁移已实现；M1 已独立复审通过并合并（PR #2，main@ef8817d），main 两模块 CI 通过。用户于 2026-10-02 要求暂停 P2；Provider 小块仅在本地分支保存，尚未接业务 API；M2 整体未完成，M3–M5 未实施且暂停。当前优先学习 P1，逐项状态见根 [验收矩阵](../docs/acceptance-matrix.md)。个人学习口述、真实模型质量/付费费用不能由代理或 Mock 代签。

## M1 启动

需要 Go 1.27.1 与 MySQL 8.4。复制 .env.example；由 shell 注入 DATABASE_DSN、JWT_SECRET 等，程序不读取机器的认证文件。JWT_SECRET 至少32字节。示例仅是本机开发值，真实凭据不得提交。

```sh
export DATABASE_DSN='app:local-development-db@tcp(127.0.0.1:3306)/ai_backend?parseTime=true'
export JWT_SECRET='local-development-only-change-this-32-byte-secret'
go run ./cmd/api
curl http://127.0.0.1:8080/health/ready
```

MySQL 中预先创建空 ai_backend 数据库与相应用户；启动自动应用嵌入的 migrations。DDL 隐式提交，迁移使用同连接命名锁、逐条可重跑的 CREATE IF NOT EXISTS，不宣称整份 DDL 原子。迁移失败时启动退出，不接受半完成业务。

```sh
curl -X POST http://127.0.0.1:8080/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.test","password":"learn-password"}'
curl -X POST http://127.0.0.1:8080/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.test","password":"learn-password"}'
curl http://127.0.0.1:8080/api/v1/conversations -H 'Authorization: Bearer <access_token>'
```

Windows 使用 curl.exe，PowerShell 环境赋值用 $env:NAME='value'。登入返回 access_token，示例中的占位符需替换；没有 RefreshToken/logout/failover。

## 已实现 API

| 方法/路径 | 行为 |
|---|---|
| POST /api/v1/auth/register | email规范化，8–72 byte bcrypt 密码，账号+本月额度事务 |
| POST /api/v1/auth/login | 返回只含AccessToken的JWT，校验HS256/issuer/audience/expiry |
| POST /api/v1/conversations | 创建会话 |
| GET /api/v1/conversations | page/page_size，范围1–10000/1–100，默认1/20 |
| GET/PATCH/DELETE /api/v1/conversations/{id} | 读取/改标题/删除；非owner403，缺失404 |
| GET /api/v1/usage | 当前UTC月的token_used/token_limit |
| GET /health/live、/health/ready | 进程存活、数据库可用 |

成功返回 code/data/request_id；失败返回 code/message/request_id。数据库/路径/内部错误不回写。JSON请求最多1MiB，拒绝未知字段/第二个JSON对象，header预算32KiB。X-Request-ID仅接受64字节内字母数字/_/-，否则重新生成。日志不记录密码/JWT/请求正文。

会话使用 created_at,id 双排序避免相同时间的页重叠。当前为 offset 分页，并发增删会移动页边界；深页有扫描成本，不能承诺快照游标语义。PATCH/DELETE补齐说明书M1明确要求的CRUD，未增加其他产品功能。

## 表、索引和连接池

| 表/索引 | WHY |
|---|---|
| 每表不透明随机ID主键 | 稳定资源标识，仍须owner鉴权；字符串主键比整数宽、次级索引更大 |
| users.email UNIQUE | 注册唯一性由数据库兜底；登录等值查找不扫全表 |
| user_quotas(user_id,month) PK | 一个用户一个UTC月一行，后续额度结算锁定同一行 |
| conversations(user_id,created_at,id) | owner过滤是最左列，后续排序可反向扫描同一索引 |
| messages(conversation_id,created_at,id) | 会话消息分页/顺序，与owner校验组合 |
| documents(owner_id,created_at,id) | 列表限定owner，再按时间排序 |
| chunks(document_id,position) UNIQUE | 每文档位置唯一、顺序读取；后续重复消费不能插入重复段落 |
| task_events.event_id PK/UNIQUE | 后续消息重复投递的去重边界 |

实际至少三条关键SQL与无索引/加索引rows对照见 [EXPLAIN材料](docs/explain.md)。MySQL rows 为优化器估计，不伪装成扫描计时。

驱动loc只影响解码，连接额外设置MySQL time_zone=+00:00；写入时间用Go UTC，不与非UTC会话CURRENT_TIMESTAMP混用。真实session+08回归已通过。

默认每进程 MaxOpenConns=20/MaxIdleConns=10、5分钟最大生命周期、1分钟最大闲置；API+Worker在本机max_connections=100下保留管理/测试空间。20是可调初值，不等于CPU核数，也不靠加连接修复慢SQL；看DBStats等待及压测再调。bcrypt先计算再开启注册事务，减少占锁时间。账号写成功但额度约束失败会回滚账号，真实集成测试验证此路径。

## 检查与学习

```sh
go test ./... -count=1
TEST_DATABASE_DSN='<owned test database>' go test -race ./... -count=1 -timeout=180s
go vet ./...
staticcheck ./...
go build ./...
```

不设置TEST_DATABASE_DSN时数据库集成测试明确skip，不能声称MySQL已测。单元测试不访问真实模型，不产生模型费用。本轮实际命令/结果见 [开发记录](docs/notes.md)。需要本人回答：聚簇与二级索引、覆盖/最左前缀、JWT无状态注销取舍、为什么这个连接池/事务边界；材料不能替本人勾选学习验收。

## 范围

原说明书后续范围（当前暂停，未实施）为一个兼容Provider+Mock、SSE、缓存/Redis Lua限流/额度幂等、Kafka异步文档与内存检索三路评测、Compose/Prometheus/压测。不用Kubernetes、ORM、RefreshToken、多Provider failover、gRPC/微服务、Workflow引擎、向量数据库、MCP、管理后台、长期记忆、死信/重试矩阵。PDF支持文本层的解析边界在M4实证后记录；不自动引入OCR。


暂停检查点：本地 codex/ai-backend-m2-m3-20261002@5a973b7 的 Provider Generate/Stream、Mock/HTTP adapter 和回归通过独立复核；分支名保留历史，不代表 M3 已开始。适配器没有推送/合并，当前 main 无生成/SSE业务端点或聊天页面。使用/测试/WHY详见该本地分支 ai-backend/README.md 和 docs/notes.md。
