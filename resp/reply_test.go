// 本文件：TestEncodeReply 固定每种回复类型的编码，拒绝靠 bool 或 int 隐式表达状态和整数。

package resp

import (
	"bytes"
	"errors"
	"testing"

	"github.com/Li-Nepenthe/mini-redis/database"
)

// TestEncodeReply 比较完整字节，既区分空值与空内容，也防止内部回复类型重新变得含糊。
func TestEncodeReply(parentT *testing.T) {
	tests := []struct {
		name    string
		result  any
		execErr error
		want    []byte
		wantErr bool
	}{
		{
			name:   "nil 结果编码为空块字符串",
			result: nil,
			want:   []byte("$-1\r\n"),
		},
		{
			name:   "字节切片编码为块字符串",
			result: []byte("hello"),
			want:   []byte("$5\r\nhello\r\n"),
		},
		{
			name:   "返回长度3",
			result: int64(3),
			want:   []byte(":3\r\n"),
		},
		{
			name:   "返回OK状态",
			result: database.StatusReply("OK"),
			want:   []byte("+OK\r\n"),
		},
		{
			name:   "返回空[]byte",
			result: []byte{},
			want:   []byte("$0\r\n\r\n"),
		},
		{
			name:    "错误非空",
			execErr: errors.New("错误非空"),
			want:    []byte("-ERR internal server error\r\n"),
		},
		{
			name:    "拒绝true类型",
			result:  true,
			wantErr: true,
		},
		{
			name:    "拒绝int类型",
			result:  3,
			wantErr: true,
		},
		{
			name:    "拒绝false  不支持的类型",
			result:  false,
			wantErr: true,
		},
		{
			name:    "拒绝string类型",
			result:  "hello",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		parentT.Run(tt.name, func(caseT *testing.T) {
			got, err := EncodeReply(tt.result, tt.execErr)
			if (err != nil) != tt.wantErr {
				caseT.Fatalf("EncodeReply() 返回错误 = %v，是否预期错误 = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !bytes.Equal(got, tt.want) {
				caseT.Fatalf("EncodeReply() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}
