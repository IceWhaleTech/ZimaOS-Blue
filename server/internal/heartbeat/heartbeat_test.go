package heartbeat

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func baseDeps(chatFn ChatFunc) RunDeps {
	return RunDeps{
		Config: &Config{
			Enabled:     true,
			LLMModel:    "test-model",
			AckMaxChars: 300,
			Visibility:  VisibilityConfig{ShowAlerts: true},
		},
		ChatFn: chatFn,
		Logger: zap.NewNop(),
	}
}

func okChatFn(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "HEARTBEAT_OK"}}, nil
}

func TestRunOnce_NilChatFunc(t *testing.T) {
	deps := baseDeps(nil)
	deps.ChatFn = nil

	result := RunOnce(context.Background(), deps)
	if result.Status != "failed" || result.Reason != "no-chat-func" {
		t.Errorf("expected failed/no-chat-func, got %s/%s", result.Status, result.Reason)
	}
}

func TestRunOnce_WithChatFunc_Success(t *testing.T) {
	deps := baseDeps(okChatFn)
	result := RunOnce(context.Background(), deps)
	if result.Status != "ran" {
		t.Errorf("expected status=ran, got %s (reason=%s)", result.Status, result.Reason)
	}
}

func TestRunOnce_LLMError(t *testing.T) {
	deps := baseDeps(func(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
		return nil, errors.New("connection refused")
	})

	result := RunOnce(context.Background(), deps)
	if result.Status != "failed" {
		t.Errorf("expected status=failed, got %s", result.Status)
	}
	if result.Reason != "llm-error: connection refused" {
		t.Errorf("unexpected reason: %s", result.Reason)
	}
}

func TestRunOnce_Disabled(t *testing.T) {
	deps := baseDeps(nil)
	deps.Config.Enabled = false

	result := RunOnce(context.Background(), deps)
	if result.Status != "skipped" || result.Reason != "disabled" {
		t.Errorf("expected skipped/disabled, got %s/%s", result.Status, result.Reason)
	}
}

func TestRunOnce_AlertDelivery(t *testing.T) {
	deps := baseDeps(func(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: "Disk usage is at 95%! Please clean up."},
		}, nil
	})

	result := RunOnce(context.Background(), deps)
	if result.Status != "ran" {
		t.Errorf("expected status=ran, got %s (reason=%s)", result.Status, result.Reason)
	}
}
