// Package main implements a self-extracting launcher for ZimaOS Blue.
// It embeds the bluecli server binary and web dist assets, extracts them
// on first run, then execs bluecli. On subsequent runs it skips extraction.
//
// On macOS, TCC (Transparency, Consent, and Control) binds speech recognition
// authorization to the parent app's bundle ID. If launched from an IDE like
// Cursor or VS Code, TCC uses that IDE's bundle ID — which won't have speech
// permission. The launcher detects this and re-launches bluecli inside
// Terminal.app so TCC uses com.apple.Terminal (which users can authorize).
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

func main() {
	// Fast path 1: handle help/status/health directly in the launcher
	// without extracting or exec-ing .bluecli.
	if len(os.Args) > 1 && tryFastCmd(os.Args[1:]) {
		return
	}

	// Fast path 2: try IPC for all command invocations before extracting or
	// exec-ing .bluecli. This keeps command handling in launcher and avoids
	// extra process spawn/TCC edge cases.
	if len(os.Args) > 1 && tryIPC(os.Args[1:]) {
		return
	}

	dir, err := launcherDir()
	if err != nil {
		log.Fatalf("[launcher] cannot resolve path: %v", err)
	}

	cliName := ".bluecli"
	if runtime.GOOS == "windows" {
		cliName = ".bluecli.exe"
	}
	cliPath := filepath.Join(dir, cliName)
	distPath := filepath.Join(dir, ".dist")

	// Check if extraction is needed (version mismatch or missing)
	needExtract := needsExtraction(dir, cliPath, distPath)
	if needExtract {
		log.Printf("[launcher] extracting bluecli + dist to %s ...", dir)
		if err := extractBluecli(cliPath); err != nil {
			log.Fatalf("[launcher] extract bluecli: %v", err)
		}
		if err := extractDist(distPath); err != nil {
			log.Fatalf("[launcher] extract dist: %v", err)
		}
		if err := writeVersion(dir); err != nil {
			log.Printf("[launcher] write version: %v", err)
		}
		log.Printf("[launcher] extraction complete")
	}

	// On macOS, if not running inside a real terminal, re-launch in Terminal.app
	// so TCC authorization prompts use com.apple.Terminal's bundle ID.
	if !isRunningInTerminal() {
		log.Printf("[launcher] not running in Terminal.app, re-launching via Terminal...")
		launchViaTerminal(cliPath, os.Args[1:])
		return
	}

	// Direct exec — we're already in Terminal or on non-macOS.
	env := os.Environ()
	if err := syscall.Exec(cliPath, os.Args, env); err != nil {
		log.Fatalf("[launcher] exec %s: %v", cliPath, err)
	}
}

// shellQuote wraps a string in single quotes for safe shell embedding.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// launcherDir returns the directory containing the launcher binary.
func launcherDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// needsExtraction checks if bluecli or dist need to be (re-)extracted.
func needsExtraction(dir, cliPath, distPath string) bool {
	// Missing bluecli?
	if _, err := os.Stat(cliPath); os.IsNotExist(err) {
		return true
	}
	// Missing dist?
	if _, err := os.Stat(filepath.Join(distPath, "index.html")); os.IsNotExist(err) {
		return true
	}
	// Version mismatch?
	versionFile := filepath.Join(dir, ".bluecli-version")
	data, err := os.ReadFile(versionFile)
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(data)) != embeddedVersion()
}

// embeddedVersion returns a hash of the embedded bluecli binary.
func embeddedVersion() string {
	h := sha256.Sum256(embeddedBluecli)
	return hex.EncodeToString(h[:8]) // 16-char hex
}

func writeVersion(dir string) error {
	return os.WriteFile(filepath.Join(dir, ".bluecli-version"), []byte(embeddedVersion()), 0o644)
}

func extractBluecli(dst string) error {
	if len(embeddedBluecli) == 0 {
		return fmt.Errorf("embedded bluecli asset is missing; build via `make build-launcher`")
	}

	// Write to a new inode then atomically replace. On macOS, in-place truncation
	// of an existing executable can leave AMFI/Syspolicy seeing stale signature
	// state ("load code signature error"), causing immediate SIGKILL on exec.
	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".bluecli-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp bluecli: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(embeddedBluecli); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp bluecli: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp bluecli: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp bluecli: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp bluecli: %w", err)
	}

	// Windows cannot rename over an existing destination.
	if runtime.GOOS == "windows" {
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove existing bluecli: %w", err)
		}
	}
	if err := os.Rename(tmpPath, dst); err != nil {
		return fmt.Errorf("replace bluecli: %w", err)
	}
	cleanup = false
	return nil
}

func extractDist(dst string) error {
	if len(embeddedDist) == 0 {
		return fmt.Errorf("embedded dist asset is missing; build via `make build-launcher`")
	}
	os.RemoveAll(dst)
	gr, err := gzip.NewReader(bytes.NewReader(embeddedDist))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}
		target := filepath.Join(dst, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)) {
			continue // path traversal guard
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, fs.FileMode(0o755))
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), fs.FileMode(0o755))
			out, err := os.Create(target)
			if err != nil {
				return err
			}
			io.Copy(out, tr)
			out.Close()
		}
	}
	return nil
}
