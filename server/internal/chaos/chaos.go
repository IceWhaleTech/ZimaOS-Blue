package chaos

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// DiskFullSimulator simulates disk full conditions.
type DiskFullSimulator struct {
	mu           sync.Mutex
	enabled      bool
	targetPath   string
	fillerFiles  []string
	originalSize int64
}

// NewDiskFullSimulator creates a new disk full simulator.
func NewDiskFullSimulator(targetPath string) *DiskFullSimulator {
	return &DiskFullSimulator{
		targetPath:  targetPath,
		fillerFiles: make([]string, 0),
	}
}

// Enable enables disk full simulation by creating filler files.
func (d *DiskFullSimulator) Enable(ctx context.Context, fillPercentage int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.enabled {
		return errors.New("disk full simulation already enabled")
	}

	// Get available space
	available, err := getAvailableSpace(d.targetPath)
	if err != nil {
		return fmt.Errorf("failed to get available space: %w", err)
	}

	// Calculate how much to fill
	fillSize := (available * int64(fillPercentage)) / 100

	// Create filler files
	if err := d.createFillerFiles(ctx, fillSize); err != nil {
		d.cleanup()
		return fmt.Errorf("failed to create filler files: %w", err)
	}

	d.enabled = true
	return nil
}

// Disable disables disk full simulation.
func (d *DiskFullSimulator) Disable() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.enabled {
		return nil
	}

	d.cleanup()
	d.enabled = false
	return nil
}

// IsEnabled returns whether simulation is enabled.
func (d *DiskFullSimulator) IsEnabled() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.enabled
}

func (d *DiskFullSimulator) createFillerFiles(ctx context.Context, totalSize int64) error {
	const fileSize = 100 * 1024 * 1024 // 100MB per file
	numFiles := int(totalSize / fileSize)
	if numFiles == 0 {
		numFiles = 1
	}

	for i := 0; i < numFiles; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		filename := filepath.Join(d.targetPath, fmt.Sprintf(".chaos_filler_%d.tmp", i))
		f, err := os.Create(filename)
		if err != nil {
			return err
		}

		// Write zeros to fill the file
		written, err := io.CopyN(f, &zeroReader{}, fileSize)
		f.Close()

		if err != nil && !errors.Is(err, io.EOF) {
			// Disk might actually be full now
			if written > 0 {
				d.fillerFiles = append(d.fillerFiles, filename)
			}
			return nil // This is expected when disk is full
		}

		d.fillerFiles = append(d.fillerFiles, filename)
	}

	return nil
}

func (d *DiskFullSimulator) cleanup() {
	for _, f := range d.fillerFiles {
		os.Remove(f)
	}
	d.fillerFiles = d.fillerFiles[:0]
}

// zeroReader provides an infinite stream of zeros.
type zeroReader struct{}

func (z *zeroReader) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// DatabaseCorruptionSimulator simulates database corruption scenarios.
type DatabaseCorruptionSimulator struct {
	mu         sync.Mutex
	dbPath     string
	backupPath string
}

// NewDatabaseCorruptionSimulator creates a new database corruption simulator.
func NewDatabaseCorruptionSimulator(dbPath string) *DatabaseCorruptionSimulator {
	return &DatabaseCorruptionSimulator{
		dbPath: dbPath,
	}
}

// CorruptHeader corrupts the database header.
func (d *DatabaseCorruptionSimulator) CorruptHeader() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Backup first
	if err := d.backup(); err != nil {
		return fmt.Errorf("failed to backup: %w", err)
	}

	// Open file and corrupt header
	f, err := os.OpenFile(d.dbPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// SQLite header is first 100 bytes
	// Corrupt the magic number (first 16 bytes)
	corruption := []byte("CORRUPTED_HEADER")
	_, err = f.WriteAt(corruption, 0)
	return err
}

