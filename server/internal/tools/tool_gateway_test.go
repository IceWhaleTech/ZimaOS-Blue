package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

type recursiveGatewayPayload struct {
	Name string                   `json:"name"`
	Self *recursiveGatewayPayload `json:"self,omitempty"`
}

type gatewayResultTool struct {
	def    ToolDefinition
	result interface{}
	err    error
	calls  int
	args   map[string]interface{}
}

func (t *gatewayResultTool) Definition() ToolDefinition { return t.def }

func (t *gatewayResultTool) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	t.calls++
	t.args = args
	if t.err != nil {
		return nil, t.err
	}
	return t.result, nil
}

type approvalStub struct {
	decision ToolApprovalDecision
	err      error
	req      ToolApprovalRequest
	calls    int
}

func (s *approvalStub) AuthorizeToolCall(_ context.Context, req ToolApprovalRequest) (ToolApprovalDecision, error) {
	s.calls++
	s.req = req
	return s.decision, s.err
}

type gatewayMetricCall struct {
	name  string
	value int64
	tags  map[string]string
}

type gatewayMetricsStub struct {
	calls []gatewayMetricCall
}

func (s *gatewayMetricsStub) RecordCounter(name string, value int64, tags map[string]string) {
	cloned := make(map[string]string, len(tags))
	for k, v := range tags {
		cloned[k] = v
	}
	s.calls = append(s.calls, gatewayMetricCall{name: name, value: value, tags: cloned})
}

type runtimeObserverStub struct {
	toolRequested     []ToolRuntimeEvent
	toolFinished      []ToolRuntimeEvent
	approvalRequested []ApprovalRuntimeEvent
	approvalResolved  []ApprovalRuntimeEvent
	questionRequested []QuestionRuntimeEvent
	questionResolved  []QuestionRuntimeEvent
}

func (s *runtimeObserverStub) OnToolRequested(event ToolRuntimeEvent) {
	s.toolRequested = append(s.toolRequested, event)
}

func (s *runtimeObserverStub) OnToolFinished(event ToolRuntimeEvent) {
	s.toolFinished = append(s.toolFinished, event)
}

func (s *runtimeObserverStub) OnApprovalRequested(event ApprovalRuntimeEvent) {
	s.approvalRequested = append(s.approvalRequested, event)
}

func (s *runtimeObserverStub) OnApprovalResolved(event ApprovalRuntimeEvent) {
	s.approvalResolved = append(s.approvalResolved, event)
}

func (s *runtimeObserverStub) OnQuestionRequested(event QuestionRuntimeEvent) {
	s.questionRequested = append(s.questionRequested, event)
}

func (s *runtimeObserverStub) OnQuestionResolved(event QuestionRuntimeEvent) {
	s.questionResolved = append(s.questionResolved, event)
}

type structuredRuntimeErrorStub struct {
	code    string
	message string
	details map[string]interface{}
}

func buildWebQueryGatewayPayloadForTests(content string) map[string]interface{} {
	return map[string]interface{}{
		"status":      "ok",
		"mode":        "search_read",
		"input":       "blue release notes",
		"query":       "blue release notes",
		"provider":    "duckduckgo",
		"title":       "Blue Release Notes v1.2.3",
		"target_url":  "https://docs.example.com/releases/1.2.3",
		"final_url":   "https://docs.example.com/releases/1.2.3",
		"content":     content,
		"next_action": "none",
		"warnings": []map[string]interface{}{
			{"code": "stale_mirror", "message": "Mirror lag observed for 2 pages."},
		},
		"sources": []interface{}{
			map[string]interface{}{
				"rank":          1,
				"title":         "Blue Release Notes v1.2.3",
				"url":           "https://docs.example.com/releases/1.2.3",
				"final_url":     "https://docs.example.com/releases/1.2.3",
				"snippet":       "April 4, 2026 release with 12 improvements, 4 fixes, and 2 migrations.",
				"source":        "duckduckgo",
				"content_chars": len(content),
				"selected":      true,
			},
			map[string]interface{}{
				"rank":          2,
				"title":         "Blue Upgrade Guide",
				"url":           "https://docs.example.com/releases/upgrade-guide",
				"final_url":     "https://docs.example.com/releases/upgrade-guide",
				"snippet":       "Upgrade guide for version 1.2.3 with migration steps and rollback notes.",
				"source":        "duckduckgo",
				"content_chars": 640,
				"selected":      false,
			},
		},
		"diagnostics": map[string]interface{}{
			"route":           "search_http",
			"candidate_count": 2,
			"selected_source": 1,
			"degraded":        false,
		},
	}
}

