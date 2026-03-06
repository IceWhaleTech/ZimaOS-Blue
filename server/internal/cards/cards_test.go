package cards

import (
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func TestToCard(t *testing.T) {
	tests := []struct {
		name     string
		tool     string
		content  string
		wantType string
		wantNil  bool
	}{
		{"calculator_success", "calculator", `{"expression":"1+1","result":2}`, "result", false},
		{"calculator_error", "calculator", `{"error":"bad expr"}`, "result", false},
		{"calculator_invalid", "calculator", `not json`, "", true},
		{"web_search", "web_search", `{"query":"go","results":[{"title":"Go"}],"total_count":1}`, "search", false},
		{"web_search_empty", "web_search", `{"query":"go","results":[],"total_count":0}`, "", true},
		{"deep_research", "deep_research", `{"query":"go","mode":"standard","answer":"summary","confidence":0.9,"evidence_count":2,"citations":[{"title":"A","url":"https://example.com"}]}`, "deep-research", false},
		{"current_time", "current_time", `{"datetime":"2025-01-01","timezone":"UTC","unix":1735689600}`, "result", false},
		{"read", "read", `{"path":"/tmp/x","content":"hello"}`, "collapsible-code", false},
		{"read_empty", "read", `{"path":"/tmp/x","content":""}`, "", true},
		{"read_error", "read", `{"error":"not found"}`, "result", false},
		{"write", "write", `{"path":"/tmp/x","message":"ok"}`, "result", false},
		{"legacy_file_read", "file_read", `{"path":"/tmp/x","content":"hello"}`, "collapsible-code", false},
		{"legacy_file_write", "file_write", `{"path":"/tmp/x","message":"ok"}`, "result", false},
		{"system_info", "system_info", `{"os":"linux","arch":"amd64"}`, "result", false},
		{"memory_search", "memory_search", `{"results":[{"text":"hi"}]}`, "result", false},
		{"memory_search_error", "memory_search", `{"error":"no index"}`, "result", false},
		{"ui_reviewer", "ui_reviewer", `{"url":"http://x.com","overall":80,"pass":true}`, "ui-review", false},
		{"ui_reviewer_error", "ui_reviewer", `{"error":"fail"}`, "ui-review", false},
		{"ui_reviewer_invalid", "ui_reviewer", `not json`, "", true},
		{"generic", "unknown_tool", `{"foo":"bar"}`, "result", false},
		{"generic_error", "unknown_tool", `{"error":"oops"}`, "result", false},
		{"generic_long", "unknown_tool", strings.Repeat("x", 600), "result", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			card := ToCard(tt.tool, tt.content)
			if tt.wantNil {
				if card != nil {
					t.Errorf("expected nil card, got %v", card)
				}
				return
			}
			if card == nil {
				t.Fatal("expected non-nil card")
			}
			if got := card["type"].(string); got != tt.wantType {
				t.Errorf("expected type %q, got %q", tt.wantType, got)
			}
		})
	}
}

func TestFormatTypeless(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Name: "calculator", Arguments: `{"expression":"1+1"}`},
		{ID: "2", Name: "current_time", Arguments: `{}`},
	}
	results := []llm.Message{
		{Role: llm.RoleTool, Content: `{"expression":"1+1","result":2}`, ToolCallID: "1"},
		{Role: llm.RoleTool, Content: `{"datetime":"now"}`, ToolCallID: "2"},
	}

	out := FormatTypeless(calls, results)
	if !strings.Contains(out, "```typeless") {
		t.Error("expected typeless fence blocks")
	}
	if strings.Count(out, "```typeless") != 2 {
		t.Errorf("expected 2 typeless blocks, got %d", strings.Count(out, "```typeless"))
	}
}

func TestFormatTypeless_MoreCallsThanResults(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Name: "calculator", Arguments: `{}`},
		{ID: "2", Name: "calculator", Arguments: `{}`},
	}
	results := []llm.Message{
		{Role: llm.RoleTool, Content: `{"result":42}`, ToolCallID: "1"},
	}

	out := FormatTypeless(calls, results)
	if strings.Count(out, "```typeless") != 1 {
		t.Errorf("expected 1 typeless block, got %d", strings.Count(out, "```typeless"))
	}
}

