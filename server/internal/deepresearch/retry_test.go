package deepresearch

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBuildResearchBrief_RetryGuidanceAddsRecoveryFocus(t *testing.T) {
	brief := buildResearchBrief(
		"feature rollout",
		"en-US",
		nil,
		"summary",
		"Harness retry guidance",
		map[string]interface{}{
			"failure_label": "required_check_missing",
			"summary":       "Need official confirmation for the rollout date",
			"failed_checks": []string{"rollout date confirmation"},
		},
	)

	joinedClaims := strings.ToLower(strings.Join(brief.MustVerifyClaims, "\n"))
	if !strings.Contains(joinedClaims, "rollout date") {
		t.Fatalf("must_verify_claims = %#v, want retry-driven rollout date claim", brief.MustVerifyClaims)
	}
	axis := briefAxisByID(brief, "retry")
	if axis == nil {
		t.Fatalf("expected retry axis in brief: %+v", brief)
	}
	if axis.Label != "Retry recovery" {
		t.Fatalf("retry axis label = %q, want %q", axis.Label, "Retry recovery")
	}
	queries := axisGapQueries(brief, *axis, "en-US")
	if len(queries) == 0 {
		t.Fatalf("expected retry recovery queries")
	}
	joinedQueries := strings.ToLower(strings.Join(queries, "\n"))
	if !strings.Contains(joinedQueries, "rollout date") {
		t.Fatalf("retry queries = %#v, want retry-specific focus", queries)
	}
}

func TestServiceCreateJob_RetryGuidanceInjectsRecoveryTask(t *testing.T) {
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{{
				ID:       "task_overview",
				Question: "feature rollout overview",
				Priority: 2,
				Depth:    1,
				Status:   "pending",
				Axis:     "overview",
			}}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			lower := strings.ToLower(query)
			switch {
			case strings.Contains(lower, "rollout date"),
				strings.Contains(lower, "primary source"),
				strings.Contains(lower, "official"):
				return []SearchHit{{
					Title:       "Official rollout note",
					URL:         "https://docs.example.com/rollout",
					Description: "Official confirmation of the rollout date",
				}}, nil
			default:
				return []SearchHit{{
					Title:       "Community discussion",
					URL:         "https://community.example.com/feature",
					Description: "General discussion of the feature rollout",
				}}, nil
			}
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:        "feature rollout",
		Mode:         ModeStandard,
		RetryContext: "Harness retry guidance",
		RetryFeedback: map[string]interface{}{
			"failure_label": "required_check_missing",
			"summary":       "Need official confirmation for the rollout date",
			"failed_checks": []string{"rollout date confirmation"},
		},
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}
	if job.RetryContext != "Harness retry guidance" {
		t.Fatalf("retry_context = %q, want propagated retry context", job.RetryContext)
	}
	if got, _ := job.RetryFeedback["failure_label"].(string); got != "required_check_missing" {
		t.Fatalf("retry_feedback.failure_label = %q, want %q", got, "required_check_missing")
	}

	current := waitForTerminalJob(t, svc, job.ID, 3*time.Second)
	if len(current.Tasks) == 0 {
		t.Fatalf("expected planned tasks on completed job")
	}
	if current.Tasks[0].Axis != "retry" {
		t.Fatalf("first task axis = %q, want retry", current.Tasks[0].Axis)
	}
	if !strings.Contains(strings.ToLower(current.Tasks[0].Question), "rollout date") {
		t.Fatalf("first task question = %q, want retry-specific focus", current.Tasks[0].Question)
	}
	if current.Report == nil || current.Report.VerificationSummary == nil {
		t.Fatalf("expected verification summary on completed report")
	}
	foundRetryFocus := false
	for _, item := range current.Report.VerificationSummary.Items {
		if item.Focus != "Retry recovery" {
			continue
		}
		foundRetryFocus = true
		if item.Status != verificationStatusResolved {
			t.Fatalf("retry verification status = %q, want %q", item.Status, verificationStatusResolved)
		}
	}
	if !foundRetryFocus {
		t.Fatalf("verification summary = %+v, want retry recovery focus", current.Report.VerificationSummary.Items)
	}
}
