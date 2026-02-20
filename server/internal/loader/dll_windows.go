package loader

import (
	"syscall"
	"unsafe"
)

var setDllDirectoryW = syscall.NewLazyDLL("kernel32.dll").NewProc("SetDllDirectoryW")

func setDllSearchDir(dir string) {
	p, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return
	}
	setDllDirectoryW.Call(uintptr(unsafe.Pointer(p)))
}
