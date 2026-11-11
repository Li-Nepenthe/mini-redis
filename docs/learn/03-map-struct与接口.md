# 第 03 章：map、struct、方法与接口

本章在项目里对应 **database 包**：用 map 存数据，用接口把引擎和日志、网络层隔开。

## 这一章能回答的面试题

1. map 的底层是怎样实现的？Go 1.24 改了什么？（八股 1）
2. map 为什么是无序的？（八股 1 追问）
3. map 并发写为什么会让程序崩溃？怎样解决？sync.Map 适合什么场景？（八股 2）
4. 哪些类型可以作为 map 的 key？
5. interface 的底层是什么？（八股 3）
6. 为什么一个 nil 指针赋给接口以后，接口不等于 nil？（八股 3）
7. 值接收者和指针接收者有什么区别？怎么选？（八股 4）
8. 空结构体有什么用？结构体的内存对齐是怎么回事？
9. Go 有继承吗？结构体嵌入是什么？
10. （配角）Redis 的数据结构，和 Mini-Redis 的实现有什么不同？

---

## 跑起来看什么

启动服务端和客户端，输入：

```text
mini-redis> SET k v
OK
mini-redis> LPUSH k x
(error) WRONGTYPE Operation against a key holding the wrong kind of value
mini-redis> LPUSH list a
(integer) 1
mini-redis> GET list
(error) WRONGTYPE Operation against a key holding the wrong kind of value
mini-redis> EXISTS k k missing
(integer) 2
```

同一个 key 只能是一种类型：存的是字符串，就不能当列表用，反过来也一样。引擎里只有一个 map，值的类型是 `any`，每次取出来都要先问一句“你是不是我要的类型”，答案是否定的就返回 `WRONGTYPE`。这句“问一句”就是本章要讲的类型断言。

`EXISTS k k missing` 返回 2：同一个 key 写两次就数两次，这是 Redis 的规定。

---

## Go 基础

### 1. map

```go
package main

import (
	"fmt"
	"maps"
	"slices"
)

func main() {
	ages := map[string]int{"alice": 30} // 字面量
	ages["bob"] = 25                    // 写入
	delete(ages, "alice")               // 删除，key 不存在也不报错

	v, ok := ages["carol"] // comma-ok：区分“不存在”和“值是零值”
	fmt.Println(v, ok, len(ages))

	// 集合：值用 struct{}，不占空间
	seen := map[string]struct{}{}
	for _, k := range []string{"a", "b", "a"} {
		seen[k] = struct{}{}
	}
	fmt.Println(len(seen))

	// 遍历顺序不固定。需要稳定的顺序，就先把 key 取出来排序（Go 1.23 起可以这样写）
	m := map[string]int{"c": 3, "a": 1, "b": 2}
	for _, k := range slices.Sorted(maps.Keys(m)) {
		fmt.Print(k, "=", m[k], " ")
	}
	fmt.Println()
}

// 输出：
// 0 false 1
// 2
// a=1 b=2 c=3
```

**map 的 key 必须是可比较的类型**：数字、字符串、布尔、指针、channel、接口，以及只由这些组成的数组和结构体。切片、map、函数不能作 key，编译时就会报错。

**map 的元素不能取地址**：`&m["k"]` 编译报错。原因是 map 扩容时会搬动元素，地址会失效。所以值是结构体时，`m["k"].Count++` 也不行，要么整个取出来、改完再存回去，要么值改用指针。

### 2. struct 与嵌入

```go
package main

import "fmt"

type Base struct{ ID int }

func (b Base) Describe() string { return fmt.Sprintf("id=%d", b.ID) }

type User struct {
	Base // 嵌入：没有字段名，只写类型
	Name string
}

func main() {
	u := User{Base: Base{ID: 7}, Name: "alice"}
	fmt.Println(u.ID)         // Base 的字段被“提升”，可以直接访问
	fmt.Println(u.Describe()) // 方法也被提升
	fmt.Println(u.Base.ID)    // 也可以写全

	p := User{Name: "bob"}
	q := User{Name: "bob"}
	fmt.Println(p == q) // 所有字段都可比较，结构体就可以用 == 比较
}

// 输出：
// 7
// id=7
// 7
// true
```

嵌入**不是继承**。`u.Describe()` 实际调用的是 `u.Base.Describe()`，方法的接收者是里面那个 Base，它不知道自己被嵌在 User 里，也就没有“子类重写父类方法”这回事。Go 用嵌入复用代码，用接口实现多态。

### 3. 方法

方法就是带了接收者的函数。接收者可以是值，也可以是指针：

```go
func (c Counter) Get() int { return c.n } // 值接收者：拿到的是一份拷贝
func (c *Counter) Inc()    { c.n++ }      // 指针接收者：能修改调用者那一份
```

第 00 章已经见过。怎样选，见八股 4。

### 4. 接口

接口是一组方法签名。**一个类型只要有这些方法，就自动实现了这个接口**，不需要写 `implements`。

