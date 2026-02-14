//go:build !windows

package main

import "fmt"

// HandleServiceCommand handles service commands.
// On non-Windows platforms, this is a no-op.
func HandleServiceCommand(args []string) bool {
	return false
}

// PrintServiceHelp prints help for service commands.
// On non-Windows platforms, service management is not available.
func PrintServiceHelp() {
	fmt.Println("Service Commands:")
	fmt.Println("  (Service management is only available on Windows)")
}