func TestDeepResearchCard_FieldsPreserved(t *testing.T) {
	content := `{"query":"ZimaOS","mode":"deep","answer":"summary","confidence":0.87,"evidence_count":3,"citations":[{"title":"Doc","url":"https://example.com"}],"open_questions":["q1"],"support_count":2,"conflict_count":1,"has_conflict":true,"status":"completed"}`
	card := ToCard("deep_research", content)
	if card == nil {
		t.Fatal("expected non-nil deep-research card")
	}
	if got := card["type"]; got != "deep-research" {
		t.Fatalf("type=%v, want deep-research", got)
	}
	if got := card["query"]; got != "ZimaOS" {
		t.Fatalf("query=%v, want ZimaOS", got)
	}
	if got := card["evidence_count"]; got != float64(3) {
		t.Fatalf("evidence_count=%v, want 3", got)
	}
	if got := card["support_count"]; got != float64(2) {
		t.Fatalf("support_count=%v, want 2", got)
	}
	if got := card["conflict_count"]; got != float64(1) {
		t.Fatalf("conflict_count=%v, want 1", got)
	}
	if got := card["has_conflict"]; got != true {
		t.Fatalf("has_conflict=%v, want true", got)
	}
}

func TestGenericCard_ShowsDetails(t *testing.T) {
	card := GenericCard("test", `{"foo":"bar","token":"secret"}`)
	if card["type"] != "result" {
		t.Fatalf("expected result type, got %v", card["type"])
	}
	if card["status"] != "success" {
		t.Fatalf("expected success status, got %v", card["status"])
	}
	if _, hasMessage := card["message"]; hasMessage {
		t.Fatalf("expected no fallback message when details exist, got %v", card["message"])
	}

	details, ok := card["details"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected details list, got %T", card["details"])
	}
	if len(details) != 2 {
		t.Fatalf("expected 2 detail items, got %d", len(details))
	}
	if details[0]["label"] != "foo" || details[0]["value"] != "bar" {
		t.Fatalf("unexpected first detail: %+v", details[0])
	}
	if details[1]["label"] != "token" || details[1]["value"] != "secret" {
		t.Fatalf("unexpected second detail: %+v", details[1])
	}
}

func TestGenericCard_ErrorIsRedacted(t *testing.T) {
	card := GenericCard("test", `{"error":"secret stack trace"}`)
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	if card["message"] != redactedErrorText {
		t.Fatalf("expected redacted error message, got %v", card["message"])
	}
	if card["error_redacted"] != true {
		t.Fatalf("expected error_redacted marker, got %v", card["error_redacted"])
	}
}

func TestExecCard_RedactsSensitiveFields(t *testing.T) {
	card := ToCard("exec", `{"command":"cat ~/.ssh/id_rsa","stdout":"SECRET","stderr":"ERR","exit_code":1,"warnings":["warn"],"session_id":"abc","host":"sandbox","risk_level":"high","duration_ms":8}`)
	if card == nil {
		t.Fatal("expected exec card")
	}
	if card["type"] != "exec" {
		t.Fatalf("expected exec type, got %v", card["type"])
	}
	if card["status"] != "error" {
		t.Fatalf("expected error status, got %v", card["status"])
	}
	if card["message"] != redactedOutputText {
		t.Fatalf("expected redacted output message, got %v", card["message"])
	}
	if card["command_redacted"] != true || card["hide_command"] != true {
		t.Fatalf("expected command redaction markers, got %+v", card)
	}
	if card["stdout_redacted"] != true || card["stderr_redacted"] != true {
		t.Fatalf("expected output redaction markers, got %+v", card)
	}
	if card["warnings_redacted"] != true || card["warning_count"] != 1 {
		t.Fatalf("expected warning redaction markers, got %+v", card)
	}
	if _, ok := card["command"]; ok {
		t.Fatalf("command field should be omitted: %+v", card)
	}
	if _, ok := card["stdout"]; ok {
		t.Fatalf("stdout field should be omitted: %+v", card)
	}
	if _, ok := card["stderr"]; ok {
		t.Fatalf("stderr field should be omitted: %+v", card)
	}
	if _, ok := card["session_id"]; ok {
		t.Fatalf("session_id field should be omitted: %+v", card)
	}
	if _, ok := card["host"]; ok {
		t.Fatalf("host field should be omitted: %+v", card)
	}
	if _, ok := card["warnings"]; ok {
		t.Fatalf("warnings field should be omitted: %+v", card)
	}
}

func TestFormatTypeless_ExecForcesCommandHidden(t *testing.T) {
	calls := []llm.ToolCall{
		{ID: "1", Name: "exec", Arguments: `{"command":"echo secret"}`},
	}
	results := []llm.Message{
		{Role: llm.RoleTool, Content: `{"stdout":"ok","exit_code":0}`, ToolCallID: "1"},
	}

	out := FormatTypeless(calls, results)
	if !strings.Contains(out, `"command_redacted":true`) {
		t.Fatalf("expected command redaction marker, got %q", out)
	}
	if !strings.Contains(out, `"hide_command":true`) {
		t.Fatalf("expected hide_command marker, got %q", out)
	}
	if strings.Contains(out, "echo secret") {
		t.Fatalf("expected exec command to be hidden, got %q", out)
	}
}

