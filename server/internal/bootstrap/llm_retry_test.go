package bootstrap

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestLLMChatRetryConfig_AppliesPerAttemptTimeoutAndRetries(t *testing.T) {
	cfg := llmChatRetryConfig{
		maxRetries:     1,
		classifier:     llm.NewDefaultErrorClassifier(),
		attemptTimeout: 20 * time.Millisecond,
		sleep: func(context.Context, time.Duration) error {
			return nil
		},
	}

	var attempts int
	var firstRemaining time.Duration

	resp, err := cfg.chat(context.Background(), llm.ChatRequest{Model: "auto"}, func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		_ = req
		attempts++
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected retry attempt context deadline")
		}
		remaining := time.Until(deadline)
		if attempts == 1 {
			firstRemaining = remaining
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return &llm.ChatResponse{Message: llm.Message{Content: "ok"}}, nil
	})
	if err != nil {
		t.Fatalf("chat returned error: %v", err)
	}
	if resp == nil || resp.Message.Content != "ok" {
		t.Fatalf("unexpected response: %#v", resp)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if firstRemaining <= 0 || firstRemaining > 100*time.Millisecond {
		t.Fatalf("first attempt remaining timeout = %v, want within (0,100ms]", firstRemaining)
	}
}

func TestLLMChatRetryConfig_PreservesShorterParentDeadline(t *testing.T) {
	cfg := llmChatRetryConfig{
		maxRetries:     1,
		classifier:     llm.NewDefaultErrorClassifier(),
		attemptTimeout: time.Second,
		sleep: func(context.Context, time.Duration) error {
			return nil
		},
	}

	parentCtx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	var attempts int
	_, err := cfg.chat(parentCtx, llm.ChatRequest{Model: "auto"}, func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
		_ = req
		attempts++
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1 when parent deadline is exhausted", attempts)
	}
}
