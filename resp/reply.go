package resp

import (
	"fmt"
	"strconv"
)

// 解析exec传来的数据 按照标准格式返回、

func EncodeReply(result any, execErr error) ([]byte, error) {
	// 首先判断错误是否为空
	if execErr != nil {
		//如果错误不为空 则只处理错误 忽略result
		return []byte("-ERR " + execErr.Error() + "\r\n"), nil
	}

	// 错误为空 处理result 由于result返回类型固定 采用Switch Type 匹配
	// switch result.(type)在运行时检查result内部的实际类型 然后选择对应的case
	switch value := result.(type) { // 可以拿到具体的value 然后在case里面使用
	// value只有进入对应的case才有类型 相当于每一层case都做了一次类型断言
	case nil: // 表示命令执行成功 但是没有数据可返回 如Get或者LPop不存在的key
		return []byte("$-1\r\n"), nil
	case []byte: //返回数据 但是要追加长度
		// 下面的对于value来说 会产生 []byte -> string -> byte的转换
		// return []byte("$" + strconv.Itoa(len(value)) + "\r\n" + string(value) + "\r\n"), nil
		// 推荐使用append追加
		header := "$" + strconv.Itoa(len(value)) + "\r\n"
		reply := make([]byte, 0, len(header)+len(value)+2)
		reply = append(reply, header...) // ...表示把header字符串按照每个字节展开 逐个相加
		reply = append(reply, value...)  // 把value中的每个byte逐个追加到reply中
		reply = append(reply, '\r', '\n')
		return reply, nil
	case bool: // value 的类型是 bool 只有SET会返回 且一般情况是为true
		if !value {
			return nil, fmt.Errorf("unexpected boolean result")
		}
		return []byte("+OK\r\n"), nil
	case int: // LPUSH 返回的是 int 表示
		return []byte(":" + strconv.Itoa(value) + "\r\n"), nil
	default:
		// 不支持的类型
		return nil, fmt.Errorf("unexpected type: %T", value)
	}
}
