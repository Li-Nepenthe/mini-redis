# 第 05 章：时间、select 与 GC

本章在项目里对应 **key 的过期**：访问时发现过期就删（惰性删除），后台每 250 毫秒抽样删一批（定期删除），大批删除后请运行时把内存还给操作系统。代码都在 [database/expire.go](../../database/expire.go)。

## 这一章能回答的面试题

1. Go 的 GC 是怎样工作的？什么是三色标记？为什么需要写屏障？（八股 1）
2. 什么是 STW？Go 的 GC 在哪些阶段会 STW？（八股 1）
3. GC 什么时候触发？GOGC 和 GOMEMLIMIT 是什么？怎样调优？（八股 2）
4. 什么是逃逸分析？变量什么时候分配在堆上？（八股 3）
5. 删了很多数据，进程的内存为什么不降？（八股 4）
6. Timer 和 Ticker 有什么区别？用完不 Stop 会怎样？Go 1.23 改了什么？（八股 5）
7. select 有多个 case 同时就绪时选哪个？default 有什么用？
8. （配角）Redis 是怎样删除过期 key 的？

---

## 跑起来看什么

启动服务端和客户端：

```text
mini-redis> SET session abc
OK
mini-redis> EXPIRE session 5
(integer) 1
mini-redis> TTL session
(integer) 5
```

等 5 秒以后再输入：

```text
mini-redis> TTL session
(integer) -2
mini-redis> GET session
(nil)
```

TTL 返回 -2 表示 key 不存在，-1 表示 key 存在但没有过期时间。

一个 key 过期以后，有两条路会把它删掉：

- 有人来读它：读命令发现它过期了，当场删掉，当作不存在。这叫**惰性删除**。
- 一直没人读：后台每 250 毫秒随机抽查一批带过期时间的 key，过期的删掉。这叫**定期删除**。

大批 key 删掉以后，还要把它们占的内存还给操作系统，这就和 GC 有关了。

---

## Go 基础

### 1. time 包

```go
package main

import (
	"fmt"
	"time"
)

func main() {
	d := 1500 * time.Millisecond // Duration 本质是 int64 纳秒
	fmt.Println(d, d.Seconds(), d > time.Second)

	start := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	deadline := start.Add(100 * time.Second)
	fmt.Println(deadline.Sub(start), deadline.After(start))

	ms := deadline.UnixMilli() // 转成毫秒时间戳，适合写进文件
	back := time.UnixMilli(ms).UTC()
	fmt.Println(back.Equal(deadline))
}

// 输出：
// 1.5s 1.5 true
// 1m40s true
// true
```

- `time.Duration` 是 int64 纳秒，可以直接和整数常量相乘：`250 * time.Millisecond`。
- 两个 `time.Time` 比较先后用 `Before`、`After`，判断相等用 `Equal`，不要用 `==`（`==` 还会比较时区等内部字段）。
- 测量耗时用 `time.Since(start)`。`time.Now()` 里带着单调时钟读数，即使系统时间被调整，算出来的耗时也不受影响。

### 2. Ticker、Timer 和 select

- `time.NewTicker(d)`：每隔 d 往 `ticker.C` 发一次当前时间，直到 `Stop`。
- `time.NewTimer(d)`：d 之后往 `timer.C` 发一次。`time.After(d)` 是它的简写，直接返回 channel。
- `select` 同时等待多个 channel 操作，哪个先就绪就执行哪个。

后台定时任务的标准写法：

```go
package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	ticks := 0
	for {
		select {
		case <-ctx.Done(): // 收到停止信号就退出循环
			fmt.Println("收到", ticks, "次 tick，然后退出：", ctx.Err())
			return
		case <-ticker.C: // 每 100 毫秒执行一次
			ticks++
		}
	}
}

// 输出：收到 3 次 tick，然后退出： context deadline exceeded
```

`context` 第 07 章细讲，这里只需要知道：`ctx.Done()` 是一个 channel，被取消或者超时时会被关闭，`<-ctx.Done()` 就能收到。

### 3. select 的规则

```go
package main

import "fmt"

func main() {
	a := make(chan int, 1)
	b := make(chan int, 1)
	picked := map[string]int{}
	for range 1000 {
		a <- 1
		b <- 1
		select { // 两个都就绪：随机选一个
		case <-a:
			picked["a"]++
			<-b
		case <-b:
			picked["b"]++
			<-a
		}
	}
	fmt.Println("a、b 都被选中过：", picked["a"] > 0 && picked["b"] > 0)

	ch := make(chan int) // 没有缓冲，也没有接收者
	select {
	case ch <- 1:
		fmt.Println("发出去了")
	default: // 其他 case 都没就绪时，立刻执行 default，不阻塞
		fmt.Println("没人接收，走 default")
	}
}

// 输出：
// a、b 都被选中过： true
// 没人接收，走 default
```

| 情况 | 结果 |
|---|---|
| 一个 case 就绪 | 执行它 |
| 多个 case 同时就绪 | **均匀随机**选一个。这是为了防止排在前面的 case 永远优先，后面的饿死 |
| 都没就绪，有 default | 立刻执行 default |
| 都没就绪，没有 default | 阻塞，直到有一个就绪 |
| case 里的 channel 是 nil | 这个 case 永远不会就绪，相当于被禁用了 |
| `select {}` | 永远阻塞 |

---

## 代码走读：database/expire.go

### 1. 两个结构：一个 map，一个切片

```go
type shard struct {
	mu       sync.RWMutex
	data     map[string]any
	expires  map[string]expiration
	expiring []string
}

type expiration struct {
	deadline time.Time
	index    int
}
```

