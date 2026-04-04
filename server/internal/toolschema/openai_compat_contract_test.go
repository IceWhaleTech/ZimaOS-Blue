package toolschema

import "testing"

func TestNormalizeForOpenAICompat_ToolLikeSchemasStripTopLevelCompositeKeywords(t *testing.T) {
	cases := []struct {
		name   string
		schema map[string]interface{}
	}{
		{
			name: "exec_like",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
					"action":  map[string]interface{}{"type": "string"},
				},
				"anyOf": []map[string]interface{}{
					{"required": []string{"command"}},
					{"required": []string{"action"}},
				},
			},
		},
		{
			name: "deep_research_like",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":  map[string]interface{}{"type": "string"},
					"job_id": map[string]interface{}{"type": "string"},
				},
				"anyOf": []interface{}{
					map[string]interface{}{"required": []string{"query"}},
					map[string]interface{}{"required": []string{"job_id"}},
				},
			},
		},
		{
			name: "top_level_enum",
			schema: map[string]interface{}{
				"type": "object",
				"enum": []string{"a", "b"},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			normalized := NormalizeForOpenAICompat(tc.schema)
			for _, key := range []string{"oneOf", "anyOf", "allOf", "enum", "not"} {
				if _, ok := normalized[key]; ok {
					t.Fatalf("top-level %s should be stripped, got %v", key, normalized[key])
				}
			}
			required, ok := normalized["required"].([]string)
			if !ok {
				t.Fatalf("required type = %T, want []string", normalized["required"])
			}
			if len(required) != 0 {
				t.Fatalf("len(required) = %d, want 0", len(required))
			}
		})
	}
}

func TestNormalizeForOpenAICompat_AskLikeNestedCompositionStillAllowed(t *testing.T) {
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
								"oneOf": []interface{}{
									map[string]interface{}{"type": "string"},
									map[string]interface{}{
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
