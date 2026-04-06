package bootstrap

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type stubResearchHarnessSubmitter struct {
	run   *harness.Run
	err   error
	specs []harness.RunSpec
}

func (s *stubResearchHarnessSubmitter) Submit(_ context.Context, spec harness.RunSpec) (*harness.Run, error) {
	s.specs = append(s.specs, spec)
	return s.run, s.err
}

func TestSubmitResearchHarnessJobUsesExistingResearchJobWhenAvailable(t *testing.T) {
	service := deepresearch.NewService(nil, stubDeepResearchSearcher{})
	strictEntity := true
	if _, err := service.CreateJob(context.Background(), deepresearch.CreateJobRequest{
		RequestedID:    "run-existing",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Query:          "research blue runtime",
	}); err != nil {
		t.Fatalf("CreateJob(existing): %v", err)
	}

	submitter := &stubResearchHarnessSubmitter{
		run: &harness.Run{
			ID:             "run-existing",
			UserID:         "user-1",
			ConversationID: "conv-1",
			Goal:           "research blue runtime",
			Status:         harness.RunStatusExecuting,
		},
	}

	job, run, err := submitResearchHarnessJob(context.Background(), submitter, service, deepresearch.CreateJobRequest{
		UserID:         "user-1",
		ConversationID: "conv-1",
		Query:          "research blue runtime",
		Mode:           deepresearch.ModeDeep,
		RouteMode:      deepresearch.RouteModeWeb,
		Lang:           "zh-CN",
		StrictEntity:   &strictEntity,
		TimeWindows:    []string{"2025"},
		ReportStyle:    "timeline",
		Budget:         &deepresearch.Budget{MaxSources: 7, MaxSeconds: 120},
	}, "/tmp/workspace")
	if err != nil {
		t.Fatalf("submitResearchHarnessJob: %v", err)
	}
	if run != submitter.run {
		t.Fatalf("run = %#v, want submitted run %#v", run, submitter.run)
	}
	if job == nil || job.ID != "run-existing" {
		t.Fatalf("job = %#v, want existing job", job)
	}
	if len(submitter.specs) != 1 {
		t.Fatalf("submit specs = %d, want 1", len(submitter.specs))
	}
	spec := submitter.specs[0]
	if spec.Kind != harness.RunKindResearch || spec.Goal != "research blue runtime" {
		t.Fatalf("spec = %#v, want research run with query", spec)
	}
	if spec.WorkspaceRoot != "/tmp/workspace" || spec.ConversationID != "conv-1" || spec.UserID != "user-1" {
		t.Fatalf("spec = %#v, want workspace/user/session binding", spec)
	}
	if got := spec.Metadata["mode"]; got != "deep_research" {
		t.Fatalf("spec.Metadata[mode] = %#v, want deep_research", got)
	}
	if got := spec.Metadata["research_depth"]; got != "deep" {
		t.Fatalf("spec.Metadata[research_depth] = %#v, want deep", got)
	}
	if got := spec.Metadata["max_sources"]; got != 7 {
		t.Fatalf("spec.Metadata[max_sources] = %#v, want 7", got)
	}
	if got := spec.Metadata["strict_entity"]; got != true {
		t.Fatalf("spec.Metadata[strict_entity] = %#v, want true", got)
	}
}

func TestSubmitResearchHarnessJobFallsBackToRunSnapshot(t *testing.T) {
	submitter := &stubResearchHarnessSubmitter{
		run: &harness.Run{
			ID:             "run-fallback",
			UserID:         "user-2",
			ConversationID: "conv-2",
			Goal:           "investigate runtime contract",
			Status:         harness.RunStatusPlanning,
		},
	}

	job, run, err := submitResearchHarnessJob(context.Background(), submitter, deepresearch.NewService(nil, stubDeepResearchSearcher{}), deepresearch.CreateJobRequest{
		UserID:         "user-2",
		ConversationID: "conv-2",
		Query:          "investigate runtime contract",
	}, "/tmp/workspace")
	if err != nil {
		t.Fatalf("submitResearchHarnessJob: %v", err)
	}
	if run != submitter.run {
		t.Fatalf("run = %#v, want submitted run %#v", run, submitter.run)
	}
	if job == nil || job.ID != "run-fallback" || job.Query != "investigate runtime contract" {
		t.Fatalf("job = %#v, want synthesized run snapshot", job)
	}
	if job.Status != deepresearch.JobStatus(harness.RunStatusPlanning) {
		t.Fatalf("job.Status = %q, want %q", job.Status, deepresearch.JobStatus(harness.RunStatusPlanning))
	}
}

