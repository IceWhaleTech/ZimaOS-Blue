// Package watcher provides file system watching capabilities.
package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// EventType represents the type of file system event.
type EventType string

const (
	EventCreate EventType = "create"
	EventWrite  EventType = "write"
	EventRemove EventType = "remove"
	EventRename EventType = "rename"
	EventChmod  EventType = "chmod"
)

// Event represents a file system event.
type Event struct {
	Type      EventType `json:"type"`
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	IsDir     bool      `json:"is_dir"`
	Timestamp time.Time `json:"timestamp"`
}

// Config holds watcher configuration.
type Config struct {
	// Paths to watch.
	Paths []string `yaml:"paths" yaml:"paths"`

	// Events to listen for.
	Events []EventType `yaml:"events" yaml:"events"`

	// Recursive enables recursive watching.
	Recursive bool `yaml:"recursive" yaml:"recursive"`

	// DebounceMs is the debounce duration in milliseconds.
	DebounceMs int `yaml:"debounce_ms" yaml:"debounce_ms"`

	// Enabled enables the watcher.
	Enabled bool `yaml:"enabled" yaml:"enabled"`

	// IgnorePatterns are glob patterns to ignore.
	IgnorePatterns []string `yaml:"ignore_patterns" yaml:"ignore_patterns"`
}

// DefaultConfig returns the default watcher configuration.
func DefaultConfig() Config {
	return Config{
		Paths:          []string{},
		Events:         []EventType{EventCreate, EventWrite, EventRemove},
		Recursive:      false,
		DebounceMs:     100,
		Enabled:        false,
		IgnorePatterns: []string{".git", ".svn", "node_modules", ".DS_Store"},
	}
}

// Handler is a function that handles file system events.
type Handler func(event Event)

// Watcher watches file system changes.
type Watcher struct {
	config   Config
	watcher  *fsnotify.Watcher
	handlers []Handler
	mu       sync.RWMutex
	cancel   context.CancelFunc
	done     chan struct{}

	// Track watched directories for recursive mode
	watchedDirs   map[string]struct{}
	watchedDirsMu sync.RWMutex

	// Debouncing
	debounce   map[string]*time.Timer
	debounceMu sync.Mutex
}

// New creates a new file system watcher.
func New(config Config) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		config:      config,
		watcher:     fsWatcher,
		handlers:    make([]Handler, 0),
		done:        make(chan struct{}),
		debounce:    make(map[string]*time.Timer),
		watchedDirs: make(map[string]struct{}),
	}, nil
}

// AddHandler adds an event handler.
func (w *Watcher) AddHandler(handler Handler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers = append(w.handlers, handler)
}

// AddPath adds a path to watch.
func (w *Watcher) AddPath(path string) error {
	if w.config.Recursive {
		return w.addPathRecursive(path)
	}
	return w.addSinglePath(path)
}

// addSinglePath adds a single path to watch.
func (w *Watcher) addSinglePath(path string) error {
	w.watchedDirsMu.Lock()
	defer w.watchedDirsMu.Unlock()

	if _, exists := w.watchedDirs[path]; exists {
		return nil
	}

	if err := w.watcher.Add(path); err != nil {
		return err
	}

	w.watchedDirs[path] = struct{}{}
	return nil
}

// addPathRecursive adds a path and all subdirectories using os.ReadDir + queue.
func (w *Watcher) addPathRecursive(root string) error {
	// Use a queue for BFS traversal (avoids deep recursion)
	queue := []string{root}

	for len(queue) > 0 {
		// Dequeue
		current := queue[0]
		queue = queue[1:]

		// Check if should ignore
		if w.shouldIgnore(current) {
			continue
		}

		// Add to watcher
		if err := w.addSinglePath(current); err != nil {
			// Skip directories we can't watch, but continue
			continue
		}

		// Read directory entries
		entries, err := os.ReadDir(current)
		if err != nil {
			// Skip directories we can't read
			continue
		}

		// Enqueue subdirectories
		for _, entry := range entries {
			if entry.IsDir() {
				subPath := filepath.Join(current, entry.Name())
				if !w.shouldIgnore(subPath) {
					queue = append(queue, subPath)
				}
			}
		}
	}

	return nil
}

// shouldIgnore checks if a path should be ignored.
func (w *Watcher) shouldIgnore(path string) bool {
	name := filepath.Base(path)
	for _, pattern := range w.config.IgnorePatterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

// RemovePath removes a path from watching.
func (w *Watcher) RemovePath(path string) error {
	w.watchedDirsMu.Lock()
	defer w.watchedDirsMu.Unlock()

	if err := w.watcher.Remove(path); err != nil {
		return err
	}

	delete(w.watchedDirs, path)
	return nil
}

// Start starts the watcher.
func (w *Watcher) Start(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)

	// Add configured paths
	for _, path := range w.config.Paths {
		if err := w.AddPath(path); err != nil {
			return err
		}
	}

	go w.run(ctx)
	return nil
}

