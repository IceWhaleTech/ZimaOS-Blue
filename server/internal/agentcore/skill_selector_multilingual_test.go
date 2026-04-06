package agentcore

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
)

func TestSkillSelector_MultilingualCuratedRoutes(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkill(t, workspaceDir, "web_query", "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
	writeSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")
	writeSelectorSkill(t, workspaceDir, "reminder", "schedule reminders and user notifications at a specific time", `blue reminder add message="Standup" time="2026-03-01 09:00"`, "reminder", "notify", "schedule")
	writeSelectorSkill(t, workspaceDir, "browser", "browse urls and interact with web pages", "blue browser.navigate url=https://example.com", "browser", "web")
	writeSelectorSkill(t, workspaceDir, "ui_reviewer", "review screenshots and UI layouts for accessibility and visual issues", `blue ui_reviewer target="screenshot.png"`, "ui", "review", "screenshot")

	selector := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	cases := []struct {
		skill         string
		wantSkill     string
		wantResearch  string
	}{
		{skill: "web_query", wantSkill: "web_query"},
		{skill: "analyze", wantSkill: "research", wantResearch: "analyze"},
		{skill: "reminder", wantSkill: "reminder"},
		{skill: "browser", wantSkill: "browser"},
		{skill: "ui_reviewer", wantSkill: "research", wantResearch: "ui_review"},
	}

	for _, tc := range cases {
		examples := routingcue.LocalizedExamples(tc.skill)
		for _, example := range examples {
			decision, err := selector.Select(context.Background(), example.Query, SelectOptions{
				Mode:                SkillSelectorModeHybrid,
				EnableRerank:        true,
				ConfidenceThreshold: 0.78,
			})
			if err != nil {
				t.Fatalf("%s locale=%s select error: %v", tc.skill, example.Locale, err)
			}
			if decision.SelectedSkill != tc.wantSkill {
				t.Fatalf("%s locale=%s expected skill=%s got %+v", tc.skill, example.Locale, tc.wantSkill, decision)
			}
			if tc.wantResearch != "" && decision.ResearchMode != tc.wantResearch {
				t.Fatalf("%s locale=%s expected research_mode=%s got %+v", tc.skill, example.Locale, tc.wantResearch, decision)
			}
			if decision.NeedClarify {
				t.Fatalf("%s locale=%s should not need clarify, got %+v", tc.skill, example.Locale, decision)
			}
		}
	}
}

func TestSkillSelector_MultilingualURLAnalyzeBypassesBrowserRule(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkill(t, workspaceDir, "browser", "browse urls and interact with web pages", "blue browser.navigate url=https://example.com", "browser", "web")
	writeSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")

	selector := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	for _, example := range routingcue.LocalizedURLBypassExamples("analyze") {
		query := example.Query
		decision, err := selector.Select(context.Background(), query, SelectOptions{
			Mode:                SkillSelectorModeHybrid,
			EnableRerank:        true,
			ConfidenceThreshold: 0.78,
		})
		if err != nil {
			t.Fatalf("Select error for %q: %v", query, err)
		}
		if decision.SelectedSkill != "research" || decision.ResearchMode != "analyze" {
			t.Fatalf("expected research/analyze for %q, got %+v", query, decision)
		}
	}
}

func TestSkillSelector_MultilingualURLUIReviewBypassesBrowserRule(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSelectorSkill(t, workspaceDir, "browser", "browse urls and interact with web pages", "blue browser.navigate url=https://example.com", "browser", "web")
	writeSelectorSkill(t, workspaceDir, "ui_reviewer", "review screenshots and UI layouts for accessibility and visual issues", `blue ui_reviewer target="https://example.com"`, "ui", "review", "screenshot")

	selector := NewSkillSelector(workspaceDir, NewHeuristicSkillReranker())
	for _, example := range routingcue.LocalizedURLBypassExamples("ui_reviewer") {
		query := example.Query
		decision, err := selector.Select(context.Background(), query, SelectOptions{
			Mode:                SkillSelectorModeHybrid,
			EnableRerank:        true,
			ConfidenceThreshold: 0.78,
		})
		if err != nil {
			t.Fatalf("Select error for %q: %v", query, err)
		}
		if decision.SelectedSkill != "research" || decision.ResearchMode != "ui_review" {
			t.Fatalf("expected research/ui_review for %q, got %+v", query, decision)
		}
	}
}
