package aof

import (
	"os"
	"syscall"
	"unsafe"
)

var lockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")

func lockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	// 非阻塞独占锁由操作系统在关闭或进程退出时释放，崩溃后不用猜测/删除陈旧锁文件。
	ok, _, err := lockFileEx.Call(file.Fd(), 3, 0, 0xffffffff, 0xffffffff, uintptr(unsafe.Pointer(&overlapped)))
	if ok == 0 {
		return err
	}
	return nil
}
