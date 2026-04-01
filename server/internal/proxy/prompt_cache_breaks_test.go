package proxy

import (
	stdjson "encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/sjson"
)

func TestPromptCacheBreakDetector_IgnoresDynamicOnlyChanges(t *testing.T) {
	detector := NewPromptCacheBreakDetector(0)

	first := promptCacheSnapshotForTest(t, anthropicPromptCacheTestRequest{
		Model:       "claude-sonnet-4-5",
		StaticText:  "stable static guidance",
		ConfigText:  "config guidance",
		DynamicText: "now=1",
	})
	if obs := detector.Observe(first, 6000); obs != nil {
		t.Fatalf("expected no observation for first snapshot, got %#v", obs)
	}

	second := promptCacheSnapshotForTest(t, anthropicPromptCacheTestRequest{
		Model:       "claude-sonnet-4-5",
		StaticText:  "stable static guidance",
		ConfigText:  "config guidance",
		DynamicText: "now=2",
	})
	if obs := detector.Observe(second, 2000); obs != nil {
		t.Fatalf("expected dynamic-only change to stay silent, got %#v", obs)
	}
}

func TestPromptCacheBreakDetector_AttributesToolSchemaChanges(t *testing.T) {
	detector := NewPromptCacheBreakDetector(0)

	first := promptCacheSnapshotForTest(t, anthropicPromptCacheTestRequest{
		Model:       "claude-sonnet-4-5",
		StaticText:  "stable static guidance",
		ConfigText:  "config guidance",
		DynamicText: "now=1",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{"type": "string"},
			},
		},
	})
	_ = detector.Observe(first, 6000)

	second := promptCacheSnapshotForTest(t, anthropicPromptCacheTestRequest{
		Model:       "claude-sonnet-4-5",
		StaticText:  "stable static guidance",
		ConfigText:  "config guidance",
		DynamicText: "now=2",
		ToolSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string"},
			},
		},
	})
	obs := detector.Observe(second, 500)
	if obs == nil {
		t.Fatal("expected tool schema change observation")
	}
	if !containsString(obs.Reasons, "tool_schemas_changed") {
		t.Fatalf("reasons = %#v, want tool_schemas_changed", obs.Reasons)
	}
	if !strings.Contains(obs.Summary, "tool schema changed") {
		t.Fatalf("summary = %q, want tool schema change detail", obs.Summary)
	}
}

func TestPromptCacheBreakDetector_AttributesCacheControlChanges(t *testing.T) {
	detector := NewPromptCacheBreakDetector(0)

	body := anthropicPromptCacheTestBody(t, anthropicPromptCacheTestRequest{
		Model:       "claude-sonnet-4-5",
		StaticText:  "stable static guidance",
		ConfigText:  "config guidance",
		DynamicText: "now=1",
	})
	first := BuildPromptCacheStateSnapshot(body, "anthropic", "claude-sonnet-4-5", "/v1/messages")
	_ = detector.Observe(first, 6000)

	mutated, err := sjson.DeleteBytes(body, "system.1.cache_control")
	if err != nil {
		t.Fatalf("delete cache_control from anthropic request: %v", err)
	}

	second := BuildPromptCacheStateSnapshot(mutated, "anthropic", "claude-sonnet-4-5", "/v1/messages")
	obs := detector.Observe(second, 500)
	if obs == nil {
		t.Fatal("expected cache-control change observation")
	}
	if !containsString(obs.Reasons, "cache_control_changed") {
		t.Fatalf("reasons = %#v, want cache_control_changed", obs.Reasons)
	}
}

func TestPromptCacheBreakDetector_AttributesModelAndExtraBodyChanges(t *testing.T) {
	detector := NewPromptCacheBreakDetector(0)

	firstBody := anthropicPromptCacheTestBody(t, anthropicPromptCacheTestRequest{
		Model:       "claude-sonnet-4-5",
		StaticText:  "stable static guidance",
		ConfigText:  "config guidance",
		DynamicText: "now=1",
		Temperature: 0.1,
	})
	first := BuildPromptCacheStateSnapshot(firstBody, "anthropic", "claude-sonnet-4-5", "/v1/messages")
	_ = detector.Observe(first, 6000)

	secondBody := anthropicPromptCacheTestBody(t, anthropicPromptCacheTestRequest{
		Model:       "claude-opus-4-1",
		StaticText:  "stable static guidance",
		ConfigText:  "config guidance",
		DynamicText: "now=2",
		Temperature: 0.7,
	})
	second := BuildPromptCacheStateSnapshot(secondBody, "anthropic", "claude-opus-4-1", "/v1/messages")
	obs := detector.Observe(second, 500)
	if obs == nil {
		t.Fatal("expected model/extra-body change observation")
	}
	if !containsString(obs.Reasons, "model_changed") {
		t.Fatalf("reasons = %#v, want model_changed", obs.Reasons)
	}
	if !containsString(obs.Reasons, "extra_body_changed") {
		t.Fatalf("reasons = %#v, want extra_body_changed", obs.Reasons)
	}
}

type anthropicPromptCacheTestRequest struct {
	Model       string
	StaticText  string
	ConfigText  string
	DynamicText string
	ToolSchema  map[string]any
	Temperature float64
}

func anthropicPromptCacheTestBody(t *testing.T, req anthropicPromptCacheTestRequest) []byte {
	t.Helper()

	toolSchema := req.ToolSchema
	if toolSchema == nil {
		toolSchema = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"input": map[string]any{"type": "string"},
			},
		}
	}

	anthropicReq := AnthropicRequest{
		Model:       req.Model,
		MaxTokens:   256,
		Temperature: req.Temperature,
		System: []AnthropicSystemBlock{
			{Type: "text", Text: req.StaticText},
			{Type: "text", Text: req.ConfigText},
			{Type: "text", Text: req.DynamicText},
		},
		Tools: []AnthropicTool{{
			Name:        "web_query",
			Description: "Search the web",
			InputSchema: toolSchema,
		}},
		Messages: []AnthropicMessage{{
			Role:    "user",
			Content: "hello",
		}},
	}
	ApplyPromptCaching(&anthropicReq)
	body, err := stdjson.Marshal(anthropicReq)
	if err != nil {
		t.Fatalf("marshal anthropic request: %v", err)
	}
	return body
}

func promptCacheSnapshotForTest(t *testing.T, req anthropicPromptCacheTestRequest) PromptCacheStateSnapshot {
	t.Helper()
	return BuildPromptCacheStateSnapshot(anthropicPromptCacheTestBody(t, req), "anthropic", req.Model, "/v1/messages")
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
