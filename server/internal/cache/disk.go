package cache

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// DiskCache implements a disk-based cache.
type DiskCache struct {
	config L2Config

	mu       sync.RWMutex
	index    map[string]*diskEntry
	diskSize int64

	// Statistics
	hits      atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	evictions atomic.Int64

	// Cleanup
	stopCh  chan struct{}
	stopped bool
}

// diskEntry holds metadata for a cached item.
type diskEntry struct {
	Key       string
	Path      string
	Size      int64
	ExpiresAt int64 // unix nanos, 0 = no expiration
	CreatedAt int64 // unix nanos
}

// NewDiskCache creates a new disk cache.
func NewDiskCache(config L2Config) (*DiskCache, error) {
	if config.Path == "" {
		config.Path = "./cache"
	}

	// Create cache directory
	if err := os.MkdirAll(config.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	c := &DiskCache{
		config: config,
		index:  make(map[string]*diskEntry),
		stopCh: make(chan struct{}),
	}

	// Load existing cache entries
	if err := c.loadIndex(); err != nil {
		return nil, fmt.Errorf("failed to load cache index: %w", err)
	}

	// Start cleanup goroutine
	if config.CleanupInterval > 0 {
		go c.cleanupLoop()
	}

	return c, nil
}

// Get retrieves a value from the cache.
func (c *DiskCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.RLock()
	entry, ok := c.index[key]
	c.mu.RUnlock()

	if !ok {
		c.misses.Add(1)
		return nil, ErrKeyNotFound
	}

	// Check expiration
	if entry.ExpiresAt != 0 && timeutil.NowNano() > entry.ExpiresAt {
		c.Delete(ctx, key)
		c.misses.Add(1)
		return nil, ErrKeyExpired
	}

	// Read from disk
	value, err := c.readFromDisk(entry.Path)
	if err != nil {
		c.misses.Add(1)
		return nil, err
	}

	c.hits.Add(1)
	return value, nil
}

// Set stores a value in the cache.
func (c *DiskCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sets.Add(1)

	// Calculate expiration
	var expiresAt int64
	if ttl > 0 {
		expiresAt = timeutil.NowNano() + int64(ttl)
	} else if c.config.DefaultTTL > 0 {
		expiresAt = timeutil.NowNano() + int64(c.config.DefaultTTL)
	}

	// Generate file path
	path := c.keyToPath(key)

	// Write to disk
	size, err := c.writeToDisk(path, value)
	if err != nil {
		return err
	}

	// Remove old entry if exists
	if oldEntry, ok := c.index[key]; ok {
		c.diskSize -= oldEntry.Size
		os.Remove(oldEntry.Path)
	}

	// Add new entry
	entry := &diskEntry{
		Key:       key,
		Path:      path,
		Size:      size,
		ExpiresAt: expiresAt,
		CreatedAt: timeutil.NowNano(),
	}
	c.index[key] = entry
	c.diskSize += size

	// Evict if over size limit
	for c.config.MaxDiskSize > 0 && c.diskSize > c.config.MaxDiskSize {
		c.evictOldest()
	}

	return nil
}

// Delete removes a key from the cache.
func (c *DiskCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.index[key]
	if !ok {
		return nil
	}

	os.Remove(entry.Path)
	c.diskSize -= entry.Size
	delete(c.index, key)
	c.deletes.Add(1)

	return nil
}

// Exists checks if a key exists in the cache.
func (c *DiskCache) Exists(ctx context.Context, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.index[key]
	if !ok {
		return false
	}

	if entry.ExpiresAt != 0 && timeutil.NowNano() > entry.ExpiresAt {
		return false
	}

	return true
}

// Clear removes all entries from the cache.
func (c *DiskCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove all files
	for _, entry := range c.index {
		os.Remove(entry.Path)
	}

	c.index = make(map[string]*diskEntry)
	c.diskSize = 0

	return nil
}

// Stats returns cache statistics.
func (c *DiskCache) Stats() CacheStats {
	c.mu.RLock()
	size := int64(len(c.index))
	memUsed := c.diskSize
	c.mu.RUnlock()

	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:       hits,
		Misses:     misses,
		Sets:       c.sets.Load(),
		Deletes:    c.deletes.Load(),
		Evictions:  c.evictions.Load(),
		Size:       size,
		Capacity:   int64(c.config.MaxSize),
		HitRate:    hitRate,
		MemoryUsed: memUsed,
	}
}

