//go:build !darwin && !linux && !windows

package main

import "fmt"

func openBrowserURL(rawURL string) error {
	return fmt.Errorf("automatic browser open is not supported on this platform")
}
