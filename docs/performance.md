# P1-S5 性能记录 · 2026-10-02

Windows/amd64；AMD Ryzen 7 9700X（8 核/16 逻辑处理器）；CIM 可用物理内存 33,396,543,488 bytes（约 31.10 GiB）；Go 1.27.1；GOMAXPROCS=16，RunParallel 默认 16 个 worker。

测试入口是 `database/benchmark_test.go:BenchmarkEngine`。预先保存 10,000 个 key（key:00000…key:09999），value 为 64 bytes；预建 GET/SET 参数，每个 worker 采用独立固定伪随机序列。read90 为均匀 key 的 90% GET/10% SET，write100 为均匀纯 SET；hot80 的 80% 请求固定 key:00000，其余从全 key 空间取样，GET/SET 各半（因此实际命中热点约 80.002%）。1 分片是同一份 Engine 的单全局读写锁，不引入另一套实现。无 TTL、无清理工作、无 AOF、无 TCP/回复编码；预填数据在计时外。并发 Benchmark 的 ns/op 是总耗时/总操作数，不能解释成单条请求的尾延迟。

```sh
go test ./database -run='^$' -bench=BenchmarkEngine -benchmem -benchtime=2s -count=3 -cpu=16
go test ./database -run='^$' -bench='BenchmarkEngine/hot80/shards16$' -benchmem -benchtime=10s -count=1 -cpu=16 -cpuprofile=cpu.pprof -o=database-profile.test.exe
go tool pprof -top -nodecount=15 database-profile.test.exe cpu.pprof
```

| 负载（ns/op 中位数；越小越好） | 单全局 RWMutex | 16 分片 | 64 分片 |
|---|---:|---:|---:|
| 读:写 = 9:1 | 386.90 | 58.11 | 34.70 |
| 纯写 | 361.60 | 109.60 | 68.48 |
| 80% 热点，读:写 = 1:1 | 381.70 | 394.30 | 405.80 |


每格为三次结果中位数，原始全部结果在下方。分配开销不随分片数变化：

| 负载 | B/op | allocs/op |
|---|---:|---:|
| read90 | 116 | 3 |
| write100 | 344 | 10 |
| hot80 | 217 | 6 |


在此机器的均匀负载中，16/64 分片减轻单锁竞争。热点负载中，16 分片 394.30ns/op 比单锁 381.70ns/op 慢约 3.3%，64 分片 405.80ns/op 慢约 6.3%。热点依然映射到一把锁，增加分片不能分散同一 key；索引、调度和缓存访问的成本与测量波动也影响结果。因此保留 16 分片为学习默认值，不能声称更多分片总更快，也不能外推 always-fsync/TCP 服务吞吐。此处比较描述样本，不是统计显著性证明。

## CPU profile

以 hot80/16 分片采样 10 秒，实际 Benchmark 12.588s，389.0 ns/op。原始 `s5-cpu.pprof` 和配套测试可执行文件保存在本任务 `work/validation/`，可用上方命令自行生成。以下是实际 pprof top，锁/调度函数是主要热点；这是 CPU 采样，休眠等待的时间不会全部体现为 CPU 样本，分析等待时间需另做 mutex/block profile，未假装本轮已有这些证据。

```text
File: database-profile.test.exe
Build ID: C:\Users\Nepenthe\AppData\Local\Temp\mini-redis-go\go-build1237463302\b001\database.test.exe2026-10-01 21:02:12.7925358 -0700 PDT
Type: cpu
Time: 2026-10-01 21:02:12 PDT
Duration: 12.42s, Total samples = 20250ms (163.02%)
Showing nodes accounting for 8690ms, 42.91% of 20250ms total
Dropped 226 nodes (cum <= 101.25ms)
Showing top 15 nodes out of 130
      flat  flat%   sum%        cum   cum%
    1780ms  8.79%  8.79%     1780ms  8.79%  runtime.procyieldAsm
    1280ms  6.32% 15.11%     1280ms  6.32%  runtime.semawakeup
    1110ms  5.48% 20.59%     1110ms  5.48%  runtime.semasleep
     480ms  2.37% 22.96%      480ms  2.37%  runtime.memmove
     470ms  2.32% 25.28%      710ms  3.51%  runtime.tryDeferToSpanScan
     450ms  2.22% 27.51%      670ms  3.31%  runtime.mallocgcSmallScanNoHeaderSC3
     430ms  2.12% 29.63%      760ms  3.75%  runtime.mallocgcTinySC2
     380ms  1.88% 31.51%      380ms  1.88%  internal/runtime/maps.ctrlGroup.matchH2 (inline)
     370ms  1.83% 33.33%      370ms  1.83%  Mini-Redis/database.fnv32 (inline)
     350ms  1.73% 35.06%    13320ms 65.78%  Mini-Redis/database.(*Engine).Exec
     340ms  1.68% 36.74%     2560ms 12.64%  internal/sync.(*Mutex).lockSlow
     330ms  1.63% 38.37%     1440ms  7.11%  runtime.growslice
     320ms  1.58% 39.95%      370ms  1.83%  runtime.casgstatus
     310ms  1.53% 41.48%      310ms  1.53%  runtime.sysUnusedOS
     290ms  1.43% 42.91%     1710ms  8.44%  runtime.lock2
```

