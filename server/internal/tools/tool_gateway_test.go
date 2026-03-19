package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
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
}

func (t *gatewayResultTool) Definition() ToolDefinition { return t.def }

func (t *gatewayResultTool) Execute(_ context.Context, _ map[string]interface{}) (interface{}, error) {
	t.calls++
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
				"required": []string{"url"},
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

func TestToolGatewayWrapsExternalContentForLLM(t *testing.T) {
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{
		def: ToolDefinition{
			Name:        "web_fetch",
			Description: "fetch",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": false,
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
				"required": []string{"url"},
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
