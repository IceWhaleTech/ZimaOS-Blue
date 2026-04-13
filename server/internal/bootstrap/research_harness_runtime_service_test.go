package bootstrap

import (
	"context"
	"fmt"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type bootstrapResearchSnapshotDriver struct {
	status   harness.RunStatus
	progress int
	metadata map[string]interface{}
	result   string
}

func cloneHarnessResearchTestMetadataMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return map[string]interface{}{}
	}
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func (d *bootstrapResearchSnapshotDriver) Kind() harness.RunKind { return harness.RunKindResearch }

func (d *bootstrapResearchSnapshotDriver) Validate(spec harness.RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *bootstrapResearchSnapshotDriver) Start(ctx context.Context, run *harness.Run, env harness.RunEnv) error {
	snapshot := *run
	snapshot.Metadata = cloneHarnessResearchTestMetadataMap(run.Metadata)
	for key, value := range d.metadata {
		snapshot.Metadata[key] = value
	}
	snapshot.Status = d.status
	snapshot.Progress = d.progress
	snapshot.Result = d.result
	return env.Manager.SyncSnapshot(ctx, &snapshot)
}

func (d *bootstrapResearchSnapshotDriver) Cancel(_ context.Context, _ *harness.Run) error { return nil }

func TestHarnessResearchRuntimeService_UsesHarnessRunsForAnalyzeJobs(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	bundle.Controller.RegisterDriver(&bootstrapResearchSnapshotDriver{
		status:   harness.RunStatusExecuting,
		progress: 42,
		metadata: map[string]interface{}{
			"mode":          "analyze",
			"stage":         "analysis",
			"latest_action": "Analyze sources",
			"iteration":     1,
		},
	})

	run, err := bundle.Controller.Submit(context.Background(), harness.RunSpec{
		Kind:           harness.RunKindResearch,
		Goal:           "Summarize the release notes",
		UserID:         "user-harness-research",
		ConversationID: "conv-harness-research",
		ProviderID:     "openai-prod",
		Metadata: map[string]interface{}{
			"mode":  "analyze",
			"topic": "Release notes summary",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	service := newHarnessResearchRuntimeService(bundle.Controller, deepresearch.NewService(nil, nil))
	jobs, err := service.ListJobsForUser("user-harness-research", "", true)
	if err != nil {
		t.Fatalf("ListJobsForUser failed: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("active jobs = %d, want 1", len(jobs))
	}
	if jobs[0].JobID != run.ID || jobs[0].Query != "Summarize the release notes" {
		t.Fatalf("job summary = %#v, want run-backed analyze summary", jobs[0])
	}
	if jobs[0].Status != deepresearch.JobStatusRunning {
		t.Fatalf("job status = %q, want %q", jobs[0].Status, deepresearch.JobStatusRunning)
	}
	if jobs[0].Stage != "analysis" || jobs[0].LatestAction != "Analyze sources" {
		t.Fatalf("job summary = %#v, want projected stage/latest_action", jobs[0])
	}

	job, err := service.GetJobForUser(run.ID, "user-harness-research", "")
	if err != nil {
		t.Fatalf("GetJobForUser failed: %v", err)
	}
	if job.ID != run.ID || string(job.Mode) != "analyze" {
		t.Fatalf("job = %#v, want analyze-mode harness projection", job)
	}
	if job.ProviderID != "openai-prod" {
		t.Fatalf("job provider_id = %q, want openai-prod", job.ProviderID)
	}
	if job.Progress != 42 || job.Stage != "analysis" {
		t.Fatalf("job = %#v, want projected progress/stage", job)
	}

	if err := service.CancelJobForUser(run.ID, "user-harness-research", ""); err != nil {
		t.Fatalf("CancelJobForUser failed: %v", err)
	}
	cancelled, err := bundle.Controller.GetStored(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("GetStored failed: %v", err)
	}
	if cancelled.Status != harness.RunStatusCancelled {
		t.Fatalf("cancelled status = %s, want cancelled", cancelled.Status)
	}
}

func TestHarnessResearchRuntimeService_ProjectsStructuredRecentProfileReport(t *testing.T) {
	db, bundle, _, _ := newTestAgentRuntimeFixture(t)
	defer db.Close()

	bundle.Controller.RegisterDriver(&bootstrapResearchSnapshotDriver{
		status:   harness.RunStatusCompleted,
		progress: 100,
		metadata: map[string]interface{}{
			"mode":              "deep_research",
			"research_depth":    "deep",
			"retrieval_profile": "recent_multi_site_v1",
			"stage":             "complete",
		},
		result: `{"answer":"Community discussion is positive overall.","confidence":0.74,"retrieval_profile":"recent_multi_site_v1","lookback_days":30,"browser_assisted":true,"items_by_source":{"reddit":[{"source":"reddit","title":"Thread","url":"https://reddit.com/r/test"}]},"errors_by_source":{"github":"rate limited"},"clusters":[{"label":"performance","item_count":2}]}`,
	})

	run, err := bundle.Controller.Submit(context.Background(), harness.RunSpec{
		Kind:           harness.RunKindResearch,
		Goal:           "What are people saying in the last 30 days about ZimaOS Blue?",
		UserID:         "user-recent-research",
		ConversationID: "conv-recent-research",
		Metadata: map[string]interface{}{
			"mode":              "deep_research",
			"research_depth":    "deep",
			"retrieval_profile": "recent_multi_site_v1",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	service := newHarnessResearchRuntimeService(bundle.Controller, deepresearch.NewService(nil, nil))
	job, err := service.GetJobForUser(run.ID, "user-recent-research", "")
	if err != nil {
		t.Fatalf("GetJobForUser failed: %v", err)
	}
	if job.Report == nil {
		t.Fatal("expected structured report to be projected")
	}
	if job.Report.Answer != "Community discussion is positive overall." {
		t.Fatalf("report answer = %q, want structured answer", job.Report.Answer)
	}
	if job.Report.RetrievalProfile != "recent_multi_site_v1" {
		t.Fatalf("report retrieval_profile = %q, want recent_multi_site_v1", job.Report.RetrievalProfile)
	}
	if job.Report.LookbackDays != 30 {
		t.Fatalf("report lookback_days = %d, want 30", job.Report.LookbackDays)
	}
	if !job.Report.BrowserAssisted {
		t.Fatal("expected browser_assisted to be projected")
	}
	if len(job.Report.ItemsBySource) != 1 {
		t.Fatalf("items_by_source = %#v, want projected items", job.Report.ItemsBySource)
	}
	if got := job.Report.ErrorsBySource["github"]; got != "rate limited" {
		t.Fatalf("errors_by_source.github = %#v, want rate limited", got)
	}
	if len(job.Report.Clusters) != 1 {
		t.Fatalf("clusters = %#v, want 1 cluster", job.Report.Clusters)
	}
}

func TestResearchRunResultReport_ParsesStructuredRecentReportWithoutAnswer(t *testing.T) {
	report := researchRunResultReport(`{"retrieval_profile":"recent_multi_site_v1","lookback_days":30,"items_by_source":{"reddit":[{"title":"Thread"}]}}`)
	if report == nil {
		t.Fatal("expected report")
	}
	if report.RetrievalProfile != "recent_multi_site_v1" {
		t.Fatalf("retrieval_profile = %q, want recent_multi_site_v1", report.RetrievalProfile)
	}
	if report.LookbackDays != 30 {
		t.Fatalf("lookback_days = %d, want 30", report.LookbackDays)
	}
	if len(report.ItemsBySource) != 1 {
		t.Fatalf("items_by_source = %#v, want parsed map", report.ItemsBySource)
	}
}
