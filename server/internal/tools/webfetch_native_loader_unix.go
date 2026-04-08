//go:build !windows

package tools

import "github.com/ebitengine/purego"

func webFetchNativeOpenLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_LAZY)
}

func webFetchNativeLookupSymbol(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}
