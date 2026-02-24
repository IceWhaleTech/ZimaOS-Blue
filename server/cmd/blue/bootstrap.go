package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

// runCompleteBootstrap removes BOOTSTRAP.md from the workspace, marking the
// first-run onboarding guide as done.
func runCompleteBootstrap() {
	dataDir := getDataDir()
	mgr := workspace.NewManager(filepath.Join(dataDir, "workspace"))

	if !mgr.IsBootstrapPending() {
		fmt.Println("Bootstrap already completed.")
		return
	}

	if err := mgr.CompleteBootstrap(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Bootstrap completed. BOOTSTRAP.md removed.")
}
