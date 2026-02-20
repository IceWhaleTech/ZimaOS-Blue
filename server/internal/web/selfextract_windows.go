//go:build !dev && windows

package web

import "os/exec"

func setSysProcAttr(cmd *exec.Cmd) {
	// Windows doesn't support Setpgid
}
