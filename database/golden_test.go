// 本文件：TestAOFRecordsGolden 固定写命令交给日志的参数序列；requireGolden 只比较已有基线，不更新文件。

package database

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestAOFRecordsGolden 在固定时钟下记录 Append 参数，避免真实时间或磁盘编码掩盖写路径的变化。
// 没有效果的写也放进序列：一旦它们多写了记录，整份 golden 就会不匹配。
func TestAOFRecordsGolden(t *testing.T) {
	engine, elapsed := clockEngine()
	log := &recordingLog{}
	engine.AttachLog(log)
	for _, parts := range [][]string{
		{"SET", "binary", "a\r\nb\x00c"},
		{"LPUSH", "list", "a", "b"},
		{"LPUSH", "list", "c"},
		{"EXPIRE", "list", "100"},
		{"SET", "zero", "v"},
		{"EXPIRE", "zero", "0"},
		{"SET", "negative", "v"},
		{"EXPIRE", "negative", "-1"},
		{"LPUSH", "drained", "x", "y"},
		{"EXPIRE", "drained", "50"},
		{"LPOP", "drained"},
		{"LPOP", "drained"},
		{"LPOP", "drained"},
		{"LPOP", "missing"},
		{"SET", "left", "L"},
		{"SET", "right", "R"},
		{"DEL", "left", "left", "missing", "right"},
		{"DEL", "missing", "missing"},
		{"EXPIRE", "missing", "10"},
		{"EXPIRE", "missing", "0"},
		{"LPUSH", "reborn-list", "old"},
		{"EXPIRE", "reborn-list", "1"},
		{"SET", "reborn-string", "old"},
		{"EXPIRE", "reborn-string", "1"},
	} {
		if _, err := engine.Exec(command(parts...)); err != nil {
			t.Fatalf("%q: %v", parts, err)
		}
	}
	// 只推进注入的时钟，不先读 key，让 LPUSH 和 SET 自己遇到过期的旧值。
	elapsed.Store(int64(2 * time.Second))
	for _, parts := range [][]string{
		{"LPUSH", "reborn-list", "new"},
		{"SET", "reborn-string", "new"},
	} {
		if _, err := engine.Exec(command(parts...)); err != nil {
			t.Fatalf("%q: %v", parts, err)
		}
	}
	var records strings.Builder
	for _, record := range log.records {
		fmt.Fprintf(&records, "%q\n", record)
	}
	requireGolden(t, "testdata/aof-records.golden", []byte(records.String()))
}

// requireGolden 故意没有更新开关，防止后续重构把行为变化当作新基线接受。
func requireGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s changed\n got:\n%s\nwant:\n%s", path, got, want)
	}
}