```go
package main

import (
	"fmt"
	"strconv"
)

type Reply interface {
	Encode() string
}

type Status string
type Integer int64

func (s Status) Encode() string  { return "+" + string(s) + "\r\n" }
func (i Integer) Encode() string { return ":" + strconv.FormatInt(int64(i), 10) + "\r\n" }

// 编译期检查：如果 Status 没有实现 Reply，这一行会编译报错
var _ Reply = Status("")

func describe(v any) string {
	switch x := v.(type) { // type switch：按动态类型分支
	case nil:
		return "nil"
	case Reply:
		return fmt.Sprintf("%q", x.Encode())
	case []byte:
		return "bytes:" + string(x)
	default:
		return fmt.Sprintf("不支持的类型 %T", x)
	}
}

func main() {
	var r Reply = Integer(42)
	fmt.Println(r.Encode() == ":42\r\n")

	i, ok := r.(Integer) // 类型断言，带 ok 的写法失败时不会 panic
	fmt.Println(i, ok)
	_, ok = r.(Status)
	fmt.Println(ok)

	fmt.Println(describe(Status("OK")))
	fmt.Println(describe([]byte("v")))
	fmt.Println(describe(true))
	fmt.Println(describe(nil))
}

// 输出：
// true
// 42 true
// false
// "+OK\r\n"
// bytes:v
// 不支持的类型 bool
// nil
```

- `any` 就是 `interface{}`，空接口，任何类型都实现了它。
- 类型断言 `x.(T)` 不带 ok 时，断言失败会 panic。
- 惯用法：**接口定义在使用它的一方**，而且尽量小，一两个方法就够。实现它的一方不需要 import 接口所在的包。

### 5. 结构体的内存布局

CPU 按“对齐”的地址读数据最快：8 字节的 int64 最好放在 8 的倍数的地址上。所以编译器会在字段之间插入空白（填充），让每个字段都对齐。**字段的顺序会影响结构体的大小。**

```go
package main

import (
	"fmt"
	"unsafe"
)

type Loose struct {
	a bool  // 1 字节，后面填充 7 字节，让 b 对齐到 8
	b int64 // 8 字节
	c bool  // 1 字节，后面填充 7 字节，让整个结构体是 8 的倍数
}

type Tight struct {
	b int64
	a bool
	c bool // a、c 挨在一起，后面只填充 6 字节
}

func main() {
	fmt.Println(unsafe.Sizeof(Loose{}), unsafe.Sizeof(Tight{}))
	fmt.Println(unsafe.Sizeof(struct{}{})) // 空结构体不占空间
}

// 输出：
// 24 16
// 0
```

- 按字段从大到小排列，一般能减少填充。只有在结构体数量很多（比如几百万个）时才值得在意。
- **空结构体 `struct{}` 大小为 0。** 常见用法：
  - `map[string]struct{}` 当集合用，只关心 key；
  - `chan struct{}` 当信号用，只关心“发生了”，不传数据（第 07 章）。

---

## 代码走读：database 包的数据结构和四个接口

### 1. 引擎和分片：两个 struct

[database/engine.go](../../database/engine.go)：

```go
type Engine struct {
	log        CommandLog
	logMu      sync.Mutex
	shards     []*shard
	shardCount uint32
	now        func() time.Time
	reclaim    func()
}
```

[database/shard.go](../../database/shard.go)：

```go
type shard struct {
	mu       sync.RWMutex
	data     map[string]any
	expires  map[string]expiration
	expiring []string
}
```

- 数据分散在多个分片里，每个分片有一把读写锁和自己的 map。为什么要分片，第 04 章讲。
- `data` 的值是 `any`：字符串存 `[]byte`，列表存 `*linkedList`。一个 key 只能在一个 map 里，所以不可能既是字符串又是列表。
- `expires` 记录哪些 key 有过期时间；`expiring` 是同一批 key 的切片，供后台随机抽查（第 05 章）。
- `now` 和 `reclaim` 是两个函数类型的字段，平时分别是 `time.Now` 和 `debug.FreeOSMemory`。测试里把 `now` 换成一个可以手动拨动的时钟，就能测“5 秒后过期”而不用真的等 5 秒。函数也是值，可以存进结构体，这比专门定义一个时钟接口更省事。
- `shards` 是 `[]*shard`，存的是指针：每个分片里有锁，锁不能被拷贝。

### 2. 路由：key 落在哪个分片

```go
func fnv32(key string) uint32 {
	hash := uint32(2166136261)
	for i := 0; i < len(key); i++ {
		hash *= 16777619
		hash ^= uint32(key[i])
	}
	return hash
}

func (e *Engine) getShard(key string) *shard {
	return e.shards[fnv32(key)&(e.shardCount-1)]
}
```

- `fnv32` 是 32 位的 FNV-1 哈希：先乘一个质数，再异或一个字节。它和标准库 `hash/fnv` 的 `New32()` 算出来的结果完全一样（动手实验 3 验证过）。
- `& (shardCount-1)` 取哈希的低几位。分片数是 2 的幂时，它和 `% shardCount` 结果相同，但只要一条位运算指令。这就是 `NewEngine` 要求分片数必须是 2 的幂的原因。

### 3. any 和类型断言：取出来先问类型

[database/write.go](../../database/write.go) 的 `planLPush`：

```go
raw, exists := s.data[key]
fresh := !exists || string(args[0]) == recordNewList
list := newLinkedList()
if !fresh {
	var ok bool
	list, ok = raw.(*linkedList)
	if !ok {
		return writePlan{}, ErrTypeMismatch
	}
}
```

