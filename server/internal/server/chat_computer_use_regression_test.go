package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type computerUseDesktopChatRecoveryProvider struct {
	name              string
	firstToolArgs     string
	recoveredToolArgs string
	firstContent      string
	recoveredContent  string
	finalContent      string
	fallbackContent   string
	finalFallbackText string
	mu                sync.Mutex
	callCount         int
	requests          []llm.ChatRequest
}

type computerUseDesktopChatMultiRoundRecoveryProvider struct {
	name              string
	firstToolArgs     string
	secondToolArgs    string
	recoveredToolArgs string
	firstContent      string
	secondContent     string
	recoveredContent  string
	finalContent      string
	fallbackContent   string
	finalFallbackText string
	mu                sync.Mutex
	callCount         int
	requests          []llm.ChatRequest
}

type computerUseDesktopChatPostToolFailureProvider struct {
	name          string
	firstToolArgs string
	firstContent  string
	err           error
	mu            sync.Mutex
	callCount     int
	requests      []llm.ChatRequest
}

func (p *computerUseDesktopChatRecoveryProvider) Name() string {
	if strings.TrimSpace(p.name) != "" {
		return strings.TrimSpace(p.name)
	}
	return "scripted-computer-use-recovery"
}

func (p *computerUseDesktopChatRecoveryProvider) Models() []string {
	return []string{"gpt-5.3-codex-spark"}
}

func (p *computerUseDesktopChatRecoveryProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	p.requests = append(p.requests, cloneChatRequestForTest(req))
	round := p.callCount
	p.callCount++
	p.mu.Unlock()

	switch round {
	case 0:
		firstContent := strings.TrimSpace(p.firstContent)
		if firstContent == "" {
			firstContent = "我先检查一下飞书窗口。"
		}
		firstToolArgs := strings.TrimSpace(p.firstToolArgs)
		if firstToolArgs == "" {
			firstToolArgs = `{"action":"ocr","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`
		}
		return &llm.ChatResponse{
			ID:    "computer-use-recovery-round-1",
			Model: req.Model,
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: firstContent,
				ToolCalls: []llm.ToolCall{{
					ID:        "call_computer_use_ocr_1",
					Name:      "computer_use",
					Arguments: firstToolArgs,
				}},
			},
		}, nil
	case 1:
		if requestCarriesDesktopChatComputerUseGuardrails(req) {
			recoveredContent := strings.TrimSpace(p.recoveredContent)
			if recoveredContent == "" {
				recoveredContent = "改用高层消息动作直接发送。"
			}
			recoveredToolArgs := strings.TrimSpace(p.recoveredToolArgs)
			if recoveredToolArgs == "" {
				recoveredToolArgs = `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`
			}
			return &llm.ChatResponse{
				ID:    "computer-use-recovery-round-2",
				Model: req.Model,
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: recoveredContent,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_computer_use_message_1",
						Name:      "computer_use",
						Arguments: recoveredToolArgs,
					}},
				},
			}, nil
		}
		fallbackContent := strings.TrimSpace(p.fallbackContent)
		if fallbackContent == "" {
			fallbackContent = "我继续截图确认界面。"
		}
		return &llm.ChatResponse{
			ID:    "computer-use-recovery-round-2-fallback",
			Model: req.Model,
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: fallbackContent,
				ToolCalls: []llm.ToolCall{{
					ID:        "call_computer_use_screenshot_1",
					Name:      "computer_use",
					Arguments: `{"action":"screenshot","app_name":"Feishu,Lark"}`,
				}},
			},
		}, nil
	default:
		if requestContainsComputerUseToolResult(req, "Host action completed and submitted") {
			finalContent := strings.TrimSpace(p.finalContent)
			if finalContent == "" {
				finalContent = "已经在飞书桌面应用里给【后端之家】发出问候。"
			}
			return &llm.ChatResponse{
				ID:    "computer-use-recovery-round-3",
				Model: req.Model,
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: finalContent,
				},
			}, nil
		}
		finalFallbackText := strings.TrimSpace(p.finalFallbackText)
		if finalFallbackText == "" {
			finalFallbackText = "还停留在截图阶段，没有完成发送。"
		}
		return &llm.ChatResponse{
			ID:    "computer-use-recovery-round-3-fallback",
			Model: req.Model,
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: finalFallbackText,
			},
		}, nil
	}
}

func (p *computerUseDesktopChatRecoveryProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	resp, err := p.Chat(ctx, req)
	if err != nil {
		close(ch)
		return nil, err
	}
	ch <- llm.StreamChunk{
		ID:        resp.ID,
		Model:     resp.Model,
		Delta:     resp.Message.Content,
		Done:      true,
		Usage:     &resp.Usage,
		ToolCalls: resp.Message.ToolCalls,
	}
	close(ch)
	return ch, nil
}

func (p *computerUseDesktopChatRecoveryProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return callback(llm.StreamChunk{
		ID:        resp.ID,
		Model:     resp.Model,
		Delta:     resp.Message.Content,
		Done:      true,
		Usage:     &resp.Usage,
		ToolCalls: resp.Message.ToolCalls,
	})
}

func (p *computerUseDesktopChatRecoveryProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callCount
}

func (p *computerUseDesktopChatRecoveryProvider) RequestAt(idx int) (llm.ChatRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.requests) {
		return llm.ChatRequest{}, false
	}
	return p.requests[idx], true
}

func (p *computerUseDesktopChatMultiRoundRecoveryProvider) Name() string {
	if strings.TrimSpace(p.name) != "" {
		return strings.TrimSpace(p.name)
	}
	return "scripted-computer-use-multi-round-recovery"
}

func (p *computerUseDesktopChatMultiRoundRecoveryProvider) Models() []string {
	return []string{"gpt-5.3-codex-spark"}
}

func (p *computerUseDesktopChatPostToolFailureProvider) Name() string {
	if strings.TrimSpace(p.name) != "" {
		return strings.TrimSpace(p.name)
	}
	return "scripted-computer-use-post-tool-failure"
}

func (p *computerUseDesktopChatPostToolFailureProvider) Models() []string {
	return []string{"gpt-5.3-codex-spark"}
}

func (p *computerUseDesktopChatPostToolFailureProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	p.requests = append(p.requests, cloneChatRequestForTest(req))
	round := p.callCount
	p.callCount++
	p.mu.Unlock()

	if round == 0 {
		firstContent := strings.TrimSpace(p.firstContent)
		if firstContent == "" {
			firstContent = "我先确认一下飞书当前窗口。"
		}
		firstToolArgs := strings.TrimSpace(p.firstToolArgs)
		if firstToolArgs == "" {
			firstToolArgs = `{"action":"screenshot","app_name":"Feishu,Lark"}`
		}
		return &llm.ChatResponse{
			ID:    "computer-use-post-tool-failure-round-1",
			Model: req.Model,
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: firstContent,
				ToolCalls: []llm.ToolCall{{
					ID:        "call_computer_use_screenshot_post_tool_failure",
					Name:      "computer_use",
					Arguments: firstToolArgs,
				}},
			},
		}, nil
	}

	if p.err != nil {
		return nil, p.err
	}
	return nil, fmt.Errorf("proxy returned 401: provider auth error")
}

func (p *computerUseDesktopChatPostToolFailureProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	resp, err := p.Chat(ctx, req)
	if err != nil {
		close(ch)
		return nil, err
	}
	ch <- llm.StreamChunk{
		ID:        resp.ID,
		Model:     resp.Model,
		Delta:     resp.Message.Content,
		Done:      true,
		Usage:     &resp.Usage,
		ToolCalls: resp.Message.ToolCalls,
	}
	close(ch)
	return ch, nil
}

func (p *computerUseDesktopChatPostToolFailureProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return callback(llm.StreamChunk{
		ID:        resp.ID,
		Model:     resp.Model,
		Delta:     resp.Message.Content,
		Done:      true,
		Usage:     &resp.Usage,
		ToolCalls: resp.Message.ToolCalls,
	})
}

func (p *computerUseDesktopChatPostToolFailureProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callCount
}

func (p *computerUseDesktopChatPostToolFailureProvider) RequestAt(idx int) (llm.ChatRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.requests) {
		return llm.ChatRequest{}, false
	}
	return p.requests[idx], true
}

