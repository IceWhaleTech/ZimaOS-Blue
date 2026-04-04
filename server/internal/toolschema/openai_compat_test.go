package toolschema

import "testing"

func TestNormalizeForOpenAICompat_StripsTopLevelCompositeKeywords(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"anyOf": []map[string]interface{}{
			{"required": []string{"query"}},
			{"required": []string{"job_id"}},
		},
	}

	normalized := NormalizeForOpenAICompat(schema)
	if _, ok := normalized["anyOf"]; ok {
		t.Fatalf("top-level anyOf should be stripped, got %v", normalized["anyOf"])
	}
	required, ok := normalized["required"].([]string)
	if !ok {
		t.Fatalf("required type = %T, want []string", normalized["required"])
	}
	if len(required) != 0 {
		t.Fatalf("len(required) = %d, want 0", len(required))
	}
}

func TestNormalizeForOpenAICompat_PreservesNestedCompositeKeywords(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"questions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
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
				},
			},
		},
	}

	normalized := NormalizeForOpenAICompat(schema)
	props := normalized["properties"].(map[string]interface{})
	questions := props["questions"].(map[string]interface{})
	items := questions["items"].(map[string]interface{})
	options := items["properties"].(map[string]interface{})["options"].(map[string]interface{})
	oneOf, ok := options["items"].(map[string]interface{})["oneOf"].([]interface{})
	if !ok {
		t.Fatalf("nested oneOf type = %T, want []interface{}", options["items"].(map[string]interface{})["oneOf"])
	}
	if len(oneOf) != 2 {
		t.Fatalf("len(nested oneOf) = %d, want 2", len(oneOf))
	}
}