- `raw.(*linkedList)` 带 ok 的类型断言：值确实是列表，ok 为 true；是字符串，ok 为 false，返回 `WRONGTYPE`，不会 panic。
- [database/read.go](../../database/read.go) 的 `get` 反过来断言 `value.([]byte)`。所有读写命令都是这样先断言、再操作。

### 4. 四个接口，都定义在使用它的一方

| 接口 | 定义在 | 方法 | 谁实现 | 用途 |
|---|---|---|---|---|
| `CommandLog` | [database/replay.go](../../database/replay.go) | `Append(args [][]byte) error` | `*aof.Store` | 引擎写日志，不知道日志是文件 |
| `Executor` | [server/server.go](../../server/server.go) | `Exec(args [][]byte) (any, error)` | `*database.Engine` | 网络层执行命令，不知道背后是哪个引擎 |
| `publicError` | [resp/reply.go](../../resp/reply.go) | `RESPError() (code, message string)` | database 的 `*commandError` | 编码器只把允许公开的错误内容发给客户端 |
| 匿名接口 `interface{ RESPStatus() string }` | resp/reply.go 的 type switch 里 | `RESPStatus() string` | `database.StatusReply` | 编码器认出“这是一个状态回复” |

这四个接口有两个共同点：

- **都只有一个方法。** 使用方需要什么就要求什么，不多要。
- **实现方都不 import 接口所在的包。** aof 不 import database，database 不 import resp 和 server。`*aof.Store` 有 `Append` 方法，就自动满足了 `CommandLog`，Go 不需要写 `implements`。把两边接起来的只有 [cmd/server/main.go](../../cmd/server/main.go) 里的两行：`dbEngine.AttachLog(store)` 和 `server.NewServer(addr, dbEngine)`。

匿名接口那一个最极端：resp 包连名字都没给它起，直接写在 type switch 的 case 里：

```go
switch value := result.(type) {
case nil:
	return []byte("$-1\r\n"), nil
case []byte:
	return appendBulk(nil, value), nil
case [][]byte:
	// ……数组
case interface{ RESPStatus() string }:
	status := value.RESPStatus()
	// ……检查不含换行，编码成 +OK\r\n
case int64:
	return []byte(":" + strconv.FormatInt(value, 10) + "\r\n"), nil
default:
	return nil, fmt.Errorf("unsupported reply type: %T", value)
}
```

这个 switch 就是“Go 类型 → RESP 类型”的完整对应表，只此一处。`int`、`bool`、普通 `string` 都不在表里，传进来直接报错，不去猜它们该编码成什么。

---

## Go 八股

### 八股 1：map 的底层是怎样实现的？

#### 一分钟口述版

> Go 1.24 起，map 改用了 Swiss Table 的设计。数据按“组”存放，每组 8 个槽位，外加一个 8 字节的控制字，每个字节对应一个槽位，记录这个槽是空的、已删除，还是存了 key；存了 key 的，还保存 key 哈希值的低 7 位。查找时先用哈希的高位定位到一组，再用一次位运算把 8 个控制字节和目标的低 7 位同时比较，只对匹配上的槽位做真正的 key 比较。这组没有、并且组里没有空槽，就按二次探测去下一组找。为了避免扩容时一次搬动太多数据，一个 map 被拆成多张表，每张表最多 1024 个槽，表满了就自己翻倍或者一分为二，用哈希的最高几位决定 key 属于哪张表。在这之前，Go 的 map 是“桶加溢出链”的结构：每个桶 8 个 key，装不下就挂溢出桶，平均负载超过 6.5 就扩容，扩容时边写边搬。很多面试题还在问旧实现，两种都要能讲。

#### 要点

**Go 1.24 起：Swiss Table**（`internal/runtime/maps`）

| 概念 | 说明 |
|---|---|
| 槽位（slot） | 存一对 key 和 value |
| 组（group） | 8 个槽位，加一个 8 字节的控制字 |
| 控制字 | 每个字节对应一个槽：空、已删除，或者“已使用 + 哈希的低 7 位（H2）” |
| 表（table） | 一个完整的哈希表，由若干组构成。容量最多 1024 个槽 |
| 目录（directory） | 一个 map 有多张表时，用哈希的最高几位选择表（可扩展哈希） |

- **查找**：用哈希的高 57 位（H1）定位起始的组 → 用 H2 和控制字一次比较 8 个槽 → 对匹配的槽比较真正的 key → 没找到、而且这组还有空槽，说明 key 不存在；没有空槽就按二次探测去下一组。
- **负载上限**：平均每组最多用 7 个槽（7/8），超过就扩容。
- **扩容**：一张表还没到 1024 个槽时，换成一张两倍大的新表；到了上限就**一分为二**。每次扩容只动一张表，不会一次搬整个 map。
- **删除**：如果这组已经满了，删除时只能把槽标成“已删除”（墓碑），不能标成“空”，否则查找会提前停下，漏掉探测序列后面的 key。墓碑在扩容时才被清理。
- **小 map**：不超过 8 个元素时只有一个组，连表都不用建。

**Go 1.23 及之前：桶加溢出链**（面试常问）