分片数取 2 的幂，是为了用 hash & (count-1) 定位且确保索引合法；构造函数拒绝 0 与非 2 的幂。缓存行伪共享是不同 goroutine 修改不同变量但落在同一 CPU 缓存行，缓存一致性仍会造成争用；本轮没有专门的硬件计数器证据，不能把它当作已定位瓶颈，也没有据猜测加 padding。

## 原始输出

```text
goos: windows
goarch: amd64
pkg: Mini-Redis/database
cpu: AMD Ryzen 7 9700X 8-Core Processor
BenchmarkEngine/read90/shards1-16 	 6242461	       387.0 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards1-16 	 6216124	       382.7 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards1-16 	 6201798	       386.9 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	39639937	        58.00 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	41616379	        58.11 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	41730928	        58.12 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	67621634	        34.91 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	66645006	        34.70 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	67054089	        34.43 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/write100/shards1-16        	 6592509	       358.2 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards1-16        	 6537192	       361.6 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards1-16        	 6116497	       362.2 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	21382884	       111.0 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	22035693	       108.4 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	21928722	       109.6 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	34404063	        68.79 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	34235436	        68.48 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	33433540	        68.31 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/hot80/shards1-16           	 6165488	       381.7 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards1-16           	 6146391	       393.6 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards1-16           	 6083882	       374.5 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 6175530	       388.9 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 5992918	       402.6 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 5733904	       394.3 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 5784231	       408.2 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 5862232	       404.6 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 5980714	       405.8 ns/op	     217 B/op	       6 allocs/op
PASS
ok  	Mini-Redis/database	71.214s
```

## 2026-10-02 · 大批 TTL 回收变更的适用边界

本次新增主动删除累计 32,768 个 key 后、间隔至少 5 秒的锁外 debug.FreeOSMemory。上方九格基准没有启动 RunCleanup、没有 TTL，因此命令/锁路径未变，保留原始数字与 profile；这些数据没有测到主动 GC、归还页与大量过期并发请求的尾延迟成本，不用它证明新的内存策略没有性能代价。真实进程工作集的修复前后结果记录在当时的开发记录里，测的是内存回落，不是吞吐基准；开发记录已在 2026-11-10 的可读性改造中删除，可用 `git show 31684b7:docs/notes.md` 查看。

文档展示的 CPU 名称行已去除末尾填充空格，以通过暂存差异检查；benchmark 数值未变，任务中的原始 s5-benchmark.txt 保留原字节。

## 2026-11-10 · 可读性改造前后的交替对比

目的：确认 PR #5 的可读性改造没有让引擎变慢。代码分别是改造前的 `31684b7` 和改造后的 `dbd2e9f`，在同一台机器、同一时段交替运行：改造前 2 次 → 改造后 2 次 → 改造前 2 次 → 改造后 2 次，每个版本每格共 4 个样本。命令（默认 benchtime 1s）：

```sh
go test ./database -run '^$' -bench BenchmarkEngine -count 2 -benchmem
```

| 负载 / 分片 | 改造前中位数 | 改造后中位数 | B/op 前→后 | allocs/op 前→后 |
|---|---:|---:|---:|---:|
| read90 / 1 | 401.1 | 410.5 | 116 → 116 | 3 → 3 |
| read90 / 16 | 61.7 | 62.2 | 116 → 116 | 3 → 3 |
| read90 / 64 | 36.9 | 36.6 | 116 → 116 | 3 → 3 |
| write100 / 1 | 469.9 | 491.8 | 344 → 336 | 10 → 10 |
| write100 / 16 | 116.2 | 118.6 | 344 → 336 | 10 → 10 |
| write100 / 64 | 73.7 | 78.5 | 344 → 336 | 10 → 10 |
| hot80 / 1 | 620.7 | 585.0 | 217 → 214 | 6 → 6 |
| hot80 / 16 | 568.7 | 592.5 | 217 → 214 | 6 → 6 |
| hot80 / 64 | 574.0 | 639.1 | 217 → 214 | 6 → 6 |