func TestRegister_OverridesBuiltin(t *testing.T) {
	Register("calculator", func(content string) map[string]interface{} {
		return map[string]interface{}{"type": "custom", "raw": content}
	})
	defer func() {
		registryMu.Lock()
		delete(registry, "calculator")
		registryMu.Unlock()
	}()

	card := ToCard("calculator", `{"expression":"1+1","result":2}`)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "custom" {
		t.Errorf("expected custom type, got %v", card["type"])
	}
}

func TestRegister_NewTool(t *testing.T) {
	Register("my_tool", func(content string) map[string]interface{} {
		return map[string]interface{}{"type": "my_card"}
	})
	defer func() {
		registryMu.Lock()
		delete(registry, "my_tool")
		registryMu.Unlock()
	}()

	card := ToCard("my_tool", `{}`)
	if card == nil || card["type"] != "my_card" {
		t.Errorf("expected my_card type, got %v", card)
	}
}

func TestUIReviewCard(t *testing.T) {
	content := `{"url":"http://example.com","overall":82.1,"pass":true,"threshold":75,"visual":{"score":82},"functional":{"score":90},"accessibility":{"score":75},"issues":[{"severity":"major","category":"accessibility","description":"Missing alt"}],"steps":[{"id":"navigate","name":"Page Load","status":"success"}]}`
	card := ToCard("ui_reviewer", content)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "ui-review" {
		t.Errorf("expected type ui-review, got %v", card["type"])
	}
	if card["url"] != "http://example.com" {
		t.Errorf("expected url, got %v", card["url"])
	}
	if card["overall"] != 82.1 {
		t.Errorf("expected overall 82.1, got %v", card["overall"])
	}
	if card["pass"] != true {
		t.Errorf("expected pass true, got %v", card["pass"])
	}
	// Steps should be present
	if card["steps"] == nil {
		t.Error("expected steps")
	}
	// Issues should be present
	if card["issues"] == nil {
		t.Error("expected issues")
	}
	// Actions should be present
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) != 3 {
		t.Errorf("expected 3 actions, got %v", card["actions"])
	} else if actions[0]["id"] != "recheck" {
		t.Errorf("expected first action id 'recheck', got %v", actions[0]["id"])
	}
	// Stable card ID from URL
	if card["id"] != "ui-review-http://example.com" {
		t.Errorf("expected card id, got %v", card["id"])
	}
}

func TestUIReviewCard_Error(t *testing.T) {
	content := `{"error":"browser not available"}`
	card := ToCard("ui_reviewer", content)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "ui-review" {
		t.Errorf("expected type ui-review, got %v", card["type"])
	}
	if card["status"] != "error" {
		t.Errorf("expected status error, got %v", card["status"])
	}
	// Error card should have retry action
	actions, ok := card["actions"].([]map[string]interface{})
	if !ok || len(actions) != 1 {
		t.Errorf("expected 1 action on error card, got %v", card["actions"])
	}
}

func TestUIReviewCard_InvalidJSON(t *testing.T) {
	card := ToCard("ui_reviewer", "not json")
	if card != nil {
		t.Errorf("expected nil card for invalid JSON, got %v", card)
	}
}

func TestSandboxCardDispatch(t *testing.T) {
	// Simulate sandbox tool result containing IPC response with _card hint
	content := `{"result":{"stdout":"{\"status\":\"ok\",\"data\":{\"_card\":\"ui_reviewer\",\"result\":\"{\\\"url\\\":\\\"http://x.com\\\",\\\"overall\\\":80,\\\"pass\\\":true}\"}}","stderr":"","exit_code":0},"message":"done"}`
	card := ToCard("sandbox", content)
	if card == nil {
		t.Fatal("expected non-nil card from sandbox dispatch")
	}
	if card["type"] != "ui-review" {
		t.Errorf("expected type ui-review, got %v", card["type"])
	}
}

func TestSandboxCardDispatch_NoHint(t *testing.T) {
	// Sandbox result without _card hint → falls through to GenericCard
	content := `{"result":{"stdout":"hello world","stderr":"","exit_code":0},"message":"done"}`
	card := ToCard("sandbox", content)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card["type"] != "result" {
		t.Errorf("expected generic result type, got %v", card["type"])
	}
}
