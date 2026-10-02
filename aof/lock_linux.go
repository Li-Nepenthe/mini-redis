package aof

import (
	"os"
	"syscall"
)

// lockFile 对已打开的 file 获取 Linux 非阻塞独占 flock，返回系统锁错误或 nil。
// 锁属于文件句柄，关闭或进程退出后由系统释放；竞争失败直接报错，避免两个进程同时追加或修复同一 AOF。
func lockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