- `hmap` 指向一个桶数组，桶的个数是 2 的幂。每个桶存 8 个 key 和 8 个 value，外加 8 个 tophash（哈希的高 8 位），用来快速排除。
- 一个桶满了，就挂一个**溢出桶**，形成链表。
- 平均每个桶超过 6.5 个元素，就把桶数翻倍；溢出桶太多但元素不多时，做一次等量扩容来整理。
- 扩容是**渐进式**的：不一次搬完，之后每次写入时顺便搬一两个桶。

**为什么换实现**：Swiss Table 用一次位运算比较 8 个槽，查找更快；数据是连续存放的，对 CPU 缓存更友好；也没有溢出链表的指针跳转。

**遍历为什么无序**：遍历的起点（从哪张表、哪个槽开始）是随机的。这是有意设计的，防止有人依赖遍历顺序；扩容以后元素的位置本来也会变。

#### 项目里的例子

- [database/shard.go](../../database/shard.go) 里，每个分片有一个 `data map[string]any`，key 是 Redis 的 key，value 是字符串（`[]byte`）或列表（`*linkedList`）。
- map 的 key 用 `string` 而不是 `[]byte`：切片不能作 key。这也正好让存储有一份自己的 key，不依赖网络缓冲区（第 01 章八股 3）。
- `NewEngine` 创建时就用 `make` 建好每个分片的 map，不会出现写 nil map 的 panic（第 00 章八股 1）。服务端用的是 `NewEngine(16)`，16 个分片。

#### 常见追问

**追问 1：map 的 value 能取地址吗？为什么？**

不能。扩容会搬动元素，地址会失效。新实现里表一分为二时也会搬动。

**追问 2：删除大量元素以后，map 占的内存会变小吗？**

不会。删除只是把槽位标成空或者已删除，表不会缩小；连 `clear(m)` 也只是清空槽位，表的大小不变。要真正释放内存，只能建一个新 map，把还要的元素拷过去，让旧 map 被 GC 回收。第 05 章讲内存时还会提到。

**追问 3：Redis 的哈希表扩容也是渐进式的，它们有什么区别？**

Redis 的 dict 有两张表，扩容时新建一张两倍大的表，之后每次操作顺便搬一个桶，搬完再丢掉旧表；这和 Go 旧实现的思路一样，都是把一次大搬迁摊到很多次操作里。Go 新实现换了一种办法：把 map 拆成多张小表，每次只扩一张，单次扩容的代价本来就小。

---

### 八股 2：map 并发写为什么会崩？怎样解决？

#### 一分钟口述版

> Go 的 map 不是并发安全的。多个 goroutine 同时写，或者一边读一边写，内部结构可能被改坏。所以运行时在 map 里放了一个“正在写”的标记：写操作开始时设置，结束时清除；如果另一个操作发现这个标记已经被设置了，就直接报 `fatal error: concurrent map writes`，终止整个程序。注意这是 fatal error，不是 panic，recover 接不住。之所以设计得这么严厉，是因为数据已经可能损坏了，带着坏数据继续跑更危险。解决办法有三种：用一把互斥锁或读写锁保护 map；用 sync.Map；或者像 Mini-Redis 这样，把数据按 key 分到多个分片，每个分片一个 map 一把锁，减少竞争。

#### 要点

- 检测是**尽力而为**的：只有两个操作正好重叠时才能发现。没被发现不代表没有问题，要用 `go test -race` 检查（第 08 章）。
- 只读不写是安全的：多个 goroutine 同时读同一个 map，没有问题。

**三种解决办法对比：**

| 办法 | 适合 | 缺点 |
|---|---|---|
| `sync.Mutex` 或 `sync.RWMutex` + map | 大多数场景，简单直接 | 所有操作抢一把锁，并发高时会排队 |
| `sync.Map` | 读多写少；或者各个 goroutine 读写的 key 基本不重叠 | 没有类型参数，存取都是 `any`；不能取长度；不适合需要“同时改几个 key”的操作 |
| 分片：N 个 map，每个一把锁 | 并发高、key 分布均匀 | 实现复杂一些；跨分片的操作要按顺序加多把锁 |

`sync.Map` 在 Go 1.24 换成了并发哈希前缀树（`HashTrieMap`）的实现，不同 key 的写入之间竞争更少。

下面这段代码大概率会让程序崩溃。它不是每次都崩，这正说明运行时的检测是“碰上了才报”：

```go
package main

import "sync"

func main() {
	m := map[int]int{}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 100000 {
				m[j] = i // 8 个 goroutine 同时写同一个 map
			}
		})
	}
	wg.Wait()
}

// 运行结果（大概率）：fatal error: concurrent map writes，后面跟着所有 goroutine 的堆栈
```

#### 项目里的例子

- **分片**：引擎用 FNV-1 算出 key 的哈希，取低 4 位（`hash & 15`，相当于对 16 取余），分到 16 个分片里，每个分片有自己的 map 和读写锁。不同分片上的命令可以并行执行。
- 为什么不用 sync.Map：Mini-Redis 的命令要做“读出旧值、检查类型、算出新值、写回去”这样的复合操作，DEL 还要同时动好几个 key。sync.Map 只保证单次操作是原子的，保证不了这一串操作整体原子，最后还是要加锁。
- 第 04 章细讲分片锁和多个 key 怎样按顺序加锁。

