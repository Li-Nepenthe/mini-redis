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

// NewRespParser 返回无共享可变状态的 RESP 请求解析器，不读取输入或启动 goroutine。
// 实际解析与通道生命周期在每次 ParseStream 调用中独立创建，同一实例可为不同连接提供解析。
func NewRespParser() *Parser {
	return &Parser{}
}

// ParseStream 为 reader 启动一个解析 goroutine，返回只读 Payload 通道，依次提供完整参数或一次解析错误后关闭通道。
// 干净 EOF 不发错误；Payload.BytesRead 仅完整成功帧有效，用于真实 AOF 偏移。生产者负责关闭 channel，消费者不可关闭它。
// ctx 可停止循环/解除 channel 发送阻塞，但不能打断任意 reader.Read；所有者必须关闭阻塞 reader 并等待通道关闭。
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

// readHeader 从 bufio reader 读取一个带 CRLF 的长度头，返回去掉 CRLF 的字节或格式/长度/读取错误。
// 头预算含 CRLF；完全无字节 EOF 返回 io.EOF，残头则包装 EOF 供 AOF 判半尾。返回切片借用 reader 缓冲，须在下次读取前处理。
// ReadSlice 使用固定缓冲避免恶意无换行输入无限扩容，不把内容段中的 CRLF 当作 bulk 边界。
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

// readRequest 按数组→bulk 长度→精确内容→CRLF 读取一条请求，返回参数、实际完整帧字节数及错误。
// ctx 在逐参数阶段检查；失败返回 nil、0、错误，开头干净 EOF 单独返回。允许空数组，但业务上是否忽略由 Handler 决定。
// 分配前检查 1024 参数、16MiB 单 bulk、64 字节头和 32MiB 整帧预算；ReadFull 可跨 TCP 半包，bufio 保留后续粘连帧。
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
