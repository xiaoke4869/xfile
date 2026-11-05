//go:build !darwin && !windows

package core

import (
	"os"
	"syscall"
)

func MoveExclusive(from, to string) error {
	if e := os.Link(from, to); e != nil {
		return e
	}
	return os.Remove(from)
}
func CaseSensitive(path string) bool               { return true }
func CrossDevice(err error) bool                   { return err == syscall.EXDEV }
func DiskInfo(path string) (string, uint64, error) { return "unsupported", ^uint64(0), nil }
