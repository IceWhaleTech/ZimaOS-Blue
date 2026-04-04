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
		{"ask", CanonicalAsk, true},
		{"browser", CanonicalBrowser, true},
		{"analyze", CanonicalAnalyze, true},
		{"reminder", CanonicalReminder, true},
		{"ui_reviewer", CanonicalUIReviewer, true},
		{"himalaya", CanonicalHimalaya, true},
		{"deep_research", CanonicalDeepResearch, true},
		{"config", CanonicalConfig, true},
		{"mgmt", CanonicalConfig, true},
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
	if !IsCutoverEligibleCanonical(CanonicalAsk) {
		t.Error("ask should be cutover eligible")
	}
	if !IsCutoverEligibleCanonical(CanonicalDeepResearch) {
		t.Error("deep_research should be cutover eligible")
	}
	if !IsCutoverEligibleCanonical(CanonicalReminder) {
		t.Error("reminder should be cutover eligible")
	}
	if !IsCutoverEligibleCanonical(CanonicalUIReviewer) {
		t.Error("ui_reviewer should be cutover eligible")
	}
	if !IsCutoverEligibleCanonical(CanonicalHimalaya) {
		t.Error("himalaya should be cutover eligible")
	}
	if !IsCutoverEligibleCanonical(CanonicalConfig) {
		t.Error("config should be cutover eligible")
	}
	if !IsCutoverEligibleCanonical(CanonicalExec) {
		t.Error("exec should be cutover eligible for workspace/local cutover")
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
	if ExecutionProfileForSkill(CanonicalReminder) != ExecutionProfileInline {
		t.Error("reminder should be inline")
	}
	if ExecutionProfileForSkill(CanonicalUIReviewer) != ExecutionProfileInline {
		t.Error("ui_reviewer should be inline")
	}
	if ExecutionProfileForSkill(CanonicalHimalaya) != ExecutionProfileInline {
		t.Error("himalaya should be inline")
	}
	if ExecutionProfileForSkill(CanonicalAsk) != ExecutionProfileInline {
		t.Error("ask should be inline")
	}
	if ExecutionProfileForSkill(CanonicalConfig) != ExecutionProfileInline {
		t.Error("config should be inline")
	}
}

func TestNativeSurfaceModeForSkill(t *testing.T) {
	if NativeSurfaceModeForSkill(CanonicalWebQuery) != NativeSurfaceModeSkillExec {
		t.Error("web_query should be skill_exec")
	}
	if NativeSurfaceModeForSkill(CanonicalAsk) != NativeSurfaceModeSkillExec {
		t.Error("ask should be skill_exec")
	}
	if NativeSurfaceModeForSkill(CanonicalReminder) != NativeSurfaceModeSkillExec {
		t.Error("reminder should be skill_exec")
	}
	if NativeSurfaceModeForSkill(CanonicalUIReviewer) != NativeSurfaceModeSkillExec {
		t.Error("ui_reviewer should be skill_exec")
	}
	if NativeSurfaceModeForSkill(CanonicalHimalaya) != NativeSurfaceModeSkillExec {
		t.Error("himalaya should be skill_exec")
	}
	if NativeSurfaceModeForSkill(CanonicalConfig) != NativeSurfaceModeSkillExec {
		t.Error("config should be skill_exec")
	}
	if NativeSurfaceModeForSkill(CanonicalExec) != NativeSurfaceModeSkillExec {
		t.Error("exec should be skill_exec")
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

	reminder := BuildDiscoveryDecision(Decision{SelectedSkill: "reminder"}, true)
	if reminder.CanonicalTarget != CanonicalReminder {
		t.Fatalf("reminder CanonicalTarget = %q, want %q", reminder.CanonicalTarget, CanonicalReminder)
	}
	if reminder.NativeSurfaceMode != NativeSurfaceModeSkillExec {
		t.Fatalf("reminder NativeSurfaceMode = %q, want %q", reminder.NativeSurfaceMode, NativeSurfaceModeSkillExec)
	}

	uiReviewer := BuildDiscoveryDecision(Decision{SelectedSkill: "ui_reviewer"}, true)
	if uiReviewer.CanonicalTarget != CanonicalUIReviewer {
		t.Fatalf("ui_reviewer CanonicalTarget = %q, want %q", uiReviewer.CanonicalTarget, CanonicalUIReviewer)
	}
	if uiReviewer.NativeSurfaceMode != NativeSurfaceModeSkillExec {
		t.Fatalf("ui_reviewer NativeSurfaceMode = %q, want %q", uiReviewer.NativeSurfaceMode, NativeSurfaceModeSkillExec)
	}

	himalaya := BuildDiscoveryDecision(Decision{SelectedSkill: "himalaya"}, true)
	if himalaya.CanonicalTarget != CanonicalHimalaya {
		t.Fatalf("himalaya CanonicalTarget = %q, want %q", himalaya.CanonicalTarget, CanonicalHimalaya)
	}
	if himalaya.NativeSurfaceMode != NativeSurfaceModeSkillExec {
		t.Fatalf("himalaya NativeSurfaceMode = %q, want %q", himalaya.NativeSurfaceMode, NativeSurfaceModeSkillExec)
	}

	ineligible := BuildDiscoveryDecision(Decision{SelectedSkill: "unknown_skill"}, true)
	if ineligible.CanonicalTarget != CanonicalUnknown {
		t.Fatalf("ineligible CanonicalTarget = %q, want %q", ineligible.CanonicalTarget, CanonicalUnknown)
	}
	if ineligible.NativeSurfaceMode != NativeSurfaceModeLegacy {
		t.Fatalf("ineligible NativeSurfaceMode = %q, want %q", ineligible.NativeSurfaceMode, NativeSurfaceModeLegacy)
	}
}

func TestBuildDiscoveryDecision_CutoverCoversAskConfigAndExecRoutes(t *testing.T) {
	tests := []struct {
		name        string
		selected    string
		wantTarget  CanonicalSkillID
		wantProfile ExecutionProfile
	}{
		{name: "ask", selected: "ask", wantTarget: CanonicalAsk, wantProfile: ExecutionProfileInline},
		{name: "reminder", selected: "reminder", wantTarget: CanonicalReminder, wantProfile: ExecutionProfileInline},
		{name: "config", selected: "config", wantTarget: CanonicalConfig, wantProfile: ExecutionProfileInline},
		{name: "mgmt_alias", selected: "mgmt", wantTarget: CanonicalConfig, wantProfile: ExecutionProfileInline},
		{name: "ui_reviewer", selected: "ui_reviewer", wantTarget: CanonicalUIReviewer, wantProfile: ExecutionProfileInline},
		{name: "himalaya", selected: "himalaya", wantTarget: CanonicalHimalaya, wantProfile: ExecutionProfileInline},
		{name: "exec", selected: "exec", wantTarget: CanonicalExec, wantProfile: ExecutionProfileInline},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision := BuildDiscoveryDecision(Decision{SelectedSkill: tc.selected}, true)
			if decision.CanonicalTarget != tc.wantTarget {
				t.Fatalf("CanonicalTarget = %q, want %q", decision.CanonicalTarget, tc.wantTarget)
			}
			if decision.ExecutionProfile != tc.wantProfile {
				t.Fatalf("ExecutionProfile = %q, want %q", decision.ExecutionProfile, tc.wantProfile)
			}
			if decision.NativeSurfaceMode != NativeSurfaceModeSkillExec {
				t.Fatalf("NativeSurfaceMode = %q, want %q", decision.NativeSurfaceMode, NativeSurfaceModeSkillExec)
			}
		})
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
