//go:build windows

package instancelock

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// Acquire 取得数据目录的独占进程锁，不用 PID 文件判断存活。
func Acquire(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	overlap := &windows.Overlapped{}
	if err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, overlap); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("another controller owns this data directory")
	}
	return func() { _ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, overlap); _ = file.Close() }, nil
}
