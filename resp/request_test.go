// 本文件：验证 AppendRequest 的二进制编码、前缀保留与协议预算，防止日志和客户端的编码再次分叉。

package resp

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"
)

func TestAppendRequest(t *testing.T) {
	args := [][]byte{[]byte("SET"), []byte("key"), {'a', '\r', '\n', 0}}
	encoded, err := AppendRequest([]byte("prefix"), args)
	if err != nil || string(encoded) != "prefix*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$4\r\na\r\n\x00\r\n" {
		t.Fatalf("encoded=%q err=%v", encoded, err)
	}
	got, size, err := ReadRequest(bufio.NewReader(bytes.NewReader(encoded[len("prefix"):])))
	if err != nil || !reflect.DeepEqual(got, args) || size != len(encoded)-len("prefix") {
		t.Fatalf("round trip args=%q size=%d err=%v", got, size, err)
	}
	for _, test := range []struct {
		name string
		args [][]byte
	}{
		{"empty", nil},
		{"array limit", make([][]byte, MaxArrayLength+1)},
		{"bulk limit", [][]byte{make([]byte, MaxBulkLength+1)}},
		{"frame limit", [][]byte{make([]byte, MaxBulkLength), make([]byte, MaxBulkLength)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got, err := AppendRequest(nil, test.args); err == nil || got != nil {
				t.Fatal("accepted oversized or empty request")
			}
		})
	}
	// dst 可以含前一条合法请求，它的字节不应占用下一条请求的预算。
	prefix := make([]byte, MaxRequestLength)
	if _, err := AppendRequest(prefix, [][]byte{[]byte("PING")}); err != nil {
		t.Fatal(err)
	}
}
