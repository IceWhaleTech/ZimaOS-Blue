//go:build windows

package providerpool

import "syscall"

// setWindowsHiddenSystem sets FILE_ATTRIBUTE_HIDDEN and FILE_ATTRIBUTE_SYSTEM on Windows.
func setWindowsHiddenSystem(path string) error {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	attrs, err := syscall.GetFileAttributes(pathPtr)
	if err != nil {
		return err
	}
	attrs |= syscall.FILE_ATTRIBUTE_HIDDEN | syscall.FILE_ATTRIBUTE_SYSTEM
	return syscall.SetFileAttributes(pathPtr, attrs)
}
