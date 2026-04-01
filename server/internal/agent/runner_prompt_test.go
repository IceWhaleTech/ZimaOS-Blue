package agent

import (
	"strings"
	"testing"
)

func TestBuildStepExecutionSystemPrompt_IncludesCodingDefaults(t *testing.T) {
	out := buildStepExecutionSystemPrompt(nil)

	required := []string{
		"superpowers and ui-ux-pro-max-skill",
		"If stack preferences are unclear, ask once and remember them for future steps.",
		"Default stack when not specified: backend Go, frontend React, mobile React Native, client Electron.",
		"for example Python or Node.js",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("expected step execution prompt to include %q, got: %s", want, out)
		}
	}
}

func TestBuildTaskRoutingContractContext_IncludesExecutionEquivalenceHints(t *testing.T) {
	out := buildTaskRoutingContractContext(map[string]interface{}{
		"gate_type": "execution_equivalence",
		"routing_contract": map[string]interface{}{
			"primary_route":       "analyze",
			"expected_cli_action": "blue analyze",
			"allow_fallback":      false,
		},
	})

	required := []string{
		"Execution routing contract:",
		"Primary route: analyze",
		"Canonical CLI action: blue analyze",
		"Do not use blue task, blue session",
		"Do not substitute unrelated tools",
		"stop and report the blocker",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("expected routing contract context to include %q, got: %s", want, out)
		}
	}
}
