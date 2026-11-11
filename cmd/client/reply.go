// 本文件：readReply 按 RESP 类型消费完整回复，readBulkReply 与 readArrayReply 保留二进制长度和嵌套边界。

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Li-Nepenthe/mini-redis/resp"
)

// 业务错误也是一条完整回复，显示后仍可继续交互；格式错误则交回调用者结束会话，避免继续读取错位的数据。
func readReply(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) < 3 || !strings.HasSuffix(line, "\r\n") {
		return "", errors.New("reply header missing CRLF")
	}
	value := line[1 : len(line)-2]
	switch line[0] {
	case '+':
		return value, nil
	case '-':
		return "(error) " + value, nil
	case ':':
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid integer reply: %w", err)
		}
		return "(integer) " + strconv.FormatInt(n, 10), nil
	case '$', '*':
		n, err := strconv.Atoi(value)
		if err != nil || n < -1 {
			return "", errors.New("invalid reply length")
		}
		if n == -1 {
			return "(nil)", nil
		}
		if line[0] == '$' {
			return readBulkReply(reader, n)
		}
		return readArrayReply(reader, n)
	default:
		return "", errors.New("unknown reply type")
	}
}

// Mini-Redis 的值受单 bulk 上限约束；先检查长度，避免坏回复诱导分配。内容中的 CRLF 仍属于值本身。
func readBulkReply(reader *bufio.Reader, size int) (string, error) {
	if size > resp.MaxBulkLength {
		return "", errors.New("reply bulk exceeds protocol limit")
	}
	data := make([]byte, size+2)
	if _, err := io.ReadFull(reader, data); err != nil {
		return "", err
	}
	if data[size] != '\r' || data[size+1] != '\n' {
		return "", errors.New("reply bulk missing CRLF")
	}
	return strconv.Quote(string(data[:size])), nil
}

// 每个元素递归消费自己的完整帧；数组长度不能套用请求参数个数上限，因为 LRANGE 可返回更长的列表。
func readArrayReply(reader *bufio.Reader, size int) (string, error) {
	if size == 0 {
		return "(empty array)", nil
	}
	var result strings.Builder
	for i := 0; i < size; i++ {
		value, err := readReply(reader)
		if err != nil {
			return "", err
		}
		if i > 0 {
			result.WriteByte('\n')
		}
		prefix := strconv.Itoa(i+1) + ") "
		result.WriteString(prefix)
		result.WriteString(strings.ReplaceAll(value, "\n", "\n"+strings.Repeat(" ", len(prefix))))
	}
	return result.String(), nil
}
