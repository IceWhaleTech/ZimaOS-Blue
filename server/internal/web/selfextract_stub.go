//go:build !dev && !darwin

package web

// SelfExtractAndRestart is a no-op on non-macOS platforms.
func SelfExtractAndRestart() bool { return true }
