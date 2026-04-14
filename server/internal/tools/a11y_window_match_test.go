package tools

import (
	"testing"

	a11yruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a11y"
)

func TestResolveA11yWindowTarget_PrefersExplicitIDMatch(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Feishu", AppName: "Feishu"},
		{ID: "win-2", Title: "Code", AppName: "Code"},
	}

	resolved, err := resolveA11yWindowTarget("win-2", "", "", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
}

func TestResolveA11yWindowTarget_UsesUniqueExactWindowTitle(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Code", AppName: "Code"},
		{ID: "win-2", Title: "Feishu", AppName: "Feishu"},
	}

	resolved, err := resolveA11yWindowTarget("", "  feishu  ", "", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
}

func TestResolveA11yWindowTarget_UsesUniqueExactAppName(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Orca Chat", AppName: "Feishu"},
		{ID: "win-2", Title: "Code", AppName: "Code"},
	}

	resolved, err := resolveA11yWindowTarget("", "", "feishu", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-1" {
		t.Fatalf("resolved = %q, want win-1", resolved)
	}
}

func TestResolveA11yWindowTarget_UsesUniqueFuzzyWindowTitleAcrossAliasTerms(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Code", AppName: "Code"},
		{ID: "win-2", Title: "Lark - Orca", AppName: "Lark"},
	}

	resolved, err := resolveA11yWindowTarget("", "Feishu，飞书，Lark", "", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
}

func TestResolveA11yWindowTarget_UsesUniqueFuzzyWindowTitleAcrossSpaceSeparatedAliases(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Code", AppName: "Code"},
		{ID: "win-2", Title: "Lark - Orca", AppName: "Lark"},
	}

	resolved, err := resolveA11yWindowTarget("", "Feishu 飞书 Lark", "", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
}

func TestResolveA11yWindowTarget_UsesUniqueFuzzyAppNameAcrossAliasTerms(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Orca Chat", AppName: "Lark"},
		{ID: "win-2", Title: "Code", AppName: "Code"},
	}

	resolved, err := resolveA11yWindowTarget("", "", "Feishu，飞书，Lark", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-1" {
		t.Fatalf("resolved = %q, want win-1", resolved)
	}
}

func TestResolveA11yWindowTarget_AllowsWindowIDAliasForUniqueExactMatch(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Code", AppName: "Code"},
		{ID: "win-2", Title: "Feishu", AppName: "Feishu"},
	}

	resolved, err := resolveA11yWindowTarget("Feishu", "", "", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
}

func TestResolveA11yWindowTarget_RejectsAmbiguousExactAlias(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Feishu", AppName: "Feishu"},
		{ID: "win-2", Title: "Feishu", AppName: "Feishu"},
	}

	_, err := resolveA11yWindowTarget("", "Feishu", "", windows)
	if err == nil {
		t.Fatal("resolveA11yWindowTarget() error = nil, want ambiguity failure")
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
	if runtimeErr.Message != "target window is ambiguous" {
		t.Fatalf("message = %q, want target window is ambiguous", runtimeErr.Message)
	}
}

func TestResolveA11yWindowTarget_UsesFocusedWindowToBreakExactAmbiguity(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Feishu", AppName: "Feishu"},
		{ID: "win-2", Title: "Feishu", AppName: "Feishu", Focused: true},
	}

	resolved, err := resolveA11yWindowTarget("", "Feishu", "", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
}

func TestResolveA11yWindowTarget_RejectsAmbiguousFuzzyWindowTitle(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Feishu - Orca", AppName: "Lark"},
		{ID: "win-2", Title: "Lark - Team", AppName: "Lark"},
	}

	_, err := resolveA11yWindowTarget("", "Feishu，飞书，Lark", "", windows)
	if err == nil {
		t.Fatal("resolveA11yWindowTarget() error = nil, want ambiguity failure")
	}
	runtimeErr, ok := err.(*a11yruntime.RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Message != "target window is ambiguous" {
		t.Fatalf("message = %q, want target window is ambiguous", runtimeErr.Message)
	}
}

func TestResolveA11yWindowTarget_UsesFocusedWindowToBreakBestFuzzyTie(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Feishu - Team", AppName: "Lark"},
		{ID: "win-2", Title: "Lark - Orca", AppName: "Lark", Focused: true},
	}

	resolved, err := resolveA11yWindowTarget("", "Feishu，飞书，Lark", "", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
}

func TestResolveA11yWindowTarget_ResolvesBestFuzzyWindowTitleCandidateByMatchedTerms(t *testing.T) {
	windows := []a11yruntime.WindowInfo{
		{ID: "win-1", Title: "Feishu - Docs", AppName: "Lark"},
		{ID: "win-2", Title: "Lark Feishu Orca", AppName: "Lark"},
	}

	resolved, err := resolveA11yWindowTarget("", "Feishu Lark Orca", "", windows)
	if err != nil {
		t.Fatalf("resolveA11yWindowTarget() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
}
