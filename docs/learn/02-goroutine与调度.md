# 第 02 章：goroutine 与调度

本章在项目里对应**连接处理**：服务端每接受一个连接，就起一个 goroutine 专门服务它。技术细节都对照过 Go 1.27.1 的源码，出处见文末。

## 这一章能回答的面试题

1. 讲讲 Go 的 GMP 调度模型。（八股 1）
2. goroutine 和线程有什么区别？goroutine 为什么轻？（八股 1 追问 1）
3. GOMAXPROCS 是什么？设得越大越好吗？（八股 1 追问 2）
4. 一个 goroutine 写了死循环，会卡住其他 goroutine 吗？（八股 1 追问 3）
5. 一个 goroutine 阻塞在网络读上，会发生什么？会占着线程吗？（八股 2）
6. 读文件和读网络，对调度器来说有什么不同？（八股 2 追问 1）
7. 一个 goroutine 里 panic 了，会怎样？（八股 3）
8. （配角）为什么“一个连接一个线程”的服务器扛不住大量连接？Redis 为什么用事件循环？

---

## 跑起来看什么

开三个终端，各启动一个客户端，连到同一个服务端，分别执行 `SET a 1`、`SET b 2`、`GET a`。三个客户端互不等待，每条命令都立刻有回复。

服务端没有为每个客户端开一个线程，而是为每个连接起了一个 goroutine。这一章要回答的是：这些 goroutine 是怎样分到少量线程上运行的，一个连接什么都不发的时候，它的 goroutine 在干什么。本章末尾的动手实验会开 1000 个连接，数一数服务端到底用了几个线程。

---

## Go 基础

### 1. go 关键字

在函数调用前面加 `go`，这个调用就会在一个新的 goroutine 里执行，当前代码不等它，立刻往下走。

```go
go handle(conn)              // 调用普通函数
go func() { ... }()          // 调用匿名函数，注意最后的 ()
```

三条规则：
- **参数在当前 goroutine 里立刻求值。** `go handle(conn)` 执行的那一刻，`conn` 的值就已经确定，传给了新 goroutine。
- **返回值被丢弃。** 要拿到结果，用 channel，或者写进一个共享的变量（要加锁或者保证各写各的）。
- **main 函数返回，整个程序就结束了**，不会等其他 goroutine。所以 main 里起了 goroutine，必须用某种方式等它们结束。

### 2. 等 goroutine 结束：sync.WaitGroup

```go
package main

import (
	"fmt"
	"runtime"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	squares := make([]int, 5)
	for i := range 5 {
		wg.Go(func() { // Go 1.25 起：等价于 wg.Add(1) 加上 go func() { defer wg.Done(); ... }()
			squares[i] = i * i // 每个 goroutine 只写自己那一格，互不冲突
		})
	}
	wg.Wait() // 等计数归零
	fmt.Println(squares)
	fmt.Println("还剩", runtime.NumGoroutine(), "个 goroutine")
}

// 输出：
// [0 1 4 9 16]
// 还剩 1 个 goroutine
```

- `wg.Go(f)` 是 Go 1.25 加的。老代码里常见的写法是：

  ```go
  wg.Add(1)
  go func() {
  	defer wg.Done()
  	// ...
  }()
  ```

  `Add` 必须在 `go` 之前调用。如果放进新 goroutine 里调用，`Wait` 可能在 `Add` 之前就执行了，看到计数是 0，直接返回。
- 最后只剩 1 个 goroutine，就是 main 自己。
- 这里每个 goroutine 写切片里不同的元素，所以不需要锁。如果多个 goroutine 写同一个变量，就是**数据竞争**，要用锁（第 04 章），并用 `go test -race` 检查（第 08 章）。
- WaitGroup 的更多用法和坑，在第 07 章讲停机时细讲。

### 3. 查看调度相关的数字

```go
package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Println(runtime.NumCPU() > 0)                      // 逻辑 CPU 数
	fmt.Println(runtime.GOMAXPROCS(0) == runtime.NumCPU()) // 传 0 表示只查询、不修改
	fmt.Println(runtime.NumGoroutine())                    // 当前 goroutine 数
}

// 输出：
// true
// true
// 1
```

在普通的电脑上，`GOMAXPROCS` 默认等于逻辑 CPU 数。在 Linux 容器里，Go 1.25 起它还会参考容器的 CPU 配额，见八股 1 的要点。

### 4. goroutine 里的 panic

一个 goroutine 里发生 panic 而没有被 recover，**整个程序都会退出**，不只是这个 goroutine。而且 recover 只能接住**同一个 goroutine** 里的 panic，main 里的 recover 管不到别的 goroutine。详见八股 3。

---

## 代码走读：项目里的 goroutine 从哪里来

整个服务端的生产代码里，`go` 关键字只出现了四次：