func (e *structuredRuntimeErrorStub) Error() string { return e.message }

func (e *structuredRuntimeErrorStub) ToolRuntimeCode() string { return e.code }

func (e *structuredRuntimeErrorStub) ToolRuntimeDetails() map[string]interface{} {
	if len(e.details) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(e.details))
	for key, value := range e.details {
		out[key] = value
	}
	return out
}

func TestToolGatewayRejectsUnknownTool(t *testing.T) {
	registry := NewRegistry()
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-1",
		ToolName:   "missing_tool",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result == nil || !strings.Contains(result.CompactLLMContent, "tool_not_found") {
		t.Fatalf("expected structured not-found payload, got %+v", result)
	}
}

func TestToolGatewayEmitsRuntimeObserverEvents(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "browser",
			Description: "browser",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"url"},
				"additionalProperties": true,
			},
		},
		result: map[string]interface{}{"ok": true},
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))
	observer := &runtimeObserverStub{}
	gateway.SetEventObserver(observer)

	ctx := WithRunID(context.Background(), "run-123")
	ctx = WithRunStep(ctx, 4)
	_, err := gateway.Execute(ctx, ToolGatewayRequest{
		ToolCallID: "call-obs",
		ToolName:   "browser",
		Arguments:  `{"url":"https://example.com"}`,
		RouteKind:  ToolRouteKindAgent,
		SessionID:  "conv-1",
		UserID:     "user-1",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(observer.toolRequested) != 1 {
		t.Fatalf("toolRequested len = %d, want 1", len(observer.toolRequested))
	}
	if len(observer.toolFinished) != 1 {
		t.Fatalf("toolFinished len = %d, want 1", len(observer.toolFinished))
	}
	if observer.toolRequested[0].RunID != "run-123" || observer.toolFinished[0].RunID != "run-123" {
		t.Fatalf("unexpected run ids: req=%q fin=%q", observer.toolRequested[0].RunID, observer.toolFinished[0].RunID)
	}
	if observer.toolRequested[0].StepIndex != 4 || observer.toolFinished[0].StepIndex != 4 {
		t.Fatalf("unexpected step indexes: req=%d fin=%d", observer.toolRequested[0].StepIndex, observer.toolFinished[0].StepIndex)
	}
	if observer.toolRequested[0].ToolName != "browser" || observer.toolFinished[0].ToolName != "browser" {
		t.Fatalf("unexpected tool names: req=%q fin=%q", observer.toolRequested[0].ToolName, observer.toolFinished[0].ToolName)
	}
	if observer.toolFinished[0].Error != "" {
		t.Fatalf("unexpected tool finished error: %q", observer.toolFinished[0].Error)
	}
}

func TestToolGatewayRejectsInvalidArguments(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "search",
			Description: "search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required": []string{"query"},
			},
		},
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-2",
		ToolName:   "search",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err == nil {
		t.Fatal("expected schema validation error, got nil")
	}
	if result == nil || !strings.Contains(result.CompactLLMContent, "invalid_tool_arguments") {
		t.Fatalf("expected invalid arguments payload, got %+v", result)
	}
}

