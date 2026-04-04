package server

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestExecuteToolCallsUsesGatewayCompactPayload(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(&staticToolMock{
		def: tools.ToolDefinition{
			Name:        "web_query",
			Description: "fetch",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{"type": "string"},
				},
				"required": []string{"input"},
			},
		},
		result: "\x1b[31mexternal content\x1b[0m",
	})
	handler := NewChatHandler(nil, nil, registry)

	results := handler.executeToolCalls(context.Background(), []llm.ToolCall{{
		ID:        "call-1",
		Name:      "web_query",
		Arguments: `{"input":"https://example.com"}`,
	}})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !strings.Contains(results[0].Content, `"trust":"untrusted_external_content"`) {
		t.Fatalf("expected compact payload wrapper, got %q", results[0].Content)
	}
	if strings.Contains(results[0].Content, "\x1b[31m") {
		t.Fatalf("expected ANSI to be stripped, got %q", results[0].Content)
	}
}

func TestExecuteToolCallsSupportsRuntimeExecOverlay(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterExecTools(registry, tools.DefaultExecConfig(), nil, nil, nil)
	handler := NewChatHandler(nil, nil, registry)

	results := handler.executeToolCalls(context.Background(), []llm.ToolCall{{
		ID:        "call-exec",
		Name:      "exec",
		Arguments: `{"command":"echo exec-overlay-ok"}`,
	}})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if strings.Contains(results[0].Content, "tool_not_found") || strings.Contains(results[0].Content, "not registered or visible") {
		t.Fatalf("expected exec tool call to succeed through gateway, got %q", results[0].Content)
	}
	if !strings.Contains(results[0].Content, "exec-overlay-ok") {
		t.Fatalf("expected exec output in compact payload, got %q", results[0].Content)
	}
}
