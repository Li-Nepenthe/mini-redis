// 本文件：通过真实日志、受控文件故障和子进程崩溃验证恢复、独占锁及刷盘确认边界。

package aof

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/database"
	"github.com/Li-Nepenthe/mini-redis/resp"
)

func args(parts ...string) [][]byte {
	result := make([][]byte, len(parts))
	for i, part := range parts {
		result[i] = []byte(part)
	}
	return result
}

func execute(t *testing.T, engine *database.Engine, want any, parts ...string) {
	t.Helper()
	result, err := engine.Exec(args(parts...))
	if err != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("%q result=%v err=%v want=%v", parts, result, err, want)
	}
}

// 测试与服务启动使用同样的恢复和绑定次序；清理时保留关闭错误，避免把最后一次刷盘失败漏掉。
func openEngine(t *testing.T, path string) (*database.Engine, *Store) {
	t.Helper()
	engine := database.NewEngine(16)
	store, err := Open(context.Background(), path, engine.Replay)
	if err != nil {
		t.Fatal(err)
	}
	engine.AttachLog(store)
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return engine, store
}

// TestReplayStringListDeleteAndTTL 两次重开真实文件，确认字符串、列表和删除结果能恢复，TTL 也不会从重启时重新计时。
func TestReplayStringListDeleteAndTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	engine, store := openEngine(t, path)
	execute(t, engine, database.StatusReply("OK"), "SET", "binary", "a\x00\r\nb")
	execute(t, engine, database.StatusReply("OK"), "SET", "deleted", "v")
	execute(t, engine, int64(1), "DEL", "deleted")
	execute(t, engine, int64(3), "LPUSH", "list", "a", "b", "c")
	execute(t, engine, []byte("c"), "LPOP", "list")
	execute(t, engine, int64(1), "EXPIRE", "list", "2")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	restored, store2 := openEngine(t, path)
	execute(t, restored, []byte("a\x00\r\nb"), "GET", "binary")
	execute(t, restored, nil, "GET", "deleted")
	execute(t, restored, [][]byte{[]byte("b"), []byte("a")}, "LRANGE", "list", "0", "-1")
	result, err := restored.Exec(args("TTL", "list"))
	if err != nil || result.(int64) > 1 {
		t.Fatalf("restart extended TTL: result=%v err=%v", result, err)
	}
	if err := store2.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
	restored, _ = openEngine(t, path)
	execute(t, restored, int64(-2), "TTL", "list")
}

// TestExpiredListWithLaterPushDoesNotResurrectOnReopen 让列表在期限内追加、期限后重放，检查追加不会让旧列表丢掉 TTL 而复活。
func TestExpiredListWithLaterPushDoesNotResurrectOnReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	engine, store := openEngine(t, path)
	execute(t, engine, int64(1), "LPUSH", "list", "a")
	execute(t, engine, int64(1), "EXPIRE", "list", "1")
	execute(t, engine, int64(2), "LPUSH", "list", "b")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	execute(t, engine, int64(-2), "TTL", "list")
	restored, _ := openEngine(t, path)
	execute(t, restored, int64(-2), "TTL", "list")
	execute(t, restored, [][]byte{}, "LRANGE", "list", "0", "-1")
}

// TestMaximumSizedLPUSHFitsPersistedCreationRecord 把 LPUSH 填到协议上限，检查新建标记仍能落盘，不能靠放宽上限掩盖记录变大的问题。
func TestMaximumSizedLPUSHFitsPersistedCreationRecord(t *testing.T) {
	request := [][]byte{[]byte("LPUSH"), []byte("max"),
		make([]byte, resp.MaxBulkLength), make([]byte, resp.MaxBulkLength-128)}
	initial, err := resp.AppendRequest(nil, request)
	if err != nil {
		t.Fatal(err)
	}
	request[3] = append(request[3], make([]byte, resp.MaxRequestLength-len(initial))...)
	wire, err := resp.AppendRequest(nil, request)
	if err != nil || len(wire) != resp.MaxRequestLength {
		t.Fatalf("boundary fixture length=%d err=%v", len(wire), err)
	}
	engine := database.NewEngine(16)
	store := &Store{file: &faultFile{}}
	engine.AttachLog(store)
	result, err := engine.Exec(request)
	if err != nil || result != int64(2) || store.size != resp.MaxRequestLength {
		t.Fatalf("maximum LPUSH result=%v err=%v persisted=%d", result, err, store.size)
	}
}

