package proxy

import (
	"bytes"
	"fmt"
	"sync"
)

// StreamingAnomalyDetector detects repetitive/looping output in streaming responses
type StreamingAnomalyDetector struct {
	windowSize      int // Sliding window size for pattern detection
	repeatThreshold int // Number of repeats to trigger anomaly
	minPatternLen   int // Minimum pattern length to consider
	maxPatternLen   int // Maximum pattern length to check
}

// StreamingAnomalyConfig configuration for anomaly detection
type StreamingAnomalyConfig struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	WindowSize       int    `json:"window_size" yaml:"window_size"`
	RepeatThreshold  int    `json:"repeat_threshold" yaml:"repeat_threshold"`
	MinPatternLength int    `json:"min_pattern_length" yaml:"min_pattern_length"`
	MaxPatternLength int    `json:"max_pattern_length" yaml:"max_pattern_length"`
	RecoveryStrategy string `json:"recovery_strategy" yaml:"recovery_strategy"` // truncate_and_retry, failover, stop
}

// DefaultStreamingAnomalyConfig returns default configuration
func DefaultStreamingAnomalyConfig() StreamingAnomalyConfig {
	return StreamingAnomalyConfig{
		Enabled:          true,
		WindowSize:       4096,
		RepeatThreshold:  3,
		MinPatternLength: 20,
		MaxPatternLength: 500,
		RecoveryStrategy: "truncate_and_retry",
	}
}

// NewStreamingAnomalyDetector creates a new detector
func NewStreamingAnomalyDetector(config StreamingAnomalyConfig) *StreamingAnomalyDetector {
	if config.WindowSize <= 0 {
		config.WindowSize = 4096
	}
	if config.RepeatThreshold <= 0 {
		config.RepeatThreshold = 3
	}
	if config.MinPatternLength <= 0 {
		config.MinPatternLength = 20
	}
	if config.MaxPatternLength <= 0 {
		config.MaxPatternLength = 500
	}

	return &StreamingAnomalyDetector{
		windowSize:      config.WindowSize,
		repeatThreshold: config.RepeatThreshold,
		minPatternLen:   config.MinPatternLength,
		maxPatternLen:   config.MaxPatternLength,
	}
}

// StreamAnomaly represents a detected streaming anomaly
type StreamAnomaly struct {
	Type        RetryableErrorType `json:"type"`
	Pattern     string             `json:"pattern"`
	RepeatCount int                `json:"repeat_count"`
	Message     string             `json:"message"`
	Position    int                `json:"position"` // Position in buffer where anomaly was detected
}

// StreamRecoveryStrategy defines how to recover from streaming anomalies
type StreamRecoveryStrategy struct {
	ForceStop           bool     `json:"force_stop"`
	RetryWithTruncation bool     `json:"retry_with_truncation"`
	TruncateToTokens    int      `json:"truncate_to_tokens"`
	FailoverToProvider  string   `json:"failover_to_provider"`
	AddStopSequences    []string `json:"add_stop_sequences"`
}

// StreamBuffer maintains a sliding window of streamed content
type StreamBuffer struct {
	mu       sync.Mutex
	buffer   []byte
	maxSize  int
	detector *StreamingAnomalyDetector

	// Tracking
	totalBytes      int64
	anomalyDetected bool
	lastAnomaly     *StreamAnomaly

	// Chunk tracking for pattern detection
	recentChunks [][]byte
	maxChunks    int
}

// NewStreamBuffer creates a new stream buffer
func NewStreamBuffer(detector *StreamingAnomalyDetector) *StreamBuffer {
	return &StreamBuffer{
		buffer:       make([]byte, 0, detector.windowSize),
		maxSize:      detector.windowSize,
		detector:     detector,
		recentChunks: make([][]byte, 0, 20),
		maxChunks:    20,
	}
}

// Write adds content to buffer and checks for anomalies
func (sb *StreamBuffer) Write(chunk []byte) (*StreamAnomaly, error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if len(chunk) == 0 {
		return nil, nil
	}

	sb.totalBytes += int64(len(chunk))

	// Add to sliding window
	sb.buffer = append(sb.buffer, chunk...)
	if len(sb.buffer) > sb.maxSize {
		// Keep only the last maxSize bytes
		sb.buffer = sb.buffer[len(sb.buffer)-sb.maxSize:]
	}

	// Track recent chunks
	chunkCopy := make([]byte, len(chunk))
	copy(chunkCopy, chunk)
	sb.recentChunks = append(sb.recentChunks, chunkCopy)
	if len(sb.recentChunks) > sb.maxChunks {
		sb.recentChunks = sb.recentChunks[1:]
	}

	// Check for repetitive patterns
	if anomaly := sb.detectRepetition(); anomaly != nil {
		sb.anomalyDetected = true
		sb.lastAnomaly = anomaly
		return anomaly, nil
	}

	// Check for chunk-level repetition
	if anomaly := sb.detectChunkRepetition(); anomaly != nil {
		sb.anomalyDetected = true
		sb.lastAnomaly = anomaly
		return anomaly, nil
	}

	return nil, nil
}

