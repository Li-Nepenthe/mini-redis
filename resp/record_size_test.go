package resp

import (
	"context"
	"strings"
	"testing"
)

// TestParserReportsActualWireRecordSizes 解析带前导零长度头和空数组，验证 BytesRead 等于原帧长度并保留全部帧。
// t 防止用规范化重新编码长度替代 AOF 真实偏移；无文件 I/O。
func TestParserReportsActualWireRecordSizes(t *testing.T) {
	commands := []string{"*01\r\n$003\r\nGET\r\n", "*0\r\n"}
	i := 0
	for payload := range NewRespParser().ParseStream(context.Background(), strings.NewReader(strings.Join(commands, ""))) {
		if payload.Err != nil || payload.BytesRead != len(commands[i]) {
			t.Fatalf("record %d: bytes=%d err=%v", i, payload.BytesRead, payload.Err)
		}
		i++
	}
	if i != len(commands) {
		t.Fatal("missing record")
	}
}
