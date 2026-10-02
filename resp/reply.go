package resp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type publicError interface {
	RESPError() (code, message string)
}

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
	case bool:
		if value {
			return []byte("+OK\r\n"), nil
		}
		return nil, errors.New("unexpected false result")
	case int:
		return []byte(":" + strconv.Itoa(value) + "\r\n"), nil
	case int64:
		return []byte(":" + strconv.FormatInt(value, 10) + "\r\n"), nil
	default:
		return nil, fmt.Errorf("unsupported reply type: %T", value)
	}
}

func appendBulk(reply, value []byte) []byte {
	reply = append(reply, '$')
	reply = strconv.AppendInt(reply, int64(len(value)), 10)
	reply = append(reply, '\r', '\n')
	reply = append(reply, value...)
	return append(reply, '\r', '\n')
}

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