// TestRecoverOnlyIncompleteTailWithActualOffsets 用带前导零的数字头区分真实偏移和重新编码后的长度，避免恢复时截错位置。
func TestRecoverOnlyIncompleteTailWithActualOffsets(t *testing.T) {
	for _, tail := range []string{"*3\r\n$3\r\nSET\r\n$4\r\ntail\r\n$5\r\nabc", "*1", "*1\r\n$4"} {
		path := filepath.Join(t.TempDir(), "appendonly.aof")
		prefix := "*03\r\n$03\r\nSET\r\n$01\r\nk\r\n$01\r\nv\r\n"
		if err := os.WriteFile(path, []byte(prefix+tail), 0o600); err != nil {
			t.Fatal(err)
		}
		engine, store := openEngine(t, path)
		execute(t, engine, []byte("v"), "GET", "k")
		if store.RecoveredTailBytes != int64(len(tail)) {
			t.Fatalf("recovered bytes=%d want=%d", store.RecoveredTailBytes, len(tail))
		}
		info, err := os.Stat(path)
		if err != nil || info.Size() != int64(len(prefix)) {
			t.Fatalf("recovered file size=%v err=%v", info, err)
		}
		execute(t, engine, database.StatusReply("OK"), "SET", "next", "ok")
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		restored, _ := openEngine(t, path)
		execute(t, restored, []byte("v"), "GET", "k")
		execute(t, restored, []byte("ok"), "GET", "next")
	}
}

// 完整坏记录不能当作残尾丢弃；协议错误和命令语义错误都必须保留原文件。
func TestRejectCorruptOrInvalidCompleteRecordsWithoutTruncating(t *testing.T) {
	for _, data := range []string{"*1\r\n$3\r\nGETXY", "*1\r\n$4\r\nPING\r\n"} {
		path := filepath.Join(t.TempDir(), "appendonly.aof")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		engine := database.NewEngine(16)
		if store, err := Open(context.Background(), path, engine.Replay); err == nil {
			_ = store.Close()
			t.Fatal("corrupt complete record was accepted")
		}
		current, err := os.ReadFile(path)
		if err != nil || string(current) != data {
			t.Fatal("corrupt complete file was silently truncated")
		}
	}
}

// 验证真实文件锁在同时打开时排他、关闭后释放，防止改用锁文件后遗留陈旧状态。
func TestAOFExclusiveLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	_, store := openEngine(t, path)
	if second, err := Open(context.Background(), path, database.NewEngine(16).Replay); err == nil {
		_ = second.Close()
		t.Fatal("two stores acquired the same AOF")
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	_, _ = openEngine(t, path)
}

// 并发写完再重开同一日志，对比恢复状态与内存状态，检查日志顺序和应用顺序是否一致。
func TestConcurrentWriteOrderMatchesReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	engine, store := openEngine(t, path)
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			for i := 0; i < 20; i++ {
				value := fmt.Sprintf("%d:%d", worker, i)
				for _, request := range [][][]byte{args("SET", "shared", value), args("LPUSH", "list", value)} {
					if _, err := engine.Exec(request); err != nil {
						t.Error(err)
					}
				}
			}
		}(worker)
	}
	workers.Wait()
	last, _ := engine.Exec(args("GET", "shared"))
	list, _ := engine.Exec(args("LRANGE", "list", "0", "-1"))
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restored, _ := openEngine(t, path)
	execute(t, restored, last, "GET", "shared")
	execute(t, restored, list, "LRANGE", "list", "0", "-1")
}

