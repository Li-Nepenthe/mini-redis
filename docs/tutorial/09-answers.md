# 第 9 章：自测题与完整参考答案

先独立写答案，再对照。每题包含结论、推导/依据、常见错误、纠偏办法；不是只给关键词提示。题号覆盖说明书S1–S5全部“必须能回答”、S6七项，以及旧指南十二题和第二层追问。覆盖映射在teaching-coverage.md。给出答案不代表本人已经通过口述。

## Q01 TCP 半包/粘包怎么处理？多读到下一帧的字节去哪？

**参考答案**：TCP只提供有序字节，不保留每次send边界。Parser持同一个bufio.Reader，先读声明头，按bulk长度ReadFull内容；半包时继续等到凑齐或EOF错误。粘包时第一条按自身长度结束，bufio保留余字节给下一次readRequest。SET k v为27bytes，接GET k为20bytes，一次读47不会变成一个命令。

**依据**：ParseStream循环复用buffer；TestParserFragmentedInput用Pipe分送，TestParserMultipleCommands检查两份输出。**常错**：认为一次Read等于一次命令，或者读到换行就结束整个value。**纠偏**：画三次Read如何填同一bulk，再画47字节在27处切开；跑第2章三字节实验。不能画清剩余20字节去处，说明把网络边界当成协议边界了。

## Q02 为什么数组/bulk？CRLF、负长度与累计超限怎样处理？

**参考答案**：数组表示参数个数，bulk长度把二进制内容与分隔符分开；空格/零字节/CRLF可在value内。readHeader要求头以CRLF结束，内容ReadFull后另查末尾CRLF。长度可解析成功却为负，所以make前单独拒负数/上限；并在下一段分配前把头、内容、分隔都计入32MiB预算。

**依据**：当前上限1024/16MiB/64bytes含CRLF/32MiB是项目选择。单bulk合法不代表整帧合法，1024×16MiB约16GiB。**常错**：说数组只是方便拆字符串；先make再检查；漏算头/CRLF。**纠偏**：手算SET27bytes与含CRLF四字节value；跑HeaderBoundary/RequestByteBudget/RequiresCRLF，后者第二段故意缺内容仍先报预算错误。

## Q03 interface、any、类型断言为什么用在这里？

**参考答案**：CommandExecutor只需要Exec的方法签名；Engine按方法集隐式实现，不需implements。Handler因此可用真实Engine，也可用慢/故障执行器测试。any可容纳String、List或结果类型，但取出必须检查实际类型；双返回断言ok失败可回WRONGTYPE，单返回失败panic。EncodeReply用类型switch将业务值变协议。

**依据**：NewRespHandler的参数是接口，Engine.Exec吻合；get/lrange检查ok。**常错**：以为any自动转型或interface意味着所有实现都可任意线程安全。**纠偏**：列出接口需要的方法与具体类型的方法，分别试String/List路径；同步和资源契约仍在文档中，方法签名不会替你保证。

## Q04 为什么复制字节？GET 与 LPOP 策略为何不同？

**参考答案**：slice描述符可共享底层数组。SET/LPUSH若保留输入别名，调用后改输入就改数据库；GET/LRANGE若暴露仍存储的字节，调用者可绕过锁改值。因此入库和这些读出都复制。LPOP移除节点后值脱离存储，可转交原字节，省额外复制。调用期间并发修改输入仍是非法竞争，复制只保证调用完成后的所有权。

**依据**：bytes.Clone/SET副本、LPush节点克隆、LPop移除节点；OwnsStoredAndReturnedBytes用X修改再读确认。**常错**：认为赋slice变量就是深拷贝；说所有返回值都必须复制。**纠偏**：画两个slice指向同一数组，标“数据库是否仍持有”；对返回GET与移除LPOP逐项判断。

## Q05 合法协议、错误参数、错误类型与编码错误如何区分？

**参考答案**：Parser判断字节帧；`SET k`帧合法但Engine数量错，回公开ERR；String上LPOP是WRONGTYPE；未知命令是ERR unknown command。EncodeReply将这些execErr编码为错误字节时自身error通常nil，连接可以继续。普通string/false/坏状态是实现结果不支持的编码错误，Handler结束该连接。坏协议只发固定一行并断开。