func (p *computerUseDesktopChatMultiRoundRecoveryProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	p.requests = append(p.requests, cloneChatRequestForTest(req))
	round := p.callCount
	p.callCount++
	p.mu.Unlock()

	switch round {
	case 0:
		firstContent := strings.TrimSpace(p.firstContent)
		if firstContent == "" {
			firstContent = "我先看一下当前飞书聊天窗口。"
		}
		firstToolArgs := strings.TrimSpace(p.firstToolArgs)
		if firstToolArgs == "" {
			firstToolArgs = `{"action":"screenshot","app_name":"Feishu,Lark"}`
		}
		return &llm.ChatResponse{
			ID:    "computer-use-multi-round-1",
			Model: req.Model,
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: firstContent,
				ToolCalls: []llm.ToolCall{{
					ID:        "call_computer_use_multi_round_1",
					Name:      "computer_use",
					Arguments: firstToolArgs,
				}},
			},
		}, nil
	case 1:
		if requestCarriesDesktopChatComputerUseGuardrails(req) &&
			requestContainsComputerUseToolResult(req, "Host screenshot captured") {
			secondContent := strings.TrimSpace(p.secondContent)
			if secondContent == "" {
				secondContent = "我先点进目标聊天项，再直接发送。"
			}
			secondToolArgs := strings.TrimSpace(p.secondToolArgs)
			if secondToolArgs == "" {
				secondToolArgs = `{"action":"click","control":"后端之家"}`
			}
			return &llm.ChatResponse{
				ID:    "computer-use-multi-round-2",
				Model: req.Model,
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: secondContent,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_computer_use_multi_round_2",
						Name:      "computer_use",
						Arguments: secondToolArgs,
					}},
				},
			}, nil
		}
	case 2:
		if requestCarriesDesktopChatComputerUseGuardrails(req) &&
			requestContainsComputerUseToolResult(req, "Host screenshot captured") &&
			requestContainsComputerUseToolResult(req, "Host click completed") {
			recoveredContent := strings.TrimSpace(p.recoveredContent)
			if recoveredContent == "" {
				recoveredContent = "聊天项已激活，直接用高层消息动作发送。"
			}
			recoveredToolArgs := strings.TrimSpace(p.recoveredToolArgs)
			if recoveredToolArgs == "" {
				recoveredToolArgs = `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`
			}
			return &llm.ChatResponse{
				ID:    "computer-use-multi-round-3",
				Model: req.Model,
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: recoveredContent,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_computer_use_multi_round_3",
						Name:      "computer_use",
						Arguments: recoveredToolArgs,
					}},
				},
			}, nil
		}
	case 3:
		if requestContainsComputerUseToolResult(req, "Host action completed and submitted") {
			finalContent := strings.TrimSpace(p.finalContent)
			if finalContent == "" {
				finalContent = "已经在飞书桌面应用里给【后端之家】发出问候。"
			}
			return &llm.ChatResponse{
				ID:    "computer-use-multi-round-4",
				Model: req.Model,
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: finalContent,
				},
			}, nil
		}
	}

	fallbackContent := strings.TrimSpace(p.fallbackContent)
	if fallbackContent == "" {
		fallbackContent = "我继续截图确认界面。"
	}
	if round >= 3 {
		finalFallbackText := strings.TrimSpace(p.finalFallbackText)
		if finalFallbackText == "" {
			finalFallbackText = "还停留在截图阶段，没有完成发送。"
		}
		return &llm.ChatResponse{
			ID:    "computer-use-multi-round-fallback-final",
			Model: req.Model,
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: finalFallbackText,
			},
		}, nil
	}
	return &llm.ChatResponse{
		ID:    "computer-use-multi-round-fallback",
		Model: req.Model,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: fallbackContent,
			ToolCalls: []llm.ToolCall{{
				ID:        "call_computer_use_screenshot_fallback",
				Name:      "computer_use",
				Arguments: `{"action":"screenshot","app_name":"Feishu,Lark"}`,
			}},
		},
	}, nil
}

func (p *computerUseDesktopChatMultiRoundRecoveryProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	resp, err := p.Chat(ctx, req)
	if err != nil {
		close(ch)
		return nil, err
	}
	ch <- llm.StreamChunk{
		ID:        resp.ID,
		Model:     resp.Model,
		Delta:     resp.Message.Content,
		Done:      true,
		Usage:     &resp.Usage,
		ToolCalls: resp.Message.ToolCalls,
	}
	close(ch)
	return ch, nil
}

func (p *computerUseDesktopChatMultiRoundRecoveryProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return callback(llm.StreamChunk{
		ID:        resp.ID,
		Model:     resp.Model,
		Delta:     resp.Message.Content,
		Done:      true,
		Usage:     &resp.Usage,
		ToolCalls: resp.Message.ToolCalls,
	})
}

func (p *computerUseDesktopChatMultiRoundRecoveryProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callCount
}

func (p *computerUseDesktopChatMultiRoundRecoveryProvider) RequestAt(idx int) (llm.ChatRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.requests) {
		return llm.ChatRequest{}, false
	}
	return p.requests[idx], true
}

type computerUseScenarioToolMock struct {
	mu    sync.Mutex
	calls []map[string]interface{}
}

func (m *computerUseScenarioToolMock) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "computer_use",
		Description: "Inspect and interact with host UI through the computer-use tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action":       map[string]interface{}{"type": "string"},
				"app_name":     map[string]interface{}{"type": "string"},
				"conversation": map[string]interface{}{"type": "string"},
				"value":        map[string]interface{}{"type": "string"},
				"submit":       map[string]interface{}{"type": "boolean"},
			},
			"additionalProperties": true,
		},
	}
}

func (m *computerUseScenarioToolMock) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	cloned := cloneComputerUseArgsForTest(args)

	m.mu.Lock()
	m.calls = append(m.calls, cloned)
	m.mu.Unlock()

	action := strings.TrimSpace(anyToStringForLLM(cloned["action"]))
	switch action {
	case "ocr":
		return map[string]interface{}{
			"title":      "computer_use",
			"type":       "result",
			"status":     "error",
			"error_code": "unsupported_action",
			"action":     action,
			"message":    "invalid action: ocr",
		}, nil
	case "message":
		return map[string]interface{}{
			"title":        "computer_use",
			"type":         "result",
			"status":       "success",
			"action":       action,
			"conversation": anyToStringForLLM(cloned["conversation"]),
			"value":        anyToStringForLLM(cloned["value"]),
			"message":      "Host action completed and submitted",
		}, nil
	case "focus":
		return map[string]interface{}{
			"title":    "computer_use",
			"type":     "result",
			"status":   "success",
			"action":   action,
			"app_name": anyToStringForLLM(cloned["app_name"]),
			"message":  "Host window focused",
		}, nil
	case "select":
		return map[string]interface{}{
			"title":        "computer_use",
			"type":         "result",
			"status":       "success",
			"action":       action,
			"conversation": anyToStringForLLM(cloned["conversation"]),
			"message":      "Host selection completed",
		}, nil
	case "click":
		return map[string]interface{}{
			"title":   "computer_use",
			"type":    "result",
			"status":  "success",
			"action":  action,
			"message": "Host click completed",
		}, nil
	case "key":
		return map[string]interface{}{
			"title":   "computer_use",
			"type":    "result",
			"status":  "success",
			"action":  action,
			"message": "Host key input completed",
		}, nil
	case "screenshot":
		return map[string]interface{}{
			"title":   "computer_use",
			"type":    "result",
			"status":  "success",
			"action":  action,
			"message": "Host screenshot captured",
		}, nil
	case "snapshot_interactive":
		return map[string]interface{}{
			"title":   "computer_use",
			"type":    "result",
			"status":  "success",
			"action":  action,
			"message": "Host interactive snapshot captured",
		}, nil
	default:
		return map[string]interface{}{
			"title":   "computer_use",
			"type":    "result",
			"status":  "error",
			"action":  action,
			"message": "unexpected action: " + action,
		}, nil
	}
}

func (m *computerUseScenarioToolMock) Calls() []map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]map[string]interface{}, 0, len(m.calls))
	for _, call := range m.calls {
		out = append(out, cloneComputerUseArgsForTest(call))
	}
	return out
}

