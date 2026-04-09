//go:build windows
// +build windows

package workspace

import (
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	windowsLocaleNameMaxLength          = 85
	windowsPreferredUILanguageNameFlags = 0x8
	windowsPreferredUILanguageBufChars  = 512
)

var (
	workspaceKernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetUserPreferredUILanguages = workspaceKernel32.NewProc("GetUserPreferredUILanguages")
	procGetUserDefaultLocaleName    = workspaceKernel32.NewProc("GetUserDefaultLocaleName")
	procGetSystemDefaultLocaleName  = workspaceKernel32.NewProc("GetSystemDefaultLocaleName")

	readUserPreferredUILanguage = windowsReadUserPreferredUILanguage
	readUserDefaultLocaleName   = windowsReadUserDefaultLocaleName
	readSystemDefaultLocaleName = windowsReadSystemDefaultLocaleName
)

// windowsLocale reads the Windows UI language directly from Win32 APIs so
// startup does not need to spawn PowerShell and flash a console window.
func windowsLocale() string {
	for _, probe := range []func() string{
		readUserPreferredUILanguage,
		readUserDefaultLocaleName,
		readSystemDefaultLocaleName,
	} {
		if locale := normalizeWindowsLocale(probe()); len(locale) >= 2 {
			return locale
		}
	}
	return ""
}

func windowsReadUserPreferredUILanguage() string {
	if err := procGetUserPreferredUILanguages.Find(); err != nil {
		return ""
	}

	var numLanguages uint32
	buf := make([]uint16, windowsPreferredUILanguageBufChars)
	bufChars := uint32(len(buf))

	r1, _, _ := procGetUserPreferredUILanguages.Call(
		uintptr(windowsPreferredUILanguageNameFlags),
		uintptr(unsafe.Pointer(&numLanguages)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufChars)),
	)
	if r1 == 0 {
		if bufChars <= uint32(len(buf)) || bufChars == 0 || bufChars > 4096 {
			return ""
		}
		buf = make([]uint16, bufChars)
		numLanguages = 0
		r1, _, _ = procGetUserPreferredUILanguages.Call(
			uintptr(windowsPreferredUILanguageNameFlags),
			uintptr(unsafe.Pointer(&numLanguages)),
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&bufChars)),
		)
		if r1 == 0 {
			return ""
		}
	}

	return firstWindowsMultiString(buf)
}

func windowsReadUserDefaultLocaleName() string {
	return windowsReadLocaleName(procGetUserDefaultLocaleName)
}

func windowsReadSystemDefaultLocaleName() string {
	return windowsReadLocaleName(procGetSystemDefaultLocaleName)
}

func windowsReadLocaleName(proc *windows.LazyProc) string {
	if err := proc.Find(); err != nil {
		return ""
	}

	buf := make([]uint16, windowsLocaleNameMaxLength)
	r1, _, _ := proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r1 == 0 {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func firstWindowsMultiString(buf []uint16) string {
	if len(buf) == 0 {
		return ""
	}
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

func normalizeWindowsLocale(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.ReplaceAll(value, "_", "-")
}
