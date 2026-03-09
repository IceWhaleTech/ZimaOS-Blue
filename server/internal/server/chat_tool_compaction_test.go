package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

func TestCompactToolResultsForLLM_ExecSearchTrimAndRerank(t *testing.T) {
	results := []map[string]interface{}{
		{"title": "Releases · blueagent/blueagent - GitHub", "url": "https://github.com/blueagent/blueagent/releases", "description": strings.Repeat("release notes ", 30)},
		{"title": "blueagent organization - GitHub", "url": "https://github.com/blueagent", "description": "org page"},
		{"title": "BlueAgent docs", "url": "https://docs.blueagent.ai/", "description": "official docs"},
		{"title": "BlueAgent News", "url": "https://blueagent.report/", "description": "news"},
		{"title": "Community mirror", "url": "https://mirror.example.com/blueagent", "description": "community mirror"},
	}
	rawResults, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("marshal results: %v", err)
	}

	raw := map[string]interface{}{
		"session_id":  "abc123",
		"status":      "completed",
		"exit_code":   0,
		"stdout":      strings.Repeat("line\n", 1200),
		"duration_ms": 5,
		"host":        "local",
		"warnings":    []string{"skill short-circuit: web_search"},
		"risk_level":  "low",
		"truncated":   false,
		"command":     `blue web_search query="BlueAgent GitHub releases"`,
		"stderr":      "",
		"data": map[string]interface{}{
			"_card":      "search",
			"provider":   "duckduckgo",
			"query":      "BlueAgent GitHub releases",
			"results":    string(rawResults),
			"totalCount": "5",
			"success":    "true",
		},
	}
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal exec payload: %v", err)
	}

	toolCalls := []llm.ToolCall{{ID: "call_1", Name: "exec"}}
	toolResults := []llm.Message{{Role: llm.RoleTool, ToolCallID: "call_1", Content: string(contentBytes)}}
	compacted := compactToolResultsForLLM(toolCalls, toolResults)
	if len(compacted) != 1 {
		t.Fatalf("expected 1 compacted result, got %d", len(compacted))
	}
	if len(compacted[0].Content) > maxLLMToolOutputBytes {
		t.Fatalf("compacted tool output too large: %d", len(compacted[0].Content))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted[0].Content), &out); err != nil {
		t.Fatalf("unmarshal compacted exec payload: %v", err)
	}

	data, ok := out["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing compacted data section: %#v", out["data"])
	}
	ranked, ok := data["results"].([]interface{})
	if !ok {
		t.Fatalf("missing compacted results: %#v", data["results"])
	}
	if len(ranked) == 0 || len(ranked) > maxLLMSearchResults {
		t.Fatalf("unexpected result count after rerank: %d", len(ranked))
	}

	seenHosts := map[string]struct{}{}
	for _, item := range ranked {
		m, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("result is not object: %#v", item)
		}
		u, _ := m["url"].(string)
		host := hostKeyForLLM(u)
		if host == "" {
			continue
		}
		if _, exists := seenHosts[host]; exists {
			t.Fatalf("duplicate host kept after rerank: %s", host)
		}
		seenHosts[host] = struct{}{}
	}
}

func TestCompactToolResultsForLLM_WebSearchOutputBounded(t *testing.T) {
	results := make([]interface{}, 0, 10)
	for i := 0; i < 10; i++ {
		results = append(results, map[string]interface{}{
			"title":       "BlueAgent update " + string(rune('A'+i)),
			"url":         "https://site" + string(rune('a'+i)) + ".example.com/blueagent/" + string(rune('0'+i)),
			"description": strings.Repeat("description ", 80),
		})
	}
	raw := map[string]interface{}{
		"query":       "BlueAgent latest news",
		"provider":    "duckduckgo",
		"total_count": 10,
		"results":     results,
	}
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal web_search payload: %v", err)
	}

	toolCalls := []llm.ToolCall{{ID: "call_search", Name: "web_search"}}
	toolResults := []llm.Message{{Role: llm.RoleTool, ToolCallID: "call_search", Content: string(contentBytes)}}
	compacted := compactToolResultsForLLM(toolCalls, toolResults)
	if len(compacted) != 1 {
		t.Fatalf("expected 1 compacted result, got %d", len(compacted))
	}
	if len(compacted[0].Content) > maxLLMToolOutputBytes {
		t.Fatalf("compacted web_search output too large: %d", len(compacted[0].Content))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted[0].Content), &out); err != nil {
		t.Fatalf("unmarshal compacted web_search payload: %v", err)
	}
	ranked, ok := out["results"].([]interface{})
	if !ok {
		t.Fatalf("missing compacted results array")
	}
	if len(ranked) == 0 || len(ranked) > maxLLMSearchResults {
		t.Fatalf("unexpected compacted results count: %d", len(ranked))
	}
	if omitted, ok := out["omitted_results"].(float64); !ok || int(omitted) != 10-len(ranked) {
		t.Fatalf("unexpected omitted_results: %#v", out["omitted_results"])
	}
}

