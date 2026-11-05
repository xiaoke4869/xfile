package core

/*
#include <stdio.h>
#include <errno.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/attr.h>
#include <sys/mount.h>
static int xfile_move(const char *from, const char *to) {
 if (renamex_np(from, to, RENAME_EXCL) == 0) return 0;
 return errno;
}
static int xfile_case(const char *path) {
 struct attrlist attrs = {0};
 attrs.bitmapcount = ATTR_BIT_MAP_COUNT;
 attrs.volattr = ATTR_VOL_INFO | ATTR_VOL_CAPABILITIES;
 struct { uint32_t length; vol_capabilities_attr_t caps; } result;
 if (getattrlist(path, &attrs, &result, sizeof(result), 0) != 0) return 0;
 return (result.caps.valid[VOL_CAPABILITIES_FORMAT] & VOL_CAP_FMT_CASE_SENSITIVE) &&
        (result.caps.capabilities[VOL_CAPABILITIES_FORMAT] & VOL_CAP_FMT_CASE_SENSITIVE);
}
static int xfile_disk(const char *path, unsigned long long *space, unsigned long long *volume) {
 struct statfs st;
 if (statfs(path, &st) != 0) return errno;
 *space = (unsigned long long)st.f_bavail * st.f_bsize;
 *volume = ((unsigned long long)(unsigned int)st.f_fsid.val[0] << 32) | (unsigned int)st.f_fsid.val[1];
 return 0;
}
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func MoveExclusive(from, to string) error {
	a, b := C.CString(from), C.CString(to)
	defer C.free(unsafe.Pointer(a))
	defer C.free(unsafe.Pointer(b))
	if code := C.xfile_move(a, b); code != 0 {
		return syscall.Errno(code)
	}
	return nil
}
func CaseSensitive(path string) bool {
	for {
		if _, e := os.Stat(path); e == nil {
			break
		}
		next := filepath.Dir(path)
		if next == path {
			return false
		}
		path = next
	}
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	return C.xfile_case(p) != 0
}
func CrossDevice(err error) bool { return err == syscall.EXDEV }
func DiskInfo(path string) (string, uint64, error) {
	p, e := existingDirectory(path)
	if e != nil {
		return "", 0, e
	}
	s := C.CString(p)
	defer C.free(unsafe.Pointer(s))
	var space, volume C.ulonglong
	if code := C.xfile_disk(s, &space, &volume); code != 0 {
		return "", 0, syscall.Errno(code)
	}
	return fmt.Sprint(uint64(volume)), uint64(space), nil
}