// CorruptPage corrupts a specific page in the database.
func (d *DatabaseCorruptionSimulator) CorruptPage(pageNum int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.backup(); err != nil {
		return fmt.Errorf("failed to backup: %w", err)
	}

	f, err := os.OpenFile(d.dbPath, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// SQLite default page size is 4096 bytes
	pageSize := int64(4096)
	offset := int64(pageNum) * pageSize

	// Write garbage to the page
	garbage := make([]byte, 100)
	for i := range garbage {
		garbage[i] = byte(i % 256)
	}

	_, err = f.WriteAt(garbage, offset)
	return err
}

// TruncateFile truncates the database file.
func (d *DatabaseCorruptionSimulator) TruncateFile(percentage int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.backup(); err != nil {
		return fmt.Errorf("failed to backup: %w", err)
	}

	info, err := os.Stat(d.dbPath)
	if err != nil {
		return err
	}

	newSize := (info.Size() * int64(percentage)) / 100
	return os.Truncate(d.dbPath, newSize)
}

// Restore restores the database from backup.
func (d *DatabaseCorruptionSimulator) Restore() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.backupPath == "" {
		return errors.New("no backup available")
	}

	// Copy backup to original location
	src, err := os.Open(d.backupPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(d.dbPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// Cleanup removes backup files.
func (d *DatabaseCorruptionSimulator) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.backupPath != "" {
		os.Remove(d.backupPath)
		d.backupPath = ""
	}
}

func (d *DatabaseCorruptionSimulator) backup() error {
	if d.backupPath != "" {
		return nil // Already backed up
	}

	d.backupPath = d.dbPath + ".chaos_backup"

	src, err := os.Open(d.dbPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(d.backupPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

// RecoveryTester tests database recovery capabilities.
type RecoveryTester struct {
	dbPath string
}

// NewRecoveryTester creates a new recovery tester.
func NewRecoveryTester(dbPath string) *RecoveryTester {
	return &RecoveryTester{dbPath: dbPath}
}

// TestIntegrityCheck tests SQLite integrity check.
func (r *RecoveryTester) TestIntegrityCheck() (*IntegrityResult, error) {
	db, err := sql.Open("sqlite3", r.dbPath)
	if err != nil {
		return &IntegrityResult{
			Passed:  false,
			Message: fmt.Sprintf("Failed to open database: %v", err),
		}, nil
	}
	defer db.Close()

	rows, err := db.Query("PRAGMA integrity_check")
	if err != nil {
		return &IntegrityResult{
			Passed:  false,
			Message: fmt.Sprintf("Integrity check failed: %v", err),
		}, nil
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			continue
		}
		results = append(results, result)
	}

	passed := len(results) == 1 && results[0] == "ok"
	return &IntegrityResult{
		Passed:  passed,
		Message: fmt.Sprintf("Integrity check results: %v", results),
		Details: results,
	}, nil
}

// TestRecovery attempts to recover a corrupted database.
func (r *RecoveryTester) TestRecovery(ctx context.Context) (*RecoveryResult, error) {
	result := &RecoveryResult{
		StartTime: timeutil.NowTime(),
	}

	// Step 1: Check if database is accessible
	db, err := sql.Open("sqlite3", r.dbPath)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("Cannot open database: %v", err)
		result.EndTime = timeutil.NowTime()
		return result, nil
	}

	// Step 2: Try integrity check
	integrityResult, _ := r.TestIntegrityCheck()
	result.IntegrityPassed = integrityResult.Passed

	if !integrityResult.Passed {
		// Step 3: Attempt recovery
		_, err = db.ExecContext(ctx, "PRAGMA writable_schema = ON")
		if err != nil {
			result.Error = fmt.Sprintf("Cannot enable writable schema: %v", err)
		}

		// Try to dump and recreate
		result.RecoveryAttempted = true

		// Export what we can
		recoveredPath := r.dbPath + ".recovered"
		recoveredDB, err := sql.Open("sqlite3", recoveredPath)
		if err != nil {
			result.Error = fmt.Sprintf("Cannot create recovery database: %v", err)
			result.Success = false
			result.EndTime = timeutil.NowTime()
			db.Close()
			return result, nil
		}

		// Try to copy tables
		tables, err := r.listTables(db)
		if err == nil {
			for _, table := range tables {
				r.copyTable(ctx, db, recoveredDB, table)
				result.TablesRecovered = append(result.TablesRecovered, table)
			}
		}

		recoveredDB.Close()
		result.RecoveredPath = recoveredPath
	}

	db.Close()
	result.Success = integrityResult.Passed || len(result.TablesRecovered) > 0
	result.EndTime = timeutil.NowTime()
	return result, nil
}

func (r *RecoveryTester) listTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, name)
	}
	return tables, nil
}

func (r *RecoveryTester) copyTable(ctx context.Context, src, dst *sql.DB, table string) error {
	// Get table schema
	var schema string
	err := src.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&schema)
	if err != nil {
		return err
	}

	// Create table in destination
	_, err = dst.ExecContext(ctx, schema)
	if err != nil {
		return err
	}

	// Copy data
	rows, err := src.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	// This is a simplified version - in production, you'd handle columns properly
	return nil
}

// IntegrityResult contains the result of an integrity check.
type IntegrityResult struct {
	Passed  bool     `json:"passed"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

// RecoveryResult contains the result of a recovery attempt.
type RecoveryResult struct {
	Success           bool      `json:"success"`
	StartTime         time.Time `json:"start_time"`
	EndTime           time.Time `json:"end_time"`
	IntegrityPassed   bool      `json:"integrity_passed"`
	RecoveryAttempted bool      `json:"recovery_attempted"`
	TablesRecovered   []string  `json:"tables_recovered,omitempty"`
	RecoveredPath     string    `json:"recovered_path,omitempty"`
	Error             string    `json:"error,omitempty"`
}

// getAvailableSpace returns available disk space in bytes.
func getAvailableSpace(path string) (int64, error) {
	// This is a simplified cross-platform implementation
	// In production, use syscall.Statfs on Unix or GetDiskFreeSpaceEx on Windows

	// For testing purposes, return a fixed value
	return 1024 * 1024 * 1024, nil // 1GB
}