func TestCompactToolResultContentForLLM_NonJSONCapped(t *testing.T) {
	raw := strings.Repeat("x", 32*1024)
	compacted := compactToolResultContentForLLM("exec", raw)
	if len(compacted) > maxLLMToolOutputBytes {
		t.Fatalf("expected capped non-json output <= %d, got %d", maxLLMToolOutputBytes, len(compacted))
	}
	if !strings.Contains(compacted, "[truncated]") {
		t.Fatalf("expected truncated marker in compacted output")
	}
}

func TestCompactToolResultContentForLLM_ExecRedactsSensitiveFields(t *testing.T) {
	raw := map[string]interface{}{
		"status":     "failed",
		"exit_code":  1,
		"command":    "cat ~/.ssh/id_rsa",
		"stdout":     "SECRET_STDOUT",
		"stderr":     "SECRET_STDERR",
		"error":      "SECRET_ERROR",
		"session_id": "sess-123",
		"host":       "sandbox",
		"warnings":   []interface{}{"warn1", "warn2"},
		"data": map[string]interface{}{
			"provider":      "duckduckgo",
			"query":         "BlueAgent",
			"message":       "sensitive internal message",
			"error":         "sensitive internal error",
			"secret_blob":   "should-not-leak",
			"debug_payload": map[string]interface{}{"token": "abc"},
		},
	}
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal exec payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("exec", string(contentBytes))
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted payload: %v", err)
	}

	for _, forbidden := range []string{"stdout", "stderr", "command", "session_id", "host", "warnings", "error"} {
		if _, ok := out[forbidden]; ok {
			t.Fatalf("expected %s to be redacted from LLM payload: %#v", forbidden, out)
		}
	}
	if out["output_redacted"] != true || out["stdout_redacted"] != true || out["stderr_redacted"] != true {
		t.Fatalf("expected output redaction markers, got %#v", out)
	}
	if out["command_redacted"] != true || out["runtime_redacted"] != true || out["error_redacted"] != true {
		t.Fatalf("expected exec redaction markers, got %#v", out)
	}
	if out["warning_count"] != float64(2) {
		t.Fatalf("expected warning_count=2, got %#v", out["warning_count"])
	}

	data, ok := out["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected compacted data map, got %#v", out["data"])
	}
	if data["message_redacted"] != true || data["error_redacted"] != true {
		t.Fatalf("expected nested data redaction markers, got %#v", data)
	}
	if _, ok := data["message"]; ok {
		t.Fatalf("expected nested message removed, got %#v", data)
	}
	if _, ok := data["error"]; ok {
		t.Fatalf("expected nested error removed, got %#v", data)
	}
	if _, ok := data["secret_blob"]; ok {
		t.Fatalf("expected unknown nested fields redacted, got %#v", data)
	}
	if _, ok := data["debug_payload"]; ok {
		t.Fatalf("expected unknown nested fields redacted, got %#v", data)
	}
	if data["extra_fields_redacted"] != float64(2) {
		t.Fatalf("expected extra_fields_redacted=2, got %#v", data["extra_fields_redacted"])
	}
}

func TestCompactToolResultsForLLM_BoundsResponsesBodySize(t *testing.T) {
	results := make([]map[string]interface{}, 0, 10)
	for i := 0; i < 10; i++ {
		results = append(results, map[string]interface{}{
			"title":       "BlueAgent item " + string(rune('A'+i)),
			"url":         "https://site" + string(rune('a'+i)) + ".example.com/blueagent",
			"description": strings.Repeat("long text ", 200),
		})
	}
	rawResults, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("marshal results: %v", err)
	}
	rawExec := map[string]interface{}{
		"status":    "completed",
		"exit_code": 0,
		"stdout":    strings.Repeat("output line\n", 2500),
		"data": map[string]interface{}{
			"_card":      "search",
			"provider":   "duckduckgo",
			"query":      "BlueAgent latest news",
			"results":    string(rawResults),
			"totalCount": "10",
			"success":    "true",
		},
	}
	rawExecBytes, err := json.Marshal(rawExec)
	if err != nil {
		t.Fatalf("marshal exec payload: %v", err)
	}

	toolCalls := []llm.ToolCall{
		{ID: "call_1", Name: "exec"},
		{ID: "call_2", Name: "exec"},
		{ID: "call_3", Name: "exec"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call_1", Content: string(rawExecBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_2", Content: string(rawExecBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_3", Content: string(rawExecBytes)},
	}
	compacted := compactToolResultsForLLM(toolCalls, toolResults)

	req := llm.ChatRequest{
		Model: "gpt-5.3-codex",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "test"},
			{Role: llm.RoleAssistant, ToolCalls: toolCalls, Content: ""},
		},
		Stream: true,
	}
	req.Messages = append(req.Messages, compacted...)
	body, err := proxybridge.MarshalResponsesRequest(req)
	if err != nil {
		t.Fatalf("marshal responses request: %v", err)
	}
	if len(body) > 28*1024 {
		t.Fatalf("responses request body too large after compaction: %d", len(body))
	}
}

