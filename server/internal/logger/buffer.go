package logger

import (
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/tidwall/gjson"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Caller    string                 `json:"caller,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// RingBuffer is a thread-safe circular buffer for log entries
type RingBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	size    int
	head    int
	count   int
	maxSize int
}

const defaultInitialBufferCapacity = 256

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &RingBuffer{
		maxSize: capacity,
	}
}

// Write implements io.Writer for zerolog
func (rb *RingBuffer) Write(p []byte) (n int, err error) {
	entry := rb.parseLogLine(p)
	if entry != nil {
		rb.Add(*entry)
	}
	return len(p), nil
}

// Add adds a log entry to the buffer
func (rb *RingBuffer) Add(entry LogEntry) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.ensureWriteCapacityLocked()
	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % len(rb.entries)
	if rb.count < rb.maxSize {
		rb.count++
	}
}

// GetAll returns all log entries in chronological order
func (rb *RingBuffer) GetAll() []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	return rb.snapshotLocked()
}

// Query returns log entries matching the given criteria
func (rb *RingBuffer) Query(level, search string, limit, offset int, startTime, endTime *time.Time) []LogEntry {
	all := rb.GetAll()

	// Filter entries
	var filtered []LogEntry
	for _, entry := range all {
		// Filter by level
		if level != "" && entry.Level != level {
			continue
		}

		// Filter by time range
		if startTime != nil && entry.Timestamp.Before(*startTime) {
			continue
		}
		if endTime != nil && entry.Timestamp.After(*endTime) {
			continue
		}

		// Filter by search term (case-insensitive search in message)
		if search != "" {
			found := false
			msgLower := toLower(entry.Message)
			searchLower := toLower(search)
			if contains(msgLower, searchLower) {
				found = true
			}
			if !found {
				continue
			}
		}

		filtered = append(filtered, entry)
	}

	// Reverse to get newest first
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	// Apply offset and limit
	if offset >= len(filtered) {
		return []LogEntry{}
	}
	filtered = filtered[offset:]
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}

	return filtered
}

// Count returns the number of entries in the buffer
func (rb *RingBuffer) Count() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

// Clear removes all entries from the buffer
func (rb *RingBuffer) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.head = 0
	rb.count = 0
}

func (rb *RingBuffer) ensureWriteCapacityLocked() {
	if rb.maxSize <= 0 {
		rb.maxSize = 1000
	}
	if len(rb.entries) == 0 {
		initial := rb.maxSize
		if initial > defaultInitialBufferCapacity {
			initial = defaultInitialBufferCapacity
		}
		rb.entries = make([]LogEntry, initial)
		rb.head = 0
		return
	}
	if rb.count < len(rb.entries) || len(rb.entries) >= rb.maxSize {
		return
	}

	newCap := len(rb.entries) * 2
	if newCap < defaultInitialBufferCapacity {
		newCap = defaultInitialBufferCapacity
	}
	if newCap > rb.maxSize {
		newCap = rb.maxSize
	}
	if newCap <= len(rb.entries) {
		return
	}

	ordered := rb.snapshotLocked()
	expanded := make([]LogEntry, newCap)
	copy(expanded, ordered)
	rb.entries = expanded
	rb.head = rb.count
}

func (rb *RingBuffer) snapshotLocked() []LogEntry {
	result := make([]LogEntry, rb.count)
	if rb.count == 0 || len(rb.entries) == 0 {
		return result
	}

	start := 0
	if rb.count == len(rb.entries) {
		start = rb.head
	}

	for i := 0; i < rb.count; i++ {
		idx := (start + i) % len(rb.entries)
		result[i] = rb.entries[idx]
	}

	return result
}

// known keys excluded from Fields
var knownKeys = map[string]bool{
	"level": true, "message": true, "msg": true,
	"time": true, "caller": true,
}

// parseLogLine parses a JSON log line from zerolog using gjson (zero-alloc field extraction).
func (rb *RingBuffer) parseLogLine(data []byte) *LogEntry {
	s := string(data)
	if !gjson.Valid(s) {
		return nil
	}

	entry := &LogEntry{Timestamp: timeutil.NowTime()}

	entry.Level = gjson.Get(s, "level").Str
	if msg := gjson.Get(s, "message"); msg.Exists() {
		entry.Message = msg.Str
	} else {
		entry.Message = gjson.Get(s, "msg").Str
	}
	if ts := gjson.Get(s, "time"); ts.Exists() {
		if t, err := time.Parse(time.RFC3339, ts.Str); err == nil {
			entry.Timestamp = t
		}
	}
	entry.Caller = gjson.Get(s, "caller").Str

	// Collect extra fields only if present
	gjson.Parse(s).ForEach(func(key, value gjson.Result) bool {
		if !knownKeys[key.Str] {
			if entry.Fields == nil {
				entry.Fields = make(map[string]interface{}, 4)
			}
			entry.Fields[key.Str] = value.Value()
		}
		return true
	})

	return entry
}

// Helper functions to avoid importing strings package
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Global ring buffer instance
var logBuffer *RingBuffer

// GetBuffer returns the global log buffer
func GetBuffer() *RingBuffer {
	return logBuffer
}

// InitBuffer initializes the global log buffer
func InitBuffer(capacity int) {
	logBuffer = NewRingBuffer(capacity)
}