- `expires` 回答“这个 key 什么时候过期”：读命令、TTL 都查它。
- `expiring` 是同一批 key 的切片，供后台**随机抽查**：随机取一个下标，O(1)。map 做不到这一点，Go 没有“从 map 里随机取一个元素”的操作，遍历 map 虽然起点随机，但要取第 k 个元素只能从头数。
- `expiration.index` 记录这个 key 在切片里的位置，删除时才能 O(1) 找到它。

### 2. 设置和清除期限

```go
func (s *shard) setExpiration(key string, deadline time.Time) {
	if old, ok := s.expires[key]; ok {
		old.deadline = deadline
		s.expires[key] = old
		return
	}
	s.expires[key] = expiration{deadline: deadline, index: len(s.expiring)}
	s.expiring = append(s.expiring, key)
}

func (s *shard) clearExpiration(key string) {
	old, ok := s.expires[key]
	if !ok {
		return
	}
	last := len(s.expiring) - 1
	movedKey := s.expiring[last]
	s.expiring[old.index] = movedKey
	s.expiring[last] = ""
	s.expiring = s.expiring[:last]
	delete(s.expires, key)
	if old.index < last {
		moved := s.expires[movedKey]
		moved.index = old.index
		s.expires[movedKey] = moved
	}
}
```

- **续期**只改时间，不再往切片里加一次，否则同一个 key 会在抽样池里出现两次。注意 `old.deadline = deadline` 改的是从 map 里取出来的**拷贝**，所以还要 `s.expires[key] = old` 存回去（第 03 章：map 的元素不能取地址）。
- **删除**用的是“末项补洞”：把切片最后一个元素搬到要删的位置，再截掉最后一格，O(1)。抽样池不要求顺序，所以可以这样做。被搬动的那个 key 位置变了，要同步改它的 `index`。
- `s.expiring[last] = ""`：截短切片之前先把最后一格清空。截短以后这一格看不见了，但还在底层数组里，不清空的话它引用的字符串就回收不掉（第 01 章：切片和底层数组）。

### 3. 惰性删除

```go
func (s *shard) purgeExpired(key string, now time.Time) bool {
	entry, ok := s.expires[key]
	if !ok || now.Before(entry.deadline) {
		return false
	}
	delete(s.data, key)
	s.clearExpiration(key)
	return true
}
```

- 读命令经 `lockRead` 调用它（第 04 章）；写命令在 `executeWrite` 里，对每个涉及的 key 先调用一次，再计算写计划；EXISTS 也会逐个调用。
- 数据和期限一起删，不能只删一个。
- 惰性删除**不写 AOF**。重启时靠日志里记录的绝对过期时间，重放后自然还是过期的（第 06 章）。

### 4. 定期删除

```go
func (e *Engine) RunCleanup(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	var lastReclaim time.Time
	deleted := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted += e.cleanupExpired()
			if deleted >= cleanupReclaimKeys && time.Since(lastReclaim) >= cleanupReclaimInterval {
				if ctx.Err() != nil {
					return
				}
				e.reclaim()
				deleted = 0
				lastReclaim = time.Now()
			}
		}
	}
}

func (e *Engine) cleanupExpired() int {
	now := e.now()
	deleted := 0
	for _, s := range e.shards {
		s.mu.Lock()
		for i := 0; i < cleanupSamples && len(s.expiring) > 0; i++ {
			key := s.expiring[rand.IntN(len(s.expiring))]
			if s.purgeExpired(key, now) {
				deleted++
			}
		}
		s.mu.Unlock()
	}
	return deleted
}
```

- `RunCleanup` 就是 Go 基础第 2 小节的标准写法：一个 Ticker，一个 select，ctx 取消就退出。
- `cleanupExpired` 每个分片拿一次写锁，最多随机抽 256 次（`cleanupSamples`），抽完立刻放锁，再去下一个分片。一次只锁一个分片，其他分片的命令照常执行。
- 删得够多（`cleanupReclaimKeys` = 32768）、而且距离上次至少 5 秒，才调用 `e.reclaim()`。它平时就是 `debug.FreeOSMemory`，调用时**不持有任何分片锁**。调用前再看一眼 ctx：已经在停机了，就不必再做一次完整的 GC。

---

## Go 八股

### 八股 1：Go 的 GC 是怎样工作的？

#### 一分钟口述版

> Go 用的是并发的标记清除算法，不分代，也不移动对象。标记用三色抽象：白色是还没访问到的对象，灰色是自己被访问到了、但它引用的对象还没扫描，黑色是自己和它引用的对象都扫描完了。GC 从根出发，也就是全局变量和各个 goroutine 的栈，先把根引用的对象标成灰色，然后不断取出灰色对象，把它引用的对象标灰，自己变黑，直到没有灰色对象。剩下的白色对象就是垃圾，在清除阶段回收。标记是和业务代码并发进行的，业务代码可能一边被扫描一边修改指针，比如把一个白色对象挂到已经扫描完的黑色对象上，同时删掉灰色对象对它的引用，这个白色对象就会被漏标，然后被错误地回收。写屏障就是为了防止这种情况：标记期间每次写指针，都要通知 GC。Go 从 1.8 起用的是混合写屏障，把被覆盖的旧指针和写入的新指针都标灰。这样栈就不需要在标记结束时重新扫描，STW 停顿缩短到了通常不到 1 毫秒。整个过程只有两次很短的 STW：开始标记前开启写屏障，标记结束时收尾。

#### 要点

**四个阶段**（`runtime/mgc.go` 开头的注释）：

