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
		{"title": "Releases · openclaw/openclaw - GitHub", "url": "https://github.com/openclaw/openclaw/releases", "description": strings.Repeat("release notes ", 30)},
		{"title": "openclaw organization - GitHub", "url": "https://github.com/openclaw", "description": "org page"},
		{"title": "OpenClaw docs", "url": "https://docs.openclaw.ai/", "description": "official docs"},
		{"title": "OpenClaw News", "url": "https://openclaw.report/", "description": "news"},
		{"title": "Community mirror", "url": "https://mirror.example.com/openclaw", "description": "community mirror"},
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
		"command":     `blue web_search query="OpenClaw GitHub releases"`,
		"stderr":      "",
		"data": map[string]interface{}{
			"_card":      "search",
			"provider":   "duckduckgo",
			"query":      "OpenClaw GitHub releases",
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
			"title":       "OpenClaw update " + string(rune('A'+i)),
			"url":         "https://site" + string(rune('a'+i)) + ".example.com/openclaw/" + string(rune('0'+i)),
			"description": strings.Repeat("description ", 80),
		})
	}
	raw := map[string]interface{}{
		"query":       "OpenClaw latest news",
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

func TestCompactToolResultsForLLM_BoundsResponsesBodySize(t *testing.T) {
	results := make([]map[string]interface{}, 0, 10)
	for i := 0; i < 10; i++ {
		results = append(results, map[string]interface{}{
			"title":       "OpenClaw item " + string(rune('A'+i)),
			"url":         "https://site" + string(rune('a'+i)) + ".example.com/openclaw",
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
			"query":      "OpenClaw latest news",
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
		"query":      "OpenClaw latest news",
		"results":    `[{"title":"OpenClaw","url":"https://openclaw.ai","description":"latest"}]`,
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
		{ID: "call_1", Name: "exec", Arguments: `{"command":"blue web_search query=\"OpenClaw latest news\""}`},
		{ID: "call_2", Name: "exec", Arguments: `{"command":"blue web_search query=\"OpenClaw GitHub releases\""}`},
		{ID: "call_3", Name: "exec", Arguments: `{"command":"blue web_search query=\"OpenClaw docs\""}`},
		{ID: "call_4", Name: "exec", Arguments: `{"command":"blue web_search query=\"OpenClaw changelog\""}`},
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