**依据**：Exec、wrongArgs、EncodeReply、Handle和TwentyCommands测试。**常错**：任何error都断连；把WRONGTYPE当协议解析失败。**纠偏**：对输入和两层error各写一列，再跑第3章业务实验，观察LPOP k的encodeErr仍nil。

## Q06 为什么分片数取2的幂？为什么不是越多越好？

**参考答案**：当前用hash&(N-1)，N=16掩码低四位全1，等价模16；N=10掩码1001只选部分编号，故构造器拒非2幂。改用%N可以支持其他数，但不是当前方案。更多分片增加独立锁机会，也增加对象、路由/调度成本；同一key始终同锁、AOF写还有logMu串行，不能保证更多更快。

**依据**：NewEngine/fnv32/getShard与9格benchmark。**常错**：把2幂说成所有哈希表要求；声称16是最优。**纠偏**：算示意hash18与N16/N10，再分别分析均匀与热点；要谈性能必须带工作负载。

## Q07 DEL a b 与 DEL b a 怎样防死锁？同分片要去重吗？

**参考答案**：若a在2、b在9，各按参数顺序拿锁就能一方持2等9、另一方持9等2。lockKeys先路由、按编号去重排序，双方都2→9，逆序释放。a/c若同分片，不去重会同一goroutine重入不可重入锁自己阻塞。去重锁不等于去重所有业务参数：EXISTS重复计数，DEL不同key实际删除数。

**依据**：lockKeys、exists、prepareWrite DEL。**常错**：随机加锁或“有Mutex就不会死锁”；所有命令都去重计数。**纠偏**：画等待环，再把两条请求的锁序列改成相同，最后跑并发多key测试加race。

## Q08 热点为什么使分片失效？缓存行伪共享是什么？

**参考答案**：80%请求同key就集中同shard，另外多数锁空闲；增加分片无法拆该key，测得热点64没有优势。伪共享则是不同变量处同一CPU缓存行，多线程各改自己的变量也触发一致性流量，是另一硬件机制。当前CPU样本显示锁/调度，不足以证明伪共享，不能无证据宣称padding解决。

**依据**：BenchmarkEngine hot80负载、performance.md边界。**常错**：凡慢就是伪共享；拿读多成绩解释所有写。**纠偏**：先列请求落在哪个key/锁，再查profile测的是什么；缺硬件/等待证据就标未确定，而非补故事。

## Q09 TTL 为什么同时惰性与主动删除？

**参考答案**：惰性在访问时以精确deadline挡住过期值，t=5到期即使worker没抽中也不可读；冷key从不访问则引用一直保留，因此主动worker有预算地删除。主动抽样不保证瞬间物理清空，但惰性保证可见性；只主动会在抽中前可能错误可见，只惰性会保留冷数据。

**依据**：lockRead/purgeExpired与cleanupExpired，ExpiredKeysAreMissing/十万key无读测试。**常错**：用worker每250ms当“可多活250ms”的许可。**纠偏**：分别写“GET结果”和“map里物理是否仍有引用”的时间线，不把可见性与回收混成一个条件。

## Q10 为什么不为每个 key 起定时器？

**参考答案**：大量key会有大量定时管理、回调与取消成本；SET/续期后旧timer仍可能删新值，必须增加版本/同步判断。当前用一worker和TTL索引抽样，访问路径补精确语义，复杂度与资源更可控。定时轮/最小堆等也能选择，但它们有更新/删除/预算取舍，本项目不引入。

**依据**：RunCleanup一个ticker、setExpiration续期不重复入池。**常错**：说每timer必定一OS线程，或说定时器方案永远错误。**纠偏**：用“t0期限5，t4续到14，旧t5回调”举例，讲它需要哪种保护，再比较本方案。

## Q11 清理为何不能长持锁或每tick强制GC？

**参考答案**：全表清理持同shard写锁会阻GET/写，key越多停顿越长；当前每shard最多256次随机检查后放锁。GC/归还OS内存还可能全局暂停，不应在分片锁内延长请求等待，也不应每250ms强制执行。累计主动删除32768且距上次回收至少5秒才锁外reclaim；首次lastReclaim零，达到阈值可立即触发。

