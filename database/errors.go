package database

import (
	"errors"
	"fmt"
)

type commandError struct {
	code, message string
	cause         error
}

// Error 返回公开的错误码与消息文本，供 error 接口显示；不拼接内部 cause。
// 没有状态修改，也不进行 RESP 转义；网络可公开的字段仍由 RESPError 和 EncodeReply 校验。
func (e *commandError) Error() string { return e.code + " " + e.message }

// Unwrap 返回内部 cause（可能为 nil），让 errors.Is/As 沿错误链匹配真实原因。
// 不修改错误，也不向网络公开 cause；错误身份与 Error 的显示文本是不同职责。
func (e *commandError) Unwrap() error { return e.cause }

// RESPError 返回允许公开的 code、message 两个字段，不包括内部 cause，供编码器按白名单输出。
// 不做换行校验或修改错误；EncodeReply 会校验错误码并清除消息 CRLF，服务端仍可使用 Unwrap 追踪原因。
func (e *commandError) RESPError() (string, string) { return e.code, e.message }

var (
	ErrTypeMismatch   = &commandError{code: "WRONGTYPE", message: "Operation against a key holding the wrong kind of value"}
	ErrWrongArgsNum   = errors.New("wrong number of arguments")
	ErrUnknownCmd     = &commandError{code: "ERR", message: "unknown command"}
	ErrInvalidInteger = &commandError{code: "ERR", message: "value is not an integer or out of range"}
)

// wrongArgs 根据小写 command 构造含命令名的参数数量错误，返回实现公开 RESPError 的 error。
// cause 保留 ErrWrongArgsNum 供 errors.Is 判断；不改变引擎状态，不能靠比较消息字符串识别此类错误。
func wrongArgs(command string) error {
	return &commandError{
		code: "ERR", message: fmt.Sprintf("wrong number of arguments for '%s' command", command),
		cause: ErrWrongArgsNum,
	}
}
