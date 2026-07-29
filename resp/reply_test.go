package resp

import (
	"bytes"
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
			name:   "nil result becomes null bulk string",
			result: nil,
			want:   []byte("$-1\r\n"),
		},
		{
			name:   "byte slice becomes bulk string",
			result: []byte("hello"),
			want:   []byte("$5\r\nhello\r\n"),
		},
	}

	for _, tt := range tests {
		// 含义：parentT 请帮我运行一个名字叫 tt.name 的子测试。
		parentT.Run(tt.name, func(caseT *testing.T) { // 这是子测试要执行的代码
			got, err := EncodeReply(tt.result, tt.execErr)
			if (err != nil) != tt.wantErr {
				caseT.Fatalf("EncodeReply() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			// 因为[]byte 不能直接用 == 比较内容，所以用它判断两个字节数组是否完全一致
			if !bytes.Equal(got, tt.want) {
				caseT.Fatalf("EncodeReply() = %q, want %q", got, tt.want)
			}
		})
	}
}
