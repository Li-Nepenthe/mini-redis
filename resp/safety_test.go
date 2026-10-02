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

func (r *countingReader) Read(buf []byte) (int, error) {
	n, err := r.Reader.Read(buf)
	r.bytesRead += n
	return n, err
}

func TestParserLimitsHeaderRead(t *testing.T) {
	reader := &countingReader{Reader: strings.NewReader("*" + strings.Repeat("9", 16<<10) + "\r\n")}
	for range NewRespParser().ParseStream(context.Background(), reader) {
	}
	if reader.bytesRead > 4096 {
		t.Fatalf("oversized header consumed %d bytes; want at most one 4096-byte reader buffer", reader.bytesRead)
	}
}

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
