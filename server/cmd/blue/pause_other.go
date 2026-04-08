//go:build !windows

package main

func newWindowsExplorerExitPauser(_ []string) func(int) {
	return func(int) {}
}
