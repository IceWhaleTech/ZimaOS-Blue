package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func TestLocalAnswerOutcome(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, empty := range []bool{false, true} {
			name := "sync"
			if stream {
				name = "stream"
			}
			if empty {
				name += "/missing_reply"
			} else {
				name += "/complete_reply"
			}
			t.Run(name, func(t *testing.T) {
				store, err := memory.NewStore(":memory:")
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()
				conv, err := store.CreateConversation(context.Background(), "Local outcome")
				if err != nil {
					t.Fatal(err)
				}
				const answer = `{"status":"answered","answers":[{"key":"gate","value":"双人复核"}],"citations":[{"source":"release.txt","quote":"发布前完成双人复核。"}]}`
				responses := []llm.ChatResponse{{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "local_read", Name: "bash", Arguments: `{"command":"cat release.txt"}`}}}}}
				if empty {
					responses = append(responses, llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant}})
				}
				responses = append(responses, llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: answer}})
				provider := &scriptedChatProvider{name: "local-outcome", responses: responses}
				registry := llm.NewProviderRegistry()
				registry.Register(provider)
				toolRegistry := tools.NewRegistry()
				stdout := strings.Repeat("source context\n", 80) + "发布前完成双人复核。"
				toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "bash"}, result: map[string]interface{}{"status": "completed", "exit_code": 0, "command": "cat release.txt", "stdout": stdout}})
				h := NewChatHandler(store, registry, toolRegistry)
				h.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
				body, err := json.Marshal(map[string]interface{}{"message": `{"corpus":"/tmp/data","task":{"question":"发布前完成哪项工作？"},"answerSchema":{"type":"object","properties":{"citations":{"type":"array"}}},"instruction":"根据 corpus 内文件回答，只输出 JSON。"}`, "provider": provider.Name(), "model": "gpt-5.3-codex-spark"})
				if err != nil {
					t.Fatal(err)
				}
				var content string
				if stream {
					for _, e := range extractJSONSSEEvents(t, runStreamTurn(t, h, conv.ID, string(body))) {
						if final, ok := e["content"].(string); ok && e["done"] == true {
							content = final
						}
					}
				} else {
					e := echo.New()
					req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewReader(body))
					req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
					rec := httptest.NewRecorder()
					c := e.NewContext(req, rec)
					c.SetParamNames("id")
					c.SetParamValues(conv.ID)
					if err := h.SendMessage(c); err != nil {
						t.Fatal(err)
					}
					var response map[string]interface{}
					if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
						t.Fatal(err)
					}
					content, _ = response["content"].(string)
				}
				if stripGroundingTypelessBlocks(content) != answer {
					t.Errorf("final answer = %q, want unchanged JSON", content)
				}
				if provider.CallCount() != len(responses) {
					t.Errorf("rounds = %d, want %d: continue only until the requested reply exists", provider.CallCount(), len(responses))
				}
				request, ok := provider.RequestAt(1)
				if !ok {
					t.Fatal("missing post-tool request")
				}
				found := false
				for _, m := range request.Messages {
					if m.Role == llm.RoleTool && strings.Contains(m.Content, "发布前完成双人复核。") {
						found = true
					}
				}
				if !found {
					t.Error("post-tool model request lost the evidence needed for the outcome")
				}
			})
		}
	}
}