func TestDeepResearchToolAdapterCreateJobFallsBackToService(t *testing.T) {
	service := deepresearch.NewService(nil, stubDeepResearchSearcher{})
	adapter := newDeepResearchToolAdapter(service, nil, "/tmp/workspace")

	job, err := adapter.CreateJob(context.Background(), tools.ResearchCreateJobRequest{
		UserID:         "user-3",
		ConversationID: "conv-3",
		Query:          "summarize blue runtime",
		Mode:           string(deepresearch.ModeDeep),
		RouteMode:      string(deepresearch.RouteModeWeb),
		TimeWindows:    []string{"2024", "2025"},
		Budget:         &tools.ResearchBudget{MaxSources: 4, MaxSeconds: 90},
		ReportStyle:    "summary",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if job == nil || job.Query != "summarize blue runtime" {
		t.Fatalf("job = %#v, want created research job", job)
	}
	if job.ID == "" || job.Status == "" {
		t.Fatalf("job = %#v, want populated id and status", job)
	}
}

func TestDeepResearchToolAdapterCreateJobUsesCanonicalHarnessSpecForAnalyze(t *testing.T) {
	submitter := &stubResearchHarnessSubmitter{
		run: &harness.Run{
			ID:             "run-analyze",
			UserID:         "user-4",
			ConversationID: "conv-4",
			Goal:           "Competitive pricing snapshot",
			Status:         harness.RunStatusPlanning,
			Metadata: map[string]interface{}{
				"mode":           "analyze",
				"topic":          "Competitive pricing snapshot",
				"urls":           []string{"https://example.com/pricing"},
				"search_queries": []string{"example pricing comparison"},
				"output_mode":    "report",
			},
		},
	}
	adapter := newDeepResearchToolAdapter(deepresearch.NewService(nil, stubDeepResearchSearcher{}), submitter, "/tmp/workspace")

	job, err := adapter.CreateJob(context.Background(), tools.ResearchCreateJobRequest{
		UserID:         "user-4",
		ConversationID: "conv-4",
		Mode:           "analyze",
		Topic:          "Competitive pricing snapshot",
		URLs:           []string{"https://example.com/pricing"},
		SearchQueries:  []string{"example pricing comparison"},
		OutputMode:     "report",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	if len(submitter.specs) != 1 {
		t.Fatalf("submit specs = %d, want 1", len(submitter.specs))
	}
	spec := submitter.specs[0]
	if spec.Kind != harness.RunKindResearch || spec.Goal != "Competitive pricing snapshot" {
		t.Fatalf("spec = %#v, want analyze harness run", spec)
	}
	if got := spec.Metadata["mode"]; got != "analyze" {
		t.Fatalf("spec.Metadata[mode] = %#v, want analyze", got)
	}
	if got := spec.Metadata["topic"]; got != "Competitive pricing snapshot" {
		t.Fatalf("spec.Metadata[topic] = %#v, want forwarded topic", got)
	}
	if got := spec.Metadata["output_mode"]; got != "report" {
		t.Fatalf("spec.Metadata[output_mode] = %#v, want report", got)
	}
	urls, ok := spec.Metadata["urls"].([]string)
	if !ok || len(urls) != 1 || urls[0] != "https://example.com/pricing" {
		t.Fatalf("spec.Metadata[urls] = %#v, want forwarded urls", spec.Metadata["urls"])
	}
	if job == nil || job.Mode != "analyze" || job.Query != "Competitive pricing snapshot" {
		t.Fatalf("job = %#v, want synthesized analyze job", job)
	}
}
