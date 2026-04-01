package agentcore

import (
	"testing"
)

func TestResolveCanonicalSkill(t *testing.T) {
	tests := []struct {
		input  string
		wantID CanonicalSkillID
		wantOK bool
	}{
		{"web_query", CanonicalWebQuery, true},
		{"web_search", CanonicalWebQuery, true},
		{"search", CanonicalWebQuery, true},
		{"browser", CanonicalBrowser, true},
		{"analyze", CanonicalAnalyze, true},
		{"deep_research", CanonicalDeepResearch, true},
		{"research", CanonicalDeepResearch, true},
		{"exec", CanonicalExec, true},
		{"unknown_skill", CanonicalUnknown, false},
		{"WEB_SEARCH", CanonicalWebQuery, true},
		{"  web_search  ", CanonicalWebQuery, true},
	}

	for _, tc := range tests {
		got, ok := ResolveCanonicalSkill(tc.input)
		if ok != tc.wantOK {
			t.Errorf("ResolveCanonicalSkill(%q) ok=%v, want %v", tc.input, ok, tc.wantOK)
			continue
		}
		if tc.wantOK && got != tc.wantID {
			t.Errorf("ResolveCanonicalSkill(%q)=%q, want %q", tc.input, got, tc.wantID)
		}
	}
}

func TestIsCutoverEligibleCanonical(t *testing.T) {
	if !IsCutoverEligibleCanonical(CanonicalWebQuery) {
		t.Error("web_query should be cutover eligible")
	}
	if !IsCutoverEligibleCanonical(CanonicalDeepResearch) {
		t.Error("deep_research should be cutover eligible")
	}
	if IsCutoverEligibleCanonical(CanonicalExec) {
		t.Error("exec should NOT be cutover eligible")
	}
	if IsCutoverEligibleCanonical(CanonicalUnknown) {
		t.Error("unknown should NOT be cutover eligible")
	}
}

func TestExecutionProfileForSkill(t *testing.T) {
	if ExecutionProfileForSkill(CanonicalDeepResearch) != ExecutionProfileRequireFork {
		t.Error("deep_research should require_fork")
	}
	if ExecutionProfileForSkill(CanonicalWebQuery) != ExecutionProfilePreferFork {
		t.Error("web_query should prefer_fork")
	}
	if ExecutionProfileForSkill(CanonicalBrowser) != ExecutionProfileInline {
		t.Error("browser should be inline")
	}
}

func TestNativeSurfaceModeForSkill(t *testing.T) {
	if NativeSurfaceModeForSkill(CanonicalWebQuery) != NativeSurfaceModeSkillExec {
		t.Error("web_query should be skill_exec")
	}
	if NativeSurfaceModeForSkill(CanonicalUnknown) != NativeSurfaceModeLegacy {
		t.Error("unknown should fallback to legacy")
	}
}

func TestBuildDiscoveryDecision_PreservesLegacyExecCollapseWhenDynamicExposureDisabled(t *testing.T) {
	decision := BuildDiscoveryDecision(Decision{
		SelectedSkill: "web_search",
		Reason:        "ir_ranked",
	}, false)

	if decision.CanonicalTarget != CanonicalWebQuery {
		t.Fatalf("CanonicalTarget = %q, want %q", decision.CanonicalTarget, CanonicalWebQuery)
	}
	if decision.NativeSurfaceMode != NativeSurfaceModeSkillExec {
		t.Fatalf("NativeSurfaceMode = %q, want %q", decision.NativeSurfaceMode, NativeSurfaceModeSkillExec)
	}
	if decision.ExecutionProfile != ExecutionProfilePreferFork {
		t.Fatalf("ExecutionProfile = %q, want %q", decision.ExecutionProfile, ExecutionProfilePreferFork)
	}
}

func TestBuildDiscoveryDecision_UsesCanonicalCutoverOnlyForEligibleDynamicRoutes(t *testing.T) {
	eligible := BuildDiscoveryDecision(Decision{SelectedSkill: "search"}, true)
	if eligible.CanonicalTarget != CanonicalWebQuery {
		t.Fatalf("eligible CanonicalTarget = %q, want %q", eligible.CanonicalTarget, CanonicalWebQuery)
	}
	if eligible.NativeSurfaceMode != NativeSurfaceModeSkillExec {
		t.Fatalf("eligible NativeSurfaceMode = %q, want %q", eligible.NativeSurfaceMode, NativeSurfaceModeSkillExec)
	}

	ineligible := BuildDiscoveryDecision(Decision{SelectedSkill: "reminder"}, true)
	if ineligible.CanonicalTarget != CanonicalUnknown {
		t.Fatalf("ineligible CanonicalTarget = %q, want %q", ineligible.CanonicalTarget, CanonicalUnknown)
	}
	if ineligible.NativeSurfaceMode != NativeSurfaceModeLegacy {
		t.Fatalf("ineligible NativeSurfaceMode = %q, want %q", ineligible.NativeSurfaceMode, NativeSurfaceModeLegacy)
	}
}

func TestBuildDiscoveryDecision_ClarifyOverridesCutover(t *testing.T) {
	decision := BuildDiscoveryDecision(Decision{
		SelectedSkill: "web_search",
		NeedClarify:   true,
		Reason:        "mixed_local_and_web",
	}, true)

	if !decision.NeedClarify {
		t.Fatal("NeedClarify = false, want true")
	}
	if decision.NativeSurfaceMode != NativeSurfaceModeClarifyNone {
		t.Fatalf("NativeSurfaceMode = %q, want %q", decision.NativeSurfaceMode, NativeSurfaceModeClarifyNone)
	}
}

func TestCapabilityDiscoveryDecisionToObservation(t *testing.T) {
	observation := BuildDiscoveryDecision(Decision{
		SelectedSkill: "deep_research",
		Reason:        "ir_ranked",
	}, true).ToObservation()

	if observation.SelectedCanonicalSkill != string(CanonicalDeepResearch) {
		t.Fatalf("SelectedCanonicalSkill = %q, want %q", observation.SelectedCanonicalSkill, CanonicalDeepResearch)
	}
	if observation.SelectedAlias != "deep_research" {
		t.Fatalf("SelectedAlias = %q, want deep_research", observation.SelectedAlias)
	}
	if observation.ExecutionProfile != ExecutionProfileRequireFork {
		t.Fatalf("ExecutionProfile = %q, want %q", observation.ExecutionProfile, ExecutionProfileRequireFork)
	}
	if !observation.SkillExecCutover {
		t.Fatal("SkillExecCutover = false, want true")
	}
	if !observation.ForkedSkillExecution {
		t.Fatal("ForkedSkillExecution = false, want true")
	}
}
