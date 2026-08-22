package resp

import (
	"bytes"
	"fmt"
	"testing"
)

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
	}

	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		parser := NewRespParser()
		for payload := range parser.ParseStream(bytes.NewReader(data)) {
			if payload == nil {
				t.Fatal("ParseStream() 返回了 nil Payload")
			}
			if (payload.Err == nil) == (payload.Data == nil) {
				t.Fatal("Payload 必须且只能包含 Data 或 Err 之一")
			}
		}
	})
}
