package core

import (
	"errors"
	"runtime"
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
)

// IFileOperation uses the shell's recycle operation; there is no permanent-delete fallback.
func TrashEntry(path string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if e, ok := err.(*ole.OleError); !ok || e.Code() != 1 {
			return err
		}
	}
	defer ole.CoUninitialize()
	operation, err := ole.CreateInstance(ole.NewGUID("{3AD05575-8857-4850-9277-11B85BDB8E09}"), ole.NewGUID("{947AAB5F-0A5C-4C13-B4D6-4BF7836FC9F8}"))
	if err != nil {
		return err
	}
	defer operation.Release()
	// FOF_SILENT | FOF_NOCONFIRMATION | FOF_ALLOWUNDO | FOF_NOERRORUI |
	// FOF_WANTNUKEWARNING | FOFX_RECYCLEONDELETE | FOFX_ADDUNDORECORD.
	if err = shellOperationCall(operation, 5, 0x4|0x10|0x40|0x400|0x4000|0x80000|0x20000000); err != nil {
		return err
	}
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var item *ole.IUnknown
	iid := ole.NewGUID("{43826D1E-E718-42EE-BC55-A1E261C37BFE}")
	proc := syscall.NewLazyDLL("shell32.dll").NewProc("SHCreateItemFromParsingName")
	hr, _, _ := proc.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&item)))
	if int32(hr) < 0 {
		return ole.NewError(hr)
	}
	defer item.Release()
	if err = shellOperationCall(operation, 18, uintptr(unsafe.Pointer(item)), 0); err != nil {
		return err
	}
	if err = shellOperationCall(operation, 21); err != nil {
		return err
	}
	var aborted int32
	if err = shellOperationCall(operation, 22, uintptr(unsafe.Pointer(&aborted))); err != nil {
		return err
	}
	if aborted != 0 {
		return errors.New("回收操作未完成，来源已保留")
	}
	return nil
}
func shellOperationCall(object *ole.IUnknown, index int, args ...uintptr) error {
	table := (*[23]uintptr)(unsafe.Pointer(object.RawVTable))
	arguments := append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)
	hr, _, _ := syscall.SyscallN(table[index], arguments...)
	if int32(hr) < 0 {
		return ole.NewError(hr)
	}
	return nil
}