#### 常见追问

**追问：为什么 Go 不直接把 map 做成并发安全的？**

加锁有成本，而大多数 map 只在一个 goroutine 里使用，不需要这个成本。需要并发访问的场景，由使用者自己选择合适的锁的粒度，比如一把锁还是分片。

---

### 八股 3：interface 的底层是什么？为什么 nil 指针赋给接口后不等于 nil？

#### 一分钟口述版

> 接口值在底层是两个指针。空接口 `any` 是 eface，存的是“动态类型”和“数据指针”；带方法的接口是 iface，第一个字段是一个 itab 指针，itab 里有接口类型、动态类型，以及这个类型实现接口方法的函数地址表，调用接口方法就是查这张表。判断接口是否为 nil，要两部分都是 nil 才算。如果把一个值为 nil 的 `*MyError` 指针赋给 error 接口，类型那一半就有了，是 `*MyError`，所以接口不等于 nil。这个坑最常出现在函数返回 error 的时候：先声明了一个具体错误类型的指针变量，最后把它返回，调用者那边的 `err != nil` 就永远成立了。正确的写法是：没有错误时直接 `return nil`。

#### 要点

**底层结构**（`runtime/runtime2.go`、`internal/abi/iface.go`）：

```text
type eface struct {      // any
	_type *_type
	data  unsafe.Pointer
}

type iface struct {      // 带方法的接口
	tab  *itab
	data unsafe.Pointer
}

type ITab struct {
	Inter *InterfaceType // 接口类型
	Type  *Type          // 动态类型
	Hash  uint32         // 动态类型的哈希，type switch 用它快速比较
	Fun   [1]uintptr     // 方法地址表，实际长度等于接口的方法个数
}
```

- 某个类型和某个接口的 itab 第一次用到时生成，之后缓存在全局表里复用。
- 把一个非指针的值存进接口，通常要把值拷贝到堆上，让 data 指向这份拷贝。这也是接口有开销的原因之一。

**typed nil 的坑：**

```go
package main

import "fmt"

type MyErr struct{}

func (*MyErr) Error() string { return "boom" }

func mayFail(fail bool) error {
	var p *MyErr // 一个值为 nil 的 *MyErr
	if fail {
		p = &MyErr{}
	}
	return p // 错误写法：类型那一半是 *MyErr，接口不是 nil
}

func mayFailFixed(fail bool) error {
	if fail {
		return &MyErr{}
	}
	return nil // 正确写法：没有错误就直接返回 nil
}

func main() {
	err := mayFail(false)
	fmt.Println(err == nil)
	fmt.Printf("%T\n", err)
	fmt.Println(mayFailFixed(false) == nil)
}

// 输出：
// false
// *main.MyErr
// true
```

#### 项目里的例子

- **CommandLog 接口定义在 database 包里**（[database/replay.go](../../database/replay.go)），只有一个 `Append` 方法。aof 包的 Store 有这个方法，就自动实现了它，aof 包不需要 import database。这样 database 不依赖 aof，aof 也不依赖 database，没有循环依赖；测试里还可以塞一个只记录参数的假日志，[database/golden_test.go](../../database/golden_test.go) 固定 AOF 记录序列，就是这样做的。
- **两个包互不 import，却能对上接口**：`StatusReply` 定义在 database 包里，有一个 `RESPStatus() string` 方法。[resp/reply.go](../../resp/reply.go) 的 `EncodeReply` 在 type switch 里写的是一个匿名接口 `case interface{ RESPStatus() string }:`，只要求“有这个方法”。于是 database 不需要 import resp，resp 也不需要 import database，SET 返回的 `StatusReply("OK")` 照样被编码成 `+OK`。这就是隐式实现的好处：接口由使用方按需定义，提供方不用知道它。
- **回复类型靠 type switch 编码**：`EncodeReply` 按执行结果的动态类型选择 RESP 类型：实现了状态接口的输出 `+OK`，`int64` 输出整数，`[]byte` 输出 bulk，`[][]byte` 输出数组，`nil` 输出空值。早期版本里 SET 返回 Go 的 `true`，编码器把 `true` 翻译成 `+OK`，而整数回复有时是 `int`、有时是 `int64`。改造时统一成 `StatusReply` 和 `int64`，去掉了 `bool` 和 `int` 两个分支，让“Go 类型 → 协议类型”的对应关系只在一处定义。
- **引擎的 value 是 `any`**：取出来以后用 `raw.(*linkedList)` 断言是不是列表，不是就返回类型错误（`WRONGTYPE`）。
- server 包只依赖一个只有 `Exec` 方法的 `Executor` 接口（[server/server.go](../../server/server.go)），不依赖具体的引擎。

#### 常见追问

**追问 1：两个接口值用 == 比较，比的是什么？**

先比动态类型，类型相同再比值。如果动态类型本身不可比较，比如切片，运行时会 panic。

**追问 2：接口调用比直接调用慢在哪里？**

要通过 itab 的方法表间接跳转，编译器也很难把它内联；值存进接口时可能还要在堆上分配。热点路径上有时会因此改用具体类型。不过对大多数代码来说，这点开销可以忽略。

**追问 3：怎样在编译期确认一个类型实现了某个接口？**

