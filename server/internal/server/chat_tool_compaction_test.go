package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

func buildWebQueryToolPayloadForLLMTests(content string) map[string]interface{} {
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
			map[string]interface{}{
				"rank":          3,
				"title":         "Blue Status",
				"url":           "https://status.example.net/incidents/2026-04-04",
				"final_url":     "https://status.example.net/incidents/2026-04-04",
				"snippet":       "Status update published on 2026-04-04 for the same release window.",
				"source":        "newswire",
				"content_chars": 420,
				"selected":      false,
			},
		},
		"diagnostics": map[string]interface{}{
			"route":           "search_http",
			"candidate_count": 3,
			"selected_source": 1,
			"degraded":        false,
		},
	}
}

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

func TestCompactAssistantToolContextForLLM_FileWriteSummarizesContent(t *testing.T) {
	msg := llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{{
			ID:   "call_write_1",
			Name: "file_write",
			Arguments: `{"path":"reports/final.md","content":"alpha\nbeta\n[truncated]","append":true}`,
		}},
	}

	compacted := compactAssistantToolContextForLLM(msg)
	if len(compacted.ToolCalls) != 1 {
		t.Fatalf("tool call count = %d, want 1", len(compacted.ToolCalls))
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(compacted.ToolCalls[0].Arguments), &args); err != nil {
		t.Fatalf("unmarshal compacted file_write args: %v", err)
	}
	if _, ok := args["content"]; ok {
		t.Fatalf("expected compacted file_write args to omit raw content: %#v", args)
	}
	if got := args["path"]; got != "reports/final.md" {
		t.Fatalf("path = %#v, want reports/final.md", got)
	}
	if omitted, _ := args["content_omitted"].(bool); !omitted {
		t.Fatalf("content_omitted = %#v, want true", args["content_omitted"])
	}
	if suspicious, _ := args["content_truncated_marker"].(bool); !suspicious {
		t.Fatalf("content_truncated_marker = %#v, want true", args["content_truncated_marker"])
	}
}

func TestCompactContinuationRecoveryMessage_FileWriteSummarizesContent(t *testing.T) {
	msg := llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{{
			ID:   "call_write_1",
			Name: "file_write",
			Arguments: fmt.Sprintf(`{"path":"reports/final.md","content":%q}`, strings.Repeat("payload\n", 600)+"[truncated]"),
		}},
	}

	compacted := compactContinuationRecoveryMessage(msg)
	if len(compacted.ToolCalls) != 1 {
		t.Fatalf("tool call count = %d, want 1", len(compacted.ToolCalls))
	}
	if strings.Contains(compacted.ToolCalls[0].Arguments, `"content"`) {
		t.Fatalf("expected continuation recovery to omit raw file content, got %s", compacted.ToolCalls[0].Arguments)
	}
	if strings.Contains(compacted.ToolCalls[0].Arguments, "\n[truncated]") {
		t.Fatalf("expected continuation recovery to avoid raw truncation marker in args, got %s", compacted.ToolCalls[0].Arguments)
	}

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(compacted.ToolCalls[0].Arguments), &args); err != nil {
		t.Fatalf("unmarshal recovery compacted file_write args: %v", err)
	}
	if suspicious, _ := args["content_truncated_marker"].(bool); !suspicious {
		t.Fatalf("content_truncated_marker = %#v, want true", args["content_truncated_marker"])
	}
}

func TestCompactToolResultContentForLLM_FileReadKeepsLargerContentBudget(t *testing.T) {
	lines := make([]string, 0, 240)
	for i := 1; i <= 240; i++ {
		lines = append(lines, fmt.Sprintf("line %03d %s", i, strings.Repeat("payload ", 2)))
	}
	raw := map[string]interface{}{
		"path":        "notes/output.txt",
		"size":        4096,
		"start_line":  1,
		"end_line":    len(lines),
		"total_lines": len(lines),
		"truncated":   false,
		"content":     strings.Join(lines, "\n"),
	}
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal file_read payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("file_read", string(contentBytes))
	if len(compacted) > maxLLMToolOutputBytes {
		t.Fatalf("compacted file_read output too large: %d", len(compacted))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted file_read payload: %v", err)
	}
	content, _ := out["content"].(string)
	if len(content) <= 512 {
		t.Fatalf("expected file_read content budget > 512 bytes, got %d", len(content))
	}
	if !strings.Contains(content, "line 120 payload payload ") {
		t.Fatalf("expected compacted file_read content to preserve mid-file lines beyond 512 bytes, got=%q", content)
	}
	if truncated, _ := out["truncated"].(bool); truncated {
		t.Fatalf("expected truncated=false when file_read content fits LLM budget, got true")
	}
}

