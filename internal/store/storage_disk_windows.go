//go:build windows

package store

import (
	"path/filepath"
	"syscall"
	"unsafe"
)

func diskAvailable(path string) int64 {
	p := filepath.Clean(path)
	if len(p) >= 2 && p[1] == ':' {
		p = p[:2] + `\`
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")
	var free, total, totalFree uint64
	ptr, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return 0
	}
	r, _, _ := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(ptr)),
		uintptr(unsafe.Pointer(&free)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&totalFree)),
	)
	if r == 0 {
		return 0
	}
	return int64(free)
}
