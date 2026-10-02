package tcp_test

import (
	"strconv"
	"strings"
	"testing"
)

// request 将 parts 按字节长度编码为 RESP 数组请求并返回字符串，供 TCP 测试构造二进制/错误命令。
// 不连接服务器或验证业务含义；字符串长度为字节数，内容可含零字节。
func request(parts ...string) string {
	var data strings.Builder
	data.WriteString("*" + strconv.Itoa(len(parts)) + "\r\n")
	for _, part := range parts {
		data.WriteString("$" + strconv.Itoa(len(part)) + "\r\n" + part + "\r\n")
	}
	return data.String()
}

// TestTwentyCommandsOnOneConnection 在一个真实 TCP 连接顺序执行 20 条命令，检查正常值、错误、List 和缺失值的准确回复。
// t 同时验证业务错误不会断连及回复边界持续同步；独立临时 listener/连接由测试清理。
func TestTwentyCommandsOnOneConnection(t *testing.T) {
	_, listener, _, _ := startServer(t)
	conn := dial(t, listener.Addr().String())
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"ping"}, "+PONG\r\n"},
		{[]string{"PING", "hi\x00"}, "$3\r\nhi\x00\r\n"},
		{[]string{"set", "foo", "bar"}, "+OK\r\n"},
		{[]string{"get", "foo"}, "$3\r\nbar\r\n"},
		{[]string{"exists", "foo", "foo", "missing"}, ":2\r\n"},
		{[]string{"lpush", "mylist", "a", "b", "c"}, ":3\r\n"},
		{[]string{"lrange", "mylist", "0", "-1"}, "*3\r\n$1\r\nc\r\n$1\r\nb\r\n$1\r\na\r\n"},
		{[]string{"lrange", "mylist", "-2", "-1"}, "*2\r\n$1\r\nb\r\n$1\r\na\r\n"},
		{[]string{"lpop", "foo"}, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{[]string{"get", "mylist"}, "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"},
		{[]string{"set", "foo"}, "-ERR wrong number of arguments for 'set' command\r\n"},
		{[]string{"lrange", "mylist", "bad", "-1"}, "-ERR value is not an integer or out of range\r\n"},
		{[]string{"lpop", "mylist"}, "$1\r\nc\r\n"},
		{[]string{"lpop", "mylist"}, "$1\r\nb\r\n"},
		{[]string{"lpop", "mylist"}, "$1\r\na\r\n"},
		{[]string{"exists", "mylist"}, ":0\r\n"},
		{[]string{"lrange", "mylist", "0", "-1"}, "*0\r\n"},
		{[]string{"unknown\r\ncommand"}, "-ERR unknown command\r\n"},
		{[]string{"del", "foo", "foo", "missing"}, ":1\r\n"},
		{[]string{"get", "foo"}, "$-1\r\n"},
	}
	for _, test := range tests {
		exchange(t, conn, request(test.args...), test.want)
	}
	t.Log("20 commands succeeded on one TCP connection, including errors without disconnect")
}
