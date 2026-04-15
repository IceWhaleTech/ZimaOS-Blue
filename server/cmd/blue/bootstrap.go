package main

import (
	"fmt"
	"os"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/bootstrap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

// runCompleteBootstrap removes BOOTSTRAP.md from the workspace, marking the
// first-run onboarding guide as done.
func runCompleteBootstrap() {
	dataDir := getDataDir()
	mgr := workspace.NewManager(bootstrap.ResolveWorkspaceDir(dataDir, nil))

	if err := mgr.CompleteBootstrap(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Bootstrap completed. BOOTSTRAP.md removed.")
}
