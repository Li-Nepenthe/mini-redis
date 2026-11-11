# 第 07 章：channel、context 与 WaitGroup

本章在项目里对应**优雅停机**：收到 Ctrl+C 以后，不再接受新连接，让正在执行的命令把回复写完，关掉空闲连接，等所有 goroutine 退出，最后关闭 AOF 文件。代码在 [server/server.go](../../server/server.go)、[server/connection.go](../../server/connection.go) 和 [cmd/server/main.go](../../cmd/server/main.go)。

## 这一章能回答的面试题

1. channel 的底层是怎样实现的？（八股 1）
2. 有缓冲和无缓冲的 channel 有什么区别？
3. 对一个已关闭的 channel 读、写、再关闭，分别会怎样？nil channel 呢？（八股 2）
4. goroutine 泄漏是怎样产生的？怎样发现和排查？（八股 3）
5. context 是做什么的？取消是怎样一层层传下去的？（八股 4）
6. WaitGroup 怎么用？有哪些坑？sync.Once 是怎样实现的？
7. 怎样实现优雅停机？（八股 5）
8. 什么时候不该用 channel？（配角）

---

## 跑起来看什么

运行 server 包里和停机有关的测试，加 `-v` 看测试自己打印的日志：

```text
go test ./server -run 'Shutdown|Close|Hundred' -v -count=1
```

输出里有这样几行（2026-11-10 实测，节选）：

```text
--- PASS: TestCloseInterruptsReadAndRejectsLateConnection (0.00s)
    server_test.go:186: 100 abnormal disconnects: goroutines baseline=3 after=3
--- PASS: TestHundredAbnormalDisconnectsReturnToBaseline (0.02s)
--- PASS: TestConcurrentCloseInterruptsIdleConnections (0.00s)
--- PASS: TestShutdownDrainsSingleInFlightReply (0.00s)
    shutdown_test.go:127: listener stopped, one reply drained, queued command rejected in 528.5µs
--- PASS: TestShutdownDrainsInFlightReplyAndRejectsPipeline (0.00s)
--- PASS: TestShutdownTimeoutDoesNotHideBlockedExecutor (0.04s)
```

这几个测试名就是本章要讲的停机规则：

- **CloseInterruptsRead**：连接空闲、挂在 Read 上时，停机要能把它打断。
- **HundredAbnormalDisconnects**：100 个客户端发了半条请求就断开。测试检查两件事：登记的连接数回到 0，goroutine 数不超过开始时的基线。这次运行基线是 3 个，结束时也是 3 个，一个都没有泄漏。
- **DrainsInFlightReply**：停机时正在执行的命令，要把回复写完。
- **RejectsPipeline**：客户端一次发了两条命令，第一条正在执行时开始停机，第二条就不再执行了。
- **TimeoutDoesNotHideBlockedExecutor**：等不及了要超时返回，但要如实说“超时了”，不能假装一切都停干净了。

---

## Go 基础

### 1. channel 的基本操作

```go
package main

import "fmt"

func main() {
	// 无缓冲：发送方会一直等到有人接收，两边在这一刻“握手”
	done := make(chan struct{})
	go func() {
		fmt.Println("worker 完成")
		close(done) // 关闭 = 广播“结束了”，所有等待的接收方都会被唤醒
	}()
	<-done

	// 有缓冲：缓冲区没满，发送方就不用等
	jobs := make(chan int, 3)
	jobs <- 1
	jobs <- 2
	jobs <- 3
	close(jobs)           // 发送方负责关闭，告诉接收方“没有更多了”
	for j := range jobs { // range 会一直读，直到 channel 被关闭并且读空
		fmt.Print(j, " ")
	}
	fmt.Println()

	v, ok := <-jobs // 已关闭并且读空：得到零值，ok 为 false
	fmt.Println(v, ok)
}

// 输出：
// worker 完成
// 1 2 3
// 0 false
```

**单向 channel**：函数参数写成 `chan<- int`（只能发）或 `<-chan int`（只能收），编译器会阻止误用。`ctx.Done()` 返回的就是 `<-chan struct{}`，你只能从里面收，不能关闭它。

### 2. context

context 用来在一组 goroutine 之间传递**取消信号**和**截止时间**（也能带少量请求范围的值）。

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

func main() {
	parent, cancel := context.WithCancel(context.Background())
	child, cancelChild := context.WithTimeout(parent, time.Hour)
	defer cancelChild() // 规则：拿到 cancel 函数就一定要调用，否则子 context 的资源要等父 context 结束才释放

	cancel()       // 取消父 context
	<-child.Done() // 子 context 也立刻被取消了
	fmt.Println(child.Err())
	fmt.Println(errors.Is(child.Err(), context.Canceled))

	ctx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	select {
	case <-time.After(time.Second):
		fmt.Println("干完了")
	case <-ctx.Done():
		fmt.Println("超时：", ctx.Err())
	}

	why, cancelWhy := context.WithCancelCause(context.Background())
	cancelWhy(errors.New("收到停机信号")) // Go 1.20 起可以附带取消原因
	fmt.Println(why.Err(), "|", context.Cause(why))
}

// 输出：
// context canceled
// true
// 超时： context deadline exceeded
// context canceled | 收到停机信号
```

使用约定：
- 作为函数的**第一个参数**传递，名字叫 `ctx`；不要存进结构体的字段里。
- 不确定用哪个时，从 `context.Background()` 开始。
- `WithValue` 只放请求范围的数据（比如请求 ID），不要用它传函数的参数。

### 3. WaitGroup 和 Once

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1) // 必须在 go 之前调用
		go func() {
			defer wg.Done()
			_ = i
		}()
	}
	wg.Wait()
	fmt.Println("3 个都结束了")

	var once sync.Once
	for range 3 {
		once.Do(func() { fmt.Println("只执行一次") })
	}

	load := sync.OnceValue(func() int { // Go 1.21 起：只算一次，之后直接返回结果
		fmt.Println("加载配置")
		return 42
	})
	fmt.Println(load(), load())
}

// 输出：
// 3 个都结束了
// 只执行一次
// 加载配置
// 42 42
```

第 02 章用过 Go 1.25 新增的 `wg.Go(f)`，它把 `Add(1)`、`go`、`defer Done()` 合成了一步，不会再把 Add 写错位置。

