# 第 8 章：验证能证明什么，不能证明什么

## 8.1 每个测试也有职责与假设

P1源码清单的152个具名函数计数包括91个测试/基准/fuzz/辅助方法，因为 fake Write/Close、startServer、clockEngine、request等也决定证据是否可信。`faultFile`只记次数/目标/注入错误，没有真实磁盘；`recordingLog`深拷贝逻辑记录，没有fsync。`TestMaximumSizedLPUSHFitsPersistedCreationRecord`用fake Store确认预算，真实文件重放由其他测试提供。

表驱动先写“输入/期望结果/期望错误”，再运行每项；如果只复制实现中的分支作为期望，就很容易同时写错。List的小数据例子、TTL固定时刻、原/回放引擎对照是独立可判断的事实。`errors.Is`判断身份，不是只比显示文字。Helper使失败指出调用者，Cleanup在测试结束释放资源，不能代替业务Close契约。

## 8.2 从快检查到较重证据

根模块可复制命令：

```powershell
go test ./... -count=1 -timeout=60s
go test -race ./... -count=1 -timeout=180s
go vet ./...
staticcheck ./...
go build ./...
gofmt -l aof cmd database resp tcp
```

预期所有测试通过，race无DATA RACE，vet/staticcheck/build退出0，gofmt没有文件名。root不会跨ai-backend新module；本轮没改它。Windows race缺C工具链时清楚记录“未运行”，不能用CGO_ENABLED=0普通测试冒充；使用现有工具，不凭教程静默安装系统依赖。

| 方法 | 输入/观察 | 正确解释 | 常见过度推断 |
|---|---|---|---|
| unit/table | 定义场景 | 结果符合这些断言 | 所有输入都正确 |
| TCP集成 | 真连接/回复 | 实际请求链闭合 | 客户端兼容全部Redis |
| race | 执行共享访问 | 已执行路径没发现竞争 | 无死锁/性能好 |
| fuzz | 种子+自动变异 | 特定断言没有被触发 | 验证全部业务语义 |
| benchmark | 固定负载/平均成本 | 同环境可比较 | ns/op等于P99 |
| CPU profile | CPU样本 | CPU消耗集中函数 | 所有等待耗时已量到 |
| 进程实验 | 自有进程/OS指标/信号 | 真实环境中的样本 | 所有硬件/负载保证 |
| CI | 精确提交+任务结果 | 这个HEAD在runner通过 | 配置文件存在就算绿 |

有Skip要写Skip原因：普通CrashProcessHelper只在专用child执行，Short会跳10万key；`no tests to run`不是目标验证。失败要留原始日志，再修复、复验，不删掉失败假装始终成功。

## 8.3 fuzz 的语料怎样演化

[resp/fuzz_test.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/fuzz_test.go) 含正常/粘连/负数/超限/残帧/二进制/空数组种子。Go运行器变异输入，根据覆盖反馈保留有意义语料；触发断言的输入可重放回归。当前断言检查Payload非nil、Data/Err互斥与成功BytesRead预算；业务结果仍由命令测试检查。

```powershell
go test ./resp -fuzz=FuzzParseStream -fuzztime=10m -parallel=2
```

预期完成约十分钟，无crash/断言失败，实际执行数随机器变化。普通go test仅跑种子不是十分钟fuzz。既有最终Parser功能版本十分钟约79,148,927次通过；本次仅文档/注释变化不重复重负载，不能把历史计数写成新运行结果。

## 8.4 benchmark 先定义实验，再解读表

[database/benchmark_test.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/database/benchmark_test.go) 预填10000 key/64字节值，ResetTimer排除建库；1/16/64 shard × 读多(约9:1)、纯写、80%单热点(约半写)共9格。每worker独立固定伪随机序列，避免共享随机锁成新瓶颈。不启动TCP/AOF/TTL worker。

```powershell
go test ./database -run='^$' -bench=BenchmarkEngine -benchmem -benchtime=2s -count=3 -cpu=16
go test ./database -run='^$' -bench='BenchmarkEngine/hot80/shards16$' -benchtime=10s -cpu=16 -cpuprofile=cpu.pprof -o=database-profile.test.exe
go tool pprof -top database-profile.test.exe cpu.pprof
```

预期9格每格3份ns/op/B/op/allocs，profile可打开；数字不要求等于记录。参数`-cpu=16`是GOMAXPROCS配置，不是自动16个物理核心。本机记录Ryzen7 9700X（8核16线程）、31.10GiB、Windows/amd64、Go1.27.1，具体表见[performance.md](https://github.com/Li-Nepenthe/mini-redis/blob/main/docs/performance.md)。

已记录均匀读多1/16/64为386.90/58.11/34.70ns/op；热点为381.70/394.30/405.80，没有更多分片优势。说明负载集中到同锁时分片失效，不是“分片永远更慢”。CPU有lockSlow/调度样本，不证明伪共享，更不证明P99；尾延迟需另设计请求延迟分布，这轮未测也不加功能。

## 8.5 工程化的理由与使用限制

Docker多阶段先构建Linux纯Go二进制，最后scratch只放二进制，非root65532，/data承载AOF；减少运行镜像依赖。已有Docker可做：

```powershell
docker build -t mini-redis:study .
docker run --name mini-redis-study -p 127.0.0.1:6382:6379 -v mini-redis-study-data:/data mini-redis:study
```

预期启动；另一终端`redis-cli -p 6382 PING`为PONG，`docker stop --time 3 mini-redis-study`停止自己的容器。需要同一卷重启才是在看恢复，不删除数据后冒称成功。官方客户端未安装可用既有redis:alpine容器在同网络调用；不要自动拉取/安装未授权工具。

GitHub Actions跑格式、race、vet、staticcheck、build；另外main含已授权M1，因此CI也有MySQL集成，但本教程不教学或推进P2。最近已完成PR#3 main4ae13a7的[CI37001111749](https://github.com/Li-Nepenthe/mini-redis/actions/runs/37001111749)全部成功；当前全注释/新教程提交需自己的精确HEAD检查，不拿旧绿灯代替。

## 8.6 从工程证据回到学习证据

自己预测小数据结果、运行、比较、合上源码解释；再按第9章答案的“依据/常错/纠偏”检查。本人自测/口述仍待完成，代理给参考答案不等于替本人验收通过。S6七点、三分钟/十五分钟示范都在Q30。

旧`系统架构.html`标为历史草稿，含旧签名/未完成状态/旧读取说明；以本书当前链与源码为准，不把历史图当新实施缺口。覆盖矩阵另见[teaching-coverage.md](https://github.com/Li-Nepenthe/mini-redis/blob/main/docs/teaching-coverage.md)。完整答案Q26–Q32。
