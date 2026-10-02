package resp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
)

type Payload struct {
	BytesRead int // 用实际消费字节定位 AOF，不能用 bufio 预读量或重新编码长度代替。
	Data      [][]byte
	Err       error
}

type Parser struct{}

const (
	MaxArrayLength   = 1024
	MaxBulkLength    = 16 << 20 // 16 MiB
	MaxHeaderLength  = 64       // 包括 CRLF，头部预算必须把帧边界计入。
	MaxRequestLength = 32 << 20
)

func NewRespParser() *Parser {
	return &Parser{}
}

// context 只能取消 channel 发送；阻塞在 Read 时，所有者还必须关闭 reader（如连接）。
func (p *Parser) ParseStream(ctx context.Context, reader io.Reader) <-chan *Payload {
	channel := make(chan *Payload)
	go func() {
		defer close(channel)
		buffer := bufio.NewReader(reader)
		for ctx.Err() == nil {
			args, size, err := readRequest(ctx, buffer)
			if err == io.EOF || ctx.Err() != nil {
				return
			}
			payload := &Payload{Data: args, Err: err, BytesRead: size}
			select {
			case channel <- payload:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	return channel
}

func readHeader(reader *bufio.Reader) ([]byte, error) {
	// ReadBytes 会随恶意无换行输入扩容；固定缓冲的 ReadSlice 能在达到上限时停止。
	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) || len(line) > MaxHeaderLength {
		return nil, fmt.Errorf("protocol error: header exceeds %d bytes", MaxHeaderLength)
	}
	if err != nil {
		if err == io.EOF && len(line) == 0 {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("protocol error: incomplete header: %w", err)
	}
	if len(line) < 3 || !bytes.HasSuffix(line, []byte("\r\n")) {
		return nil, errors.New("protocol error: header missing CRLF")
	}
	return line[:len(line)-2], nil
}

// 只让合法的数组/bulk 帧进入业务层；业务参数错误交给 Exec。
// io.ReadFull 按协议长度凑齐内容，不把一次 TCP Read 的大小误当成命令边界。
func readRequest(ctx context.Context, reader *bufio.Reader) ([][]byte, int, error) {
	line, err := readHeader(reader)
	if err != nil {
		return nil, 0, err
	}
	if line[0] != '*' {
		return nil, 0, errors.New("protocol error: expected array")
	}
	length, err := strconv.Atoi(string(line[1:]))
	if err != nil || length < 0 || length > MaxArrayLength {
		return nil, 0, errors.New("protocol error: invalid array length")
	}
	used := len(line) + 2
	args := make([][]byte, length)
	for i := range args {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		header, err := readHeader(reader)
		if err != nil {
			return nil, 0, fmt.Errorf("protocol error: incomplete bulk header: %w", err)
		}
		if header[0] != '$' {
			return nil, 0, errors.New("protocol error: expected bulk string")
		}
		size, err := strconv.Atoi(string(header[1:]))
		if err != nil || size < 0 || size > MaxBulkLength {
			return nil, 0, errors.New("protocol error: invalid bulk length")
		}
		used += len(header) + 2
		// 分配下一段前检查整条请求预算；仅限制单 bulk 仍允许 1024 段各占 16 MiB。
		if size+2 > MaxRequestLength-used {
			return nil, 0, errors.New("protocol error: request exceeds byte limit")
		}
		used += size + 2
		content := make([]byte, size+2)
		if _, err := io.ReadFull(reader, content); err != nil {
			return nil, 0, fmt.Errorf("protocol error: incomplete bulk content: %w", err)
		}
		if content[size] != '\r' || content[size+1] != '\n' {
			return nil, 0, errors.New("protocol error: bulk content missing CRLF")
		}
		args[i] = content[:size]
	}
	return args, used, nil
}
