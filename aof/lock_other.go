// 本文件：lockFile 为尚未支持文件锁的平台返回明确错误，让未启用 AOF 的服务仍可使用。

//go:build !linux && !darwin && !windows

package aof

import (
	"fmt"
	"os"
	"runtime"
)

// 无法保证单写者时拒绝开启 AOF，避免用无效的锁给恢复和追加提供错误保证。
func lockFile(_ *os.File) error {
	return fmt.Errorf("AOF file locking is not supported on %s", runtime.GOOS)
}
