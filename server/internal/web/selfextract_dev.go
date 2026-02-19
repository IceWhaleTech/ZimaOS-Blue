//go:build dev

package web

// SelfExtractAndRestart is a no-op in dev builds.
func SelfExtractAndRestart() bool { return true }
