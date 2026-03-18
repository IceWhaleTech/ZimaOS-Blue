package proxy

import (
	stdjson "encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestCompactOverflowForContinuationItem_WriteChunkArgumentsUseJSONAwareCompaction(t *testing.T) {
	originalContent := strings.Repeat("abcdefghij", 700)
	argsJSON, err := stdjson.Marshal(map[string]interface{}{
		"session_id": "write_session_1",
		"path":       "docs/report.md",
		"content":    originalContent,
	})
	if err != nil {
		t.Fatalf("marshal args failed: %v", err)
	}

	rawItem, err := stdjson.Marshal(map[string]interface{}{
		"type":      "function_call",
		"call_id":   "call_1",
		"name":      "write_chunk",
		"arguments": string(argsJSON),
	})
	if err != nil {
		t.Fatalf("marshal raw item failed: %v", err)
	}

	item := gjson.ParseBytes(rawItem)
	compacted := compactOverflowForContinuationItem(string(rawItem), item)
	compactedArgs := gjson.Get(compacted, "arguments").String()
	if compactedArgs == "" {
		t.Fatalf("expected compacted arguments, got empty string from %s", compacted)
	}
	if strings.Contains(compactedArgs, responsesContinuationToolTrimMarker) {
		t.Fatalf("write arguments should use JSON-aware compaction, got generic trim marker: %s", compactedArgs)
	}
	if !stdjson.Valid([]byte(compactedArgs)) {
		t.Fatalf("compacted arguments must remain valid JSON, got: %s", compactedArgs)
	}

	var payload map[string]interface{}
	if err := stdjson.Unmarshal([]byte(compactedArgs), &payload); err != nil {
		t.Fatalf("unmarshal compacted arguments failed: %v", err)
	}
	if got, _ := payload["session_id"].(string); got != "write_session_1" {
		t.Fatalf("session_id = %q, want %q", got, "write_session_1")
	}
	if got, _ := payload["path"].(string); got != "docs/report.md" {
		t.Fatalf("path = %q, want %q", got, "docs/report.md")
	}
	if got, _ := payload["content"].(string); got != "[omitted large write payload already executed]" {
		t.Fatalf("content marker = %q, want omission marker", got)
	}
	if got, ok := payload["content_chars"].(float64); !ok || int(got) != len([]rune(originalContent)) {
		t.Fatalf("content_chars = %#v, want %d", payload["content_chars"], len([]rune(originalContent)))
	}
	if got, ok := payload["content_bytes"].(float64); !ok || int(got) != len(originalContent) {
		t.Fatalf("content_bytes = %#v, want %d", payload["content_bytes"], len(originalContent))
	}
}