| 阶段 | 是否 STW | 做什么 |
|---|---|---|
| 1. 清除终止 | **是**（第一次 STW） | 让所有 P 停在安全点；如果上一轮还有没清除完的，先清完；然后**开启写屏障**。写屏障必须在所有 P 上都开启以后才能开始扫描，所以这一步要在 STW 里做 |
| 2. 标记 | 否，并发 | 恢复运行。扫描栈、全局变量等根，然后处理灰色对象，直到没有灰色对象。扫描某个 goroutine 的栈时只暂停这一个 goroutine。这期间新分配的对象直接标成黑色 |
| 3. 标记终止 | **是**（第二次 STW） | 关闭标记工作，做一些收尾 |
| 4. 清除 | 否，并发 | 关闭写屏障；后台逐步回收白色对象占用的内存，分配内存时也会顺带清除一些 |

**为什么需要写屏障**：并发标记时，如果同时满足下面两个条件，白色对象就会被漏掉：
1. 一个黑色对象新增了指向这个白色对象的指针；
2. 所有能到达这个白色对象的灰色路径都被删掉了。

黑色对象不会再被扫描，这个白色对象也就永远不会被标记，最后被当成垃圾回收，程序访问它时就出错了。

**混合写屏障**（`runtime/mbarrier.go`，Go 1.8 起）结合了两种写屏障：
- Yuasa 式的删除屏障：把**被覆盖的旧指针**指向的对象标灰，破坏条件 2；
- Dijkstra 式的插入屏障：把**新写入的指针**指向的对象标灰，破坏条件 1。

好处是：栈上的指针写入不需要屏障（栈上的写入太频繁，加屏障的代价太大），也不需要在标记终止时重新扫描所有栈。以前重新扫描栈是 STW 时间长的主要原因。

**辅助标记（mark assist）**：如果某个 goroutine 分配内存太快，GC 标记跟不上，这个 goroutine 在分配时会被要求先帮着做一些标记工作。所以 GC 期间，分配得多的请求会变慢。

**Green Tea GC**（Go 1.25 作为实验加入，Go 1.26 起默认开启）：算法仍然是三色标记，变化在**扫描的顺序**。以前是一个对象一个对象地扫描，在内存里到处跳；Green Tea 先把同一个内存页（span）上的待扫描对象攒起来，再一次扫描完，对 CPU 缓存更友好。这是对实现的优化，面试时讲清三色标记和写屏障就够了，能提一句会是加分项。

> **项目里没有直接体现，但面试常问**：三色标记、写屏障、各阶段的 STW，都在运行时内部。Mini-Redis 的代码里和 GC 直接相关的只有一处：大批 key 过期以后主动请求回收内存（八股 4）。

#### 怎样亲眼看 GC

设置环境变量 `GODEBUG=gctrace=1`，每次 GC 结束时运行时会往标准错误打印一行。下面是本机运行一个分配了 200 MB 短命对象的小程序时的输出（节选）：

```text
gc 26 @0.021s 0%: 0+0.53+0 ms clock, 0+0/0.53/0+0 ms cpu, 35->36->19 MB, 37 MB goal, 0 MB stacks, 0 MB globals, 16 P
gc 27 @0.023s 0%: 0+0+0 ms clock, 0+0/0/0+0 ms cpu, 37->37->0 MB, 40 MB goal, 0 MB stacks, 0 MB globals, 16 P (forced)
```

| 字段 | 意思 |
|---|---|
| `gc 26` | 第 26 次 GC |
| `@0.021s` | 程序启动后 0.021 秒 |
| `0%` | 程序启动以来，GC 占用的时间比例 |
| `0+0.53+0 ms clock` | 三个阶段的实际耗时：STW 清除终止 + 并发标记 + STW 标记终止。两头的 STW 都接近 0 |
| `35->36->19 MB` | GC 开始时的堆大小 → 结束时的堆大小 → 其中存活的部分 |
| `37 MB goal` | 这一轮的目标堆大小，见八股 2 |
| `16 P` | 用了几个 P |
| `(forced)` | 这次 GC 是代码主动调用 `runtime.GC()` 或 `debug.FreeOSMemory()` 触发的 |

Windows 的计时精度较低，很短的阶段会显示成 0。

#### 常见追问

**追问 1：Go 的 GC 为什么不分代、不压缩？**

分代假设“大部分对象很快就死”，Go 的逃逸分析已经把很多短命对象放在了栈上，堆上分代带来的好处没那么大；而分代和压缩都要移动对象，需要更复杂的写屏障，还会让 cgo 传出去的指针失效。Go 选择了低延迟和实现简单。

**追问 2：GC 的 STW 时间和堆大小有关吗？**

基本无关。两次 STW 做的事都很少，耗时通常在 1 毫秒以内。和堆大小有关的是并发标记阶段的 CPU 开销。

---

### 八股 2：GC 什么时候触发？GOGC 和 GOMEMLIMIT 怎样调？

#### 一分钟口述版

> GC 主要由堆的增长触发。每轮 GC 结束时，运行时会算出下一轮的目标堆大小：上一轮标记后存活的堆，加上“存活堆、栈和全局变量”乘以 GOGC 除以 100。GOGC 默认是 100，大致可以理解成：存活 100 MB，堆长到 200 MB 左右就开始下一轮。调度器还会根据分配速度提前一点开始标记，争取在到达目标时正好标完。此外，两分钟没有 GC 会强制执行一次；代码也可以调用 `runtime.GC()` 手动触发。调优主要是两个参数：GOGC 调大，GC 次数少，CPU 开销低，但内存占用高；调小则相反。Go 1.19 加入了 GOMEMLIMIT，是一个软性的内存上限，堆接近这个上限时，即使没到 GOGC 算出的目标，也会更积极地 GC。常见的用法是在容器里把 GOMEMLIMIT 设成内存限制的九成左右，既不会因为 GOGC 太大被 OOM 杀掉，平时也不必频繁 GC。

