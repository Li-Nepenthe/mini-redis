package resp

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/Li-Nepenthe/mini-redis/database"
	"testing"
)

type testPublicError struct{ code, message string }

// Error 返回固定内部错误文字，供公开错误测试证明编码器不直接输出 error 的文本。
// 不使用 code/message、不修改对象；实际公开字段由 RESPError 提供。
func (e testPublicError) Error() string { return "private error chain" }

// RESPError 返回模拟对象的 code/message，允许测试注入换行或非法码检查公开错误边界。
// 不清洗输入也不修改对象，清洗/拒绝责任在 EncodeReply，Error 文本仍保持私有。
func (e testPublicError) RESPError() (string, string) { return e.code, e.message }

// TestCommandReplyEncoding 验证状态、数组、二进制、int64、公开/包装/内部错误及换行注入的准确 RESP 字节。
// t 还断言状态换行产生编码错误；业务错误能编码为回复不等于编码器本身出错，无网络副作用。
func TestCommandReplyEncoding(t *testing.T) {
	for _, test := range []struct {
		name   string
		result any
		err    error
		want   string
	}{
		{"pong", database.StatusReply("PONG"), nil, "+PONG\r\n"},
		{"array", [][]byte{[]byte("c"), []byte("\x00\r\n"), {}}, nil, "*3\r\n$1\r\nc\r\n$3\r\n\x00\r\n\r\n$0\r\n\r\n"},
		{"empty array", [][]byte{}, nil, "*0\r\n"},
		{"int64", int64(-2), nil, ":-2\r\n"},
		{"wrongtype", nil, database.ErrTypeMismatch, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{"wrapped public", nil, fmt.Errorf("private /secret/path: %w", database.ErrTypeMismatch), "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{"internal", nil, errors.New("private /secret/path\r\n+injected"), "-ERR internal server error\r\n"},
		{"public newline", nil, testPublicError{"ERR", "one\r\ntwo"}, "-ERR one  two\r\n"},
		{"invalid error code", nil, testPublicError{"ERR\r\n+", "message"}, "-ERR internal server error\r\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := EncodeReply(test.result, test.err)
			if err != nil || !bytes.Equal(got, []byte(test.want)) {
				t.Fatalf("reply=%q err=%v want=%q", got, err, test.want)
			}
		})
	}
	if _, err := EncodeReply(database.StatusReply("OK\r\n+injected"), nil); err == nil {
		t.Fatal("status reply allowed protocol injection")
	}
}
