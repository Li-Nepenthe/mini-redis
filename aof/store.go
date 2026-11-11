// 本文件：Open 同步恢复 AOF 并修复半条尾部，Append 负责写入确认与失败回滚，Close 释放文件和锁。

package aof

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/Li-Nepenthe/mini-redis/resp"
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

// Open 取得独占锁后同步恢复记录，失败时关闭文件，成功时把句柄交给 Store。
// 只有 EOF 导致的残帧可以截断；完整坏记录必须拒绝启动，避免静默丢掉已经确认的数据。
// 恢复中途失败时内存可能已有部分记录，调用者应丢弃这个引擎。
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
	defer func() {
		if err != nil {
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
	reader := bufio.NewReader(file)
	var offset, recovered int64
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		args, size, readErr := resp.ReadRequest(reader)
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
				return nil, fmt.Errorf("invalid AOF at byte %d: %w", offset, readErr)
			}
			if err := file.Truncate(offset); err != nil {
				return nil, fmt.Errorf("truncate AOF tail: %w", err)
			}
			if err := file.Sync(); err != nil {
				return nil, fmt.Errorf("sync AOF recovery: %w", err)
			}
			recovered = info.Size() - offset
			break
		}
		if err := replay(args); err != nil {
			return nil, fmt.Errorf("replay AOF at byte %d: %w", offset, err)
		}
		offset += int64(size)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek AOF append position: %w", err)
	}
	return &Store{file: file, size: offset, RecoveredTailBytes: recovered}, nil
}

// Append 完整写入并刷盘后才推进确认位置，供引擎决定是否修改内存。
// 写入或刷盘失败后持续拒写；编码错误发生在文件操作前，不影响已确认的数据。
func (s *Store) Append(args [][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	if s.failed != nil {
		return s.failed
	}
	data, err := resp.AppendRequest(nil, args)
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

// 故障后尽力回到最后确认的位置，并保留原始错误和回滚错误。
// 即使回滚成功也不能当作自动恢复，否则后续写入会掩盖磁盘故障；调用者须持 Store.mu。
func (s *Store) fail(cause error) error {
	truncateErr := s.file.Truncate(s.size)
	syncErr := s.file.Sync()
	s.failed = fmt.Errorf("append AOF: %w", errors.Join(cause, truncateErr, syncErr))
	log.Printf("AOF persistence failure: %v", s.failed)
	return s.failed
}

// 关闭与追加共享一把锁，保证最后一次刷盘不会和写入交错；重复关闭仍返回已保存的故障。
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
