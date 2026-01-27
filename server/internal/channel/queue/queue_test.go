package queue

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/channel"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	q := New(cfg, logger)
	if q == nil {
		t.Fatal("expected non-nil queue")
	}

	if q.config.MaxSize != cfg.MaxSize {
		t.Errorf("expected max size %d, got %d", cfg.MaxSize, q.config.MaxSize)
	}
}

func TestEnqueue(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxSize = 10

	q := New(cfg, logger)

	msg := channel.Message{
		ID:          "test-msg-1",
		ChannelName: "telegram",
		ChatID:      "chat-123",
		UserID:      "user-456",
		Content:     "Hello, World!",
		Timestamp:   time.Now(),
	}

	id, err := q.Enqueue("telegram", msg)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	if id == "" {
		t.Error("expected non-empty queue ID")
	}

	// Verify message is in queue
	qm, exists := q.Get(id)
	if !exists {
		t.Fatal("message not found in queue")
	}

	if qm.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, qm.Status)
	}

	if qm.Message.Content != msg.Content {
		t.Errorf("expected content %s, got %s", msg.Content, qm.Message.Content)
	}
}

func TestEnqueueFull(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxSize = 2

	q := New(cfg, logger)

	// Fill the queue
	for i := 0; i < 2; i++ {
		msg := channel.Message{
			ID:      "msg-" + string(rune('0'+i)),
			Content: "test",
		}
		_, err := q.Enqueue("test", msg)
		if err != nil {
			t.Fatalf("failed to enqueue message %d: %v", i, err)
		}
	}

	// Try to add one more
	msg := channel.Message{
		ID:      "msg-overflow",
		Content: "overflow",
	}
	_, err := q.Enqueue("test", msg)
	if err == nil {
		t.Error("expected error when queue is full")
	}
}

func TestProcessMessage(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.ProcessingTimeoutSeconds = 5

	q := New(cfg, logger)

	var processed atomic.Bool
	q.SetHandler(func(ctx context.Context, msg channel.Message) (*channel.OutgoingMessage, error) {
		processed.Store(true)
		return &channel.OutgoingMessage{
			ChatID:  msg.ChatID,
			Content: "Response: " + msg.Content,
		}, nil
	})

	// Start queue with 1 worker
	ctx := context.Background()
	if err := q.Start(ctx, 1); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}

	// Enqueue a message
	msg := channel.Message{
		ID:      "test-msg",
		ChatID:  "chat-123",
		Content: "Hello",
	}
	id, err := q.Enqueue("test", msg)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	if !processed.Load() {
		t.Error("message was not processed")
	}

	// Check status
	qm, exists := q.Get(id)
	if !exists {
		t.Fatal("message not found")
	}

	if qm.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, qm.Status)
	}

	if qm.Response == nil {
		t.Error("expected response to be set")
	}

	// Stop queue
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := q.Stop(stopCtx); err != nil {
		t.Errorf("failed to stop queue: %v", err)
	}
}

func TestRetry(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	cfg.RetryDelayMS = 50

	q := New(cfg, logger)

	var attempts atomic.Int32
	q.SetHandler(func(ctx context.Context, msg channel.Message) (*channel.OutgoingMessage, error) {
		count := attempts.Add(1)
		if count < 3 {
			return nil, errors.New("temporary error")
		}
		return &channel.OutgoingMessage{Content: "success"}, nil
	})

	ctx := context.Background()
	if err := q.Start(ctx, 1); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}

	msg := channel.Message{
		ID:      "retry-msg",
		Content: "test",
	}
	id, err := q.Enqueue("test", msg)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Wait for retries
	time.Sleep(500 * time.Millisecond)

	if attempts.Load() < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempts.Load())
	}

	qm, exists := q.Get(id)
	if !exists {
		t.Fatal("message not found")
	}

	if qm.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, qm.Status)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	q.Stop(stopCtx)
}

func TestMaxRetries(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxRetries = 2
	cfg.RetryDelayMS = 10

	q := New(cfg, logger)

	q.SetHandler(func(ctx context.Context, msg channel.Message) (*channel.OutgoingMessage, error) {
		return nil, errors.New("permanent error")
	})

	ctx := context.Background()
	if err := q.Start(ctx, 1); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}

	msg := channel.Message{
		ID:      "fail-msg",
		Content: "test",
	}
	id, err := q.Enqueue("test", msg)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Wait for all retries
	time.Sleep(200 * time.Millisecond)

	qm, exists := q.Get(id)
	if !exists {
		t.Fatal("message not found")
	}

	if qm.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, qm.Status)
	}

	if qm.RetryCount != cfg.MaxRetries {
		t.Errorf("expected retry count %d, got %d", cfg.MaxRetries, qm.RetryCount)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	q.Stop(stopCtx)
}