#### 要点

**目标堆大小**（`runtime/mgcpacer.go`）：

```text
目标 = 上一轮存活的堆 + (上一轮存活的堆 + 栈 + 全局变量) × GOGC / 100
```

- 把栈和全局变量算进去，是 Go 1.18 加的。
- 有一个下限：4 MB × GOGC / 100。所以很小的程序，堆到 4 MB 之前不会 GC。

**触发方式：**

| 方式 | 说明 |
|---|---|
| 堆接近目标 | 最常见。根据分配速度提前开始，争取标记完成时堆刚好到目标 |
| 定时 | 超过 2 分钟没有 GC，强制执行一次 |
| 手动 | `runtime.GC()`；`debug.FreeOSMemory()` 会先 GC，再尽量把内存还给操作系统 |

**两个参数：**

| 参数 | 作用 | 设置方式 |
|---|---|---|
| `GOGC` | 默认 100；`off` 表示关闭按比例触发的 GC | 环境变量，或者 `debug.SetGCPercent` |
| `GOMEMLIMIT` | 软性内存上限（Go 1.19 起），默认不限制 | 环境变量，例如 `GOMEMLIMIT=900MiB`；或者 `debug.SetMemoryLimit` |

GOMEMLIMIT 是“软”的：存活的数据本身就超过了上限，它也没有办法，只会不停地 GC。为了不让程序卡死在 GC 上，运行时对这种情况下 GC 占用的 CPU 设了一个上限，大约 50%。

**减少 GC 压力，最有效的办法是少分配**：预先分配切片容量、复用缓冲区（`sync.Pool`）、避免不必要的逃逸（八股 3）。调参数是次要的手段。

#### 项目里的例子

- Mini-Redis 没有调 GOGC 和 GOMEMLIMIT，用的是默认值。
- 存储里的每个值都是一个独立的 `[]byte`，每个列表节点也是一次分配。key 很多时，GC 要扫描的对象就很多。这是选择“一个值一块内存”这种简单结构的代价。Redis 是 C 写的，没有 GC，但它为了省内存同样做了很多紧凑编码（第 03 章配角）。

#### 常见追问

**追问：GOGC=off 加上 GOMEMLIMIT，会怎样？**

只有堆接近内存上限时才 GC。适合内存充足、希望 GC 尽量少的批处理程序。风险是存活数据一旦逼近上限，就会频繁 GC。

---

### 八股 3：什么是逃逸分析？

#### 一分钟口述版

> 变量放在栈上还是堆上，Go 程序员不需要决定，由编译器的逃逸分析决定。原则是：编译器能证明一个变量在函数返回后不会再被用到，就放在栈上，函数返回时自动回收，完全不给 GC 增加负担；证明不了，就放到堆上，由 GC 回收，这叫“逃逸”。常见的逃逸情况有：返回局部变量的地址；把变量存进全局变量、存进堆上对象的字段、发到 channel 里；被闭包捕获，而闭包逃逸了；把值装进接口，传给编译器看不透的函数，比如 fmt.Println；变量太大，或者切片的大小在编译期不知道。用 `go build -gcflags=-m` 可以看到编译器对每个变量的判断。逃逸本身不是错误，只是有分配和 GC 的成本，在热点路径上才值得优化。

#### 要点

```go
package main

import "fmt"

type point struct{ x, y int }

var keep *point

func onStack() int {
	p := point{1, 2} // 不逃逸：函数返回后没人再用它
	return p.x + p.y
}

func onHeap() *point {
	p := point{3, 4} // 逃逸：返回了它的地址，函数返回后还要用
	return &p
}

func main() {
	keep = onHeap()
	fmt.Println(onStack(), *keep)
}

// 输出：3 {3 4}
```

`go build -gcflags=-m` 的输出（节选，本机实测）：

```text
./main.go:9:6: can inline onStack
./main.go:14:6: can inline onHeap
./main.go:15:2: moved to heap: p
```

- `moved to heap: p`：onHeap 里的 p 逃逸到了堆上。
- onStack 里的 p 没有出现，说明它留在栈上。
- `can inline`：函数很小，会被内联到调用处。内联以后，编译器能看到更多上下文，有时能避免逃逸。

**常见的逃逸原因：**

| 写法 | 为什么逃逸 |
|---|---|
| `return &x` | 函数返回后还要用 |
| 存进全局变量，或者堆上对象的字段 | 生命周期超过当前函数 |
| 发送到 channel | 接收方在另一个 goroutine，什么时候用完不知道 |
| 被闭包捕获，而闭包逃逸了 | 例如闭包被 `go` 启动，或者被返回 |
| 装进接口，传给 `fmt.Println` 这类函数 | 编译器看不透接口背后的调用会不会保存它 |
| `make([]byte, n)`，n 不是常量且不够小 | 编译期不知道大小，栈帧放不下 |
| 太大的变量 | 隐式分配的变量超过 64 KB，放到堆上 |

#### 项目里的例子

- **写计划里的 `apply` 闭包会逃逸。** [database/write.go](../../database/write.go) 的 `planSet` 返回一个 `writePlan`，里面的 `apply` 是一个捕获了分片、key 和 value 的闭包，它在函数返回后才被调用，所以闭包和它捕获的变量都在堆上（动手实验 2 用 `-gcflags=-m` 亲眼看）。每条写命令多一次小分配，换来的是“先算计划、日志落盘后再执行”这个清楚的结构。这个取舍是值得的：开启 AOF 时写命令的耗时主要在 fsync 上，多一次分配完全可以忽略。
- **存储里的值本来就该在堆上。** SET 存的值要一直活到被删除，肯定逃逸，这不是问题。

#### 常见追问

