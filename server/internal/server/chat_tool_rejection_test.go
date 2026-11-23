package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func newsToolRejectionGoal(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../harness/testdata/outcome-delivery.json")
	if err != nil {
		t.Fatal(err)
	}
	var dataset struct {
		Items []struct {
			ID    string `json:"id"`
			Input struct {
				Goal string `json:"goal"`
			} `json:"input"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &dataset); err != nil {
		t.Fatal(err)
	}
	for _, item := range dataset.Items {
		if item.ID == "chat-xinhua-report-recovers-rejected-native-call" {
			return item.Input.Goal
		}
	}
	t.Fatal("missing Harness news regression")
	return ""
}

func TestChatRejectedNativeToolCall(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, scenario := range []struct {
			name              string
			recovers          bool
			maxRounds         int
			wantCalls         int
			missingFinalUsage bool
		}{
			{name: "recovers", recovers: true, wantCalls: 3},
			{name: "recovers_with_usage_estimate", recovers: true, wantCalls: 3, missingFinalUsage: true},
			{name: "persistent_rejection", wantCalls: 3},
			{name: "round_budget_exhausted", maxRounds: 1, wantCalls: 1},
		} {
			if stream && scenario.missingFinalUsage {
				continue
			}
			t.Run(fmt.Sprintf("stream=%v/%s", stream, scenario.name), func(t *testing.T) {
				store, err := memory.NewStore(":memory:")
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()
				conv, err := store.CreateConversation(context.Background(), "news rejection")
				if err != nil {
					t.Fatal(err)
				}
				const report = "新华网最终总结（测试）：测试要闻甲。来源：https://www.xinhuanet.com/test-news.html"
				responses := []llm.ChatResponse{{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "rejected", Name: "web_query", Arguments: `{"input":"https://www.xinhuanet.com/"}`}}}}}
				if scenario.recovers {
					responses = append(responses,
						llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "supported", Name: "exec", Arguments: `{"command":"blue web_query input=\"https://www.xinhuanet.com/\""}`}}}},
						llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: report}},
					)
				}
				for i := range responses {
					responses[i].Usage = llm.Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7}
				}
				responses[0].Usage = llm.Usage{PromptTokens: 101, CompletionTokens: 17, TotalTokens: 118}
				if scenario.missingFinalUsage {
					responses[len(responses)-1].Usage = llm.Usage{}
				}
				provider := &scriptedChatProvider{name: "news-rejection", responses: responses}
				providers := llm.NewProviderRegistry()
				providers.Register(provider)
				h := newClarifyNoneSelectionChatHandler(t, store, providers, nil)
				h.SetSystemPromptBuilder(agentcore.NewSystemPromptBuilder(&agentcore.Config{WorkspaceDir: t.TempDir()}))
				if scenario.maxRounds > 0 {
					agentMode := true
					h.settingsHandler.settings.AgentMode = &agentMode
					h.settingsHandler.settings.AgentLoopPolicyMaxToolRounds = &scenario.maxRounds
				}
				sessions := tools.NewSessionRegistry()
				defer sessions.Cleanup()
				execCfg := tools.DefaultExecConfig()
				execCfg.Security = tools.ExecSecurityDeny
				shell := tools.NewExecTool(execCfg, sessions, nil, nil, nil)
				shellCalls := 0
				shell.SetSkillExecutor(func(context.Context, string, map[string]any) (map[string]string, error) {
					shellCalls++
					return map[string]string{"content": "测试要闻甲；" + strings.Repeat("测试资料证据\n", 1200), "url": "https://www.xinhuanet.com/test-news.html"}, nil
				})
				native := &webSearchToolMock{name: "web_query"}
				h.toolRegistry.Register(shell)
				h.toolRegistry.Register(native)
				body, err := json.Marshal(map[string]interface{}{"message": newsToolRejectionGoal(t), "provider": provider.Name(), "model": "gpt-5.3-codex-spark"})
				if err != nil {
					t.Fatal(err)
				}
				e := echo.New()
				path := "/api/v1/conversations/" + conv.ID + "/messages"
				if stream {
					path += "/stream"
					e.POST("/api/v1/conversations/:id/messages/stream", h.StreamMessage)
				} else {
					e.POST("/api/v1/conversations/:id/messages", h.SendMessage)
				}
				req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				if native.calls != 0 {
					t.Fatal("unavailable native tool was executed")
				}
				first, ok := provider.RequestAt(0)
				if !ok {
					t.Fatal("missing initial model request")
				}
				assertLLMRequestToolNames(t, first.Tools, "exec")
				if scenario.recovers {
					if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), report) || shellCalls != 1 || provider.CallCount() < 3 || provider.CallCount() > 5 {
						t.Fatalf("want evidence-backed report after rejection; status=%d model_calls=%d exec_calls=%d body=%s", rec.Code, provider.CallCount(), shellCalls, rec.Body.String())
					}
					afterExec, ok := provider.RequestAt(2)
					if !ok {
						t.Fatal("missing model continuation after exec")
					}
					evidenceSeen := false
					for _, msg := range afterExec.Messages {
						if msg.Role == llm.RoleTool && strings.Contains(msg.Content, "测试要闻甲") {
							evidenceSeen = true
						}
					}
					if !evidenceSeen {
						t.Fatal("gateway discarded actual source evidence before model continuation")
					}
					msg, err := store.GetLatestAssistantMessage(context.Background(), conv.ID)
					if err != nil || msg == nil || msg.Stats == nil {
						t.Fatalf("missing persisted report usage: message=%v err=%v", msg, err)
					}
					wantInput := 101 + 5*(provider.CallCount()-1)
					wantOutput := 17 + 2*(provider.CallCount()-1)
					if scenario.missingFinalUsage {
						if msg.Stats.InputTokens <= 106 || msg.Stats.OutputTokens <= 19 {
							t.Errorf("missing final usage must retain the final-response estimate plus known rounds: %+v", msg.Stats)
						}
					} else if msg.Stats.InputTokens != wantInput || msg.Stats.OutputTokens != wantOutput || msg.Stats.TotalTokens != wantInput+wantOutput {
						t.Errorf("usage must include rejected round exactly once: got=%+v want_input=%d want_output=%d calls=%d", msg.Stats, wantInput, wantOutput, provider.CallCount())
					}
				} else {
					if shellCalls != 0 || provider.CallCount() != scenario.wantCalls {
						t.Fatalf("rejection must stop after two corrective retries: model_calls=%d exec_calls=%d", provider.CallCount(), shellCalls)
					}
					if !strings.Contains(rec.Body.String(), "unavailable tools") {
						t.Fatalf("persistent rejection silently completed instead of reporting an error: %s", rec.Body.String())
					}
					if !stream && rec.Code < http.StatusBadRequest {
						t.Fatalf("persistent rejection returned success: %d", rec.Code)
					}
				}
				if scenario.maxRounds == 1 {
					return
				}
				second, ok := provider.RequestAt(1)
				if !ok {
					t.Fatal("no corrective retry after rejected tool call")
				}
				assertLLMRequestToolNames(t, second.Tools, "exec")
				feedback := second.Messages[len(second.Messages)-1].Content
				for _, term := range []string{"web_query", "exec", "not executed"} {
					if !strings.Contains(feedback, term) {
						t.Errorf("corrective feedback lacks %q: %s", term, feedback)
					}
				}
			})
		}
	}
}