func TestToolGatewayRecoversConcatenatedJSONObjectArguments(t *testing.T) {
	registry := NewRegistry()
	tool := &gatewayResultTool{
		def: ToolDefinition{
			Name:        "search",
			Description: "search",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string"},
				},
				"required": []string{"query"},
			},
		},
		result: map[string]interface{}{"ok": true},
	}
	registry.Register(tool)
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-concat",
		ToolName:   "search",
		Arguments:  `{}{"query":"latest blue release"}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("result is nil")
	}
	if got := tool.args["query"]; got != "latest blue release" {
		t.Fatalf("query = %v, want %q", got, "latest blue release")
	}
}

func TestToolGatewayWrapsExternalContentForLLM(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "web_fetch",
			Description: "fetch",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": true,
			},
		},
		result: "\x1b[31munsafe html\x1b[0m",
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-3",
		ToolName:   "web_fetch",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.CompactLLMContent), &payload); err != nil {
		t.Fatalf("decode compact payload: %v", err)
	}
	if payload["trust"] != "untrusted_external_content" {
		t.Fatalf("trust = %v, want untrusted_external_content", payload["trust"])
	}
	if strings.Contains(result.CompactLLMContent, "\x1b[31m") {
		t.Fatalf("compact payload should strip ANSI, got %q", result.CompactLLMContent)
	}
}

func TestToolGatewayCompactsPDFPayloadForLLM(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "pdf",
			Description: "pdf",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": true,
			},
		},
		result: pdfextract.ExtractResult{
			Document:      pdfextract.DocumentInfo{FileName: "report.pdf", PageCount: 2},
			Text:          "merged document text",
			RawText:       "raw merged document text",
			SelectedPages: []int{1, 2},
			Pages: []pdfextract.PageText{
				{Number: 1, Text: "page one", RawText: "raw page one", CharCount: 8, Source: "text"},
				{Number: 2, Text: "page two", RawText: "raw page two", CharCount: 8, Source: "text"},
			},
			CharCount: 16,
		},
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-pdf",
		ToolName:   "pdf",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.CompactLLMContent), &payload); err != nil {
		t.Fatalf("decode compact payload: %v", err)
	}
	if _, exists := payload["raw_text"]; exists {
		t.Fatalf("compact payload should drop top-level raw_text: %#v", payload)
	}
	if _, exists := payload["text"]; exists {
		t.Fatalf("compact payload should avoid duplicate merged text when pages are available: %#v", payload)
	}
	pages, ok := payload["pages"].([]interface{})
	if !ok || len(pages) != 2 {
		t.Fatalf("pages = %#v, want 2 entries", payload["pages"])
	}
	firstPage, ok := pages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first page = %#v", pages[0])
	}
	if _, exists := firstPage["raw_text"]; exists {
		t.Fatalf("compact page payload should drop raw_text: %#v", firstPage)
	}
	if got := firstPage["text"]; got != "page one" {
		t.Fatalf("first page text = %v, want page one", got)
	}
}

func TestToolGatewayCompactsPDFPayloadForLLM_DerivesPagesBeforeSanitize(t *testing.T) {
	longPage := strings.Repeat("A", 5000)
	secondPage := strings.Repeat("B", 5000)
	thirdPage := "final page needle"

	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "pdf",
			Description: "pdf",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": true,
			},
		},
		result: pdfextract.ExtractResult{
			Document:      pdfextract.DocumentInfo{FileName: "report.pdf", PageCount: 3},
			Text:          "[Page 1]\n" + longPage + "\n\n[Page 2]\n" + secondPage + "\n\n[Page 3]\n" + thirdPage,
			SelectedPages: []int{1, 2, 3},
			CharCount:     len(longPage) + len(secondPage) + len(thirdPage),
		},
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-pdf-derived-pages",
		ToolName:   "pdf",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.CompactLLMContent), &payload); err != nil {
		t.Fatalf("decode compact payload: %v", err)
	}
	if _, exists := payload["text"]; exists {
		t.Fatalf("compact payload should prefer derived pages over merged text: %#v", payload)
	}
	pages, ok := payload["pages"].([]interface{})
	if !ok || len(pages) != 3 {
		t.Fatalf("pages = %#v, want 3 entries", payload["pages"])
	}
	lastPage, ok := pages[2].(map[string]interface{})
	if !ok {
		t.Fatalf("last page = %#v", pages[2])
	}
	if got := lastPage["number"]; got != float64(3) {
		t.Fatalf("last page number = %v, want 3", got)
	}
	if got := lastPage["text"]; got != thirdPage {
		t.Fatalf("last page text = %v, want %q", got, thirdPage)
	}
}

func TestToolGatewayHonorsApprovalDeny(t *testing.T) {
	registry := NewRegistry()
	tool := &gatewayResultTool{
		def: ToolDefinition{
			Name:        "browser",
			Description: "browser",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string"},
				},
				"required":             []string{"url"},
				"additionalProperties": true,
			},
		},
		result: map[string]interface{}{"ok": true},
	}
	registry.Register(tool)

	approver := &approvalStub{
		decision: ToolApprovalDecision{
			Allowed: false,
			Approval: ToolApprovalEnvelope{
				Required:     true,
				Mode:         "ask",
				ID:           "approval-1",
				BindingHash:  "hash-1",
				PolicySource: "approval.tool_policies.browser",
				RiskLevel:    "medium",
			},
		},
	}
	gateway := NewToolGateway(registry, NewExecutor(registry))
	gateway.SetApprover(approver)

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-4",
		ToolName:   "browser",
		Arguments:  `{"url":"https://example.com"}`,
		RouteKind:  ToolRouteKindChat,
		SessionID:  "conv-1",
	})
	if err == nil {
		t.Fatal("expected approval error, got nil")
	}
	if tool.calls != 0 {
		t.Fatalf("tool executed %d times, want 0", tool.calls)
	}
	if approver.calls != 1 {
		t.Fatalf("approver calls = %d, want 1", approver.calls)
	}
	if result == nil || result.Approval == nil || result.Approval.ID != "approval-1" {
		t.Fatalf("expected approval metadata, got %+v", result)
	}
}

func TestToolGatewayPreservesStructuredRuntimeExecutionError(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "subagents",
			Description: "subagents",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"goal": map[string]interface{}{"type": "string"},
				},
				"required": []string{"goal"},
			},
		},
		err: &structuredRuntimeErrorStub{
			code:    "budget_exceeded",
			message: "max depth exceeded",
			details: map[string]interface{}{"stage": "policy", "max_depth": 2},
		},
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-structured-runtime-error",
		ToolName:   "subagents",
		Arguments:  `{"goal":"investigate retry path"}`,
		RouteKind:  ToolRouteKindAgent,
	})
	if err == nil {
		t.Fatal("expected structured runtime error")
	}
	gatewayErr, ok := err.(*ToolGatewayError)
	if !ok {
		t.Fatalf("expected ToolGatewayError, got %T", err)
	}
	if gatewayErr.Code != "budget_exceeded" {
		t.Fatalf("code = %q, want budget_exceeded", gatewayErr.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.CompactLLMContent), &payload); err != nil {
		t.Fatalf("decode compact payload: %v", err)
	}
	if got := payload["code"]; got != "budget_exceeded" {
		t.Fatalf("payload code = %v, want budget_exceeded", got)
	}
	details, ok := payload["details"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload details = %#v", payload["details"])
	}
	if got := details["stage"]; got != "policy" {
		t.Fatalf("details.stage = %v, want policy", got)
	}
	if got := details["tool"]; got != "subagents" {
		t.Fatalf("details.tool = %v, want subagents", got)
	}
}

func TestToolGatewayPreservesWrappedStructuredRuntimeExecutionError(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "subagents",
			Description: "subagents",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"goal": map[string]interface{}{"type": "string"},
				},
				"required": []string{"goal"},
			},
		},
		err: fmt.Errorf("wrapped runtime failure: %w", &structuredRuntimeErrorStub{
			code:    "subagent_disabled",
			message: "subagents are disabled for the current agent",
			details: map[string]interface{}{"stage": "policy", "agent_id": "main"},
		}),
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	_, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-wrapped-runtime-error",
		ToolName:   "subagents",
		Arguments:  `{"goal":"investigate retry path"}`,
		RouteKind:  ToolRouteKindAgent,
	})
	if err == nil {
		t.Fatal("expected wrapped structured runtime error")
	}
	gatewayErr, ok := err.(*ToolGatewayError)
	if !ok {
		t.Fatalf("expected ToolGatewayError, got %T", err)
	}
	if gatewayErr.Code != "subagent_disabled" {
		t.Fatalf("code = %q, want subagent_disabled", gatewayErr.Code)
	}
}

func TestToolGatewayPreservesStructuredQuestionRuntimeError(t *testing.T) {
	registry := NewRegistry()
	questionMgr := NewQuestionManager(sse.NewBroker(), func() bool { return false }, 0)
	questionMgr.SetTimeoutActionFunc(func() string { return "error" })
	RegisterAskTool(registry, questionMgr)

	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-question-runtime-error",
		ToolName:   "ask",
		Arguments:  `{"questions":[{"question":"Pick one","options":[{"label":"A","value":"a"}]}]}`,
		RouteKind:  ToolRouteKindAgent,
		UserID:     "user-1",
		SessionID:  "session-1",
	})
	if err == nil {
		t.Fatal("expected structured question runtime error")
	}
	gatewayErr, ok := err.(*ToolGatewayError)
	if !ok {
		t.Fatalf("expected ToolGatewayError, got %T", err)
	}
	if gatewayErr.Code != "question_delivery_unavailable" {
		t.Fatalf("code = %q, want question_delivery_unavailable", gatewayErr.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.CompactLLMContent), &payload); err != nil {
		t.Fatalf("decode compact payload: %v", err)
	}
	if got := payload["code"]; got != "question_delivery_unavailable" {
		t.Fatalf("payload code = %v, want question_delivery_unavailable", got)
	}
	details, ok := payload["details"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload details = %#v", payload["details"])
	}
	if got := details["tool"]; got != "ask" {
		t.Fatalf("details.tool = %v, want ask", got)
	}
	if got := details["question_count"]; got != float64(1) {
		t.Fatalf("details.question_count = %v, want 1", got)
	}
}

func TestToolGatewayRejectsToolHiddenFromRoute(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:                "agent_only",
			Description:         "agent only",
			Parameters:          map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
			VisibilityAllowlist: []string{string(ToolRouteKindAgent)},
		},
		result: map[string]interface{}{"ok": true},
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-visibility",
		ToolName:   "agent_only",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err == nil {
		t.Fatal("expected visibility error, got nil")
	}
	if result == nil || !strings.Contains(result.CompactLLMContent, "tool_not_visible_for_route") {
		t.Fatalf("expected route visibility payload, got %+v", result)
	}
}

func TestToolGatewayNormalizesCompatAliasToUnifiedTool(t *testing.T) {
	registry := NewRegistry()
	webTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "web_query",
			Description: "web",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{"type": "string"},
					"url":    map[string]interface{}{"type": "string"},
				},
				"additionalProperties": true,
			},
		},
	}
	registry.Register(webTool)

	gateway := NewToolGateway(registry, NewExecutor(registry))
	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-web-compat",
		ToolName:   "web_fetch",
		Arguments:  `{"href":"https://example.com"}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.NormalizedCall.ToolName != "web_query" {
		t.Fatalf("tool name = %q, want web_query", result.NormalizedCall.ToolName)
	}
	if got := webTool.args["action"]; got != "fetch" {
		t.Fatalf("action = %v, want fetch", got)
	}
	if got := webTool.args["url"]; got != "https://example.com" {
		t.Fatalf("url = %v, want https://example.com", got)
	}
}

