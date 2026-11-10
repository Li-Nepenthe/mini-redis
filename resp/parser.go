// 本文件：ReadRequest 和 readHeader 同步解析一条 RESP 请求，保留协议上限、错误分类和实际字节计数。

package resp

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	MaxArrayLength   = 1024
	MaxBulkLength    = 16 << 20 // 16 MiB
	MaxHeaderLength  = 64       // 包括 CRLF，头部预算必须把帧边界计入。
	MaxRequestLength = 32 << 20
)

// 头部必须有界，避免恶意的无换行输入无限扩容。
// 完全没有字节的 EOF 与残头分开返回，AOF 才能区分文件结束和半条尾部。
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

// ReadRequest 同步读取一条请求；成功时返回实际消费字节数，不能用预读量或重新编码长度代替。
// 只有请求开头的干净 EOF 原样返回，其余残帧包装 EOF，供 AOF 决定是否修复尾部。
func ReadRequest(reader *bufio.Reader) ([][]byte, int, error) {
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
