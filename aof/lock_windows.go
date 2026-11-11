// 本文件：lockFile 为 Windows 的 AOF 取得非阻塞独占文件锁，句柄关闭时由系统释放。

package aof

import (
	"math"
	"os"

	"golang.org/x/sys/windows"
)

// 从偏移零锁到最大范围，连将来追加的字节也受保护，不能只锁启动时已有的内容。
func lockFile(file *os.File) error {
	var overlapped windows.Overlapped
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY)
	return windows.LockFileEx(windows.Handle(file.Fd()), flags, 0, math.MaxUint32, math.MaxUint32, &overlapped)
}
