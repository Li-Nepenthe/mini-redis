package database

import (
	"errors"
	"reflect"
	"testing"
	"time"
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
	for _, args := range [][]string{{"__EXPIREATMS", "string", "0"}, {"_LNEW", "string", "v"}} {
		if _, err := e.Exec(command(args...)); !errors.Is(err, ErrUnknownCmd) {
			t.Fatal("private replay command exposed to clients")
		}
	}
}

type recordingLog struct{ records [][][]byte }

func (l *recordingLog) Append(args [][]byte) error {
	record := make([][]byte, len(args))
	for i, arg := range args {
		record[i] = append([]byte(nil), arg...)
	}
	l.records = append(l.records, record)
	return nil
}

func TestReplayPreservesHistoricalExpiryAndCreation(t *testing.T) {
	for _, name := range []string{"expired-list", "renewed-string", "recreated-list", "expired-string-to-list", "cleaned-list"} {
		t.Run(name, func(t *testing.T) {
			e, elapsed := clockEngine()
			log := &recordingLog{}
			e.AttachLog(log)
			if name == "renewed-string" || name == "expired-string-to-list" {
				requireExec(t, e, true, "SET", "k", "old")
			} else {
				requireExec(t, e, 1, "LPUSH", "k", "old")
			}
			requireExec(t, e, 1, "EXPIRE", "k", "5")
			switch name {
			case "expired-list":
				elapsed.Store(int64(time.Second))
				requireExec(t, e, 2, "LPUSH", "k", "new")
			case "renewed-string":
				elapsed.Store(int64(4 * time.Second))
				requireExec(t, e, 1, "EXPIRE", "k", "10")
			default:
				elapsed.Store(int64(6 * time.Second))
				if name == "cleaned-list" {
					// 清理删除没有持久化记录；新建标记也必须覆盖这个路径。
					requireExec(t, e, int64(-2), "TTL", "k")
				}
				requireExec(t, e, 1, "LPUSH", "k", "new")
			}
			elapsed.Store(int64(6 * time.Second))
			restored, restoredElapsed := clockEngine()
			restoredElapsed.Store(elapsed.Load())
			otherLog := &recordingLog{}
			restored.AttachLog(otherLog)
			for _, record := range log.records {
				if err := restored.Replay(record); err != nil {
					t.Fatalf("replay %q: %v", record, err)
				}
			}
			if len(otherLog.records) != 0 {
				t.Fatal("replay appended records again")
			}
			for _, request := range [][]string{{"TTL", "k"}, {"LRANGE", "k", "0", "-1"}, {"GET", "k"}} {
				want, wantErr := e.Exec(command(request...))
				got, gotErr := restored.Exec(command(request...))
				if !reflect.DeepEqual(got, want) || !errors.Is(gotErr, wantErr) {
					t.Fatalf("%q restored=%v/%v current=%v/%v", request, got, gotErr, want, wantErr)
				}
			}
		})
	}
}

func TestLegacyReplayKeepsExpiryUntilFinalState(t *testing.T) {
	e, elapsed := clockEngine()
	elapsed.Store(int64(6 * time.Second))
	for _, request := range [][]string{
		{"SET", "k", "v"},
		{"__EXPIREATMS", "k", "1790899205000"},
		{"__EXPIREATMS", "k", "1790899214000"},
	} {
		if err := e.Replay(command(request...)); err != nil {
			t.Fatal(err)
		}
	}
	requireExec(t, e, []byte("v"), "GET", "k")
	requireExec(t, e, int64(8), "TTL", "k")
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
