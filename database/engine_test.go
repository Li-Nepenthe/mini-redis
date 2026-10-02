package database

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func command(parts ...string) [][]byte {
	args := make([][]byte, len(parts))
	for i := range parts {
		args[i] = []byte(parts[i])
	}
	return args
}

func requireExec(t *testing.T, e *Engine, want any, parts ...string) {
	t.Helper()
	got, err := e.Exec(command(parts...))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%q: got=%#v err=%v want=%#v", parts, got, err, want)
	}
}

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
