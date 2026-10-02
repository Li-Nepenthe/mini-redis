package aof

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/Li-Nepenthe/mini-redis/database"
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

func (f *faultFile) Write(data []byte) (int, error) {
	f.writes++
	if f.writeErr != nil {
		return len(data) / 2, f.writeErr
	}
	return len(data), nil
}
func (f *faultFile) Sync() error            { f.syncs++; return f.syncErr }
func (f *faultFile) Truncate(n int64) error { f.truncated = n; return nil }
func (f *faultFile) Close() error           { f.closed = true; return nil }

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