写一行 `var _ Reply = (*T)(nil)`。没实现的话，编译直接报错。

---

### 八股 4：值接收者和指针接收者有什么区别？怎么选？

#### 一分钟口述版

> 值接收者拿到的是调用者的一份拷贝，改了也不影响调用者；指针接收者拿到的是地址，能修改调用者本身。它们还影响接口的实现：类型 T 的方法集只包含值接收者的方法，`*T` 的方法集两种都包含。所以如果一个方法是指针接收者，只有 `*T` 实现了接口，把 T 的值赋给接口会编译报错。怎么选：方法要修改接收者，用指针；结构体里有锁，必须用指针，因为拷贝锁会让锁失效；结构体很大，用指针避免拷贝。小的、不可变的类型，比如 `time.Time`，用值。还有一条惯例：一个类型的方法，要么都用值接收者，要么都用指针接收者，不要混用。

#### 要点

| | 值接收者 `func (t T) M()` | 指针接收者 `func (t *T) M()` |
|---|---|---|
| 拿到的是 | 一份拷贝 | 地址 |
| 能修改调用者吗 | 不能 | 能 |
| T 的值能调用吗 | 能 | 能，只要它可以取地址（是变量），Go 自动取 `&t` |
| T 的值能赋给接口吗 | 能 | **不能**，只有 `*T` 实现了接口 |

为什么 T 的值不能满足指针接收者的接口：接口里存的是 T 的一份拷贝，取不到原来那个变量的地址，调用指针方法也改不到原值，所以编译器干脆不允许。

```go
type Store struct{ mu sync.Mutex }
func (s *Store) Append(b []byte) error { ... }

var _ CommandLog = &Store{} // 可以
var _ CommandLog = Store{}  // 编译错误：Store does not implement CommandLog (method Append has pointer receiver)
```

#### 项目里的例子

- Engine、shard、aof.Store、server.Server 都用**指针接收者**：它们都有锁，都要修改自己的状态。如果误写成值接收者，`go vet` 的 copylocks 检查会报出“复制了锁”（动手实验 1）。
- [database/engine.go](../../database/engine.go) 的 `StatusReply` 是一个底层类型为 `string` 的自定义类型，它的 `RESPStatus` 方法用**值接收者**：它很小，也不可变。

#### 常见追问

**追问：值接收者的方法里修改接收者，会发生什么？**

编译能通过，但改的是拷贝，调用者看不到。这是常见的 bug：本来想改状态，接收者却写成了值。

---

### 配角：Redis 的数据结构和 Mini-Redis 的实现

| | Redis | Mini-Redis |
|---|---|---|
| String | SDS（带长度和空闲空间的动态字符串），小整数还会直接编码成整数 | `[]byte` |
| List | quicklist：一串 listpack（紧凑的连续内存块）连成的双向链表 | 普通的双向链表，每个节点一个值 |
| 整个 key 空间 | 一个哈希表（dict），渐进式扩容 | 16 个分片，每个分片一个 Go map |

Redis 的选择都是在省内存、对 CPU 缓存友好：一个元素一个节点的链表，指针本身就要占不少空间，而且节点散落在内存各处。Mini-Redis 是学习项目，选了最直观的结构，把重点放在并发、持久化和协议上。面试时被问到“为什么不像 Redis 那样做”，可以这样回答：先保证正确和清楚，性能瓶颈测出来了再优化。

---

## 关键设计与为什么

**1. 一个 `map[string]any` 存所有类型，取出时做类型断言。**
- 为什么：Redis 规定一个 key 只能有一种类型。只有一个 map，就从结构上保证了这一点，不会出现同一个 key 既在字符串表里、又在列表表里的情况。
- 代价：每次访问都要做一次类型断言；`[]byte` 存进 `any` 时还要多一层包装。和网络 IO 比，这点开销可以忽略。
- 放弃了什么：用两个 map 分别存字符串和列表，类型更明确，但每次写入都要检查另一个 map 里有没有同名的 key，容易漏。

**2. 接口由使用方定义，而且只有一个方法。**
- `CommandLog`、`Executor`、`publicError` 都是这样。使用方只要求自己用到的那一个方法，实现方不需要 import 使用方的包。
- 好处是包之间的依赖只有一个方向，测试时也能轻松换上假的实现：database 的测试用假日志，server 的测试用一个会故意卡住的假执行器。

**3. 状态回复用自定义类型 `StatusReply`，不用 `bool` 或 `string`。**
- `+OK` 和 `$2\r\nOK\r\n` 在协议里是两种东西：前者是状态，后者是二进制数据。如果 SET 返回普通的 `string("OK")`，编码器就得猜它是哪一种。
- 给它一个专门的类型，类型本身就说明了意思，编码器不用猜。

**4. 分片数必须是 2 的幂，用位运算选分片。**
- `hash & (n-1)` 比 `hash % n` 快，代价是分片数只能取 1、2、4、8、16……。`NewEngine` 在构造时就检查，不满足直接 panic（第 06 章讲为什么用 panic）。

---

## 动手实验

**实验 1：把 Exec 改成值接收者，看 go vet 怎么说。**

把 [database/engine.go](../../database/engine.go) 里的

