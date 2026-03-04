package deepresearch

import (
	"testing"
	"time"
)

func TestApplyEntityDisambiguation_StrictFilter(t *testing.T) {
	evidence := []Evidence{
		{ID: "ev1", TaskID: "t1", Title: "蓝驰创投付强分享出海观点", Snippet: "蓝驰 合伙人 付强", URL: "https://example.com/1"},
		{ID: "ev2", TaskID: "t1", Title: "某汽车公司付强担任总裁", Snippet: "与蓝驰无关", URL: "https://example.com/2"},
	}
	tasks := map[string]Task{
		"t1": {NegKeywords: []string{"汽车公司", "无关"}},
	}
	filtered, stats := applyEntityDisambiguation("蓝驰创投 付强 合伙人", true, 0.72, evidence, tasks)
	if len(filtered) != 1 {
		t.Fatalf("filtered len = %d, want 1", len(filtered))
	}
	if stats.FilteredCount == 0 {
		t.Fatalf("expected filtered count > 0")
	}
}

func TestAnalyzeEvidenceConsistencyByClaim(t *testing.T) {
	evidence := []Evidence{
		{Title: "Official update confirms rollout", Snippet: "confirmed by company", ClaimKey: "rollout|feature"},
		{Title: "Rumor says rollout is not happening", Snippet: "not confirmed", ClaimKey: "rollout|feature"},
	}
	support, conflict, hasConflict := analyzeEvidenceConsistencyByClaim(evidence)
	if support == 0 {
		t.Fatalf("support = %d, want > 0", support)
	}
	if conflict == 0 {
		t.Fatalf("conflict = %d, want > 0", conflict)
	}
	if !hasConflict {
		t.Fatalf("expected hasConflict=true")
	}
}

func TestExtractEvidenceYear(t *testing.T) {
	ts := time.Date(2022, 8, 1, 0, 0, 0, 0, time.UTC)
	ev1 := Evidence{PublishedAt: &ts}
	if got := extractEvidenceYear(ev1); got != 2022 {
		t.Fatalf("extractEvidenceYear(published_at)=%d, want 2022", got)
	}
	ev2 := Evidence{Title: "2024年观点更新"}
	if got := extractEvidenceYear(ev2); got != 2024 {
		t.Fatalf("extractEvidenceYear(title)=%d, want 2024", got)
	}
}

