// 本文件：用固定输入和分段 Pipe 检查 ReadRequest 的参数结果、错误和连续读取。

package resp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"reflect"
	"testing"
)

// 保留各类合法、越界和损坏输入的预期结果，报错时不能同时交出部分参数。
func TestParser(parentT *testing.T) {

	tests := []struct {
		name    string
		input   string
		want    [][]byte
		wantErr bool
	}{
		{
			name:  "解析 GET 命令",
			input: "*2\r\n$3\r\nGET\r\n$4\r\nname\r\n",
			want: [][]byte{
				[]byte("GET"),
				[]byte("name"),
			},
		},
		{
			name:  "解析 SET 命令",
			input: "*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$5\r\nAlice\r\n",
			want: [][]byte{
				[]byte("SET"),
				[]byte("name"),
				[]byte("Alice"),
			},
		},
		{
			name:    "拒绝非数组输入",
			input:   "GET\r\n",
			wantErr: true,
		},
		{
			name:    "拒绝非法数组长度",
			input:   "*x\r\n",
			wantErr: true,
		},
		{
			name:    "拒绝非法块字符串长度",
			input:   "*1\r\n$x\r\n",
			wantErr: true,
		},
		{
			name:    "拒绝非块字符串参数",
			input:   "*1\r\n!3\r\nabc\r\n",
			wantErr: true,
		},
		{
			name:    "拒绝不完整的块字符串头",
			input:   "*1\r\n$3",
			wantErr: true,
		},
		{
			name:    "拒绝不完整的块字符串内容",
			input:   "*1\r\n$5\r\nabc",
			wantErr: true,
		},
		{
			name:    "拒绝非法的块字符串结束符",
			input:   "*1\r\n$5\r\nabcdeXY",
			wantErr: true,
		},
		{
			name:  "解析包含 CRLF 的块字符串",
			input: "*1\r\n$12\r\nhello\r\nworld\r\n",
			want: [][]byte{
				[]byte("hello\r\nworld"),
			},
		},
		{
			name:    "拒绝负数组长度",
			input:   "*-1\r\n",
			wantErr: true,
		},
		{
			name:    "拒绝超大数组",
			input:   fmt.Sprintf("*%d\r\n", MaxArrayLength+1),
			wantErr: true,
		},
		{
			name:    "拒绝负块字符串长度",
			input:   "*1\r\n$-1\r\n",
			wantErr: true,
		},
		{
			name:    "拒绝超大块字符串",
			input:   fmt.Sprintf("*1\r\n$%d\r\n", MaxBulkLength+1),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		parentT.Run(tt.name, func(caseT *testing.T) {
			reader := bytes.NewReader([]byte(tt.input))
			got, _, gotErr := ReadRequest(bufio.NewReader(reader))

			if (gotErr != nil) != tt.wantErr {
				caseT.Fatalf("ReadRequest() 返回错误 = %v，是否预期错误 = %v",
					gotErr, tt.wantErr)
			}

			if tt.wantErr {
				if got != nil {
					caseT.Fatalf("ReadRequest() 报错后仍然返回了数据：%q", got)
				}
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				caseT.Fatalf("ReadRequest() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}

// 连续读两条粘连请求，确保读缓冲中的后续帧不会丢失。
func TestParserMultipleCommands(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [][][]byte
		wantErr bool
	}{
		{
			name: "测试多条输入",
			input: "*2\r\n$3\r\nGET\r\n$4\r\nname\r\n" +
				"*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$5\r\nAlice\r\n",
			want: [][][]byte{
				{
					[]byte("GET"),
					[]byte("name"),
				},
				{
					[]byte("SET"),
					[]byte("name"),
					[]byte("Alice"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader([]byte(tt.input))
			buffer := bufio.NewReader(reader)
			var got [][][]byte
			var gotErr error
			for {
				args, _, err := ReadRequest(buffer)
				if err == io.EOF {
					break
				}
				if err != nil {
					gotErr = err
					break
				}
				got = append(got, args)
			}

			if (gotErr != nil) != tt.wantErr {
				t.Fatalf("ReadRequest() 返回错误 = %v，是否预期错误 = %v",
					gotErr, tt.wantErr)
			}

			if tt.wantErr {
				if got != nil {
					t.Fatalf("ReadRequest() 报错后仍然返回了数据：%q", got)
				}
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ReadRequest() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}

// 用 Pipe 分段写入，确保 ReadRequest 会等待完整请求，且下一次读取能识别干净 EOF。
func TestParserFragmentedInput(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   [][]byte
	}{
		{
			name: "分片解析 GET 命令",
			chunks: []string{
				"*2\r\n$3\r\nG",
				"ET\r\n$4\r\nna",
				"me\r\n",
			},
			want: [][]byte{
				[]byte("GET"),
				[]byte("name"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Pipe 允许分段发送并阻塞读取，才能覆盖真正的半包等待。
			reader, writer := io.Pipe()
			defer reader.Close()
			go func() {
				for _, chunk := range tt.chunks {
					_, err := writer.Write([]byte(chunk))
					if err != nil {
						_ = writer.CloseWithError(err)
						return
					}
				}
				_ = writer.Close() // 手动关闭后触发EOF
			}()

			buffer := bufio.NewReader(reader)
			got, _, err := ReadRequest(buffer)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := ReadRequest(buffer); err != io.EOF {
				t.Fatalf("after fragmented frame: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ReadRequest() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}
