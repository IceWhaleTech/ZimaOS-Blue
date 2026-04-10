package drivers

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
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

type stubResearchModeTool struct {
	args   []map[string]interface{}
	result interface{}
	err    error
	cards  []map[string]interface{}
}

func (s *stubResearchModeTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	s.args = append(s.args, cloneMap(args))
	for _, card := range s.cards {
		tools.EmitCard(ctx, cloneMap(card))
	}
	return s.result, s.err
}

func waitForResearchRun(t *testing.T, controller *harness.Controller, runID string) *harness.Run {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		current, err := controller.GetStored(context.Background(), runID)
		if err != nil {
			t.Fatalf("GetStored(%q) failed: %v", runID, err)
		}
		switch current.Status {
		case harness.RunStatusCompleted, harness.RunStatusFailed, harness.RunStatusCancelled:
			return current
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for run %q, status=%s", runID, current.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func decodeResearchRunResult(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode research run result: %v raw=%s", err, raw)
	}
	return out
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

func TestResearchDriverStart_PassesProviderIDToService(t *testing.T) {
	svc := deepresearch.NewService(deepresearch.NewHeuristicPlanner(), testResearchSearcher{})
	driver := &ResearchDriver{service: svc}

	run := &harness.Run{
		ID:         "research-provider-job",
		UserID:     "user-1",
		Goal:       "feature rollout",
		ProviderID: "openai-prod",
	}

	if err := driver.Start(context.Background(), run, harness.RunEnv{}); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	current := waitForDeepResearchJob(t, svc, run.ID)
	if current.ProviderID != "openai-prod" {
		t.Fatalf("provider_id = %q, want openai-prod", current.ProviderID)
	}
}