| 位置 | 起了什么 | 有几个 | 什么时候结束 |
|---|---|---|---|
| [server/server.go](../../server/server.go) 的 `startConnection` | `go s.handle(conn)`：服务一个连接 | 每个连接一个 | 客户端断开、协议错误或停机 |
| [cmd/server/main.go](../../cmd/server/main.go) 的 `run` | 后台清理过期 key 的循环 `RunCleanup` | 一个 | ctx 被取消 |
| [cmd/server/main.go](../../cmd/server/main.go) 的 `run` | 等待停机信号，收到后调用 `Shutdown` | 一个 | 收到信号，或者 `Serve` 先返回 |
| [server/server.go](../../server/server.go) 的 `Shutdown` | 调用 `wg.Wait()`，等完就关闭一个 channel | 停机时一个 | 所有连接的 goroutine 都结束 |

再加上 main 自己（它在 `Serve` 里循环 `Accept`），以及 `signal.NotifyContext` 在标准库内部起的 goroutine。所以连接数是 N 时，服务端的 goroutine 数大约是 N 再加上固定的几个，随连接数线性增长，每个连接只多一个。

### 1. 接受连接：Serve 和 startConnection

```go
for {
	conn, err := listener.Accept()
	if err != nil {
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return fmt.Errorf("accept: %w", err)
	}
	if !s.startConnection(conn) {
		return nil
	}
}
```

```go
func (s *Server) startConnection(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping {
		_ = conn.Close()
		return false
	}
	s.conns[conn] = false
	s.wg.Add(1)
	go s.handle(conn)
	return true
}
```

- `Accept` 没有新连接时会阻塞。和 Read 一样，它等的是网络事件，main 这个 goroutine 挂在网络轮询器上，不占线程（八股 2）。
- `s.wg.Add(1)` 写在 `go` 之前，而且和“是否在停机”的检查放在同一把锁里。这样停机开始以后，不会再有新的 goroutine 被加进 WaitGroup（第 07 章）。
- `s.conns[conn] = false` 把连接登记为“空闲”，停机时要靠这张表找到所有连接。

### 2. 服务一个连接：handle

[server/connection.go](../../server/connection.go) 的 `handle` 是一个循环：读一条请求、执行、写回回复，再读下一条。

```go
reader := bufio.NewReader(conn)
for {
	args, _, readErr := resp.ReadRequest(reader)
	// ……出错或停机就 return
	result, execErr := s.executor.Exec(args)
	reply, err := resp.EncodeReply(result, execErr)
	// ……
	if err := writeReply(conn, reply); err != nil {
		return
	}
	// ……
}
```

这是一段完全同步的代码：没有回调，也没有“数据到了通知我”。客户端什么都不发，`ReadRequest` 里的 `conn.Read` 就一直不返回，这个 goroutine 停在那里。对线程来说，这并不是阻塞：运行时把它挂到网络轮询器上，线程转去运行别的 goroutine。

### 3. 后台清理：一个长期存在的 goroutine

```go
cleanupDone := make(chan struct{})
go func() {
	defer close(cleanupDone)
	dbEngine.RunCleanup(ctx)
}()
```

[database/expire.go](../../database/expire.go) 的 `RunCleanup` 每 250 毫秒被 `time.Ticker` 唤醒一次，抽查一批 key 有没有过期。两次唤醒之间，它在 `select` 里等计时器，不占线程。`run` 退出前会先取消 ctx，再等 `cleanupDone` 被关闭，确认这个 goroutine 真的结束了（第 07 章）。

---

## Go 八股

### 八股 1：讲讲 Go 的 GMP 调度模型

#### 一分钟口述版

> Go 调度器要解决的问题是：成千上万个 goroutine，怎样分到少量操作系统线程上去跑。它有三个角色。G 是 goroutine，保存自己的栈和执行位置；M 是操作系统线程，真正执行代码；P 是处理器，相当于“执行 Go 代码的许可证”，M 必须拿到一个 P 才能运行 Go 代码。P 的个数就是 GOMAXPROCS，默认等于 CPU 逻辑核数，所以同一时刻最多有这么多个 goroutine 在并行。
>
> 每个 P 有一个本地队列，最多放 256 个 G，另外还有一个全局队列。M 先从自己 P 的本地队列取 G；本地空了，就去全局队列、网络轮询器找，再找不到就从别的 P 那里偷一半过来，这叫工作窃取。
>
> 阻塞分两种。网络读写阻塞时，G 挂到网络轮询器上，M 和 P 接着跑别的 G。系统调用阻塞时，比如写文件 fsync，M 会跟着卡住，调度器就把 P 拿走交给另一个 M，其他 G 不受影响。最后，一个 G 连续运行超过 10 毫秒会被抢占，免得饿死别人。

#### 要点

**三个角色**

