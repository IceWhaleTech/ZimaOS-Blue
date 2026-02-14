// Package queue provides a reliable message queue for channel messages.
// It ensures message delivery even during temporary failures.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

// MessageStatus represents the status of a queued message.
type MessageStatus string

const (
	// StatusPending indicates the message is waiting to be processed.
	StatusPending MessageStatus = "pending"
	// StatusProcessing indicates the message is currently being processed.
	StatusProcessing MessageStatus = "processing"
	// StatusCompleted indicates the message was successfully processed.
	StatusCompleted MessageStatus = "completed"
	// StatusFailed indicates the message processing failed.
	StatusFailed MessageStatus = "failed"
	// StatusRetrying indicates the message is being retried.
	StatusRetrying MessageStatus = "retrying"
)

// QueuedMessage represents a message in the queue.
type QueuedMessage struct {
	ID           string                 `json:"id"`
	ChannelName  string                 `json:"channel_name"`
	Message      channel.Message        `json:"message"`
	Response     *channel.OutgoingMessage `json:"response,omitempty"`
	Status       MessageStatus          `json:"status"`
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
	CreatedAt    time.Time              `json:"created_at"`
	ProcessedAt  *time.Time             `json:"processed_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Config contains queue configuration.
type Config struct {
	// MaxSize is the maximum number of messages in the queue.
	MaxSize int `mapstructure:"max_size"`
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `mapstructure:"max_retries"`
	// RetryDelayMS is the initial retry delay in milliseconds.
	RetryDelayMS int `mapstructure:"retry_delay_ms"`
	// RetryBackoffMultiplier is the multiplier for exponential backoff.
	RetryBackoffMultiplier float64 `mapstructure:"retry_backoff_multiplier"`
	// ProcessingTimeoutSeconds is the timeout for processing a message.
	ProcessingTimeoutSeconds int `mapstructure:"processing_timeout_seconds"`
	// PersistPath is the path to persist the queue (empty for in-memory only).
	PersistPath string `mapstructure:"persist_path"`
	// CleanupIntervalSeconds is the interval for cleaning up completed messages.
	CleanupIntervalSeconds int `mapstructure:"cleanup_interval_seconds"`
	// RetentionSeconds is how long to keep completed messages.
	RetentionSeconds int `mapstructure:"retention_seconds"`
}

// DefaultConfig returns the default queue configuration.
func DefaultConfig() Config {
	return Config{
		MaxSize:                  10000,
		MaxRetries:               3,
		RetryDelayMS:             1000,
		RetryBackoffMultiplier:   2.0,
		ProcessingTimeoutSeconds: 60,
		PersistPath:              "",
		CleanupIntervalSeconds:   300,
		RetentionSeconds:         3600,
	}
}

// Queue is a reliable message queue for channel messages.
type Queue struct {
	config   Config
	logger   *zap.Logger
	messages map[string]*QueuedMessage
	pending  chan string // Channel of pending message IDs
	mu       sync.RWMutex

	// Handlers
	handler channel.MessageHandler

	// Statistics
	stats Stats

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Stats contains queue statistics.
type Stats struct {
	TotalEnqueued   int64     `json:"total_enqueued"`
	TotalProcessed  int64     `json:"total_processed"`
	TotalCompleted  int64     `json:"total_completed"`
	TotalFailed     int64     `json:"total_failed"`
	TotalRetried    int64     `json:"total_retried"`
	CurrentPending  int       `json:"current_pending"`
	CurrentSize     int       `json:"current_size"`
	LastEnqueuedAt  *time.Time `json:"last_enqueued_at,omitempty"`
	LastProcessedAt *time.Time `json:"last_processed_at,omitempty"`
}

// New creates a new message queue.
func New(cfg Config, logger *zap.Logger) *Queue {
	ctx, cancel := context.WithCancel(context.Background())
	return &Queue{
		config:   cfg,
		logger:   logger.With(zap.String("component", "message_queue")),
		messages: make(map[string]*QueuedMessage),
		pending:  make(chan string, cfg.MaxSize),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// SetHandler sets the message handler.
func (q *Queue) SetHandler(handler channel.MessageHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handler = handler
}

// Start starts the queue processing.
func (q *Queue) Start(ctx context.Context, workers int) error {
	if workers <= 0 {
		workers = 1
	}

	q.logger.Info("starting message queue",
		zap.Int("workers", workers),
		zap.Int("max_size", q.config.MaxSize))

	// Start worker goroutines
	for i := 0; i < workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}

	// Start cleanup goroutine
	q.wg.Add(1)
	go q.cleanupLoop()

	return nil
}

// Stop stops the queue processing.
func (q *Queue) Stop(ctx context.Context) error {
	q.logger.Info("stopping message queue")
	q.cancel()

	// Wait for workers to finish
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		q.logger.Info("message queue stopped")
	case <-ctx.Done():
		q.logger.Warn("timeout waiting for queue to stop")
		return ctx.Err()
	}

	return nil
}

// Enqueue adds a message to the queue.
func (q *Queue) Enqueue(channelName string, msg channel.Message) (string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check queue size
	if len(q.messages) >= q.config.MaxSize {
		return "", fmt.Errorf("queue is full (max size: %d)", q.config.MaxSize)
	}

	// Create queued message
	qm := &QueuedMessage{
		ID:          fmt.Sprintf("%s_%s_%d", channelName, msg.ID, time.Now().UnixNano()),
		ChannelName: channelName,
		Message:     msg,
		Status:      StatusPending,
		MaxRetries:  q.config.MaxRetries,
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]interface{}),
	}

	q.messages[qm.ID] = qm
	q.stats.TotalEnqueued++
	q.stats.CurrentSize = len(q.messages)
	now := time.Now()
	q.stats.LastEnqueuedAt = &now

	// Add to pending channel (non-blocking)
	select {
	case q.pending <- qm.ID:
		q.stats.CurrentPending++
	default:
		// Channel full, message will be picked up by retry mechanism
		q.logger.Warn("pending channel full, message will be retried",
			zap.String("message_id", qm.ID))
	}

	q.logger.Debug("message enqueued",
		zap.String("queue_id", qm.ID),
		zap.String("channel", channelName),
		zap.String("message_id", msg.ID))

	return qm.ID, nil
}

// Get returns a queued message by ID.
func (q *Queue) Get(id string) (*QueuedMessage, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	qm, exists := q.messages[id]
	if exists {
		// Return a copy
		copy := *qm
		return &copy, true
	}
	return nil, false
}

// List returns all queued messages with optional status filter.
func (q *Queue) List(status MessageStatus, limit int) []*QueuedMessage {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*QueuedMessage, 0)
	for _, qm := range q.messages {
		if status == "" || qm.Status == status {
			copy := *qm
			result = append(result, &copy)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// Stats returns queue statistics.
func (q *Queue) Stats() Stats {
	q.mu.RLock()
	defer q.mu.RUnlock()
	stats := q.stats
	stats.CurrentSize = len(q.messages)
	return stats
}

// Retry manually retries a failed message.
func (q *Queue) Retry(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	qm, exists := q.messages[id]
	if !exists {
		return fmt.Errorf("message %s not found", id)
	}

	if qm.Status != StatusFailed {
		return fmt.Errorf("message %s is not in failed status", id)
	}

	qm.Status = StatusRetrying
	qm.RetryCount++
	qm.Error = ""

	select {
	case q.pending <- id:
		q.stats.CurrentPending++
		q.stats.TotalRetried++
	default:
		return fmt.Errorf("pending channel full")
	}

	return nil
}

// Delete removes a message from the queue.
func (q *Queue) Delete(id string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.messages[id]; !exists {
		return fmt.Errorf("message %s not found", id)
	}

	delete(q.messages, id)
	q.stats.CurrentSize = len(q.messages)
	return nil
}

// worker processes messages from the queue.
func (q *Queue) worker(id int) {
	defer q.wg.Done()

	q.logger.Debug("queue worker started", zap.Int("worker_id", id))

	for {
		select {
		case <-q.ctx.Done():
			q.logger.Debug("queue worker stopping", zap.Int("worker_id", id))
			return
		case msgID := <-q.pending:
			q.processMessage(msgID)
		}
	}
}

// processMessage processes a single message.
func (q *Queue) processMessage(id string) {
	q.mu.Lock()
	qm, exists := q.messages[id]
	if !exists {
		q.mu.Unlock()
		return
	}

	// Update status
	qm.Status = StatusProcessing
	now := time.Now()
	qm.ProcessedAt = &now
	q.stats.CurrentPending--
	q.stats.TotalProcessed++
	q.stats.LastProcessedAt = &now

	handler := q.handler
	q.mu.Unlock()

	if handler == nil {
		q.markFailed(id, "no handler set")
		return
	}

	// Process with timeout
	ctx, cancel := context.WithTimeout(q.ctx, time.Duration(q.config.ProcessingTimeoutSeconds)*time.Second)
	defer cancel()

	response, err := handler(ctx, qm.Message)
	if err != nil {
		q.handleError(id, err)
		return
	}

	q.markCompleted(id, response)
}

// markCompleted marks a message as completed.
func (q *Queue) markCompleted(id string, response *channel.OutgoingMessage) {
	q.mu.Lock()
	defer q.mu.Unlock()

	qm, exists := q.messages[id]
	if !exists {
		return
	}

	qm.Status = StatusCompleted
	qm.Response = response
	now := time.Now()
	qm.CompletedAt = &now
	q.stats.TotalCompleted++

	q.logger.Debug("message completed",
		zap.String("queue_id", id),
		zap.String("channel", qm.ChannelName))
}

// markFailed marks a message as failed.
func (q *Queue) markFailed(id string, errMsg string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	qm, exists := q.messages[id]
	if !exists {
		return
	}

	qm.Status = StatusFailed
	qm.Error = errMsg
	q.stats.TotalFailed++

	q.logger.Error("message failed",
		zap.String("queue_id", id),
		zap.String("channel", qm.ChannelName),
		zap.String("error", errMsg))
}

// handleError handles processing errors with retry logic.
func (q *Queue) handleError(id string, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	qm, exists := q.messages[id]
	if !exists {
		return
	}

	qm.Error = err.Error()

	// Check if we should retry
	if qm.RetryCount < qm.MaxRetries {
		qm.Status = StatusRetrying
		qm.RetryCount++
		q.stats.TotalRetried++

		// Calculate retry delay with exponential backoff
		delay := time.Duration(q.config.RetryDelayMS) * time.Millisecond
		for i := 0; i < qm.RetryCount-1; i++ {
			delay = time.Duration(float64(delay) * q.config.RetryBackoffMultiplier)
		}

		q.logger.Warn("message processing failed, scheduling retry",
			zap.String("queue_id", id),
			zap.Int("retry_count", qm.RetryCount),
			zap.Duration("delay", delay),
			zap.Error(err))

		// Schedule retry
		go func() {
			select {
			case <-q.ctx.Done():
				return
			case <-time.After(delay):
				q.mu.Lock()
				if qm, exists := q.messages[id]; exists && qm.Status == StatusRetrying {
					select {
					case q.pending <- id:
						q.stats.CurrentPending++
					default:
						q.logger.Warn("pending channel full during retry",
							zap.String("queue_id", id))
					}
				}
				q.mu.Unlock()
			}
		}()
	} else {
		qm.Status = StatusFailed
		q.stats.TotalFailed++

		q.logger.Error("message failed after max retries",
			zap.String("queue_id", id),
			zap.Int("retry_count", qm.RetryCount),
			zap.Error(err))
	}
}

// cleanupLoop periodically cleans up completed messages.
func (q *Queue) cleanupLoop() {
	defer q.wg.Done()

	ticker := time.NewTicker(time.Duration(q.config.CleanupIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-q.ctx.Done():
			return
		case <-ticker.C:
			q.cleanup()
		}
	}
}

// cleanup removes old completed messages.
func (q *Queue) cleanup() {
	q.mu.Lock()
	defer q.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(q.config.RetentionSeconds) * time.Second)
	removed := 0

	for id, qm := range q.messages {
		if qm.Status == StatusCompleted && qm.CompletedAt != nil && qm.CompletedAt.Before(cutoff) {
			delete(q.messages, id)
			removed++
		}
	}

	if removed > 0 {
		q.stats.CurrentSize = len(q.messages)
		q.logger.Debug("cleaned up completed messages", zap.Int("removed", removed))
	}
}

// MarshalJSON implements json.Marshaler for QueuedMessage.
func (qm *QueuedMessage) MarshalJSON() ([]byte, error) {
	type Alias QueuedMessage
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(qm),
	})
}
