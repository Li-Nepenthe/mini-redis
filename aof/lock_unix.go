// 本文件：lockFile 为 Linux 和 macOS 的 AOF 取得非阻塞独占文件锁。

//go:build linux || darwin

package aof

import (
	"os"

	"golang.org/x/sys/unix"
)

// 竞争失败直接拒绝启动，避免两个进程同时修复或追加同一个文件；锁随句柄关闭或进程退出释放。
func lockFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}