func TestCompactToolResultContentForLLM_FileReadMarksTruncatedWhenLLMCompactsContent(t *testing.T) {
	rawContent := strings.Repeat("alpha beta gamma delta epsilon zeta eta theta iota kappa\n", 260)
	raw := map[string]interface{}{
		"path":        "notes/output.txt",
		"size":        len(rawContent),
		"start_line":  1,
		"end_line":    260,
		"total_lines": 260,
		"truncated":   false,
		"content":     rawContent,
	}
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal file_read payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("file_read", string(contentBytes))
	if len(compacted) > maxLLMToolOutputBytes {
		t.Fatalf("compacted file_read output too large: %d", len(compacted))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted file_read payload: %v", err)
	}
	content, _ := out["content"].(string)
	if len(content) >= len(rawContent) {
		t.Fatalf("expected file_read content to be compacted for LLM, got len=%d want < %d", len(content), len(rawContent))
	}
	if !strings.Contains(content, "[truncated]") {
		t.Fatalf("expected compacted file_read content to include truncated marker, got=%q", content)
	}
	if truncated, _ := out["truncated"].(bool); !truncated {
		t.Fatalf("expected truncated=true when LLM compacts file_read content, got %#v", out["truncated"])
	}
}

func TestCompactToolResultContentForLLM_WebQueryPreservesSelectedResultAndFacts(t *testing.T) {
	raw := buildWebQueryToolPayloadForLLMTests(strings.Repeat("On April 4, 2026, version 1.2.3 shipped 12 improvements, 4 fixes, and 2 migrations. ", 180))
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal web_query payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("web_query", string(contentBytes))
	if len(compacted) > maxLLMToolOutputBytes {
		t.Fatalf("compacted web_query output too large: %d", len(compacted))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted web_query payload: %v", err)
	}
	if got, ok := out["has_results"].(bool); !ok || !got {
		t.Fatalf("has_results = %#v, want true", out["has_results"])
	}
	selected, ok := out["selected_result"].(map[string]interface{})
	if !ok {
		t.Fatalf("selected_result = %#v, want object", out["selected_result"])
	}
	if got := selected["url"]; got != "https://docs.example.com/releases/1.2.3" {
		t.Fatalf("selected_result.url = %v, want selected page", got)
	}
	facts, ok := out["key_facts"].([]interface{})
	if !ok || len(facts) == 0 {
		t.Fatalf("key_facts = %#v, want non-empty array", out["key_facts"])
	}
	joinedFacts := make([]string, 0, len(facts))
	for _, fact := range facts {
		joinedFacts = append(joinedFacts, fmt.Sprint(fact))
	}
	if got := strings.Join(joinedFacts, " "); !strings.Contains(got, "2026") && !strings.Contains(got, "1.2.3") {
		t.Fatalf("key_facts = %q, want date/version evidence", got)
	}
	if got, ok := out["llm_compacted"].(bool); !ok || !got {
		t.Fatalf("llm_compacted = %#v, want true", out["llm_compacted"])
	}
}

func TestCompactToolResultContentForLLM_ExecKeepsCoreFields(t *testing.T) {
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

	for _, required := range []string{"stdout", "stderr", "command", "session_id", "host", "error"} {
		v, ok := out[required].(string)
		if !ok || strings.TrimSpace(v) == "" {
			t.Fatalf("expected %s to remain visible in compacted payload: %#v", required, out)
		}
	}
	for _, removedMarker := range []string{
		"output_redacted", "stdout_redacted", "stderr_redacted",
		"command_redacted", "runtime_redacted", "error_redacted",
	} {
		if _, exists := out[removedMarker]; exists {
			t.Fatalf("did not expect legacy redaction marker %s: %#v", removedMarker, out)
		}
	}
	if out["warning_count"] != float64(2) {
		t.Fatalf("expected warning_count=2, got %#v", out["warning_count"])
	}
	if _, ok := out["warnings"]; !ok {
		t.Fatalf("expected warnings array preserved, got %#v", out)
	}

	data, ok := out["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected compacted data map, got %#v", out["data"])
	}
	if msg, _ := data["message"].(string); strings.TrimSpace(msg) == "" {
		t.Fatalf("expected nested message preserved, got %#v", data)
	}
	if errMsg, _ := data["error"].(string); strings.TrimSpace(errMsg) == "" {
		t.Fatalf("expected nested error preserved, got %#v", data)
	}
	extra, ok := data["extra"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested extra map, got %#v", data["extra"])
	}
	if _, ok := extra["secret_blob"]; !ok {
		t.Fatalf("expected secret_blob in extra fields, got %#v", extra)
	}
	if _, ok := extra["debug_payload"]; !ok {
		t.Fatalf("expected debug_payload in extra fields, got %#v", extra)
	}
	if _, exists := data["extra_fields_redacted"]; exists {
		t.Fatalf("did not expect legacy extra_fields_redacted marker, got %#v", data)
	}
}

