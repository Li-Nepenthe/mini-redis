package tcp_test

import "testing"

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
