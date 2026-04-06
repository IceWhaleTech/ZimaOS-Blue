package main

import (
	"os"
	"path/filepath"
	"strings"
)

func candidateIPCSocketPaths() []string {
	if sockPath := strings.TrimSpace(os.Getenv("BLUE_IPC_SOCKET")); sockPath != "" {
		return []string{sockPath}
	}
	return appendSocketCandidate(nil, filepath.Join(getDataDir(), "blue.sock"), "/tmp/blue.sock")
}

func candidateAuditIPCSocketPaths() []string {
	if sockPath := strings.TrimSpace(os.Getenv("BLUE_AUDIT_IPC_SOCKET")); sockPath != "" {
		return []string{sockPath}
	}
	return appendSocketCandidate(nil, filepath.Join(getDataDir(), "session_audit.sock"), "/tmp/session_audit.sock")
}

func appendSocketCandidate(paths []string, candidates ...string) []string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		duplicate := false
		for _, existing := range paths {
			if existing == candidate {
				duplicate = true
				break
			}
		}
		if !duplicate {
			paths = append(paths, candidate)
		}
	}
	return paths
}
