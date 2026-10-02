package database

import (
	"errors"
	"fmt"
)

type commandError struct {
	code, message string
	cause         error
}

func (e *commandError) Error() string { return e.code + " " + e.message }
func (e *commandError) Unwrap() error { return e.cause }

// 公开字段与内部错误链分开，客户端只能看到稳定业务错误，服务端仍可 errors.Is。
func (e *commandError) RESPError() (string, string) { return e.code, e.message }

var (
	ErrTypeMismatch   = &commandError{code: "WRONGTYPE", message: "Operation against a key holding the wrong kind of value"}
	ErrWrongArgsNum   = errors.New("wrong number of arguments")
	ErrUnknownCmd     = &commandError{code: "ERR", message: "unknown command"}
	ErrInvalidInteger = &commandError{code: "ERR", message: "value is not an integer or out of range"}
)

func wrongArgs(command string) error {
	return &commandError{
		code: "ERR", message: fmt.Sprintf("wrong number of arguments for '%s' command", command),
		cause: ErrWrongArgsNum,
	}
}
