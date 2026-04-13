package builtin

import (
	"sync"
	"testing"
)

func TestAnalyzeSkillTopicURLRegex_InitializesOnDemand(t *testing.T) {
	originalPattern := analyzeSkillTopicURLPattern
	originalOnce := analyzeSkillTopicURLPatternOnce

	analyzeSkillTopicURLPattern = nil
	analyzeSkillTopicURLPatternOnce = sync.Once{}
	t.Cleanup(func() {
		analyzeSkillTopicURLPattern = originalPattern
		analyzeSkillTopicURLPatternOnce = originalOnce
	})

	if analyzeSkillTopicURLPattern != nil {
		t.Fatal("expected analyze skill topic URL regex to start nil")
	}

	a := NewAnalyze()
	input := map[string]any{
		"topic": "Summarise https://example.com/blog and extract the key points.",
	}
	if err := a.Validate(input); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if analyzeSkillTopicURLPattern == nil {
		t.Fatal("expected analyze skill topic URL regex to initialize on first topic URL promotion")
	}
}
