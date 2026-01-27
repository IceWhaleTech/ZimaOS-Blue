package logger

import (
	"encoding/json"
	"sync"
	"time"
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
	mu       sync.RWMutex
	entries  []LogEntry
	size     int
	head     int
	count    int
	maxSize  int
}

// NewRingBuffer creates a new ring buffer with the specified capacity
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &RingBuffer{
		entries: make([]LogEntry, capacity),
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

	rb.entries[rb.head] = entry
	rb.head = (rb.head + 1) % rb.maxSize
	if rb.count < rb.maxSize {
		rb.count++
	}
}

// GetAll returns all log entries in chronological order
func (rb *RingBuffer) GetAll() []LogEntry {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	result := make([]LogEntry, rb.count)
	if rb.count == 0 {
		return result
	}

	start := 0
	if rb.count == rb.maxSize {
		start = rb.head
	}

	for i := 0; i < rb.count; i++ {
		idx := (start + i) % rb.maxSize
		result[i] = rb.entries[idx]
	}

	return result
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

// parseLogLine parses a JSON log line from zerolog
func (rb *RingBuffer) parseLogLine(data []byte) *LogEntry {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	entry := &LogEntry{
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
	}

	// Extract standard fields
	if level, ok := raw["level"].(string); ok {
		entry.Level = level
		delete(raw, "level")
	}

	if msg, ok := raw["message"].(string); ok {
		entry.Message = msg
		delete(raw, "message")
	} else if msg, ok := raw["msg"].(string); ok {
		entry.Message = msg
		delete(raw, "msg")
	}

	if timeStr, ok := raw["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
			entry.Timestamp = t
		}
		delete(raw, "time")
	}

	if caller, ok := raw["caller"].(string); ok {
		entry.Caller = caller
		delete(raw, "caller")
	}

	// Store remaining fields
	for k, v := range raw {
		entry.Fields[k] = v
	}

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
