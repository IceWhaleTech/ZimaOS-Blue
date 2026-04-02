package database

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

const startupIntegrityStateFileName = ".startup-integrity-state"

var startupQuickCheckEnabled atomic.Bool
var startupQuickCheckStateMu sync.Mutex
var startupQuickCheckedPaths map[string]struct{}

func init() {
	startupQuickCheckEnabled.Store(true)
	startupQuickCheckedPaths = make(map[string]struct{})
}

func startupIntegrityStatePath(dataDir string) string {
	return filepath.Join(strings.TrimSpace(dataDir), startupIntegrityStateFileName)
}

// BeginStartupIntegritySession records that startup is in progress and returns whether the
// previous run ended cleanly. Missing or unreadable state defaults to "not clean" so callers
// can keep conservative recovery behavior.
func BeginStartupIntegritySession(dataDir string) (bool, error) {
	if strings.TrimSpace(dataDir) == "" {
		return false, fmt.Errorf("data directory is empty")
	}

	previousClean, err := readStartupIntegrityState(dataDir)
	if err != nil {
		previousClean = false
	}

	if writeErr := writeStartupIntegrityState(dataDir, "dirty"); writeErr != nil {
		if err != nil {
			return false, fmt.Errorf("read startup integrity state: %w; write dirty state: %v", err, writeErr)
		}
		return previousClean, fmt.Errorf("write startup integrity state: %w", writeErr)
	}

	return previousClean, err
}

// MarkStartupIntegrityClean records that the process shut down cleanly.
func MarkStartupIntegrityClean(dataDir string) error {
	if strings.TrimSpace(dataDir) == "" {
		return fmt.Errorf("data directory is empty")
	}
	return writeStartupIntegrityState(dataDir, "clean")
}

// SetStartupQuickCheckEnabled toggles proactive PRAGMA quick_check on database open.
func SetStartupQuickCheckEnabled(enabled bool) {
	startupQuickCheckEnabled.Store(enabled)
	startupQuickCheckStateMu.Lock()
	startupQuickCheckedPaths = make(map[string]struct{})
	startupQuickCheckStateMu.Unlock()
}

// StartupQuickCheckEnabled reports whether proactive PRAGMA quick_check is enabled.
func StartupQuickCheckEnabled() bool {
	return startupQuickCheckEnabled.Load()
}

func consumeStartupQuickCheckPath(dbPath string) bool {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" || !StartupQuickCheckEnabled() {
		return false
	}

	normalizedPath := filepath.Clean(dbPath)

	startupQuickCheckStateMu.Lock()
	defer startupQuickCheckStateMu.Unlock()

	if _, exists := startupQuickCheckedPaths[normalizedPath]; exists {
		return false
	}

	startupQuickCheckedPaths[normalizedPath] = struct{}{}
	return true
}

func readStartupIntegrityState(dataDir string) (bool, error) {
	raw, err := os.ReadFile(startupIntegrityStatePath(dataDir))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(raw)) == "clean", nil
}

func writeStartupIntegrityState(dataDir string, state string) error {
	path := startupIntegrityStatePath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(state+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