| 角色 | 是什么 | 有多少 |
|---|---|---|
| G | goroutine。保存自己的栈、当前执行到哪里、状态（可运行、运行中、等待中等） | 可以有几十万个。初始栈从 2 KB 起，不够用时自动扩容；Go 1.19 起，新 goroutine 的初始栈会参考历史上 goroutine 的平均栈用量 |
| M | 操作系统线程，真正在 CPU 上执行指令 | 按需创建，默认上限 10000。通常略多于 P 的个数，因为有的 M 卡在系统调用里，有的在空闲等待 |
| P | 调度用的“处理器”，持有一个本地运行队列 | 等于 GOMAXPROCS |

**为什么要有 P。** Go 1.1 之前只有 G 和 M，所有 M 从同一个全局队列取任务，每次都要抢一把全局锁，核数一多就互相等待。引入 P 以后，每个 P 有自己的本地队列，大部分调度都在本地完成，不需要全局锁。

**找下一个 G 的顺序**（源码里的 `findRunnable`）：
1. 本地队列。为了公平，每调度 61 次会先看一眼全局队列，防止全局队列里的 G 一直轮不到。
2. 全局队列。
3. 网络轮询器：哪些 goroutine 等的网络数据已经到了。
4. 从别的 P 的本地队列偷一半。

**新建的 goroutine 放在哪里。** `go f()` 创建的 G 放进当前 P 的 `runnext` 位置，也就是“下一个就运行它”；原来在 `runnext` 的 G 挪到本地队列尾部。本地队列满了（256 个），就把一半（128 个）连同新 G 一起挪到全局队列。

**两种阻塞，处理方式不同：**
- **网络 IO**：G 进入等待状态，登记到网络轮询器上，M 不会卡住，接着跑别的 G。网络轮询器在 Linux 上用 epoll，macOS 上用 kqueue，Windows 上用 IOCP。数据到了以后，G 重新变成可运行状态，放回某个 P 的队列。八股 2 会细讲这个过程。
- **系统调用**，例如读写文件、fsync：M 会真的卡在内核里。后台的监控线程 sysmon 定期检查：某个 P 卡在系统调用里超过一个检查周期（源码注释写的是至少 20 微秒），并且它的本地队列里还有 G 在等，或者已经没有空闲的 P 可以接手别的活，就把这个 P 拿走，交给另一个 M 继续运行。即使不满足这些条件，卡满 10 毫秒也会被拿走。系统调用返回后，原来的 M 先试着拿回原来的 P；拿不到就找空闲的 P；都没有，就把 G 放进全局队列，自己去休眠。

**抢占。** sysmon 发现一个 G 连续运行超过 10 毫秒，就要求它让出。Go 1.14 起是**异步抢占**：给线程发信号（Windows 上是挂起线程）打断它，几乎在任何位置都能停下来。所以即使是一个没有函数调用的死循环，也不会永远占着 P。Go 1.14 之前只能在函数调用处检查是否该让出，紧凑的死循环会一直霸占 P。

**GOMAXPROCS 的默认值。** 默认等于可用的逻辑 CPU 数。Go 1.25 起，在 Linux 上还会参考容器的 CPU 配额（cgroup），取两者中较小的那个，并且运行时会定期更新。所以在 Docker 或 Kubernetes 里限制了 2 个 CPU，GOMAXPROCS 也会是 2，不再是宿主机的核数。

> **项目里没有直接体现，但面试常问**：本地队列的 256、每 61 次看一眼全局队列、工作窃取、`runnext`、异步抢占。这些都是运行时内部的机制，Mini-Redis 的代码里看不到，但面试官追问 GMP 时经常问到。

#### 项目里的例子

**1. 一个连接一个 goroutine。** [server/server.go](../../server/server.go) 的 `startConnection` 每登记一个连接，就执行一次 `go s.handle(conn)`。如果换成“一个连接一个操作系统线程”，1000 个客户端就要 1000 个线程，每个线程的栈是 MB 级的。用 goroutine 时，空闲连接的 G 卡在 `conn.Read` 上，被挂到网络轮询器，不占用任何线程。下面的动手实验用真实数据说明了这一点。

**2. AOF 的 fsync 是会卡住线程的系统调用。** 开启 AOF 时，每条写命令都会走到 [aof/store.go](../../aof/store.go) 的 `Append`，最后调用 `s.file.Sync()`。这时执行它的 M 会卡在内核里，但调度器会把 P 交给别的 M，其他连接的 goroutine 照常运行。

这里有一个容易忽略的地方：[database/write.go](../../database/write.go) 的 `executeWrite` 在 fsync 期间一直**拿着** `logMu` 和相关分片的写锁。别的 goroutine 如果也要这几把锁，就只能等着，包括想读同一个分片的 GET。它们是在锁上排队，不占线程，但请求确实变慢了。也就是说：**调度器能保证线程不被浪费，但不会替你释放锁。**