func TestToolGatewaySanitizesScalarPayloads(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "scalar_payload",
			Description: "scalar payload",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		result: map[string]interface{}{
			"ok":    true,
			"count": 3,
			"items": []interface{}{true, 2.5, "\x1b[31mclean\x1b[0m"},
		},
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-scalars",
		ToolName:   "scalar_payload",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.CompactLLMContent), &payload); err != nil {
		t.Fatalf("decode compact payload: %v", err)
	}
	if got := payload["ok"]; got != true {
		t.Fatalf("ok = %v, want true", got)
	}
	if got := payload["count"]; got != float64(3) {
		t.Fatalf("count = %v, want 3", got)
	}
	items, ok := payload["items"].([]interface{})
	if !ok || len(items) != 3 {
		t.Fatalf("items = %#v, want 3 sanitized entries", payload["items"])
	}
	if got := items[0]; got != true {
		t.Fatalf("items[0] = %v, want true", got)
	}
	if got := items[1]; got != 2.5 {
		t.Fatalf("items[1] = %v, want 2.5", got)
	}
	if got := items[2]; got != "clean" {
		t.Fatalf("items[2] = %v, want clean", got)
	}
}

func TestToolGatewayCompactsWebQueryPayloadForLLM(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "web_query",
			Description: "web query",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": true,
			},
		},
		result: buildWebQueryGatewayPayloadForTests(strings.Repeat("On April 4, 2026, version 1.2.3 shipped 12 improvements, 4 fixes, and 2 migrations. ", 80)),
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-web-query-compact",
		ToolName:   "web_query",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.CompactLLMContent), &payload); err != nil {
		t.Fatalf("decode compact payload: %v", err)
	}
	if got, ok := payload["has_results"].(bool); !ok || !got {
		t.Fatalf("has_results = %#v, want true", payload["has_results"])
	}
	selected, ok := payload["selected_result"].(map[string]interface{})
	if !ok {
		t.Fatalf("selected_result = %#v, want object", payload["selected_result"])
	}
	if got := selected["url"]; got != "https://docs.example.com/releases/1.2.3" {
		t.Fatalf("selected_result.url = %v, want selected page", got)
	}
	facts, ok := payload["key_facts"].([]interface{})
	if !ok || len(facts) == 0 {
		t.Fatalf("key_facts = %#v, want non-empty array", payload["key_facts"])
	}
}

