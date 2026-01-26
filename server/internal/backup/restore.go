package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RestoreOptions configures restore behavior
type RestoreOptions struct {
	// OverwriteExisting overwrites existing files
	OverwriteExisting bool
	// RestoreConfig restores configuration files
	RestoreConfig bool
	// RestoreData restores data files
	RestoreData bool
	// DryRun only validates without actually restoring
	DryRun bool
}

// DefaultRestoreOptions returns default restore options
func DefaultRestoreOptions() RestoreOptions {
	return RestoreOptions{
		OverwriteExisting: true,
		RestoreConfig:     true,
		RestoreData:       true,
		DryRun:            false,
	}
}

// RestoreResult contains the result of a restore operation
type RestoreResult struct {
	Success       bool     `json:"success"`
	FilesRestored int      `json:"files_restored"`
	FilesSkipped  int      `json:"files_skipped"`
	Errors        []string `json:"errors,omitempty"`
}

// Restore restores a backup
func (m *Manager) Restore(ctx context.Context, id string, opts RestoreOptions) (*RestoreResult, error) {
	// Verify backup first
	if err := m.Verify(id); err != nil {
		return nil, fmt.Errorf("backup verification failed: %w", err)
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

	result := &RestoreResult{
		Success: true,
	}

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
		} else {
			result.FilesSkipped++
			continue
		}

		// Skip if relPath is empty (root directory entry)
		if relPath == "" || relPath == "." {
			continue
		}

		targetPath := filepath.Join(targetDir, relPath)

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
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("failed to create parent directory for %s: %v", targetPath, err))
				continue
			}

			// Create file
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
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
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Create file
		outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
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
