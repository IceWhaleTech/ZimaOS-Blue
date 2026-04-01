package agent

import (
	"context"
	"strings"
	"testing"
)

func TestRunner_GeneratePlan_FallsBackWhenPlannerReturnsPlainText(t *testing.T) {
	cases := []struct {
		name     string
		goal     string
		wantText string
	}{
		{
			name:     "web_query localized",
			goal:     "Vyhledej nejnovejsi dokumentaci k OpenAI Responses API.",
			wantText: "use web_query",
		},
		{
			name:     "workspace localized",
			goal:     "看下 workspace 里的 README，总结一下项目是做什么的。",
			wantText: "workspace files",
		},
		{
			name:     "analyze url",
			goal:     "总结 https://example.com/blog 并提炼关键要点。",
			wantText: "public content",
		},
		{
			name:     "reminder localized",
			goal:     "帮我明早 9 点提醒我发送周报。",
			wantText: "reminder capability",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			llmStub := &captureResponseLLM{response: "This documentation belongs to Cursor, an AI code editor, not ZimaOS."}
			runner := &Runner{llm: llmStub}

			plan, err := runner.generatePlan(context.Background(), tc.goal, "")
			if err != nil {
				t.Fatalf("generatePlan returned unexpected error: %v", err)
			}
			if plan == nil || len(plan.Steps) != 3 {
				t.Fatalf("plan = %#v, want 3 fallback steps", plan)
			}
			if plan.Goal != tc.goal {
				t.Fatalf("plan.Goal = %q, want %q", plan.Goal, tc.goal)
			}
			var planText strings.Builder
			for _, step := range plan.Steps {
				planText.WriteString(step.Description)
				planText.WriteByte('\n')
			}
			if !strings.Contains(strings.ToLower(planText.String()), strings.ToLower(tc.wantText)) {
				t.Fatalf("plan text = %q, want substring %q", planText.String(), tc.wantText)
			}
			if len(plan.SuccessCriteria) == 0 || len(plan.FallbackPlan) == 0 {
				t.Fatalf("expected fallback plan to keep success criteria and fallback plan, got %#v", plan)
			}
		})
	}
}