func TestCompactToolResultContentForLLM_PDFKeepsDistributedPageExcerpts(t *testing.T) {
	payload := map[string]interface{}{
		"document": map[string]interface{}{
			"path":       "/tmp/workspace/openclaw_report.pdf",
			"file_name":  "openclaw_report.pdf",
			"page_count": 6,
		},
		"selected_pages": []interface{}{1, 2, 3, 4, 5, 6},
		"char_count":     24000,
		"text": strings.Join([]string{
			"[Page 1]\nExecutive summary and overview " + strings.Repeat("alpha ", 80),
			"[Page 2]\nSkill registry methodology " + strings.Repeat("beta ", 80),
			"[Page 3]\nLargest category is AI & LLMs: 287 " + strings.Repeat("gamma ", 80),
			"[Page 4]\nSecond category is Search & Research: 253 " + strings.Repeat("delta ", 80),
			"[Page 5]\nGateway exposes a typed WebSocket API " + strings.Repeat("epsilon ", 80),
			"[Page 6]\nRegistry collected on February 7, 2026 and proposes 6 benchmark tasks " + strings.Repeat("zeta ", 80),
		}, "\n\n"),
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pdf payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("pdf", string(contentBytes))
	if len(compacted) > maxLLMToolOutputBytes {
		t.Fatalf("compacted pdf output too large: %d", len(compacted))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted pdf payload: %v", err)
	}
	text, _ := out["text"].(string)
	for _, needle := range []string{"[Page 1]", "[Page 3]", "[Page 6]", "AI & LLMs: 287", "February 7, 2026"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected compacted pdf text to keep %q, got=%q", needle, text)
		}
	}
}

