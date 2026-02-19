//go:build !dev

package web

import (
	"encoding/binary"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
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

	offset, ok := readAppendedOffset(exe)
	if !ok {
		return true // no appended data, continue normally
	}

	log.Printf("[web] detected appended dist in binary, extracting sidecar...")

	// Extract dist to sidecar directory next to binary
	sidecar := filepath.Join(filepath.Dir(exe), "dist")
	if err := extractAppendedDist(exe, offset, sidecar); err != nil {
		log.Printf("[web] sidecar extraction failed: %v", err)
		return true // fall through, tryExtractAppended will handle it
	}
	log.Printf("[web] dist extracted to sidecar: %s", sidecar)

	// Truncate binary to remove appended data, restoring clean Mach-O
	if err := os.Truncate(exe, offset); err != nil {
		log.Printf("[web] truncate failed: %v", err)
		return true // sidecar extracted, dist will work but TCC may fail
	}
	log.Printf("[web] binary truncated to %d bytes (clean Mach-O restored)", offset)

	// Fork: start a child process with the now-clean binary.
	// The child gets a fresh TCC identity check against the clean Mach-O.
	log.Printf("[web] starting child process with clean binary...")
	child := exec.Command(exe, os.Args[1:]...)
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Stdin = os.Stdin
	child.Env = os.Environ()
	// Inherit the process group so signals propagate
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: false}

	if err := child.Start(); err != nil {
		log.Printf("[web] failed to start child: %v", err)
		return true // continue in current process as fallback
	}
	log.Printf("[web] child process started (pid=%d), parent exiting", child.Process.Pid)
	os.Exit(0)
	return true // unreachable
}

// readAppendedOffset reads the 8-byte LE trailer from the binary.
// Returns the offset where appended data starts, and true if valid.
func readAppendedOffset(exe string) (int64, bool) {
	f, err := os.Open(exe)
	if err != nil {
		return 0, false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return 0, false
	}
	if fi.Size() < 16 {
		return 0, false
	}

	if _, err := f.Seek(-8, io.SeekEnd); err != nil {
		return 0, false
	}
	var offset int64
	if err := binary.Read(f, binary.LittleEndian, &offset); err != nil {
		return 0, false
	}
	if offset <= 0 || offset >= fi.Size()-8 {
		return 0, false
	}
	return offset, true
}

// extractAppendedDist extracts the tar.gz at the given offset to dst.
func extractAppendedDist(exe string, offset int64, dst string) error {
	f, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	// Remove old sidecar if present
	os.RemoveAll(dst)
	return extractTarGzFromReader(f, dst)
}