type faultFile struct {
	writeErr, syncErr error
	writes, syncs     int
	truncated         int64
	closed            bool
}

// 故意返回部分写入与错误，检查 Store 是否回退到最后确认的位置。
func (f *faultFile) Write(data []byte) (int, error) {
	f.writes++
	if f.writeErr != nil {
		return len(data) / 2, f.writeErr
	}
	return len(data), nil
}

func (f *faultFile) Sync() error { f.syncs++; return f.syncErr }

func (f *faultFile) Truncate(n int64) error { f.truncated = n; return nil }

func (f *faultFile) Close() error { f.closed = true; return nil }

// 写入或同步失败后，后续追加仍应拒绝；关闭也不能抹掉最初的故障原因。
func TestWriteAndSyncFailuresAreSticky(t *testing.T) {
	cause := errors.New("simulated storage failure")
	for _, file := range []*faultFile{{writeErr: cause}, {syncErr: cause}} {
		store := &Store{file: file, size: 100}
		if err := store.Append(args("SET", "k", "v")); !errors.Is(err, cause) {
			t.Fatalf("append failure=%v", err)
		}
		writes := file.writes
		if err := store.Append(args("SET", "k", "next")); !errors.Is(err, cause) || file.writes != writes {
			t.Fatal("store continued after a storage failure")
		}
		if file.truncated != 100 || store.size != 100 {
			t.Fatal("failure did not roll back to durable prefix")
		}
		if err := store.Close(); !errors.Is(err, cause) || !file.closed {
			t.Fatal("Close hid the storage error or leaked file")
		}
	}
}

// 助手确认一千条写入后保持运行，由父测试直接终止；不经过正常关闭才能检验逐条刷盘的承诺。
func TestCrashProcessHelper(t *testing.T) {
	if os.Getenv("MINIREDIS_AOF_CHILD") != "1" {
		t.Skip("used only by owned crash-test subprocess")
	}
	engine := database.NewEngine(16)
	store, err := Open(context.Background(), os.Getenv("MINIREDIS_AOF_PATH"), engine.Replay)
	if err != nil {
		t.Fatal(err)
	}
	engine.AttachLog(store)
	for i := 0; i < 1000; i++ {
		if _, err := engine.Exec(args("SET", "key-"+strconv.Itoa(i), "v")); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println("READY")
	for {
		time.Sleep(time.Second)
	}
}

// 只终止本测试创建的助手，再恢复它的日志，检查已经确认的数据与系统锁能否保留和释放。
// 这里验证进程崩溃，不把结果当成硬件断电保证。
func TestThousandWritesSurviveForcedProcessTermination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	child := exec.Command(os.Args[0], "-test.run=^TestCrashProcessHelper$")
	child.Env = append(os.Environ(), "MINIREDIS_AOF_CHILD=1", "MINIREDIS_AOF_PATH="+path)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	child.Stderr = &stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	var waited atomic.Bool
	t.Cleanup(func() {
		if !waited.Load() {
			_ = child.Process.Kill()
			_ = child.Wait()
		}
	})
	ready := make(chan error, 1)
	// 独立读取 READY 解除 stdout 等待；主测试控制强杀与 Wait，清理只指向自己创建的 child。
	go func() {
		line, err := bufio.NewReader(stdout).ReadString('\n')
		if err == nil && line != "READY\n" && line != "READY\r\n" {
			err = fmt.Errorf("unexpected child output %q", line)
		}
		ready <- err
	}()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("child did not acknowledge 1000 writes")
	}
	// 直接结束助手，不给它执行正常关闭或额外刷盘的机会。
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	waited.Store(true)
	engine, _ := openEngine(t, path)
	for i := 0; i < 1000; i++ {
		execute(t, engine, []byte("v"), "GET", "key-"+strconv.Itoa(i))
	}
	t.Log("1000 acknowledged writes recovered after forced termination without Close; OS file lock released")
}
