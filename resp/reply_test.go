package resp

import (
	"bytes"
	"errors"
	"testing"
)

// 这是 Go 识别测试的固定规则：函数名以 Test 开头，并接收 *testing.T作为参数 go test 会自动扫描并运行它
// TestEncodeReply这个测试函数需要 Go 测试框架传给我一个 t t 的类型是 *testing.T
// testing是Go标准库中的测试包
// testing.T 是 Go 为“一次测试”创建的对象，里面记录：
// 当前测试是否失败、测试日志、测试名称、子测试、清理函数等
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