func TestToolGatewayCompactsWebQueryPayloadForLLM_MaterializesLargePayload(t *testing.T) {
	workspaceRoot := t.TempDir()
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "web_query",
			Description: "web query",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": true,
			},
		},
		result: buildWebQueryGatewayPayloadForTests(strings.Repeat("On April 4, 2026, version 1.2.3 shipped 12 improvements, 4 fixes, and 2 migrations. ", 220)),
	})
	gateway := NewToolGateway(registry, NewExecutor(registry))

	ctx := WithFSScope(context.Background(), []string{workspaceRoot}, map[string]string{"workspace": workspaceRoot})
	result, err := gateway.Execute(ctx, ToolGatewayRequest{
		ToolCallID: "call-web-query-materialize",
		ToolName:   "web_query",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
		SessionID:  "conv-web-query",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.CompactLLMContent), &payload); err != nil {
		t.Fatalf("decode compact payload: %v", err)
	}
	artifactPath, _ := payload["research_artifact_path"].(string)
	if strings.TrimSpace(artifactPath) == "" {
		t.Fatalf("research_artifact_path = %#v, want non-empty path", payload["research_artifact_path"])
	}
	if got, ok := payload["materialized"].(bool); !ok || !got {
		t.Fatalf("materialized = %#v, want true", payload["materialized"])
	}
	artifactBytes, err := os.ReadFile(filepath.Join(workspaceRoot, artifactPath))
	if err != nil {
		t.Fatalf("read materialized artifact: %v", err)
	}
	artifact := string(artifactBytes)
	if !strings.Contains(artifact, `"query": "blue release notes"`) {
		t.Fatalf("artifact missing query metadata: %q", artifact)
	}
	if !strings.Contains(artifact, `"sources"`) {
		t.Fatalf("artifact missing audit JSON section: %q", artifact)
	}
}

