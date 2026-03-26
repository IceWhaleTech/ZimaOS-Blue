package agentcore

import (
	"strings"
	"testing"
)

func TestBuildKnowledgeBaseResearchGuidance(t *testing.T) {
	guidance := BuildKnowledgeBaseResearchGuidance()
	checks := []string{
		"Knowledge-base-grade deep research",
		"at least 2 diverse retrieval rounds",
		"<phase id=\"1\" name=\"scope\">",
		"<phase id=\"4\" name=\"audit\">",
		"source_inventory",
		"per-object analysis",
		"conflicts, freshness, and gaps",
	}
	for _, want := range checks {
		if !strings.Contains(guidance, want) {
			t.Fatalf("guidance missing %q: %s", want, guidance)
		}
	}
}
