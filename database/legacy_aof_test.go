// 本文件：TestLegacyAOFGolden 用固定的旧 AOF 经过 Open、解析和 Replay 恢复，再比较公开读取结果。

package database

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Li-Nepenthe/mini-redis/aof"
	"github.com/Li-Nepenthe/mini-redis/resp"
)

// TestLegacyAOFGolden 的输入是手工固定的 RESP 文件，不经过当前写路径生成。
// 复制到临时目录再打开，避免文件锁或恢复截断改动仓库里的兼容性样本。
func TestLegacyAOFGolden(t *testing.T) {
	fixture, err := os.ReadFile("testdata/legacy.aof")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "legacy.aof")
	if err := os.WriteFile(path, fixture, 0o600); err != nil {
		t.Fatal(err)
	}
	engine, elapsed := clockEngine()
	elapsed.Store(int64(10 * time.Second))
	store, err := aof.Open(context.Background(), path, engine.Replay)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	if store.RecoveredTailBytes != 0 {
		t.Fatalf("complete legacy AOF lost %d bytes", store.RecoveredTailBytes)
	}
	// Windows 文件锁也限制其他句柄读取；重放完成后先释放锁，再核对原始字节。
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, fixture) {
		t.Fatal("opening the complete legacy AOF changed its bytes")
	}

	var state strings.Builder
	for _, parts := range [][]string{
		{"GET", "binary"},
		{"GET", "persistent"},
		{"TTL", "persistent"},
		{"GET", "deleted"},
		{"GET", "zero"},
		{"LRANGE", "list", "0", "-1"},
		{"TTL", "list"},
		{"LRANGE", "drained", "0", "-1"},
		{"TTL", "drained"},
		{"LRANGE", "reborn-list", "0", "-1"},
		{"TTL", "reborn-list"},
		{"GET", "reborn-string"},
		{"TTL", "reborn-string"},
		{"GET", "expired"},
		{"TTL", "expired"},
		{"GET", "renewed"},
		{"TTL", "renewed"},
		{"LRANGE", "was-string", "0", "-1"},
		{"TTL", "was-string"},
		{"LRANGE", "legacy-list", "0", "-1"},
		{"TTL", "legacy-list"},
		{"EXISTS", "binary", "binary", "persistent", "list", "reborn-list", "reborn-string", "renewed", "was-string", "legacy-list"},
		{"EXISTS", "deleted", "zero", "drained", "expired", "missing"},
	} {
		result, execErr := engine.Exec(command(parts...))
		if execErr != nil {
			t.Fatalf("%q: %v", parts, execErr)
		}
		// 比较 RESP 回复，不绑定 int 或 int64 等内部表示，后续统一回复类型时基线仍有效。
		reply, err := resp.EncodeReply(result, nil)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&state, "%q => %q\n", parts, reply)
	}
	requireGolden(t, "testdata/legacy-state.golden", []byte(state.String()))
}
