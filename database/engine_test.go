package database

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// command 将 parts 文本转为 Engine 所需的 [][]byte 并返回，供单元测试快速构造命令。
// 不检查参数合法性、不编码协议、不修改引擎；错误路径可以故意传空或非法参数。
func command(parts ...string) [][]byte {
	args := make([][]byte, len(parts))
	for i := range parts {
		args[i] = []byte(parts[i])
	}
	return args
}

// requireExec 用 parts 执行 e，要求无错误且结果与 want 深度相等，由 t 记录失败，无返回值。
// 设置 Helper 将失败定位到测试调用处；被执行命令可修改 Engine，但本辅助函数不额外操作状态。
func requireExec(t *testing.T, e *Engine, want any, parts ...string) {
	t.Helper()
	got, err := e.Exec(command(parts...))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%q: got=%#v err=%v want=%#v", parts, got, err, want)
	}
}

// TestCommandSequence 按顺序验证大小写命令、二进制 PING、String、List、重复 EXISTS/DEL 与缺失值返回类型。
// t 比较业务结果而非 RESP 字节；List 最后一个元素弹出后 key 消失，编码另由 resp/tcp 测试覆盖。
func TestCommandSequence(t *testing.T) {
	e := NewEngine(16)
	for _, test := range []struct {
		args []string
		want any
	}{
		{[]string{"ping"}, StatusReply("PONG")},
		{[]string{"PiNg", "hello\x00\r\n"}, []byte("hello\x00\r\n")},
		{[]string{"get", "missing"}, nil},
		{[]string{"set", "foo", "bar"}, true},
		{[]string{"GeT", "foo"}, []byte("bar")},
		{[]string{"exists", "foo", "foo", "missing"}, 2},
		{[]string{"lpush", "mylist", "a", "b", "c"}, 3},
		{[]string{"lrange", "mylist", "0", "-1"}, [][]byte{[]byte("c"), []byte("b"), []byte("a")}},
		{[]string{"lpop", "mylist"}, []byte("c")},
		{[]string{"lpop", "mylist"}, []byte("b")},
		{[]string{"lpop", "mylist"}, []byte("a")},
		{[]string{"exists", "mylist"}, 0},
		{[]string{"lpop", "mylist"}, nil},
		{[]string{"lrange", "missing", "0", "-1"}, [][]byte{}},
		{[]string{"del", "foo", "foo", "missing"}, 1},
		{[]string{"exists", "foo"}, 0},
	} {
		requireExec(t, e, test.want, test.args...)
	}
}

// TestCommandArgumentAndTypeErrors 表驱动验证未知命令、数量、整数溢出与 String/List 类型冲突。
// t 使用 errors.Is 比较错误身份，避免仅比较错误文字；测试初始设置两种类型后执行非法命令，无独立网络 I/O。
func TestCommandArgumentAndTypeErrors(t *testing.T) {
	e := NewEngine(16)
	requireExec(t, e, true, "SET", "string", "v")
	requireExec(t, e, 1, "LPUSH", "list", "v")
	for _, test := range []struct {
		args []string
		want error
	}{
		{nil, ErrUnknownCmd},
		{[]string{"unknown\r\ncommand"}, ErrUnknownCmd},
		{[]string{"SET", "k"}, ErrWrongArgsNum},
		{[]string{"GET"}, ErrWrongArgsNum},
		{[]string{"PING", "one", "two"}, ErrWrongArgsNum},
		{[]string{"LPUSH", "k"}, ErrWrongArgsNum},
		{[]string{"LPOP", "k", "extra"}, ErrWrongArgsNum},
		{[]string{"DEL"}, ErrWrongArgsNum},
		{[]string{"EXISTS"}, ErrWrongArgsNum},
		{[]string{"LRANGE", "k", "0"}, ErrWrongArgsNum},
		{[]string{"LRANGE", "k", "bad", "-1"}, ErrInvalidInteger},
		{[]string{"LRANGE", "k", "0", "9223372036854775808"}, ErrInvalidInteger},
		{[]string{"GET", "list"}, ErrTypeMismatch},
		{[]string{"LPUSH", "string", "v"}, ErrTypeMismatch},
		{[]string{"LPOP", "string"}, ErrTypeMismatch},
		{[]string{"LRANGE", "string", "0", "-1"}, ErrTypeMismatch},
	} {
		_, err := e.Exec(command(test.args...))
		if !errors.Is(err, test.want) {
			t.Fatalf("%q: error=%v want=%v", test.args, err, test.want)
		}
	}
}