// Close closes the cache.
func (c *DiskCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.stopped {
		close(c.stopCh)
		c.stopped = true
	}

	return nil
}

// keyToPath converts a key to a file path.
func (c *DiskCache) keyToPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	hashStr := hex.EncodeToString(hash[:])

	// Use first 2 chars as subdirectory for better file distribution
	subdir := hashStr[:2]
	filename := hashStr[2:] + ".cache"

	return filepath.Join(c.config.Path, subdir, filename)
}

// writeToDisk writes a value to disk.
func (c *DiskCache) writeToDisk(path string, value interface{}) (int64, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory: %w", err)
	}

	// Encode value using JSON
	data, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("failed to encode value: %w", err)
	}

	// Compress if enabled
	if c.config.Compression {
		var compBuf bytes.Buffer
		w, err := gzip.NewWriterLevel(&compBuf, c.config.CompressionLevel)
		if err != nil {
			return 0, fmt.Errorf("failed to create gzip writer: %w", err)
		}
		if _, err := w.Write(data); err != nil {
			w.Close()
			return 0, fmt.Errorf("failed to compress data: %w", err)
		}
		w.Close()
		data = compBuf.Bytes()
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, fmt.Errorf("failed to write file: %w", err)
	}

	return int64(len(data)), nil
}

// readFromDisk reads a value from disk.
func (c *DiskCache) readFromDisk(path string) (interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Decompress if enabled
	if c.config.Compression {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer r.Close()

		data, err = io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress data: %w", err)
		}
	}

	// Decode value using JSON
	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("failed to decode value: %w", err)
	}

	return value, nil
}

// loadIndex loads the cache index from disk.
func (c *DiskCache) loadIndex() error {
	// Use queue-based iteration instead of recursive walk
	queue := []string{c.config.Path}

	for len(queue) > 0 {
		currentDir := queue[0]
		queue = queue[1:]

		entries, err := os.ReadDir(currentDir)
		if err != nil {
			continue // Skip directories we can't read
		}

		for _, entry := range entries {
			path := filepath.Join(currentDir, entry.Name())

			if entry.IsDir() {
				queue = append(queue, path)
				continue
			}

			if filepath.Ext(path) != ".cache" {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue // Skip files we can't stat
			}

			// We don't have the original key, so we use the path as key
			// This is a limitation - in production, you'd want to store metadata
			e := &diskEntry{
				Key:       path,
				Path:      path,
				Size:      info.Size(),
				CreatedAt: info.ModTime().UnixNano(),
			}
			c.index[path] = e
			c.diskSize += info.Size()
		}
	}

	return nil
}

// evictOldest removes the oldest entry.
func (c *DiskCache) evictOldest() {
	var oldest *diskEntry
	for _, entry := range c.index {
		if oldest == nil || entry.CreatedAt < oldest.CreatedAt {
			oldest = entry
		}
	}

	if oldest != nil {
		os.Remove(oldest.Path)
		c.diskSize -= oldest.Size
		delete(c.index, oldest.Key)
		c.evictions.Add(1)

		if c.config.OnEvict != nil {
			c.config.OnEvict(oldest.Key, nil)
		}
	}
}

// cleanupLoop periodically removes expired entries.
func (c *DiskCache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes expired entries.
func (c *DiskCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := timeutil.NowNano()
	var toRemove []string

	for key, entry := range c.index {
		if entry.ExpiresAt != 0 && now > entry.ExpiresAt {
			toRemove = append(toRemove, key)
		}
	}

	for _, key := range toRemove {
		entry := c.index[key]
		os.Remove(entry.Path)
		c.diskSize -= entry.Size
		delete(c.index, key)
		c.evictions.Add(1)
	}
}

// DiskSize returns the current disk usage.
func (c *DiskCache) DiskSize() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.diskSize
}

// Len returns the number of items in the cache.
func (c *DiskCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.index)
}