func TestResearchDriverStart_AnalyzeModeRunsInsideHarnessEnvelope(t *testing.T) {
	controller := newDriverTestController(t)
	analyzeTool := &stubResearchModeTool{
		cards: []map[string]interface{}{
			{"type": "analyze-progress", "step": "data_collection", "name": "Collect data", "status": "running"},
			{"type": "analyze-progress", "step": "analysis", "name": "Analyze", "status": "success"},
		},
		result: `{"topic":"Market analysis","answer":"Revenue grew 18%.","output_mode":"inline"}`,
	}
	driver := NewResearchDriver(nil, controller)
	driver.SetAnalyzeExecutor(analyzeTool)
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind:   harness.RunKindResearch,
		Goal:   "Summarize the quarterly report",
		UserID: "user-analyze",
		Metadata: map[string]interface{}{
			"mode":           "analyze",
			"topic":          "Market analysis",
			"text":           "Revenue grew 18% quarter over quarter.",
			"output_mode":    "inline",
			"report_style":   "dashboard",
			"search_queries": []string{"company quarterly report"},
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	current := waitForResearchRun(t, controller, run.ID)
	if current.Status != harness.RunStatusCompleted {
		t.Fatalf("status = %s, want completed", current.Status)
	}
	if got := metadataString(current.Metadata, "mode"); got != "analyze" {
		t.Fatalf("metadata mode = %q, want analyze", got)
	}
	if current.Progress != 100 {
		t.Fatalf("progress = %d, want 100", current.Progress)
	}
	if len(analyzeTool.args) != 1 {
		t.Fatalf("tool args calls = %d, want 1", len(analyzeTool.args))
	}
	if got := metadataString(analyzeTool.args[0], "topic"); got != "Market analysis" {
		t.Fatalf("tool topic = %q, want Market analysis", got)
	}
	if got := metadataString(analyzeTool.args[0], "output_mode"); got != "inline" {
		t.Fatalf("tool output_mode = %q, want inline", got)
	}
	result := decodeResearchRunResult(t, current.Result)
	if got := metadataString(result, "topic"); got != "Market analysis" {
		t.Fatalf("result topic = %q, want Market analysis", got)
	}
}

func TestResearchDriverStart_UIReviewModeRunsInsideHarnessEnvelope(t *testing.T) {
	controller := newDriverTestController(t)
	uiTool := &stubResearchModeTool{
		cards: []map[string]interface{}{
			{"type": "ui-review-progress", "step": "navigate", "name": "Open page", "status": "running", "url": "https://example.com"},
			{"type": "ui-review-progress", "step": "navigate", "name": "Open page", "status": "success", "url": "https://example.com"},
		},
		result: `{"url":"https://example.com","overall":82,"pass":true,"threshold":75}`,
	}
	driver := NewResearchDriver(nil, controller)
	driver.SetUIReviewExecutor(uiTool)
	controller.RegisterDriver(driver)

	run, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind:   harness.RunKindResearch,
		Goal:   "Review this landing page",
		UserID: "user-ui",
		Metadata: map[string]interface{}{
			"mode":   "ui_review",
			"action": "review_url",
			"url":    "https://example.com",
			"device": "desktop",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	current := waitForResearchRun(t, controller, run.ID)
	if current.Status != harness.RunStatusCompleted {
		t.Fatalf("status = %s, want completed", current.Status)
	}
	if got := metadataString(current.Metadata, "mode"); got != "ui_review" {
		t.Fatalf("metadata mode = %q, want ui_review", got)
	}
	if current.Progress != 100 {
		t.Fatalf("progress = %d, want 100", current.Progress)
	}
	if len(uiTool.args) != 1 {
		t.Fatalf("tool args calls = %d, want 1", len(uiTool.args))
	}
	if got := metadataString(uiTool.args[0], "action"); got != "review_url" {
		t.Fatalf("tool action = %q, want review_url", got)
	}
	if got := metadataString(uiTool.args[0], "url"); got != "https://example.com" {
		t.Fatalf("tool url = %q, want https://example.com", got)
	}
	result := decodeResearchRunResult(t, current.Result)
	if got := metadataString(result, "url"); got != "https://example.com" {
		t.Fatalf("result url = %q, want https://example.com", got)
	}
}

func TestResearchDriverValidate_AnalyzeModeRejectsTypedNilExecutor(t *testing.T) {
	driver := NewResearchDriver(nil, newDriverTestController(t))

	var analyzeTool *tools.AnalyzeTool
	driver.SetAnalyzeExecutor(analyzeTool)

	err := driver.Validate(harness.RunSpec{
		Kind: harness.RunKindResearch,
		Goal: "Summarize the quarterly report",
		Metadata: map[string]interface{}{
			"mode": "analyze",
		},
	})
	if err == nil {
		t.Fatal("Validate returned nil error for typed-nil analyze executor")
	}
	if err.Error() != "analyze runtime is not available" {
		t.Fatalf("Validate error = %q, want %q", err.Error(), "analyze runtime is not available")
	}
}

func TestResearchDriverStart_AnalyzeModeRejectsTypedNilExecutor(t *testing.T) {
	controller := newDriverTestController(t)
	driver := NewResearchDriver(nil, controller)

	var analyzeTool *tools.AnalyzeTool
	driver.SetAnalyzeExecutor(analyzeTool)

	err := driver.Start(context.Background(), &harness.Run{
		ID:        "research-typed-nil-start",
		RootRunID: "research-typed-nil-start",
		Kind:      harness.RunKindResearch,
		Goal:      "Summarize the quarterly report",
		Metadata: map[string]interface{}{
			"mode": "analyze",
		},
	}, harness.RunEnv{Manager: controller})
	if err == nil {
		t.Fatal("Start returned nil error for typed-nil analyze executor")
	}
	if err.Error() != "analyze runtime is not available" {
		t.Fatalf("Start error = %q, want %q", err.Error(), "analyze runtime is not available")
	}
}

func TestJobToRun_PersistsFamilyModeAndResearchDepth(t *testing.T) {
	job := &deepresearch.Job{
		ID:             "job-1",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Query:          "feature rollout",
		Status:         deepresearch.JobStatusCompleted,
		Mode:           deepresearch.ModeDeep,
		Report: &deepresearch.Report{
			Answer: "summary",
		},
		CreatedAt: time.Now().UTC().Add(-time.Minute),
		UpdatedAt: time.Now().UTC(),
	}
	completedAt := time.Now().UTC()
	job.CompletedAt = &completedAt

	run := jobToRun(&harness.Run{
		ID:   "job-1",
		Kind: harness.RunKindResearch,
		Metadata: map[string]interface{}{
			"mode": "deep_research",
		},
	}, job)

	if got := metadataString(run.Metadata, "mode"); got != "deep_research" {
		t.Fatalf("metadata mode = %q, want deep_research", got)
	}
	if got := metadataString(run.Metadata, "research_depth"); got != "deep" {
		t.Fatalf("metadata research_depth = %q, want deep", got)
	}
}
