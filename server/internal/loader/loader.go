// Package loader provides OS-aware shared library loading via FFI (zero CGo).
// Modeled after yzma's loader pattern: purego + jupiterrider/ffi.
package loader

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jupiterrider/ffi"
)

const EnvLibPath = "ZIMAOS_BLUE_LIB"

// SetDLLSearchDir sets the DLL search directory on Windows so sibling
// dependencies resolve correctly. No-op on other platforms.
func SetDLLSearchDir(dir string) {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	setDllSearchDir(dir)
}

// LoadLibrary loads a shared library by name from the given directory.
// If path is empty, falls back to ZIMAOS_BLUE_LIB environment variable.
func LoadLibrary(path, lib string) (ffi.Lib, error) {
	if path == "" {
		path = os.Getenv(EnvLibPath)
	}
	if path == "" {
		return ffi.Lib{}, fmt.Errorf("library path not specified and %s env variable not set", EnvLibPath)
	}
	// Resolve to absolute path so Windows can find the DLL.
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	// On Windows, set DLL search directory so sibling deps resolve.
	setDllSearchDir(path)
	filename := LibraryFilename(path, lib)
	return ffi.Load(filename)
}

// LibraryFilename returns the platform-specific shared library path.
func LibraryFilename(dir, lib string) string {
	switch runtime.GOOS {
	case "linux", "freebsd":
		return filepath.Join(dir, fmt.Sprintf("lib%s.so", lib))
	case "windows":
		return filepath.Join(dir, fmt.Sprintf("%s.dll", lib))
	case "darwin":
		return filepath.Join(dir, fmt.Sprintf("lib%s.dylib", lib))
	default:
		return filepath.Join(dir, lib)
	}
}

// LibraryExists checks if a shared library file exists at the given path.
func LibraryExists(dir, lib string) bool {
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	filename := LibraryFilename(dir, lib)
	_, err := os.Stat(filename)
	return err == nil
}

// DiscoverLibs scans a directory for shared library files and returns their base names.
func DiscoverLibs(dir string) ([]string, error) {
	var pattern string
	switch runtime.GOOS {
	case "linux", "freebsd":
		pattern = "*.so"
	case "windows":
		pattern = "*.dll"
	case "darwin":
		pattern = "*.dylib"
	default:
		return nil, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("directory not found: %w", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return nil, err
	}

	libs := make([]string, 0, len(matches))
	for _, m := range matches {
		libs = append(libs, filepath.Base(m))
	}
	return libs, nil
}
