package agentcore

import (
	"strings"
	"testing"
)

func TestBuildKnowledgeBaseResearchGuidance(t *testing.T) {
	guidance := BuildKnowledgeBaseResearchGuidance()
	checks := []string{
		"Knowledge-base-grade research",
		"at least 2 diverse retrieval rounds",
		"<phase id=\"1\" name=\"scope\">",
		"<phase id=\"4\" name=\"audit\">",
		"source_inventory",
		"per-object analysis",
		"conflicts, freshness, and gaps",
		"research capability via deep_research",
	}
	for _, want := range checks {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
	if strings.Contains(guidance, "Knowledge-base-grade deep research") {
		t.Fatalf("guidance should not use legacy deep research mission copy: %s", guidance)
	}
}
