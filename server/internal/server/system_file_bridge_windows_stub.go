//go:build !windows

package server

import "fmt"

func revealPathWindows(path string, isDir bool) error {
	_ = path
	_ = isDir
	return fmt.Errorf("reveal-path is not supported on non-windows hosts")
}
