package resp

import (
	"bytes"
	"errors"
	"testing"
)

// TestEncodeReply 用 parentT 的表驱动子测试验证 nil/空 bulk/字节/整数/true/内部错误的准确编码及不支持类型报错。
// 区分业务错误编码结果与函数的 Go error；比较字节而非界面显示，无网络 I/O。
func TestEncodeReply(parentT *testing.T) {
	// tests := []struct { ... }{ ... } 是一个结构体切片

	/**
	这是测试用例表。每一项都包含：
		输入 result
		输入 execErr
		期望的字节结果 want
		是否期望出现 Go error
	*/
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
			result: 3,
			want:   []byte(":3\r\n"),
		},
		{
			name:   "返回true",
			result: true,
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
		// 含义：parentT 请帮我运行一个名字叫 tt.name 的子测试。
		parentT.Run(tt.name, func(caseT *testing.T) { // 这是子测试要执行的代码
			got, err := EncodeReply(tt.result, tt.execErr)
			// 这个表达式的含义就是如果错误存在 是否符合预期的存在
			if (err != nil) != tt.wantErr {
				caseT.Fatalf("EncodeReply() 返回错误 = %v，是否预期错误 = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			// 因为[]byte 不能直接用 == 比较内容，所以用它判断两个字节数组是否完全一致
			if !bytes.Equal(got, tt.want) {
				caseT.Fatalf("EncodeReply() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}