func TestCompactToolResultContentForLLM_PDFKeepsAllCompactPagesWhenAvailable(t *testing.T) {
	pages := make([]interface{}, 0, 6)
	for i := 1; i <= 6; i++ {
		pages = append(pages, map[string]interface{}{
			"number": i,
			"text":   fmt.Sprintf("page %d details %s", i, strings.Repeat(string(rune('a'+i-1)), 160)),
		})
	}
	payload := map[string]interface{}{
		"document": map[string]interface{}{
			"path":       "/tmp/workspace/openclaw_report.pdf",
			"file_name":  "openclaw_report.pdf",
			"page_count": 6,
		},
		"selected_pages": []interface{}{1, 2, 3, 4, 5, 6},
		"char_count":     9000,
		"text":           "merged text that should be omitted when compact pages are present",
		"pages":          pages,
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pdf payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("pdf", string(contentBytes))
	if len(compacted) > maxLLMToolOutputBytes {
		t.Fatalf("compacted pdf output too large: %d", len(compacted))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted pdf payload: %v", err)
	}
	if _, exists := out["text"]; exists {
		t.Fatalf("expected compacted pdf to omit merged text when compact pages are available: %#v", out)
	}
	compactedPages, ok := out["pages"].([]interface{})
	if !ok || len(compactedPages) != 6 {
		t.Fatalf("pages = %#v, want 6 entries", out["pages"])
	}
	lastPage, ok := compactedPages[5].(map[string]interface{})
	if !ok {
		t.Fatalf("last page = %#v", compactedPages[5])
	}
	if got := lastPage["number"]; got != float64(6) {
		t.Fatalf("last page number = %v, want 6", got)
	}
	if text := anyToStringForLLM(lastPage["text"]); !strings.Contains(text, "page 6 details") {
		t.Fatalf("last page text = %q, want page 6 details", text)
	}
}

func TestCompactToolResultContentForLLM_PDFSynthesizesPagesFromMarkdownWhenPagesMissing(t *testing.T) {
	payload := map[string]interface{}{
		"document": map[string]interface{}{
			"path":       "/tmp/workspace/openclaw_report.pdf",
			"file_name":  "openclaw_report.pdf",
			"page_count": 11,
		},
		"outline": []interface{}{
			map[string]interface{}{"title": "Recommended new tasks and task modifications for PinchBench", "level": 2, "page_number": 6},
			map[string]interface{}{"title": "Proposed tasks", "level": 3, "page_number": 6},
			map[string]interface{}{"title": "Secure skill installation and safe configuration", "level": 4, "page_number": 6},
			map[string]interface{}{"title": "Browser automation with \"no API\" constraints and recovery", "level": 4, "page_number": 7},
			map[string]interface{}{"title": "Scheduled daily briefing with data fusion + memory write-back", "level": 4, "page_number": 8},
			map[string]interface{}{"title": "Prompt-injection and tool-blast-radius containment", "level": 4, "page_number": 9},
		},
		"markdown": strings.Join([]string{
			"[Page 1]\nThe report mentions your 10 existing benchmark tasks and an execution summary.",
			"[Page 2]\nRegistry methodology and filtering notes.",
			"[Page 3]\nLargest category is AI & LLMs: 287.",
			"[Page 4]\nSecond category is Search & Research: 253.",
			"[Page 5]\nThe OpenClaw gateway exposes a typed WebSocket API.",
			"[Page 6]\n## Proposed tasks\n\n#### Secure skill installation and safe configuration",
			"[Page 7]\n#### Browser automation with \"no API\" constraints and recovery\n\n#### Multi-channel routing and session isolation",
			"[Page 8]\n#### Scheduled daily briefing with data fusion + memory write-back\n\n#### PR review and repair loop with CI feedback",
			"[Page 9]\n#### Prompt-injection and tool-blast-radius containment",
			"[Page 10]\nPrioritized roadmap and visualization requests.",
			"[Page 11]\nReferences and source links.",
		}, "\n\n"),
		"raw_text": strings.Join([]string{
			"[Page 1]\nnoisy raw text that should not crowd out structured pages",
			"[Page 2]\nmore noisy raw text",
		}, "\n\n"),
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pdf payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("pdf", string(contentBytes))
	if len(compacted) > maxLLMToolOutputBytes {
		t.Fatalf("compacted pdf output too large: %d", len(compacted))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted pdf payload: %v", err)
	}
	if _, exists := out["raw_text"]; exists {
		t.Fatalf("expected raw_text omitted once structured/synthetic page samples are present: %#v", out)
	}

	pages, ok := out["pages"].([]interface{})
	if !ok || len(pages) != 11 {
		t.Fatalf("pages = %#v, want 11 entries", out["pages"])
	}
	pageSnippets := map[int]string{}
	for _, item := range pages {
		row, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("page item = %#v", item)
		}
		pageNumber := int(row["number"].(float64))
		pageSnippets[pageNumber] = anyToStringForLLM(row["markdown"])
	}
	for pageNumber, needle := range map[int]string{
		6: "## Proposed tasks",
		7: "Browser automation with \"no API\" constraints and recovery",
		8: "Scheduled daily briefing with data fusion + memory write-back",
		9: "Prompt-injection and tool-blast-radius containment",
	} {
		if !strings.Contains(pageSnippets[pageNumber], needle) {
			t.Fatalf("page %d markdown = %q, want to contain %q", pageNumber, pageSnippets[pageNumber], needle)
		}
	}
}

func TestCompactToolResultContentForLLM_PDFCreateKeepsArtifactMetadata(t *testing.T) {
	payload := map[string]interface{}{
		"action": "create",
		"path":   "reports/direct_markdown.pdf",
		"format": "pdf",
		"engine": "native_pdf_ir",
		"validation": map[string]interface{}{
			"ok":         true,
			"readable":   true,
			"page_count": 1,
		},
		"size":    1234,
		"success": true,
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pdf create payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("pdf", string(contentBytes))
	if len(compacted) > maxLLMToolOutputBytes {
		t.Fatalf("compacted pdf output too large: %d", len(compacted))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted pdf create payload: %v", err)
	}
	doc, ok := out["document"].(map[string]interface{})
	if !ok {
		t.Fatalf("document = %#v, want object", out["document"])
	}
	if got := anyToStringForLLM(doc["path"]); got != "reports/direct_markdown.pdf" {
		t.Fatalf("document.path = %q, want reports/direct_markdown.pdf", got)
	}
	if got := anyToStringForLLM(doc["engine"]); got != "native_pdf_ir" {
		t.Fatalf("document.engine = %q, want native_pdf_ir", got)
	}
	if got := anyToIntForLLM(doc["page_count"]); got != 1 {
		t.Fatalf("document.page_count = %d, want 1", got)
	}
	if got := anyToIntForLLM(doc["size_bytes"]); got != 1234 {
		t.Fatalf("document.size_bytes = %d, want 1234", got)
	}
}

func TestCompactToolResultContentForLLM_PDFKeepsStructuredMarkdownOutlineBlocksAndTables(t *testing.T) {
	payload := map[string]interface{}{
		"document": map[string]interface{}{
			"path":       "/tmp/workspace/structured_report.pdf",
			"file_name":  "structured_report.pdf",
			"page_count": 2,
		},
		"selected_pages": []interface{}{1, 2},
		"markdown":       "[Page 1]\n\n# Executive Summary\n\n- First finding\n\n[Page 2]\n\n## Metrics\n\n| Name | Count |\n| --- | --- |\n| AI & LLMs | 287 |",
		"outline": []interface{}{
			map[string]interface{}{"title": "Executive Summary", "level": 1, "page_number": 1},
			map[string]interface{}{"title": "Metrics", "level": 2, "page_number": 2},
		},
		"pages": []interface{}{
			map[string]interface{}{
				"number":   1,
				"markdown": "# Executive Summary\n\n- First finding",
				"blocks": []interface{}{
					map[string]interface{}{"kind": "heading", "heading_level": 1, "markdown": "# Executive Summary"},
					map[string]interface{}{"kind": "list", "markdown": "- First finding", "children": []interface{}{map[string]interface{}{"kind": "list_item", "text": "First finding"}}},
				},
			},
			map[string]interface{}{
				"number":   2,
				"markdown": "## Metrics\n\n| Name | Count |\n| --- | --- |\n| AI & LLMs | 287 |",
				"tables": []interface{}{
					map[string]interface{}{
						"number_of_rows":    2,
						"number_of_columns": 2,
						"markdown":          "| Name | Count |\n| --- | --- |\n| AI & LLMs | 287 |",
					},
				},
			},
		},
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pdf payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("pdf", string(contentBytes))
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted pdf payload: %v", err)
	}
	if markdown := anyToStringForLLM(out["markdown"]); !strings.Contains(markdown, "Executive Summary") {
		t.Fatalf("expected compacted markdown to survive, got=%q", markdown)
	}
	outline, ok := out["outline"].([]interface{})
	if !ok || len(outline) != 2 {
		t.Fatalf("outline = %#v, want 2 entries", out["outline"])
	}
	pages, ok := out["pages"].([]interface{})
	if !ok || len(pages) != 2 {
		t.Fatalf("pages = %#v, want 2 entries", out["pages"])
	}
	firstPage, ok := pages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first page = %#v", pages[0])
	}
	if _, ok := firstPage["blocks"].([]interface{}); !ok {
		t.Fatalf("expected compacted blocks on first page, got=%#v", firstPage)
	}
	secondPage, ok := pages[1].(map[string]interface{})
	if !ok {
		t.Fatalf("second page = %#v", pages[1])
	}
	if _, ok := secondPage["tables"].([]interface{}); !ok {
		t.Fatalf("expected compacted tables on second page, got=%#v", secondPage)
	}
}

func TestContentForChatToolHistory_CompactsPDFAuditPayloadForLLM(t *testing.T) {
	compact := `{"document":{"path":"openclaw_report.pdf"},"pages":[{"number":1,"text":"sample"}]}`
	raw := `{"document":{"path":"openclaw_report.pdf"},"outline":[{"title":"Executive Summary","level":1,"page_number":1}],"markdown":"[Page 1]\nfull raw text","pages":[{"number":1,"text":"sample"}],"raw_text":"[Page 1]\nfull raw text"}`
	want := compactToolResultContentForLLM("pdf", raw)
	if got := contentForChatToolHistory(context.Background(), "pdf", "", compact, raw); got != want {
		t.Fatalf("contentForChatToolHistory() = %q, want compacted payload %q", got, want)
	}
}

func TestContentForChatToolHistory_CompactsFileReadAuditPayloadForPDF(t *testing.T) {
	compact := `{"document":{"path":"openclaw_report.pdf"},"pages":[{"number":1,"text":"sample"}]}`
	raw := `{"document":{"path":"openclaw_report.pdf"},"outline":[{"title":"Executive Summary","level":1,"page_number":1}],"markdown":"[Page 1]\nfull raw text","pages":[{"number":1,"text":"sample"}],"raw_text":"[Page 1]\nfull raw text"}`
	want := compactToolResultContentForLLM("read", raw)
	if got := contentForChatToolHistory(context.Background(), "read", "", compact, raw); got != want {
		t.Fatalf("contentForChatToolHistory() = %q, want compacted payload %q", got, want)
	}
}

func TestCompactToolResultContentForLLM_PDFOrdersOutlineAndPagesBeforeMarkdown(t *testing.T) {
	payload := map[string]interface{}{
		"document": map[string]interface{}{
			"path":       "/tmp/workspace/openclaw_report.pdf",
			"file_name":  "openclaw_report.pdf",
			"page_count": 10,
		},
		"outline": []interface{}{
			map[string]interface{}{"title": "Proposed tasks", "level": 2, "page_number": 6},
			map[string]interface{}{"title": "Prompt-injection and tool-blast-radius containment", "level": 4, "page_number": 9},
		},
		"pages": []interface{}{
			map[string]interface{}{"number": 6, "markdown": "## Proposed tasks"},
			map[string]interface{}{"number": 9, "markdown": "#### Prompt-injection and tool-blast-radius containment"},
		},
		"markdown": "[Page 6]\n\n## Proposed tasks\n\nnoisy body " + strings.Repeat("alpha ", 120),
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pdf payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("pdf", string(contentBytes))
	if outlineHierarchyIdx := strings.Index(compacted, `"outline_hierarchy"`); outlineHierarchyIdx < 0 {
		t.Fatalf("expected outline_hierarchy in compacted payload: %q", compacted)
	} else if outlineIdx := strings.Index(compacted, `"outline"`); outlineIdx < 0 {
		t.Fatalf("expected outline in compacted payload: %q", compacted)
	} else if pagesIdx := strings.Index(compacted, `"pages"`); pagesIdx < 0 {
		t.Fatalf("expected pages in compacted payload: %q", compacted)
	} else if markdownIdx := strings.Index(compacted, `"markdown"`); markdownIdx < 0 {
		t.Fatalf("expected markdown in compacted payload: %q", compacted)
	} else {
		if outlineHierarchyIdx > markdownIdx {
			t.Fatalf("expected outline_hierarchy before markdown, got=%q", compacted)
		}
		if outlineIdx > markdownIdx {
			t.Fatalf("expected outline before markdown, got=%q", compacted)
		}
		if pagesIdx > markdownIdx {
			t.Fatalf("expected pages before markdown, got=%q", compacted)
		}
	}
}

func TestCompactToolResultContentForLLM_PDFKeepsAllProposedTaskOutlineEntries(t *testing.T) {
	outline := make([]interface{}, 0, 27)
	for i := 1; i <= 20; i++ {
		outline = append(outline, map[string]interface{}{
			"title":       fmt.Sprintf("Section %02d", i),
			"level":       2,
			"page_number": i,
		})
	}
	outline = append(outline, map[string]interface{}{
		"title":       "Proposed tasks",
		"level":       3,
		"page_number": 6,
	})
	for _, title := range []string{
		"Secure skill installation and safe configuration",
		"Browser automation with \"no API\" constraints and recovery",
		"Multi-channel routing and session isolation",
		"Scheduled daily briefing with data fusion + memory write-back",
		"PR review and repair loop with CI feedback",
		"Prompt-injection and tool-blast-radius containment",
	} {
		outline = append(outline, map[string]interface{}{
			"title":       title,
			"level":       4,
			"page_number": 6,
		})
	}

	payload := map[string]interface{}{
		"document": map[string]interface{}{
			"path":       "/tmp/workspace/openclaw_report.pdf",
			"file_name":  "openclaw_report.pdf",
			"page_count": 11,
		},
		"outline": outline,
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pdf payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("pdf", string(contentBytes))
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted pdf payload: %v", err)
	}
	rows, ok := out["outline"].([]interface{})
	if !ok {
		t.Fatalf("outline = %#v", out["outline"])
	}
	hierarchyRows, ok := out["outline_hierarchy"].([]interface{})
	if !ok || len(hierarchyRows) == 0 {
		t.Fatalf("outline_hierarchy = %#v", out["outline_hierarchy"])
	}
	found := 0
	parentFound := false
	for _, item := range rows {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		title, _ := row["title"].(string)
		if title == "Proposed tasks" {
			parentFound = true
			if got := anyToIntForLLM(row["child_count"]); got != 6 {
				t.Fatalf("Proposed tasks child_count = %d, want 6; compacted=%q", got, compacted)
			}
		}
		switch title {
		case "Secure skill installation and safe configuration",
			"Browser automation with \"no API\" constraints and recovery",
			"Multi-channel routing and session isolation",
			"Scheduled daily briefing with data fusion + memory write-back",
			"PR review and repair loop with CI feedback",
			"Prompt-injection and tool-blast-radius containment":
			found++
		}
	}
	if !parentFound {
		t.Fatalf("expected Proposed tasks parent entry in compacted outline; compacted=%q", compacted)
	}
	if found != 6 {
		t.Fatalf("found %d proposed task outline entries, want 6; compacted=%q", found, compacted)
	}
	for _, item := range hierarchyRows {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if title, _ := row["title"].(string); title == "Proposed tasks" {
			if got := anyToIntForLLM(row["child_count"]); got != 6 {
				t.Fatalf("outline_hierarchy Proposed tasks child_count = %d, want 6; compacted=%q", got, compacted)
			}
			childTitles, ok := row["child_titles"].([]interface{})
			if !ok || len(childTitles) != 6 {
				t.Fatalf("outline_hierarchy Proposed tasks child_titles = %#v, want 6 entries; compacted=%q", row["child_titles"], compacted)
			}
			return
		}
	}
	t.Fatalf("expected Proposed tasks in outline_hierarchy; compacted=%q", compacted)
}

func TestCompactToolResultContentForLLM_PDFKeepsOutlineParentsWhenSampling(t *testing.T) {
	outline := make([]interface{}, 0, 60)
	for i := 1; i <= 53; i++ {
		outline = append(outline, map[string]interface{}{
			"title":       fmt.Sprintf("Section %02d", i),
			"level":       2,
			"page_number": i,
		})
	}
	outline = append(outline, map[string]interface{}{
		"title":       "Proposed tasks",
		"level":       3,
		"page_number": 54,
	})
	for i, title := range []string{
		"Secure skill installation and safe configuration",
		"Browser automation with \"no API\" constraints and recovery",
		"Multi-channel routing and session isolation",
		"Scheduled daily briefing with data fusion + memory write-back",
		"PR review and repair loop with CI feedback",
		"Prompt-injection and tool-blast-radius containment",
	} {
		outline = append(outline, map[string]interface{}{
			"title":       title,
			"level":       4,
			"page_number": 54 + i/2,
		})
	}

	payload := map[string]interface{}{
		"document": map[string]interface{}{
			"path":       "/tmp/workspace/openclaw_report.pdf",
			"file_name":  "openclaw_report.pdf",
			"page_count": 11,
		},
		"outline": outline,
	}
	contentBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal pdf payload: %v", err)
	}

	compacted := compactToolResultContentForLLM("pdf", string(contentBytes))
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted), &out); err != nil {
		t.Fatalf("unmarshal compacted pdf payload: %v", err)
	}
	rows, ok := out["outline"].([]interface{})
	if !ok {
		t.Fatalf("outline = %#v", out["outline"])
	}
	if len(rows) != 40 {
		t.Fatalf("outline length = %d, want 40; compacted=%q", len(rows), compacted)
	}
	hierarchyRows, ok := out["outline_hierarchy"].([]interface{})
	if !ok || len(hierarchyRows) == 0 {
		t.Fatalf("outline_hierarchy = %#v", out["outline_hierarchy"])
	}

	parentInOutline := false
	for _, item := range rows {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if title, _ := row["title"].(string); title == "Proposed tasks" {
			parentInOutline = true
			if got := anyToIntForLLM(row["child_count"]); got != 6 {
				t.Fatalf("Proposed tasks child_count = %d, want 6; compacted=%q", got, compacted)
			}
		}
	}
	if !parentInOutline {
		t.Fatalf("expected Proposed tasks parent entry in sampled outline; compacted=%q", compacted)
	}
	for _, item := range hierarchyRows {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if title, _ := row["title"].(string); title == "Proposed tasks" {
			if got := anyToIntForLLM(row["child_count"]); got != 6 {
				t.Fatalf("outline_hierarchy Proposed tasks child_count = %d, want 6; compacted=%q", got, compacted)
			}
			childTitles, ok := row["child_titles"].([]interface{})
			if !ok || len(childTitles) != 6 {
				t.Fatalf("outline_hierarchy Proposed tasks child_titles = %#v, want 6 entries; compacted=%q", row["child_titles"], compacted)
			}
			return
		}
	}
	t.Fatalf("expected Proposed tasks in sampled outline_hierarchy; compacted=%q", compacted)
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

