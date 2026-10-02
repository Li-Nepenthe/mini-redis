package database

import (
	"errors"
	"reflect"
	"testing"
)

type failingLog struct {
	err   error
	calls int
}

func (l *failingLog) Append([][]byte) error { l.calls++; return l.err }

func TestPersistenceFailureDoesNotMutateMemory(t *testing.T) {
	e := NewEngine(16)
	requireExec(t, e, true, "SET", "s", "old")
	requireExec(t, e, 2, "LPUSH", "list", "a", "b")
	cause := errors.New("simulated disk failure")
	log := &failingLog{err: cause}
	e.AttachLog(log)
	for _, args := range [][]string{{"SET", "s", "new"}, {"DEL", "s"}, {"LPUSH", "list", "new"}, {"LPOP", "list"}, {"EXPIRE", "s", "10"}, {"EXPIRE", "s", "0"}} {
		_, err := e.Exec(command(args...))
		if !errors.Is(err, cause) {
			t.Fatalf("%q: error=%v", args, err)
		}
		requireExec(t, e, []byte("old"), "GET", "s")
		requireExec(t, e, int64(-1), "TTL", "s")
		requireExec(t, e, [][]byte{[]byte("b"), []byte("a")}, "LRANGE", "list", "0", "-1")
	}
	if log.calls != 6 {
		t.Fatal("write failure was silently ignored")
	}
}

func TestInvalidAndNoOpWritesDoNotAppend(t *testing.T) {
	e := NewEngine(16)
	requireExec(t, e, true, "SET", "string", "v")
	log := &failingLog{}
	e.AttachLog(log)
	for _, args := range [][]string{{"SET", "k"}, {"LPUSH", "string", "v"}, {"EXPIRE", "missing", "10"}, {"LPOP", "missing"}, {"DEL", "missing"}} {
		_, _ = e.Exec(command(args...))
	}
	if log.calls != 0 {
		t.Fatal("invalid or no-op command entered AOF")
	}
	if err := e.Replay(command("PING")); err == nil {
		t.Fatal("AOF accepted a read/status command")
	}
	if _, err := e.Exec(command("__EXPIREATMS", "string", "0")); !errors.Is(err, ErrUnknownCmd) {
		t.Fatal("private replay command exposed to clients")
	}
}

func TestReplayExpirationUsesAbsoluteDeadline(t *testing.T) {
	e, _ := clockEngine()
	if err := e.Replay(command("SET", "k", "v")); err != nil {
		t.Fatal(err)
	}
	if err := e.Replay(command("__EXPIREATMS", "k", "1790899205000")); err != nil {
		t.Fatal(err)
	}
	result, err := e.Exec(command("TTL", "k"))
	if err != nil || !reflect.DeepEqual(result, int64(5)) {
		t.Fatalf("absolute expiry TTL=%v err=%v", result, err)
	}
}
