package toolschema_test

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/toolschema"
)

func TestNormalizeForOpenAICompat_RealToolDefinitionsStripTopLevelCompositeKeywords(t *testing.T) {
	cases := []struct {
		name   string
		schema map[string]interface{}
	}{
		{
			name:   "exec",
			schema: tools.NewExecTool(tools.DefaultExecConfig(), tools.NewSessionRegistry(), nil, nil, nil).Definition().Parameters,
		},
		{
			name:   "deep_research",
			schema: tools.NewDeepResearchTool(nil).Definition().Parameters,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			normalized := toolschema.NormalizeForOpenAICompat(tc.schema)
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

func TestNormalizeForOpenAICompat_RealAskDefinitionPreservesNestedComposition(t *testing.T) {
	normalized := toolschema.NormalizeForOpenAICompat((&tools.AskTool{}).Definition().Parameters)

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
