// 本文件：EncodeReply 把执行结果编码成 RESP；appendBulk 保留二进制内容，validErrorCode 限制可公开的错误码。

package resp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type publicError interface {
	// 只公开约定的错误码和消息，内部原因可能包含路径等敏感信息。
	RESPError() (code, message string)
}

// EncodeReply 让执行结果的类型直接决定协议类型：实现 RESPStatus 的值输出简单状态，
// int64 输出整数，[]byte 输出 bulk，[][]byte 输出数组，nil 输出空值。
// bool 和普通 string 没有约定的协议含义，不能猜测它们是否代表 OK 或一段二进制数据。
// 业务错误只公开 RESPError 的内容；其余错误统一隐藏，防止内部细节进入客户端回复。
func EncodeReply(result any, execErr error) ([]byte, error) {
	if execErr != nil {
		var public publicError
		if errors.As(execErr, &public) {
			code, message := public.RESPError()
			if validErrorCode(code) {
				// 错误只能占一行，即使公开业务错误也不能带 CRLF 注入另一条 RESP 回复。
				message = strings.NewReplacer("\r", " ", "\n", " ").Replace(message)
				return []byte("-" + code + " " + message + "\r\n"), nil
			}
		}
		return []byte("-ERR internal server error\r\n"), nil
	}
	switch value := result.(type) {
	case nil:
		return []byte("$-1\r\n"), nil
	case []byte:
		return appendBulk(nil, value), nil
	case [][]byte:
		reply := []byte("*" + strconv.Itoa(len(value)) + "\r\n")
		for _, element := range value {
			reply = appendBulk(reply, element)
		}
		return reply, nil
	case interface{ RESPStatus() string }:
		status := value.RESPStatus()
		if strings.ContainsAny(status, "\r\n") {
			return nil, errors.New("invalid status reply")
		}
		return []byte("+" + status + "\r\n"), nil
	case int64:
		return []byte(":" + strconv.FormatInt(value, 10) + "\r\n"), nil
	default:
		return nil, fmt.Errorf("unsupported reply type: %T", value)
	}
}

// appendBulk 按字节长度写出内容，值里的换行和零字节都不需要转义；空内容与空值是两种回复。
func appendBulk(reply, value []byte) []byte {
	reply = append(reply, '$')
	reply = strconv.AppendInt(reply, int64(len(value)), 10)
	reply = append(reply, '\r', '\n')
	reply = append(reply, value...)
	return append(reply, '\r', '\n')
}

// validErrorCode 只接受大写字母，防止错误码夹带换行，把额外协议帧注入回复。
func validErrorCode(code string) bool {
	if code == "" {
		return false
	}
	for _, ch := range code {
		if ch < 'A' || ch > 'Z' {
			return false
		}
	}
	return true
}
