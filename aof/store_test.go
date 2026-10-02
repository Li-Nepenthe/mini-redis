package aof

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/Li-Nepenthe/mini-redis/database"
	"github.com/Li-Nepenthe/mini-redis/resp"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// args 将 parts 的每个字符串转为独立字节参数并返回二维切片，便于测试直接调用 Engine/Store。
// 只构造输入，不进行 RESP 编码或修改引擎；可变字节应由被测实现按所有权契约处理。
func args(parts ...string) [][]byte {
	result := make([][]byte, len(parts))
	for i, part := range parts {
		result[i] = []byte(part)
	}
	return result
}

// execute 用 parts 执行 engine 命令，并由 t 检查无错误且结果与 want 深度相等，无返回值。
// 失败通过 Fatalf 停止当前测试；命令本身可修改内存和 AOF，Helper 让失败位置指向调用者。
func execute(t *testing.T, engine *database.Engine, want any, parts ...string) {
	t.Helper()
	result, err := engine.Exec(args(parts...))
	if err != nil || !reflect.DeepEqual(result, want) {
		t.Fatalf("%q result=%v err=%v want=%v", parts, result, err, want)
	}
}

// openEngine 为 path 创建 16 分片引擎、通过 Replay 恢复并绑定 Store，返回引擎与日志。
// 打开失败由 t.Fatal 停止；注册 t.Cleanup 关闭文件并记录关闭错误，测试可显式提前 Close 后再打开同一路径。
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

// TestReplayStringListDeleteAndTTL 验证 String 二进制值、DEL、List 顺序/弹出和绝对 TTL 在真实临时 AOF 重开后恢复。
// 由 t 记录断言，无返回值；短暂等待后确认 TTL 不从重启重新计时，最后过期不可见，文件由测试清理。
func TestReplayStringListDeleteAndTTL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	engine, store := openEngine(t, path)
	execute(t, engine, true, "SET", "binary", "a\x00\r\nb")
	execute(t, engine, true, "SET", "deleted", "v")
	execute(t, engine, 1, "DEL", "deleted")
	execute(t, engine, 3, "LPUSH", "list", "a", "b", "c")
	execute(t, engine, []byte("c"), "LPOP", "list")
	execute(t, engine, 1, "EXPIRE", "list", "2")
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

// TestExpiredListWithLaterPushDoesNotResurrectOnReopen 验证期限内追加的 List 在整体过期后重开 AOF 不会丢 TTL 而复活。
// 真实临时文件与 1.1 秒等待复现历史期限错误；t 比较 TTL=-2 和空 LRANGE，不测试过期后的重新创建分支。
func TestExpiredListWithLaterPushDoesNotResurrectOnReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	engine, store := openEngine(t, path)
	execute(t, engine, 1, "LPUSH", "list", "a")
	execute(t, engine, 1, "EXPIRE", "list", "1")
	execute(t, engine, 2, "LPUSH", "list", "b")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	execute(t, engine, int64(-2), "TTL", "list")
	restored, _ := openEngine(t, path)
	execute(t, restored, int64(-2), "TTL", "list")
	execute(t, restored, [][]byte{}, "LRANGE", "list", "0", "-1")
}

// TestMaximumSizedLPUSHFitsPersistedCreationRecord 构造恰好 32MiB 的合法 LPUSH，验证新建 _LNEW 落盘不扩大帧预算。
// 注入内存 faultFile 而非真实大文件，t 检查返回长度与确认 size，避免仅靠改宽解析上限掩盖持久化编码问题。
func TestMaximumSizedLPUSHFitsPersistedCreationRecord(t *testing.T) {
	request := [][]byte{[]byte("LPUSH"), []byte("max"),
		make([]byte, resp.MaxBulkLength), make([]byte, resp.MaxBulkLength-128)}
	initial, err := encodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	request[3] = append(request[3], make([]byte, resp.MaxRequestLength-len(initial))...)
	wire, err := encodeRequest(request)
	if err != nil || len(wire) != resp.MaxRequestLength {
		t.Fatalf("boundary fixture length=%d err=%v", len(wire), err)
	}
	engine := database.NewEngine(16)
	store := &Store{file: &faultFile{}}
	engine.AttachLog(store)
	result, err := engine.Exec(request)
	if err != nil || result != 2 || store.size != resp.MaxRequestLength {
		t.Fatalf("maximum LPUSH result=%v err=%v persisted=%d", result, err, store.size)
	}
}