**追问：怎样减少逃逸？**

先用 `-gcflags=-m` 和 benchmark 的 `-benchmem`（第 08 章）确认热点路径上确实有大量分配，再针对性地改：传值代替传指针（小结构体）、预先分配好缓冲区让调用方传进来、避免在热点路径上把值装进接口。不要为了避免逃逸把代码改得难懂。

---

### 八股 4：删了很多数据，进程的内存为什么不降？

#### 一分钟口述版

> 这个问题要分几层看。第一层，数据结构本身不缩小：Go 的 map 删除元素以后，表的大小不变，连 `clear` 也不会缩小；切片截短以后，底层数组也还在。第二层，GC 还没运行：GC 是按堆增长触发的，删完数据以后如果程序很闲，没有新的分配，可能要等两分钟的强制 GC 才会回收。第三层，GC 回收了，但运行时不会立刻把内存还给操作系统，而是留着下次分配时复用；后台有一个清道夫（scavenger）慢慢地把长时间不用的内存页还回去。第四层，看的指标对不对：Linux 上 Go 1.16 之前用 MADV_FREE 归还内存，操作系统不急着回收，进程的 RSS 看起来不降。要立刻归还，可以调用 `debug.FreeOSMemory()`，它会强制 GC，然后尽量把空闲内存都还给操作系统；但它有一次完整 GC 的成本，不能频繁调用。

#### 要点

| 层次 | 现象 | 怎么办 |
|---|---|---|
| 数据结构不缩小 | map 删除后表不缩小；大数组被一个小切片引用着 | 必要时重建一个新 map 或拷贝出小切片，让旧的被回收 |
| GC 还没运行 | 程序空闲，没有分配，到不了目标堆大小 | 等 2 分钟的强制 GC，或者主动 `runtime.GC()` |
| 运行时留着内存复用 | 堆已经回收，但进程的内存占用没变 | 后台 scavenger 会逐步归还；`debug.FreeOSMemory()` 立刻归还 |
| 观察的指标 | RSS 不能及时反映归还 | Go 1.16 起 Linux 默认用 MADV_DONTNEED，RSS 会立刻下降（`GODEBUG=madvdontneed`） |

用 `runtime.ReadMemStats` 可以分开看：`HeapAlloc` 是堆上已分配的字节数（包括还没回收的垃圾），`HeapReleased` 是已经还给操作系统的字节数。

#### 项目里的例子

这正是 Mini-Redis 后台清理任务要处理的问题：

- **为什么要主动归还**：一个缓存服务，大批 key 过期以后可能正好没有什么写入，没有新分配，GC 就不会运行，过期数据占的内存一直不还。所以 `RunCleanup` 累计删除至少 32768 个 key（常量 `cleanupReclaimKeys`）、并且距离上次归还至少 5 秒（`cleanupReclaimInterval`）时，调用一次 `debug.FreeOSMemory()`。
- **为什么不每次都调用**：它会做一次完整的 GC，有 STW，也有 CPU 开销。所以设了数量和时间两道门槛。
- **为什么在锁外调用**：GC 期间如果还拿着分片锁，所有访问这个分片的命令都要等 GC 结束。[database/cleanup_test.go](../../database/cleanup_test.go) 的 `TestLargeExpirationReclaimsOutsideShardLocks` 专门检查“归还内存发生在所有分片锁之外”。
- **它还管不到的地方**：分片的 map 删除 key 以后不会缩小，所以能归还的主要是值（`[]byte`、链表节点）占的内存，map 自己的表还留着。这是 Go map 的特性，要彻底解决只能定期重建 map。Mini-Redis 没有做，因为 key 的数量通常会再涨回来，表会被复用。

另一处细节：`clearExpiration` 从抽样用的切片里删一个 key 时，用最后一个元素填补空位，并把原来最后一格**设成空字符串**，再截短切片。截短以后，最后那一格虽然看不见了，但还在底层数组里；如果不清空，它引用的字符串就一直不能被 GC 回收。代码走读里会看到这一行 `s.expiring[last] = ""`。

#### 常见追问

**追问：容器里看到 Go 程序内存一直很高，怎样判断是泄漏还是正常？**

先看 `HeapAlloc` 或者 pprof 的 heap profile（第 08 章）：存活的堆在持续增长，才是泄漏，常见原因是 goroutine 泄漏（第 07 章）、全局 map 只增不删、大数组被小切片拖住。存活的堆稳定，只是 RSS 高，多半是运行时留着内存复用，可以用 GOMEMLIMIT 约束。

---

### 八股 5：Timer 和 Ticker 有什么区别？Go 1.23 改了什么？

#### 一分钟口述版

> Timer 是一次性的，到时间往它的 channel 发一个值；Ticker 是周期性的，每隔一段时间发一次，直到调用 Stop。接收方处理得慢，Ticker 不会攒一堆值等你，而是调整间隔或者丢掉一些 tick。Go 1.23 对计时器做了两处重要的改动。第一，没被引用的 Timer 和 Ticker，即使没有 Stop，也能被 GC 回收。以前在循环里写 `time.After`，每次循环都会创建一个计时器，它要等到时间到了才能被回收，在高频循环里会积累大量内存，现在没有这个问题了。第二，计时器的 channel 变成了同步的。以前它有一个缓冲位，Stop 或 Reset 返回以后，channel 里可能还留着一个过期的旧值，下次接收会读到它，很容易出 bug；现在 Stop 和 Reset 返回后，保证不会再收到旧值。这两个新行为由 go.mod 里的 go 版本决定是否启用，Go 1.27 已经删掉了恢复旧行为的开关。

#### 要点