func TestToolGatewayExecutesWebQueryVideoLanguageFallbackE2E(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/watch":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><body><script>
var ytInitialPlayerResponse = {"videoDetails":{"videoId":"abc123","title":"Demo video","author":"Demo creator","lengthSeconds":"42","thumbnail":{"thumbnails":[{"url":"https://img.example/thumb.jpg"}]}},"microformat":{"playerMicroformatRenderer":{"publishDate":"2026-03-23","availableCountries":["US"]}},"captions":{"playerCaptionsTracklistRenderer":{"captionTracks":[{"baseUrl":"https://www.youtube.com/api/timedtext?v=abc123&lang=en","name":{"simpleText":"English (UK) auto-generated"},"kind":"asr"}]}}};
</script></body></html>`))
		case r.URL.Path == "/api/timedtext":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"events":[{"tStartMs":0,"dDurationMs":1600,"segs":[{"utf8":"Hello from the fallback subtitle lane."}]},{"tStartMs":1600,"dDurationMs":1900,"segs":[{"utf8":"This end to end gateway test verifies language fallback survives the real web_query entry path."}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	fetchTool := NewWebFetchTool(WebFetchConfig{AllowPrivateHosts: true})
	fetchTool.httpClient = newRewrittenHTTPClient(t, server)
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      args["url"].(string),
				FinalURL: args["url"].(string),
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Demo video",
				Content:  "Gateway e2e page summary.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	webTool := NewWebQueryTool(nil, fetchTool, readTool, nil, nil)
	registry := NewRegistry()
	registry.Register(webTool)
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(WithLang(context.Background(), "zh-TW"), ToolGatewayRequest{
		ToolCallID: "call-video-e2e",
		ToolName:   "web_query",
		Arguments:  `{"input":"https://www.youtube.com/watch?v=abc123"}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	raw, ok := result.ExecutionResult.(string)
	if !ok {
		t.Fatalf("execution result type = %T, want string", result.ExecutionResult)
	}
	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Mode != "video_read" {
		t.Fatalf("mode = %q, want video_read", envelope.Mode)
	}
	if envelope.Transcript == nil || envelope.Transcript.Language != "en-gb" {
		t.Fatalf("transcript = %+v, want language en-gb", envelope.Transcript)
	}
	if envelope.Media == nil || envelope.Media.Language != "en-gb" {
		t.Fatalf("media = %+v, want language en-gb", envelope.Media)
	}
	if strings.Contains(strings.ToLower(envelope.Media.Language), "us") {
		t.Fatalf("media language leaked country code fallback: %+v", envelope.Media)
	}
}

