//go:build !dev && darwin

package web

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// SelfExtractAndRestart checks if the binary has appended dist data.
// On macOS, it extracts the dist as a sidecar directory next to the binary,
// truncates the binary to remove the appended data (restoring clean Mach-O),
// then forks itself as a child process and exits. The child process starts
// fresh with a clean binary that passes TCC's strict signature validation.
//
// Returns true if the process should continue normally (no restart needed).
// If a restart is needed, this function does not return (os.Exit).
// On non-macOS or if no appended data is found, this is a no-op.
func SelfExtractAndRestart() bool {
	if runtime.GOOS != "darwin" {
		return true
	}

	exe, err := os.Executable()
	if err != nil {
		return true
	}
	// Resolve symlinks so we truncate the real binary
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return true
	}

	layout, ok := readAppendedLayout(exe)
	if !ok {
		return true // no appended data, continue normally
	}

	log.Printf("[web] detected appended dist in binary, extracting sidecar...")

	// Extract dist to sidecar directory next to binary
	sidecar := filepath.Join(filepath.Dir(exe), "dist")
	if err := extractAppendedDist(exe, layout, sidecar); err != nil {
		log.Printf("[web] sidecar extraction failed: %v", err)
		return true // fall through, tryExtractAppended will handle it
	}
	log.Printf("[web] dist extracted to sidecar: %s", sidecar)

	// Truncate binary to remove appended data, restoring clean Mach-O
	if err := os.Truncate(exe, layout.offset); err != nil {
		log.Printf("[web] truncate failed: %v", err)
		return true // sidecar extracted, dist will work but TCC may fail
	}
	log.Printf("[web] binary truncated to %d bytes (clean Mach-O restored)", layout.offset)

	// Fork: start a child process with the now-clean binary.
	// The child gets a fresh TCC identity check against the clean Mach-O.
	log.Printf("[web] starting child process with clean binary...")
	child := exec.Command(exe, os.Args[1:]...)
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Stdin = os.Stdin
	child.Env = os.Environ()
	setSysProcAttr(child)

	if err := child.Start(); err != nil {
		log.Printf("[web] failed to start child: %v", err)
		return true // continue in current process as fallback
	}
	log.Printf("[web] child process started (pid=%d), parent exiting", child.Process.Pid)
	os.Exit(0)
	return true // unreachable
}

// readAppendedOffset reads the appended data layout from the binary.
// Returns the offset where appended data starts, and true if valid.
func readAppendedOffset(exe string) (int64, bool) {
	layout, ok := readAppendedLayout(exe)
	if !ok {
		return 0, false
	}
	return layout.offset, true
}

// extractAppendedDist extracts the tar.gz to dst.
func extractAppendedDist(exe string, layout appendedLayout, dst string) error {
	f, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer f.Close()

	if layout.dataEnd < layout.offset {
		return io.ErrUnexpectedEOF
	}

	// Remove old sidecar if present
	os.RemoveAll(dst)
	section := io.NewSectionReader(f, layout.offset, layout.dataEnd-layout.offset)
	return extractTarGzFromReader(section, dst)
}