### 4. 接收系统信号

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
<-ctx.Done() // 按下 Ctrl+C，或者收到 SIGTERM，ctx 就被取消
```

`signal.NotifyContext`（Go 1.16 起）把“收到信号”变成了“context 被取消”，后面的停机流程就可以统一用 context 来写。

---

## 代码走读：一次 Ctrl+C 之后发生了什么

### 1. Server 里和停机有关的字段

[server/server.go](../../server/server.go)：

```go
type Server struct {
	addr     string
	executor Executor
	mu       sync.Mutex
	listener net.Listener
	conns    map[net.Conn]bool // true 表示正在执行命令或写回复，停机等它写完；false 表示空闲，停机直接关闭。
	stopping bool
	wg       sync.WaitGroup
	shutdown sync.Once
	done     chan struct{}
	stopErr  error
	closeErr error
}
```

| 字段 | 用途 |
|---|---|
| `mu` | 保护下面的 `listener`、`conns`、`stopping` |
| `conns` | 所有活着的连接，以及它们是“正在执行”还是“空闲” |
| `stopping` | 停机开始的标志。新连接、新命令进来前都要先看它 |
| `wg` | 每个连接的 goroutine 计一个数，停机时等它归零 |
| `shutdown` | `sync.Once`，保证停机流程只执行一次，`Shutdown` 被调用几次都一样 |
| `done` | 停机流程做完时关闭，`Serve` 等它 |
| `stopErr`、`closeErr` | 停机的结果。前者包括超时，后者只有关闭监听的错误 |

### 2. 连接循环：两道停机检查

[server/connection.go](../../server/connection.go) 的 `handle`：

```go
for {
	args, _, readErr := resp.ReadRequest(reader)
	if readErr == io.EOF {
		return
	}
	if readErr == nil && len(args) == 0 {
		continue
	}
	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		return
	}
	s.conns[conn] = true
	s.mu.Unlock()
	if readErr != nil {
		_ = writeReply(conn, []byte("-ERR Protocol error\r\n"))
		return
	}
	result, execErr := s.executor.Exec(args)
	reply, err := resp.EncodeReply(result, execErr)
	if err != nil {
		log.Printf("encode reply: %v", err)
		return
	}
	if err := writeReply(conn, reply); err != nil {
		return
	}
	s.mu.Lock()
	s.conns[conn] = false
	stopping := s.stopping
	s.mu.Unlock()
	if stopping {
		return
	}
}
```

两道检查各防一种情况：

- **执行前**：请求已经读进来了，但停机已经开始，就不执行，直接退出。这一道和 `Shutdown` 用同一把锁，所以“开始执行”和“开始停机”之间有明确的先后：要么在停机之前已经标记为正在执行，`Shutdown` 会等它；要么在停机之后才来，被拦下。
- **写完回复后**：`Shutdown` 不会关闭正在执行的连接，它在等这个连接自己结束。所以写完回复以后要再看一眼，在停机就主动退出，不再回去读下一条。少了这一道，连接会回到 Read 上等待，`Shutdown` 要一直等到超时（动手实验 1）。

函数开头的 defer 负责收尾，从哪里返回都会执行：

```go
defer func() {
	_ = conn.Close()
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
	s.wg.Done()
}()
```

### 3. Shutdown：Once、快照、等待和超时

```go
func (s *Server) Shutdown(ctx context.Context) error {
	s.shutdown.Do(func() {
		listener, conns := s.stopConnections()
		s.closeErr = closeListener(listener)
		s.stopErr = s.closeErr
		for conn, active := range conns {
			if !active {
				_ = conn.Close()
			}
		}
		finished := make(chan struct{})
		go func() { s.wg.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-ctx.Done():
			for conn := range conns {
				_ = conn.Close()
			}
			s.stopErr = errors.Join(s.stopErr, ctx.Err())
		}
		close(s.done)
	})
	return s.stopErr
}
```

- `stopConnections` 在锁里把 `stopping` 设成 true，再拷贝一份 `conns` 的快照。之后的网络操作（关监听、关连接）都在锁外做，不拖住正要退出的连接循环。
- 关闭空闲的连接：挂在 Read 上的 goroutine 立刻收到错误，走到执行前的那道检查，看到 `stopping` 就退出。
- `wg.Wait()` 本身不能和 ctx 一起放进 select，所以起一个小 goroutine 去等，等完就关闭 `finished`。这是“把一个阻塞调用变成可以 select 的 channel”的常见写法。
- ctx 先到期：强制关闭所有连接，把 `ctx.Err()` 并进返回值。这时可能还有执行器卡在 Exec 里（比如 fsync 卡住了），`Shutdown` 不等它，但也不假装它已经结束了：返回的错误里有 `context deadline exceeded`。
- 整个流程包在 `sync.Once` 里，第二次调用 `Shutdown` 直接返回第一次的结果。

`Close()` 是强制版本：先关掉所有连接（包括正在执行的），再调用 `Shutdown(context.Background())`，最后 `s.wg.Wait()`。哪怕之前的 `Shutdown` 已经超时返回了，`Close` 也会一直等到所有连接的 goroutine 真正退出。

### 4. cmd/server：按依赖的反方向收尾

[cmd/server/main.go](../../cmd/server/main.go) 的 `run`，省略了监听端口和创建引擎的部分（`……` 处省略了代码）：

```go
if aofPath != "" {
	// ……创建目录
	store, err := aof.Open(ctx, aofPath, dbEngine.Replay)
	// ……出错就返回，恢复了半条尾部就记一条日志
	dbEngine.AttachLog(store)
	defer func() { runErr = errors.Join(runErr, store.Close()) }()
}

cleanupDone := make(chan struct{})
go func() {
	defer close(cleanupDone)
	dbEngine.RunCleanup(ctx)
}()
defer func() {
	cancel()
	<-cleanupDone
}()