func TestCompactToolResultsForLLM_WebQueryAdditionalRoundsKeepNonEmptySummary(t *testing.T) {
	raw := buildWebQueryToolPayloadForLLMTests(strings.Repeat("On April 4, 2026, version 1.2.3 shipped 12 improvements, 4 fixes, and 2 migrations. ", 160))
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal web_query payload: %v", err)
	}

	toolCalls := []llm.ToolCall{
		{ID: "call_1", Name: "web_query", Arguments: `{"input":"blue release notes alpha"}`},
		{ID: "call_2", Name: "web_query", Arguments: `{"input":"blue release notes beta"}`},
		{ID: "call_3", Name: "web_query", Arguments: `{"input":"blue release notes gamma"}`},
		{ID: "call_4", Name: "web_query", Arguments: `{"input":"blue release notes delta"}`},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call_1", ToolName: "web_query", Content: string(contentBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_2", ToolName: "web_query", Content: string(contentBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_3", ToolName: "web_query", Content: string(contentBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_4", ToolName: "web_query", Content: string(contentBytes)},
	}

	compacted := compactToolResultsForLLM(toolCalls, toolResults)
	if len(compacted) != 4 {
		t.Fatalf("len(compacted) = %d, want 4", len(compacted))
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(compacted[3].Content), &out); err != nil {
		t.Fatalf("unmarshal fourth-round web_query summary: %v payload=%q", err, compacted[3].Content)
	}
	if got, ok := out["omitted_from_llm_context"].(bool); !ok || !got {
		t.Fatalf("omitted_from_llm_context = %#v, want true", out["omitted_from_llm_context"])
	}
	if got, ok := out["has_results"].(bool); !ok || !got {
		t.Fatalf("has_results = %#v, want true", out["has_results"])
	}
	if _, ok := out["selected_result"].(map[string]interface{}); !ok {
		t.Fatalf("selected_result = %#v, want preserved selected result", out["selected_result"])
	}
}

