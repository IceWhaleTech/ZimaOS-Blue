package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
)

// PendingRestore represents a pending restore operation that will be applied on restart
type PendingRestore struct {
	BackupID   string `json:"backup_id"`
	BackupPath string `json:"backup_path"`
	StagingDir string `json:"staging_dir"`
	CreatedAt  string `json:"created_at"`
}

// RestoreOptions configures restore behavior
type RestoreOptions struct {
	// OverwriteExisting overwrites existing files
	OverwriteExisting bool
	// RestoreConfig restores configuration files
	RestoreConfig bool
	// RestoreData restores data files
	RestoreData bool
	// RestoreSkills restores skills files
	RestoreSkills bool
	// CreateCheckpoint creates a full backup before restore.
	CreateCheckpoint bool
	// CheckpointReason tags the pre-restore checkpoint metadata.
	CheckpointReason string
	// DryRun only validates without actually restoring
	DryRun bool
	// SkipVerify skips checksum verification
	// Security: This option is deprecated and will log a warning.
	// Checksum verification is critical for detecting tampered backups.
	SkipVerify bool
	// RequireRestart if true, stages the restore for next restart instead of immediate restore
	// This is the recommended approach for hot recovery to avoid database lock issues
	RequireRestart bool
}

// DefaultRestoreOptions returns default restore options
// Security: SkipVerify defaults to false to ensure backup integrity
func DefaultRestoreOptions() RestoreOptions {
	return RestoreOptions{
		OverwriteExisting: true,
		RestoreConfig:     true,
		RestoreData:       true,
		RestoreSkills:     true,
		CreateCheckpoint:  true,
		CheckpointReason:  "pre_restore",
		DryRun:            false,
		SkipVerify:        false, // Security: Always verify by default
	}
}

// RestoreResult contains the result of a restore operation
type RestoreResult struct {
	Success          bool     `json:"success"`
	FilesRestored    int      `json:"files_restored"`
	FilesSkipped     int      `json:"files_skipped"`
	CheckpointID     string   `json:"checkpoint_id,omitempty"`
	CheckpointAt     string   `json:"checkpoint_at,omitempty"`
	CheckpointReason string   `json:"checkpoint_reason,omitempty"`
	Errors           []string `json:"errors,omitempty"`
}

