package aof

import (
	"context"
	"errors"
	"fmt"
	"github.com/Li-Nepenthe/mini-redis/resp"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
)

type logFile interface {
	// Writer 的 Write 允许短写/部分写伴错误，由 Store 循环或触发确认前缀回滚。
	io.Writer
	// Sync 返回 nil 才表示此轮同步成功；故障不能被缓存的成功状态掩盖。
	Sync() error
	// Truncate 将文件长度缩至确认前缀；失败由 Store.fail 合并并持续拒写。
	Truncate(int64) error
	// Close 释放文件句柄及系统独占锁，关闭错误必须传给上层。
	Close() error
}

type Store struct {
	mu                 sync.Mutex
	file               logFile
	size               int64
	closed             bool
	failed             error
	RecoveredTailBytes int64
}

// Open 创建或打开 path 指定的普通 AOF 文件、独占加锁，并依次把完整记录交给 replay 恢复内存。
// ctx 用于取消恢复；replay 不能为 nil，必须能处理私有持久化命令。成功返回由调用者 Close 的 Store，失败返回错误并回收文件。
// 仅 EOF/UnexpectedEOF 导致的不完整尾部会截到最后完整记录并 Sync；完整坏帧/回放错误保留文件并拒绝启动。
// 偏移取 Parser 实际消费字节，不能用 bufio 预读游标或重新编码长度。失败前 replay 可能已改变内存，调用者应丢弃该恢复引擎。
func Open(ctx context.Context, path string, replay func([][]byte) error) (store *Store, err error) {
	if replay == nil {
		return nil, errors.New("AOF replay callback is required")
	}
	// Windows 追加模式的文件句柄不能截断；用文件独占锁和 Store.mu 保证单写者，
	// 重放后定位一次再顺序写，才能安全修复半条尾部。
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open AOF: %w", err)
	}
	owned := true
	defer func() {
		if owned {
			err = errors.Join(err, file.Close())
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("AOF must be a regular file")
	}
	if err := lockFile(file); err != nil {
		return nil, fmt.Errorf("lock AOF: %w", err)
	}
	parseCtx, cancel := context.WithCancel(ctx)
	ch := resp.NewRespParser().ParseStream(parseCtx, file)
	// 该收尾闭包在文件最终移交/回收前取消生产者并等待通道，防止回放提前报错留下 goroutine。
	defer func() {
		cancel()
		if err != nil {
			_ = file.Close()
			owned = false
		}
		for range ch {
		}
	}()
	var offset, recovered int64
	for payload := range ch {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if payload.Err != nil {
			if !errors.Is(payload.Err, io.EOF) && !errors.Is(payload.Err, io.ErrUnexpectedEOF) {
				return nil, fmt.Errorf("invalid AOF at byte %d: %w", offset, payload.Err)
			}
			// 只恢复不完整尾部；完整坏记录不能自动截断，否则可能静默丢掉已确认数据。
			if err := file.Truncate(offset); err != nil {
				return nil, fmt.Errorf("truncate AOF tail: %w", err)
			}
			if err := file.Sync(); err != nil {
				return nil, fmt.Errorf("sync AOF recovery: %w", err)
			}
			recovered = info.Size() - offset
			break
		}
		if err := replay(payload.Data); err != nil {
			return nil, fmt.Errorf("replay AOF at byte %d: %w", offset, err)
		}
		offset += int64(payload.BytesRead)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek AOF append position: %w", err)
	}
	owned = false
	return &Store{file: file, size: offset, RecoveredTailBytes: recovered}, nil
}

// encodeRequest 把 args 的每个二进制参数编码为 RESP 数组中的 bulk，返回独立记录字节或边界错误。
// 检查参数数目、单 bulk 与整条记录预算，不修改 args，也不写文件；不验证业务命令含义，业务校验由 Engine 完成。
func encodeRequest(args [][]byte) ([]byte, error) {
	if len(args) == 0 || len(args) > resp.MaxArrayLength {
		return nil, errors.New("invalid AOF argument count")
	}
	data := []byte("*" + strconv.Itoa(len(args)) + "\r\n")
	for _, arg := range args {
		if len(arg) > resp.MaxBulkLength {
			return nil, errors.New("AOF bulk exceeds protocol limit")
		}
		data = append(data, '$')
		data = strconv.AppendInt(data, int64(len(arg)), 10)
		data = append(data, '\r', '\n')
		if len(arg)+2 > resp.MaxRequestLength-len(data) {
			return nil, errors.New("AOF record exceeds protocol limit")
		}
		data = append(data, arg...)
		data = append(data, '\r', '\n')
	}
	return data, nil
}

// Append 将 args 编码后完整写入 AOF 并 Sync；nil 表示该记录已经通过本次同步确认。
// 与 Close 共用 Store.mu，size 仅在完整写入且同步成功后增加，表示上次确认前缀而非当前文件长度。
// 关闭后返回 os.ErrClosed；已有存储失败持续返回原失败；编码错误不写文件。写入/同步失败触发尽力回滚并粘住失败状态。
func (s *Store) Append(args [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	if s.failed != nil {
		return s.failed
	}
	data, err := encodeRequest(args)
	if err != nil {
		return err
	}
	size := len(data)
	for len(data) > 0 {
		n, err := s.file.Write(data)
		if err != nil {
			return s.fail(err)
		}
		if n == 0 {
			return s.fail(io.ErrShortWrite)
		}
		data = data[n:]
	}
	if err := s.file.Sync(); err != nil {
		return s.fail(err)
	}
	s.size += int64(size)
	return nil
}

// fail 在调用者持有 Store.mu 时处理 cause：尽力 Truncate 到确认的 size 并再次 Sync，保存合并错误并写内部日志。
// 返回包含原始原因及回滚失败原因的错误；即使回滚成功也持续拒绝后续 Append，不能把一次存储故障掩盖为已自动恢复。
func (s *Store) fail(cause error) error {
	// 尽力回滚到上次确认的前缀；即使回滚成功也持续拒绝写入，避免掩盖存储故障。
	truncateErr := s.file.Truncate(s.size)
	syncErr := s.file.Sync()
	s.failed = fmt.Errorf("append AOF: %w", errors.Join(cause, truncateErr, syncErr))
	log.Printf("AOF persistence failure: %v", s.failed)
	return s.failed
}

// Close 在 Store.mu 下将存储标记关闭、必要时 Sync 并关闭文件，返回持久化/关闭的合并错误。
// 重复调用不再操作文件而返回已保存的错误；先前 Append 失败仍会关闭句柄并保留原因，不能用 Close 清除失败状态。
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.failed
	}
	s.closed = true
	if s.failed == nil {
		s.failed = s.file.Sync()
	}
	s.failed = errors.Join(s.failed, s.file.Close())
	return s.failed
}