func cloneComputerUseArgsForTest(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return map[string]interface{}{}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return map[string]interface{}{}
	}
	var cloned map[string]interface{}
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return map[string]interface{}{}
	}
	return cloned
}

func requestCarriesDesktopChatComputerUseGuardrails(req llm.ChatRequest) bool {
	for _, msg := range req.Messages {
		if (strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks")) &&
			strings.Contains(msg.Content, "`message`, `select`, and `type`") &&
			strings.Contains(msg.Content, "`activate`, `mouse_click`, `move_to`, `accessibility_tree`, or `ocr`") {
			return true
		}
	}
	return false
}

func TestShouldInjectComputerUseDesktopChatContinuationNudge_DetectsMessageIntentWithoutConversationSelector(t *testing.T) {
	if !shouldInjectComputerUseDesktopChatContinuationNudge("resp_prev_1", []llm.ToolCall{{
		Name:      "computer_use",
		Arguments: `{"action":"ocr","intent":"message","app_name":"Feishu,Lark","value":"你好，我是 Blue。","submit":true}`,
	}}) {
		t.Fatal("expected message-intent computer_use continuation to trigger desktop-chat nudge even without conversation selector")
	}
}

func TestShouldInjectComputerUseDesktopChatContinuationNudgeForRequest_DetectsDesktopChatRoutingMessageAfterGenericFocus(t *testing.T) {
	if !shouldInjectComputerUseDesktopChatContinuationNudgeForRequest(
		"resp_prev_1",
		"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
		[]llm.ToolCall{{
			Name:      "computer_use",
			Arguments: `{"action":"focus","app_name":"Feishu,Lark"}`,
		}},
	) {
		t.Fatal("expected desktop-chat routing message to trigger continuation nudge even when the first computer_use action is only a generic focus")
	}
}

func TestShouldInjectComputerUseDesktopChatContinuationNudgeForRequest_DetectsDesktopChatRoutingMessageAfterBareFocus(t *testing.T) {
	if !shouldInjectComputerUseDesktopChatContinuationNudgeForRequest(
		"resp_prev_1",
		"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
		[]llm.ToolCall{{
			Name:      "computer_use",
			Arguments: `{"action":"focus"}`,
		}},
	) {
		t.Fatal("expected desktop-chat routing message to trigger continuation nudge even when the first computer_use action is a bare generic focus")
	}
}

func TestShouldInjectComputerUseDesktopChatContinuationNudgeForRequest_DetectsDesktopChatRoutingMessageAfterGenericSelect(t *testing.T) {
	if !shouldInjectComputerUseDesktopChatContinuationNudgeForRequest(
		"resp_prev_1",
		"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
		[]llm.ToolCall{{
			Name:      "computer_use",
			Arguments: `{"action":"select","conversation":"后端之家"}`,
		}},
	) {
		t.Fatal("expected desktop-chat routing message to trigger continuation nudge even when the first computer_use action is a generic select")
	}
}

func TestShouldInjectComputerUseDesktopChatContinuationNudgeForRequest_DetectsDesktopChatRoutingMessageAfterGenericClick(t *testing.T) {
	if !shouldInjectComputerUseDesktopChatContinuationNudgeForRequest(
		"resp_prev_1",
		"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
		[]llm.ToolCall{{
			Name:      "computer_use",
			Arguments: `{"action":"click","control":"后端之家"}`,
		}},
	) {
		t.Fatal("expected desktop-chat routing message to trigger continuation nudge even when the first computer_use action is a generic click")
	}
}

func TestShouldInjectComputerUseDesktopChatContinuationNudgeForRequest_DetectsDesktopChatRoutingMessageAfterGenericKey(t *testing.T) {
	if !shouldInjectComputerUseDesktopChatContinuationNudgeForRequest(
		"resp_prev_1",
		"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
		[]llm.ToolCall{{
			Name:      "computer_use",
			Arguments: `{"action":"key","keys":["tab"]}`,
		}},
	) {
		t.Fatal("expected desktop-chat routing message to trigger continuation nudge even when the first computer_use action is a generic key input")
	}
}

func TestShouldInjectComputerUseDesktopChatContinuationNudgeForRequest_DetectsDesktopChatRoutingMessageAfterScreenshot(t *testing.T) {
	if !shouldInjectComputerUseDesktopChatContinuationNudgeForRequest(
		"resp_prev_1",
		"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
		[]llm.ToolCall{{
			Name:      "computer_use",
			Arguments: `{"action":"screenshot","app_name":"Feishu,Lark"}`,
		}},
	) {
		t.Fatal("expected desktop-chat routing message to trigger continuation nudge even when the first computer_use action is a screenshot")
	}
}

func TestShouldInjectComputerUseDesktopChatContinuationNudgeForRequest_DetectsDesktopChatRoutingMessageAfterSnapshotInteractive(t *testing.T) {
	if !shouldInjectComputerUseDesktopChatContinuationNudgeForRequest(
		"resp_prev_1",
		"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
		[]llm.ToolCall{{
			Name:      "computer_use",
			Arguments: `{"action":"snapshot_interactive","app_name":"Feishu,Lark"}`,
		}},
	) {
		t.Fatal("expected desktop-chat routing message to trigger continuation nudge even when the first computer_use action is an interactive snapshot")
	}
}

func TestBuildToolFallbackTextWithOptions_DesktopChatPendingDoesNotClaimCompletion(t *testing.T) {
	fallback, toolCount := buildToolFallbackTextWithOptions([]llm.Message{
		{
			Role:    llm.RoleUser,
			Content: "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
		},
		{
			Role:     llm.RoleTool,
			ToolName: "computer_use",
			Content:  `{"title":"computer_use","type":"result","status":"success","action":"screenshot","message":"Host screenshot captured"}`,
		},
	}, 4096, toolFallbackTextOptions{toolCardsVisible: true})
	if toolCount != 1 {
		t.Fatalf("toolCount = %d, want 1", toolCount)
	}
	if strings.Contains(fallback, "我已完成这些工具步骤") {
		t.Fatalf("expected desktop-chat pending fallback to avoid completion wording, got %q", fallback)
	}
	if !strings.Contains(fallback, "还没有确认发送成功") {
		t.Fatalf("expected desktop-chat pending fallback to state incomplete send, got %q", fallback)
	}
}

func newDesktopChatFocusFirstRecoveryProvider(name string) *computerUseDesktopChatRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-focus-first-recovery"
	}
	return &computerUseDesktopChatRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"focus","app_name":"Feishu,Lark"}`,
		recoveredToolArgs: `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先聚焦飞书桌面应用。",
		recoveredContent:  "飞书窗口已就绪，改用高层消息动作直接发送。",
		finalContent:      "已经在飞书桌面应用里给【后端之家】发出问候。",
	}
}

func newDesktopChatBareFocusRecoveryProvider(name string) *computerUseDesktopChatRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-bare-focus-recovery"
	}
	return &computerUseDesktopChatRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"focus"}`,
		recoveredToolArgs: `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先把聊天应用切到前台。",
		recoveredContent:  "现在直接用高层消息动作发送。",
		finalContent:      "已经在飞书桌面应用里给【后端之家】发出问候。",
	}
}

func newDesktopChatSelectFirstRecoveryProvider(name string) *computerUseDesktopChatRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-select-first-recovery"
	}
	return &computerUseDesktopChatRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"select","conversation":"后端之家"}`,
		recoveredToolArgs: `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先切到目标聊天会话。",
		recoveredContent:  "会话已定位，直接用高层消息动作发送。",
		finalContent:      "已经在飞书桌面应用里给【后端之家】发出问候。",
	}
}

func newDesktopChatClickFirstRecoveryProvider(name string) *computerUseDesktopChatRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-click-first-recovery"
	}
	return &computerUseDesktopChatRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"click","control":"后端之家"}`,
		recoveredToolArgs: `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先点进目标聊天项。",
		recoveredContent:  "聊天项已激活，直接用高层消息动作发送。",
		finalContent:      "已经在飞书桌面应用里给【后端之家】发出问候。",
	}
}

