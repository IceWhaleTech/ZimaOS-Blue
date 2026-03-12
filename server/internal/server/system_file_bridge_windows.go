//go:build windows

package server

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	coinitApartmentThreaded uintptr = 0x2
	hResultChangedMode      uintptr = 0x80010106
)

var (
	modOle32               = windows.NewLazySystemDLL("ole32.dll")
	procCoInitializeEx     = modOle32.NewProc("CoInitializeEx")
	procCoUninitialize     = modOle32.NewProc("CoUninitialize")
	procCoTaskMemFree      = modOle32.NewProc("CoTaskMemFree")
	modShell32             = windows.NewLazySystemDLL("shell32.dll")
	procSHParseDisplayName = modShell32.NewProc("SHParseDisplayName")
	procSHOpenFolderSelect = modShell32.NewProc("SHOpenFolderAndSelectItems")
)

func revealPathWindows(path string, isDir bool) error {
	shouldUninit, err := coInitializeCOM()
	if err != nil {
		return err
	}
	if shouldUninit {
		defer procCoUninitialize.Call()
	}

	itemPIDL, err := shParseDisplayName(path)
	if err != nil {
		return err
	}
	defer coTaskMemFree(itemPIDL)

	if isDir {
		return shOpenFolderAndSelectItems(itemPIDL, 0, 0)
	}

	parent := filepath.Dir(path)
	folderPIDL, err := shParseDisplayName(parent)
	if err != nil {
		return err
	}
	defer coTaskMemFree(folderPIDL)

	selection := [1]uintptr{itemPIDL}
	return shOpenFolderAndSelectItems(folderPIDL, 1, uintptr(unsafe.Pointer(&selection[0])))
}

func coInitializeCOM() (bool, error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	if hr == 0 || hr == 1 { // S_OK / S_FALSE
		return true, nil
	}
	if hr == hResultChangedMode {
		return false, nil
	}
	return false, fmt.Errorf("failed to initialize COM: 0x%08X", uint32(hr))
}

func shParseDisplayName(path string) (uintptr, error) {
	widePath, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, fmt.Errorf("invalid path: %w", err)
	}

	var pidl uintptr
	hr, _, _ := procSHParseDisplayName.Call(
		uintptr(unsafe.Pointer(widePath)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		0,
	)
	if hResultFailed(hr) || pidl == 0 {
		return 0, fmt.Errorf("failed to parse path: 0x%08X", uint32(hr))
	}
	return pidl, nil
}

func shOpenFolderAndSelectItems(folderPIDL uintptr, count uintptr, selectedPIDLs uintptr) error {
	hr, _, _ := procSHOpenFolderSelect.Call(folderPIDL, count, selectedPIDLs, 0)
	if hResultFailed(hr) {
		return fmt.Errorf("failed to reveal path in Explorer: 0x%08X", uint32(hr))
	}
	return nil
}

func coTaskMemFree(p uintptr) {
	if p != 0 {
		procCoTaskMemFree.Call(p)
	}
}

func hResultFailed(hr uintptr) bool {
	return int32(hr) < 0
}
