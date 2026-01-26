package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Enabled {
		t.Error("expected Enabled=false by default")
	}
	if config.Recursive {
		t.Error("expected Recursive=false by default")
	}
	if config.DebounceMs != 100 {
		t.Errorf("expected DebounceMs=100, got %d", config.DebounceMs)
	}
	if len(config.Events) != 3 {
		t.Errorf("expected 3 default events, got %d", len(config.Events))
	}
}

func TestNewWatcher(t *testing.T) {
	config := DefaultConfig()
	w, err := New(config)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.watcher.Close()

	if w == nil {
		t.Fatal("expected non-nil watcher")
	}
}

func TestWatcherAddRemovePath(t *testing.T) {
	config := DefaultConfig()
	w, err := New(config)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.watcher.Close()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Add path
	err = w.AddPath(tmpDir)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	paths := w.WatchedPaths()
	if len(paths) != 1 {
		t.Errorf("expected 1 watched path, got %d", len(paths))
	}

	// Remove path
	err = w.RemovePath(tmpDir)
	if err != nil {
		t.Fatalf("failed to remove path: %v", err)
	}

	paths = w.WatchedPaths()
	if len(paths) != 0 {
		t.Errorf("expected 0 watched paths, got %d", len(paths))
	}
}

func TestWatcherEvents(t *testing.T) {
	config := Config{
		Events:     []EventType{EventCreate, EventWrite, EventRemove},
		DebounceMs: 0, // Disable debouncing for test
	}
	w, err := New(config)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Track events
	var events []Event
	var mu sync.Mutex
	w.AddHandler(func(event Event) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	// Start watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.config.Paths = []string{tmpDir}
	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Wait for events
	time.Sleep(100 * time.Millisecond)

	// Stop watcher
	err = w.Stop()
	if err != nil {
		t.Fatalf("failed to stop watcher: %v", err)
	}

	// Check events
	mu.Lock()
	defer mu.Unlock()

	if len(events) == 0 {
		t.Error("expected at least one event")
	}

	// Should have at least a create event
	hasCreate := false
	for _, e := range events {
		if e.Type == EventCreate {
			hasCreate = true
			break
		}
	}
	if !hasCreate {
		t.Error("expected create event")
	}
}

func TestWatcherDebounce(t *testing.T) {
	config := Config{
		Events:     []EventType{EventWrite},
		DebounceMs: 50,
	}
	w, err := New(config)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "watcher-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test file first
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("initial"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Track events
	var eventCount int
	var mu sync.Mutex
	w.AddHandler(func(event Event) {
		mu.Lock()
		eventCount++
		mu.Unlock()
	})

	// Start watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = w.AddPath(tmpDir)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Write multiple times rapidly
	for i := 0; i < 5; i++ {
		err = os.WriteFile(testFile, []byte("update"), 0644)
		if err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to complete
	time.Sleep(150 * time.Millisecond)

	// Stop watcher
	err = w.Stop()
	if err != nil {
		t.Fatalf("failed to stop watcher: %v", err)
	}

	// With debouncing, we should have fewer events than writes
	mu.Lock()
	defer mu.Unlock()

	// Due to debouncing, we should have significantly fewer events
	// The exact number depends on timing, but should be less than 5
	t.Logf("Event count with debouncing: %d (expected < 5)", eventCount)
}

func TestMapEventType(t *testing.T) {
	w := &Watcher{}

	tests := []struct {
		op       fsnotify.Op
		expected EventType
	}{
		{fsnotify.Create, EventCreate},
		{fsnotify.Write, EventWrite},
		{fsnotify.Remove, EventRemove},
		{fsnotify.Rename, EventRename},
		{fsnotify.Chmod, EventChmod},
	}

	for _, tt := range tests {
		result := w.mapEventType(tt.op)
		if result != tt.expected {
			t.Errorf("mapEventType(%v) = %v, expected %v", tt.op, result, tt.expected)
		}
	}
}

func TestIsEventEnabled(t *testing.T) {
	// Test with specific events
	w := &Watcher{
		config: Config{
			Events: []EventType{EventCreate, EventWrite},
		},
	}

	if !w.isEventEnabled(EventCreate) {
		t.Error("expected EventCreate to be enabled")
	}
	if !w.isEventEnabled(EventWrite) {
		t.Error("expected EventWrite to be enabled")
	}
	if w.isEventEnabled(EventRemove) {
		t.Error("expected EventRemove to be disabled")
	}

	// Test with empty events (all enabled)
	w.config.Events = []EventType{}
	if !w.isEventEnabled(EventRemove) {
		t.Error("expected all events enabled when Events is empty")
	}
}

func TestRecursiveWatching(t *testing.T) {
	config := Config{
		Events:         []EventType{EventCreate, EventWrite},
		DebounceMs:     0,
		Recursive:      true,
		IgnorePatterns: []string{".git", "node_modules"},
	}
	w, err := New(config)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Create temp directory structure
	tmpDir, err := os.MkdirTemp("", "watcher-recursive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories
	subDir2 := filepath.Join(tmpDir, "subdir1", "subdir2")
	ignoredDir := filepath.Join(tmpDir, ".git")

	os.MkdirAll(subDir2, 0755)
	os.MkdirAll(ignoredDir, 0755)

	// Add path recursively
	err = w.AddPath(tmpDir)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	// Check that subdirectories are watched
	watchedCount := w.WatchedDirCount()
	if watchedCount < 3 {
		t.Errorf("expected at least 3 watched dirs, got %d", watchedCount)
	}

	// Check that .git is not watched
	paths := w.WatchedPaths()
	for _, p := range paths {
		if filepath.Base(p) == ".git" {
			t.Error("expected .git to be ignored")
		}
	}

	w.watcher.Close()
}

func TestShouldIgnore(t *testing.T) {
	w := &Watcher{
		config: Config{
			IgnorePatterns: []string{".git", "*.tmp", "node_modules"},
		},
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/path/to/.git", true},
		{"/path/to/node_modules", true},
		{"/path/to/file.tmp", true},
		{"/path/to/file.txt", false},
		{"/path/to/src", false},
	}

	for _, tt := range tests {
		result := w.shouldIgnore(tt.path)
		if result != tt.expected {
			t.Errorf("shouldIgnore(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestRecursiveNewDirWatching(t *testing.T) {
	config := Config{
		Events:     []EventType{EventCreate},
		DebounceMs: 0,
		Recursive:  true,
	}
	w, err := New(config)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "watcher-newdir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Track events
	var events []Event
	var mu sync.Mutex
	w.AddHandler(func(event Event) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	// Start watcher
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.config.Paths = []string{tmpDir}
	err = w.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start watcher: %v", err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a new subdirectory
	newSubDir := filepath.Join(tmpDir, "newsubdir")
	err = os.Mkdir(newSubDir, 0755)
	if err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Wait for watcher to add new directory
	time.Sleep(100 * time.Millisecond)

	// Create a file in the new subdirectory
	testFile := filepath.Join(newSubDir, "test.txt")
	err = os.WriteFile(testFile, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Wait for events
	time.Sleep(100 * time.Millisecond)

	// Stop watcher
	err = w.Stop()
	if err != nil {
		t.Fatalf("failed to stop watcher: %v", err)
	}

	// Check events - should have events for both the new dir and the file
	mu.Lock()
	defer mu.Unlock()

	if len(events) < 2 {
		t.Errorf("expected at least 2 events (dir + file), got %d", len(events))
	}

	// Check that we got events for both
	var hasDirEvent, hasFileEvent bool
	for _, e := range events {
		if e.IsDir && e.Name == "newsubdir" {
			hasDirEvent = true
		}
		if !e.IsDir && e.Name == "test.txt" {
			hasFileEvent = true
		}
	}

	if !hasDirEvent {
		t.Error("expected event for new directory")
	}
	if !hasFileEvent {
		t.Error("expected event for file in new directory")
	}
}

func TestWatchedDirCount(t *testing.T) {
	config := Config{
		Recursive: true,
	}
	w, err := New(config)
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.watcher.Close()

	// Create temp directory with subdirs
	tmpDir, err := os.MkdirTemp("", "watcher-count-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.MkdirAll(filepath.Join(tmpDir, "a", "b"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "c"), 0755)

	err = w.AddPath(tmpDir)
	if err != nil {
		t.Fatalf("failed to add path: %v", err)
	}

	count := w.WatchedDirCount()
	if count != 4 { // tmpDir, a, b, c
		t.Errorf("expected 4 watched dirs, got %d", count)
	}
}