**3. 后台清理 goroutine。** [cmd/server/main.go](../../cmd/server/main.go) 用 `go func() { defer close(cleanupDone); dbEngine.RunCleanup(ctx) }()` 启动过期清理，`RunCleanup` 每 250 毫秒被 `time.Ticker` 唤醒一次。两次唤醒之间，这个 G 在等计时器，不占线程。

**4. 基准测试里的 GOMAXPROCS=16。** README 的性能数据是在 GOMAXPROCS=16 下测的：16 个 P，最多 16 个 goroutine 真正并行。只有一把全局锁时，16 个 goroutine 排队抢同一把锁（386.9 ns/op）；分成 16 个分片后，它们大多能同时进行（58.1 ns/op）。热点场景下所有请求都落在同一个分片，又回到排队，所以分片带不来好处。

#### 常见追问

**追问 1：goroutine 和线程有什么区别？**
- **栈**：goroutine 的栈从 2 KB 起，按需增长和收缩；线程的栈在创建时就固定，通常是 MB 级（Linux 常见 8 MB，Windows 默认 1 MB）。
- **创建和切换**：goroutine 由 Go 运行时在用户态调度，切换时只保存少量寄存器；线程由操作系统调度，切换要进出内核。
- **调度方**：goroutine 由 Go 运行时调度，线程由操作系统调度。goroutine 最终仍然跑在线程上，GMP 就是把前者映射到后者的方式。

**追问 2：GOMAXPROCS 设得越大越好吗？**

不是。P 多于 CPU 核数时，多出来的线程只会互相抢 CPU，增加切换开销。IO 多的服务也不需要调大，因为等网络的 goroutine 本来就不占 P。真正要注意的是容器环境：Go 1.25 之前，GOMAXPROCS 默认按宿主机核数设置，限制了 2 个 CPU 的容器可能开出 64 个 P，常见做法是引入 `uber-go/automaxprocs`；Go 1.25 起运行时会自己读取 cgroup 配额。

**追问 3：一个 goroutine 写了死循环，会卡住其他 goroutine 吗？**

Go 1.14 起不会。sysmon 发现它连续运行超过 10 毫秒，就发信号异步抢占，把 P 让给别人。Go 1.14 之前会：抢占只发生在函数调用处，没有函数调用的死循环会一直占着这个 P；如果 GOMAXPROCS 是 1，整个程序就卡死了。

---

### 八股 2：一个 goroutine 阻塞在网络读上，会发生什么？

#### 一分钟口述版

> Go 的网络连接在底层都是非阻塞的 socket。goroutine 调用 `conn.Read` 时，运行时先直接尝试读；如果暂时没有数据，系统调用会立刻返回“稍后再试”，而不是把线程卡住。这时运行时把这个 goroutine 登记到网络轮询器上，标记成“等待网络 IO”，然后把它挂起，当前的 M 和 P 马上去运行别的 goroutine。网络轮询器底层是 epoll、kqueue 或 IOCP，调度器在找活干的时候会问它“哪些连接有数据了”，sysmon 也会在超过 10 毫秒没人问的时候去问一次。数据到了，对应的 goroutine 重新变成可运行状态，放回队列；轮到它时，再读一次就成功了。所以从代码上看，Read 是阻塞的同步调用；但对线程来说，它没有被阻塞。一个线程就能轮流服务大量连接。

#### 要点

一次 `conn.Read` 在没有数据时的路径（Linux 为例）：

1. `conn.Read` → `internal/poll` 的 `FD.Read` → 系统调用 `read`。socket 是非阻塞的，没有数据时返回 `EAGAIN`。
2. 收到 `EAGAIN`，调用 `pd.waitRead`，进入运行时的 `netpollblock`。
3. `netpollblock` 调用 `gopark`，把 G 挂起，等待原因记为“IO wait”。M 回到调度循环，去运行别的 G。
4. 数据到达，内核在 epoll 里标记这个 socket 可读。
5. 有三个地方会去问网络轮询器：
   - 某个 M 在 `findRunnable` 里找不到活干时；
   - 没有活干的 M 会阻塞在 `netpoll` 上，直到有连接就绪或者有计时器到期；
   - sysmon 发现超过 10 毫秒没人问过时。
6. 就绪的 G 被放回运行队列。轮到它时，从第 1 步重新 `read`，这次读到了数据。

**读超时也走这里。** `conn.SetReadDeadline(t)` 会在这个连接的轮询描述符上设一个运行时计时器。时间一到，挂起的 G 被唤醒，Read 返回 `os.ErrDeadlineExceeded`。所以给连接设超时也不需要额外的线程。

#### 项目里的例子

