package bootstrap

import "os"

func resolveIPCSocketPath(_ string) string {
	if sockPath := os.Getenv("BLUE_IPC_SOCKET"); sockPath != "" {
		return sockPath
	}
	return "/tmp/blue.sock"
}
