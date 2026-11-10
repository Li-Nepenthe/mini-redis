// 本文件：用字节输入固定各类回复的显示结果，重点验证数组后紧跟下一条回复时不会错位。

package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Li-Nepenthe/mini-redis/resp"
)

func TestReadReply(t *testing.T) {
	for _, test := range []struct{ name, input, want string }{
		{"status", "+OK\r\n", "OK"},
		{"error", "-WRONGTYPE wrong type\r\n", "(error) WRONGTYPE wrong type"},
		{"integer", ":3\r\n", "(integer) 3"},
		{"negative integer", ":-2\r\n", "(integer) -2"},
		{"int64 maximum", ":9223372036854775807\r\n", "(integer) 9223372036854775807"},
		{"bulk", "$1\r\na\r\n", `"a"`},
		{"binary bulk", "$4\r\na\r\n\x00\r\n", `"a\r\n\x00"`},
		{"quoted bulk", "$2\r\n\"\\\r\n", `"\"\\"`},
		{"empty bulk", "$0\r\n\r\n", `""`},
		{"null bulk", "$-1\r\n", "(nil)"},
		{"null array", "*-1\r\n", "(nil)"},
		{"empty array", "*0\r\n", "(empty array)"},
		{"array", "*2\r\n$1\r\na\r\n:3\r\n", "1) \"a\"\n2) (integer) 3"},
		{"nested array", "*2\r\n:1\r\n*2\r\n$1\r\na\r\n$-1\r\n", "1) (integer) 1\n2) 1) \"a\"\n   2) (nil)"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(test.input))
			got, err := readReply(reader)
			if err != nil || got != test.want {
				t.Fatalf("got=%q err=%v want=%q", got, err, test.want)
			}
			if _, err := readReply(reader); err != io.EOF {
				t.Fatalf("reply left bytes behind: %v", err)
			}
		})
	}
}

func TestReadReplyArrayThenNextReply(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("*2\r\n$1\r\na\r\n$1\r\nb\r\n+PONG\r\n$-1\r\n"))
	for _, want := range []string{"1) \"a\"\n2) \"b\"", "PONG", "(nil)"} {
		got, err := readReply(reader)
		if err != nil || got != want {
			t.Fatalf("got=%q err=%v want=%q", got, err, want)
		}
	}
}

func TestReadReplyRejectsMalformed(t *testing.T) {
	for _, input := range []string{
		"+OK\n", "+unfinished", ":x\r\n", ":9223372036854775808\r\n", "$x\r\n", "$-2\r\n", "*-2\r\n",
		fmt.Sprintf("$%d\r\n", resp.MaxBulkLength+1), "$3\r\nab", "$1\r\naXX", "*2\r\n+OK\r\n", "*1\r\n!bad\r\n", "?unknown\r\n",
	} {
		if got, err := readReply(bufio.NewReader(strings.NewReader(input))); err == nil || got != "" {
			t.Fatalf("accepted %q: %q, %v", input, got, err)
		}
	}
}