结论：两个版本的差异大多在几个百分点以内，方向不一致；hot80 同一格内的样本相差可达 100 多 ns，64 分片那一格的差异落在这个波动范围里。分配次数不变，每次操作的字节数略少。样本只有 4 个，没有做统计检验，只能说没有看到稳定的退化，不能说有提升。

另外要注意：这次的改造前代码（31684b7），write100 的单分片一格和 hot80 的三格，比上面 10 月的记录慢了约 30%–60%，其余格子差别不大。原因没有查清，两次之间至少有三点不同：代码版本（10 月的表在 3d95ea3 时测得，之后 fbbce11 改过 SET 所走的写准备路径）、运行参数（10 月是 `-benchtime=2s -count=3`，这次是默认的 1 秒、共 4 次）、机器状态。所以不同日期的数字不能直接比较，比较两个版本要在同一时段、用同样的参数交替运行。

原始输出：

```text
== base round 1
BenchmarkEngine/read90/shards1-16 	 2922589	       405.7 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards1-16 	 2971801	       390.6 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	19575121	        60.53 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	20194200	        62.30 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	30166670	        37.12 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	32176843	        37.43 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/write100/shards1-16        	 2388795	       512.4 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards1-16        	 2487020	       476.9 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	10050578	       115.8 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	 9977724	       116.5 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	16125954	        74.92 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	15633854	        72.75 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/hot80/shards1-16           	 1840622	       643.1 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards1-16           	 1853622	       606.1 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 2148554	       557.8 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 2172218	       559.4 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 2135740	       569.1 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 2141821	       562.9 ns/op	     217 B/op	       6 allocs/op
== final round 1
BenchmarkEngine/read90/shards1-16 	 3027314	       397.9 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards1-16 	 3050619	       420.6 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	18782193	        62.45 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	19134391	        62.86 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	32773810	        36.72 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	31337482	        36.97 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/write100/shards1-16        	 2429778	       480.4 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards1-16        	 2423563	       492.6 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	10015539	       119.1 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	10192424	       118.1 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	15437517	        75.42 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	15742536	        74.36 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/hot80/shards1-16           	 2030330	       579.6 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards1-16           	 2045254	       590.3 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 2139626	       567.9 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 2141610	       564.0 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 1933701	       666.4 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 1861083	       609.3 ns/op	     214 B/op	       6 allocs/op
== base round 2
BenchmarkEngine/read90/shards1-16 	 3044316	       396.5 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards1-16 	 3046754	       420.7 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	19341954	        61.63 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	18920640	        61.68 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	31613888	        36.70 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	32615431	        36.05 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/write100/shards1-16        	 2583469	       456.3 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards1-16        	 2672353	       462.8 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	10098638	       118.4 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	10270041	       115.4 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	15722940	        73.96 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	15258536	        73.37 ns/op	     344 B/op	      10 allocs/op
BenchmarkEngine/hot80/shards1-16           	 1909767	       623.9 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards1-16           	 1942180	       617.5 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 2074695	       578.8 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 2065156	       578.0 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 2044545	       578.9 ns/op	     217 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 2051691	       601.4 ns/op	     217 B/op	       6 allocs/op
== final round 2
BenchmarkEngine/read90/shards1-16 	 3039547	       400.4 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards1-16 	 3025198	       424.5 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	18854523	        61.86 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards16-16         	18916762	        61.74 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	32078185	        36.41 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/read90/shards64-16         	32476231	        36.47 ns/op	     116 B/op	       3 allocs/op
BenchmarkEngine/write100/shards1-16        	 2220385	       498.5 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards1-16        	 2363072	       490.9 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	 9904454	       117.1 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards16-16       	 8733394	       124.7 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	14739534	        82.88 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/write100/shards64-16       	14880841	        81.63 ns/op	     336 B/op	      10 allocs/op
BenchmarkEngine/hot80/shards1-16           	 1745953	       641.7 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards1-16           	 2511796	       525.2 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 2544890	       617.1 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards16-16          	 1693614	       649.7 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 1905766	       611.8 ns/op	     214 B/op	       6 allocs/op
BenchmarkEngine/hot80/shards64-16          	 1739542	       714.8 ns/op	     214 B/op	       6 allocs/op
```
