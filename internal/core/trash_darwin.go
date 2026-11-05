package core

/*
#cgo LDFLAGS: -framework Foundation
#include <stdlib.h>
char *xfile_trash(const char *path);
*/
import "C"
import (
	"errors"
	"unsafe"
)

func TrashEntry(path string) error {
	p := C.CString(path)
	defer C.free(unsafe.Pointer(p))
	message := C.xfile_trash(p)
	if message == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(message))
	return errors.New(C.GoString(message))
}
