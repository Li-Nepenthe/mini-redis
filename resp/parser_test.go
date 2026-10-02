package resp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"testing"
)

// TestParser 用 parentT 的表驱动子测试验证 GET/SET、含 CRLF 内容及非数组/非法长度/残帧/结束符等解析结果。
// 期望错误时不得附带 Data，成功时参数逐字节匹配；bytes.Reader 模型不单独验证真实半包等待。
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
			parser := NewRespParser()
			channel := parser.ParseStream(context.Background(), reader)
			var got [][]byte
			var gotErr error
			for payload := range channel {
				if payload.Err != nil {

					gotErr = payload.Err
					continue
				}
				got = payload.Data
			}

			if (gotErr != nil) != tt.wantErr {
				caseT.Fatalf("ParseStream() 返回错误 = %v，是否预期错误 = %v",
					gotErr, tt.wantErr)
			}

			if tt.wantErr {
				if got != nil {
					caseT.Fatalf("ParseStream() 报错后仍然返回了数据：%q", got)
				}
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				caseT.Fatalf("ParseStream() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}

// TestParserMultipleCommands 将两条请求粘连在同一 reader，验证按顺序得到两份参数而不吞掉后续帧。
// t 收集通道全部 Payload 并比较完整序列，没有网络操作；半包阻塞另由 Pipe 测试覆盖。
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
			parser := NewRespParser()
			channel := parser.ParseStream(context.Background(), reader)
			var got [][][]byte
			var gotErr error
			for payload := range channel {
				if payload.Err != nil {
					gotErr = payload.Err
					continue
				}
				got = append(got, payload.Data)
			}

			if (gotErr != nil) != tt.wantErr {
				t.Fatalf("ParseStream() 返回错误 = %v，是否预期错误 = %v",
					gotErr, tt.wantErr)
			}

			if tt.wantErr {
				if got != nil {
					t.Fatalf("ParseStream() 报错后仍然返回了数据：%q", got)
				}
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseStream() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}

// TestParserFragmentedInput 通过 io.Pipe 分次发送同一 GET 的头和内容，验证 Parser 等待并拼齐正确参数。
// t 管理写端 EOF/错误并读取到通道结束，真实覆盖分段阻塞而非将完整字符串一次性给 Parser。
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
			parser := NewRespParser()
			channel := parser.ParseStream(context.Background(), reader)
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

			var got [][]byte
			for payload := range channel {
				if payload.Err != nil {
					t.Fatalf("分片解析失败：%v", payload.Err)
				}
				got = payload.Data
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseStream() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}
