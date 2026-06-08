//go:build !windows

package store

import (
	"path/filepath"
	"syscall"
)

func diskAvailable(path string) int64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(filepath.Clean(path), &stat); err != nil {
		return 0
	}
	return int64(stat.Bavail) * stat.Bsize
}
