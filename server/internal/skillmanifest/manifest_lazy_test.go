package skillmanifest

import (
	"sync"
	"testing"
)

func TestSkillManifestRegexes_InitializeOnDemand(t *testing.T) {
	originalScriptPattern := skillScriptPathRegexp
	originalScriptOnce := skillScriptPathRegexpOnce
	originalSlugPattern := nonSlugChars
	originalSlugOnce := nonSlugCharsOnce

	skillScriptPathRegexp = nil
	skillScriptPathRegexpOnce = sync.Once{}
	nonSlugChars = nil
	nonSlugCharsOnce = sync.Once{}
	t.Cleanup(func() {
		skillScriptPathRegexp = originalScriptPattern
		skillScriptPathRegexpOnce = originalScriptOnce
		nonSlugChars = originalSlugPattern
		nonSlugCharsOnce = originalSlugOnce
	})

	if skillScriptPathRegexp != nil || nonSlugChars != nil {
		t.Fatal("expected skill manifest regexes to start nil")
	}

	gotID := normalizeSkillID("  Fancy Skill/Name  ")
	if gotID != "fancy_skill_name" {
		t.Fatalf("normalizeSkillID() = %q, want %q", gotID, "fancy_skill_name")
	}
	if nonSlugChars == nil {
		t.Fatal("expected normalizeSkillID to initialize nonSlugChars on demand")
	}
	if skillScriptPathRegexp != nil {
		t.Fatal("expected script path regex to remain nil until script extraction runs")
	}

	gotPaths := extractScriptPaths("Run scripts/bootstrap.sh and ./scripts/fetch_data.py")
	if len(gotPaths) != 2 {
		t.Fatalf("extractScriptPaths() = %v, want 2 script paths", gotPaths)
	}
	if gotPaths[0] != "./scripts/fetch_data.py" || gotPaths[1] != "scripts/bootstrap.sh" {
		t.Fatalf("extractScriptPaths() = %v, want sorted extracted script paths", gotPaths)
	}
	if skillScriptPathRegexp == nil {
		t.Fatal("expected extractScriptPaths to initialize skillScriptPathRegexp on demand")
	}
}
