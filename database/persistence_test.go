// 本文件：用假日志观察写入失败和记录顺序，验证日志确认前内存不变，以及 Replay 对历史期限和新建边界的处理。

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

// Append 增加 calls 并返回预先配置的 err，不存储参数或真实写盘。
// 用于观察 Engine 是否尝试持久化及失败时禁止 apply；无同步保护，由引擎 logMu 串行调用。
func (l *failingLog) Append([][]byte) error { l.calls++; return l.err }

// TestPersistenceFailureDoesNotMutateMemory 让六种写入都遇到日志失败，再读取字符串、列表和 TTL。
// 这样能发现计划阶段提前修改内存的问题，不能只凭返回了错误就认为写入已经撤销。
func TestPersistenceFailureDoesNotMutateMemory(t *testing.T) {
	e := NewEngine(16)
	requireExec(t, e, StatusReply("OK"), "SET", "s", "old")
	requireExec(t, e, int64(2), "LPUSH", "list", "a", "b")
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

// TestInvalidAndNoOpWritesDoNotAppend 检查无效写入不会污染日志，同时把客户端命令与私有恢复标记隔开。
func TestInvalidAndNoOpWritesDoNotAppend(t *testing.T) {
	e := NewEngine(16)
	requireExec(t, e, StatusReply("OK"), "SET", "string", "v")
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

// Append 深拷贝 args 的每个参数后加入 records 并返回 nil，记录逻辑持久化内容供回放对照。
// 不真实刷盘；深拷贝防止后续参数复用改写历史，测试在引擎写序列或 logMu 下使用。
func (l *recordingLog) Append(args [][]byte) error {
	record := make([][]byte, len(args))
	for i, arg := range args {
		record[i] = append([]byte(nil), arg...)
	}
	l.records = append(l.records, record)
	return nil
}

// TestReplayPreservesHistoricalExpiryAndCreation 在相同时间读取运行中的引擎与恢复后的引擎。
// 对比能发现重放时过早删除旧值、丢失新建边界或再次追加日志的问题。
func TestReplayPreservesHistoricalExpiryAndCreation(t *testing.T) {
	for _, name := range []string{"expired-list", "renewed-string", "recreated-list", "expired-string-to-list", "cleaned-list"} {
		t.Run(name, func(t *testing.T) {
			e, elapsed := clockEngine()
			log := &recordingLog{}
			e.AttachLog(log)
			if name == "renewed-string" || name == "expired-string-to-list" {
				requireExec(t, e, StatusReply("OK"), "SET", "k", "old")
			} else {
				requireExec(t, e, int64(1), "LPUSH", "k", "old")
			}
			requireExec(t, e, int64(1), "EXPIRE", "k", "5")
			switch name {
			case "expired-list":
				elapsed.Store(int64(time.Second))
				requireExec(t, e, int64(2), "LPUSH", "k", "new")
			case "renewed-string":
				elapsed.Store(int64(4 * time.Second))
				requireExec(t, e, int64(1), "EXPIRE", "k", "10")
			default:
				elapsed.Store(int64(6 * time.Second))
				if name == "cleaned-list" {
					// 清理删除没有持久化记录；新建标记也必须覆盖这个路径。
					requireExec(t, e, int64(-2), "TTL", "k")
				}
				requireExec(t, e, int64(1), "LPUSH", "k", "new")
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

// TestLegacyReplayKeepsExpiryUntilFinalState 在重启时刻 6 秒顺序回放期限 5 与后续期限 14，验证 String 仍为 v、TTL 为 8。
// t 防止恢复时按当前时间提前删历史值；不声称没有新建边界的所有旧草稿 AOF 都可无损恢复。
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

// TestReplayExpirationUsesAbsoluteDeadline 用固定基准时钟回放 SET 和 __EXPIREATMS，检查剩余 TTL 精确为 5。
// t 检查绝对毫秒期限解释，不进行真实睡眠/文件操作；重启不能把原期限变成新的相对生命周期。
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
