//go:build !windows

package main

// HandleServiceCommand is a no-op on non-Windows platforms.
// Service management on Unix is done via systemctl/launchctl.
func HandleServiceCommand(args []string) bool {
	return false
}

// PrintServiceHelp prints help for service commands on Unix.
func PrintServiceHelp() {
	// No Windows service commands on Unix
}