// Restore restores a backup
func (m *Manager) Restore(ctx context.Context, id string, opts RestoreOptions) (*RestoreResult, error) {
	startedAt := time.Now()
	result := &RestoreResult{
		Success: true,
	}

	backupLogf(
		"restore started backup_id=%s overwrite=%t restore_config=%t restore_data=%t restore_skills=%t create_checkpoint=%t dry_run=%t skip_verify=%t",
		id,
		opts.OverwriteExisting,
		opts.RestoreConfig,
		opts.RestoreData,
		opts.RestoreSkills,
		opts.CreateCheckpoint,
		opts.DryRun,
		opts.SkipVerify,
	)

	m.mu.RLock()
	_, exists := m.backups[id]
	m.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("backup not found: %s", id)
	}

	// Security: Log warning if verification is skipped
	if opts.SkipVerify {
		backupLogf("restore verification skipped backup_id=%s", id)
	}

	if opts.CreateCheckpoint && !opts.DryRun {
		reason := opts.CheckpointReason
		if reason == "" {
			reason = "pre_restore"
		}
		backupLogf("restore creating checkpoint backup_id=%s reason=%s", id, reason)
		checkpoint, err := m.CreateCheckpoint(ctx, reason)
		if err != nil {
			return nil, fmt.Errorf("failed to create restore checkpoint: %w", err)
		}
		result.CheckpointID = checkpoint.ID
		result.CheckpointAt = checkpoint.CreatedAt.Format(time.RFC3339)
		result.CheckpointReason = checkpoint.CheckpointReason
		backupLogf("restore created checkpoint backup_id=%s checkpoint_id=%s", id, checkpoint.ID)
	}

	// Verify backup first (unless skipped)
	if !opts.SkipVerify {
		backupLogf("restore verifying backup backup_id=%s", id)
		if err := m.Verify(id); err != nil {
			return nil, fmt.Errorf("backup verification failed: %w", err)
		}
	}

	m.mu.RLock()
	info := m.backups[id]
	m.mu.RUnlock()

	// Open backup file
	file, err := os.Open(info.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to read tar header: %v", err))
			continue
		}

		// Determine target directory based on prefix
		var targetDir string
		relPath := header.Name

		if strings.HasPrefix(relPath, "config/") || strings.HasPrefix(relPath, "config\\") {
			if !opts.RestoreConfig {
				result.FilesSkipped++
				continue
			}
			targetDir = m.configDir
			relPath = strings.TrimPrefix(relPath, "config/")
			relPath = strings.TrimPrefix(relPath, "config\\")
		} else if strings.HasPrefix(relPath, "data/") || strings.HasPrefix(relPath, "data\\") {
			if !opts.RestoreData {
				result.FilesSkipped++
				continue
			}
			targetDir = m.dataDir
			relPath = strings.TrimPrefix(relPath, "data/")
			relPath = strings.TrimPrefix(relPath, "data\\")
		} else if strings.HasPrefix(relPath, "skills/") || strings.HasPrefix(relPath, "skills\\") {
			if !opts.RestoreSkills || m.skillsDir == "" {
				result.FilesSkipped++
				continue
			}
			targetDir = m.skillsDir
			relPath = strings.TrimPrefix(relPath, "skills/")
			relPath = strings.TrimPrefix(relPath, "skills\\")
		} else {
			result.FilesSkipped++
			continue
		}

		// Skip if relPath is empty (root directory entry)
		if relPath == "" || relPath == "." {
			continue
		}
		if shouldSkipBackupFile(filepath.Base(relPath), header.Size) {
			result.FilesSkipped++
			continue
		}

		targetPath := filepath.Join(targetDir, relPath)

		// Security: Validate that the target path is within the expected directory
		// This prevents path traversal attacks via malicious backup files
		absTargetDir, _ := filepath.Abs(targetDir)
		absTargetPath, _ := filepath.Abs(targetPath)
		if !strings.HasPrefix(absTargetPath, absTargetDir) {
			result.Errors = append(result.Errors, fmt.Sprintf("security: path traversal detected for %s", relPath))
			result.FilesSkipped++
			continue
		}

		// Check if file exists
		if !opts.OverwriteExisting {
			if _, err := os.Stat(targetPath); err == nil {
				result.FilesSkipped++
				continue
			}
		}

		// Dry run - don't actually restore
		if opts.DryRun {
			result.FilesRestored++
			continue
		}

		// Create directory or file
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create directory %s: %v", targetPath, err))
				continue
			}
		} else {
			// Ensure parent directory exists
			// Security: Use 0700 for directories containing restored files
			if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create parent directory for %s: %v", targetPath, err))
				continue
			}

			// Security: Sanitize file mode - don't allow setuid/setgid/sticky bits
			fileMode := os.FileMode(header.Mode) & 0777
			if fileMode > 0700 {
				fileMode = 0600 // Default to owner-only for sensitive files
			}

			// Create file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
			if err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create file %s: %v", targetPath, err))
				continue
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				result.Errors = append(result.Errors, fmt.Sprintf("failed to write file %s: %v", targetPath, err))
				continue
			}
			outFile.Close()
		}

		result.FilesRestored++
	}

	if len(result.Errors) > 0 {
		result.Success = false
	}

	// Checkpoint WAL for all restored databases so committed frames are merged
	// into the main DB files and WAL is truncated safely.
	if !opts.DryRun && result.Success {
		backupLogf("restore checkpointing restored sqlite databases backup_id=%s", id)
		if opts.RestoreData {
			if err := database.CheckpointAllDatabasesInDir(m.dataDir, database.CheckpointTruncate); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("warning: failed to checkpoint WAL in data dir: %v", err))
			}
		}
		if opts.RestoreConfig {
			if err := database.CheckpointAllDatabasesInDir(m.configDir, database.CheckpointTruncate); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("warning: failed to checkpoint WAL in config dir: %v", err))
			}
		}
	}

	backupLogf(
		"restore completed backup_id=%s success=%t files_restored=%d files_skipped=%d errors=%d duration=%s",
		id,
		result.Success,
		result.FilesRestored,
		result.FilesSkipped,
		len(result.Errors),
		time.Since(startedAt),
	)

	return result, nil
}