**依据**：cleanupExpired/RunCleanup及锁外reclaim测试。**常错**：声称“锁外GC就完全无延迟”，或“启动必须等5秒”。**纠偏**：区分分片持锁时长与全局GC成本；画lastReclaim初值和更新时刻，跑TryLock检查。

## Q12 TTL=-2/-1/0、SET/List 与墙上时钟有什么边界？

**参考答案**：-2缺失或已到期，-1存在无期限；剩余秒四舍五入，t4.8距期限5仅0.2秒显示0但仍活。SET覆写清TTL；存活List增/非末弹保TTL，末弹删key/TTL；EXPIRE非正删除，缺失返回0。绝对期限按墙上时钟判断，时间调整会影响寿命，不是单调时钟不变保证。

**依据**：ttl和prepareWrite、固定时钟TTLSemantics。**常错**：显示0即缺失；重启一律重新加相对TTL。**纠偏**：手算0/0.4/1.1/4.8/5秒，先比deadline再做显示取整。

## Q13 key 已删，为什么进程内存还高？如何判断实验？

**参考答案**：删引用、GC发现不可达、运行时归还空闲页、OS工作集是不同层；map容量/栈/代码还在。显式GC后HeapAlloc降不证明空闲进程工作集已降，Peak记录历史最高更不会降。大批主动过期后锁外限频FreeOSMemory请求页归还，真实无AOF/无GET/无外部GC采样观察到明显回落，具体数值依机器负载。

**依据**：第5章失败/修复数据和主指南脚本。**常错**：重启代替回收；拿Peak当当前；外部GC却宣称worker足够。**纠偏**：标PID、写时长、相同进程与当前WorkingSet，15/30秒采样，并把HeapAlloc实验另列。

## Q14 先日志还是先内存？Sync 后没回复，重启如何？

**参考答案**：先验证准备、完整Append+Sync、再apply、再回复。先改内存遇日志失败会有只在内存可见的新值；先写无效命令会污染恢复。Sync成功apply前或回复断连时客户端未确认，记录仍可能恢复，所以“未确认”不等于“没有发生”；已确认写的强杀恢复由测试验证。准备阶段合法清理过期旧值是另一个副作用。

**依据**：executeWrite/prepareWrite/Append、失败注入与千条强杀。**常错**：只说“先日志安全”却不说明提交/回复间不确定；说every-fsync保证一切硬件断电零丢。**纠偏**：在五个失败时刻分别写日志/内存/客户端是否确定，不笼统把所有失败当回滚成功。

## Q15 fsync/Sync 的代价是什么？为什么不拿无AOF吞吐当持久化成绩？

**参考答案**：Sync请求文件同步，增加系统/存储等待；当前持logMu与分片锁等待，串行写且同分片读也受阻。普通Write成功不是同一确认承诺。周期fsync可以摊薄成本但引入确认窗口/缓冲和停机约束；本项目只always-fsync。无AOF的Engine benchmark没有这项成本，不能外推。

**依据**：Store.Append同步位置、executeWrite持锁以及benchmark边界。**常错**：把Sync当仅语言缓存清空；声称本项目周期策略最多丢一秒。**纠偏**：沿锁持有区间标磁盘等待，读README崩溃窗口而不背通用Redis默认值。

## Q16 绝对期限为什么仍会回放错？续期和重建各需什么？

**参考答案**：回放是恢复历史当时状态，不是每条用重启当前时间判断。t0期限5、t4续到14、t6重启，若先删期限5值，后续续期不能恢复；期限内LPUSH也可能变成无TTL新List。Replay/prepare(replay=true)不按现在提前删，完整历史后判断最终期限。过期后真实新建还需_LNEW边界，否则旧List元素/TTL或String类型会混入。

**依据**：第6章A/B/C时间线、HistoricalExpiryAndCreation/LegacyReplay测试。**常错**：只答“用绝对时间”或“回放永远不删”就结束。**纠偏**：逐条列历史状态，区分“期限内追加”与“过期后新建”，再对比原引擎和Replay的GET/LRANGE/TTL。

