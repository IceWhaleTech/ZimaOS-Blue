//go:build windows

package tts

import (
	"syscall"
)

// loadLibraryWindows loads library on Windows using syscall
func loadLibraryWindows(libPath string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(libPath)
	if err != nil {
		return 0, err
	}
	return uintptr(handle), nil
}

// loadLibraryUnix is not available on Windows
func loadLibraryUnix(libPath string) (uintptr, error) {
	panic("loadLibraryUnix not available on Windows")
}
