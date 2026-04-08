//go:build windows

package tools

import "testing"

func TestWebFetchNativeWindowsLoaderResolvesKernel32Symbol(t *testing.T) {
	handle, err := webFetchNativeOpenLibrary("kernel32.dll")
	if err != nil {
		t.Fatalf("webFetchNativeOpenLibrary() error = %v", err)
	}

	sym, err := webFetchNativeLookupSymbol(handle, "GetCurrentProcessId")
	if err != nil {
		t.Fatalf("webFetchNativeLookupSymbol() error = %v", err)
	}
	if sym == 0 {
		t.Fatal("webFetchNativeLookupSymbol() returned 0")
	}
}