func newDesktopChatKeyFirstRecoveryProvider(name string) *computerUseDesktopChatRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-key-first-recovery"
	}
	return &computerUseDesktopChatRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"key","keys":["tab"]}`,
		recoveredToolArgs: `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先用键盘把焦点切到目标聊天区域。",
		recoveredContent:  "焦点已就绪，直接用高层消息动作发送。",
		finalContent:      "已经在飞书桌面应用里给【后端之家】发出问候。",
	}
}

func newDesktopChatScreenshotFirstRecoveryProvider(name string) *computerUseDesktopChatRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-screenshot-first-recovery"
	}
	return &computerUseDesktopChatRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"screenshot","app_name":"Feishu,Lark"}`,
		recoveredToolArgs: `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先截一张图确认当前聊天窗口。",
		recoveredContent:  "界面已经确认，改用高层消息动作直接发送。",
		finalContent:      "已经在飞书桌面应用里给【后端之家】发出问候。",
	}
}

func newDesktopChatSnapshotInteractiveFirstRecoveryProvider(name string) *computerUseDesktopChatRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-snapshot-interactive-first-recovery"
	}
	return &computerUseDesktopChatRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"snapshot_interactive","app_name":"Feishu,Lark"}`,
		recoveredToolArgs: `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先抓取可交互元素，确认当前聊天窗口。",
		recoveredContent:  "结构已经确认，改用高层消息动作直接发送。",
		finalContent:      "已经在飞书桌面应用里给【后端之家】发出问候。",
	}
}

func newDesktopChatScreenshotThenClickRecoveryProvider(name string) *computerUseDesktopChatMultiRoundRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-screenshot-then-click-recovery"
	}
	return &computerUseDesktopChatMultiRoundRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"screenshot","app_name":"Feishu,Lark"}`,
		secondToolArgs:    `{"action":"click","control":"后端之家"}`,
		recoveredToolArgs: `{"action":"message","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先看一下当前飞书聊天窗口。",
		secondContent:     "我先点进目标聊天项，再直接发送。",
		recoveredContent:  "聊天项已激活，直接用高层消息动作发送。",
		finalContent:      "已经在飞书桌面应用里给【后端之家】发出问候。",
	}
}

func newCurrentComposerComputerUseRecoveryProvider(name string) *computerUseDesktopChatRecoveryProvider {
	if strings.TrimSpace(name) == "" {
		name = "scripted-computer-use-message-intent-recovery"
	}
	return &computerUseDesktopChatRecoveryProvider{
		name:              name,
		firstToolArgs:     `{"action":"ocr","intent":"message","app_name":"Feishu,Lark","value":"你好，我是 Blue。","submit":true}`,
		recoveredToolArgs: `{"action":"message","intent":"message","app_name":"Feishu,Lark","value":"你好，我是 Blue。","submit":true}`,
		firstContent:      "我先尝试在当前聊天框里发送。",
		recoveredContent:  "改用高层消息动作直接在当前聊天框发送。",
		finalContent:      "已经在当前飞书聊天里发出问候。",
	}
}

