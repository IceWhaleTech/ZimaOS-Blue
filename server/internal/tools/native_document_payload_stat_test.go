package tools

import (
	"context"
	"os"
	"testing"
)

func TestDOCXToolValidateAllowsPayloadStatFailure(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewDOCXTool([]string{tmpDir}, nil, nil)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/stat_guard.docx",
		"title":  "Guard",
		"paragraphs": []interface{}{
			"Body",
		},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	restore := nativeDocumentStatForPayload
	nativeDocumentStatForPayload = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() {
		nativeDocumentStatForPayload = restore
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate",
		"path":   "reports/stat_guard.docx",
	})
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["success"]; got != true {
		t.Fatalf("success = %v, want true", got)
	}
	if _, ok := payload["size"]; ok {
		t.Fatalf("expected size to be omitted when payload stat fails, got %#v", payload["size"])
	}
}

func TestXLSXToolValidateAllowsPayloadStatFailure(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/stat_guard.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Metrics",
				"columns": []interface{}{
					map[string]interface{}{"header": "Name", "key": "name"},
				},
				"rows": []interface{}{
					map[string]interface{}{"name": "North"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	restore := nativeDocumentStatForPayload
	nativeDocumentStatForPayload = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() {
		nativeDocumentStatForPayload = restore
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate",
		"path":   "reports/stat_guard.xlsx",
	})
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["success"]; got != true {
		t.Fatalf("success = %v, want true", got)
	}
	if _, ok := payload["size"]; ok {
		t.Fatalf("expected size to be omitted when payload stat fails, got %#v", payload["size"])
	}
}

func TestPPTXToolValidateAllowsPayloadStatFailure(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/stat_guard.pptx",
		"title":  "Launch",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Overview",
				"paragraphs": []interface{}{"Ready"},
			},
		},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	restore := nativeDocumentStatForPayload
	nativeDocumentStatForPayload = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() {
		nativeDocumentStatForPayload = restore
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate",
		"path":   "decks/stat_guard.pptx",
	})
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["success"]; got != true {
		t.Fatalf("success = %v, want true", got)
	}
	if _, ok := payload["size"]; ok {
		t.Fatalf("expected size to be omitted when payload stat fails, got %#v", payload["size"])
	}
}

func TestPPTXToolReplaceTextAllowsPayloadStatFailure(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/stat_guard_template.pptx",
		"title":  "Launch",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Overview",
				"paragraphs": []interface{}{"Launch overview"},
			},
		},
	}); err != nil {
		t.Fatalf("create failed: %v", err)
	}

	restore := nativeDocumentStatForPayload
	nativeDocumentStatForPayload = func(string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	t.Cleanup(func() {
		nativeDocumentStatForPayload = restore
	})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "replace_text",
		"path":   "decks/stat_guard_template.pptx",
		"replacements": map[string]interface{}{
			"Overview": "Executive Overview",
		},
	})
	if err != nil {
		t.Fatalf("replace_text failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["success"]; got != true {
		t.Fatalf("success = %v, want true", got)
	}
	if _, ok := payload["size"]; ok {
		t.Fatalf("expected size to be omitted when payload stat fails, got %#v", payload["size"])
	}
}
