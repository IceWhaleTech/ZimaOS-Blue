package tools

import "testing"

func TestValidateToolSchema_AllowsTypedCompositeSlices(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"payload": map[string]interface{}{
				"anyOf": []map[string]interface{}{
					{"type": "string"},
					{"type": "array", "items": map[string]interface{}{"type": "string"}},
				},
			},
		},
	}

	if err := ValidateToolSchema(schema); err != nil {
		t.Fatalf("ValidateToolSchema() error = %v", err)
	}
}

func TestNormalizeToolSchemaForLLM_PreservesTypedCompositeSlices(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"payload": map[string]interface{}{
				"anyOf": []map[string]interface{}{
					{"type": "string"},
					{"type": "array", "items": map[string]interface{}{"type": "string"}},
				},
			},
		},
	}

	normalized := NormalizeToolSchemaForLLM("", "", "", schema)
	props, ok := normalized["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties type = %T, want map[string]interface{}", normalized["properties"])
	}
	payload, ok := props["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload type = %T, want map[string]interface{}", props["payload"])
	}
	anyOf, ok := payload["anyOf"].([]interface{})
	if !ok {
		t.Fatalf("payload.anyOf type = %T, want []interface{}", payload["anyOf"])
	}
	if len(anyOf) != 2 {
		t.Fatalf("len(payload.anyOf) = %d, want 2", len(anyOf))
	}
}

func TestValidateToolArguments_HonorsEnumStringSlices(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"mode": map[string]interface{}{
				"type": "string",
				"enum": []string{"fast", "deep"},
			},
		},
		"required": []string{"mode"},
	}

	if err := ValidateToolArguments(schema, map[string]interface{}{"mode": "fast"}); err != nil {
		t.Fatalf("ValidateToolArguments(valid) error = %v", err)
	}
	if err := ValidateToolArguments(schema, map[string]interface{}{"mode": "other"}); err == nil {
		t.Fatal("expected enum validation error, got nil")
	}
}
