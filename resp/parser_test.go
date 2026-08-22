package resp

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"testing"
)

func TestParser(parentT *testing.T) {

	// 创建匿名结构体
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
		// 测试没有*前缀
		{
			name:    "拒绝非数组输入",
			input:   "GET\r\n",
			wantErr: true,
		},
		// 测试*后面不是数字
		{
			name:    "拒绝非法数组长度",
			input:   "*x\r\n",
			wantErr: true,
		},
		// 测试$后面不是数字
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
		// 测试参数头并未完整结束的情况
		{
			name:    "拒绝不完整的块字符串头",
			input:   "*1\r\n$3",
			wantErr: true,
		},
		// 参数头完整 但是参数内容不完整
		{
			name:    "拒绝不完整的块字符串内容",
			input:   "*1\r\n$5\r\nabc",
			wantErr: true,
		},
		// 处理ReadFull读满了 但是结尾不是\r\n的情况
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
			// 要把input转为reader 然后传入parseStream函数中
			reader := bytes.NewReader([]byte(tt.input))
			parser := NewRespParser()
			channel := parser.ParseStream(reader)
			var got [][]byte
			var gotErr error
			// 获取到chan后 读取chan的内容
			for payload := range channel {
				if payload.Err != nil {
					//// 如果是传输结束 则ERR为IO.EOF
					//if errors.Is(payload.Err, io.EOF) {
					//	// 此时会进入下一轮循环 但发现channel关闭后 循环结束
					//	continue
					//}

					// 记录所有出现的Err
					gotErr = payload.Err
					continue
				}
				got = payload.Data
			}

			// 先检查实际是否报错以及是否和预期错误一致
			if (gotErr != nil) != tt.wantErr {
				caseT.Fatalf("ParseStream() 返回错误 = %v，是否预期错误 = %v",
					gotErr, tt.wantErr)
			}

			// 如果预期出现err 则直接返回
			if tt.wantErr {
				// 如果data不为空
				if got != nil {
					// 说明parser在遇到错误后还返回了数据
					caseT.Fatalf("ParseStream() 报错后仍然返回了数据：%q", got)
				}
				return
			}

			// 比较是否和预期结果一致
			if !reflect.DeepEqual(got, tt.want) {
				caseT.Fatalf("ParseStream() 返回值 = %q，预期值 = %q", got, tt.want)
			}
		})
	}
}

/**
读取channel的几种方式
方法一： 读取一次
payload := <-channel 这种方法会阻塞 直到channel中出现一个值
payload, ok := <-channel 这里的ok用于判断 channel是否关闭
如果 成功读取 则ok == true
如果 channel已关闭且没有剩余数据 ok == false
if !ok {channel已经关闭}

方法二： 持续读取 直到channel关闭 等价于源源不断执行接收并在channel关闭后退出
适合生产者发送多条数据的场景

for payload := range channel {
	fmt.Println(payload)
}


方法三：使用select 适合同时等待多个 Channel、取消信号或超时。



select {
case value := <-ch1:
	// ch1 可以接收数据时运行

case value := <-ch2:
	// ch2 可以接收数据时运行

case <-ctx.Done():
	// Context 被取消或超时时运行

default:
	// 当前没有任何 Channel 可以操作时立即运行
}

但是select只选择一次 如果要多轮选择 要加for循环

*/

// 测试连续命令
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
			channel := parser.ParseStream(reader)
			var got [][][]byte
			var gotErr error
			for payload := range channel {
				if payload.Err != nil {
					gotErr = payload.Err
					continue
				}
				got = append(got, payload.Data)
			}

			// 检查是否报错以及是否和预期出现错误
			if (gotErr != nil) != tt.wantErr {
				t.Fatalf("ParseStream() 返回错误 = %v，是否预期错误 = %v",
					gotErr, tt.wantErr)
			}

			// 如果预期出现错误 则直接返回
			if tt.wantErr {
				// 出现错误 但是data不为空
				if got != nil {
					// 说明parser在遇到错误后返回了数据
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

// 测试命令不是一次性到达 而是分段到达的情况
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

			// writer写入数据-->reader读取数据-->Parser解析
			// 这里用pipe 是因为可以手动掌握开关
			// 而之前的io.reader再读取内容结束后就会返回EOF 而不是继续等待
			reader, writer := io.Pipe()
			parser := NewRespParser()
			channel := parser.ParseStream(reader)
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
