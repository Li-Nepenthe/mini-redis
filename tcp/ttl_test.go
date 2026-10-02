package tcp_test

import "testing"

// TestTTLCommandsOnTCP 在真实连接检查缺失/无期限/有期限及 EXPIRE0 删除的 RESP 整数与空 bulk 编码。
// t 逐条比较已知字节，连接和 listener 由测试辅助清理；精确过期时间边界另用数据库固定时钟测试。
func TestTTLCommandsOnTCP(t *testing.T) {
	_, listener, _, _ := startServer(t)
	conn := dial(t, listener.Addr().String())
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"ttl", "key"}, ":-2\r\n"},
		{[]string{"set", "key", "v"}, "+OK\r\n"},
		{[]string{"ttl", "key"}, ":-1\r\n"},
		{[]string{"expire", "key", "5"}, ":1\r\n"},
		{[]string{"ttl", "key"}, ":5\r\n"},
		{[]string{"expire", "key", "0"}, ":1\r\n"},
		{[]string{"get", "key"}, "$-1\r\n"},
		{[]string{"ttl", "key"}, ":-2\r\n"},
	} {
		exchange(t, conn, request(test.args...), test.want)
	}
}