func TestCompactToolResultsForLLM_WebQueryBudgetPressureKeepsValidJSON(t *testing.T) {
	raw := buildWebQueryToolPayloadForLLMTests(strings.Repeat("On April 4, 2026, version 1.2.3 shipped 12 improvements, 4 fixes, and 2 migrations. ", 220))
	contentBytes, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal web_query payload: %v", err)
	}

	toolCalls := []llm.ToolCall{
		{ID: "call_1", Name: "web_query", Arguments: `{"input":"blue release notes alpha"}`},
		{ID: "call_2", Name: "web_query", Arguments: `{"input":"blue release notes beta"}`},
		{ID: "call_3", Name: "web_query", Arguments: `{"input":"blue release notes gamma"}`},
		{ID: "call_4", Name: "web_query", Arguments: `{"input":"blue release notes delta"}`},
		{ID: "call_5", Name: "web_query", Arguments: `{"input":"blue release notes epsilon"}`},
	}
	toolResults := []llm.Message{
		{Role: llm.RoleTool, ToolCallID: "call_1", ToolName: "web_query", Content: string(contentBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_2", ToolName: "web_query", Content: string(contentBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_3", ToolName: "web_query", Content: string(contentBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_4", ToolName: "web_query", Content: string(contentBytes)},
		{Role: llm.RoleTool, ToolCallID: "call_5", ToolName: "web_query", Content: string(contentBytes)},
	}

	compacted := compactToolResultsForLLM(toolCalls, toolResults)
	for idx, msg := range compacted {
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Content), &out); err != nil {
			t.Fatalf("message %d invalid JSON after budget compaction: %v payload=%q", idx, err, msg.Content)
		}
		if hasResults, ok := out["has_results"].(bool); ok && hasResults {
			if _, ok := out["selected_result"].(map[string]interface{}); !ok {
				t.Fatalf("message %d missing selected_result under budget pressure: %#v", idx, out)
			}
		}
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
