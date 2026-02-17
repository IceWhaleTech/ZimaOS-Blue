package heartbeat

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

// mockProvider implements llm.Provider for testing.
type mockProvider struct {
	name    string
	chatFn  func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

func (m *mockProvider) Name() string    { return m.name }
func (m *mockProvider) Models() []string { return []string{"test-model"} }
func (m *mockProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, req)
	}
	return &llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: "HEARTBEAT_OK"}}, nil
}
func (m *mockProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("not implemented")
}
func (m *mockProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, cb llm.StreamCallback) error {
	return errors.New("not implemented")
}

func baseDeps(registry *llm.ProviderRegistry) RunDeps {
	return RunDeps{
		Config: &Config{
			Enabled:     true,
			LLMProvider: "test",
			LLMModel:    "test-model",
			AckMaxChars: 300,
			Visibility:  VisibilityConfig{ShowAlerts: true},
		},
		LLMRegistry: registry,
		Logger:      zap.NewNop(),
	}
}

func TestRunOnce_NilRegistry(t *testing.T) {
	deps := baseDeps(nil)
	deps.LLMRegistry = nil

	result := RunOnce(context.Background(), deps)
	if result.Status != "failed" || result.Reason != "no-llm-registry" {
		t.Errorf("expected failed/no-llm-registry, got %s/%s", result.Status, result.Reason)
	}
}

func TestRunOnce_ProviderNotFound(t *testing.T) {
	registry := llm.NewProviderRegistry()
	// Register nothing — provider "test" won't be found
	deps := baseDeps(registry)

	result := RunOnce(context.Background(), deps)
	if result.Status != "failed" || result.Reason != "provider-not-found" {
		t.Errorf("expected failed/provider-not-found, got %s/%s", result.Status, result.Reason)
	}
}

func TestRunOnce_EmptyProviderFallback(t *testing.T) {
	registry := llm.NewProviderRegistry()
	registry.Register(&mockProvider{name: "fallback"})

	deps := baseDeps(registry)
	deps.Config.LLMProvider = "" // empty — should fallback to first available

	result := RunOnce(context.Background(), deps)
	if result.Status == "failed" && result.Reason == "provider-not-found" {
		t.Error("should have fallen back to 'fallback' provider, but got provider-not-found")
	}
	// Should succeed (ran) since mock returns HEARTBEAT_OK
	if result.Status != "ran" {
		t.Errorf("expected status=ran, got %s (reason=%s)", result.Status, result.Reason)
	}
}

func TestRunOnce_WithRegistry_Success(t *testing.T) {
	registry := llm.NewProviderRegistry()
	registry.Register(&mockProvider{name: "test"})

	deps := baseDeps(registry)
	result := RunOnce(context.Background(), deps)
	if result.Status != "ran" {
		t.Errorf("expected status=ran, got %s (reason=%s)", result.Status, result.Reason)
	}
}

func TestRunOnce_LLMError(t *testing.T) {
	registry := llm.NewProviderRegistry()
	registry.Register(&mockProvider{
		name: "test",
		chatFn: func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
			return nil, errors.New("connection refused")
		},
	})

	deps := baseDeps(registry)
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
	registry := llm.NewProviderRegistry()
	registry.Register(&mockProvider{
		name: "test",
		chatFn: func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{
				Message: llm.Message{Role: llm.RoleAssistant, Content: "Disk usage is at 95%! Please clean up."},
			}, nil
		},
	})

	deps := baseDeps(registry)
	result := RunOnce(context.Background(), deps)
	if result.Status != "ran" {
		t.Errorf("expected status=ran, got %s (reason=%s)", result.Status, result.Reason)
	}
}
