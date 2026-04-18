package tools

import (
	"encoding/json"
	"testing"
)

func TestCanonicalizeToolArgumentsJSON_RepairsMissingValuesAndDanglingKeys(t *testing.T) {
	raw := `{"action":"create","path":"reports/out.pdf","fields":,"include":{"pages":true},"input_path":"","markdown"}`

	normalized, ok := CanonicalizeToolArgumentsJSON(raw)
	if !ok {
		t.Fatalf("CanonicalizeToolArgumentsJSON() ok=false, got %q", normalized)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(normalized), &payload); err != nil {
		t.Fatalf("normalized payload is not JSON object: %v (content=%q)", err, normalized)
	}

	if _, exists := payload["fields"]; !exists {
		t.Fatalf("expected fields key to remain, got %#v", payload)
	}
	// Tool payload canonicalization sanitizes `null` into an empty object payload
	// to keep downstream schema validation happy for object-like fields.
	if _, ok := payload["fields"].(map[string]interface{}); !ok {
		t.Fatalf("expected fields to be an object after repair, got %#v", payload["fields"])
	}
	if _, exists := payload["markdown"]; exists {
		t.Fatalf("expected dangling markdown key to be removed, got %#v", payload)
	}
}

func TestFirstCompatPathString_FallsBackWhenPrimaryKeyEmpty(t *testing.T) {
	args := map[string]interface{}{
		"path":        "",
		"output_path": "reports/out.pdf",
	}
	if got := firstCompatPathString(args); got != "reports/out.pdf" {
		t.Fatalf("firstCompatPathString() = %q, want %q", got, "reports/out.pdf")
	}
}

