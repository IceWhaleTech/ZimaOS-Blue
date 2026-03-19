package server

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type recursiveChatToolPayload struct {
	Name string                    `json:"name"`
	Self *recursiveChatToolPayload `json:"self,omitempty"`
}

func TestExecuteToolCallsWithoutGatewayGuardsRecursivePayloads(t *testing.T) {
	registry := tools.NewRegistry()
	payload := &recursiveChatToolPayload{Name: "root"}
	payload.Self = payload
	registry.Register(&staticToolMock{
		def: tools.ToolDefinition{
			Name:        "recursive_tool",
			Description: "recursive payload tool",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		result: payload,
	})
	handler := NewChatHandler(nil, nil, registry)
	handler.toolGateway = nil

	results := handler.executeToolCalls(context.Background(), []llm.ToolCall{{
		ID:        "call-recursive",
		Name:      "recursive_tool",
		Arguments: `{}`,
	}})
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !strings.Contains(results[0].Content, `"name":"root"`) {
		t.Fatalf("content = %q, want name field", results[0].Content)
	}
	if !strings.Contains(results[0].Content, `[circular payload omitted]`) {
		t.Fatalf("content = %q, want circular marker", results[0].Content)
	}
}
