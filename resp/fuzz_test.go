// 本文件：FuzzReadRequest 对同步解析器做结构与边界模糊测试。

package resp

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"
)

// 保留多帧、空帧、边界和损坏输入种子，检查数据与错误互斥，且字节计数始终落在输入内。
func FuzzReadRequest(f *testing.F) {
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

	f.Fuzz(func(t *testing.T, data []byte) {
		reader := bufio.NewReader(bytes.NewReader(data))
		consumed := 0
		for {
			args, size, err := ReadRequest(reader)
			if (err == nil) == (args == nil) {
				t.Fatal("request must contain either args or error")
			}
			if err != nil {
				if size != 0 {
					t.Fatal("failed request reported complete bytes")
				}
				return
			}
			if size <= 0 || size > MaxRequestLength {
				t.Fatal("complete record has invalid wire byte count")
			}
			consumed += size
			if consumed > len(data) {
				t.Fatal("wire byte count exceeds input")
			}
		}
	})
}