func TestToolGatewayExecutesBilibiliLabelLanguageFallbackE2E(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/video/BV1demo":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><body><script>
window.__INITIAL_STATE__={"videoData":{"bvid":"BV1demo","title":"Bili demo","pic":"//img.example/bili.jpg","owner":{"name":"Uploader"},"duration":65,"pubdate":1711142400}};
window.__playinfo__={"data":{"subtitle":{"subtitles":[{"lan_doc":"繁體中文","subtitle_url":"//www.bilibili.com/subtitle.json"}]}}};
</script></body></html>`))
		case "/subtitle.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"body":[{"from":0.0,"to":1.2,"content":"第一句字幕。"},{"from":1.2,"to":3.8,"content":"這是一段用來驗證 bilibili 標籤語言回退邏輯的字幕內容。"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	fetchTool := NewWebFetchTool(WebFetchConfig{AllowPrivateHosts: true})
	fetchTool.httpClient = newRewrittenHTTPClient(t, server)
	readTool := &scriptedWebTool{
		def: ToolDefinition{Name: "web_read", Description: "read", Parameters: map[string]interface{}{"type": "object", "additionalProperties": true}},
		exec: func(args map[string]interface{}) (interface{}, error) {
			resp := webReadResponse{
				URL:      args["url"].(string),
				FinalURL: args["url"].(string),
				Format:   "text",
				Source:   webAccessSourceHTTP,
				Title:    "Bili demo",
				Content:  "Gateway e2e bilibili page summary.",
			}
			b, _ := json.Marshal(resp)
			return string(b), nil
		},
	}
	webTool := NewWebQueryTool(nil, fetchTool, readTool, nil, nil)
	registry := NewRegistry()
	registry.Register(webTool)
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(WithLang(context.Background(), "en-US"), ToolGatewayRequest{
		ToolCallID: "call-bili-e2e",
		ToolName:   "web_query",
		Arguments:  `{"input":"https://www.bilibili.com/video/BV1demo"}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	raw, ok := result.ExecutionResult.(string)
	if !ok {
		t.Fatalf("execution result type = %T, want string", result.ExecutionResult)
	}
	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Transcript == nil || envelope.Transcript.Language != "zh-tw" {
		t.Fatalf("transcript = %+v, want language zh-tw", envelope.Transcript)
	}
	if envelope.Media == nil || envelope.Media.Language != "zh-tw" {
		t.Fatalf("media = %+v, want language zh-tw", envelope.Media)
	}
}