- **空闲连接不占线程。** 客户端连上以后什么都不发，服务端那个 goroutine 就一直停在第 3 步。动手实验里，1000 个空闲连接只用了十几个线程，原因就在这里。
- **停机时靠关闭连接打断 Read。** 停机要让空闲连接的 goroutine 退出，但它们正挂在 Read 上，等不到数据就永远不会醒。[server/server.go](../../server/server.go) 的 `Shutdown` 直接关闭这些连接：被关闭的连接上挂起的 Read 会立刻返回错误，goroutine 就能退出了。设置一个已经过去的读截止时间也能达到同样的效果。第 07 章细讲停机。

#### 常见追问

**追问 1：读文件为什么不走网络轮询器？**

Linux 的 epoll 不支持普通文件：对普通文件来说，“可读”总是成立的，非阻塞模式对它不起作用。所以读写文件是真正的阻塞系统调用，M 会卡在内核里，调度器靠 sysmon 把 P 拿走（八股 1 的“两种阻塞”）。Mini-Redis 的 AOF 写入和 fsync 走的就是这条路。

**追问 2：既然有网络轮询器，为什么不全部用它？比如事件循环那样？**

网络轮询器本来就在底层做着事件循环的工作，只是对写代码的人隐藏了。Go 的选择是：让程序员写同步代码，运行时把它转成事件驱动的效率。代价是每个连接要有一个 goroutine 的栈，但 goroutine 的栈只有几 KB，比线程小得多，这个代价很小。

---

### 八股 3：一个 goroutine 里 panic 了，会怎样？

#### 一分钟口述版

> 如果这个 panic 没有被 recover，**整个程序**都会打印堆栈然后退出，退出码是 2，不只是出错的那个 goroutine。recover 只能在 defer 的函数里调用，而且只能接住**同一个 goroutine** 里的 panic，所以 main 里写的 recover 救不了别的 goroutine。网络服务一般会在每个请求的 goroutine 里加一个 defer recover，比如 net/http 就会在处理每个连接的地方 recover，避免一个请求的 bug 让整个服务挂掉。但 recover 不是越多越好：如果 panic 发生在修改共享数据的中途，recover 之后数据可能已经处于不一致的状态，继续服务反而更危险。

#### 要点

- panic 会沿着当前 goroutine 的调用栈往上走，依次执行每一层的 defer。某个 defer 里调用了 `recover()`，panic 就停在那里，函数正常返回。
- 走到 goroutine 的最顶层还没人 recover，运行时打印 panic 信息和所有 goroutine 的堆栈，以退出码 2 结束进程。
- 有一类错误连 recover 也接不住，叫 **fatal error**，例如 map 的并发写（第 03 章）、所有 goroutine 都在等待造成的死锁（第 04 章）。运行时认为这时内部状态已经不可信，直接终止进程。
- panic 和 recover 的更多细节，第 06 章讲错误处理时细讲。

#### 项目里的例子

- **Mini-Redis 的生产代码里没有 recover**，[server/connection.go](../../server/connection.go) 的连接循环里也没有。一条命令执行到一半 panic，说明程序有 bug，而且内存里的数据可能只改了一半。这时与其 recover 之后带着可能损坏的数据继续服务，不如让进程退出，重启后从 AOF 重放出一个一致的状态。这是一个有意的取舍。
- 对比 net/http：HTTP 处理函数一般是无状态的，一个请求出错不影响别的请求，所以每个连接 recover 是合理的。两者的区别在于：panic 时有没有可能留下改了一半的共享状态。

#### 常见追问

**追问：子 goroutine 的 panic，父 goroutine 能捕获吗？**

不能。Go 的 goroutine 之间没有父子关系，每个 goroutine 的 panic 只能在它自己里面 recover。如果需要把错误交给启动它的那一方，要在子 goroutine 里 recover，再通过 channel 或者 `errgroup` 把错误传回去。

---

### 配角：为什么“一个连接一个线程”的服务器扛不住？

#### 一分钟口述版

> 一个连接一个线程，写起来最简单，但连接一多就扛不住。每个线程有 MB 级的栈，一万个连接就要占掉几 GB 的虚拟内存；线程切换由内核完成，线程越多，切换越频繁，CPU 都花在切换上了。这就是常说的 C10K 问题。业界的解法是事件循环：用 epoll 这类机制，一个线程监听大量连接，哪个连接有数据就处理哪个，Redis 和 Nginx 都是这样做的。缺点是代码要拆成回调或者状态机，而且任何一个处理慢了，整个循环都被拖住。Go 的做法是两者结合：代码写成“一个连接一个 goroutine”的同步形式，运行时在底层用 epoll 做事件循环，所以既好写，又能撑住大量连接。

#### 要点

