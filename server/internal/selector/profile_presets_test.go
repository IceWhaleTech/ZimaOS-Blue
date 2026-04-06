package selector

import "testing"

func TestCompactTermsNormalizesAndDeduplicates(t *testing.T) {
	got := CompactTerms(" Browser ", "browser", "BROWSER", "", "  ", "web")
	want := []string{"browser", "web"}
	if len(got) != len(want) {
		t.Fatalf("len(CompactTerms) = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CompactTerms()[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestApplyCommonProfilePresetBrowser(t *testing.T) {
	base := SelectorProfile{
		Name:         "browser",
		ExactAliases: []string{"browser"},
		ContextCues:  []string{"browser"},
	}

	got, ok := ApplyCommonProfilePreset(base, "browser")
	if !ok {
		t.Fatal("expected browser preset")
	}
	if !containsTerm(got.Actions, "open") || !containsTerm(got.Actions, "browse") {
		t.Fatalf("expected browser actions to include open/browse, got %v", got.Actions)
	}
	if !containsTerm(got.Objects, "url") || !containsTerm(got.Objects, "webpage") {
		t.Fatalf("expected browser objects to include url/webpage, got %v", got.Objects)
	}
	if !containsTerm(got.ContextCues, "http://") || !containsTerm(got.ContextCues, "https://") {
		t.Fatalf("expected browser context cues to include URL hints, got %v", got.ContextCues)
	}
	if !containsTerm(got.PreferredDomains, DomainLiveWeb) || !containsTerm(got.PreferredDomains, DomainURLPresent) {
		t.Fatalf("expected browser preferred domains to include live web and URL present, got %v", got.PreferredDomains)
	}
}

func TestApplyCommonProfilePresetUIReviewer(t *testing.T) {
	base := SelectorProfile{Name: "ui_reviewer"}
	got, ok := ApplyCommonProfilePreset(base, "ui_reviewer")
	if !ok {
		t.Fatal("expected ui_reviewer preset")
	}
	if !containsTerm(got.Actions, "review") || !containsTerm(got.Actions, "accessibility check") {
		t.Fatalf("expected ui_reviewer actions to include review/accessibility check, got %v", got.Actions)
	}
	if !containsTerm(got.Objects, "ui") || !containsTerm(got.Objects, "screenshot") || !containsTerm(got.Objects, "accessibility") {
		t.Fatalf("expected ui_reviewer objects to include ui/screenshot/accessibility, got %v", got.Objects)
	}
	if !containsTerm(got.RequireAnyDomains, DomainUIArtifact) {
		t.Fatalf("expected ui_reviewer required domains to include %q, got %v", DomainUIArtifact, got.RequireAnyDomains)
	}
	if !containsTerm(got.PreferredDomains, DomainUIArtifact) {
		t.Fatalf("expected ui_reviewer preferred domains to include %q, got %v", DomainUIArtifact, got.PreferredDomains)
	}
}

func TestApplyCommonProfilePresetReturnsFalseForUnknownName(t *testing.T) {
	got, ok := ApplyCommonProfilePreset(SelectorProfile{Name: "custom"}, "custom")
	if ok {
		t.Fatalf("expected no preset for custom profile, got %+v", got)
	}
}

func containsTerm(terms []string, want string) bool {
	for _, term := range terms {
		if term == want {
			return true
		}
	}
	return false
}