## Q17 _LNEW 为什么同长度？私有命令与旧日志有什么限制？

**参考答案**：LPUSH发现缺失/过期key时记录新建List意图，_LNEW清旧类型/TTL后从空List插入；存活List仍普通LPUSH。两者同5bytes且参数不变，32MiB合法帧不会因内部标记变长失败。Exec拒_LNEW/__EXPIREATMS，Replay才接受。旧草稿没新建标记时信息不足，不能保证所有历史歧义可恢复。

**依据**：prepareWrite记录替换、Replay与MaximumSizedLPUSH测试。**常错**：暴露私有命令给客户端；通过扩大帧预算回避标记变长；承诺自动迁移无记录的历史。**纠偏**：计算编码前后长度，再构造旧String过期→新List例子，指出缺少哪条信息。

## Q18 半尾能截，完整坏记录为什么不截？偏移为何不用游标？

**参考答案**：尾部确实不完整可能来自写中断，只截最后完整成功记录之后；完整非法帧/非法业务恢复记录若自动删后缀，会静默丢可能已确认的内容，必须拒启动保留证据。bufio预读可越过已处理位置，重新编码前导零头又变短；所以offset只累加成功Payload.BytesRead。

**依据**：Open、ReportsActualWireRecordSizes、RecoverOnlyIncompleteTail/RejectCorrupt测试。**常错**：所有解析错误都修尾；错误BytesRead=0就认为未消费字节。**纠偏**：列完整prefix长度、预读游标与规范化编码长度三个数，确认回滚目标只来自成功prefix。

## Q19 Append 的编码错误、存储失败与 Close 分别怎样处理？

**参考答案**：编码边界非法先返回不写盘，不进入sticky失败；真正Write/Sync故障才fail，尽力Truncate到旧size并Sync，组合保存cause，后续Append持续拒写。Close仍释放文件并保留失败；重复Close返回保存结果，关闭后的Append优先os.ErrClosed。Open失败也可能已回放前缀到临时引擎，不能投入服务。

**依据**：Append/fail/Close/Open。**常错**：回滚尝试就等于恢复成功；所有error都会把Store永久失效。**纠偏**：按代码错误来源分列“有没有I/O、有没有size前移、是否sticky、是否仍要释放句柄”，用faultFile记录验证。

## Q20 为什么真实 Redis 需要 AOF Rewrite，本项目为什么不做？

**参考答案**：追加日志保留多次覆盖/已删除key的旧命令，文件增长、启动需回放更多历史。Rewrite可把当前状态压成更短恢复表示，但须处理并发写、切换文件、崩溃恢复等额外边界。理解动机不等于在当前项目实现；说明书明确不做，本实现保留追加增长限制。

**依据**：一个key SET十次恢复只需最终值，但当前日志十条；计划排除项。**常错**：写了截半尾就说实现Rewrite。**纠偏**：分清“修尾丢未完整记录”和“重表示完整状态”，列后者新增一致性职责。

## Q21 goroutine 泄漏有哪些成因？cancel 为什么可能仍卡住？

**参考答案**：可能卡在Read/Write、没人接收的channel发送/接收、锁等待、无退出条件worker或启动后无人等待。Parser同时有Read与send阻塞：cancel解除select发送等待，Close reader解除Read，最后等channel关闭确认结束。CancelFunc只发信号不等待，不能杀任意执行器；消费者不能抢关生产者channel。

**依据**：ParseStream、Handle defer、EarlyReturnStopsParser/Cancellation。**常错**：加context就一切自动停止；只关conn或只cancel。**纠偏**：写当前栈具体卡哪条语句，为每类阻塞指定解除动作和结束证明，而不是说“都defer了”。

## Q22 Shutdown 的 active 门、WaitGroup 与超时如何配合？

**参考答案**：Handler用同mutex决定closed与state.active，关门后不执行预读新请求，已active的执行/回复允许排空；idle/半帧关闭。Server/Handler登记Add与closed同锁，先关门再Wait，避免Wait后新增任务。ctx到期只关闭网络并返回错误，任意Exec可能仍运行；最终Close还同步等，不能承诺永久阻塞时有界返回。

