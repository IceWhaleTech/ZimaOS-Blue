package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Config holds backup configuration
type Config struct {
	Enabled       bool   `mapstructure:"enabled"`
	Schedule      string `mapstructure:"schedule"` // cron expression
	RetentionDays int    `mapstructure:"retention_days"`
	Path          string `mapstructure:"path"`
}

// BackupInfo contains metadata about a backup
type BackupInfo struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	Path      string    `json:"path"`
	Type      string    `json:"type"` // full, config, data
	Checksum  string    `json:"checksum"`
	Version   string    `json:"version"`
	Files     []string  `json:"files"`
}

// BackupType represents the type of backup
type BackupType string

const (
	BackupTypeFull   BackupType = "full"
	BackupTypeConfig BackupType = "config"
	BackupTypeData   BackupType = "data"
)

// Manager handles backup and restore operations
type Manager struct {
	config    Config
	dataDir   string
	configDir string

	mu      sync.RWMutex
	backups map[string]*BackupInfo
}

// NewManager creates a new backup manager
func NewManager(cfg Config, dataDir, configDir string) (*Manager, error) {
	if cfg.Path == "" {
		cfg.Path = "./backups"
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 7
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(cfg.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	m := &Manager{
		config:    cfg,
		dataDir:   dataDir,
		configDir: configDir,
		backups:   make(map[string]*BackupInfo),
	}

	// Load existing backups
	if err := m.loadBackups(); err != nil {
		return nil, fmt.Errorf("failed to load existing backups: %w", err)
	}

	return m, nil
}

// Create creates a new backup
func (m *Manager) Create(ctx context.Context, backupType BackupType) (*BackupInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	timestamp := time.Now()
	filename := fmt.Sprintf("backup_%s_%s.tar.gz", backupType, timestamp.Format("20060102_150405"))
	backupPath := filepath.Join(m.config.Path, filename)

	// Create backup file
	file, err := os.Create(backupPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	var files []string

	// Add files based on backup type
	switch backupType {
	case BackupTypeFull:
		if err := m.addDirectory(tarWriter, m.dataDir, "data", &files); err != nil {
			os.Remove(backupPath)
			return nil, fmt.Errorf("failed to backup data directory: %w", err)
		}
		if err := m.addDirectory(tarWriter, m.configDir, "config", &files); err != nil {
			os.Remove(backupPath)
			return nil, fmt.Errorf("failed to backup config directory: %w", err)
		}
	case BackupTypeConfig:
		if err := m.addDirectory(tarWriter, m.configDir, "config", &files); err != nil {
			os.Remove(backupPath)
			return nil, fmt.Errorf("failed to backup config directory: %w", err)
		}
	case BackupTypeData:
		if err := m.addDirectory(tarWriter, m.dataDir, "data", &files); err != nil {
			os.Remove(backupPath)
			return nil, fmt.Errorf("failed to backup data directory: %w", err)
		}
	}

	// Close writers to flush data
	tarWriter.Close()
	gzWriter.Close()
	file.Close()

	// Calculate checksum
	checksum, err := m.calculateChecksum(backupPath)
	if err != nil {
		os.Remove(backupPath)
		return nil, fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Get file size
	stat, err := os.Stat(backupPath)
	if err != nil {
		os.Remove(backupPath)
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	info := &BackupInfo{
		ID:        id,
		CreatedAt: timestamp,
		SizeBytes: stat.Size(),
		Path:      backupPath,
		Type:      string(backupType),
		Checksum:  checksum,
		Version:   "0.5.0",
		Files:     files,
	}

	m.backups[id] = info

	// Save metadata
	if err := m.saveMetadata(info); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return info, nil
}

// List returns all available backups
func (m *Manager) List() []*BackupInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	backups := make([]*BackupInfo, 0, len(m.backups))
	for _, b := range m.backups {
		backups = append(backups, b)
	}

	// Sort by creation time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	return backups
}

// Get returns a specific backup by ID
func (m *Manager) Get(id string) (*BackupInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, ok := m.backups[id]
	if !ok {
		return nil, fmt.Errorf("backup not found: %s", id)
	}
	return info, nil
}

// Delete removes a backup
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.backups[id]
	if !ok {
		return fmt.Errorf("backup not found: %s", id)
	}

	// Remove backup file
	if err := os.Remove(info.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove backup file: %w", err)
	}

	// Remove metadata file
	metadataPath := info.Path + ".json"
	os.Remove(metadataPath)

	delete(m.backups, id)
	return nil
}

// Cleanup removes backups older than retention period
func (m *Manager) Cleanup() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	removed := 0

	for id, info := range m.backups {
		if info.CreatedAt.Before(cutoff) {
			if err := os.Remove(info.Path); err != nil && !os.IsNotExist(err) {
				continue
			}
			metadataPath := info.Path + ".json"
			os.Remove(metadataPath)
			delete(m.backups, id)
			removed++
		}
	}

	return removed, nil
}

// Verify checks the integrity of a backup
func (m *Manager) Verify(id string) error {
	m.mu.RLock()
	info, ok := m.backups[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("backup not found: %s", id)
	}

	// Check file exists
	if _, err := os.Stat(info.Path); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Verify checksum
	checksum, err := m.calculateChecksum(info.Path)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	if checksum != info.Checksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", info.Checksum, checksum)
	}

	return nil
}

func (m *Manager) addDirectory(tw *tar.Writer, srcDir, prefix string, files *[]string) error {
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist, skip
	}

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symbolic links to avoid issues
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Skip special files (devices, sockets, etc.)
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// For regular files, read content first to get accurate size
		// This prevents "write too long" errors when file changes during backup
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				// Skip files we can't open (permission issues, etc.)
				return nil
			}
			defer file.Close()

			// Read file content into memory (for small files) or get accurate size
			content, err := io.ReadAll(file)
			if err != nil {
				// Skip files we can't read
				return nil
			}

			// Create header with accurate size
			header := &tar.Header{
				Name:    filepath.Join(prefix, relPath),
				Mode:    int64(info.Mode().Perm()),
				Size:    int64(len(content)),
				ModTime: info.ModTime(),
			}
			*files = append(*files, header.Name)

			if err := tw.WriteHeader(header); err != nil {
				return err
			}

			if _, err := tw.Write(content); err != nil {
				return err
			}
		} else if info.IsDir() {
			// Handle directories
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = filepath.Join(prefix, relPath)
			*files = append(*files, header.Name)

			if err := tw.WriteHeader(header); err != nil {
				return err
			}
		}

		return nil
	})
}

func (m *Manager) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (m *Manager) loadBackups() error {
	entries, err := os.ReadDir(m.config.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		metadataPath := filepath.Join(m.config.Path, entry.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue
		}

		var info BackupInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		// Verify backup file exists
		if _, err := os.Stat(info.Path); err == nil {
			m.backups[info.ID] = &info
		}
	}

	return nil
}

func (m *Manager) saveMetadata(info *BackupInfo) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}

	metadataPath := info.Path + ".json"
	return os.WriteFile(metadataPath, data, 0644)
}

// RepairChecksum recalculates and updates the checksum for a backup
func (m *Manager) RepairChecksum(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, ok := m.backups[id]
	if !ok {
		return fmt.Errorf("backup not found: %s", id)
	}

	// Check file exists
	if _, err := os.Stat(info.Path); err != nil {
		return fmt.Errorf("backup file not found: %w", err)
	}

	// Recalculate checksum
	newChecksum, err := m.calculateChecksum(info.Path)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Update checksum
	info.Checksum = newChecksum

	// Save updated metadata
	if err := m.saveMetadata(info); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}