func TestManualRetry(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxRetries = 1
	cfg.RetryDelayMS = 10

	q := New(cfg, logger)

	var shouldFail atomic.Bool
	shouldFail.Store(true)

	q.SetHandler(func(ctx context.Context, msg channel.Message) (*channel.OutgoingMessage, error) {
		if shouldFail.Load() {
			return nil, errors.New("error")
		}
		return &channel.OutgoingMessage{Content: "success"}, nil
	})

	ctx := context.Background()
	if err := q.Start(ctx, 1); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}

	msg := channel.Message{
		ID:      "manual-retry-msg",
		Content: "test",
	}
	id, err := q.Enqueue("test", msg)
	if err != nil {
		t.Fatalf("failed to enqueue: %v", err)
	}

	// Wait for failure
	time.Sleep(100 * time.Millisecond)

	qm, _ := q.Get(id)
	if qm.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, qm.Status)
	}

	// Fix the handler and retry
	shouldFail.Store(false)
	if err := q.Retry(id); err != nil {
		t.Fatalf("failed to retry: %v", err)
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	qm, _ = q.Get(id)
	if qm.Status != StatusCompleted {
		t.Errorf("expected status %s after retry, got %s", StatusCompleted, qm.Status)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	q.Stop(stopCtx)
}

func TestList(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	q := New(cfg, logger)

	// Add some messages
	for i := 0; i < 5; i++ {
		msg := channel.Message{
			ID:      "list-msg-" + string(rune('0'+i)),
			Content: "test",
		}
		q.Enqueue("test", msg)
	}

	// List all
	all := q.List("", 0)
	if len(all) != 5 {
		t.Errorf("expected 5 messages, got %d", len(all))
	}

	// List with limit
	limited := q.List("", 3)
	if len(limited) != 3 {
		t.Errorf("expected 3 messages, got %d", len(limited))
	}

	// List by status
	pending := q.List(StatusPending, 0)
	if len(pending) != 5 {
		t.Errorf("expected 5 pending messages, got %d", len(pending))
	}
}

func TestDelete(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	q := New(cfg, logger)

	msg := channel.Message{
		ID:      "delete-msg",
		Content: "test",
	}
	id, _ := q.Enqueue("test", msg)

	// Delete
	if err := q.Delete(id); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	// Verify deleted
	_, exists := q.Get(id)
	if exists {
		t.Error("message should have been deleted")
	}

	// Delete non-existent
	if err := q.Delete("non-existent"); err == nil {
		t.Error("expected error when deleting non-existent message")
	}
}

func TestStats(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()

	q := New(cfg, logger)

	// Initial stats
	stats := q.Stats()
	if stats.TotalEnqueued != 0 {
		t.Errorf("expected 0 enqueued, got %d", stats.TotalEnqueued)
	}

	// Enqueue some messages
	for i := 0; i < 3; i++ {
		msg := channel.Message{
			ID:      "stats-msg-" + string(rune('0'+i)),
			Content: "test",
		}
		q.Enqueue("test", msg)
	}

	stats = q.Stats()
	if stats.TotalEnqueued != 3 {
		t.Errorf("expected 3 enqueued, got %d", stats.TotalEnqueued)
	}
	if stats.CurrentSize != 3 {
		t.Errorf("expected current size 3, got %d", stats.CurrentSize)
	}
}

func TestCleanup(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.RetentionSeconds = 1
	cfg.CleanupIntervalSeconds = 1

	q := New(cfg, logger)

	q.SetHandler(func(ctx context.Context, msg channel.Message) (*channel.OutgoingMessage, error) {
		return &channel.OutgoingMessage{Content: "ok"}, nil
	})

	ctx := context.Background()
	if err := q.Start(ctx, 1); err != nil {
		t.Fatalf("failed to start queue: %v", err)
	}

	msg := channel.Message{
		ID:      "cleanup-msg",
		Content: "test",
	}
	id, _ := q.Enqueue("test", msg)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify completed
	qm, exists := q.Get(id)
	if !exists || qm.Status != StatusCompleted {
		t.Fatal("message should be completed")
	}

	// Wait for cleanup
	time.Sleep(2 * time.Second)

	// Verify cleaned up
	_, exists = q.Get(id)
	if exists {
		t.Error("message should have been cleaned up")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	q.Stop(stopCtx)
}

func TestConcurrentEnqueue(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig()
	cfg.MaxSize = 1000

	q := New(cfg, logger)

	// Enqueue concurrently
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 10; j++ {
				msg := channel.Message{
					ID:      "concurrent-" + string(rune('0'+n)) + "-" + string(rune('0'+j)),
					Content: "test",
				}
				q.Enqueue("test", msg)
			}
			done <- struct{}{}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := q.Stats()
	if stats.TotalEnqueued != 100 {
		t.Errorf("expected 100 enqueued, got %d", stats.TotalEnqueued)
	}
}
