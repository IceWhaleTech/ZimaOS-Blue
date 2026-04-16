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

func TestNormalizeToolSchemaForLLM_AddsEmptyRequiredForNestedObjects(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"questions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"question": map[string]interface{}{"type": "string"},
						"options": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"oneOf": []map[string]interface{}{
									{"type": "string"},
									{
										"type": "object",
										"properties": map[string]interface{}{
											"label": map[string]interface{}{"type": "string"},
										},
									},
								},
							},
						},
					},
					"required": []string{"question"},
				},
			},
		},
	}

	normalized := NormalizeToolSchemaForLLM("", "", "", schema)
	required, ok := normalized["required"].([]string)
	if !ok {
		t.Fatalf("top-level required type = %T, want []string", normalized["required"])
	}
	if len(required) != 0 {
		t.Fatalf("top-level required = %v, want empty", required)
	}

	props := normalized["properties"].(map[string]interface{})
	questions := props["questions"].(map[string]interface{})
	items := questions["items"].(map[string]interface{})
	options := items["properties"].(map[string]interface{})["options"].(map[string]interface{})
	oneOf := options["items"].(map[string]interface{})["oneOf"].([]interface{})
	nestedObject := oneOf[1].(map[string]interface{})
	nestedRequired, ok := nestedObject["required"].([]string)
	if !ok {
		t.Fatalf("nested required type = %T, want []string", nestedObject["required"])
	}
	if len(nestedRequired) != 0 {
		t.Fatalf("nested required = %v, want empty", nestedRequired)
	}
}

func TestNormalizeToolSchemaForLLM_AddsItemsForDocumentToolArrays(t *testing.T) {
	toolSchemas := []struct {
		name   string
		schema map[string]interface{}
		keys   []string
	}{
		{
			name:   "docx",
			schema: NewDOCXTool(nil, nil, nil).Definition().Parameters,
			keys:   []string{"sections", "notes", "paragraphs"},
		},
		{
			name:   "xlsx",
			schema: NewXLSXTool(nil, nil, nil).Definition().Parameters,
			keys:   []string{"notes", "sheets", "columns", "rows"},
		},
		{
			name:   "pptx",
			schema: NewPPTXTool(nil, nil, nil).Definition().Parameters,
			keys:   []string{"sections", "paragraphs", "notes"},
		},
		{
			name:   "pdf",
			schema: NewPDFTool(nil).Definition().Parameters,
			keys:   []string{"sections", "paragraphs", "notes"},
		},
		{
			name:   "office",
			schema: NewOfficeTool(nil, nil, nil).Definition().Parameters,
			keys:   []string{"rows", "columns", "sheets", "sections"},
		},
	}

	for _, tc := range toolSchemas {
		t.Run(tc.name, func(t *testing.T) {
			normalized := NormalizeToolSchemaForLLM("", "", "", tc.schema)
			props, ok := normalized["properties"].(map[string]interface{})
			if !ok {
				t.Fatalf("properties type = %T, want map[string]interface{}", normalized["properties"])
			}
			for _, key := range tc.keys {
				field, ok := props[key].(map[string]interface{})
				if !ok {
					t.Fatalf("%s field type = %T, want map[string]interface{}", key, props[key])
				}
				items, ok := field["items"].(map[string]interface{})
				if !ok {
					t.Fatalf("%s.items missing from normalized schema: %#v", key, field)
				}
				if items == nil {
					t.Fatalf("%s.items = nil, want schema object", key)
				}
			}
		})
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

func TestDocumentToolThemeEnumsIncludeEditorial(t *testing.T) {
	tests := []struct {
		name string
		enum []string
	}{
		{
			name: "docx",
			enum: NewDOCXTool(nil, nil, nil).Definition().Parameters["properties"].(map[string]interface{})["theme"].(map[string]interface{})["enum"].([]string),
		},
		{
			name: "xlsx",
			enum: NewXLSXTool(nil, nil, nil).Definition().Parameters["properties"].(map[string]interface{})["theme"].(map[string]interface{})["enum"].([]string),
		},
		{
			name: "pptx",
			enum: NewPPTXTool(nil, nil, nil).Definition().Parameters["properties"].(map[string]interface{})["theme"].(map[string]interface{})["enum"].([]string),
		},
	}

	convertProps := NewConvertTool(nil, nil, nil, nil).Definition().Parameters["properties"].(map[string]interface{})
	convertOptions := convertProps["options"].(map[string]interface{})["properties"].(map[string]interface{})
	tests = append(tests,
		struct {
			name string
			enum []string
		}{
			name: "convert.presentation",
			enum: convertOptions["presentation"].(map[string]interface{})["properties"].(map[string]interface{})["theme"].(map[string]interface{})["enum"].([]string),
		},
		struct {
			name string
			enum []string
		}{
			name: "convert.pptx",
			enum: convertOptions["pptx"].(map[string]interface{})["properties"].(map[string]interface{})["theme"].(map[string]interface{})["enum"].([]string),
		},
	)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			found := false
			for _, candidate := range tc.enum {
				if candidate == "editorial" {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("theme enum = %v, want editorial", tc.enum)
			}
		})
	}
}