| | Timer | Ticker |
|---|---|---|
| 触发 | 一次 | 周期性，直到 Stop |
| 创建 | `time.NewTimer(d)`、`time.After(d)`、`time.AfterFunc(d, f)` | `time.NewTicker(d)`、`time.Tick(d)` |
| 重新设置 | `Reset(d)` | `Reset(d)`（Go 1.15 起） |
| 接收方太慢 | | 调整间隔或丢掉 tick，不会积压 |

**Go 1.23 的两处变化**（由 go.mod 的 go 版本 ≥ 1.23 启用）：
1. 没被引用的计时器可以被 GC 回收，不再需要靠 Stop 来释放内存。不过要让计时器**不再触发**，仍然要调用 Stop。
2. channel 变成同步的：`Stop` 或 `Reset` 返回后，不会再收到旧值。

Go 1.27 删除了 `asynctimerchan` 这个 GODEBUG 开关，旧的异步行为已经无法恢复。

#### 项目里的例子

- `RunCleanup` 用 `time.NewTicker(cleanupInterval)`（250 毫秒），然后 `defer ticker.Stop()`。Mini-Redis 是 `go 1.26.0`，不写 Stop 内存也会被回收；但写上能让“这个 ticker 在函数返回后不再触发”这件事一目了然。
- 为什么不给每个 key 设一个 Timer：100 万个带过期时间的 key 就要 100 万个计时器，内存和调度开销都太大。Mini-Redis 只用一个 Ticker 驱动后台抽样，再加上访问时的惰性检查，这和 Redis 的思路一样（见下面的配角）。

---

### 配角：Redis 怎样删除过期的 key

#### 一分钟口述版

> Redis 同时用两种策略。惰性删除：访问一个 key 时，先检查它过没过期，过期了就删掉，当作不存在。只用惰性删除的话，一个过期以后再也没人访问的 key 会永远占着内存，所以还有定期删除：后台定时从设置了过期时间的 key 里随机抽一批，删掉其中已过期的；如果这一批里过期的比例很高，说明过期的 key 还很多，就接着再抽一轮，同时限制每次执行的时间，防止阻塞正常请求。两种策略结合，CPU 和内存都能兼顾。即便如此，还是可能有过期 key 没被删掉，内存满了的时候，由内存淘汰策略（比如 LRU）兜底。

#### 和 Mini-Redis 对照

| | Redis | Mini-Redis |
|---|---|---|
| 惰性删除 | 访问时检查 | 一样：读命令通过 `lockRead` 检查，写命令在执行前清理过期的 key |
| 定期删除 | 后台定时随机抽样；过期比例高就继续抽；有时间上限 | 每 250 毫秒，每个分片最多随机检查 256 次；不根据过期比例追加 |
| 随机抽样怎样做到 O(1) | 从哈希表里随机取 | 每个分片维护一个“设置了期限的 key”的切片，随机取下标；删除时用最后一个元素填补空位，也是 O(1) |
| 内存满了 | 按 maxmemory 策略淘汰 | 不支持 |

**为什么每个分片最多检查 256 次**：清理要拿分片的写锁，检查次数不设上限的话，一个有几百万个过期 key 的分片会被锁住很久，所有访问它的命令都要等。设了上限，一轮清不完就留给下一轮，换来的是请求延迟可控。

---

## 关键设计与为什么

**1. 惰性删除加定期抽样，不给每个 key 开计时器。**
- 100 万个带期限的 key 就要 100 万个计时器，内存和调度的开销都太大。一个 Ticker 驱动后台抽样，加上访问时的检查，已经够用。
- 代价：过期的 key 不是“到点就消失”，而是“到点以后，下次被读到或被抽到时消失”。对客户端来说没有区别，因为读到时一定会被当成不存在。

**2. 每个分片每轮最多抽 256 次，抽完就放锁。**
- 清理要拿分片的写锁。不设上限，一个分片里有大量过期 key 时，清理会长时间占着锁。
- 和 Redis 的区别：Redis 发现抽到的过期比例高，会接着再抽一轮；Mini-Redis 不追加，一轮清不完就等下一个 250 毫秒。动手实验 1 里，10 万个同时过期的 key 大约 6 秒清完。

**3. 大批删除后主动归还内存，但设了两道门槛，并且在锁外调用。**
- 删得少时不值得做一次完整的 GC；连续大量删除时，最多 5 秒做一次。
- `debug.FreeOSMemory` 会暂停所有分配者一小段时间。在锁外调用，正在等锁的命令不会被它拖住。

**4. 时间来自一个可替换的函数 `e.now`。**
- 平时是 `time.Now`，测试里换成一个可以手动拨动的时钟（[database/ttl_test.go](../../database/ttl_test.go) 的 `clockEngine`）。“设置 5 秒期限，拨到 4.6 秒，TTL 应该是 0；拨到 5 秒，key 应该消失”，这样的测试不用真的等，也不会因为机器慢而偶尔失败。

**5. TTL 按半秒四舍五入显示，删除按精确时间。**
- 剩 4.6 秒时 TTL 显示 5；剩 0.4 秒时显示 0，但 key 还活着。显示和判断分开，判断永远用精确的期限。

---

## 动手实验

**实验 1：亲眼看定期删除和强制 GC。**

项目里有一个用真实时间的测试：写入 10 万个 5 秒后过期的 key，之后一次也不读，看后台清理多久能把它们删完。打开 gctrace 运行它：

```text
GODEBUG=gctrace=1 go test ./database -run TestExpireHundredThousandKeysWithoutReads -v -count=1
```

PowerShell 里先执行 `$env:GODEBUG = "gctrace=1"`，再运行 `go test`，做完记得 `Remove-Item Env:GODEBUG`。

输出的最后几行（2026-11-10 实测）：

