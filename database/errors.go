// 本文件：commandError 将客户端可见消息与内部原因分开，wrongArgs 保留可供 errors.Is 判断的错误身份。

package database

import (
	"errors"
	"fmt"
)

type commandError struct {
	code, message string
	cause         error
}

// 公开文本只包含安全的错误码和消息，内部原因留在错误链里。
func (e *commandError) Error() string { return e.code + " " + e.message }

// 调用者仍可沿错误链定位日志等内部故障，不必把原因暴露给客户端。
func (e *commandError) Unwrap() error { return e.cause }

// 协议层只取这两个公开字段，再统一校验换行和错误码。
func (e *commandError) RESPError() (string, string) { return e.code, e.message }

var (
	ErrTypeMismatch   = &commandError{code: "WRONGTYPE", message: "Operation against a key holding the wrong kind of value"}
	ErrWrongArgsNum   = errors.New("wrong number of arguments")
	ErrUnknownCmd     = &commandError{code: "ERR", message: "unknown command"}
	ErrInvalidInteger = &commandError{code: "ERR", message: "value is not an integer or out of range"}
)

// 消息带上命令名便于诊断，错误链保留共同身份，调用者不必比较文本。
func wrongArgs(command string) error {
	return &commandError{
		code: "ERR", message: fmt.Sprintf("wrong number of arguments for '%s' command", command),
		cause: ErrWrongArgsNum,
	}
}