| 模型 | 好处 | 代价 | 例子 |
|---|---|---|---|
| 一个连接一个线程 | 代码直观 | 线程栈大，切换开销大，连接数上不去 | 早期的 Apache |
| 事件循环 | 一个线程服务大量连接 | 代码拆成回调或状态机；一个慢操作阻塞所有连接 | Redis、Nginx |
| 一个连接一个 goroutine | 代码直观，底层是事件驱动 | 每个连接一个 goroutine 的栈（KB 级）；调度由运行时负责 | Go 写的网络服务，包括 Mini-Redis |

Redis 的命令执行是单线程的事件循环（6.0 起可以用多个线程做网络读写，但命令仍然在一个线程上执行）。它不用加锁，代价是一条慢命令会卡住所有客户端。Mini-Redis 是多个 goroutine 并发执行命令，所以需要第 04 章讲的分片锁。

---

## 关键设计与为什么

**1. 一个连接一个 goroutine，代码按同步的顺序写。**
- 为什么：`handle` 读起来就是“读一条、执行、写回”，和人脑里的流程一样，不需要回调或状态机。网络等待由运行时转成事件驱动，效率不差。
- 放弃了什么：Redis 那样的单线程事件循环。单线程执行命令不需要锁，但 Mini-Redis 的命令会在多个 goroutine 里同时执行，所以要有第 04 章的分片锁。

**2. 每个连接只有一个 goroutine。**
- 读请求和执行请求本来就是一前一后的，同一个 goroutine 顺序做完就行。早期的版本多起了一个解析 goroutine，用 channel 把请求递过来，结果停机时要多处理“取消解析、排空 channel、等它退出”这一整套收尾。第 07 章讲为什么去掉它。

**3. 连接循环里不 recover。**
- 一条命令 panic 说明有 bug，内存可能只改了一半。让进程退出，重启后从 AOF 恢复，比带着可能损坏的数据继续服务更安全。

**4. 没有限制连接数。**
- goroutine 很便宜，1000 个连接只多用了几个线程。但连接数没有上限，每个连接仍要占一个 goroutine 的栈、一个 bufio 缓冲区和一个文件描述符，连接无限多时内存和描述符终究会耗尽。生产环境的服务通常会设一个最大连接数（真实的 Redis 有 `maxclients`），Mini-Redis 作为学习项目没有做。

---

## 动手实验：1000 个连接用了多少线程

**目的**：亲眼看到“一个连接一个 goroutine”不等于“一个连接一个线程”。

**第 1 步：启动服务，打开调度器跟踪。** `GODEBUG=schedtrace=1000` 让运行时每秒打印一行调度器状态。先编译出可执行文件再运行；如果直接 `go run`，go 命令自己也会打印跟踪信息，和服务的输出混在一起。

PowerShell，在 mini-redis 根目录：

```powershell
go build -o mr.exe ./cmd/server
$env:GODEBUG = "schedtrace=1000"
.\mr.exe -addr 127.0.0.1:16379 -aof=
```

- `-aof=` 表示关闭持久化，实验不需要写文件。PowerShell 5.1 会吞掉空字符串参数，所以不要写成 `-aof ""`。
- 实验结束后，在这个终端按 Ctrl+C 停止服务，再执行 `Remove-Item Env:GODEBUG`，否则之后在这个终端运行的 Go 程序都会打印跟踪信息。

**第 2 步：打开 1000 个空闲连接。** 在另一个终端，把下面的代码保存到一个临时目录的 `main.go`，然后运行 `go run main.go`：

```go
// 打开 1000 个空闲连接并保持 6 秒，用来观察服务端线程数会不会随连接数增长。
package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	var conns []net.Conn
	for i := 0; i < 1000; i++ {
		conn, err := net.Dial("tcp", "127.0.0.1:16379")
		if err != nil {
			fmt.Println("连接失败：", err)
			return
		}
		conns = append(conns, conn)
	}
	fmt.Println("已建立", len(conns), "个连接，保持 6 秒")
	time.Sleep(6 * time.Second)
	for _, conn := range conns {
		conn.Close()
	}
}
```

**你应该看到**（2026-11-10 实测：Windows，AMD Ryzen 7 9700X，16 个逻辑 CPU，Go 1.27.1。节选，每行末尾的 `schedticks` 省略）：

```text
SCHED 3011ms: gomaxprocs=16 idleprocs=16 threads=7 spinningthreads=0 needspinning=0 idlethreads=3 runqueue=0 [ 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 ]
SCHED 6027ms: gomaxprocs=16 idleprocs=16 threads=14 spinningthreads=0 needspinning=0 idlethreads=10 runqueue=0 [ 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 ]
SCHED 11070ms: gomaxprocs=16 idleprocs=16 threads=17 spinningthreads=0 needspinning=0 idlethreads=13 runqueue=0 [ 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 ]
```

