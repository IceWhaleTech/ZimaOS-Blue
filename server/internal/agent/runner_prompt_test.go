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
