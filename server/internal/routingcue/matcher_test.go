package routingcue

import "testing"

func TestInferSkill_LocalizedExamples(t *testing.T) {
	for skill, examples := range localizedSkillExamples {
		for _, example := range examples {
			got, ok := InferSkill(example.Query)
			if !ok || got != skill {
				t.Fatalf("InferSkill(%q) = (%q, %v), want (%q, true)", example.Query, got, ok, skill)
			}
		}
	}
}

func TestInferSkill_LocalizedURLBypassExamples(t *testing.T) {
	for skill, examples := range localizedURLBypassExamples {
		for _, example := range examples {
			got, ok := InferSkill(example.Query)
			if !ok || got != skill {
				t.Fatalf("InferSkill(%q) = (%q, %v), want (%q, true)", example.Query, got, ok, skill)
			}
		}
	}
}

func TestInferSkill_MixedIntentReturnsFalse(t *testing.T) {
	query := "看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？"
	if got, ok := InferSkill(query); ok {
		t.Fatalf("InferSkill(%q) = (%q, true), want no high-confidence match", query, got)
	}
}