// TestListRangeBoundaries 验证 LRANGE 的闭区间、负索引、越界裁剪、反向/空区间与 int64 极端索引。
// t 对固定三元素 List 比较返回的二维字节数组，无返回值；未在此测大链表性能。
func TestListRangeBoundaries(t *testing.T) {
	e := NewEngine(16)
	requireExec(t, e, 3, "LPUSH", "list", "a", "b", "c")
	for _, test := range []struct {
		start, stop string
		want        [][]byte
	}{
		{"0", "0", [][]byte{[]byte("c")}},
		{"1", "2", [][]byte{[]byte("b"), []byte("a")}},
		{"-2", "-1", [][]byte{[]byte("b"), []byte("a")}},
		{"-100", "100", [][]byte{[]byte("c"), []byte("b"), []byte("a")}},
		{"-9223372036854775808", "9223372036854775807", [][]byte{[]byte("c"), []byte("b"), []byte("a")}},
		{"3", "10", [][]byte{}},
		{"2", "1", [][]byte{}},
		{"0", "-4", [][]byte{}},
	} {
		requireExec(t, e, test.want, "LRANGE", "list", test.start, test.stop)
	}
}

// TestEngineOwnsStoredAndReturnedBytes 在 SET/LPUSH 后修改原输入，再修改 GET/LRANGE 返回字节，验证存储不被别名污染。
// t 随后再次读/弹出确认原值；测试调用结束后的所有权，不允许调用期间并发修改输入。
func TestEngineOwnsStoredAndReturnedBytes(t *testing.T) {
	e := NewEngine(16)
	value := []byte("original")
	if _, err := e.Exec([][]byte{[]byte("SET"), []byte("k"), value}); err != nil {
		t.Fatal(err)
	}
	value[0] = 'X'
	requireExec(t, e, []byte("original"), "GET", "k")
	result, _ := e.Exec(command("GET", "k"))
	result.([]byte)[0] = 'X'
	requireExec(t, e, []byte("original"), "GET", "k")
	listValue := []byte("list")
	if _, err := e.Exec([][]byte{[]byte("LPUSH"), []byte("list"), listValue}); err != nil {
		t.Fatal(err)
	}
	listValue[0] = 'X'
	result, _ = e.Exec(command("LRANGE", "list", "0", "-1"))
	result.([][]byte)[0][0] = 'X'
	requireExec(t, e, []byte("list"), "LPOP", "list")
}

// TestConcurrentListAndMultiKeyCommands 让 8 个 worker 同时 SET、EXISTS、反序 DEL 和共享 List LPUSH，检查 800 个元素不丢失。
// t 等待所有 worker；可配合 race 检测共享访问，有限运行通过不等于证明所有时序永无死锁。
func TestConcurrentListAndMultiKeyCommands(t *testing.T) {
	e := NewEngine(16)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for i := 0; i < 100; i++ {
				key := fmt.Sprintf("key-%d-%d", worker, i)
				for _, args := range [][][]byte{
					command("SET", key, "v"), command("EXISTS", key, "other"),
					command("DEL", "other", key), command("LPUSH", "shared-list", key),
				} {
					if _, err := e.Exec(args); err != nil {
						t.Error(err)
					}
				}
			}
		}(worker)
	}
	workers.Wait()
	result, err := e.Exec(command("LRANGE", "shared-list", "0", "-1"))
	if err != nil || len(result.([][]byte)) != 800 {
		t.Fatalf("concurrent pushes lost data: result=%v err=%v", result, err)
	}
}