这三行分别是：
- **3011ms**：服务已经启动，还没有连接。
- **6027ms**：1000 个连接都建立好了，正保持着。
- **11070ms**：连接全部关闭以后。

你的机器上时间和线程数会略有不同，但规律是一样的。

各字段的意思：

| 字段 | 意思 |
|---|---|
| `gomaxprocs=16` | P 的个数 |
| `idleprocs` | 空闲的 P。1000 个连接都在等数据，没有 G 要运行，所以 16 个 P 全空闲 |
| `threads` | 已经创建的 M（线程）总数 |
| `idlethreads` | 其中空闲的 M |
| `runqueue=0 [ 0 0 … ]` | 全局队列里的 G 数；方括号里依次是每个 P 本地队列的 G 数 |

**怎么解读：**
- 1000 个连接保持着的时候，服务端有 1000 多个 goroutine，线程却只有 14 个，其中 10 个还是空闲的。
- 线程数在连接建立和关闭的那一刻各涨了几个：一下子有很多 G 要运行，调度器唤醒更多的 P 和 M 来处理。之后线程数不再增长，因为同一时刻最多只有 16 个 M 在运行 Go 代码（受 P 的个数限制）。
- 空闲的线程不会被销毁，留着下次复用，所以最后停在 17 个。
- 如果是一个连接一个线程，这里至少会有 1000 个线程。
- 和改造前对比：改造前每个连接有两个 goroutine，一个处理连接，一个专门解析请求（`ParseStream`）。同样的实验，当时是 7、15、20 个线程。goroutine 数少了一半，线程数却差不多，这正说明线程数取决于“同时有多少 G 要运行”，而不是 goroutine 的总数。

**想一想：** 如果把第 2 步改成每个连接不停地发 `SET`，同时用 `-aof=data/test.aof` 打开持久化，`threads` 会怎样变化？提示：fsync 是会卡住线程的系统调用。

---

## 常见误区

- **以为 main 会等 goroutine 跑完。** main 一返回，程序就结束了。
- **把 `wg.Add(1)` 写在新 goroutine 里面。** `Wait` 可能先执行，看到计数是 0 就直接返回了。
- **以为 recover 能接住所有 panic。** 它只管同一个 goroutine，也接不住 fatal error。
- **以为阻塞的 Read 会占着一个线程。** 网络 Read 不会；文件读写和 fsync 会。
- **以为 GOMAXPROCS 越大并发越高。** 并发多少由 goroutine 数决定；GOMAXPROCS 决定的是同时有几个在并行，超过 CPU 核数只会增加切换开销。
- **以为 goroutine 多了一定要做 goroutine 池。** goroutine 很便宜，大多数情况不需要池。真正要限制的是它们共同使用的资源，比如数据库连接、内存、下游服务的并发数。

---

## 自测

**1. Mini-Redis 的服务端在哪几个地方起了 goroutine？每个 goroutine 什么时候结束？**

<details><summary>答案</summary>

`startConnection` 为每个连接起一个，客户端断开、协议错误或停机时结束；`run` 里起一个后台清理，ctx 取消时结束；`run` 里再起一个等待停机信号，收到信号或 `Serve` 先返回时结束；`Shutdown` 里起一个等 WaitGroup，所有连接结束时结束。

</details>

**2. 1000 个客户端连上来什么都不发，服务端的 1000 个 goroutine 在干什么？占了多少线程？**

<details><summary>答案</summary>

都停在 `conn.Read` 里。socket 是非阻塞的，没有数据时运行时把 G 挂到网络轮询器上，M 和 P 去做别的事。实验里 1000 个空闲连接只用了 14 个线程，其中 10 个还是空闲的。

</details>

**3. 开启 AOF 后，一条 SET 在 fsync 时，执行它的线程会怎样？别的连接会不会受影响？**

<details><summary>答案</summary>

fsync 是阻塞的系统调用，这个 M 会卡在内核里，sysmon 会把它的 P 交给别的 M，其他 goroutine 照常运行。但 `executeWrite` 在 fsync 期间拿着 `logMu` 和分片写锁，要用这些锁的命令（其他写命令、同分片的读）会在锁上排队。线程没有浪费，请求却变慢了。

</details>

**4. `startConnection` 为什么把 `s.wg.Add(1)` 和 `stopping` 的检查放在同一把锁里？**

<details><summary>答案</summary>

停机时先在锁里把 `stopping` 设为 true，之后才 `wg.Wait()`。`Add` 只会在 `stopping` 为 false 时、在同一把锁里执行，所以停机开始以后不会再有新的 `Add`，`Wait` 不会漏等刚起的 goroutine。

</details>

**5. 为什么连接循环里不写 recover？什么样的服务适合在每个请求里 recover？**

<details><summary>答案</summary>

