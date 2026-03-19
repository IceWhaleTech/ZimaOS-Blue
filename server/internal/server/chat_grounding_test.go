package server

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type groundingProviderStub struct {
	name    string
	content string
	err     error
}

func (p groundingProviderStub) Name() string { return p.name }

func (p groundingProviderStub) Models() []string { return []string{"auto"} }

func (p groundingProviderStub) Chat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.err != nil {
		return nil, p.err
	}
	return &llm.ChatResponse{
		Model: "auto",
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: p.content,
		},
	}, nil
}

func (p groundingProviderStub) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk)
	close(ch)
	return ch, nil
}

func (p groundingProviderStub) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return nil
}

type chatRuntimeMetricsStub struct {
	counts map[string]int64
}

func (m *chatRuntimeMetricsStub) RecordAPICall(string, bool, float64, int64, int64, int64, int64, string) {
}

func (m *chatRuntimeMetricsStub) RecordAPICallForUser(string, string, bool, float64, int64, int64, int64, int64, string) {
}

func (m *chatRuntimeMetricsStub) RecordSpeed(string, float64, float64, float64) {}

func (m *chatRuntimeMetricsStub) RecordCounter(name string, value int64, _ map[string]string) {
	if m.counts == nil {
		m.counts = make(map[string]int64)
	}
	m.counts[name] += value
}

func enableChatGroundingForTest(handler *ChatHandler) {
	handler.SetFlagEvaluator(config.NewFlagEvaluator(&config.GrayscaleConfig{
		Enabled: true,
		Flags: []config.FeatureFlag{{
			Name:       chatGroundingVerificationFlagName,
			Enabled:    true,
			Percentage: 100,
		}},
	}))
}

func TestRunChatGroundingVerificationProducesVerifiedOutput(t *testing.T) {
	providers := llm.NewProviderRegistry()
	providers.Register(groundingProviderStub{
		name: "grounding",
		content: `{"summary":"The file contains the expected text.","claims":[` +
			`{"type":"file_content_excerpt","tool_call_ids":["call-1"],"path":"README.md","excerpt":"hello grounded world"}` +
			`]}`,
	})

	handler := NewChatHandler(nil, providers, tools.NewRegistry())
	enableChatGroundingForTest(handler)
	outcome := handler.runChatGroundingVerification(context.Background(), chatGroundingRequest{
		Model:     "auto",
		UserID:    "user-1",
		SessionID: "conv-1",
		Locale:    "en-US",
		Goal:      "Read the README",
		Draft:     "The README says hello grounded world.",
		Messages: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{{
					ID:        "call-1",
					Name:      "read",
					Arguments: `{"path":"README.md"}`,
				}},
			},
			{
				Role:       llm.RoleTool,
				ToolCallID: "call-1",
				ToolName:   "read",
				Content:    `{"path":"README.md","content":"hello grounded world","size":20}`,
			},
		},
	})

	if !outcome.Used {
		t.Fatal("expected grounding to run")
	}
	if outcome.Status != "grounded" {
		t.Fatalf("status = %q, want grounded", outcome.Status)
	}
	if !strings.Contains(outcome.Output, "Verified") {
		t.Fatalf("output = %q, want verified section", outcome.Output)
	}
	if !strings.Contains(outcome.Output, `README.md excerpt="hello grounded world"`) {
		t.Fatalf("output = %q, want grounded excerpt", outcome.Output)
	}
}

