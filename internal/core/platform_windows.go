package core

import (
	"errors"
	"strings"
	"syscall"
	"unsafe"
)

var moveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
var volumePathProc = syscall.NewLazyDLL("kernel32.dll").NewProc("GetVolumePathNameW")
var diskSpaceProc = syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")

func longPath(p string) string {
	if strings.HasPrefix(p, `\\?\`) {
		return p
	}
	if strings.HasPrefix(p, `\\`) {
		return `\\?\UNC\` + p[2:]
	}
	return `\\?\` + p
}
func MoveExclusive(from, to string) error {
	a, e := syscall.UTF16PtrFromString(longPath(from))
	if e != nil {
		return e
	}
	b, e := syscall.UTF16PtrFromString(longPath(to))
	if e != nil {
		return e
	}
	ok, _, err := moveFileEx.Call(uintptr(unsafe.Pointer(a)), uintptr(unsafe.Pointer(b)), 0)
	if ok == 0 {
		return err
	}
	return nil
}

// Conservative for Windows directories with per-directory case sensitivity enabled.
func CaseSensitive(path string) bool { return false }
func CrossDevice(err error) bool     { return errors.Is(err, syscall.Errno(17)) }
func DiskInfo(path string) (string, uint64, error) {
	p, e := existingDirectory(path)
	if e != nil {
		return "", 0, e
	}
	str, e := syscall.UTF16PtrFromString(p)
	if e != nil {
		return "", 0, e
	}
	buf := make([]uint16, 32768)
	ok, _, err := volumePathProc.Call(uintptr(unsafe.Pointer(str)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if ok == 0 {
		return "", 0, err
	}
	volume := syscall.UTF16ToString(buf)
	var available uint64
	ok, _, err = diskSpaceProc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&available)), 0, 0)
	if ok == 0 {
		return "", 0, err
	}
	return strings.ToLower(volume), available, nil
}
