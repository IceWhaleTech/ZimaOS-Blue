//go:build darwin

package main

import "os/exec"

func openBrowserURL(rawURL string) error {
	return exec.Command("open", rawURL).Start()
}
