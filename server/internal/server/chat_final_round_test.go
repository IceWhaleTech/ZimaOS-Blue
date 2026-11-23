package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func TestChatFinalReportAtRoundLimit(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "stream"}[stream], func(t *testing.T) {
			store, err := memory.NewStore(":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			conv, err := store.CreateConversation(context.Background(), "last-round news report")
			if err != nil {
				t.Fatal(err)
			}
			report := "新华网简报：测试要闻甲。来源：https://www.xinhuanet.com/ 。日期尚未逐篇核实。"
			provider := &scriptedChatProvider{name: "final-round", responses: []llm.ChatResponse{
				{Message: llm.Message{Role: llm.RoleAssistant, Content: "- [ ] 核对逐篇发布日期\n- [ ] 汇总要闻", ToolCalls: []llm.ToolCall{{ID: "read-page", Name: "exec", Arguments: `{"command":"blue web_query input=\"https://www.xinhuanet.com/\""}`}}}, Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12}},
				{Message: llm.Message{Role: llm.RoleAssistant, Content: report}, Usage: llm.Usage{PromptTokens: 20, CompletionTokens: 8, TotalTokens: 28}},
			}}
			providers := llm.NewProviderRegistry()
			providers.Register(provider)
			h := newClarifyNoneSelectionChatHandler(t, store, providers, nil)
			h.SetSystemPromptBuilder(agentcore.NewSystemPromptBuilder(&agentcore.Config{WorkspaceDir: t.TempDir()}))
			enabled := true
			limit := 2
			h.settingsHandler.settings.AgentMode = &enabled
			h.settingsHandler.settings.AgentLoopPolicyMaxToolRounds = &limit
			sessions := tools.NewSessionRegistry()
			defer sessions.Cleanup()
			cfg := tools.DefaultExecConfig()
			cfg.Security = tools.ExecSecurityDeny
			execTool := tools.NewExecTool(cfg, sessions, nil, nil, nil)
			execTool.SetSkillExecutor(func(context.Context, string, map[string]any) (map[string]string, error) {
				return map[string]string{"content": "测试要闻甲", "url": "https://www.xinhuanet.com/"}, nil
			})
			h.toolRegistry.Register(execTool)
			request, _ := json.Marshal(map[string]interface{}{"message": newsToolRejectionGoal(t), "provider": provider.Name(), "model": "gpt-5.3-codex-spark"})
			if stream {
				body := runStreamTurn(t, h, conv.ID, string(request))
				final := ""
				for _, e := range extractJSONSSEEvents(t, body) {
					if c, ok := e["content"].(string); ok {
						final = c
					}
					if e["error"] != nil {
						t.Fatalf("unexpected error: %v", e)
					}
				}
				if !strings.Contains(final, report) {
					t.Fatalf("last-round final content lost: final=%q", final)
				}
			} else {
				e := echo.New()
				e.POST("/conversations/:id/messages", h.SendMessage)
				req := httptest.NewRequest(http.MethodPost, "/conversations/"+conv.ID+"/messages", bytes.NewReader(request))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				result := rec.Body.String()
				if !strings.Contains(result, report) {
					t.Fatalf("last-round final content lost: %s", result)
				}
			}
			if provider.CallCount() != 2 {
				t.Fatalf("calls=%d want2", provider.CallCount())
			}
			msg, err := store.GetLatestAssistantMessage(context.Background(), conv.ID)
			if err != nil || msg == nil || !strings.Contains(msg.Content, report) {
				t.Fatalf("last-round report not saved: msg=%+v err=%v", msg, err)
			}
			if msg.Stats == nil || msg.Stats.InputTokens != 30 || msg.Stats.OutputTokens != 10 {
				t.Fatalf("last-round usage lost: %+v", msg.Stats)
			}
		})
	}
}