```text
gc 15 @0.123s 1%: 0+0.99+0 ms clock, 0+0/3.9/9.9+0 ms cpu, 62->62->38 MB, 65 MB goal, 0 MB stacks, 0 MB globals, 16 P
gc 16 @0.148s 1%: 0+0.99+0 ms clock, 0+0/3.9/10+0 ms cpu, 68->68->43 MB, 76 MB goal, 0 MB stacks, 0 MB globals, 16 P (forced)
gc 17 @7.004s 0%: 0+1.0+0 ms clock, 0+0/3.7/5.9+0 ms cpu, 43->43->34 MB, 87 MB goal, 0 MB stacks, 0 MB globals, 16 P (forced)
gc 18 @11.311s 0%: 0+0.51+0 ms clock, 0+0/1.5/0+0 ms cpu, 34->34->14 MB, 68 MB goal, 0 MB stacks, 0 MB globals, 16 P (forced)
    ttl_test.go:206: 100000 keys with 5s TTL, no reads: all removed in 11.162s after writes; live HeapAlloc 45940200 -> 14740712 bytes; map capacity may remain
--- PASS: TestExpireHundredThousandKeysWithoutReads (11.31s)
```

逐行看：

- **gc 15 之前**：写入 10 万个 key 时，堆不断变大，GC 按目标堆大小自动触发了十几次。
- **gc 16 @0.148s (forced)**：测试写完以后主动调用 `runtime.GC()`，记下此时存活的堆，43 MB。
- **0.148s 到 7.004s 之间一次 GC 都没有**：测试只是在等，几乎不分配内存，GC 就不会自己运行。这就是八股 4 说的“删了数据，GC 却还没运行”。
- **gc 17 @7.004s (forced)**：key 在 5 秒左右开始过期，后台清理删到第 32768 个时，调用了 `debug.FreeOSMemory`，于是有了这次强制 GC。存活的堆从 43 MB 降到 34 MB。
- **gc 18 @11.311s (forced)**：10 万个 key 全部删完后，测试再调用一次 `runtime.GC()`，存活的堆降到 14 MB。剩下的主要是 map 自己的表，它不会缩小（日志里的 “map capacity may remain”）。
- 两次强制 GC 之间相隔 4 秒多，不到 5 秒，所以清理任务没有再调用第二次 `FreeOSMemory`。
- 从开始过期到全部删完，大约用了 6 秒。可以算一下：每轮每个分片最多抽 256 次，16 个分片就是 4096 次，每秒 4 轮，大约每秒删 1.6 万个，10 万个要 6 秒左右。

**实验 2：看写路径上哪些东西逃逸到了堆上。**

```text
go build -gcflags=-m ./database 2>&1 | grep "write.go" | grep "escape"
```

节选（2026-11-10 实测）：

```text
database/write.go:112:37: StatusReply("OK") escapes to heap
database/write.go:112:66: func literal escapes to heap
database/write.go:114:17: value escapes to heap
database/write.go:135:31: int64((~r0) + len(args) - 2) escapes to heap
database/write.go:135:83: func literal escapes to heap
database/write.go:140:18: &node{...} escapes to heap
database/write.go:208:25: value does not escape
```

- `func literal escapes to heap`：`planSet`（112 行）、`planLPush`（135 行）返回的 `apply` 闭包都逃逸了，原因见八股 3。
- `value escapes to heap`：SET 的值要存进 map，一直活到被删除，本来就该在堆上。
- `StatusReply("OK")` 和 `int64(...)` 也逃逸了：它们被装进了 `writePlan.reply` 这个 `any` 字段。这就是第 03 章说的“值装进接口可能要分配”。
- `&node{...}`：链表节点要一直挂在列表上，也在堆上。
- 最后一行是反例：`parseExpirySeconds` 的参数 `value` 只在函数里读一下，就留在了栈上。

---

## 常见误区

- **用 `==` 比较两个 time.Time。** 应该用 `Equal`。
- **以为 select 会按 case 的书写顺序优先。** 多个就绪时是随机的。需要优先级，要写两层 select。
- **在 for-select 里用 `time.After` 做超时，并且以为每次循环都会泄漏。** Go 1.23 起不会泄漏了；但每次循环都新建计时器，超时是从这次循环开始算的，不是从整个操作开始算，这一点仍然要注意。
- **以为 STW 时间和堆大小成正比。** 现在的 STW 通常在 1 毫秒以内，和堆大小基本无关。
- **以为 `runtime.GC()` 之后内存一定会还给操作系统。** 要立刻归还，用 `debug.FreeOSMemory()`。
- **为了避免逃逸，把代码改得很难读。** 先测量，确认是热点再优化。

---

## 自测

**1. 一个 key 过期以后，什么时候真正从内存里删掉？**

<details><summary>答案</summary>

两种时机，哪个先到算哪个：有命令读到或写到它时，当场删掉（惰性删除）；后台清理每 250 毫秒随机抽查，抽到它时删掉（定期删除）。

</details>

**2. 为什么既要 `expires` 这个 map，又要 `expiring` 这个切片？**

<details><summary>答案</summary>

map 用来按 key 查期限；切片用来随机抽样，随机取一个下标是 O(1)，map 没有这种操作。`expiration.index` 记下 key 在切片里的位置，删除时用末项补洞，也是 O(1)。

</details>

**3. `clearExpiration` 里为什么要写 `s.expiring[last] = ""`？**

<details><summary>答案</summary>

截短切片后，最后一格还留在底层数组里，仍然引用着那个 key 字符串，GC 回收不了它。先清空再截短，这个字符串才能被回收。

</details>

**4. 后台清理为什么每个分片最多抽 256 次？Redis 的做法有什么不同？**

<details><summary>答案</summary>

