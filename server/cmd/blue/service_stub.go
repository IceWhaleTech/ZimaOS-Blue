//go:build !windows

package main

// HandleServiceCommand handles service-related commands on non-Windows platforms.
// Returns false as service commands are only supported on Windows.
func HandleServiceCommand(args []string) bool {
	return false
}

// PrintServiceHelp prints service command help on non-Windows platforms.
// Does nothing as service commands are only supported on Windows.
func PrintServiceHelp() {
	// No service commands available on non-Windows platforms
}
