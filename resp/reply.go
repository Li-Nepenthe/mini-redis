package resp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type publicError interface {
	// RESPError 提供可公开的码和消息，不能携带内部 cause；编码器仍负责码格式与 CRLF 校验。
	RESPError() (code, message string)
}

// EncodeReply 将 result 或 execErr 编为一条 RESP 响应，返回字节及编码错误，不进行网络 I/O。
// 业务错误仅通过 RESPError 白名单公开，内部错误用通用 ERR；错误码只允许大写字母，消息去 CRLF，cause 留供 errors.Is/As。
// 支持 nil、[]byte、[][]byte、状态接口、true、int/int64；false、普通 string、非法状态或其他结果类型返回编码错误。二进制内容按长度编码。
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

// appendBulk 在 reply 尾部追加 value 的 bulk 长度头、原始二进制内容与 CRLF，返回扩展后的切片。
// 不修改 value，但 append 可能复用/扩容 reply 底层数组；空 value 编为 $0，与 nil 结果的 $-1 语义不同。
func appendBulk(reply, value []byte) []byte {
	reply = append(reply, '$')
	reply = strconv.AppendInt(reply, int64(len(value)), 10)
	reply = append(reply, '\r', '\n')
	reply = append(reply, value...)
	return append(reply, '\r', '\n')
}

// validErrorCode 检查 code 是否为非空的纯 ASCII 大写字母，返回能否安全用作公开 RESP 错误码。
// 不修改输入；拒绝空白、数字、小写和换行，避免错误码把另一条协议帧注入回复。
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