**依据**：connectionsForStop/waitDone/Handle/Serve/Shutdown，流水线与blocked executor测试。**常错**：listener关就立即Close切断当前回复；Shutdown返回超时即全部结束。**纠偏**：画两个queued请求，标只第一条active，分别观察超时返回与release后回收。

## Q23 启动/关闭谁拥有资源？defer 顺序与先监听为何重要？

**参考答案**：run先监听，失败不碰AOF；成功恢复后AttachLog，启动cleanup和信号观察；Server拥有listener/任务，Handler拥有conn/Parser，Store拥有文件。Serve结束后等观察，defer先cancel+wait cleanup，再关闭先注册的Store。后注册先执行，不能主函数退出就假定后台完成；Close错误合并返回。

**依据**：run与第7章资源表。**常错**：先开日志再占端口；先关AOF再等还在写的业务；把Once当自动清理。**纠偏**：按获得顺序画资源栈，再倒排defer并标每一步实际Done信号。

## Q24 为什么错误不能含内部链？WRONGTYPE 与 errors.Is 怎样共存？

**参考答案**：类型冲突公开码WRONGTYPE；数量/整数等ERR。commandError.Error仅公开文字，Unwrap保cause供Is/As，RESPError只给码/消息；EncodeReply白名单选公开字段、验证码并清CRLF，其他错误统一internal server error。包裹%w后仍可识别原身份，不把内部路径写到网络。

**依据**：errors.go/reply.go与包装/注入测试。**常错**：比较Error字符串当身份；公开接口就不再校验换行；%v也能保错误链。**纠偏**：跑主指南错误程序，确认Is/As为true而线路不含request failed/内部路径；把CRLF消息代入编码观察空格。

## Q25 Temporary 为什么弃用？Accept 永久错误怎样处理？

**参考答案**：Temporary对“暂时”的定义缺乏足够一致、可靠的通用重试含义；网络不同操作/错误情境不能只靠这一布尔值制定策略。当前Server不做复杂重试：net.ErrClosed正常结束，其他Accept错误包装返回；让上层知道失败，避免无退避continue忙循环。监听错误则在资源未获得前返回。

**依据**：Serve及errorListener一次调用断言。**常错**：说弃用意味着所有网络错误都永久，或继续照旧永久continue。**纠偏**：把“关闭”和“注入permanent”分别跑一遍，数Accept调用并保cause。若要改变重试策略需要新约束，不属于当前注释修订。

## Q26 Fuzz 语料如何演化？通过意味着什么？

**参考答案**：手写正常/错误种子起步，运行器自动变异，根据覆盖反馈保留有用输入；触发失败的输入可保存并重放。当前回调验证结构互斥、非nil、成功帧字节预算，不验证全部业务语义。普通go test跑种子，显式-fuzz/-fuzztime才长时间探索；已有十分钟运行是特定版本证据，不保证任意恶意输入永远安全。

**依据**：FuzzParseStream与第8章命令。**常错**：没有panic就所有命令正确；普通test结束就称十分钟fuzz。**纠偏**：列出当前实际断言和未覆盖性质，记录运行命令/时长/版本，失败后保留最小重放输入。

## Q27 Benchmark、CPU profile 能说明 P99 或全部锁等待吗？

**参考答案**：并行benchmark的ns/op是整体受控负载平均操作成本；P99是99%请求不超过的延迟阈值，须收集分布。CPU profile是CPU样本，睡眠/等待不一定以同样成本出现；mutex/block profile与端到端延迟回答其他问题。这里无TCP/AOF/TTL，不外推持久化吞吐和过期尾延迟。

**依据**：BenchmarkEngine预填/ResetTimer/RunParallel、performance.md。**常错**：把ns/op倒数当真实服务QPS、lockSlow样本等于等待占比。**纠偏**：先写计时区间、并发配置与遗漏成本，再选择合适测量工具，本轮未做的写未测。

## Q28 怎么判断一个测试通过不是空匹配/假对象误读？