func TestToolGatewayExecutesDirectMediaASRLanguageFallbackE2E(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/clip.mp3" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("fake mp3 bytes"))
	}))
	defer server.Close()

	fetchTool := NewWebFetchTool(WebFetchConfig{AllowPrivateHosts: true})
	webTool := NewWebQueryTool(nil, fetchTool, nil, nil, nil)
	webTool.SetSTTService(&scriptedWebQuerySTTService{
		response: &stt.TranscribeResponse{
			Text:     "Local ASR returned transcript text but did not provide a stable language code, so the gateway should fall back to the request context locale.",
			Language: "",
			Duration: 4.2,
			Segments: []stt.Segment{
				{Start: 0, End: 2.1, Text: "Local ASR returned transcript text but did not provide a stable language code."},
				{Start: 2.1, End: 4.2, Text: "So the gateway should fall back to the request context locale."},
			},
			Confidence: 0.88,
		},
	})
	registry := NewRegistry()
	registry.Register(webTool)
	gateway := NewToolGateway(registry, NewExecutor(registry))

	result, err := gateway.Execute(WithLang(context.Background(), "ja-JP"), ToolGatewayRequest{
		ToolCallID: "call-direct-media-e2e",
		ToolName:   "web_query",
		Arguments:  `{"input":"` + server.URL + `/clip.mp3"}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	raw, ok := result.ExecutionResult.(string)
	if !ok {
		t.Fatalf("execution result type = %T, want string", result.ExecutionResult)
	}
	var envelope webQueryEnvelope
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Transcript == nil || envelope.Transcript.Source != "asr_local" || envelope.Transcript.Language != "ja-jp" {
		t.Fatalf("transcript = %+v, want asr_local + ja-jp", envelope.Transcript)
	}
	if envelope.Media == nil || envelope.Media.Language != "ja-jp" {
		t.Fatalf("media = %+v, want language ja-jp", envelope.Media)
	}
}

func TestSanitizeToolPayloadGuardsCircularValues(t *testing.T) {
	payload := map[string]interface{}{}
	payload["self"] = payload

	sanitized := sanitizeToolPayload(payload, maxToolGatewayLLMStringBytes)
	asMap, ok := sanitized.(map[string]interface{})
	if !ok {
		t.Fatalf("sanitized payload = %#v, want map", sanitized)
	}
	if got := asMap["self"]; got != "[circular payload omitted]" {
		t.Fatalf("self = %v, want circular marker", got)
	}
}

func TestSanitizeToolPayloadGuardsRecursiveStructPointers(t *testing.T) {
	payload := &recursiveGatewayPayload{Name: "root"}
	payload.Self = payload

	sanitized := sanitizeToolPayload(payload, maxToolGatewayLLMStringBytes)
	asMap, ok := sanitized.(map[string]interface{})
	if !ok {
		t.Fatalf("sanitized payload = %#v, want map", sanitized)
	}
	if got := asMap["name"]; got != "root" {
		t.Fatalf("name = %v, want root", got)
	}
	if got := asMap["self"]; got != "[circular payload omitted]" {
		t.Fatalf("self = %v, want circular marker", got)
	}
}

func TestToolGatewayRecordsCircularPayloadMetric(t *testing.T) {
	registry := NewRegistry()
	payload := &recursiveGatewayPayload{Name: "root"}
	payload.Self = payload
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "recursive_payload",
			Description: "recursive payload",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
			},
		},
		result: payload,
	})
	metrics := &gatewayMetricsStub{}
	gateway := NewToolGateway(registry, NewExecutor(registry))
	gateway.SetMetricsRecorder(metrics)

	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{
		ToolCallID: "call-recursive",
		ToolName:   "recursive_payload",
		Arguments:  `{}`,
		RouteKind:  ToolRouteKindChat,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil || !strings.Contains(result.AuditContent, "[circular payload omitted]") {
		t.Fatalf("audit content = %#v, want circular marker", result)
	}
	if len(metrics.calls) == 0 {
		t.Fatal("expected at least one metric call")
	}
	found := false
	for _, call := range metrics.calls {
		if call.name == "tool_payload_circular_total" && call.tags["tool"] == "recursive_payload" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("metrics = %#v, want tool_payload_circular_total for recursive_payload", metrics.calls)
	}
}