// detectRepetition uses sliding window to detect repetitive output
func (sb *StreamBuffer) detectRepetition() *StreamAnomaly {
	bufLen := len(sb.buffer)
	if bufLen < sb.detector.minPatternLen*sb.detector.repeatThreshold {
		return nil
	}

	// Try different pattern lengths, starting from smaller patterns
	maxLen := sb.detector.maxPatternLen
	if maxLen > bufLen/sb.detector.repeatThreshold {
		maxLen = bufLen / sb.detector.repeatThreshold
	}

	for patternLen := sb.detector.minPatternLen; patternLen <= maxLen; patternLen++ {
		// Get the pattern from the end of buffer
		pattern := sb.buffer[bufLen-patternLen:]

		// Count consecutive occurrences backwards
		count := 1
		pos := bufLen - patternLen*2

		for pos >= 0 {
			segment := sb.buffer[pos : pos+patternLen]
			if bytes.Equal(segment, pattern) {
				count++
				pos -= patternLen
			} else {
				break
			}
		}

		if count >= sb.detector.repeatThreshold {
			// Found repetitive pattern
			patternStr := string(pattern)
			if len(patternStr) > 100 {
				patternStr = patternStr[:100] + "..."
			}

			return &StreamAnomaly{
				Type:        ErrorTypeRepetitiveOutput,
				Pattern:     patternStr,
				RepeatCount: count,
				Message:     fmt.Sprintf("Detected %d consecutive repetitions of %d-byte pattern", count, patternLen),
				Position:    bufLen,
			}
		}
	}

	return nil
}

// detectChunkRepetition detects if recent chunks are identical
func (sb *StreamBuffer) detectChunkRepetition() *StreamAnomaly {
	if len(sb.recentChunks) < sb.detector.repeatThreshold {
		return nil
	}

	// Check if the last N chunks are identical
	lastChunk := sb.recentChunks[len(sb.recentChunks)-1]
	if len(lastChunk) < sb.detector.minPatternLen {
		return nil
	}

	count := 1
	for i := len(sb.recentChunks) - 2; i >= 0; i-- {
		if bytes.Equal(sb.recentChunks[i], lastChunk) {
			count++
		} else {
			break
		}
	}

	if count >= sb.detector.repeatThreshold {
		patternStr := string(lastChunk)
		if len(patternStr) > 100 {
			patternStr = patternStr[:100] + "..."
		}

		return &StreamAnomaly{
			Type:        ErrorTypeRepetitiveOutput,
			Pattern:     patternStr,
			RepeatCount: count,
			Message:     fmt.Sprintf("Detected %d identical consecutive chunks", count),
			Position:    len(sb.buffer),
		}
	}

	return nil
}

// HasAnomaly returns whether an anomaly was detected
func (sb *StreamBuffer) HasAnomaly() bool {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.anomalyDetected
}

// GetLastAnomaly returns the last detected anomaly
func (sb *StreamBuffer) GetLastAnomaly() *StreamAnomaly {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.lastAnomaly
}

// GetTotalBytes returns total bytes processed
func (sb *StreamBuffer) GetTotalBytes() int64 {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.totalBytes
}

// GetValidContent returns content before the anomaly (if any)
func (sb *StreamBuffer) GetValidContent() []byte {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if !sb.anomalyDetected || sb.lastAnomaly == nil {
		return sb.buffer
	}

	// Try to find where the repetition started
	pattern := []byte(sb.lastAnomaly.Pattern)
	if len(pattern) == 0 {
		return sb.buffer
	}

	// Find first occurrence of the pattern
	idx := bytes.Index(sb.buffer, pattern)
	if idx > 0 {
		return sb.buffer[:idx]
	}

	return sb.buffer
}

// Reset clears the buffer
func (sb *StreamBuffer) Reset() {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	sb.buffer = sb.buffer[:0]
	sb.recentChunks = sb.recentChunks[:0]
	sb.totalBytes = 0
	sb.anomalyDetected = false
	sb.lastAnomaly = nil
}

// GetRecoveryStrategy returns appropriate recovery strategy for anomaly
func GetRecoveryStrategy(anomaly *StreamAnomaly, config StreamingAnomalyConfig) *StreamRecoveryStrategy {
	if anomaly == nil {
		return nil
	}

	strategy := &StreamRecoveryStrategy{
		ForceStop: true,
	}

	switch config.RecoveryStrategy {
	case "truncate_and_retry":
		strategy.RetryWithTruncation = true
		// Add the repetitive pattern as a stop sequence to prevent loop
		if len(anomaly.Pattern) > 0 {
			stopSeq := anomaly.Pattern
			if len(stopSeq) > 50 {
				stopSeq = stopSeq[:50]
			}
			strategy.AddStopSequences = []string{stopSeq}
		}

	case "failover":
		strategy.FailoverToProvider = "" // Will be selected by failover handler

	case "stop":
		// Just stop, don't retry
		strategy.RetryWithTruncation = false
	}

	return strategy
}
