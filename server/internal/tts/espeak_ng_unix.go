//go:build darwin || linux

package tts

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// loadLibraryUnix loads library on Unix-like systems (Linux, macOS)
func loadLibraryUnix(libPath string) (uintptr, error) {
	// Use purego for Unix systems
	handle, err := purego.Dlopen(libPath, purego.RTLD_LAZY)
	if err != nil {
		return 0, fmt.Errorf("dlopen failed: %w", err)
	}
	return handle, nil
}

// loadLibraryWindows is not available on Unix
func loadLibraryWindows(libPath string) (uintptr, error) {
	panic("loadLibraryWindows not available on Unix")
}
