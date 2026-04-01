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

type searchRecoveryLLM struct {
	callIndex int
	requests  []llm.ChatRequest
}

func (m *searchRecoveryLLM) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.requests = append(m.requests, req)
	m.callIndex++
	switch m.callIndex {
	case 1:
		return &llm.ChatResponse{
			Message: llm.Message{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{{
					ID:        "call-web-1",
					Name:      "web_query",
					Arguments: `{"input":"openai responses api docs"}`,
				}},
			},
		}, nil
	case 2:
		return &llm.ChatResponse{
			Message: llm.Message{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{{
					ID:        "call-web-2",
					Name:      "web_query",
					Arguments: `{"input":"latest openai responses api docs"}`,
				}},
			},
		}, nil
	case 3:
		return &llm.ChatResponse{
			Message: llm.Message{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{{
					ID:        "call-web-3",
					Name:      "web_query",
					Arguments: `{"input":"openai responses api latest documentation"}`,
				}},
			},
		}, nil
	default:
		last := req.Messages[len(req.Messages)-1]
		if last.Role != llm.RoleUser || !strings.Contains(last.Content, "Tool execution is repeating without clear progress.") {
			return &llm.ChatResponse{
				Message: llm.Message{Role: llm.RoleAssistant, Content: "missing recovery nudge"},
			}, nil
		}
		return &llm.ChatResponse{
			Message: llm.Message{Role: llm.RoleAssistant, Content: "The latest Responses API docs are on the OpenAI platform docs and the step is complete."},
		}, nil
	}
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

func TestExecuteLoopWithToolsInjectsRecoveryBeforeAbortingLoop(t *testing.T) {
	store := testStore(t)
	registry := tools.NewRegistry()
	web := tools.NewMockTool("web_query", "mock web query")
	web.SetResult(map[string]interface{}{
		"status":     "ok",
		"mode":       "search_read",
		"target_url": "https://platform.openai.com/docs/api-reference/responses",
	})
	registry.Register(web)

	llmStub := &searchRecoveryLLM{}
	runner := NewRunner(store, llmStub, registry, tools.NewExecutor(registry), nil, RunnerConfig{
		MaxToolRoundsPerStep: 6,
	})
	runner.toolGateway = nil

	task := &Task{
		ID:             "task-search-recovery",
		UserID:         "u1",
		ConversationID: "conv-search-recovery",
	}
	output, err := runner.executeLoopWithTools(context.Background(), task, 0, "find the latest docs", "system prompt", "user prompt", []llm.Tool{{
		Name:       "web_query",
		Parameters: map[string]interface{}{"type": "object"},
	}})
	if err != nil {
		t.Fatalf("executeLoopWithTools() error = %v", err)
	}
	if !strings.Contains(output, "step is complete") {
		t.Fatalf("output = %q, want recovered completion", output)
	}
	if len(llmStub.requests) != 4 {
		t.Fatalf("requests = %d, want 4", len(llmStub.requests))
	}
	lastReq := llmStub.requests[3]
	lastMsg := lastReq.Messages[len(lastReq.Messages)-1]
	if lastMsg.Role != llm.RoleUser || !strings.Contains(lastMsg.Content, "Tool execution is repeating without clear progress.") {
		t.Fatalf("last recovery message = %#v, want loop recovery nudge", lastMsg)
	}
}
