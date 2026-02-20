//go:build !darwin

package main

// isRunningInTerminal always returns true on non-macOS (no TCC concerns).
func isRunningInTerminal() bool { return true }

// launchViaTerminal is a no-op on non-macOS.
func launchViaTerminal(cliPath string, args []string) {}
