//go:build linux

package main

import "os/exec"

func openBrowserURL(rawURL string) error {
	return exec.Command("xdg-open", rawURL).Start()
}
