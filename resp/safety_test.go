package resp

import (
	"context"
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

// TestParserHeaderBoundary 构造含 CRLF 恰好 64 字节与超一字节的头，验证接受/拒绝边界。
// t 检查合法数据与错误结果，防止头预算漏算分隔符，无真实连接。
func TestParserHeaderBoundary(t *testing.T) {
	for _, extra := range []int{0, 1} {
		header := "*" + strings.Repeat("0", MaxHeaderLength-4+extra) + "1\r\n"
		input := header + "$0\r\n\r\n"
		var gotErr error
		var got [][]byte
		for payload := range NewRespParser().ParseStream(context.Background(), strings.NewReader(input)) {
			got, gotErr = payload.Data, payload.Err
		}
		if extra == 0 && (gotErr != nil || len(got) != 1) {
			t.Fatalf("exact header limit: args=%v err=%v", got, gotErr)
		}
		if extra == 1 && gotErr == nil {
			t.Fatal("accepted header over the limit")
		}
	}
}

// TestParserRequestByteBudget 构造两个单独合法 bulk 但整帧超限，故意不提供第二段内容。
// t 要求预算错误先于分配/读取，而非最终 EOF；覆盖累计请求上限而不仅单 bulk 上限。
func TestParserRequestByteBudget(t *testing.T) {
	// The second body is intentionally absent: a byte-budget error must occur
	// before allocating or trying to read that body, rather than an EOF error.
	input := fmt.Sprintf("*2\r\n$%d\r\n", MaxBulkLength) + strings.Repeat("x", MaxBulkLength) +
		fmt.Sprintf("\r\n$%d\r\n", MaxBulkLength)
	var gotErr error
	for payload := range NewRespParser().ParseStream(context.Background(), strings.NewReader(input)) {
		if payload.Data != nil {
			t.Fatal("oversized request produced data")
		}
		gotErr = payload.Err
	}
	if gotErr == nil || !strings.Contains(gotErr.Error(), "request exceeds byte limit") {
		t.Fatalf("want request byte-budget error, got %v", gotErr)
	}
}

// TestParserIncompleteCommandIsNotCleanEOF 验证残数组头、缺 bulk 和残内容各发送一次错误，不能误当干净 EOF 消失。
// t 统计 Payload 并禁止成功数据，支撑 AOF 只截不完整尾部的判定。
func TestParserIncompleteCommandIsNotCleanEOF(t *testing.T) {
	for _, input := range []string{"*1", "*1\r\n", "*1\r\n$1\r\nx"} {
		var count int
		for payload := range NewRespParser().ParseStream(context.Background(), strings.NewReader(input)) {
			if payload.Err == nil {
				t.Fatalf("incomplete input produced data: %q", input)
			}
			count++
		}
		if count != 1 {
			t.Fatalf("input %q: want one error, got %d", input, count)
		}
	}
}

// TestParserCancellation 用 io.Pipe 启动解析后取消并关闭 reader，等待通道在预算内关闭。
// t 验证取消加 I/O 打断的组合，不声称 context 可以自行中断任意 Reader.Read。
func TestParserCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	ch := NewRespParser().ParseStream(ctx, reader)
	if _, err := io.WriteString(writer, "*1\r\n$3\r\nGET\r\n"); err != nil {
		t.Fatal(err)
	}
	cancel()
	// A general io.Reader cannot be interrupted by context; the owner closes it.
	_ = reader.Close()
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("parser did not stop after cancellation and reader close")
	}
}

// Read 转发到嵌入的 Reader，将实际 n 累加到 bytesRead，并返回原 n、err。
// 测试借此观察 bufio 预读量，不把它当 Payload 的完整帧大小；仅单解析 goroutine 使用，无额外锁。
func (r *countingReader) Read(buf []byte) (int, error) {
	n, err := r.Reader.Read(buf)
	r.bytesRead += n
	return n, err
}

// TestParserLimitsHeaderRead 输入 16KiB 无有效短头，验证错误后底层 reader 最多消费一个 4096 字节缓冲。
// t 用 countingReader 观察实际预读，证明没有 ReadBytes 式无界扩容；不是要求全部头上限恰好只读 64 字节。
func TestParserLimitsHeaderRead(t *testing.T) {
	reader := &countingReader{Reader: strings.NewReader("*" + strings.Repeat("9", 16<<10) + "\r\n")}
	for range NewRespParser().ParseStream(context.Background(), reader) {
	}
	if reader.bytesRead > 4096 {
		t.Fatalf("oversized header consumed %d bytes; want at most one 4096-byte reader buffer", reader.bytesRead)
	}
}

// TestParserRequiresCRLF 分别给数组头与 bulk 头只带 LF，验证产生协议错误且不返回成功参数。
// t 使用有限内存输入，区分合法内容中的换行与头部必须的 CRLF 分隔。
func TestParserRequiresCRLF(t *testing.T) {
	for _, input := range []string{"*1\n$3\r\nGET\r\n", "*1\r\n$3\nGET\r\n"} {
		var gotError bool
		for payload := range NewRespParser().ParseStream(context.Background(), strings.NewReader(input)) {
			if payload.Err != nil {
				gotError = true
			} else {
				t.Errorf("accepted header without CRLF: %q", input)
			}
		}
		if !gotError {
			t.Errorf("missing protocol error for %q", input)
		}
	}
}
