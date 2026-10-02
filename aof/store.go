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
	io.Writer
	Sync() error
	Truncate(int64) error
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

// 文件句柄由 Open 在失败时回收，成功后交给 Store；独占锁防止第二个进程同时追加/截断。
// 重放偏移取 Parser 实际消费量：bufio 会预读，重新编码又会改变合法的非规范数字头长度。
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

// size 只在完整写入且 Sync 成功后前移，作为“上次确认前缀”；不是文件当前可见长度。
// 与 Close 同锁，防止关闭句柄或刷盘失败期间另一写入绕过确认边界。
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

func (s *Store) fail(cause error) error {
	// 尽力回滚到上次确认的前缀；即使回滚成功也持续拒绝写入，避免掩盖存储故障。
	truncateErr := s.file.Truncate(s.size)
	syncErr := s.file.Sync()
	s.failed = fmt.Errorf("append AOF: %w", errors.Join(cause, truncateErr, syncErr))
	log.Printf("AOF persistence failure: %v", s.failed)
	return s.failed
}

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
