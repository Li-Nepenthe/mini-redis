# P1 教学覆盖矩阵

质量门槛：每个主题必须有术语、问题/朴素方案、当前函数协作/代价、具体例子、可运行检查与结果/失败定位、完整题答。AST形式覆盖不能替代独立语义审查；个人掌握不能由代理代签。主指南导航十个正文文件，Library单篇将它们合成完整读本，文档ZIP包含全部章节。

| 主题/正文 | 准确代码位置与函数 | 小数据/故障例子 | 实验与预期 | 完整答案 |
|---|---|---|---|---|
| 00 问题/Go最少基础 | package doc.go；Engine.Exec/CommandExecutor | 两进程共享user:7，slice/any/接口 | 第1章空目录三行值/回复 | Q03/Q32 |
| 01 模拟从空构建 | go.mod；cmd/server.run | 参数→map→回复，依赖步骤 | lab01 stored=v、SET/GET字节 | Q04/Q31 |
| 02 TCP/RESP边界 | ParseStream/readHeader/readRequest | 27+20bytes、三字节Read、内容含CRLF | lab02 两记录大小27/20；Parser边界PASS | Q01/Q02 |
| 02 回复/业务错误 | EncodeReply/appendBulk/validErrorCode | nil/空bulk/空array，公开换行 | Reply/ProtocolError测试PASS | Q05/Q24 |
| 03 数据与List | Exec/LinkedList各方法/get/lrange/exists | a b c→[c,b,a]，负索引，重复key计数 | lab03九行准确回复，命令测试PASS | Q04/Q05 |
| 03 写准备 | executeWrite/prepareWrite | 五返回、类型错也解锁、no-op不日志 | PersistenceFailure/InvalidAndNoOp PASS | Q14/Q19 |
| 04 路由/锁顺序 | NewEngine/fnv32/getShard/lockKeys | 18&15=2，2/9反序等待环 | lab04 elements=6；并发/race/回放一致 | Q06/Q07 |
| 04 热点/硬件概念 | BenchmarkEngine；performance.md | 80%同key，缓存行伪共享对比 | 既有9格/CPU记录，未测P99 | Q08/Q27 |
| 05 TTL语义/读升级 | lockRead/purgeExpired/ttl/parseExpirySeconds | 5/5/4/0/-2，放锁间续期重查 | 固定时钟/全部命令过期PASS | Q09/Q12 |
| 05 索引/清理 | setExpiration/clearExpiration/cleanupExpired/RunCleanup | [a,b,c]删b→[a,c]，冷key/旧timer | 索引/取消/锁外reclaim/10万key PASS | Q10/Q11 |
| 05 OS回收故障 | RunCleanup/reclaim；主指南完整脚本 | HeapAlloc vs 工作集/Peak，旧失败与修复 | 既有同进程无GET/外部GC15/30秒回落 | Q13 |
| 06 确认/持久化 | AttachLog/executeWrite/Store.Append/fail/Close | Write/Sync/apply/reply五个失败点 | 失败注入、千条强杀PASS | Q14/Q15/Q19 |
| 06 实际尾偏移/系统锁 | Open/encodeRequest/平台lockFile | 前导零头、半尾/完整坏帧 | lab06 v与tail匹配true；真实文件测试PASS | Q18/Q19 |
| 06 历史TTL/新建 | Replay/prepareWrite(_LNEW) | 续到14@重启6，期限内push/过期新建 | 五种固定历史+legacy+32MiB PASS | Q16/Q17 |
| 06 Rewrite动机/不做 | 原说明书排除项，Store追加行为 | 同key覆盖十次累积日志 | 仅分析，不新增Rewrite | Q20 |
| 07 网络/资源/停机 | Handle/Shutdown/Close/Serve/run/waitDone | cancel/send vs Close/Read，active两帧 | 早退/空闲/超时后释放/流水线/重开AOF PASS | Q21/Q22/Q23 |
| 07 Accept错误/客户端边界 | ListenAndServe/errorListener；cmd/client.main | 端口冲突、永久错误一次、数组显示限制 | TCP故障测试PASS，不增强客户端 | Q25/Q29 |
| 08 测试模型/工程 | Test/fake函数、FuzzParseStream/BenchmarkEngine/CI | fake vs 文件、context vs OS信号 | 普通/race/vet/SC/build/format与精确HEAD CI | Q26/Q27/Q28 |
| 09 七点/讲解/个人验收 | 全链，notes/acceptance-matrix | 3分钟全稿、15分钟含画图/观察脚本 | 参考材料完成不等于本人通过 | Q30/Q31/Q32 |

## 原说明书及旧指南问题逐项映射

| 来源 | 每项问题对应完整答案 |
|---|---|
| S1 四问 | 分配前校验Q02；泄漏类别Q21；TemporaryQ25；Fuzz演化Q26 |
| S2 三问 | 数组Q02；类型码Q05/Q24；内部链Q24 |
| S3 三问 | 双策略Q09；每key timer Q10；长持锁Q11 |
| S4 三问 | 日志/内存失败Q14；Sync代价Q15；Rewrite Q20 |
| S5 三问 | 2幂Q06；热点Q08；伪共享Q08 |
| S6 七点 | TCP Q01；RESP Q02；分片Q06/Q07；TTL Q09–Q13；AOF Q14–Q20；生命周期Q21–Q23；两崩溃Q29 |
| 旧指南12题/第二层 | 主指南第11章逐项映射Q01/Q02/Q04/Q05/Q06–Q19/Q21–Q24/Q27/Q29 |
| 补充追问/讲稿 | Temporary Q25、Rewrite Q20、限锁Q11、错误身份Q24、fuzz Q26、伪共享Q08、3/15分钟Q30 |

题答的完整解释在[09-answers.md](tutorial/09-answers.md)，没有以关键词定位替代正文。所有实验属于专用学习目录/测试自有资源，不操作真实AOF，不恢复P2实施。实际运行记录和本轮独立审查/新CI结果见docs/notes.md最新条目；表中PASS应与相应记录核对，旧重负载证据没有冒称本轮重新执行。
