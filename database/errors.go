package database

import "errors"

var (
	ErrTypeMismatch = errors.New("type mismatch")             // 数据类型不符合预期 (例如对 string 做 LPOP)
	ErrWrongArgsNum = errors.New("wrong number of arguments") // 参数个数错误
	ErrUnknownCmd   = errors.New("unknown command")           // 未知指令
)
