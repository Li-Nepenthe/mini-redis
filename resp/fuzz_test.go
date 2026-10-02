package resp

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

// FuzzParseStream 用 f 注册正常、多帧、边界和损坏输入种子，再对变异字节运行 Parser。
// 回调断言 Payload 非 nil、Data/Err 互斥、完整帧 BytesRead 合法，防止 panic 与无效状态；普通 test 只跑种子，长 fuzz 需显式 -fuzz。
func FuzzParseStream(f *testing.F) {
	seeds := [][]byte{
		{},
		[]byte("*2\r\n$3\r\nGET\r\n$4\r\nname\r\n"),
		[]byte("*2\r\n$3\r\nGET\r\n$4\r\nname\r\n*1\r\n$4\r\nPING\r\n"),
		[]byte("*-1\r\n"),
		[]byte(fmt.Sprintf("*%d\r\n", MaxArrayLength+1)),
		[]byte("*1\r\n$-1\r\n"),
		[]byte(fmt.Sprintf("*1\r\n$%d\r\n", MaxBulkLength+1)),
		[]byte("*1\r\n$5\r\nabc"),
		[]byte("*x\r\n"),
		{0x00, 0xff, '\r', '\n'},
		bytes.Repeat([]byte("9"), MaxHeaderLength+1),
		[]byte("*1\n$3\r\nGET\r\n"),
		[]byte("*0\r\n"),
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	// 每次变异只用有限 bytes.Reader，读到通道关闭；这里只断言结构/边界，业务语义另由命令测试覆盖。
	f.Fuzz(func(t *testing.T, data []byte) {
		parser := NewRespParser()
		for payload := range parser.ParseStream(context.Background(), bytes.NewReader(data)) {
			if payload == nil {
				t.Fatal("ParseStream() 返回了 nil Payload")
			}
			if (payload.Err == nil) == (payload.Data == nil) {
				t.Fatal("Payload 必须且只能包含 Data 或 Err 之一")
			}
			if payload.Err == nil && (payload.BytesRead <= 0 || payload.BytesRead > MaxRequestLength) {
				t.Fatal("complete record has invalid wire byte count")
			}
		}
	})
}