func TestCompactToolResultsForLLM_EnforcesTotalBudget(t *testing.T) {
	toolCalls := []llm.ToolCall{
		{ID: "call_1", Name: "exec"},
		{ID: "call_2", Name: "exec"},
		{ID: "call_3", Name: "exec"},
		{ID: "call_4", Name: "exec"},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call_1", Content: `{"stdout":"` + strings.Repeat("A", 12000) + `"}`},
		{Role: llm.RoleTool, ToolCallID: "call_2", Content: `{"stdout":"` + strings.Repeat("B", 12000) + `"}`},
		{Role: llm.RoleTool, ToolCallID: "call_3", Content: `{"stdout":"` + strings.Repeat("C", 12000) + `"}`},
		{Role: llm.RoleTool, ToolCallID: "call_4", Content: `{"stdout":"` + strings.Repeat("D", 12000) + `"}`},
	}

	compacted := compactToolResultsForLLM(toolCalls, toolResults)
	if len(compacted) != 4 {
		t.Fatalf("expected 4 compacted messages, got %d", len(compacted))
	}
	total := 0
	for _, m := range compacted {
		total += len(m.Content)
	}
	if total > maxLLMToolOutputsTotalBytes {
		t.Fatalf("total compacted tool bytes = %d, want <= %d", total, maxLLMToolOutputsTotalBytes)
	}
}

func TestCompactToolResultsForLLM_KeepsMultipleSearchPayloadsAndCompactsLaterRounds(t *testing.T) {
	searchData := map[string]interface{}{
		"_card":      "search",
		"provider":   "duckduckgo",
		"query":      "BlueAgent latest news",
		"results":    `[{"title":"BlueAgent","url":"https://blueagent.ai","description":"latest"}]`,
		"totalCount": "1",
		"success":    "true",
	}
	rawExec := map[string]interface{}{
		"status":    "completed",
		"exit_code": 0,
		"data":      searchData,
	}
	rawExecBytes, err := json.Marshal(rawExec)
	if err != nil {
		t.Fatalf("marshal exec payload: %v", err)
	}

	toolCalls := []llm.ToolCall{
		{ID: "call_1", Name: "exec", Arguments: `{"command":"blue web_search query=\"BlueAgent latest news\""}`},
		{ID: "call_2", Name: "exec", Arguments: `{"command":"blue web_search query=\"BlueAgent GitHub releases\""}`},
		{ID: "call_3", Name: "exec", Arguments: `{"command":"blue web_search query=\"BlueAgent docs\""}`},
		{ID: "call_4", Name: "exec", Arguments: `{"command":"blue web_search query=\"BlueAgent changelog\""}`},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call_1", Content: string(rawExecBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_2", Content: string(rawExecBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_3", Content: string(rawExecBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_4", Content: string(rawExecBytes)},
	}

	compacted := compactToolResultsForLLM(toolCalls, toolResults)
	if len(compacted) != 4 {
		t.Fatalf("expected 4 compacted messages, got %d", len(compacted))
	}

	for i := 0; i < maxLLMSearchRoundsKeepFull; i++ {
		var full map[string]interface{}
		if err := json.Unmarshal([]byte(compacted[i].Content), &full); err != nil {
			t.Fatalf("unmarshal compacted payload %d: %v", i, err)
		}
		data, ok := full["data"].(map[string]interface{})
		if !ok {
			t.Fatalf("payload %d missing data block: %#v", i, full)
		}
		if _, ok := data["results"]; !ok {
			t.Fatalf("payload %d should keep search results: %#v", i, data)
		}
	}

	for i := maxLLMSearchRoundsKeepFull; i < len(compacted); i++ {
		var summary map[string]interface{}
		if err := json.Unmarshal([]byte(compacted[i].Content), &summary); err != nil {
			t.Fatalf("unmarshal compacted summary %d: %v", i, err)
		}
		if omitted, ok := summary["omitted_from_llm_context"].(bool); !ok || !omitted {
			t.Fatalf("message %d expected omitted_from_llm_context=true, got %#v", i, summary["omitted_from_llm_context"])
		}
		if _, ok := summary["results"]; !ok {
			t.Fatalf("message %d expected compacted result preview, got %#v", i, summary)
		}
	}
}