func TestChatHandlerSendMessage_ComputerUseMessageIntentWithoutConversationSelectorRecoversFromInvalidOCRToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use current composer recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newCurrentComposerComputerUseRecoveryProvider("scripted-computer-use-message-intent-recovery")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我直接在当前飞书聊天框里回复一句：你好，我是Blue。你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-message-intent-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bad tool call + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "ocr" {
		t.Fatalf("first computer_use action = %q, want ocr", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	if got := anyToStringForLLM(calls[1]["conversation"]); got != "" {
		t.Fatalf("second computer_use conversation = %q, want empty for current-composer reply path", got)
	}
	if got := anyToStringForLLM(calls[1]["value"]); got != "你好，我是 Blue。" {
		t.Fatalf("second computer_use value = %q, want 你好，我是 Blue。", got)
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second request to carry desktop chat guardrails, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(strings.TrimSpace(content), "已经在当前飞书聊天里发出问候。") {
		t.Fatalf("expected final current-composer completion summary, got %q", content)
	}
	if strings.Contains(content, "Host screenshot captured") || strings.Contains(content, "invalid action") {
		t.Fatalf("expected final response to avoid screenshot/error drift details, got %q", content)
	}
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRequestRecoversFromGenericFocusToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use generic focus desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatFocusFirstRecoveryProvider("scripted-computer-use-focus-first-recovery")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-focus-first-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic focus + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "focus" {
		t.Fatalf("first computer_use action = %q, want focus", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host window focused") {
		t.Fatalf("expected second request to include successful focus result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second request to carry desktop chat guardrails after generic focus, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(strings.TrimSpace(content), "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
	if strings.Contains(content, "Host screenshot captured") || strings.Contains(content, "invalid action") {
		t.Fatalf("expected final response to avoid screenshot/error drift details, got %q", content)
	}
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRequestRecoversFromBareFocusToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use bare focus desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatBareFocusRecoveryProvider("scripted-computer-use-bare-focus-recovery")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-bare-focus-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bare focus + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "focus" {
		t.Fatalf("first computer_use action = %q, want focus", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host window focused") {
		t.Fatalf("expected second request to include successful bare focus result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second request to carry desktop chat guardrails after bare focus, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(strings.TrimSpace(content), "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
	if strings.Contains(content, "Host screenshot captured") || strings.Contains(content, "invalid action") {
		t.Fatalf("expected final response to avoid screenshot/error drift details, got %q", content)
	}
}

func TestStreamMessage_ComputerUseDesktopChatRequestRecoversFromBareFocusToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use bare focus desktop chat recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatBareFocusRecoveryProvider("scripted-computer-use-bare-focus-recovery-stream")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-bare-focus-recovery-stream","model":"gpt-5.3-codex-spark"}`)

	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", body)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bare focus + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "focus" {
		t.Fatalf("first computer_use action = %q, want focus", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host window focused") {
		t.Fatalf("expected second stream request to include successful bare focus result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second stream request to carry desktop chat guardrails after bare focus, got %#v", secondReq.Messages)
	}

	if !strings.Contains(body, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
	if strings.Contains(body, "Host screenshot captured") || strings.Contains(body, `"action":"screenshot"`) {
		t.Fatalf("expected streamed response to avoid screenshot fallback drift, got body=%s", body)
	}
}

func TestProcessChannelMessage_ComputerUseDesktopChatRequestRecoversFromBareFocusToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatBareFocusRecoveryProvider("scripted-computer-use-bare-focus-recovery-im")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_bare_focus_computer_use_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bare focus + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "focus" {
		t.Fatalf("first computer_use action = %q, want focus", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host window focused") {
		t.Fatalf("expected second IM request to include successful bare focus result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second IM request to carry desktop chat guardrails after bare focus, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseDesktopChatRequestRecoversFromBareFocusToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatBareFocusRecoveryProvider("scripted-computer-use-bare-focus-recovery-im-resume")
	provider.callCount = 1
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_bare_focus_computer_use")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_focus_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"focus"}`,
	}
	routingMessage := "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_bare_focus_computer_use",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先把聊天应用切到前台。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_bare_focus_computer_use",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "focus" {
		t.Fatalf("first computer_use action = %q, want focus", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		firstReq, _ := provider.RequestAt(0)
		t.Fatalf("second computer_use action = %q, want message; first resumed request=%s", got, summarizeChatRequestMessagesForDebug(firstReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	firstReq, ok := provider.RequestAt(0)
	if !ok {
		t.Fatalf("missing first resumed request capture")
	}
	if !requestContainsComputerUseToolResult(firstReq, "Host window focused") {
		t.Fatalf("expected resumed IM request to include successful bare focus result, got %#v", firstReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range firstReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected resumed IM request to carry desktop chat guardrails after bare focus, got %#v", firstReq.Messages)
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second resumed request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host action completed and submitted") {
		t.Fatalf("expected second resumed request to include successful computer_use message result, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected resumed IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRequestRecoversFromGenericSelectToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use generic select desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatSelectFirstRecoveryProvider("scripted-computer-use-select-first-recovery")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-select-first-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic select + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "select" {
		t.Fatalf("first computer_use action = %q, want select", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host selection completed") {
		t.Fatalf("expected second request to include successful generic select result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second request to carry desktop chat guardrails after generic select, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(strings.TrimSpace(content), "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
	if strings.Contains(content, "Host screenshot captured") || strings.Contains(content, "invalid action") {
		t.Fatalf("expected final response to avoid screenshot/error drift details, got %q", content)
	}
}

func TestStreamMessage_ComputerUseDesktopChatRequestRecoversFromGenericSelectToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use generic select desktop chat recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatSelectFirstRecoveryProvider("scripted-computer-use-select-first-recovery-stream")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-select-first-recovery-stream","model":"gpt-5.3-codex-spark"}`)

	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", body)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic select + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "select" {
		t.Fatalf("first computer_use action = %q, want select", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host selection completed") {
		t.Fatalf("expected second stream request to include successful generic select result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second stream request to carry desktop chat guardrails after generic select, got %#v", secondReq.Messages)
	}

	if !strings.Contains(body, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
	if strings.Contains(body, "Host screenshot captured") || strings.Contains(body, `"action":"screenshot"`) {
		t.Fatalf("expected streamed response to avoid screenshot fallback drift, got body=%s", body)
	}
}

func TestProcessChannelMessage_ComputerUseDesktopChatRequestRecoversFromGenericSelectToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatSelectFirstRecoveryProvider("scripted-computer-use-select-first-recovery-im")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_generic_select_computer_use_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic select + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "select" {
		t.Fatalf("first computer_use action = %q, want select", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host selection completed") {
		t.Fatalf("expected second IM request to include successful generic select result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second IM request to carry desktop chat guardrails after generic select, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseDesktopChatRequestRecoversFromGenericSelectToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatSelectFirstRecoveryProvider("scripted-computer-use-select-first-recovery-im-resume")
	provider.callCount = 1
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_generic_select_computer_use")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_select_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"select","conversation":"后端之家"}`,
	}
	routingMessage := "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_generic_select_computer_use",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先切到目标聊天会话。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_generic_select_computer_use",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "select" {
		t.Fatalf("first computer_use action = %q, want select", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		firstReq, _ := provider.RequestAt(0)
		t.Fatalf("second computer_use action = %q, want message; first resumed request=%s", got, summarizeChatRequestMessagesForDebug(firstReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	firstReq, ok := provider.RequestAt(0)
	if !ok {
		t.Fatalf("missing first resumed request capture")
	}
	if !requestContainsComputerUseToolResult(firstReq, "Host selection completed") {
		t.Fatalf("expected resumed IM request to include successful generic select result, got %#v", firstReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range firstReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected resumed IM request to carry desktop chat guardrails after generic select, got %#v", firstReq.Messages)
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second resumed request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host action completed and submitted") {
		t.Fatalf("expected second resumed request to include successful computer_use message result, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected resumed IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRequestRecoversFromGenericClickToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use generic click desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatClickFirstRecoveryProvider("scripted-computer-use-click-first-recovery")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-click-first-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic click + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "click" {
		t.Fatalf("first computer_use action = %q, want click", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host click completed") {
		t.Fatalf("expected second request to include successful generic click result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second request to carry desktop chat guardrails after generic click, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(strings.TrimSpace(content), "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
	if strings.Contains(content, "Host screenshot captured") || strings.Contains(content, "invalid action") {
		t.Fatalf("expected final response to avoid screenshot/error drift details, got %q", content)
	}
}

func TestStreamMessage_ComputerUseDesktopChatRequestRecoversFromGenericClickToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use generic click desktop chat recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatClickFirstRecoveryProvider("scripted-computer-use-click-first-recovery-stream")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-click-first-recovery-stream","model":"gpt-5.3-codex-spark"}`)

	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", body)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic click + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "click" {
		t.Fatalf("first computer_use action = %q, want click", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host click completed") {
		t.Fatalf("expected second stream request to include successful generic click result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second stream request to carry desktop chat guardrails after generic click, got %#v", secondReq.Messages)
	}

	if !strings.Contains(body, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
	if strings.Contains(body, "Host screenshot captured") || strings.Contains(body, `"action":"screenshot"`) {
		t.Fatalf("expected streamed response to avoid screenshot fallback drift, got body=%s", body)
	}
}

func TestProcessChannelMessage_ComputerUseDesktopChatRequestRecoversFromGenericClickToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatClickFirstRecoveryProvider("scripted-computer-use-click-first-recovery-im")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_generic_click_computer_use_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic click + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "click" {
		t.Fatalf("first computer_use action = %q, want click", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host click completed") {
		t.Fatalf("expected second IM request to include successful generic click result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second IM request to carry desktop chat guardrails after generic click, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseDesktopChatRequestRecoversFromGenericClickToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatClickFirstRecoveryProvider("scripted-computer-use-click-first-recovery-im-resume")
	provider.callCount = 1
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_generic_click_computer_use")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_click_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"click","control":"后端之家"}`,
	}
	routingMessage := "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_generic_click_computer_use",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先点进目标聊天项。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_generic_click_computer_use",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "click" {
		t.Fatalf("first computer_use action = %q, want click", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		firstReq, _ := provider.RequestAt(0)
		t.Fatalf("second computer_use action = %q, want message; first resumed request=%s", got, summarizeChatRequestMessagesForDebug(firstReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	firstReq, ok := provider.RequestAt(0)
	if !ok {
		t.Fatalf("missing first resumed request capture")
	}
	if !requestContainsComputerUseToolResult(firstReq, "Host click completed") {
		t.Fatalf("expected resumed IM request to include successful generic click result, got %#v", firstReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range firstReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected resumed IM request to carry desktop chat guardrails after generic click, got %#v", firstReq.Messages)
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second resumed request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host action completed and submitted") {
		t.Fatalf("expected second resumed request to include successful computer_use message result, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected resumed IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRequestRecoversFromGenericKeyToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use generic key desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatKeyFirstRecoveryProvider("scripted-computer-use-key-first-recovery")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-key-first-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic key + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "key" {
		t.Fatalf("first computer_use action = %q, want key", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host key input completed") {
		t.Fatalf("expected second request to include successful generic key result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second request to carry desktop chat guardrails after generic key, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(strings.TrimSpace(content), "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
	if strings.Contains(content, "Host screenshot captured") || strings.Contains(content, "invalid action") {
		t.Fatalf("expected final response to avoid screenshot/error drift details, got %q", content)
	}
}

func TestStreamMessage_ComputerUseDesktopChatRequestRecoversFromGenericKeyToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use generic key desktop chat recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatKeyFirstRecoveryProvider("scripted-computer-use-key-first-recovery-stream")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-key-first-recovery-stream","model":"gpt-5.3-codex-spark"}`)

	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", body)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic key + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "key" {
		t.Fatalf("first computer_use action = %q, want key", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host key input completed") {
		t.Fatalf("expected second stream request to include successful generic key result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second stream request to carry desktop chat guardrails after generic key, got %#v", secondReq.Messages)
	}

	if !strings.Contains(body, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
	if strings.Contains(body, "Host screenshot captured") || strings.Contains(body, `"action":"screenshot"`) {
		t.Fatalf("expected streamed response to avoid screenshot fallback drift, got body=%s", body)
	}
}

func TestProcessChannelMessage_ComputerUseDesktopChatRequestRecoversFromGenericKeyToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatKeyFirstRecoveryProvider("scripted-computer-use-key-first-recovery-im")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_generic_key_computer_use_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (generic key + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "key" {
		t.Fatalf("first computer_use action = %q, want key", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host key input completed") {
		t.Fatalf("expected second IM request to include successful generic key result, got %#v", secondReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second IM request to carry desktop chat guardrails after generic key, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseDesktopChatRequestRecoversFromGenericKeyToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatKeyFirstRecoveryProvider("scripted-computer-use-key-first-recovery-im-resume")
	provider.callCount = 1
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_generic_key_computer_use")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_key_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"key","keys":["tab"]}`,
	}
	routingMessage := "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_generic_key_computer_use",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先用键盘把焦点切到目标聊天区域。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_generic_key_computer_use",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "key" {
		t.Fatalf("first computer_use action = %q, want key", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		firstReq, _ := provider.RequestAt(0)
		t.Fatalf("second computer_use action = %q, want message; first resumed request=%s", got, summarizeChatRequestMessagesForDebug(firstReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	firstReq, ok := provider.RequestAt(0)
	if !ok {
		t.Fatalf("missing first resumed request capture")
	}
	if !requestContainsComputerUseToolResult(firstReq, "Host key input completed") {
		t.Fatalf("expected resumed IM request to include successful generic key result, got %#v", firstReq.Messages)
	}
	var sawGuardrails bool
	for _, msg := range firstReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected resumed IM request to carry desktop chat guardrails after generic key, got %#v", firstReq.Messages)
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second resumed request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host action completed and submitted") {
		t.Fatalf("expected second resumed request to include successful computer_use message result, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected resumed IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestStreamMessage_ComputerUseMessageIntentWithoutConversationSelectorRecoversFromInvalidOCRToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use current composer recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newCurrentComposerComputerUseRecoveryProvider("scripted-computer-use-message-intent-recovery-stream")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我直接在当前飞书聊天框里回复一句：你好，我是Blue。你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-message-intent-recovery-stream","model":"gpt-5.3-codex-spark"}`)

	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", body)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bad tool call + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "ocr" {
		t.Fatalf("first computer_use action = %q, want ocr", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	if got := anyToStringForLLM(calls[1]["conversation"]); got != "" {
		t.Fatalf("second computer_use conversation = %q, want empty for current-composer reply path", got)
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second stream request to carry desktop chat guardrails, got %#v", secondReq.Messages)
	}

	if !strings.Contains(body, "已经在当前飞书聊天里发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
	if strings.Contains(body, "Host screenshot captured") || strings.Contains(body, `"action":"screenshot"`) {
		t.Fatalf("expected streamed response to avoid screenshot fallback drift, got body=%s", body)
	}
}

func TestProcessChannelMessage_ComputerUseMessageIntentWithoutConversationSelectorRecoversFromInvalidOCRToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newCurrentComposerComputerUseRecoveryProvider("scripted-computer-use-message-intent-recovery-im")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_current_composer_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我直接在当前飞书聊天框里回复一句：你好，我是Blue。你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bad tool call + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "ocr" {
		t.Fatalf("first computer_use action = %q, want ocr", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	if got := anyToStringForLLM(calls[1]["conversation"]); got != "" {
		t.Fatalf("second computer_use conversation = %q, want empty for current-composer reply path", got)
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second IM request to carry desktop chat guardrails, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在当前飞书聊天里发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseMessageIntentWithoutConversationSelectorRecoversFromInvalidOCRToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newCurrentComposerComputerUseRecoveryProvider("scripted-computer-use-message-intent-recovery-im-resume")
	provider.callCount = 1
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_current_composer")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_ocr_current_composer_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"ocr","intent":"message","app_name":"Feishu,Lark","value":"你好，我是 Blue。","submit":true}`,
	}
	routingMessage := "帮我直接在当前飞书聊天框里回复一句：你好，我是Blue。你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_current_composer",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先尝试在当前聊天框里发送。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_current_composer",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "ocr" {
		t.Fatalf("first computer_use action = %q, want ocr", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		firstReq, _ := provider.RequestAt(0)
		t.Fatalf("second computer_use action = %q, want message; first resumed request=%s", got, summarizeChatRequestMessagesForDebug(firstReq))
	}
	if got := anyToStringForLLM(calls[1]["conversation"]); got != "" {
		t.Fatalf("second computer_use conversation = %q, want empty for current-composer reply path", got)
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	firstReq, ok := provider.RequestAt(0)
	if !ok {
		t.Fatalf("missing first resumed request capture")
	}
	var sawGuardrails bool
	for _, msg := range firstReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected resumed IM request to carry desktop chat guardrails, got %#v", firstReq.Messages)
	}

	if !strings.Contains(resp, "已经在当前飞书聊天里发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected resumed IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func requestContainsComputerUseToolResult(req llm.ChatRequest, snippet string) bool {
	for _, msg := range req.Messages {
		if msg.Role != llm.RoleTool || strings.TrimSpace(msg.ToolName) != "computer_use" {
			continue
		}
		if strings.Contains(msg.Content, snippet) {
			return true
		}
	}
	return false
}

func summarizeChatRequestMessagesForDebug(req llm.ChatRequest) string {
	parts := make([]string, 0, len(req.Messages))
	for _, msg := range req.Messages {
		content := strings.ReplaceAll(strings.TrimSpace(msg.Content), "\n", " ")
		if len(content) > 180 {
			content = content[:180]
		}
		if msg.ToolName != "" {
			parts = append(parts, string(msg.Role)+":"+msg.ToolName+":"+content)
			continue
		}
		parts = append(parts, string(msg.Role)+":"+content)
	}
	return strings.Join(parts, " || ")
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRecoversFromInvalidOCRToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := &computerUseDesktopChatRecoveryProvider{}
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue。","provider":"scripted-computer-use-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bad tool call + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "ocr" {
		t.Fatalf("first computer_use action = %q, want ocr", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	if got := anyToStringForLLM(calls[1]["conversation"]); got != "后端之家" {
		t.Fatalf("second computer_use conversation = %q, want 后端之家", got)
	}
	if got := anyToStringForLLM(calls[1]["value"]); got != "你好，我是 Blue。" {
		t.Fatalf("second computer_use value = %q, want 你好，我是 Blue。", got)
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	toolMsg, payload := requireToolPayloadMessage(t, secondReq, "computer_use")
	if len(toolMsg.Content) == 0 {
		t.Fatal("expected non-empty computer_use tool payload in follow-up request")
	}
	if got := anyToStringForLLM(payload["status"]); got != "error" {
		t.Fatalf("payload status = %q, want error; content=%q payload=%#v", got, toolMsg.Content, payload)
	}
	if got := anyToStringForLLM(payload["message"]); got != "invalid action: ocr" {
		t.Fatalf("payload message = %q, want invalid action: ocr; content=%q payload=%#v", got, toolMsg.Content, payload)
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
			if !strings.Contains(msg.Content, "`message`, `select`, and `type`") {
				t.Fatalf("expected second request guardrails to prefer high-level computer_use actions, got %q", msg.Content)
			}
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second request to carry desktop chat guardrails, got %#v", secondReq.Messages)
	}

	thirdReq, ok := provider.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	if !requestContainsComputerUseToolResult(thirdReq, "Host action completed and submitted") {
		t.Fatalf("expected third request to include successful computer_use message result, got %#v", thirdReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(strings.TrimSpace(content), "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
	if strings.Contains(content, "Host screenshot captured") || strings.Contains(content, "invalid action") {
		t.Fatalf("expected final response to avoid screenshot/error drift details, got %q", content)
	}
}

func TestStreamMessage_ComputerUseDesktopChatRecoversFromInvalidOCRToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use desktop chat recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := &computerUseDesktopChatRecoveryProvider{}
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-recovery","model":"gpt-5.3-codex-spark"}`)

	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", body)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bad tool call + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "ocr" {
		t.Fatalf("first computer_use action = %q, want ocr", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
			if !strings.Contains(msg.Content, "`message`, `select`, and `type`") {
				t.Fatalf("expected second stream request guardrails to prefer high-level computer_use actions, got %q", msg.Content)
			}
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second stream request to carry desktop chat guardrails, got %#v", secondReq.Messages)
	}

	thirdReq, ok := provider.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	if !requestContainsComputerUseToolResult(thirdReq, "Host action completed and submitted") {
		t.Fatalf("expected third request to include successful computer_use message result, got %#v", thirdReq.Messages)
	}

	if !strings.Contains(body, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
	if strings.Contains(body, "Host screenshot captured") || strings.Contains(body, `"action":"screenshot"`) {
		t.Fatalf("expected streamed response to avoid screenshot fallback drift, got body=%s", body)
	}
}

func TestProcessChannelMessage_ComputerUseDesktopChatRecoversFromInvalidOCRToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := &computerUseDesktopChatRecoveryProvider{}
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_computer_use_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if provider.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (bad tool call + recovered message call + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "ocr" {
		t.Fatalf("first computer_use action = %q, want ocr", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawGuardrails bool
	for _, msg := range secondReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
			if !strings.Contains(msg.Content, "`message`, `select`, and `type`") {
				t.Fatalf("expected second IM request guardrails to prefer high-level computer_use actions, got %q", msg.Content)
			}
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected second IM request to carry desktop chat guardrails, got %#v", secondReq.Messages)
	}

	thirdReq, ok := provider.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	if !requestContainsComputerUseToolResult(thirdReq, "Host action completed and submitted") {
		t.Fatalf("expected third request to include successful computer_use message result, got %#v", thirdReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseDesktopChatRecoversFromInvalidOCRToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := &computerUseDesktopChatRecoveryProvider{callCount: 1}
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_computer_use")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_ocr_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"ocr","app_name":"Feishu,Lark","conversation":"后端之家","value":"你好，我是 Blue。","submit":true}`,
	}
	routingMessage := "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_computer_use",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先检查一下飞书窗口。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_computer_use",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "ocr" {
		t.Fatalf("first computer_use action = %q, want ocr", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		firstReq, _ := provider.RequestAt(0)
		t.Fatalf("second computer_use action = %q, want message; first resumed request=%s", got, summarizeChatRequestMessagesForDebug(firstReq))
	}
	for _, call := range calls {
		if got := anyToStringForLLM(call["action"]); got == "screenshot" {
			t.Fatalf("unexpected screenshot fallback call: %#v", call)
		}
	}

	firstReq, ok := provider.RequestAt(0)
	if !ok {
		t.Fatalf("missing first resumed request capture")
	}
	var sawGuardrails bool
	for _, msg := range firstReq.Messages {
		if strings.Contains(msg.Content, "desktop chat or messaging task") ||
			strings.Contains(msg.Content, "desktop chat or messaging tasks") {
			sawGuardrails = true
			if !strings.Contains(msg.Content, "`message`, `select`, and `type`") {
				t.Fatalf("expected resumed IM request guardrails to prefer high-level computer_use actions, got %q", msg.Content)
			}
		}
	}
	if !sawGuardrails {
		t.Fatalf("expected resumed IM request to carry desktop chat guardrails, got %#v", firstReq.Messages)
	}
	if !requestContainsComputerUseToolResult(firstReq, "invalid action: ocr") {
		t.Fatalf("expected resumed IM request to include failed computer_use result, got %#v", firstReq.Messages)
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second resumed request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host action completed and submitted") {
		t.Fatalf("expected second resumed request to include successful computer_use message result, got %#v", secondReq.Messages)
	}

	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
	if strings.Contains(resp, "Host screenshot captured") || strings.Contains(resp, "invalid action") {
		t.Fatalf("expected resumed IM response to avoid screenshot/error drift details, got %q", resp)
	}
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRequestRecoversFromScreenshotToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use screenshot desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatScreenshotFirstRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-screenshot-first-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "screenshot" {
		t.Fatalf("first computer_use action = %q, want screenshot", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host screenshot captured") {
		t.Fatalf("expected second request to include screenshot result, got %#v", secondReq.Messages)
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected second request to carry desktop chat guardrails after screenshot, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
}

func TestStreamMessage_ComputerUseDesktopChatRequestRecoversFromScreenshotToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use screenshot desktop chat recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatScreenshotFirstRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-screenshot-first-recovery","model":"gpt-5.3-codex-spark"}`)

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "screenshot" {
		t.Fatalf("first computer_use action = %q, want screenshot", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host screenshot captured") {
		t.Fatalf("expected second stream request to include screenshot result, got %#v", secondReq.Messages)
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected second stream request to carry desktop chat guardrails after screenshot, got %#v", secondReq.Messages)
	}
	if !strings.Contains(body, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
}

func TestStreamMessage_ComputerUseDesktopChatPostToolFailureFallbackStaysIncomplete(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use desktop chat post-tool failure fallback stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := &computerUseDesktopChatPostToolFailureProvider{
		name: "scripted-computer-use-post-tool-failure-stream",
		err:  fmt.Errorf("proxy returned 401: provider prov_test auth error (401): {\"error\":{\"message\":\"未提供令牌\"}}"),
	}
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-post-tool-failure-stream","model":"gpt-5.3-codex-spark"}`)

	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected fallback response instead of STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", body)
	}
	if strings.Contains(body, "我已完成这些工具步骤") {
		t.Fatalf("expected post-tool desktop chat failure to avoid completion wording, got body=%s", body)
	}
	if !strings.Contains(body, "还没有确认发送成功") {
		t.Fatalf("expected post-tool desktop chat failure to remain explicitly incomplete, got body=%s", body)
	}
	if provider.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (tool call + failed follow-up), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 1 {
		t.Fatalf("computer_use calls = %d, want 1", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "screenshot" {
		t.Fatalf("first computer_use action = %q, want screenshot", got)
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected failed follow-up request to carry desktop chat guardrails, got %#v", secondReq.Messages)
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host screenshot captured") {
		t.Fatalf("expected failed follow-up request to include screenshot result, got %#v", secondReq.Messages)
	}
}

func TestProcessChannelMessage_ComputerUseDesktopChatRequestRecoversFromScreenshotToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatScreenshotFirstRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_screenshot_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "screenshot" {
		t.Fatalf("first computer_use action = %q, want screenshot", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host screenshot captured") {
		t.Fatalf("expected second IM request to include screenshot result, got %#v", secondReq.Messages)
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected second IM request to carry desktop chat guardrails after screenshot, got %#v", secondReq.Messages)
	}
	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseDesktopChatRecoversFromScreenshotToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatScreenshotFirstRecoveryProvider("")
	provider.callCount = 1
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_screenshot_recovery")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_screenshot_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"screenshot","app_name":"Feishu,Lark"}`,
	}
	routingMessage := "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_screenshot_recovery",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先看一下当前飞书聊天窗口。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_screenshot_recovery",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "screenshot" {
		t.Fatalf("first computer_use action = %q, want screenshot", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		firstReq, _ := provider.RequestAt(0)
		t.Fatalf("second computer_use action = %q, want message; first resumed request=%s", got, summarizeChatRequestMessagesForDebug(firstReq))
	}

	firstReq, ok := provider.RequestAt(0)
	if !ok {
		t.Fatalf("missing first resumed request capture")
	}
	if !requestContainsComputerUseToolResult(firstReq, "Host screenshot captured") {
		t.Fatalf("expected resumed IM request to include screenshot result, got %#v", firstReq.Messages)
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(firstReq) {
		t.Fatalf("expected resumed IM request to carry desktop chat guardrails after screenshot, got %#v", firstReq.Messages)
	}
	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRequestRecoversFromSnapshotInteractiveToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use interactive snapshot desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatSnapshotInteractiveFirstRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-snapshot-interactive-first-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "snapshot_interactive" {
		t.Fatalf("first computer_use action = %q, want snapshot_interactive", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host interactive snapshot captured") {
		t.Fatalf("expected second request to include interactive snapshot result, got %#v", secondReq.Messages)
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected second request to carry desktop chat guardrails after interactive snapshot, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
}

func TestStreamMessage_ComputerUseDesktopChatRequestRecoversFromSnapshotInteractiveToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use interactive snapshot desktop chat recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatSnapshotInteractiveFirstRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-snapshot-interactive-first-recovery","model":"gpt-5.3-codex-spark"}`)

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "snapshot_interactive" {
		t.Fatalf("first computer_use action = %q, want snapshot_interactive", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host interactive snapshot captured") {
		t.Fatalf("expected second stream request to include interactive snapshot result, got %#v", secondReq.Messages)
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected second stream request to carry desktop chat guardrails after interactive snapshot, got %#v", secondReq.Messages)
	}
	if !strings.Contains(body, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
}

func TestProcessChannelMessage_ComputerUseDesktopChatRequestRecoversFromSnapshotInteractiveToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatSnapshotInteractiveFirstRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_snapshot_interactive_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "snapshot_interactive" {
		t.Fatalf("first computer_use action = %q, want snapshot_interactive", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("second computer_use action = %q, want message; second request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host interactive snapshot captured") {
		t.Fatalf("expected second IM request to include interactive snapshot result, got %#v", secondReq.Messages)
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected second IM request to carry desktop chat guardrails after interactive snapshot, got %#v", secondReq.Messages)
	}
	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseDesktopChatRecoversFromSnapshotInteractiveToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatSnapshotInteractiveFirstRecoveryProvider("")
	provider.callCount = 1
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_snapshot_interactive_recovery")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_snapshot_interactive_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"snapshot_interactive","app_name":"Feishu,Lark"}`,
	}
	routingMessage := "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_snapshot_interactive_recovery",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先抓取一下当前聊天窗口的可交互结构。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_snapshot_interactive_recovery",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	calls := computerUseMock.Calls()
	if len(calls) != 2 {
		t.Fatalf("computer_use calls = %d, want 2", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "snapshot_interactive" {
		t.Fatalf("first computer_use action = %q, want snapshot_interactive", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "message" {
		firstReq, _ := provider.RequestAt(0)
		t.Fatalf("second computer_use action = %q, want message; first resumed request=%s", got, summarizeChatRequestMessagesForDebug(firstReq))
	}

	firstReq, ok := provider.RequestAt(0)
	if !ok {
		t.Fatalf("missing first resumed request capture")
	}
	if !requestContainsComputerUseToolResult(firstReq, "Host interactive snapshot captured") {
		t.Fatalf("expected resumed IM request to include interactive snapshot result, got %#v", firstReq.Messages)
	}
	if !requestCarriesDesktopChatComputerUseGuardrails(firstReq) {
		t.Fatalf("expected resumed IM request to carry desktop chat guardrails after interactive snapshot, got %#v", firstReq.Messages)
	}
	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
}

func TestChatHandlerSendMessage_ComputerUseDesktopChatRequestRecoversFromScreenshotThenClickToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use screenshot then click desktop chat recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatScreenshotThenClickRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-screenshot-then-click-recovery","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if provider.CallCount() != 4 {
		t.Fatalf("expected 4 LLM rounds (screenshot + click + recovered message + final summary), got %d", provider.CallCount())
	}

	calls := computerUseMock.Calls()
	if len(calls) != 3 {
		t.Fatalf("computer_use calls = %d, want 3", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "screenshot" {
		t.Fatalf("first computer_use action = %q, want screenshot", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "click" {
		t.Fatalf("second computer_use action = %q, want click", got)
	}
	if got := anyToStringForLLM(calls[2]["action"]); got != "message" {
		thirdReq, _ := provider.RequestAt(2)
		t.Fatalf("third computer_use action = %q, want message; third request=%s", got, summarizeChatRequestMessagesForDebug(thirdReq))
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host screenshot captured") || !requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected second request to carry screenshot result plus desktop-chat guardrails, got %#v", secondReq.Messages)
	}
	thirdReq, ok := provider.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	if !requestContainsComputerUseToolResult(thirdReq, "Host screenshot captured") ||
		!requestContainsComputerUseToolResult(thirdReq, "Host click completed") ||
		!requestCarriesDesktopChatComputerUseGuardrails(thirdReq) {
		t.Fatalf("expected third request to carry screenshot+click results plus desktop-chat guardrails, got %#v", thirdReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final desktop-chat completion summary, got %q", content)
	}
}

func TestStreamMessage_ComputerUseDesktopChatRequestRecoversFromScreenshotThenClickToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Computer use screenshot then click desktop chat recovery stream")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatScreenshotThenClickRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成","provider":"scripted-computer-use-screenshot-then-click-recovery","model":"gpt-5.3-codex-spark"}`)

	if provider.CallCount() != 4 {
		t.Fatalf("expected 4 LLM rounds (screenshot + click + recovered message + final summary), got %d", provider.CallCount())
	}
	calls := computerUseMock.Calls()
	if len(calls) != 3 {
		t.Fatalf("computer_use calls = %d, want 3", len(calls))
	}
	if got := anyToStringForLLM(calls[2]["action"]); got != "message" {
		thirdReq, _ := provider.RequestAt(2)
		t.Fatalf("third computer_use action = %q, want message; third request=%s", got, summarizeChatRequestMessagesForDebug(thirdReq))
	}
	thirdReq, ok := provider.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	if !requestContainsComputerUseToolResult(thirdReq, "Host screenshot captured") ||
		!requestContainsComputerUseToolResult(thirdReq, "Host click completed") ||
		!requestCarriesDesktopChatComputerUseGuardrails(thirdReq) {
		t.Fatalf("expected third stream request to carry screenshot+click results plus desktop-chat guardrails, got %#v", thirdReq.Messages)
	}
	if !strings.Contains(body, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final streamed completion summary, got body=%s", body)
	}
}

func TestProcessChannelMessage_ComputerUseDesktopChatRequestRecoversFromScreenshotThenClickToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatScreenshotThenClickRecoveryProvider("")
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_screenshot_then_click_recovery",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	if provider.CallCount() != 4 {
		t.Fatalf("expected 4 LLM rounds (screenshot + click + recovered message + final summary), got %d", provider.CallCount())
	}
	calls := computerUseMock.Calls()
	if len(calls) != 3 {
		t.Fatalf("computer_use calls = %d, want 3", len(calls))
	}
	if got := anyToStringForLLM(calls[2]["action"]); got != "message" {
		thirdReq, _ := provider.RequestAt(2)
		t.Fatalf("third computer_use action = %q, want message; third request=%s", got, summarizeChatRequestMessagesForDebug(thirdReq))
	}
	thirdReq, ok := provider.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	if !requestContainsComputerUseToolResult(thirdReq, "Host screenshot captured") ||
		!requestContainsComputerUseToolResult(thirdReq, "Host click completed") ||
		!requestCarriesDesktopChatComputerUseGuardrails(thirdReq) {
		t.Fatalf("expected third IM request to carry screenshot+click results plus desktop-chat guardrails, got %#v", thirdReq.Messages)
	}
	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final IM completion summary, got %q", resp)
	}
}

func TestProcessChannelMessage_CheckpointResumeComputerUseDesktopChatRecoversFromScreenshotThenClickToMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	provider := newDesktopChatScreenshotThenClickRecoveryProvider("")
	provider.callCount = 1
	registry.Register(provider)

	toolRegistry := tools.NewRegistry()
	computerUseMock := &computerUseScenarioToolMock{}
	toolRegistry.Register(computerUseMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume_screenshot_then_click_recovery")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "reply",
		Action:    "send",
	})
	pendingTool := llm.ToolCall{
		ID:        "call_computer_use_screenshot_then_click_resume_1",
		Name:      "computer_use",
		Arguments: `{"action":"screenshot","app_name":"Feishu,Lark"}`,
	}
	routingMessage := "帮我在飞书桌面应用上和【后端之家】打一个招呼，告诉他们是Blue发送的消息，你可以使用辅助（computer_use）工具来完成"
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume_screenshot_then_click_recovery",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  routingMessage,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", PreviousResponseID: "resume_prev_response_1"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先看一下当前飞书聊天窗口。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume_screenshot_then_click_recovery",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}

	if provider.CallCount() != 4 {
		t.Fatalf("expected 4 LLM rounds across resume (click + recovered message + final summary after seeded screenshot), got %d", provider.CallCount())
	}
	calls := computerUseMock.Calls()
	if len(calls) != 3 {
		t.Fatalf("computer_use calls = %d, want 3", len(calls))
	}
	if got := anyToStringForLLM(calls[0]["action"]); got != "screenshot" {
		t.Fatalf("first computer_use action = %q, want screenshot", got)
	}
	if got := anyToStringForLLM(calls[1]["action"]); got != "click" {
		t.Fatalf("second computer_use action = %q, want click", got)
	}
	if got := anyToStringForLLM(calls[2]["action"]); got != "message" {
		secondReq, _ := provider.RequestAt(1)
		t.Fatalf("third computer_use action = %q, want message; second resumed request=%s", got, summarizeChatRequestMessagesForDebug(secondReq))
	}

	secondReq, ok := provider.RequestAt(1)
	if !ok {
		t.Fatalf("missing second resumed request capture")
	}
	if !requestContainsComputerUseToolResult(secondReq, "Host screenshot captured") ||
		!requestContainsComputerUseToolResult(secondReq, "Host click completed") ||
		!requestCarriesDesktopChatComputerUseGuardrails(secondReq) {
		t.Fatalf("expected second resumed request to carry screenshot+click results plus desktop-chat guardrails, got %#v", secondReq.Messages)
	}
	if !strings.Contains(resp, "已经在飞书桌面应用里给【后端之家】发出问候。") {
		t.Fatalf("expected final resumed IM completion summary, got %q", resp)
	}
}
