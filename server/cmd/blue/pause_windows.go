//go:build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"unsafe"

	serviceutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/service"
	"golang.org/x/sys/windows"
)

const th32csSnapProcess = 0x00000002

type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [windows.MAX_PATH]uint16
}

var (
	kernel32                     = windows.NewLazySystemDLL("kernel32.dll")
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
)

func newWindowsExplorerExitPauser(args []string) func(int) {
	parentName, _ := currentParentProcessName()
	if !shouldPauseOnExit(args, serviceutil.IsInteractive(), parentName, os.Getenv) {
		return func(int) {}
	}
	return func(exitCode int) {
		if exitCode != 0 {
			_, _ = os.Stderr.WriteString("\r\n[INFO] Process exited. Press any key to close this window . . .\r\n")
		} else {
			_, _ = os.Stdout.WriteString("\r\n[INFO] Process exited. Press any key to close this window . . .\r\n")
		}
		pauseCmd := exec.Command("cmd.exe", "/c", "pause")
		pauseCmd.Stdout = os.Stdout
		pauseCmd.Stderr = os.Stderr
		pauseCmd.Stdin = os.Stdin
		_ = pauseCmd.Run()
	}
}

func shouldPauseOnExit(args []string, interactive bool, parentName string, getenv func(string) string) bool {
	_ = parentName
	if strings.TrimSpace(getenv("ZIMAOS_BLUE_NO_PAUSE_ON_EXIT")) != "" {
		return false
	}
	if strings.TrimSpace(getenv("ZIMAOS_BLUE_PAUSE_ON_EXIT")) != "" {
		return true
	}
	if len(args) != 1 || !interactive {
		return false
	}
	return true
}

func currentParentProcessName() (string, error) {
	currentPID := uint32(os.Getpid())
	snapshot, _, callErr := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snapshot == uintptr(windows.InvalidHandle) {
		if callErr != windows.ERROR_SUCCESS && callErr != nil {
			return "", callErr
		}
		return "", windows.ERROR_INVALID_HANDLE
	}
	defer windows.CloseHandle(windows.Handle(snapshot))

	entries := make(map[uint32]processEntry32)
	var entry processEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	ret, _, err := procProcess32FirstW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
	if ret == 0 {
		if err != windows.ERROR_SUCCESS && err != nil {
			return "", err
		}
		return "", windows.ERROR_NO_MORE_FILES
	}
	for {
		entries[entry.ProcessID] = entry
		entry.Size = uint32(unsafe.Sizeof(entry))
		ret, _, err = procProcess32NextW.Call(snapshot, uintptr(unsafe.Pointer(&entry)))
		if ret == 0 {
			break
		}
	}

	current, ok := entries[currentPID]
	if !ok {
		return "", windows.ERROR_NOT_FOUND
	}
	parent, ok := entries[current.ParentProcessID]
	if !ok {
		return "", windows.ERROR_NOT_FOUND
	}
	return windows.UTF16ToString(parent.ExeFile[:]), nil
}
