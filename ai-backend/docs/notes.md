# P2 开发记录

## 2026-10-02 · M1：基线与实施

原 workspace D:\Code\GO项目\mini-redis 开始时main6daf725、无本地待提交/待推送。P1 PR #1复审修复后合并为84d3c8cf7c853ff68addae3b5056c2bf80308a8c；main CI36991251698全步骤通过。P2工作树为任务work/mini-redis-p2，分支codex/ai-backend-p2-20261002，从该main创建；保护原目录、P1代码与.idea。没有AGENTS.md或本地skill新增约束。

用户授权先完成说明书内工程再学习，并授权必要分支/提交/PR经测试审查后合并。M1–M3必做、M4–M5推荐增强在本次范围；根目录acceptance-matrix.md是逐项映射，不代替原说明书。学习问答/口述保持待本人。真实模型服务地址、聊天/Embedding模型、凭据注入与费用上限尚未提供，先Mock/local fake上游；未调用付费模型。

Go1.27.1，Gin1.12.0、mysql-driver1.10.1、jwt/v5 5.3.1、x/crypto0.57.0已用官方Go模块代理核实版本并下载到本任务缓存，go.sum锁定依赖；无主机安装/全局配置更改。SQL不用ORM，全部手写带参数查询；使用说明书users/quotas/conversations/messages/documents/chunks/task_events模型。

实现了注册bcrypt（8–72byte防截断）、HS256 JWT签名/issuer/audience/expiry、会话CRUD稳定双排序分页、owner403、统一错误/RequestID/Recover、1MiB JSON/32KiB header预算、health、启动迁移与10秒HTTP排空。PATCH/DELETE是M1“CRUD”所需接口，对原API表的省略做此窄补充。注册与初始本月额度同事务，额度CHECK失败不留下半账号。WHY见代码关键位置与README索引/连接池表。

### 验证

普通go test ./...通过，其中未设置TEST_DATABASE_DSN时真实数据库测试明确skip。随后创建唯一标签、本机随机端口的官方mysql:8.4临时容器，实际版本8.4.11，digest mysql@sha256:6ea90827b1100f8f2ae306a539f86d2c264a26ed435a2a9f75551dd5c3aeb242；没有连接任何已有用户数据库。

设置自有fixture的TEST_DATABASE_DSN，go test -race ./... -count=1 -timeout=180s -v通过：bcrypt/JWT负例、400/401/403/404/500/分页/正文预算/Recover，真实注册事务回滚/密码散列/唯一性/第二页不重/owner读写删除以及HTTP路由登录验收通过。父代理随后将Gin默认恢复输出关闭，保留结构化分类，避免原始panic或请求进入日志；该小改后再验命令与结果另记。

EXPLAIN在100 users/10,000 conversations/10,000 messages固定样本上，独立复制表实际DROP/ADD索引：users ALL104→ref1，conversations ALL9902→ref100，messages ALL9824→ref1；实际业务表const1/ref100/ref1，无ALL。详情docs/explain.md，原始receipt在任务work/validation/p2-m1-explain.json；估计rows非实测扫描时间，对照表已删除。

M1尚需最终curl实际进程/检查复核；CI与PR须实际返回后补记录。没有把程序强制cleanup当成优雅停机验收，也没有提前标记M2–M5完成。

### 来源与下一项

[Gin绑定/校验](https://gin-gonic.com/en/docs/binding/binding-and-validation/)、[官方MySQL镜像](https://hub.docker.com/_/mysql)、[JWT库](https://github.com/golang-jwt/jwt)、[database/sql连接池](https://pkg.go.dev/database/sql#DB.SetMaxOpenConns)。本地验证结论以记录的实际输出为准。

下一项M2：统一Generate/Stream、Mock、一个HTTP兼容Provider，SSE/取消/重试边界、消息与usage、最小聊天页面；之后M3额度/缓存/限流/幂等。M4的Kafka仍用KRaft单节点，无需新主机安装。仓库description/topics当前缺可用写入口，已单独向父会话报告，不影响P2工程。


### M1 最终本机检查点

关闭Gin默认恢复输出后重新执行当前代码全量race+真实MySQL集成、vet、Staticcheck、build、gofmt，结果以任务work/validation/p2-m1-final-checks.json为最终依据。实际启动隐藏本机API子进程，已有curl.exe完成注册/登录JWT/错误token401/A看B403/CRUD/两页不重/ready，全部通过；测试账号为自有临时DB的新数据，无真实模型调用。日志含request_id，不存密码/JWT；验收后仅强制清理自有API进程，这不是优雅停机验收，M5再执行信号排空。

CI配置在独立ai-backend job提供MySQL服务、模块工作目录与go.sum缓存，全量race运行真实Repository集成而非skip；尚未观察该配置的远端结果，需审查后提交并在准确HEAD上实际验收。M1独立审查和PR结果也须返回后记录，不能预先宣布通过。


### M1独立审查修复：MySQL会话时区

审查指出mysql-driver loc只解码、不设置服务端会话。已在自有测试库专用单连接设session+08:00实证：改名UpdatedAt被解码为UTC18:21，实际UTC10:21，偏移约8小时；失败日志保留p2-m1-timezone-before-fix.txt。未改global/系统/用户DB。修复为新连接显式time_zone=+00:00、改名updated_at使用Go UTC参数；专用session+08下更新仍正确，新连接即使DSN携带+08仍按UTC建立。补两项真实MySQL回归，当前全量race/真实DB、vet、Staticcheck、build、格式通过；最终证据仍为p2-m1-final-checks.json及对应日志，替代修复前的同名final记录。原审查HEAD a19bd92的其他M1功能/范围与离线race均通过；修复commit需再次审查并观察准确HEAD CI。