**参考答案**：检查实际匹配的测试名/子例和PASS，no tests to run不能算；Skip说明条件，Short十万key未执行。fake文件只测调用/故障，真实文件测锁/重放，context取消测试run不是OS信号。Fatal停止当前测试goroutine，Error只标失败继续；Helper改善位置，Cleanup回收测试资源。不同证据拼成完整结论，不能靠名称猜行为。

**依据**：每个测试函数的目标注释、audit清单和CI步骤日志。**常错**：所有名含Shutdown都证实Ctrl+C，最大帧fake测试证实真实磁盘。**纠偏**：给每个测试标输入模型/断言/资源/不证明什么，再和实际函数体核对。

## Q29 S1 的两个崩溃模型是什么？哪些基线已有，哪些本轮补？

**参考答案**：模型一监听失败被忽略后调用nil listener，会panic；模型二负长度解析成功后直接make负长度会panic，巨大正长度还可耗资源。开始的6daf725已限制数组/bulk、拒负数并有Fuzz骨架，不能说这些全是10/02新写。实际推进补监听/Accept、头/累计预算、取消/读打断/等待等；完整历史记录在notes。

**依据**：原说明书保留08/22快照、基线注释/测试与开发记录。**常错**：照旧计划把已存在防护说成新功能，或把性能故障说成panic。**纠偏**：区分“解释失败模型”与“本次实际diff”，用基线/最终源码各对一次。

## Q30 S6 七点与三分钟/十五分钟怎样完整讲？

**七点参考答法**：TCP半/粘按长度持续解析；RESP数组/bulk区分参数与二进制；分片锁按稳定路由且多key有序去重；TTL精确惰性+预算主动、回收层次分开；AOF准备→同步日志→内存、未确认可能发生、历史期限与新建边界；goroutine由启动者cancel/Close/等待；S1两崩溃模型和基线归属见Q29。每点必须能展开相应Q题，背七个词不算通过。

**三分钟参考讲稿**（自行计时、自然讲述，不要求机械背词）：

> 我做的是Go单机内存数据库，只兼容RESP2的一部分，用官方客户端执行String和List十条命令。目标是通过一个小服务理解网络字节边界、并发共享状态、持久化确认与生命周期，而不是用命令数量证明能力。SET k v在线上是27字节数组和bulk，TCP可能分段或与GET一起到。Parser按头和长度读满，先校验负数和预算，Handler顺序交给Engine，再将业务值编码写回。存储默认16分片，FNV-1稳定路由；多key锁排序去重防反序死锁，字节入库与仍存储值的返回都复制，防别名改数据。
>
> TTL访问时精确判断，worker有限抽样回收冷数据；删引用不等于OS内存立刻下降，所以大批主动过期后在锁外限频请求归还。AOF不是先改内存：准备后完整Write+Sync成功才apply，再回复。未收到回复也可能已落盘；每写同步牺牲吞吐，不能拿无AOF基准外推。一个真实问题是恢复按重启时间提前删旧期限值，后续续期或LPUSH恢复错；修复先完整重放历史，再判断最终期限，过期后新List另记_LNEW边界。停机先关新请求门，当前回复排空，cancel加关连接并等Parser，最后worker和日志回收。普通测试、race、故障注入、真实文件/强杀和精确CI形成证据；我的个人自测口述还需要自己验收，P2目前暂停。

**十五分钟参考演示脚本**：以下给完整讲述内容和可展示例子，时间含画图、观察/切源码。单读文字不保证恰好十五分钟；练习时计时调整，不编造自己的熟练程度。

