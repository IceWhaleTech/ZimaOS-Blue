//go:build !windows

package providerpool

// setWindowsHiddenSystem is a no-op on non-Windows platforms.
func setWindowsHiddenSystem(_ string) error { return nil }
