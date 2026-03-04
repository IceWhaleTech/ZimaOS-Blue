package deepresearch

import "testing"

func TestLooksLikePersonTimelineResearch(t *testing.T) {
	if !looksLikePersonTimelineResearch("针对蓝驰合伙人付强，逐一调研他在不同时期的文章和观点", "zh-CN") {
		t.Fatalf("expected person timeline query to be detected")
	}
	if looksLikePersonTimelineResearch("帮我查一下今天上海天气", "zh-CN") {
		t.Fatalf("weather query should not be detected as person timeline research")
	}
}

func TestPlanPersonResearchTasks_ModeDeep(t *testing.T) {
	tasks := planPersonResearchTasks("针对付强做不同时期观点调研", ModeDeep, "zh-CN")
	if len(tasks) != 5 {
		t.Fatalf("tasks len = %d, want 5", len(tasks))
	}
	if tasks[0].Category != "identity_validation" {
		t.Fatalf("first task category = %q, want identity_validation", tasks[0].Category)
	}
	if tasks[1].TimeWindow == "" || tasks[2].TimeWindow == "" || tasks[3].TimeWindow == "" {
		t.Fatalf("expected timeline windows on period tasks")
	}
}

func TestPlanPersonResearchTasks_ModeFast(t *testing.T) {
	tasks := planPersonResearchTasks("付强 观点 演变", ModeFast, "zh-CN")
	if len(tasks) != 3 {
		t.Fatalf("tasks len = %d, want 3", len(tasks))
	}
}

