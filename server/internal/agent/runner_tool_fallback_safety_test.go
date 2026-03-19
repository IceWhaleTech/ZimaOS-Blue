package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type recursiveRunnerPayload struct {
	Name string                  `json:"name"`
	Self *recursiveRunnerPayload `json:"self,omitempty"`
}

type captureToolLoopLLM struct {
	callIndex int
	requests  []llm.ChatRequest
}

func (m *captureToolLoopLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.requests = append(m.requests, req)
	m.callIndex++
	if m.callIndex == 1 {
		return &llm.ChatResponse{
			Message: llm.Message{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{{
					ID:        "call-recursive",
					Name:      "recursive_tool",
					Arguments: `{}`,
				}},
			},
		}, nil
	}
	return &llm.ChatResponse{
		Message: llm.Message{Role: llm.RoleAssistant, Content: "done"},
	}, nil
}

func TestExecuteLoopWithToolsWithoutGatewayGuardsRecursivePayloads(t *testing.T) {
	store := testStore(t)
	registry := tools.NewRegistry()
	payload := &recursiveRunnerPayload{Name: "root"}
	payload.Self = payload
	mock := tools.NewMockTool("recursive_tool", "recursive payload tool")
	mock.SetResult(payload)
	registry.Register(mock)

	llmStub := &captureToolLoopLLM{}
	runner := NewRunner(store, llmStub, registry, tools.NewExecutor(registry), nil, RunnerConfig{
		MaxToolRoundsPerStep: 2,
	})
	runner.toolGateway = nil

	task := &Task{
		ID:             "task-recursive",
		UserID:         "u1",
		ConversationID: "conv-recursive",
	}
	_, err := runner.executeLoopWithTools(context.Background(), task, 0, "inspect tool result", "system prompt", "user prompt", []llm.Tool{{
		Name:       "recursive_tool",
		Parameters: map[string]interface{}{"type": "object"},
	}})
	if err != nil {
		t.Fatalf("executeLoopWithTools() error = %v", err)
	}
	if len(llmStub.requests) < 2 {
		t.Fatalf("requests = %d, want at least 2", len(llmStub.requests))
	}
	var toolMessage *llm.Message
	for i := range llmStub.requests[1].Messages {
		msg := &llmStub.requests[1].Messages[i]
		if msg.Role == llm.RoleTool && msg.ToolCallID == "call-recursive" {
			toolMessage = msg
			break
		}
	}
	if toolMessage == nil {
		t.Fatalf("second request missing tool message: %#v", llmStub.requests[1].Messages)
	}
	if !strings.Contains(toolMessage.Content, `"name":"root"`) {
		t.Fatalf("tool content = %q, want name field", toolMessage.Content)
	}
	if !strings.Contains(toolMessage.Content, `[circular payload omitted]`) {
		t.Fatalf("tool content = %q, want circular marker", toolMessage.Content)
	}
}
