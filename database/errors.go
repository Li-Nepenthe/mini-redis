package database

import "errors"

var (
	ErrTypeMismatch = errors.New("数据类型不匹配") // 数据类型不符合预期 (例如对 string 做 LPOP)
	ErrWrongArgsNum = errors.New("参数数量错误")  // 参数个数错误
	ErrUnknownCmd   = errors.New("未知命令")    // 未知指令
)