```go
func (e *Engine) Exec(args [][]byte) (any, error) {
```

改成 `func (e Engine) Exec(...)`，运行 `go vet ./database`：

```text
database\engine.go:41:9: Exec passes lock by value: github.com/Li-Nepenthe/mini-redis/database.Engine contains sync.Mutex
```

Engine 里有 `logMu sync.Mutex`，值接收者会把整个 Engine 连同锁复制一份，每次调用锁的都是副本，等于没锁。编译器不会拦你，vet 会。做完改回来。

**实验 2：把各种 Go 类型交给 EncodeReply。**

在仓库里建 `tmpexp/enc/main.go`：

```go
package main

import (
	"fmt"

	"github.com/Li-Nepenthe/mini-redis/database"
	"github.com/Li-Nepenthe/mini-redis/resp"
)

func main() {
	for _, v := range []any{database.StatusReply("OK"), int64(3), []byte("v"), [][]byte{[]byte("a")}, nil, int(3), true, "OK"} {
		out, err := resp.EncodeReply(v, nil)
		fmt.Printf("%-22T %q %v\n", v, out, err)
	}
}
```

`go run ./tmpexp/enc` 的输出：

```text
database.StatusReply   "+OK\r\n" <nil>
int64                  ":3\r\n" <nil>
[]uint8                "$1\r\nv\r\n" <nil>
[][]uint8              "*1\r\n$1\r\na\r\n" <nil>
<nil>                  "$-1\r\n" <nil>
int                    "" unsupported reply type: int
bool                   "" unsupported reply type: bool
string                 "" unsupported reply type: string
```

前五种是引擎会返回的类型，各自对应一种 RESP 回复。后三种不在对应表里，`EncodeReply` 直接报错。连接循环遇到这种错误会记日志并断开连接：这说明引擎有 bug，不能把一个猜出来的回复发给客户端。

**实验 3：key 在 16 个分片上分布得均匀吗？**

在 database 目录下临时建一个 `zz_lab_test.go`（包内测试才能调用小写的 `fnv32`）：

```go
package database

import (
	"fmt"
	"hash/fnv"
	"strconv"
	"testing"
)

func TestLabShardSpread(t *testing.T) {
	e := NewEngine(16)
	counts := make([]int, 16)
	for i := 0; i < 100000; i++ {
		counts[fnv32("key:"+strconv.Itoa(i))&(e.shardCount-1)]++
	}
	fmt.Println(counts)
}

func TestLabFNVAllocs(t *testing.T) {
	key := "user:1001"
	hand := testing.AllocsPerRun(1000, func() { _ = fnv32(key) })
	std := testing.AllocsPerRun(1000, func() {
		h := fnv.New32()
		h.Write([]byte(key))
		_ = h.Sum32()
	})
	h := fnv.New32()
	h.Write([]byte(key))
	fmt.Println("allocs hand/std:", hand, std, "same:", h.Sum32() == fnv32(key))
}
```

运行 `go test ./database -run TestLab -v -count=1`，输出里的两行：

```text
[6281 6281 6255 6255 6288 6288 6308 6308 6217 6217 6245 6245 6214 6214 6192 6192]
allocs hand/std: 0 0 same: true
```

- 10 万个 key，每个分片 6200 个左右，最多和最少相差不到 2%，很均匀。
- 相邻两个分片的数字两两相同。原因在 FNV-1 的最后一步：直接把最后一个字节异或进哈希的低位。这些 key 只在最后一位数字上不同，而 0 和 1、2 和 3……这些数字两两只差最低一位，所以总是成对地落进相邻的两个分片。对均匀程度没有影响。
- 手写的 `fnv32` 和标准库 `hash/fnv` 结果相同，也都不分配内存。

做完删掉这个测试文件。

---

## 常见误区

- **依赖 map 的遍历顺序。** 每次运行都可能不同。需要顺序就排序。
- **以为 recover 能接住 map 并发写。** 那是 fatal error，进程直接退出。
- **以为 sync.Map 是 map 的“线程安全升级版”，到处替换。** 它只适合特定场景，复合操作还是要加锁。
- **函数返回一个值为 nil 的具体错误类型指针。** 调用者看到的 err 不等于 nil。
- **把嵌入当继承。** 被嵌入类型的方法不知道外层类型的存在，没有重写。
- **同一个类型混用值接收者和指针接收者。** 容易搞不清哪个能满足接口，也容易误改拷贝。

---

## 自测

**1. 对一个字符串 key 执行 LPUSH，引擎是在哪一步发现类型不对的？**

<details><summary>答案</summary>

`planLPush` 从分片的 `data` 里取出值，做类型断言 `raw.(*linkedList)`。值是 `[]byte`，ok 为 false，返回 `ErrTypeMismatch`，客户端收到 `WRONGTYPE`。这时还没有写日志，也没有修改内存。

</details>

**2. aof 包没有 import database，`*aof.Store` 怎么就成了 `database.CommandLog`？**

<details><summary>答案</summary>

Go 的接口是隐式实现的：一个类型只要有接口要求的全部方法，就自动实现了这个接口。`*aof.Store` 有 `Append(args [][]byte) error` 方法，所以可以赋给 `CommandLog`。两者在 `cmd/server` 的 `dbEngine.AttachLog(store)` 这一行接上。