network := server.NewServer(addr, dbEngine)
watchStop, watchDone := make(chan struct{}), make(chan struct{})
go func() {
	defer close(watchDone)
	select {
	case <-ctx.Done():
		shutdownCtx, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = network.Shutdown(shutdownCtx)
	case <-watchStop:
	}
}()
// ……打印“开始监听”
err = network.Serve(listener)
close(watchStop)
<-watchDone
return err
```

按 Ctrl+C 以后，有几件事是同时开始的：

1. ctx 被取消。后台清理和等待信号的 goroutine 等的是同一个 ctx（`run` 里用 `context.WithCancel` 从信号 ctx 派生出来的），所以两边同时收到。后台清理在 select 里看到 `ctx.Done()`，马上退出；等待信号的 goroutine 开始调用 `Shutdown`。
2. `Shutdown` 第一件事就是关闭监听。`Serve` 里阻塞的 `Accept` 立刻返回 `net.ErrClosed`，`Serve` 进入收尾，等 `Shutdown` 做完（`<-s.done`）。
3. `Shutdown` 关闭空闲连接，等执行中的连接写完回复。这一步最多等 2 秒，超时就强制关闭所有连接。
4. `Shutdown` 做完，`Serve` 返回。`close(watchStop)` 再 `<-watchDone`，确认等待信号的 goroutine 也结束了。
5. `run` 返回，defer 按后进先出执行：再调用一次 `cancel()`（已经取消过，没有影响），等 `cleanupDone`，确认后台清理确实退出了；然后关闭 AOF：刷盘、关闭文件、释放文件锁；最后的 `listener.Close()` 是第二次关闭，监听在第 2 步就已经关了。

两点要注意：

- **2 秒只管第 3 步**，也就是等连接排空。之后关闭 AOF 要拿 `Store.mu`、要刷盘，这些没有时间限制。如果有一条写命令卡在 fsync 里，关闭 AOF 也会等它。
- **defer 的顺序保证的是“确认结束”的顺序**，不是“开始停止”的顺序：后台清理在第 1 步就开始退出了，但 `run` 要等到第 5 步，确认它真的结束了，才去关闭它依赖的 AOF。

为什么 `Shutdown` 用的是一个新的 `context.Background()` 派生的 ctx，而不是已经被取消的那个？因为那个 ctx 已经取消了，拿它去 `Shutdown`，会立刻走到超时的分支，一点排空的时间都没有。

---

## Go 八股

### 八股 1：channel 的底层是怎样实现的？

#### 一分钟口述版

> channel 在运行时是一个 hchan 结构体，由一把锁保护。它有一个环形缓冲区，记录了容量、当前元素个数、发送和接收的下标；还有两个等待队列，分别挂着因为发不出去而阻塞的发送者，和因为没有数据而阻塞的接收者；另外有一个关闭标记。发送时先加锁：如果有接收者在等，就把数据**直接拷贝到那个接收者的栈上**，然后唤醒它，连缓冲区都不经过；没有接收者但缓冲区有空位，就放进缓冲区；都不行，就把自己挂到发送队列上，让出 CPU。接收的过程是对称的。关闭时，把所有等待的接收者都唤醒，它们拿到零值；所有等待的发送者也被唤醒，然后 panic。无缓冲的 channel 就是容量为 0 的情况，发送和接收必须同时到场，所以它也是一种同步手段。

#### 要点

**结构**（`runtime/chan.go`，省略了几个和调试、计时器相关的字段）：

```text
type hchan struct {
	qcount   uint           // 缓冲区里现有的元素个数
	dataqsiz uint           // 缓冲区容量
	buf      unsafe.Pointer // 环形缓冲区
	elemsize uint16
	closed   uint32         // 关闭标记
	elemtype *_type
	sendx    uint           // 下一次发送写到哪一格
	recvx    uint           // 下一次接收从哪一格读
	recvq    waitq          // 等着接收的 goroutine
	sendq    waitq          // 等着发送的 goroutine
	lock     mutex
}
```

**发送 `ch <- v` 的三种情况：**
1. 接收队列里有人在等：直接把 v 拷贝给它，唤醒它。
2. 缓冲区没满：拷贝进 `buf[sendx]`，`sendx` 往前走一格。
3. 都不行：把当前 goroutine 包装成一个等待节点，挂到 `sendq`，调用 `gopark` 让出 CPU，等接收者来唤醒。

**接收 `<-ch` 的三种情况：**
1. 发送队列里有人在等：
   - 无缓冲：直接从那个发送者手里拷贝；
   - 有缓冲（说明缓冲区是满的）：从缓冲区头部取一个，再把那个发送者的值放进缓冲区尾部，保证先进先出。
2. 缓冲区有数据：从 `buf[recvx]` 取。
3. 都不行：挂到 `recvq` 等待。

**为什么“直接拷贝到对方的栈上”**：少拷贝一次，也少一次加锁解锁。这是运行时里少数会直接写另一个 goroutine 的栈的地方。

**无缓冲和有缓冲的区别：**

| | 无缓冲 `make(chan T)` | 有缓冲 `make(chan T, n)` |
|---|---|---|
| 发送什么时候返回 | 接收方拿走数据的那一刻 | 放进缓冲区就返回 |
| 用途 | 同步：确认对方已经收到了 | 解耦：允许生产和消费的速度暂时不一致 |
| 内存可见性 | 发送前的写入，对接收后的代码可见 | 同样有这个保证 |

#### 项目里的例子

> **项目里没有直接体现，但面试常问**：hchan 的字段和发送接收的过程都在运行时内部。

改造前，每个连接有一个解析 goroutine，通过一个 channel 把解析好的请求交给处理 goroutine。改造时去掉了这个 channel，原因见本章的配角。现在 Mini-Redis 生产代码里**自己创建**的 channel，全都是 `chan struct{}`，只用来通知“某件事已经结束了”，靠关闭来广播，从不传数据。（标准库给的 channel 不在此列，比如后台清理从 `ticker.C` 收到的是 `time.Time`。）

| channel | 在哪里 | 谁关闭 | 谁在等 |
|---|---|---|---|
| `ctx.Done()` | `main` 用 `signal.NotifyContext` 得到信号 ctx，`run` 再用 `context.WithCancel` 派生一个子 ctx | 收到 Ctrl+C 时父 ctx 取消，子 ctx 跟着取消；`run` 返回前也会调用 `cancel()` | 后台清理、等待信号的 goroutine，等的都是子 ctx |
| `cleanupDone` | `cmd/server` 的 `run` | 后台清理的 goroutine 退出时 | `run` 返回前 |
| `watchStop`、`watchDone` | `cmd/server` 的 `run` | `Serve` 返回后、等待信号的 goroutine 退出时 | 等待信号的 goroutine、`run` |
| `s.done` | `server.Server` | `Shutdown` 做完时 | `Serve` 返回前 |
| `finished` | `Shutdown` 内部 | `wg.Wait()` 返回后 | `Shutdown` 的 select |

#### 常见追问

**追问：channel 是并发安全的吗？它的性能怎样？**

是的。发送、接收、关闭都要在 hchan 的锁下修改它的状态（运行时也有几条不加锁的快速判断，比如非阻塞发送时发现缓冲区已满，直接返回）。正因为有锁，大量 goroutine 同时争用同一个 channel 时，性能不如分片的数据结构；单纯保护一个计数器，也不如 atomic。channel 擅长的是传递所有权和事件，不是做高频的共享计数。

---

### 八股 2：对已关闭的 channel 和 nil channel 操作会怎样？

#### 一分钟口述版

> 记一张表就行。对 nil channel，发送和接收都会永远阻塞，关闭会 panic。对已关闭的 channel，接收不会阻塞：缓冲区里还有数据就先把数据读完，读完以后每次都立刻返回零值，第二个返回值 ok 是 false；发送会 panic；再关闭一次也会 panic。所以有一条原则：只由发送方关闭 channel，而且只关闭一次；有多个发送方时，不要让它们去关，而是另外用一个信号通知它们停止，比如 context。nil channel 永远阻塞，这个特性在 select 里很有用：把某个 case 的 channel 设成 nil，就相当于临时禁用了这个分支。

#### 要点

| 操作 | nil channel | 已关闭的 channel | 正常的 channel |
|---|---|---|---|
| 发送 `ch <- v` | 永远阻塞 | **panic**：send on closed channel | 有接收者或者缓冲区有空位就成功，否则阻塞 |
| 接收 `<-ch` | 永远阻塞 | 先读完缓冲区；读完后立刻返回零值，`ok == false` | 有数据就返回，否则阻塞 |
| 关闭 `close(ch)` | **panic**：close of nil channel | **panic**：close of closed channel | 成功，唤醒所有等待者 |

```go
package main

