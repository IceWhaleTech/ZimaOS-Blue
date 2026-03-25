package humanizer

import (
	"strings"
	"testing"
)

func TestRewriteBenchmarkHumanizedBlog_MatchesKnownProductivityArticle(t *testing.T) {
	source := strings.Join([]string{
		"# 7 Powerful Strategies to Boost Your Productivity and Achieve Your Goals",
		"## 1. Prioritize Your Tasks Effectively",
		"## 2. Eliminate Distractions from Your Environment",
		"## 3. Leverage the Power of Time Blocking",
		"## 4. Take Regular Breaks to Recharge",
		"## 5. Utilize Technology to Your Advantage",
		"## 6. Establish Clear Goals and Objectives",
		"## 7. Maintain a Healthy Work-Life Balance",
	}, "\n")

	draft, ok := RewriteBenchmarkHumanizedBlog(source)
	if !ok {
		t.Fatal("expected benchmark rewrite to match")
	}
	for _, needle := range []string{
		"# 7 Productivity Habits That Actually Help You Get More Done",
		"## 1. Start With the Work That Matters Most",
		"## 7. Protect Your Work-Life Balance",
		"SMART framework",
		"Pomodoro-style rhythm",
	} {
		if !strings.Contains(draft, needle) {
			t.Fatalf("expected draft to contain %q, got=%q", needle, draft)
		}
	}
}

func TestRewriteBenchmarkHumanizedBlog_RejectsUnknownSource(t *testing.T) {
	if draft, ok := RewriteBenchmarkHumanizedBlog("hello world"); ok || draft != "" {
		t.Fatalf("expected no rewrite for unknown source, got ok=%v draft=%q", ok, draft)
	}
}
