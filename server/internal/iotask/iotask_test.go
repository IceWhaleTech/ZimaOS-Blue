package iotask

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxConcurrent != 4 {
		t.Errorf("expected MaxConcurrent=4, got %d", config.MaxConcurrent)
	}
	if config.BufferSize != 32*1024 {
		t.Errorf("expected BufferSize=32768, got %d", config.BufferSize)
	}
	if !config.Enabled {
		t.Error("expected Enabled=true by default")
	}
}

func TestNewManager(t *testing.T) {
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	stats := m.Stats()
	if stats.TotalTasks != 0 {
		t.Errorf("expected 0 total tasks, got %d", stats.TotalTasks)
	}
}

func TestCopyFile(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(srcDir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create manager
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	// Copy file
	dstFile := filepath.Join(dstDir, "test.txt")
	task, err := m.Copy(srcFile, dstFile)
	if err != nil {
		t.Fatalf("failed to create copy task: %v", err)
	}

	if task.ID == "" {
		t.Error("expected non-empty task ID")
	}
	if task.Type != TaskTypeCopy {
		t.Errorf("expected type copy, got %s", task.Type)
	}

	// Wait for completion
	for i := 0; i < 50; i++ {
		task, _ = m.GetTask(task.ID)
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s (error: %s)", task.Status, task.Error)
	}

	// Verify destination file
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("content mismatch: expected %s, got %s", content, dstContent)
	}
}