</details>

**3. SET 为什么返回 `StatusReply("OK")`，而不是字符串 `"OK"`？**

<details><summary>答案</summary>

在 RESP 里，状态回复 `+OK` 和 bulk 字符串 `$2\r\nOK` 是两种类型。用一个专门的类型，编码器靠 type switch 就能认出它是状态回复；如果用普通的 string，编码器分不清它是状态还是数据。实际上 `EncodeReply` 对普通 string 会直接报错。

</details>

**4. 为什么 `NewEngine` 要求分片数是 2 的幂？**

<details><summary>答案</summary>

路由用的是 `hash & (shardCount-1)`。只有分片数是 2 的幂时，`shardCount-1` 的二进制才全是 1，按位与才等价于取余。分片数是 10 时，`hash & 9` 只会落到 0、1、8、9 四个分片上。

</details>

**5. Engine 的方法如果写成值接收者，会出什么问题？怎样发现？**

<details><summary>答案</summary>

每次调用都会复制整个 Engine，包括里面的 `sync.Mutex`。每次加锁的都是副本，起不到互斥的作用。编译能通过，但 `go vet` 的 copylocks 检查会报 `passes lock by value`。

</details>

**6. 一个函数声明返回 `error`，里面写了 `var p *MyErr; return p`，调用者的 `err != nil` 为什么是 true？**

<details><summary>答案</summary>

接口值由“动态类型”和“数据指针”两部分组成，两部分都是 nil 才等于 nil。`return p` 把类型 `*MyErr` 放进了接口，只有数据指针是 nil，所以接口不等于 nil。没有错误时要直接 `return nil`。

</details>

---

## 开发过程回放

以下按 git 历史整理。

- **2026-06-20，`94042f3`，第一版存储引擎。** 一开始就是分片的结构：`shard` 里有一把 `sync.RWMutex` 和一个 `map[string]any`，引擎持有多个分片。链表类型叫 `Node`、`LinkedList`，是大写导出的。
- **2026-10-01，`3d95ea3`，第一阶段完成。** 分片里加入了过期时间的两个结构（`expires` 和 `expiring`）；`CommandLog` 接口把引擎和 aof 包接起来；回复编码里，SET 返回 `true`，编码器把 `bool` 翻译成 `+OK`。
- **2026-11-10，PR #5，可读性改造。**
  - 回复类型统一：SET 返回 `StatusReply("OK")`，所有整数回复都用 `int64`，编码器去掉了 `bool` 和 `int` 两个分支。
  - `Node`、`LinkedList`、`NewLinkedList` 改成小写；`Engine.LPush`、`Engine.LPop` 两个导出方法没有任何调用者，删掉。
  - 网络层原来依赖一个 `tcp.Handler` 接口，加上 resp 包里一整套处理器；现在只依赖一个只有 `Exec` 方法的 `Executor` 接口。

---

## 核对来源（Go 1.27.1）

| 说法 | 出处 |
|---|---|
| Swiss Table 设计：组、控制字、H1/H2、二次探测、墓碑、表拆分、目录 | `internal/runtime/maps/map.go` 开头的设计注释 |
| 每组 8 个槽 | `internal/abi/map.go`：`MapGroupSlots = 1 << MapGroupSlotsBits // 8` |
| 每组平均最多 7 个 | `internal/runtime/maps/group.go`：`maxAvgGroupLoad = 7` |
| 单表最多 1024 个槽 | `internal/runtime/maps/table.go`：`maxTableCapacity = 1024` |
| 不超过 8 个元素只用一个组 | `internal/runtime/maps/map.go`：`if hint <= abi.MapGroupSlots` 及相关注释 |
| 遍历起点随机 | `internal/runtime/maps/table.go`：`it.entryOffset = rand()`、`it.dirOffset = rand()` |
| 并发写检测与 fatal | `internal/runtime/maps/map.go`：`fatal("concurrent map writes")`、`fatal("concurrent map read and map write")` |
| clear 不缩小 map | `internal/runtime/maps/map.go`：`Clear` 只清空各表，注释 “TODO: shrink directory?” |
| sync.Map 用 HashTrieMap 实现 | `sync/map.go`：`m isync.HashTrieMap[any, any]`；`internal/sync/hashtriemap.go` |
| eface、iface | `runtime/runtime2.go` |
| itab 的字段 | `internal/abi/iface.go`：`type ITab struct` |
| Go 1.24 改用 Swiss Table；Go 1.24 改写 sync.Map | Go 1.24 release notes |
| 旧实现：桶 8 个、负载因子 6.5、渐进式搬迁 | Go 1.23 的 `runtime/map.go`（本机没有 1.23 源码，定稿前由 GPT 核对） |
| `hash/fnv` 的 New32 是 FNV-1，与 `fnv32` 结果相同，二者都不分配 | `hash/fnv/fnv.go`：`sum32.Write` 先乘后异或；2026-11-10 实验 3 实测 |
| copylocks 的报错、EncodeReply 对各类型的输出、分片分布 | 2026-11-10 在 dbd2e9f 上实际运行 |
| 早期引擎结构、旧的 bool/int 回复分支 | `git show 94042f3:database/Engine.go`、`git show 31684b7:resp/reply.go` |