Mini-Redis 的命令会修改共享的内存数据，panic 时数据可能只改了一半，recover 后继续服务反而危险，不如退出后从 AOF 恢复。请求之间没有共享状态、出错不会留下改了一半的数据的服务，例如大多数 HTTP 处理函数，适合每个请求 recover。

</details>

**6. 去掉“每个连接一个解析 goroutine”以后，goroutine 数少了一半，线程数为什么几乎没变？**

<details><summary>答案</summary>

线程数取决于同一时刻有多少 G 需要运行，以及有多少 M 卡在系统调用里，而不是 goroutine 的总数。空闲连接的 goroutine 都挂在网络轮询器上，不需要线程，多一倍也不占线程。

</details>

---

## 开发过程回放

以下按 git 历史整理。

- **2026-03-17，`6fe6b67`，第一次提交。** `tcp/server.go` 就已经是“每接受一个连接，起一个 goroutine 处理”的结构。
- **2026-05-20，`2343d6e`，加入解析 goroutine。** `ParseStream` 为每个连接再起一个 goroutine 专门解析，通过 channel 把结果交给处理函数。从此每个连接有两个 goroutine。
- **2026-10-01，`3d95ea3`，有序停机。** 加入 WaitGroup、停机标志和截止时间，停机时要同时收拾两个 goroutine 和它们之间的 channel。resp 包的处理函数和 tcp 包的 Server 各有一套停机逻辑。
- **2026-11-10，PR #5，可读性改造。** 去掉解析 goroutine，每个连接只剩一个 goroutine，同步地读、执行、写回；tcp 包改名为 server，两套停机逻辑合成一套。重跑本章的实验：1000 个空闲连接时，线程数从 15 个变成 14 个，基本不变。

---

## 核对来源（Go 1.27.1）

| 说法 | 出处 |
|---|---|
| G、M、P 的定义；P 是执行 Go 代码必须持有的资源 | `runtime/proc.go` 开头的注释 |
| 本地队列 256 | `runtime/runtime2.go`：`runq [256]guintptr` |
| 每 61 次调度看一眼全局队列 | `runtime/proc.go`：`pp.schedtick%61 == 0` |
| 本地队列满时把一半挪到全局队列 | `runtime/proc.go`：`runqputslow` |
| 新 G 放进 `runnext` | `runtime/proc.go`：`newproc` 调用 `runqput(pp, newg, true)` |
| 系统调用中超过一个 sysmon 周期（至少 20 微秒）就收回 P；时间片 10 毫秒 | `runtime/proc.go`：`retake`、`forcePreemptNS = 10ms` |
| 初始栈 2 KB；按平均用量调整默认开启 | `runtime/stack.go`：`stackMin = 2048`、`startingStackSize`；`runtime/runtime1.go`：`adaptivestackstart = 1` |
| 线程数上限 10000 | `runtime/proc.go`：`sched.maxmcount = 10000` |
| GOMAXPROCS 在 Linux 上参考 cgroup 配额并定期更新（Go 1.25 起） | `runtime/cgroup_linux.go`：`defaultGOMAXPROCS`；`doc/godebug.md`：`containermaxprocs`、`updatemaxprocs` |
| 各平台的网络轮询实现 | `runtime/netpoll_epoll.go`、`netpoll_kqueue.go`、`netpoll_windows.go` |
| 网络读遇到 EAGAIN 后等待轮询器 | `internal/poll/fd_unix.go`：`err == syscall.EAGAIN && fd.pd.pollable()`；`internal/poll/fd_poll_runtime.go`：`waitRead` |
| 挂起 G，等待原因是 IO wait | `runtime/netpoll.go`：`netpollblock` 调用 `gopark(..., waitReasonIOWait, ...)` |
| 调度器空闲时、sysmon 超过 10 毫秒时轮询网络 | `runtime/proc.go`：`findRunnable` 中的 `netpoll`；sysmon 中的 “poll network if not polled for more than 10ms” |
| 读写截止时间是轮询描述符上的计时器 | `runtime/netpoll.go`：`poll_runtime_pollSetDeadline` |
| 未 recover 的 panic 以退出码 2 结束进程 | `runtime/panic.go`：`exit(2)` |
| net/http 在每个连接里 recover | `net/http/server.go`：`conn.serve` 中的 `recover()` |
| `WaitGroup.Go` | `sync/waitgroup.go`：`func (wg *WaitGroup) Go`（Go 1.25 加入） |
| Redis 的 `maxclients` | Redis 配置文档（定稿前由 GPT 核对） |
| 1000 个空闲连接的线程数 7 → 14 → 17 | 2026-11-10 在 dbd2e9f 上实测；改造前（31684b7）同一实验为 7 → 15 → 20 |
| 项目里的 goroutine 只在四处启动 | `git grep -n "go func\|go s\." -- '*.go' ':!*_test.go'` |
