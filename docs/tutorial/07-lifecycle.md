# 第 7 章：从启动者到最终退出证明

## 7.1 网络接入的最少接口

模拟构建这一步新增 `resp.CommandExecutor`（Exec）、`resp.ProtocolParser`（ParseStream）、`tcp.Handler`（Handle/Shutdown/Close）。`NewRespHandler` 注入前两者，`NewServer` 注入Handler；构造器尚不监听或启动Parser。每个接口的注释说明输入交接与谁关闭，不是只罗列方法名。

`Server.ListenAndServe` 创建listener，失败返回包装错误；`Serve(listener)` 接管已获得的监听器，每次Accept后登记任务并启动Handle。每连接一条Handler goroutine、再有一条Parser goroutine，属于当前架构，不是说所有Redis都这样实现。`cmd/server.run` 先监听端口，成功才开AOF，避免重复启动碰在用文件。

朴素“无限Accept，错误continue”会在永久错误上忙循环；旧式Temporary不是可靠的统一重试判断。当前net.ErrClosed是正常停止，其他Accept错误直接返回。监听失败不能在nil接口上Accept或defer Close，返回错误要先检查资源是否真正获得。

源码：[handler.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/resp/handler.go)、[server.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/tcp/server.go)、[cmd/server/main.go](https://github.com/Li-Nepenthe/mini-redis/blob/main/cmd/server/main.go)。

## 7.2 一个连接中发生的交接

1. Handle 在 mutex 下检查closed、记录conn、Add(1)，再启动Parser。closed与Add同锁，因此关门后不会有新增任务。
2. 从Payload通道按序收，检查ctx与停机门，再把state.active设true。这个边界决定“已经开始”是什么。
3. 执行Exec→EncodeReply→writeReply；writeReply处理短写直到完整回复。业务错误可继续；协议错误发固定一行后结束。
4. 回复写完将active=false；若停机已触发返回，不再执行预读流水线请求。
5. defer先cancel，再Close连接，再range通道等生产者关闭，然后取消登记、Done。

为何同时cancel和Close：Parser可能卡在“无人收的channel发送”，也可能卡在“等待更多网络字节”。cancel解前者，Close打断后者；range到关闭是实际结束证据。消费者不抢close channel，生产者才知道什么时候不会再send。

## 7.3 Shutdown 与 Close 的两个承诺

`connectionsForStop` 在同锁下关门，快照区分idle与active，不在锁内做网络I/O。Shutdown取消Parser并关idle/半帧连接；active写截止来自ctx，允许当前回复完成。`waitDone` Once启动Wait，所有Handle退出才close done。

若ctx到期，Shutdown关所有连接并返回ctx.Err；它不能取消执行器里任意等待或磁盘Sync。因此“Shutdown返回超时”和“工作全退出”必须分别描述。Close强制关网络后同步等全部工作，没有期限，永久阻塞Exec仍会卡住Close。

Server层也有WaitGroup：closed和Add同锁防Wait后新增任务。Shutdown通过Once关闭listener、调Handler.Shutdown，成功才等Server.wg；Serve的defer若在draining则等shutdownDone，不能因listener已关就抢先Close当前回复。重复Shutdown共享第一次ctx/结果，不重新开始新预算。

## 7.4 run 的资源栈为什么按这个顺序

| 获得者 | 资源 | 结束动作/证明 |
|---|---|---|
| run | listener | Server关闭；Serve结束 |
| Open→Store | 文件/系统锁 | 最后Store.Close返回 |
| run | RunCleanup worker | cancel后等待cleanupDone |
| run | 信号观察goroutine | ctx或watchStop，等待watchDone |
| Server | Handle goroutine | Handler返回，再Server.wg.Done |
| Handler | Parser/连接 | cancel+Close+等channel关闭，再Handler.wg.Done |

defer后进先出：AOF.Close先注册，cleanup取消等待后注册，所以实际先等cleanup再关AOF。网络Serve返回也要等观察goroutine，不能主函数走了留下悬挂。仅调用cancel没有等待证明；仅defer conn.Close也不能回收卡在channel发送的Parser。

错误用errors.Join保留多个关闭原因，而不是“日志打印过就返回nil”。2秒是网络排空预算，测试样本正常3秒内不证明极端Exec/fsync/GC总耗时。真实Windows Ctrl+C与Linux SIGTERM的旧验收记录是独立样本；下面context取消测试不是OS信号测试。

## 7.5 可运行故障检查

```powershell
go test ./tcp -run='TestListenAndServePortConflict|TestServeAcceptErrors|TestMalformedClientsDoNotAffectHealthyConnection|TestHundredAbnormalDisconnectsReturnToBaseline|TestConcurrentCloseInterruptsIdleConnections|TestShutdownDrainsInFlightReplyAndRejectsPipeline' -count=1 -timeout=60s -v
go test ./resp ./cmd/server -run='TestHandlerEarlyReturnStopsParser|TestHandlerCloseInterruptsRead|TestShutdownTimeoutDoesNotHideBlockedExecutor|TestRunShutdownFlushesAndReleasesAOF|TestRunCancellationStopsBackgroundWork|TestRunListenFailureStopsBackgroundWork' -count=1 -timeout=60s -v
```

预期所有匹配测试PASS。流水线样本只执行第一条、写完+OK、第二条不执行、连接EOF；超时样本先得到DeadlineExceeded，释放测试执行器后最终Handle回收；100个自有RST连接回到goroutine基线；run的AOF可重开且GET=v。

失败定位：监听冲突panic→先查错误后defer；Shutdown卡死→定位在Read、send、Exec、write、Wait哪一层；Close已返回但计数未归零→查登记/Done/通道等待；当前回复被截断→查Serve是否提前Close；预读第二条仍执行→查active门与shutdown同锁是否成立。

这些测试不动其他服务；`cmd/client.main` 固定6379、Fields拆词和数组首行限制保留为教学示例，不能用它的数组错乱判数据库失败。本项目不增强客户端。对应完整答案Q20–Q23、Q25、Q29、Q30。