1. **第0–1分钟，问题与边界**：说明两个进程共享小状态的需求，为什么选单服务/map/日志；只String/List/十命令，没有Redis全部特性。指根module与P2独立module，声明模拟教学顺序并非提交历史。
2. **第1–3分钟，请求完整链**：手写SET27与GET20，总47。讲TCP无帧边界、bufio保存余字节、ReadFull跨半包、错误和干净EOF区别；展示三字节实验两行精确输出。跟踪Serve→Handle→ParseStream/readRequest→Exec→EncodeReply→writeReply，指出Parser只验证协议，SET缺value由业务判断。
3. **第3–5分钟，存储与并发**：画LPUSH a b c如何成[c,b,a]，末弹删除key。画slice共享数组，说明GET为何复制、LPOP为何转移。再画a在2、b在9的反序等待环，说明lockKeys有序去重与N2幂的mask演算；热点同key无法拆，logMu为何把日志+apply顺序串起来。
4. **第5–7分钟，TTL与内存**：画deadline5及显示5/5/4/0/-2，解释0仍可能存活。画expiring[a,b,c]删b后[a,c]、c.index更新。惰性保证可见，主动释放冷数据引用；250ms/256预算控制锁，32768/5秒锁外回收，首次无需等启动五秒。列引用/GC/运行时页/工作集四层，说明无GET/外部GC的真实采样及Peak误区。
5. **第7–10分钟，AOF与故障**：在prepare返回的result/record/apply/unlock/err上逐项指作用，标Write、Sync、apply、回复间五个故障点。日志失败不apply，回滚尝试也持续拒写。画非规范头长度与预读偏移差别：成功BytesRead才累计，半尾可修、完整坏记录拒绝。用t0期限5/t4续到14/t6重启推导提前删错误，再用过期后新List解释_LNEW；说明私有命令与旧歧义日志限制，Rewrite动机与明确不做。
6. **第10–12分钟，生命周期**：列listener、conn、Parser、cleanup、watcher、Store六资源的获得者。讲cancel只解除send、Close解除Read、等channel关闭才Done；active门允许一条当前回复，不放行预读第二条。展示blocked executor测试先超时再release回收，说明Close仍可能卡任意Exec。倒排run的defer：先清理等待再关文件，端口先于AOF避免重复启动碰日志。
7. **第12–14分钟，如何验证**：展示命名测试的输入/断言，fake faultFile与真实文件区别；千条强杀只杀自有child，context取消不是OS信号。列race、fuzz、benchmark、CPU profile、OS采样、CI各自边界；解释9格基准不含TCP/AOF/TTL，热点没优势有负载原因，没有测P99就不说P99。打开精确提交CI的实际jobs而非只看徽章。
8. **第14–15分钟，真实问题与自己尚未通过项**：把历史TTL和OS内存两个问题分别按“症状→定位层→修复→复验”再讲一次，指出数值只属该环境；说本人待自测/口述、仓库元信息缺写入口、P2暂停。让听者选一题深挖，按本章依据和反例核对，而不背关键词回避。

**常错/纠偏**：只讲架构不讲失败；按未来计划声称做了Rewrite/完整P2；把代理生成答案当自己掌握。录音后逐题检查“边界、代价、证据”是否真正讲出，缺哪一段就回对应小实验，不勾个人验收。

## Q31 现在到底完成什么，哪些不能勾完成？

**参考答案**：P1工程S1–S5和S6大部分代码/运行证据已完成；本次补全注释与系统教程需经过独立审查、精确新CI。description/topics缺写入口仍待补；个人问题、自测/三分钟十五分钟仍本人待验，因此不能按日期宣布所有S6/封版。P2 M1已合并，Provider仅本地5a973b7、M2未整体完成，后续暂停，不要恢复开发。

**依据**：执行说明书§6、acceptance-matrix与最新notes/PR。**常错**：材料齐等于本人学会；按10/03日历自动封版。**纠偏**：分别画工程证据、教学材料、个人学习三栏；本人未讲过的保持未验。

## Q32 每个包/函数都有注释，是否就证明文档正确？

**参考答案**：不存在缺Doc只证明形式覆盖。还要逐项核对用途、参数、返回、错误/副作用、锁/资源前提是否与实现一致；AST字数/中文只能筛候选，不会理解语义。7包包括tcp_test；152函数包括未导出、平台文件、测试/fake/benchmark/fuzz，接口方法另有契约。文档必须串起交接并让实验可复现，完整题答用来定位理解偏差。

**依据**：comment-coverage统计/完整清单、独立复核、代码token等价与章节实验。**常错**：把0缺失当0错误；只算导出函数；把fake/外部测试包藏掉。**纠偏**：随机挑一个未导出函数和一个测试助手，说出锁/副作用/不证明什么，再让他人对照函数体逐条找反例。