// RestoreFile restores a single file from a backup
func (m *Manager) RestoreFile(ctx context.Context, id string, filePath string, targetPath string) error {
	// Verify backup first
	if err := m.Verify(id); err != nil {
		return fmt.Errorf("backup verification failed: %w", err)
	}

	m.mu.RLock()
	info := m.backups[id]
	m.mu.RUnlock()

	// Open backup file
	file, err := os.Open(info.Path)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Find and extract the specific file
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			return fmt.Errorf("file not found in backup: %s", filePath)
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		if header.Name != filePath {
			continue
		}

		// Found the file, extract it
		if header.Typeflag == tar.TypeDir {
			return fmt.Errorf("cannot restore directory as single file")
		}

		// Ensure parent directory exists
		// Security: Use 0700 for directories
		if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Security: Sanitize file mode
		fileMode := os.FileMode(header.Mode) & 0777
		if fileMode > 0700 {
			fileMode = 0600
		}

		// Create file
		outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, tarReader); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		return nil
	}
}

// ListFiles returns the list of files in a backup
func (m *Manager) ListFiles(id string) ([]string, error) {
	m.mu.RLock()
	info, ok := m.backups[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("backup not found: %s", id)
	}

	return info.Files, nil
}

// pendingRestoreFile is the filename for pending restore marker
const pendingRestoreFile = "pending_restore.json"

// StageRestore prepares a backup for restore on next restart.
// This is the recommended approach for hot recovery to avoid database lock issues.
// The actual restore will happen when ApplyPendingRestore is called during startup.
func (m *Manager) StageRestore(ctx context.Context, id string) (*PendingRestore, error) {
	// Verify backup first
	if err := m.Verify(id); err != nil {
		return nil, fmt.Errorf("backup verification failed: %w", err)
	}

	m.mu.RLock()
	info, ok := m.backups[id]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("backup not found: %s", id)
	}

	// Create pending restore marker
	pending := &PendingRestore{
		BackupID:   id,
		BackupPath: info.Path,
		StagingDir: "", // Direct restore from backup file
		CreatedAt:  info.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	// Write pending restore marker
	markerPath := filepath.Join(m.dataDir, pendingRestoreFile)
	data, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pending restore: %w", err)
	}

	if err := os.WriteFile(markerPath, data, 0600); err != nil {
		return nil, fmt.Errorf("failed to write pending restore marker: %w", err)
	}

	backupLogf("restore staged for restart backup_id=%s marker=%s", id, markerPath)

	return pending, nil
}

// HasPendingRestore checks if there is a pending restore operation
func (m *Manager) HasPendingRestore() bool {
	markerPath := filepath.Join(m.dataDir, pendingRestoreFile)
	_, err := os.Stat(markerPath)
	return err == nil
}

// GetPendingRestore returns the pending restore info if any
func (m *Manager) GetPendingRestore() (*PendingRestore, error) {
	markerPath := filepath.Join(m.dataDir, pendingRestoreFile)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read pending restore marker: %w", err)
	}

	var pending PendingRestore
	if err := json.Unmarshal(data, &pending); err != nil {
		return nil, fmt.Errorf("failed to parse pending restore marker: %w", err)
	}

	return &pending, nil
}

// ApplyPendingRestore applies a pending restore operation.
// This should be called during startup BEFORE opening any databases.
func (m *Manager) ApplyPendingRestore(ctx context.Context) (*RestoreResult, error) {
	pending, err := m.GetPendingRestore()
	if err != nil {
		return nil, err
	}
	if pending == nil {
		return nil, nil // No pending restore
	}

	backupLogf("applying pending restore backup_id=%s", pending.BackupID)

	// Perform the actual restore
	opts := DefaultRestoreOptions()
	opts.OverwriteExisting = true

	result, err := m.Restore(ctx, pending.BackupID, opts)

	// Clear the pending restore marker regardless of result
	// to prevent infinite restore loops on failure
	markerPath := filepath.Join(m.dataDir, pendingRestoreFile)
	os.Remove(markerPath)

	if err != nil {
		return result, fmt.Errorf("failed to apply pending restore: %w", err)
	}

	backupLogf("applied pending restore backup_id=%s success=%t", pending.BackupID, result != nil && result.Success)

	return result, nil
}

// CancelPendingRestore cancels a pending restore operation
func (m *Manager) CancelPendingRestore() error {
	markerPath := filepath.Join(m.dataDir, pendingRestoreFile)
	if err := os.Remove(markerPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cancel pending restore: %w", err)
	}
	return nil
}