import "fmt"

func try(name string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, "→ panic:", r)
		}
	}()
	f()
	fmt.Println(name, "→ 正常")
}

func main() {
	ch := make(chan int, 1)
	ch <- 7
	close(ch)
	v, ok := <-ch
	fmt.Println("缓冲区里的数据还能读到：", v, ok)
	v, ok = <-ch
	fmt.Println("读完以后：", v, ok)

	try("发送到已关闭的 channel", func() { ch <- 1 })
	try("再次关闭", func() { close(ch) })
	var nilCh chan int
	try("关闭 nil channel", func() { close(nilCh) })
}

// 输出：
// 缓冲区里的数据还能读到： 7 true
// 读完以后： 0 false
// 发送到已关闭的 channel → panic: send on closed channel
// 再次关闭 → panic: close of closed channel
// 关闭 nil channel → panic: close of nil channel
```

**关闭是一种广播**：一个值只能被一个接收者收到，但关闭会让所有接收者都立刻返回。所以“通知所有 goroutine 停下”，用关闭一个 channel 来做，而不是往里面发 N 个值。`ctx.Done()` 就是这样实现的。

#### 项目里的例子

- 停机信号就是一次“关闭”：`signal.NotifyContext` 收到 Ctrl+C 以后取消 ctx，ctx 的 Done channel 被关闭，所有在 `select` 里等 `ctx.Done()` 的 goroutine 同时收到。[database/expire.go](../../database/expire.go) 的后台清理 `RunCleanup` 和 `cmd/server` 里等待信号的 goroutine，都在这样等。

#### 常见追问

**追问：有多个发送方时，怎样安全地关闭 channel？**

不要让任何一个发送方去关闭，否则别的发送方可能还在发，就会 panic。常见做法：用一个 WaitGroup 等所有发送方结束，由一个专门的 goroutine 在 `wg.Wait()` 之后关闭；或者根本不关闭数据 channel，用 context 通知发送方停止，数据 channel 交给 GC 回收。

---

### 八股 3：goroutine 泄漏是怎样产生的？怎样排查？

#### 一分钟口述版

> goroutine 泄漏就是一个 goroutine 永远阻塞，既不退出，也不会被 GC 回收，它的栈和它引用的所有对象都会一直占着内存。最常见的原因是阻塞在 channel 上：往一个没人接收的 channel 发送，比如调用方超时先返回了，结果没人接；从一个永远不会关闭的 channel 里 range；在 select 里等待，却没有可以退出的分支。其他原因还有：等一把永远不会被释放的锁；网络读写没有设置超时，对方一直不发数据也不断开。预防的办法是：每启动一个 goroutine，就想清楚它在什么条件下退出，通常是给它一个 context；channel 的缓冲要够用，保证发送方不会因为接收方走了而卡住。排查时，先看 `runtime.NumGoroutine()` 是不是一直在涨，再用 pprof 的 goroutine profile 看它们卡在哪一行。Go 1.27 的 pprof 里还有一个 goroutineleak profile，它借助 GC 找出那些阻塞在“已经没人能访问到的 channel 或锁”上的 goroutine，可以直接定位泄漏。

#### 要点

**一个典型的泄漏**：调用方等结果，最多等 10 毫秒；干活的 goroutine 要 50 毫秒。

```go
package main

import (
	"errors"
	"fmt"
	"runtime"
	"time"
)

func fetch(buffered bool) error {
	result := make(chan int)
	if buffered {
		result = make(chan int, 1) // 修复：留一个缓冲位，没人接收也能发出去
	}
	go func() {
		time.Sleep(50 * time.Millisecond) // 模拟一个慢操作
		result <- 42                      // 无缓冲时：调用方已经走了，这里永远阻塞
	}()
	select {
	case v := <-result:
		_ = v
		return nil
	case <-time.After(10 * time.Millisecond):
		return errors.New("timeout")
	}
}

