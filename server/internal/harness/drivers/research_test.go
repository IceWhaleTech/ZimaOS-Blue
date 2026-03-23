package drivers

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type testResearchSearcher struct{}

func (testResearchSearcher) Search(ctx context.Context, query string, maxResults int, lang string) ([]deepresearch.SearchHit, error) {
	return []deepresearch.SearchHit{{
		Title:       "Official rollout note",
		URL:         "https://docs.example.com/rollout",
		Description: "Official confirmation of the rollout date",
	}}, nil
}

func waitForDeepResearchJob(t *testing.T, svc *deepresearch.Service, jobID string) *deepresearch.Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		current, err := svc.GetJob(jobID)
		if err != nil {
			t.Fatalf("GetJob(%q) failed: %v", jobID, err)
		}
		switch current.Status {
		case deepresearch.JobStatusCompleted, deepresearch.JobStatusFailed, deepresearch.JobStatusCancelled:
			return current
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for job %q, status=%s", jobID, current.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestResearchDriverStart_PassesRetryMetadataToService(t *testing.T) {
	svc := deepresearch.NewService(deepresearch.NewHeuristicPlanner(), testResearchSearcher{})
	driver := &ResearchDriver{service: svc}

	run := &harness.Run{
		ID:     "research-retry-job",
		UserID: "user-1",
		Goal:   "feature rollout",
		Metadata: map[string]interface{}{
			"retry_context": "Harness retry guidance",
			"retry_feedback": map[string]interface{}{
				"failure_label": "required_check_missing",
				"summary":       "Need official confirmation for the rollout date",
				"failed_checks": []string{"rollout date confirmation"},
			},
		},
	}

	if err := driver.Start(context.Background(), run, harness.RunEnv{}); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	current := waitForDeepResearchJob(t, svc, run.ID)
	if current.RetryContext != "Harness retry guidance" {
		t.Fatalf("retry_context = %q, want propagated retry context", current.RetryContext)
	}
	if got, _ := current.RetryFeedback["failure_label"].(string); got != "required_check_missing" {
		t.Fatalf("retry_feedback.failure_label = %q, want %q", got, "required_check_missing")
	}
}
