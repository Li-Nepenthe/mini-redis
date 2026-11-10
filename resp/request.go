// 本文件：AppendRequest 为 AOF 和客户端编码 RESP 请求，在分配内容前检查协议预算。

package resp

import (
	"errors"
	"strconv"
)

// AppendRequest 在 dst 后追加一条请求；预算只计算这条请求，不把前缀算进去。
// 出错时返回 nil，调用者必须丢弃此次结果；命令含义仍由引擎检查。
func AppendRequest(dst []byte, args [][]byte) ([]byte, error) {
	if len(args) == 0 || len(args) > MaxArrayLength {
		return nil, errors.New("invalid request argument count")
	}
	start := len(dst)
	data := append(dst, '*')
	data = strconv.AppendInt(data, int64(len(args)), 10)
	data = append(data, '\r', '\n')
	for _, arg := range args {
		if len(arg) > MaxBulkLength {
			return nil, errors.New("request bulk exceeds protocol limit")
		}
		data = append(data, '$')
		data = strconv.AppendInt(data, int64(len(arg)), 10)
		data = append(data, '\r', '\n')
		if len(arg)+2 > MaxRequestLength-(len(data)-start) {
			return nil, errors.New("request exceeds protocol limit")
		}
		data = append(data, arg...)
		data = append(data, '\r', '\n')
	}
	return data, nil
}