// TestRecoverOnlyIncompleteTailWithActualOffsets 验证非规范数字头的完整前缀按真实字节保留，仅截断各种不完整尾部。
// t 在临时文件检查恢复字节数、文件长度及后续追加再重放的数据，不用重新编码长度冒充原偏移。
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
		execute(t, engine, true, "SET", "next", "ok")
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		restored, _ := openEngine(t, path)
		execute(t, restored, []byte("v"), "GET", "k")
		execute(t, restored, []byte("ok"), "GET", "next")
	}
}

// TestRejectCorruptOrInvalidCompleteRecordsWithoutTruncating 验证完整非法 RESP/非持久化命令使 Open 失败且原文件字节不被截断。
// t 使用临时文件分别覆盖协议错误与 Replay 错误；无返回值，失败不能被解释成可自动修尾。
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

// TestAOFExclusiveLock 验证同一路径同时只能打开一个 Store，Close 后锁释放可以重新打开。
// t 使用临时文件和当前平台实现，无返回值；这是操作系统文件锁检查，不是靠陈旧锁文件模拟。
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

// TestConcurrentWriteOrderMatchesReplay 让 8 个 worker 并发写共享 String/List，比较内存最终值与真实 AOF 重开后的状态。
// t 检查日志顺序和 apply 顺序一致，等待全部 worker 后才关闭文件；不声称任意调度的各客户端回复顺序完全相同。
func TestConcurrentWriteOrderMatchesReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "appendonly.aof")
	engine, store := openEngine(t, path)
	var workers sync.WaitGroup
	// worker 值作为参数传入，所有写结束后再比较/关闭，避免把未完成操作混进恢复断言。
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

// Write 记录调用次数，正常返回 len(data)、nil；设置 writeErr 时返回半段长度与注入错误。
// 不实际存储 data，专门模拟部分写失败供 Store.fail 测试，无并发保护，测试通过 Store 锁串行使用。
func (f *faultFile) Write(data []byte) (int, error) {
	f.writes++
	if f.writeErr != nil {
		return len(data) / 2, f.writeErr
	}
	return len(data), nil
}

// Sync 增加模拟同步次数并返回配置的 syncErr，供测试观察刷盘失败及回滚再次同步。
// 不执行真实文件 I/O；错误保持不变，不能把该模拟器的 nil 当作真实磁盘确认。
func (f *faultFile) Sync() error { f.syncs++; return f.syncErr }

// Truncate 记录请求的 n 为 truncated 并返回 nil，用于验证回滚目标是原确认前缀。
// 不调整实际文件或写游标，测试只检查请求行为；无独立并发保护。
func (f *faultFile) Truncate(n int64) error { f.truncated = n; return nil }

// Close 将模拟器的 closed 标记为 true 并返回 nil，供断言失败状态也必须回收句柄。
// 不涉及真实资源，重复调用只保持标记，历史错误的保留由 Store 负责。
func (f *faultFile) Close() error { f.closed = true; return nil }

// TestWriteAndSyncFailuresAreSticky 分别注入部分写和同步失败，验证回滚到 size、后续拒写及 Close 保留原 cause。
// t 同时检查调用次数、确认偏移与关闭标记；使用 faultFile，不把尽力回滚当作所有硬件都能成功恢复的证明。
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

// TestCrashProcessHelper 仅在 MINIREDIS_AOF_CHILD=1 的自有子进程中写 1000 条并输出 READY，然后保持运行等待父测试强杀。
// 普通 go test 会 Skip；从专用环境路径打开 AOF，无主动 Close/flush 收尾，用于验证每条 Append 已同步确认。
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

// TestThousandWritesSurviveForcedProcessTermination 启动本测试二进制的专用助手，等待 READY 后只强杀该子进程。
// t 重开临时 AOF 验证 1000 条确认写完整恢复及系统锁释放；注册清理防泄漏，不向其他进程发信号，也不是断电模拟。
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
	// Kill only the subprocess this test created. It does not Close/flush AOF.
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