func TestRunChatGroundingVerificationFallsBackToUnknownAndRecordsMetric(t *testing.T) {
	providers := llm.NewProviderRegistry()
	providers.Register(groundingProviderStub{
		name:    "grounding",
		content: `{"summary":`,
	})

	handler := NewChatHandler(nil, providers, tools.NewRegistry())
	enableChatGroundingForTest(handler)
	metrics := &chatRuntimeMetricsStub{}
	handler.SetMetricsRecorder(metrics)

	outcome := handler.runChatGroundingVerification(context.Background(), chatGroundingRequest{
		Model:     "auto",
		UserID:    "user-1",
		SessionID: "conv-2",
		Locale:    "en-US",
		Goal:      "Inspect the file",
		Draft:     "The file definitely contains the requested text.",
		Messages: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{{
					ID:        "call-2",
					Name:      "read",
					Arguments: `{"path":"README.md"}`,
				}},
			},
			{
				Role:       llm.RoleTool,
				ToolCallID: "call-2",
				ToolName:   "read",
				Content:    `{"path":"README.md","content":"hello grounded world","size":20}`,
			},
		},
	})

	if !outcome.Used {
		t.Fatal("expected grounding to run")
	}
	if outcome.Status != "fallback" {
		t.Fatalf("status = %q, want fallback", outcome.Status)
	}
	if !strings.Contains(outcome.Output, "Unverified") {
		t.Fatalf("output = %q, want unverified section", outcome.Output)
	}
	if got := metrics.counts["grounding_fallback_total"]; got != 1 {
		t.Fatalf("grounding_fallback_total = %d, want 1", got)
	}
}

func TestRunChatGroundingVerificationRespectsDisabledFlag(t *testing.T) {
	providers := llm.NewProviderRegistry()
	providers.Register(groundingProviderStub{
		name:    "grounding",
		content: `{"summary":"ignored","claims":[{"type":"unknown","text":"unknown"}]}`,
	})

	handler := NewChatHandler(nil, providers, tools.NewRegistry())
	handler.SetFlagEvaluator(config.NewFlagEvaluator(&config.GrayscaleConfig{
		Enabled: true,
		Flags: []config.FeatureFlag{{
			Name:    chatGroundingVerificationFlagName,
			Enabled: false,
		}},
	}))

	outcome := handler.runChatGroundingVerification(context.Background(), chatGroundingRequest{
		Model:     "auto",
		UserID:    "user-1",
		SessionID: "conv-3",
		Locale:    "en-US",
		Goal:      "Inspect the file",
		Draft:     "This draft should remain untouched.",
		Messages: []llm.Message{
			{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{{
					ID:        "call-3",
					Name:      "read",
					Arguments: `{"path":"README.md"}`,
				}},
			},
			{
				Role:       llm.RoleTool,
				ToolCallID: "call-3",
				ToolName:   "read",
				Content:    `{"path":"README.md","content":"hello grounded world","size":20}`,
			},
		},
	})

	if outcome.Used {
		t.Fatalf("expected grounding to be skipped, got %+v", outcome)
	}
}

func TestWrapStreamingChatGroundingBodyAddsNotePrefix(t *testing.T) {
	got := wrapStreamingChatGroundingBody("en-US", "Verified:\n- README exists")
	if !strings.HasPrefix(got, "Grounding Note:") {
		t.Fatalf("note = %q, want Grounding Note prefix", got)
	}

	zh := wrapStreamingChatGroundingBody("zh-CN", "已验证内容：\n- README 存在")
	if !strings.HasPrefix(zh, "校验说明：") {
		t.Fatalf("zh note = %q, want Chinese prefix", zh)
	}
}

func TestGroundingProviderStubError(t *testing.T) {
	providers := llm.NewProviderRegistry()
	providers.Register(groundingProviderStub{name: "grounding", err: fmt.Errorf("boom")})

	handler := NewChatHandler(nil, providers, tools.NewRegistry())
	enableChatGroundingForTest(handler)
	outcome := handler.runChatGroundingVerification(context.Background(), chatGroundingRequest{
		Model:     "auto",
		UserID:    "user-1",
		SessionID: "conv-4",
		Draft:     "test",
		Messages: []llm.Message{
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "call-4", Name: "exec", Arguments: `{"command":"pwd"}`}}},
			{Role: llm.RoleTool, ToolCallID: "call-4", ToolName: "exec", Content: `{"stdout":"/tmp","exit_code":0}`},
		},
	})
	if !outcome.Used || outcome.Status != "fallback" {
		t.Fatalf("outcome = %+v, want used fallback", outcome)
	}
}