清理要持有分片的写锁，次数不设上限会长时间挡住这个分片上的所有命令。Redis 在抽到的过期比例高时会继续抽，并限制总时间；Mini-Redis 不追加，留给下一轮。

</details>

**5. 删了 10 万个 key，内存为什么可能不降？Mini-Redis 做了什么？还有什么没解决？**

<details><summary>答案</summary>

没有新的分配，GC 可能不运行；GC 回收了，运行时也会先留着内存复用。Mini-Redis 在累计删掉 32768 个 key 后、最多每 5 秒调用一次 `debug.FreeOSMemory`，强制 GC 并归还内存。没解决的是 map 本身：删了 key，map 的表也不会缩小。

</details>

**6. `planSet` 返回的 `apply` 闭包为什么在堆上？这算问题吗？**

<details><summary>答案</summary>

闭包在 `planSet` 返回以后才被调用，它捕获的分片、key、value 要活得比 `planSet` 更久，编译器只能把它们放到堆上。每条写命令多一次小分配，换来“先算计划、日志落盘后再执行”的清楚结构，开启 AOF 时和 fsync 比起来可以忽略。

</details>

---

## 开发过程回放

以下按 git 历史整理。

- **2026-10-01，`3d95ea3`，第一阶段完成。** 一次加入了 EXPIRE、TTL、惰性删除、后台抽样清理，以及“末项补洞、清空旧末项”的抽样切片。大批过期后锁外调用 `debug.FreeOSMemory` 也是这时加的。期限写进 AOF 时记的是绝对毫秒时间 `__EXPIREATMS`，重启不会让 key 多活。
- **2026-10-02，`fbbce11`，修复重放边界。** 复审发现惰性删除不写日志，会让重放结果和运行时不一致：一个列表过期后被惰性删掉，又用 LPUSH 新建，日志里却只记了 LPUSH，重放时旧元素和旧期限会混进来。修复办法是新增私有记录 `_LNEW`，表示“这是一个全新的列表”（第 06 章细讲）。
- **2026-11-10，PR #5，可读性改造。** `ttl.go` 改名为 `expire.go`，`lockRead` 从引擎文件移进来，和过期相关的代码都在一个文件里；注释改成讲为什么，比如“为什么要清空旧末项”“为什么在锁外归还内存”。

---

## 核对来源（Go 1.27.1）

| 说法 | 出处 |
|---|---|
| 并发标记清除，不分代、不压缩；四个阶段及各自是否 STW；新分配的对象直接标黑 | `runtime/mgc.go` 开头的注释 |
| 清除终止和开启写屏障在同一次 STW 里 | `runtime/mgc.go`：`gcStart` 中 `stopTheWorldWithSema(stwGCSweepTerm)` → `finishsweep_m` → `setGCPhase(_GCmark)` → `startTheWorldWithSema` |
| 内存上限下 GC 的 CPU 上限约 50% | `runtime/mgclimit.go` 开头的注释 |
| 混合写屏障：Yuasa 删除屏障 + Dijkstra 插入屏障 | `runtime/mbarrier.go`：“Go uses a hybrid barrier that combines a Yuasa-style deletion barrier … with Dijkstra insertion barrier” |
| Green Tea：按 span 攒批扫描，提高局部性；在 1.27.1 中默认开启 | `runtime/mgcmark_greenteagc.go` 开头的注释；`internal/buildcfg/exp.go`：`GreenTeaGC: true` |
| Green Tea 在 Go 1.25 作为实验加入，1.26 默认开启 | Go 1.25、Go 1.26 release notes（定稿前由 GPT 核对） |
| 目标堆大小公式 | `runtime/mgcpacer.go`：`gcPercentHeapGoal = c.heapMarked + (c.heapMarked+c.lastStackScan.Load()+c.globalsScan.Load())*uint64(gcPercent)/100` |
| 堆的下限 4 MB × GOGC / 100 | `runtime/mgcpacer.go` 中 `heapMinimum` 的注释 |
| 2 分钟强制 GC | `runtime/proc.go`：`forcegcperiod int64 = 2 * 60 * 1e9` |
| gctrace 各字段的含义 | `runtime/extern.go` 中 `gctrace` 的说明；示例输出为 2026-11-10 本机实测 |
| 逃逸分析输出 | 2026-11-10 本机 `go build -gcflags=-m` 实测 |
| 隐式变量超过 64 KB 放到堆上 | `cmd/compile/internal/ir/cfg.go`：`MaxImplicitStackVarSize = int64(64 * 1024)` |
| Linux 的 madvdontneed | `runtime/mem_linux.go`；Go 1.16 release notes |
| Ticker 会调整间隔或丢 tick | `time/tick.go`：“adjust the time interval or drop ticks to make up for slow receivers” |
| Go 1.23 起未 Stop 的计时器可被回收、channel 变为同步 | `time/sleep.go` 和 `time/tick.go` 中 NewTimer、NewTicker 的注释 |
| Go 1.27 删除 asynctimerchan | `doc/godebug.md`：“Go 1.27 removed the `asynctimerchan` setting” |
| select 在多个就绪 case 中均匀随机选择 | `doc/go_spec.html` 的 Select statements 一节：“chosen via a uniform pseudo-random selection” |
| map 删除和 clear 都不缩小 | `internal/runtime/maps/map.go`：`Clear` 及 “TODO: shrink directory?” |
| 10 万个 key 过期时的 gctrace、写路径的逃逸分析 | 2026-11-10 在 dbd2e9f 上实际运行 |
| `_LNEW` 在 fbbce11 引入，`__EXPIREATMS`、`FreeOSMemory`、清空旧末项在 3d95ea3 引入 | `git log -S` 查询 |
