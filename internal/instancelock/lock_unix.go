//go:build !windows

// Package instancelock 保证同一数据目录只有一个部署控制器，进程退出由内核释放锁。
package instancelock

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Acquire 取得数据目录的独占进程锁，不用 PID 文件判断存活。
func Acquire(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("another controller owns this data directory")
	}
	return func() { _ = unix.Flock(int(file.Fd()), unix.LOCK_UN); _ = file.Close() }, nil
}
