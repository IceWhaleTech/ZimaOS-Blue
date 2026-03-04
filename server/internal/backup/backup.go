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
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// Config holds backup configuration
type Config struct {
	Enabled            bool          `yaml:"enabled"`
	Schedule           string        `yaml:"schedule"` // cron expression
	RetentionDays      int           `yaml:"retention_days"`
	Path               string        `yaml:"path"`
	SkillsPath         string        `yaml:"skills_path"`
	AutoBackup         bool          `yaml:"auto_backup"`
	AutoBackupInterval time.Duration `yaml:"auto_backup_interval"`
	AutoBackupOnChange bool          `yaml:"auto_backup_on_change"`
	ChangePollInterval time.Duration `yaml:"change_poll_interval"`
	ChangeDebounce     time.Duration `yaml:"change_debounce"`
}

// BackupInfo contains metadata about a backup
type BackupInfo struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	SizeBytes int64     `json:"size_bytes"`
	Path      string    `json:"path"`
	Type      string    `json:"type"` // full, config, data
	// CreatedBy indicates who created this backup: manual, auto, checkpoint.
	CreatedBy string   `json:"created_by,omitempty"`
	Checksum  string   `json:"checksum"`
	Version   string   `json:"version"`
	Files     []string `json:"files"`
	// IsCheckpoint marks backups created automatically before restore.
	IsCheckpoint bool `json:"is_checkpoint,omitempty"`
	// CheckpointReason indicates why the checkpoint was created.
	CheckpointReason string `json:"checkpoint_reason,omitempty"`
}

// BackupType represents the type of backup
type BackupType string

const (
	BackupTypeFull   BackupType = "full"
	BackupTypeConfig BackupType = "config"
	BackupTypeData   BackupType = "data"
)

type BackupSource string

const (
	BackupSourceManual     BackupSource = "manual"
	BackupSourceAuto       BackupSource = "auto"
	BackupSourceCheckpoint BackupSource = "checkpoint"
)

// Manager handles backup and restore operations
type Manager struct {
	config    Config
	dataDir   string
	configDir string
	skillsDir string

	mu      sync.RWMutex
	backups map[string]*BackupInfo

	// Progress tracking
	progressMu sync.RWMutex
	progress   *Progress

	// Auto-backup tracking
	autoMu         sync.Mutex
	autoCancel     context.CancelFunc
	autoDone       chan struct{}
	lastSnapshot   string
	lastAutoBackup time.Time
	pendingChange  bool
}

var (
	excludedBackupDirs = map[string]bool{
		"backups":     true, // Backup directory itself
		"cache":       true, // Runtime cache
		"log":         true, // Runtime logs
		"logs":        true, // Runtime logs
		"models":      true, // Generic models directory
		"onnxruntime": true, // ONNX Runtime files (large, can be re-downloaded)
	}
	excludedBackupExtensions = map[string]bool{
		".bin":         true, // Binary model files (often large)
		".ckpt":        true, // Model checkpoints
		".gguf":        true, // LLM model files
		".log":         true, // Log files
		".onnx":        true, // ONNX model files
		".pt":          true, // PyTorch model files
		".pth":         true, // PyTorch model files
		".safetensors": true, // Model weights
	}
	alwaysIncludeLargeBackupExtensions = map[string]bool{
		".db":      true, // Keep core SQLite data even if large
		".sqlite":  true, // Keep core SQLite data even if large
		".sqlite3": true, // Keep core SQLite data even if large
		".shm":     true, // SQLite shared memory
		".wal":     true, // SQLite write-ahead log
	}
)

const maxBackupFileSizeBytes int64 = 10 * 1024 * 1024 // 10 MiB

func shouldSkipBackupDir(name string) bool {
	return excludedBackupDirs[strings.ToLower(name)]
}

func shouldSkipBackupFile(name string, size int64) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if excludedBackupExtensions[ext] {
		return true
	}
	if size > maxBackupFileSizeBytes && !alwaysIncludeLargeBackupExtensions[ext] {
		return true
	}
	return false
}

// Progress tracks the current backup/restore operation
type Progress struct {
	InProgress     bool      `json:"in_progress"`
	Operation      string    `json:"operation"` // "backup" or "restore"
	ProgressPct    int       `json:"progress"`  // 0-100
	CurrentFile    string    `json:"current_file"`
	FilesProcessed int       `json:"files_processed"`
	TotalFiles     int       `json:"total_files"`
	BytesProcessed int64     `json:"bytes_processed"`
	TotalBytes     int64     `json:"total_bytes"`
	StartedAt      time.Time `json:"started_at"`
	Error          string    `json:"error,omitempty"`
}

// GetProgress returns the current progress
func (m *Manager) GetProgress() *Progress {
	m.progressMu.RLock()
	defer m.progressMu.RUnlock()
	if m.progress == nil {
		return &Progress{InProgress: false}
	}
	// Return a copy
	p := *m.progress
	return &p
}