func TestCopyDirectory(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source structure
	subDir := filepath.Join(srcDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	files := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
	}

	for name, content := range files {
		path := filepath.Join(srcDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to create file %s: %v", name, err)
		}
	}

	// Create manager
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	// Copy directory
	dstPath := filepath.Join(dstDir, "copied")
	task, err := m.Copy(srcDir, dstPath)
	if err != nil {
		t.Fatalf("failed to create copy task: %v", err)
	}

	// Wait for completion
	for i := 0; i < 50; i++ {
		task, _ = m.GetTask(task.ID)
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s (error: %s)", task.Status, task.Error)
	}

	// Verify files
	for name, expectedContent := range files {
		path := filepath.Join(dstPath, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("failed to read %s: %v", name, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("content mismatch for %s: expected %s, got %s", name, expectedContent, content)
		}
	}
}

func TestMoveFile(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(srcDir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create manager
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	// Move file
	dstFile := filepath.Join(dstDir, "moved.txt")
	task, err := m.Move(srcFile, dstFile)
	if err != nil {
		t.Fatalf("failed to create move task: %v", err)
	}

	// Wait for completion
	for i := 0; i < 50; i++ {
		task, _ = m.GetTask(task.ID)
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s (error: %s)", task.Status, task.Error)
	}

	// Verify source is gone
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("source file should not exist after move")
	}

	// Verify destination exists
	dstContent, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if string(dstContent) != string(content) {
		t.Errorf("content mismatch: expected %s, got %s", content, dstContent)
	}
}

func TestDeleteFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create file to delete
	filePath := filepath.Join(tmpDir, "delete_me.txt")
	if err := os.WriteFile(filePath, []byte("delete me"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create manager
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	// Delete file
	task, err := m.Delete(filePath)
	if err != nil {
		t.Fatalf("failed to create delete task: %v", err)
	}

	// Wait for completion
	for i := 0; i < 50; i++ {
		task, _ = m.GetTask(task.ID)
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s (error: %s)", task.Status, task.Error)
	}

	// Verify file is gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("file should not exist after delete")
	}
}

func TestDeleteDirectory(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create directory structure to delete
	deleteDir := filepath.Join(tmpDir, "delete_me")
	subDir := filepath.Join(deleteDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}

	// Create files
	files := []string{
		filepath.Join(deleteDir, "file1.txt"),
		filepath.Join(subDir, "file2.txt"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	// Create manager
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	// Delete directory
	task, err := m.Delete(deleteDir)
	if err != nil {
		t.Fatalf("failed to create delete task: %v", err)
	}

	// Wait for completion
	for i := 0; i < 50; i++ {
		task, _ = m.GetTask(task.ID)
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s (error: %s)", task.Status, task.Error)
	}

	// Verify directory is gone
	if _, err := os.Stat(deleteDir); !os.IsNotExist(err) {
		t.Error("directory should not exist after delete")
	}
}

func TestCancelTask(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create a large file to give us time to cancel
	srcFile := filepath.Join(srcDir, "large.txt")
	content := make([]byte, 10*1024*1024) // 10MB
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create manager with rate limiting to slow down copy
	config := DefaultConfig()
	config.RateLimitBytesPerSec = 1024 * 1024 // 1MB/s
	m := New(config)
	defer m.Close()

	// Start copy
	dstFile := filepath.Join(dstDir, "large.txt")
	task, err := m.Copy(srcFile, dstFile)
	if err != nil {
		t.Fatalf("failed to create copy task: %v", err)
	}

	// Wait for task to start
	time.Sleep(100 * time.Millisecond)

	// Cancel task
	if err := m.Cancel(task.ID); err != nil {
		t.Fatalf("failed to cancel task: %v", err)
	}

	// Wait a bit for cancellation to take effect
	time.Sleep(200 * time.Millisecond)

	// Check status
	task, _ = m.GetTask(task.ID)
	if task.Status != TaskStatusCancelled {
		t.Errorf("expected status cancelled, got %s", task.Status)
	}
}

func TestCancelNonExistentTask(t *testing.T) {
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	err := m.Cancel("non-existent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestListTasks(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source files
	for i := 0; i < 3; i++ {
		srcFile := filepath.Join(srcDir, "file"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create source file: %v", err)
		}
	}

	// Create manager
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	// Create multiple tasks
	for i := 0; i < 3; i++ {
		srcFile := filepath.Join(srcDir, "file"+string(rune('0'+i))+".txt")
		dstFile := filepath.Join(dstDir, "file"+string(rune('0'+i))+".txt")
		_, err := m.Copy(srcFile, dstFile)
		if err != nil {
			t.Fatalf("failed to create copy task: %v", err)
		}
	}

	tasks := m.ListTasks()
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}
}

func TestManagerStats(t *testing.T) {
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	stats := m.Stats()
	if stats.TotalTasks != 0 {
		t.Errorf("expected 0 total tasks, got %d", stats.TotalTasks)
	}
	if stats.MaxConcurrent != config.MaxConcurrent {
		t.Errorf("expected max concurrent %d, got %d", config.MaxConcurrent, stats.MaxConcurrent)
	}
}

func TestManagerClosed(t *testing.T) {
	config := DefaultConfig()
	m := New(config)
	m.Close()

	_, err := m.Copy("/src", "/dst")
	if err != ErrManagerClosed {
		t.Errorf("expected ErrManagerClosed, got %v", err)
	}
}

func TestWithMetadata(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(srcDir, "test.txt")
	if err := os.WriteFile(srcFile, []byte("content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create manager
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	// Create task with metadata
	dstFile := filepath.Join(dstDir, "test.txt")
	task, err := m.Copy(srcFile, dstFile,
		WithMetadata("key1", "value1"),
		WithMetadata("key2", "value2"),
	)
	if err != nil {
		t.Fatalf("failed to create copy task: %v", err)
	}

	if task.Metadata["key1"] != "value1" {
		t.Errorf("expected metadata key1=value1, got %s", task.Metadata["key1"])
	}
	if task.Metadata["key2"] != "value2" {
		t.Errorf("expected metadata key2=value2, got %s", task.Metadata["key2"])
	}
}

func TestProgressTracking(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source file with known size
	srcFile := filepath.Join(srcDir, "test.txt")
	content := make([]byte, 1024*100) // 100KB
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create manager
	config := DefaultConfig()
	m := New(config)
	defer m.Close()

	// Copy file
	dstFile := filepath.Join(dstDir, "test.txt")
	task, err := m.Copy(srcFile, dstFile)
	if err != nil {
		t.Fatalf("failed to create copy task: %v", err)
	}

	// Wait for completion
	for i := 0; i < 50; i++ {
		task, _ = m.GetTask(task.ID)
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s", task.Status)
	}

	// Check progress
	if task.Progress.TotalBytes != int64(len(content)) {
		t.Errorf("expected total bytes %d, got %d", len(content), task.Progress.TotalBytes)
	}
	if task.Progress.ProcessedBytes != int64(len(content)) {
		t.Errorf("expected processed bytes %d, got %d", len(content), task.Progress.ProcessedBytes)
	}
	if task.Progress.Percentage != 100 {
		t.Errorf("expected percentage 100, got %f", task.Progress.Percentage)
	}
}

func TestProgressReader(t *testing.T) {
	content := []byte("Hello, World!")
	reader := NewProgressReader(
		&mockReader{data: content},
		int64(len(content)),
		nil,
	)

	buf := make([]byte, 5)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes read, got %d", n)
	}
	if reader.Processed() != 5 {
		t.Errorf("expected 5 processed, got %d", reader.Processed())
	}

	expectedPercentage := float64(5) / float64(len(content)) * 100
	if reader.Percentage() != expectedPercentage {
		t.Errorf("expected percentage %f, got %f", expectedPercentage, reader.Percentage())
	}
}

type mockReader struct {
	data   []byte
	offset int
}

func (r *mockReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, context.Canceled
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

func TestRateLimiting(t *testing.T) {
	// Create temp directories
	srcDir := t.TempDir()
	dstDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(srcDir, "test.txt")
	content := make([]byte, 1024*100) // 100KB
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Create manager with rate limiting
	config := DefaultConfig()
	config.RateLimitBytesPerSec = 50 * 1024 // 50KB/s
	m := New(config)
	defer m.Close()

	// Copy file
	dstFile := filepath.Join(dstDir, "test.txt")
	task, err := m.Copy(srcFile, dstFile)
	if err != nil {
		t.Fatalf("failed to create copy task: %v", err)
	}

	startTime := time.Now()

	// Wait for completion
	for i := 0; i < 100; i++ {
		task, _ = m.GetTask(task.ID)
		if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	elapsed := time.Since(startTime)

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s (error: %s)", task.Status, task.Error)
	}

	// With 100KB file and 50KB/s rate limit, should take at least 1.5 seconds
	// (accounting for some overhead, we check for at least 1 second)
	if elapsed < time.Second {
		t.Errorf("rate limiting not working: copy took only %v", elapsed)
	}
}
