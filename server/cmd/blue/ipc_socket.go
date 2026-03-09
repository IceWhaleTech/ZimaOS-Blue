package main

import "os"

func candidateIPCSocketPaths() []string {
	if sockPath := os.Getenv("BLUE_IPC_SOCKET"); sockPath != "" {
		return []string{sockPath}
	}
	return []string{
		"/tmp/blue.sock",
		getDataDir() + "/blue.sock",
	}
}
