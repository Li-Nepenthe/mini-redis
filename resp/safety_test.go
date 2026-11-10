// 本文件：检查同步解析的头部与整帧预算、残帧错误、CRLF，以及关闭读取端能否打断等待。

package resp

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

type countingReader struct {
	io.Reader
	bytesRead int
}

// 把 CRLF 也计入头部上限，检查恰好到边界与多一个字节的区别。
func TestParserHeaderBoundary(t *testing.T) {
	for _, extra := range []int{0, 1} {
		header := "*" + strings.Repeat("0", MaxHeaderLength-4+extra) + "1\r\n"
		input := header + "$0\r\n\r\n"
		got, _, gotErr := ReadRequest(bufio.NewReader(strings.NewReader(input)))
		if extra == 0 && (gotErr != nil || len(got) != 1) {
			t.Fatalf("exact header limit: args=%v err=%v", got, gotErr)
		}
		if extra == 1 && gotErr == nil {
			t.Fatal("accepted header over the limit")
		}
	}
}

// 故意不提供第二段内容，超限必须先于分配和读取被发现，不能误报为 EOF。
func TestParserRequestByteBudget(t *testing.T) {
	input := fmt.Sprintf("*2\r\n$%d\r\n", MaxBulkLength) + strings.Repeat("x", MaxBulkLength) +
		fmt.Sprintf("\r\n$%d\r\n", MaxBulkLength)
	got, _, gotErr := ReadRequest(bufio.NewReader(strings.NewReader(input)))
	if got != nil {
		t.Fatal("oversized request produced data")
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "request exceeds byte limit") {
		t.Fatalf("want request byte-budget error, got %v", gotErr)
	}
}

func TestParserIncompleteCommandIsNotCleanEOF(t *testing.T) {
	for _, input := range []string{"*1", "*1\r\n", "*1\r\n$1\r\nx"} {
		got, size, err := ReadRequest(bufio.NewReader(strings.NewReader(input)))
		if err == nil || err == io.EOF || got != nil || size != 0 {
			t.Fatalf("incomplete input %q: args=%q size=%d err=%v", input, got, size, err)
		}
	}
}

// 同步读取不接收 context；连接所有者关闭读取端，才能打断正在等待的半条请求。
func TestReadRequestCloseInterruptsRead(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	done := make(chan error, 1)
	go func() { _, _, err := ReadRequest(bufio.NewReader(reader)); done <- err }()
	if _, err := io.WriteString(writer, "*1\r\n$3\r\nG"); err != nil {
		t.Fatal(err)
	}
	_ = reader.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed reader completed a partial frame")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("read did not stop after reader close")
	}
}

func (r *countingReader) Read(buf []byte) (int, error) {
	n, err := r.Reader.Read(buf)
	r.bytesRead += n
	return n, err
}

// 用底层读取量确认超长头不会无界扩容；bufio 可以预读一个缓冲区，不必恰好停在 64 字节。
func TestParserLimitsHeaderRead(t *testing.T) {
	reader := &countingReader{Reader: strings.NewReader("*" + strings.Repeat("9", 16<<10) + "\r\n")}
	if _, _, err := ReadRequest(bufio.NewReader(reader)); err == nil {
		t.Fatal("accepted oversized header")
	}
	if reader.bytesRead > 4096 {
		t.Fatalf("oversized header consumed %d bytes; want at most one 4096-byte reader buffer", reader.bytesRead)
	}
}

func TestParserRequiresCRLF(t *testing.T) {
	for _, input := range []string{"*1\n$3\r\nGET\r\n", "*1\r\n$3\nGET\r\n"} {
		got, _, err := ReadRequest(bufio.NewReader(strings.NewReader(input)))
		if got != nil {
			t.Errorf("accepted header without CRLF: %q", input)
		}
		if err == nil {
			t.Errorf("missing protocol error for %q", input)
		}
	}
}