func main() {
	fmt.Println(fetch(false))
	time.Sleep(200 * time.Millisecond)
	fmt.Println("无缓冲版本之后：", runtime.NumGoroutine(), "个 goroutine")

	fmt.Println(fetch(true))
	time.Sleep(200 * time.Millisecond)
	fmt.Println("有缓冲版本之后：", runtime.NumGoroutine(), "个 goroutine")
}

// 输出：
// timeout
// 无缓冲版本之后： 2 个 goroutine
// timeout
// 有缓冲版本之后： 2 个 goroutine
```

两次都是 2 个：一个是 main，另一个是**第一次**泄漏的那个，它永远不会退出了。第二次调用用了有缓冲的 channel，干活的 goroutine 发完就结束了，没有新增。

**用 goroutineleak profile 定位。** 这个 profile 在 Go 1.27 起默认可用；Go 1.26 要在构建时设置 `GOEXPERIMENT=goroutineleakprofile` 才有。下面是一个完整的程序，只保留了泄漏的那个版本，末尾输出 profile：

```go
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/pprof"
	"time"
)

func fetch() error {
	result := make(chan int) // 无缓冲：调用方走了，就没人接收
	go func() {
		time.Sleep(50 * time.Millisecond)
		result <- 42
	}()
	select {
	case v := <-result:
		_ = v
		return nil
	case <-time.After(10 * time.Millisecond):
		return errors.New("timeout")
	}
}

