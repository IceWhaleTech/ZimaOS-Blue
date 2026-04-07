package agentcore

import (
	"context"
	"strings"
	"testing"
)

func TestSystemPromptBuilder_CachesInitializeOnDemand(t *testing.T) {
	builder := NewSystemPromptBuilder(&Config{})

	if builder.staticCache != nil || builder.configCache != nil {
		t.Fatal("expected system prompt caches to start cold")
	}

	prompt := builder.Build(context.Background(), "")
	if !strings.Contains(prompt, "<role>") {
		t.Fatalf("Build() = %q, want prompt to include role guidance", prompt)
	}
	if builder.staticCache == nil || builder.configCache == nil {
		t.Fatal("expected system prompt caches to initialize on first build")
	}
}