// Stop stops the watcher.
func (w *Watcher) Stop() error {
	if w.cancel != nil {
		w.cancel()
	}

	// Wait for run loop to finish
	<-w.done

	// Clean up debounce timers
	w.debounceMu.Lock()
	for _, timer := range w.debounce {
		timer.Stop()
	}
	w.debounce = make(map[string]*time.Timer)
	w.debounceMu.Unlock()

	return w.watcher.Close()
}

// run is the main event loop.
func (w *Watcher) run(ctx context.Context) {
	defer close(w.done)

	for {
		select {
		case <-ctx.Done():
			return

		case fsEvent, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleFsEvent(fsEvent)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue
			_ = err
		}
	}
}

// handleFsEvent processes a fsnotify event.
func (w *Watcher) handleFsEvent(fsEvent fsnotify.Event) {
	eventType := w.mapEventType(fsEvent.Op)
	if eventType == "" {
		return
	}

	// Check if this event type is enabled
	if !w.isEventEnabled(eventType) {
		return
	}

	// Check if should ignore
	if w.shouldIgnore(fsEvent.Name) {
		return
	}

	// Check if it's a directory
	isDir := false
	if info, err := os.Stat(fsEvent.Name); err == nil {
		isDir = info.IsDir()
	}

	event := Event{
		Type:      eventType,
		Path:      fsEvent.Name,
		Name:      filepath.Base(fsEvent.Name),
		IsDir:     isDir,
		Timestamp: timeutil.NowTime(),
	}

	// Handle recursive watching for new directories
	if w.config.Recursive && eventType == EventCreate && isDir {
		// Add new directory to watch
		go w.addPathRecursive(fsEvent.Name)
	}

	// Handle directory removal
	if eventType == EventRemove {
		w.watchedDirsMu.Lock()
		delete(w.watchedDirs, fsEvent.Name)
		w.watchedDirsMu.Unlock()
	}

	// Apply debouncing
	if w.config.DebounceMs > 0 {
		w.debounceEvent(event)
	} else {
		w.dispatchEvent(event)
	}
}

// mapEventType maps fsnotify.Op to EventType.
func (w *Watcher) mapEventType(op fsnotify.Op) EventType {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return EventCreate
	case op&fsnotify.Write == fsnotify.Write:
		return EventWrite
	case op&fsnotify.Remove == fsnotify.Remove:
		return EventRemove
	case op&fsnotify.Rename == fsnotify.Rename:
		return EventRename
	case op&fsnotify.Chmod == fsnotify.Chmod:
		return EventChmod
	default:
		return ""
	}
}

// isEventEnabled checks if an event type is enabled.
func (w *Watcher) isEventEnabled(eventType EventType) bool {
	if len(w.config.Events) == 0 {
		return true // All events enabled by default
	}

	for _, e := range w.config.Events {
		if e == eventType {
			return true
		}
	}
	return false
}

// debounceEvent debounces an event.
func (w *Watcher) debounceEvent(event Event) {
	w.debounceMu.Lock()
	defer w.debounceMu.Unlock()

	key := event.Path + string(event.Type)

	// Cancel existing timer
	if timer, exists := w.debounce[key]; exists {
		timer.Stop()
	}

	// Create new timer
	duration := time.Duration(w.config.DebounceMs) * time.Millisecond
	w.debounce[key] = time.AfterFunc(duration, func() {
		w.dispatchEvent(event)

		w.debounceMu.Lock()
		delete(w.debounce, key)
		w.debounceMu.Unlock()
	})
}

// dispatchEvent dispatches an event to all handlers.
func (w *Watcher) dispatchEvent(event Event) {
	w.mu.RLock()
	handlers := make([]Handler, len(w.handlers))
	copy(handlers, w.handlers)
	w.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

// WatchedPaths returns the list of watched paths.
func (w *Watcher) WatchedPaths() []string {
	return w.watcher.WatchList()
}

// WatchedDirCount returns the number of watched directories.
func (w *Watcher) WatchedDirCount() int {
	w.watchedDirsMu.RLock()
	defer w.watchedDirsMu.RUnlock()
	return len(w.watchedDirs)
}

// Config returns the current configuration.
func (w *Watcher) Config() Config {
	return w.config
}