func main() {
	fmt.Println(fetch())
	time.Sleep(200 * time.Millisecond)
	pprof.Lookup("goroutineleak").WriteTo(os.Stdout, 1)
}
```

Go 1.27.1 上的输出（地址部分省略）：

```text
timeout
goroutineleak profile: total 1
1 @ 0x… 0x… 0x… 0x… 0x…
#	0x…	main.fetch.func1+0x…	.../main.go:15
```

它直接指出了泄漏的是哪个函数、卡在哪一行：第 15 行，`result <- 42`。原理是：跑一次特殊的 GC，如果一个 goroutine 阻塞在某个 channel 上，而这个 channel 已经没有任何还能运行的 goroutine 能访问到了，那它就再也不可能被唤醒，判定为泄漏。服务里引入了 `net/http/pprof` 的话，访问 `/debug/pprof/goroutineleak` 也能看到。

**排查工具：**

| 工具 | 用法 |
|---|---|
| `runtime.NumGoroutine()` | 作为监控指标，持续上涨就要警惕 |
| pprof 的 goroutine profile | 列出所有 goroutine 卡在哪一行，相同位置的会合并计数（第 08 章） |
| pprof 的 goroutineleak profile | 只列出判定为泄漏的 goroutine |
| 测试里检查 | 测试结束时比较 goroutine 数，或者用 `go.uber.org/goleak` |

#### 项目里的例子

- **每个连接的 goroutine 都有明确的退出条件**：客户端断开（Read 返回 EOF）、协议错误、或者停机。停机时，空闲连接的 goroutine 正挂在 Read 上，等不到数据，所以 [server/server.go](../../server/server.go) 的 `Shutdown` 会关闭这些连接，让 Read 返回错误。如果只设一个“停机”标记而不打断 Read，这些 goroutine 就会一直挂着。
- **后台清理的 goroutine** 在 `select` 里同时等 `ctx.Done()` 和 ticker，ctx 一取消就退出。
- [server/server_test.go](../../server/server_test.go) 的 `TestHundredAbnormalDisconnectsReturnToBaseline` 让 100 个客户端发半条请求就断开，再检查 goroutine 数回到基线；`TestConcurrentCloseInterruptsIdleConnections` 检查并发调用 Close 以后，登记的连接全部退出。

#### 常见追问

**追问：goroutine 泄漏和内存泄漏是什么关系？**

goroutine 泄漏一定会造成内存泄漏：它的栈至少几 KB，而且它引用的所有对象都不能被回收。如果它还拿着连接、文件这类资源，还会造成资源泄漏。反过来，内存泄漏不一定是 goroutine 泄漏造成的，比如一个只增不删的全局 map。

---

### 八股 4：context 是做什么的？取消是怎样传下去的？

#### 一分钟口述版

> context 解决的是“一个请求派生出很多 goroutine，请求被取消或者超时了，怎样通知它们全部停下”的问题。它是一棵树：从 Background 开始，每次调用 WithCancel、WithTimeout、WithDeadline，都会从父节点派生出一个子节点，子节点会把自己登记到父节点上。取消一个节点时，它会关闭自己的 Done channel，再依次取消所有子节点；子节点被取消，不影响父节点。所以取消信号只会从上往下传。goroutine 里通过 `select` 等 `ctx.Done()` 来响应取消，再用 `ctx.Err()` 看是被取消还是超时。使用时有几条规则：拿到 cancel 函数一定要调用，一般是 defer，否则子节点会一直挂在父节点上，直到父节点结束才释放，`go vet` 的 lostcancel 检查会报这个问题；ctx 作为第一个参数传递，不要存在结构体里；WithValue 只放请求范围的数据。

#### 要点

**创建 context 的函数：**

| 函数 | 什么时候取消 |
|---|---|
| `WithCancel(parent)` | 调用返回的 cancel 函数时 |
| `WithTimeout(parent, d)` / `WithDeadline(parent, t)` | 时间到，或者调用 cancel |
| `WithCancelCause(parent)`（Go 1.20） | 同 WithCancel，还能附带取消原因，用 `context.Cause(ctx)` 取出 |
| `WithoutCancel(parent)`（Go 1.21） | 永远不会被父节点取消，但保留父节点的值。用于“请求结束后还要继续做完”的收尾工作 |
| `AfterFunc(ctx, f)`（Go 1.21） | 不是创建 context，而是在 ctx 被取消时调用 f |

**取消的传播：**
- 子节点创建时，登记到最近的一个可取消的祖先节点的 children 集合里。
- 取消一个节点：关闭它的 Done channel → 递归取消所有子节点 → 把自己从父节点的 children 里删掉。
- Done channel 是关闭而不是发送，所以所有等它的 goroutine 都能同时收到（八股 2 讲的广播）。

**ctx.Err() 的两种值：** `context.Canceled`（被取消）、`context.DeadlineExceeded`（超时）。用 `errors.Is` 判断。

**context 不能做的事**：它只是“通知”，不会强行停止任何 goroutine。goroutine 自己不去检查 `ctx.Done()`，或者正阻塞在一个不认识 context 的操作上（比如没有设置截止时间的 `conn.Read`），取消信号就到不了它。所以网络连接要配合截止时间或者直接关闭连接，才能真正打断。

#### 项目里的例子

- [cmd/server/main.go](../../cmd/server/main.go) 的 `main` 用 `signal.NotifyContext` 得到一个“收到 Ctrl+C 就取消”的 ctx。`run` 再用 `context.WithCancel` 从它派生一个子 ctx，后台清理任务等的是这个子 ctx。`run` 返回前调用 `cancel()`，即使没有收到 Ctrl+C（比如 `Serve` 因为别的错误先返回了），后台清理也会停下。
- 停机有一个最长等待时间：等待信号的 goroutine 用 `context.WithTimeout(context.Background(), 2*time.Second)` 调用 `Shutdown`，超时了就不再等，强制关闭所有连接，并返回 `ctx.Err()`。
- 改造前，`aof.Open` 里的解析器要靠 ctx 取消来停下那个解析 goroutine；改成同步读以后，Open 只需要在每条记录之间检查一下 ctx。用不用 context，跟有没有需要被叫停的 goroutine 直接相关。

#### 常见追问

**追问：为什么 context 不建议存在结构体里？**

context 代表的是**一次调用**的生命周期，而结构体通常活得比一次调用长。存在结构体里，就说不清它到底管哪一次调用了。例外是像 `http.Request` 这种本身就代表一次请求的结构体。

---

### 八股 5：怎样实现优雅停机？

#### 一分钟口述版

> 优雅停机的目标是：不再接新活，手上的活做完，资源都释放干净，但也不能无限地等下去。一般分四步。第一，收到停止信号，比如 SIGTERM，用 `signal.NotifyContext` 把它变成 context 的取消。第二，停止接收新请求：关闭监听的 socket，让 Accept 返回错误，接受连接的循环就退出了。第三，处理已有的连接：正在执行的请求让它执行完、把响应写完；空闲的连接直接关掉；在这一步之前已经读进来、但还没开始执行的请求，就不再执行了。第四，等所有处理连接的 goroutine 都退出，用 WaitGroup 等，而且要带一个超时，超时了就强制关闭所有连接。最后按依赖关系的反方向释放资源：先停网络层，再停后台任务，最后关闭存储，因为网络层还在用存储。

#### 要点

**通用的四步：**

| 步骤 | 做什么 | 用到的工具 |
|---|---|---|
| 1. 接收信号 | Ctrl+C、SIGTERM → context 被取消 | `signal.NotifyContext` |
| 2. 停止接收 | 关闭监听 socket，Accept 循环退出 | `listener.Close()` |
| 3. 处理已有连接 | 执行中的做完；空闲的关掉；没开始的不再执行 | 每个连接的状态标记；`conn.Close()` 或截止时间打断阻塞的 Read |
| 4. 等待和超时 | 等所有连接的 goroutine 退出；超时就强制关闭 | `sync.WaitGroup`、带超时的 ctx |

**关闭的顺序要和依赖的方向相反**：网络层依赖存储引擎，存储引擎依赖 AOF 文件，所以先停网络层，再停后台清理，最后关 AOF。反过来做，就可能出现“连接还在执行命令，AOF 已经关了”。

**等待为什么要有超时**：有的客户端可能一直不读响应，写回复就会卡住；如果一直等，进程就永远退不出。Kubernetes 默认只给 30 秒，超时后直接 SIGKILL，那时什么收尾都来不及做。所以要自己掌握主动权：在超时之前，先强制关闭。

#### 项目里的例子

[server/server.go](../../server/server.go) 的 `Server` 负责连接和停机。每个连接的循环（[server/connection.go](../../server/connection.go) 的 `handle`）是：

1. 读一条请求；
2. 加锁，检查是否在停机。在停机就退出；没停机就把自己标记成“正在执行”；
3. 执行命令，写回复；
4. 加锁，标记回“空闲”。如果正在停机，就退出。

`Shutdown(ctx)` 做的事：
- 标记停机，关闭监听；
- 关闭所有**空闲**的连接，打断它们正在等待的 Read；
- **正在执行**的连接不动，等它写完回复，在第 4 步自己退出；
- 等所有连接的 goroutine 结束。ctx 先到期，就强制关闭全部连接，返回 `ctx.Err()`。客户端一直不读回复、写回复卡住时，也是靠这一步打断。

停机测试保证了三点：
- 停机时，已经开始执行的命令会把回复写完；
- 流水线里已经读进来、但还没开始执行的请求，不再执行；
- ctx 超时以后，Shutdown 如实返回错误，不假装一切都已经停止。

`cmd/server/main.go` 的关闭顺序：`Server.Shutdown` → `Serve` 返回 → 等后台清理退出 → 关闭 AOF。代码走读会逐段看。

#### 常见追问

**追问：已经读进来、还没执行的请求，为什么不执行完再退出？**

“执行完再退出”听起来更友好，但流水线里可能还有很多条，客户端也可能一直在发，停机就会被无限推迟。所以 Mini-Redis 的规则是：以“开始执行”为界，开始了的在排空期限内尽量做完，没开始的一律不做。

但要注意，**客户端没收到回复，并不能断定命令没执行**。比如一条 LPUSH 已经执行完、回复还没写出去，2 秒排空期限就到了，连接被强制关闭：客户端读到的是 EOF，列表里却已经多了一个元素。网络在回复途中断开也是一样。所以没收到回复时，结果是“未知”，LPUSH 这种执行两次结果就不同的命令，不能盲目重试。

---

### 配角：什么时候不该用 channel？

#### 一分钟口述版

> channel 适合在 goroutine 之间传递数据的所有权、通知事件，或者让生产者和消费者以不同的速度运行。但如果两件事本来就是严格顺序的，比如“读一条请求，处理它，再读下一条”，硬拆成两个 goroutine、中间用 channel 连起来，只会多出一个 goroutine、一次 channel 的同步，还多出一堆问题：谁负责关闭 channel，取消时怎样让两边都退出，退出前要不要把 channel 里剩下的东西读空。这时候，在一个 goroutine 里顺序执行，代码最简单，也最不容易出错。

#### 项目里的例子

**改造前**，每个连接有两个 goroutine：
- 解析 goroutine：从连接读字节、解析成请求，发到一个 channel；
- 处理 goroutine：从 channel 里取请求、执行、写回复。

这带来的问题：
- 每个连接多一个 goroutine。第 02 章的实验里，1000 个连接是 2000 多个 goroutine。
- 停机变复杂：既要让处理 goroutine 停下，又要叫停解析 goroutine，还要把 channel 读空，防止解析 goroutine 卡在发送上泄漏。改造前，`tcp.Server` 和 `resp.RespHandler` 各有 9 个和停机有关的字段（锁、WaitGroup、Once、通道、标志、停机错误），两套逻辑分在两个包里，想弄清“停机时谁先关、谁等谁”，要来回对照。
- 解析 goroutine 会提前读进后面的请求，“停机时没开始的请求不执行”这条规则就更难保证了。

**改造后**，每个连接只有一个 goroutine，按“读一条 → 执行 → 写回复”顺序循环，用 `resp.ReadRequest` 同步读。读和处理本来就是一前一后的，不需要 channel。停机相关的字段从两套共 18 个减少到一套 8 个：`mu`、`conns`、`stopping`、`wg`、`shutdown`、`done`、`stopErr`、`closeErr`。

这个例子可以在面试时用来回答“你在项目里做过什么取舍”：channel 是 Go 的特色，但用在不需要并发的地方，只会增加复杂度。

---

## 关键设计与为什么

**1. 以“开始执行”为界。**
- 开始了的命令在 2 秒的排空期限内做完、把回复写完；没开始的一律不做，哪怕已经读进了缓冲区。
- 这条规则让停机的结果是可以预期的：停机不会无限推迟，也不会去执行客户端以为“还在排队”的请求。
- 它保证不了的是：客户端没收到回复时，命令一定没执行。排空超时、网络断开，都可能出现“执行了但回复没送到”。这时结果未知，LPUSH 这种命令不能盲目重试。

**2. 停机检查和“标记正在执行”在同一把锁里。**
- 这样“开始执行”和“开始停机”之间一定有先后，不存在两边都以为对方会处理的空档。`startConnection` 里登记连接、`wg.Add(1)` 也和 `stopping` 的检查放在同一把锁里，停机开始后不会再有新的 goroutine 加入等待。

**3. 用关闭连接来打断 Read，而不是 context。**
- `conn.Read` 不认识 context，取消信号到不了它。关闭连接，挂起的 Read 立刻返回错误。
- 只关空闲的连接；正在执行的连接等它自己写完回复后退出。

**4. 超时要如实报告。**
- 2 秒内没等完，`Shutdown` 强制关闭所有连接，返回 `context deadline exceeded`，不假装执行器已经停下了。
- `Close` 则一定等到所有连接的 goroutine 真正退出才返回，执行器卡住它也等。两个方法分工明确：一个有时间限制，一个保证回收干净。

**5. 一个连接一个 goroutine，不用 channel 传请求。**
- 见上面的配角。字段从两套 18 个变成一套 8 个，停机逻辑从两个包合到一个包里。

---

## 动手实验

**实验 1：去掉“写完回复后检查停机”，看哪个测试会抓住它。**

在 [server/connection.go](../../server/connection.go) 的 `handle` 末尾，把

```go
stopping := s.stopping
```

改成 `stopping := false`，然后运行：

```text
go test ./server -run TestShutdownDrainsSingleInFlightReply -count=1
```

输出（2026-11-10 实测）：

```text
--- FAIL: TestShutdownDrainsSingleInFlightReply (2.00s)
    shutdown_test.go:67: Shutdown() = context deadline exceeded after 2.0004197s; want nil within 1s