func (m *Manager) startProgress(operation string) {
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	m.progress = &Progress{
		InProgress: true,
		Operation:  operation,
		StartedAt:  timeutil.NowTime(),
	}
}

func (m *Manager) updateProgress(currentFile string, filesProcessed, totalFiles int, bytesProcessed, totalBytes int64) {
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	if m.progress == nil {
		return
	}
	m.progress.CurrentFile = currentFile
	m.progress.FilesProcessed = filesProcessed
	m.progress.TotalFiles = totalFiles
	m.progress.BytesProcessed = bytesProcessed
	m.progress.TotalBytes = totalBytes
	if totalFiles > 0 {
		m.progress.ProgressPct = (filesProcessed * 100) / totalFiles
	} else if totalBytes > 0 {
		m.progress.ProgressPct = int((bytesProcessed * 100) / totalBytes)
	}
}

func (m *Manager) endProgress(err error) {
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	if m.progress == nil {
		return
	}
	m.progress.InProgress = false
	m.progress.ProgressPct = 100
	if err != nil {
		m.progress.Error = err.Error()
	}
}

// NewManager creates a new backup manager
func NewManager(cfg Config, dataDir, configDir string) (*Manager, error) {
	if cfg.Path == "" {
		cfg.Path = "./backups"
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 7
	}
	if cfg.SkillsPath == "" {
		cfg.SkillsPath = filepath.Join(dataDir, "workspace", ".claude", "skills")
	}
	if cfg.AutoBackupInterval <= 0 {
		cfg.AutoBackupInterval = 6 * time.Hour
	}
	if cfg.ChangePollInterval <= 0 {
		cfg.ChangePollInterval = time.Minute
	}
	if cfg.ChangeDebounce < 0 {
		cfg.ChangeDebounce = 0
	}

	// Ensure backup directory exists
	// Security: Use 0700 to prevent other users from reading backup files
	if err := os.MkdirAll(cfg.Path, 0700); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	m := &Manager{
		config:    cfg,
		dataDir:   dataDir,
		configDir: configDir,
		skillsDir: cfg.SkillsPath,
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
	return m.createWithSource(ctx, backupType, BackupSourceManual)
}

func (m *Manager) createWithSource(ctx context.Context, backupType BackupType, source BackupSource) (*BackupInfo, error) {
	if source == "" {
		source = BackupSourceManual
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Start progress tracking
	m.startProgress("backup")
	defer func() {
		// Will be called with nil if successful
	}()

	id := uuid.New().String()
	timestamp := timeutil.NowTime()
	filename := fmt.Sprintf("backup_%s_%s_%s.tar.gz", backupType, timestamp.Format("20060102_150405"), id[:8])
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
		if m.shouldBackupSkillsIndependently() {
			if err := m.addDirectory(tarWriter, m.skillsDir, "skills", &files); err != nil {
				os.Remove(backupPath)
				return nil, fmt.Errorf("failed to backup skills directory: %w", err)
			}
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
		if m.shouldBackupSkillsIndependently() {
			if err := m.addDirectory(tarWriter, m.skillsDir, "skills", &files); err != nil {
				os.Remove(backupPath)
				return nil, fmt.Errorf("failed to backup skills directory: %w", err)
			}
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
		CreatedBy: string(source),
		Checksum:  checksum,
		Version:   "0.5.0",
		Files:     files,
	}

	m.backups[id] = info

	// Save metadata
	if err := m.saveMetadata(info); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	// End progress tracking
	m.endProgress(nil)

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

	return m.removeBackupLocked(id, info)
}

// Cleanup removes backups older than retention period
func (m *Manager) Cleanup() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := timeutil.NowTime().AddDate(0, 0, -m.config.RetentionDays)
	removed := 0

	for id, info := range m.backups {
		if info.CreatedAt.Before(cutoff) {
			if err := m.removeBackupLocked(id, info); err != nil {
				continue
			}
			removed++
		}
	}

	return removed, nil
}

// CleanupAutoBackupsKeepLatest keeps only the newest auto backup and removes older auto backups.
func (m *Manager) CleanupAutoBackupsKeepLatest() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	autoBackups := make([]*BackupInfo, 0)
	for _, info := range m.backups {
		if info.CreatedBy == string(BackupSourceAuto) {
			autoBackups = append(autoBackups, info)
		}
	}
	if len(autoBackups) <= 1 {
		return 0, nil
	}

	sort.Slice(autoBackups, func(i, j int) bool {
		return autoBackups[i].CreatedAt.After(autoBackups[j].CreatedAt)
	})

	removed := 0
	for _, info := range autoBackups[1:] {
		if err := m.removeBackupLocked(info.ID, info); err != nil {
			continue
		}
		removed++
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

	// Use queue-based iteration instead of recursive walk
	queue := []string{srcDir}
	filesProcessed := 0

	for len(queue) > 0 {
		currentDir := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(currentDir)
		if err != nil {
			continue // Skip directories we can't read
		}

		for _, entry := range entries {
			// Skip excluded directories
			if entry.IsDir() && shouldSkipBackupDir(entry.Name()) {
				continue
			}

			path := filepath.Join(currentDir, entry.Name())

			info, err := entry.Info()
			if err != nil {
				continue // Skip files we can't stat
			}

			// Skip symbolic links to avoid issues
			if info.Mode()&os.ModeSymlink != 0 {
				continue
			}

			// Skip special files (devices, sockets, etc.)
			if !info.Mode().IsRegular() && !info.IsDir() {
				continue
			}

			// Skip files with excluded extensions
			if info.Mode().IsRegular() {
				if shouldSkipBackupFile(entry.Name(), info.Size()) {
					continue
				}
			}

			// Get relative path
			relPath, err := filepath.Rel(srcDir, path)
			if err != nil {
				continue
			}

			// Update progress
			filesProcessed++
			m.updateProgress(relPath, filesProcessed, 0, 0, 0)

			if info.IsDir() {
				// Add directory to queue for processing
				queue = append(queue, path)

				// Handle directories
				header, err := tar.FileInfoHeader(info, "")
				if err != nil {
					continue
				}
				header.Name = filepath.Join(prefix, relPath)
				*files = append(*files, header.Name)

				if err := tw.WriteHeader(header); err != nil {
					return err
				}
			} else if info.Mode().IsRegular() {
				// For regular files, stream content directly to tar writer
				file, err := os.Open(path)
				if err != nil {
					continue // Skip files we can't open
				}

				// Get current file size
				stat, err := file.Stat()
				if err != nil {
					file.Close()
					continue
				}
				fileSize := stat.Size()

				header := &tar.Header{
					Name:    filepath.Join(prefix, relPath),
					Mode:    int64(info.Mode().Perm()),
					Size:    fileSize,
					ModTime: stat.ModTime(),
				}
				*files = append(*files, header.Name)

				if err := tw.WriteHeader(header); err != nil {
					file.Close()
					return err
				}

				// Stream file content directly to tar writer
				// Use LimitReader to prevent writing more than declared size
				// This handles the case where file grows during backup
				written, err := io.Copy(tw, io.LimitReader(file, fileSize))
				file.Close()

				if err != nil {
					return fmt.Errorf("failed to write file %s: %w", path, err)
				}

				// If file was truncated during copy, pad with zeros
				if written < fileSize {
					padding := make([]byte, fileSize-written)
					if _, err := tw.Write(padding); err != nil {
						return fmt.Errorf("failed to pad file %s: %w", path, err)
					}
				}
			}
		}
	}

	return nil
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
		if info.CreatedBy == "" {
			if info.IsCheckpoint {
				info.CreatedBy = string(BackupSourceCheckpoint)
			} else {
				info.CreatedBy = string(BackupSourceManual)
			}
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

// CreateCheckpoint creates a full backup checkpoint of the current state.
func (m *Manager) CreateCheckpoint(ctx context.Context, reason string) (*BackupInfo, error) {
	if reason == "" {
		reason = "pre_restore"
	}

	info, err := m.createWithSource(ctx, BackupTypeFull, BackupSourceCheckpoint)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	stored := m.backups[info.ID]
	if stored == nil {
		return info, nil
	}
	stored.IsCheckpoint = true
	stored.CheckpointReason = reason
	stored.CreatedBy = string(BackupSourceCheckpoint)
	if err := m.saveMetadata(stored); err != nil {
		return nil, err
	}
	return stored, nil
}

func (m *Manager) removeBackupLocked(id string, info *BackupInfo) error {
	if err := os.Remove(info.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove backup file: %w", err)
	}
	metadataPath := info.Path + ".json"
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove backup metadata: %w", err)
	}
	delete(m.backups, id)
	return nil
}

func (m *Manager) shouldBackupSkillsIndependently() bool {
	if m.skillsDir == "" {
		return false
	}
	if _, err := os.Stat(m.skillsDir); err != nil {
		return false
	}
	// skills dir is already covered by data backup.
	if isSubPath(m.dataDir, m.skillsDir) {
		return false
	}
	return true
}

func isSubPath(parent, child string) bool {
	if parent == "" || child == "" {
		return false
	}
	parentAbs, err := filepath.Abs(parent)
	if err != nil {
		return false
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	parentAbs = filepath.Clean(parentAbs)
	childAbs = filepath.Clean(childAbs)
	if parentAbs == childAbs {
		return true
	}
	prefix := parentAbs + string(os.PathSeparator)
	return strings.HasPrefix(childAbs, prefix)
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
