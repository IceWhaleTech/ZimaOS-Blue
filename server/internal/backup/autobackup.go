package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type watchRoot struct {
	name string
	path string
}

// StartAutoBackup starts background auto backup (periodic and/or change-triggered).
func (m *Manager) StartAutoBackup(parent context.Context) {
	if !m.config.Enabled {
		return
	}
	if !m.config.AutoBackup && !m.config.AutoBackupOnChange {
		return
	}
	if parent == nil {
		parent = context.Background()
	}

	m.autoMu.Lock()
	if m.autoCancel != nil {
		m.autoMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	m.autoCancel = cancel
	m.autoDone = done
	m.autoMu.Unlock()

	go m.runAutoBackupLoop(ctx, done)
}

// StopAutoBackup stops the background auto backup loop.
func (m *Manager) StopAutoBackup() {
	m.autoMu.Lock()
	cancel := m.autoCancel
	done := m.autoDone
	m.autoCancel = nil
	m.autoDone = nil
	m.autoMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func (m *Manager) runAutoBackupLoop(ctx context.Context, done chan struct{}) {
	defer func() {
		m.autoMu.Lock()
		m.autoCancel = nil
		m.autoDone = nil
		m.autoMu.Unlock()
		close(done)
	}()

	if m.config.AutoBackupOnChange {
		digest, err := m.snapshotDigest()
		if err == nil {
			m.autoMu.Lock()
			m.lastSnapshot = digest
			m.autoMu.Unlock()
		}
	}

	var (
		intervalTicker *time.Ticker
		changeTicker   *time.Ticker
		intervalCh     <-chan time.Time
		changeCh       <-chan time.Time
	)

	if m.config.AutoBackup && m.config.AutoBackupInterval > 0 {
		intervalTicker = time.NewTicker(m.config.AutoBackupInterval)
		intervalCh = intervalTicker.C
		defer intervalTicker.Stop()
	}
	if m.config.AutoBackupOnChange && m.config.ChangePollInterval > 0 {
		changeTicker = time.NewTicker(m.config.ChangePollInterval)
		changeCh = changeTicker.C
		defer changeTicker.Stop()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-intervalCh:
			m.runAutoBackup(ctx)
		case <-changeCh:
			m.handleChangeTick(ctx)
		}
	}
}

func (m *Manager) handleChangeTick(ctx context.Context) {
	digest, err := m.snapshotDigest()
	if err != nil {
		return
	}

	shouldBackup := false

	m.autoMu.Lock()
	if m.lastSnapshot == "" {
		m.lastSnapshot = digest
		m.autoMu.Unlock()
		return
	}
	if digest != m.lastSnapshot {
		m.lastSnapshot = digest
		m.pendingChange = true
	}
	if m.pendingChange {
		if m.config.ChangeDebounce <= 0 || m.lastAutoBackup.IsZero() || timeutil.NowTime().Sub(m.lastAutoBackup) >= m.config.ChangeDebounce {
			shouldBackup = true
		}
	}
	m.autoMu.Unlock()

	if shouldBackup {
		m.runAutoBackup(ctx)
	}
}

func (m *Manager) runAutoBackup(ctx context.Context) {
	backupCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	if _, err := m.createWithSource(backupCtx, BackupTypeFull, BackupSourceAuto); err != nil {
		return
	}
	_, _ = m.CleanupAutoBackupsKeepLatest()
	_, _ = m.Cleanup()

	m.autoMu.Lock()
	m.lastAutoBackup = timeutil.NowTime()
	m.pendingChange = false
	m.autoMu.Unlock()
}

func (m *Manager) snapshotDigest() (string, error) {
	roots := m.watchRoots()
	if len(roots) == 0 {
		return "", nil
	}

	var entries []string
	for _, root := range roots {
		info, err := os.Stat(root.path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		entries = append(entries, fmt.Sprintf("%s|.|%d|%d|%o", root.name, info.Size(), info.ModTime().UnixNano(), info.Mode().Perm()))

		queue := []string{root.path}
		for len(queue) > 0 {
			currentDir := queue[0]
			queue = queue[1:]

			dirEntries, err := os.ReadDir(currentDir)
			if err != nil {
				continue
			}

			for _, entry := range dirEntries {
				if entry.IsDir() && excludedBackupDirs[entry.Name()] {
					continue
				}

				path := filepath.Join(currentDir, entry.Name())
				fileInfo, err := entry.Info()
				if err != nil {
					continue
				}
				if fileInfo.Mode()&os.ModeSymlink != 0 {
					continue
				}
				if !fileInfo.Mode().IsRegular() && !fileInfo.IsDir() {
					continue
				}
				if fileInfo.Mode().IsRegular() {
					if excludedBackupExtensions[filepath.Ext(entry.Name())] {
						continue
					}
				}

				relPath, err := filepath.Rel(root.path, path)
				if err != nil {
					continue
				}
				entries = append(entries, fmt.Sprintf("%s|%s|%d|%d|%o", root.name, filepath.ToSlash(relPath), fileInfo.Size(), fileInfo.ModTime().UnixNano(), fileInfo.Mode().Perm()))

				if entry.IsDir() {
					queue = append(queue, path)
				}
			}
		}
	}

	sort.Strings(entries)
	sum := sha256.New()
	for _, entry := range entries {
		if _, err := io.WriteString(sum, entry); err != nil {
			return "", err
		}
		if _, err := io.WriteString(sum, "\n"); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func (m *Manager) watchRoots() []watchRoot {
	candidates := []watchRoot{
		{name: "data", path: m.dataDir},
		{name: "config", path: m.configDir},
	}
	if m.shouldBackupSkillsIndependently() {
		candidates = append(candidates, watchRoot{name: "skills", path: m.skillsDir})
	}

	// De-duplicate exact paths
	seen := make(map[string]bool, len(candidates))
	roots := make([]watchRoot, 0, len(candidates))
	for _, c := range candidates {
		if c.path == "" {
			continue
		}
		absPath, err := filepath.Abs(c.path)
		if err != nil {
			continue
		}
		absPath = filepath.Clean(absPath)
		if seen[absPath] {
			continue
		}
		seen[absPath] = true
		roots = append(roots, watchRoot{name: c.name, path: absPath})
	}

	// Remove paths covered by another root.
	filtered := make([]watchRoot, 0, len(roots))
	for _, root := range roots {
		covered := false
		for _, other := range roots {
			if root.path == other.path {
				continue
			}
			if isSubPath(other.path, root.path) {
				covered = true
				break
			}
		}
		if !covered {
			filtered = append(filtered, root)
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].name < filtered[j].name
	})
	return filtered
}
