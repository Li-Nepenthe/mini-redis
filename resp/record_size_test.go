// 本文件：验证 ReadRequest 对非规范长度头和空数组报告原始帧的实际字节数。

package resp

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

// 带前导零的长度头不能被规范化后的编码长度代替，否则 AOF 的截断位置会偏移。
func TestParserReportsActualWireRecordSizes(t *testing.T) {
	commands := []string{"*01\r\n$003\r\nGET\r\n", "*0\r\n"}
	i := 0
	reader := bufio.NewReader(strings.NewReader(strings.Join(commands, "")))
	for {
		_, size, err := ReadRequest(reader)
		if err == io.EOF {
			break
		}
		if err != nil || size != len(commands[i]) {
			t.Fatalf("record %d: bytes=%d err=%v", i, size, err)
		}
		i++
	}
	if i != len(commands) {
		t.Fatal("missing record")
	}
}
