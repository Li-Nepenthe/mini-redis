// 本文件：用 TestSessionGolden 固定一次真实 TCP 会话的全部回复，作为后续协议和服务重构的字节基线。

package server

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestSessionGolden 把正常命令、业务错误和最后的协议错误放进同一条流水线。
// 读到 EOF 才比较，既能发现多余回复，也能发现协议错误后没有关闭连接。
func TestSessionGolden(t *testing.T) {
	want, err := os.ReadFile("testdata/session.golden")
	if err != nil {
		t.Fatal(err)
	}
	_, listener, _ := startServer(t)
	conn := dial(t, listener.Addr().String())
	var input strings.Builder
	for _, parts := range [][]string{
		{"PING"},
		{"PING", "echo\r\n\x00"},
		{"SET", "binary", "a\r\nb\x00c"},
		{"GET", "binary"},
		{"GET", "missing"},
		{"TTL", "binary"},
		{"TTL", "missing"},

		{"LPUSH", "list", "a", "b", "c"},
		{"LRANGE", "list", "0", "1"},
		{"LRANGE", "list", "-2", "-1"},
		{"LRANGE", "list", "-100", "100"},
		{"LRANGE", "list", "100", "200"},
		{"LRANGE", "list", "-9", "-8"},
		{"LRANGE", "list", "2", "1"},
		{"GET", "list"},
		{"LPOP", "list"},
		{"LPOP", "list"},
		{"LPOP", "list"},
		{"LPOP", "list"},
		{"LPOP", "missing"},
		{"LRANGE", "list", "0", "-1"},
		{"EXISTS", "list"},
		{"TTL", "list"},

		{"SET", "left", "L"},
		{"SET", "right", "R"},
		{"EXISTS", "binary", "binary", "missing", "left"},
		{"DEL", "left", "left", "missing", "right"},
		{"EXISTS", "left", "right"},
		{"DEL", "missing"},

		// EXPIRE 和 TTL 相邻发送，不靠等待过期来生成随机器负载变化的结果。
		{"SET", "expiring", "v"},
		{"EXPIRE", "expiring", "100"},
		{"TTL", "expiring"},
		{"EXPIRE", "expiring", "9223372036854775807"},
		{"EXPIRE", "expiring", "9223372036854775808"},
		{"EXPIRE", "expiring", "not-an-integer"},
		{"EXPIRE", "expiring", "0"},
		{"TTL", "expiring"},
		{"SET", "negative", "v"},
		{"EXPIRE", "negative", "-10"},
		{"GET", "negative"},
		{"TTL", "negative"},
		{"EXPIRE", "missing", "100"},

		{"GET"},
		{"SET", "binary"},
		{"PING", "a", "b"},
		{"LRANGE", "list", "bad", "-1"},
		{"unknown"},
		{"_LNEW", "binary", "v"},
		{"__EXPIREATMS", "binary", "0"},
		{"ping"},
		{"get", "binary"},
		{}, // 原始 *0\r\n 不应产生回复，也不能阻止下一条命令执行。
		{"PING", "after-empty"},
	} {
		input.WriteString(request(parts...))
	}
	input.WriteString("PING\r\n")
	if n, err := io.WriteString(conn, input.String()); err != nil || n != input.Len() {
		t.Fatalf("write pipeline: bytes=%d want=%d err=%v", n, input.Len(), err)
	}
	got, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read replies through server EOF: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("TCP replies changed\n got: %q\nwant: %q", got, want)
	}
}
