package agentcore

import (
	"sync"
	"testing"
)

func TestSkillScriptPathRegex_InitializesOnDemand(t *testing.T) {
	originalPattern := skillScriptPathRegexp
	originalOnce := skillScriptPathRegexpOnce

	skillScriptPathRegexp = nil
	skillScriptPathRegexpOnce = sync.Once{}
	t.Cleanup(func() {
		skillScriptPathRegexp = originalPattern
		skillScriptPathRegexpOnce = originalOnce
	})

	if skillScriptPathRegexp != nil {
		t.Fatal("expected skill script path regex to start nil")
	}

	got := extractScriptPaths("Run scripts/analyze_video.py and ./scripts/fetch_comments.py")
	if len(got) != 2 {
		t.Fatalf("extractScriptPaths() = %v, want 2 script paths", got)
	}
	if got[0] != "scripts/analyze_video.py" || got[1] != "./scripts/fetch_comments.py" {
		t.Fatalf("extractScriptPaths() = %v, want extracted script paths in order", got)
	}
	if skillScriptPathRegexp == nil {
		t.Fatal("expected skill script path regex to initialize on first extraction")
	}
}
