package scenecompose

import "testing"

func TestHeuristicPlannerExtractsSingleScene(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("一只猫在森林里，柔和阳光"))
	if got := plan.Background; got != "森林里，柔和阳光" && got != "森林里" && got == "" {
		t.Fatalf("background = %q, want non-empty forest phrase", got)
	}
	if len(plan.Foreground) != 1 {
		t.Fatalf("foreground count = %d, want 1", len(plan.Foreground))
	}
	if plan.Foreground[0].Type != "cat" {
		t.Fatalf("type = %q, want cat", plan.Foreground[0].Type)
	}
	if plan.Foreground[0].Layout.Depth != "midground" {
		t.Fatalf("depth = %q, want midground", plan.Foreground[0].Layout.Depth)
	}
}

func TestHeuristicPlannerExtractsMultiForegroundLayout(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("右边一只小狗，左边一只猫"))
	if len(plan.Foreground) != 2 {
		t.Fatalf("foreground count = %d, want 2", len(plan.Foreground))
	}
	if plan.Foreground[0].Layout.Horizontal != "right" {
		t.Fatalf("first horizontal = %q, want right", plan.Foreground[0].Layout.Horizontal)
	}
	if plan.Foreground[1].Layout.Horizontal != "left" {
		t.Fatalf("second horizontal = %q, want left", plan.Foreground[1].Layout.Horizontal)
	}
}

func TestHeuristicPlannerUsesCompiledCueMatchersForSceneTraits(t *testing.T) {
	plan := normalizeScenePlan(heuristicPlan("friendly small orange cartoon robot, city street, rainy night, top-down view, floating"))
	if plan.Background != "city street" {
		t.Fatalf("background = %q, want city street", plan.Background)
	}
	if plan.Style != "cartoon" {
		t.Fatalf("style = %q, want cartoon", plan.Style)
	}
	if plan.TimeOfDay != "night" {
		t.Fatalf("time_of_day = %q, want night", plan.TimeOfDay)
	}
	if plan.Weather != "rainy" {
		t.Fatalf("weather = %q, want rainy", plan.Weather)
	}
	if plan.CameraView != "top-down" {
		t.Fatalf("camera_view = %q, want top-down", plan.CameraView)
	}
	if len(plan.Foreground) != 1 {
		t.Fatalf("foreground count = %d, want 1", len(plan.Foreground))
	}
	if plan.Foreground[0].Type != "robot" {
		t.Fatalf("type = %q, want robot", plan.Foreground[0].Type)
	}
	if got := plan.Foreground[0].Attributes; len(got) != 3 || got[0] != "orange" || got[1] != "small" || got[2] != "friendly" {
		t.Fatalf("attributes = %v, want [orange small friendly]", got)
	}
	if plan.Foreground[0].Layout.Grounded {
		t.Fatal("expected floating subject not to be grounded")
	}
	if plan.Foreground[0].Layout.Vertical != "high" {
		t.Fatalf("vertical = %q, want high", plan.Foreground[0].Layout.Vertical)
	}
}