FAIL
```

这个测试只发一条 SET，让执行器卡住，执行中调用 `Shutdown`（最多等 2 秒），再放行执行器。正常情况下，回复写完、连接自己退出，`Shutdown` 不到一毫秒就返回 nil。少了这一道检查，连接写完回复后回到 Read 上等待。这时它在 `s.conns` 里其实已经被标回了“空闲”，但 `Shutdown` 用的是停机开始那一刻拍下的快照：快照里它是“正在执行”，所以没有关它，之后也不会重新扫描。没人去关这个连接，`Shutdown` 只能一直等到 2 秒超时。

这个测试是代码复核时补上的：当时的测试里，客户端总是一次发两条命令，第二条会被“执行前的检查”拦下，连接也就退出了。两道检查互相兜底，单删哪一道，原有的测试都发现不了。后来分别补了 `TestShutdownDrainsSingleInFlightReply` 和 [server/connection_test.go](../../server/connection_test.go) 的 `TestConnectionRejectsRequestAfterStopping`，各管一道。做完把代码改回来。

**实验 2：用 goroutineleak 找出泄漏的 goroutine。**

运行八股 3 里那个完整的程序（需要 Go 1.27，或者 Go 1.26 加 `GOEXPERIMENT=goroutineleakprofile`），看它怎样指出卡住的那一行。再试着改一改：把 `make(chan int)` 换成 `make(chan int, 1)`，profile 里的泄漏数应该变成 0。

---

## 常见误区

- **由接收方关闭 channel，或者多个发送方都去关闭。** 只由唯一的发送方关闭。
- **用 `for v := range ch` 读一个永远不会被关闭的 channel。** 循环永远不会结束。
- **创建了 context 却不调用 cancel。** 子 context 要等父 context 结束才释放。
- **以为 context 取消就能停止 goroutine。** goroutine 自己不检查，取消就没有作用。
- **以为 ctx 取消能打断 `conn.Read`。** 不能，要关闭连接或者设截止时间。
- **把 `wg.Add` 写进新 goroutine 里。** `go vet` 的 waitgroup 检查会报告这个错误；`go test` 自动运行的 vet 不包括这项，要单独运行 `go vet`。
- **调试时用 time.Sleep 等 goroutine 结束。** 用 WaitGroup 或 channel 明确地等。
- **以为写了两道检查，测试就一定覆盖了两道。** 两道检查可能互相兜底，单删一道测试照样通过（动手实验 1）。

---

## 自测

**1. Ctrl+C 之后，到 AOF 文件被关闭，中间依次发生了什么？**

<details><summary>答案</summary>

ctx 被取消，后台清理马上退出，同时等待信号的 goroutine 调用 `Shutdown`：标记停机、关闭监听（`Accept` 随即返回错误）、关闭空闲连接、最多等 2 秒让执行中的连接写完 → `Shutdown` 做完，`Serve` 返回 → `run` 确认等待信号的 goroutine 结束 → defer 依次执行：确认后台清理已经退出、关闭 AOF（没有时间限制）、再关一次监听（第一次已经关过）。

</details>

**2. 停机时，一个空闲连接的 goroutine 正挂在 Read 上，它是怎样退出的？**

<details><summary>答案</summary>

`Shutdown` 关闭这个连接，挂起的 Read 立刻返回错误，`ReadRequest` 把它包装成协议错误返回。`handle` 拿着这个错误进入执行前的检查，看到 `stopping` 为 true，直接返回，defer 里注销连接、`wg.Done()`。

</details>

**3. 客户端一次发了两条 SET，第一条正在执行时开始停机。第二条会执行吗？客户端会收到几条回复？**

<details><summary>答案</summary>

第二条不执行，客户端只收到第一条的 `+OK`，然后连接被关闭。第一条已经开始执行，`Shutdown` 等它写完；写完后，写完回复后的检查发现在停机，连接退出，缓冲区里的第二条不再处理。

</details>

**4. 为什么 `Shutdown` 里要起一个 goroutine 去 `wg.Wait()`？**

<details><summary>答案</summary>

`wg.Wait()` 是阻塞调用，不能放进 select 和 `ctx.Done()` 一起等。起一个 goroutine 去等，等完关闭 `finished` channel，主流程就能 select `finished` 和 `ctx.Done()`，哪个先到处理哪个。

</details>

**5. 等待信号的 goroutine 调用 `Shutdown` 时，为什么用 `context.Background()` 派生新的 ctx，而不是用收到信号的那个 ctx？**

<details><summary>答案</summary>

收到信号的那个 ctx 已经被取消了，传给 `Shutdown` 会立刻走到超时分支，强制关闭所有连接，正在执行的命令来不及写完回复。需要一个新的、带 2 秒期限的 ctx 作为排空的时间。

</details>

**6. 改造后，生产代码里的 channel 有什么共同点？**

<details><summary>答案</summary>

项目自己创建的 channel 都是 `chan struct{}`，都只用关闭来通知“某件事结束了”，从不发送数据：`cleanupDone`、`watchStop`、`watchDone`、`Server.done`、`finished`，再加上 context 提供的 `ctx.Done()`。传递请求这种“数据”的 channel 已经去掉了。标准库给的 `ticker.C` 是例外，它传的是时间。

</details>

---

## 开发过程回放

以下按 git 历史整理。

- **2026-05-20，`2343d6e`，引入解析 channel。** `ParseStream` 起一个 goroutine 解析，通过 `chan *Payload` 把结果交给处理函数。当时的注释写着：“chan *Payload 双向通道，既能读又能写”。
- **2026-10-01，`3d95ea3`，有序停机。** tcp 包的 `Server` 和 resp 包的 `RespHandler` 各自加上了 closed 标志、WaitGroup、两个 `sync.Once` 和停机错误；`cmd/server` 收到信号后先停网络、再等后台、最后关 AOF 的顺序也在这时定下。
- **2026-11-10，PR #5，可读性改造。**
  - 去掉解析 goroutine 和它的 channel，tcp 包改名为 server，两套停机逻辑合成一套。字段从 18 个减到 8 个。
  - 原来的停机测试场景全部保留，改用真实连接或 `net.Pipe`，不再依赖注入的假解析器。
  - 代码复核时用“故意改坏一处，看测试能不能发现”的办法，发现两道停机检查在测试里互相兜底；补了两个测试，同时删掉了一段多余的代码：`Shutdown` 曾经给执行中的连接设写截止时间，但 ctx 到期时本来就会关闭所有连接，两者效果相同。

---

## 核对来源（Go 1.27.1）

| 说法 | 出处 |
|---|---|
| hchan 的字段 | `runtime/chan.go`：`type hchan struct` |
| 发送时有接收者在等就直接拷贝到对方栈上 | `runtime/chan.go`：`chansend` 中的 `send`、`sendDirect` |
| 缓冲区满时接收：取头部，把发送者的值放到尾部 | `runtime/chan.go`：`recv` 的注释 |
| 关闭时唤醒所有接收者和发送者，发送者 panic | `runtime/chan.go`：`closechan`；`chansend` 中的 `panic(plainError("send on closed channel"))` |
| goroutineleak profile 默认可用、原理 | `runtime/pprof/pprof.go`：内置 profile 列表里的 `"goroutineleak"`、`writeGoroutineLeak`；`runtime/mgc.go`：`goroutineLeakGC`；`runtime/mgcmark.go` 中的相关注释。输出为 2026-11-10 本机实测 |
| `/debug/pprof/goroutineleak` | `net/http/pprof/pprof.go` 中的 profile 描述 |
| WaitGroup.Go | `sync/waitgroup.go`（Go 1.25 加入） |
| vet 的 waitgroup 检查；`go test` 自动运行的 vet 不包括 waitgroup、lostcancel、copylocks | `cmd/vendor/golang.org/x/tools/go/analysis/passes/waitgroup/doc.go`；`cmd/vet/doc.go`；`cmd/go/internal/test/test.go`：`defaultVetFlags` |
| WithCancelCause、WithoutCancel、AfterFunc、OnceValue | Go 1.20、Go 1.21 release notes；`context/context.go`；`sync/oncefunc.go` |
| signal.NotifyContext | Go 1.16 release notes；`os/signal/signal.go` |
| 停机测试的 `-v` 输出、去掉写完后检查的失败输出 | 2026-11-10 在 dbd2e9f 上实际运行 |
| 两套共 18 个停机字段 → 8 个 | `git show 31684b7:tcp/server.go`、`git show 31684b7:resp/handler.go`；PR #5 的第 4 步说明；`server/server.go` 的 `Server` 定义 |
| 复核补测试、删除 SetWriteDeadline | PR #5 的提交 `dbd2e9f` |
