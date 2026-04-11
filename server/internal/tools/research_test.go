package tools

import (
	"context"
	"errors"
	"testing"
)

type mockResearchService struct {
	create func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error)
	get    func(id, userID string) (*ResearchJob, error)
}

func (m *mockResearchService) CreateJob(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
	if m.create == nil {
		return nil, errors.New("create not implemented")
	}
	return m.create(ctx, req)
}

func (m *mockResearchService) GetJobForUser(id, userID string) (*ResearchJob, error) {
	if m.get == nil {
		return nil, errors.New("get not implemented")
	}
	return m.get(id, userID)
}

func TestResearchRunToolWaitsForCompletion(t *testing.T) {
	service := &mockResearchService{}
	service.create = func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
		if req.Query != "research this" {
			t.Fatalf("query = %q, want %q", req.Query, "research this")
		}
		return &ResearchJob{ID: "job-1", Status: "running", Query: req.Query, RequestedRouteMode: req.RouteMode, EffectiveRouteMode: "web"}, nil
	}
	calls := 0
	service.get = func(id, userID string) (*ResearchJob, error) {
		calls++
		return &ResearchJob{ID: id, Status: "completed", Query: "research this", EffectiveRouteMode: "web", Answer: "done", Confidence: 0.8}, nil
	}
	tool := NewResearchRunTool(service)
	res, err := tool.Execute(WithUserID(context.Background(), "u1"), map[string]interface{}{"query": "research this"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["status"]; got != "completed" {
		t.Fatalf("status = %v, want completed", got)
	}
	if got := payload["answer"]; got != "done" {
		t.Fatalf("answer = %v, want done", got)
	}
	if calls == 0 {
		t.Fatal("expected GetJobForUser to be called")
	}
}

func TestResearchRunToolReturnsAcceptedWhenWaitFalse(t *testing.T) {
	service := &mockResearchService{
		create: func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
			return &ResearchJob{ID: "job-2", Status: "pending", Query: req.Query, EffectiveRouteMode: "experiment"}, nil
		},
	}
	tool := NewResearchRunTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{"query": "run benchmark", "wait": false})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["accepted"]; got != true {
		t.Fatalf("accepted = %v, want true", got)
	}
	if got := payload["terminal"]; got != false {
		t.Fatalf("terminal = %v, want false", got)
	}
}

func TestResearchRunToolSupportsNestedCamelCaseArgs(t *testing.T) {
	service := &mockResearchService{}
	service.create = func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
		if req.Query != "research this" {
			t.Fatalf("query = %q, want %q", req.Query, "research this")
		}
		if req.RouteMode != "hybrid" || req.Lang != "en-US" || req.ReportStyle != "brief" {
			t.Fatalf("unexpected request: %#v", req)
		}
		if req.Budget == nil || req.Budget.MaxSources != 3 || req.Budget.MaxSeconds != 9 {
			t.Fatalf("unexpected budget: %#v", req.Budget)
		}
		if req.StrictEntity == nil || !*req.StrictEntity {
			t.Fatalf("expected strict entity true, got %#v", req.StrictEntity)
		}
		if len(req.TimeWindows) != 2 || req.TimeWindows[0] != "7d" {
			t.Fatalf("unexpected time windows: %#v", req.TimeWindows)
		}
		return &ResearchJob{ID: "job-9", Status: "pending", Query: req.Query, EffectiveRouteMode: "hybrid"}, nil
	}
	tool := NewResearchRunTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"query":              "research this",
			"routeMode":          "hybrid",
			"language":           "en-US",
			"strictEntity":       true,
			"timeWindows":        []interface{}{"7d", "30d"},
			"reportStyle":        "brief",
			"maxSources":         3,
			"maxSeconds":         9,
			"wait":               false,
			"pollIntervalMs":     1,
			"waitTimeoutSeconds": 1,
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["accepted"]; got != true {
		t.Fatalf("accepted = %v, want true", got)
	}
}

func TestResearchToolUsesCanonicalDefinitionName(t *testing.T) {
	tool := NewDeepResearchTool(nil)
	if got := tool.Definition().Name; got != "research" {
		t.Fatalf("definition name = %q, want research", got)
	}
}

func TestResearchToolAutoSelectsAnalyzeMode(t *testing.T) {
	service := &mockResearchService{
		create: func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
			if req.Query != "Analyze https://example.com/blog and summarize the key findings into a short report." {
				t.Fatalf("query = %q, want url analysis query", req.Query)
			}
			if req.Mode != "analyze" {
				t.Fatalf("mode = %q, want analyze", req.Mode)
			}
			return &ResearchJob{ID: "job-auto", Status: "pending", Query: req.Query, Mode: req.Mode, EffectiveRouteMode: "web"}, nil
		},
	}
	tool := NewDeepResearchTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "Analyze https://example.com/blog and summarize the key findings into a short report.",
		"mode":  "auto",
		"wait":  false,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["accepted"]; got != true {
		t.Fatalf("accepted = %v, want true", got)
	}
	if got := payload["mode"]; got != "analyze" {
		t.Fatalf("mode = %v, want analyze", got)
	}
}

func TestResearchToolExplicitModeBypassesAutoSelection(t *testing.T) {
	service := &mockResearchService{
		create: func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
			if req.Mode != "ui_review" {
				t.Fatalf("mode = %q, want ui_review", req.Mode)
			}
			return &ResearchJob{ID: "job-ui", Status: "pending", Query: req.Query, Mode: req.Mode, EffectiveRouteMode: "web"}, nil
		},
	}
	tool := NewDeepResearchTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"query": "Review https://example.com/pricing for accessibility and layout issues.",
		"mode":  "ui_review",
		"wait":  false,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["mode"]; got != "ui_review" {
		t.Fatalf("mode = %v, want ui_review", got)
	}
}

func TestResearchToolForwardsAnalyzeModeSpecificArgs(t *testing.T) {
	service := &mockResearchService{
		create: func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
			if req.Mode != "analyze" {
				t.Fatalf("mode = %q, want analyze", req.Mode)
			}
			if req.Query != "Competitive pricing snapshot" {
				t.Fatalf("query = %q, want topic fallback", req.Query)
			}
			if req.Topic != "Competitive pricing snapshot" {
				t.Fatalf("topic = %q, want forwarded topic", req.Topic)
			}
			if len(req.URLs) != 2 || req.URLs[0] != "https://example.com/pricing" || req.URLs[1] != "https://example.com/blog" {
				t.Fatalf("urls = %#v, want forwarded urls", req.URLs)
			}
			if len(req.SearchQueries) != 2 || req.SearchQueries[0] != "example pricing comparison" || req.SearchQueries[1] != "competitor plan changes" {
				t.Fatalf("search queries = %#v, want forwarded search queries", req.SearchQueries)
			}
			if req.Text != "Use internal pricing notes too." {
				t.Fatalf("text = %q, want forwarded text", req.Text)
			}
			if req.OutputMode != "report" {
				t.Fatalf("output mode = %q, want report", req.OutputMode)
			}
			if req.ReportStyle != "briefing" {
				t.Fatalf("report style = %q, want briefing", req.ReportStyle)
			}
			if req.Lang != "en-US" {
				t.Fatalf("lang = %q, want en-US", req.Lang)
			}
			return &ResearchJob{ID: "job-analyze", Status: "pending", Query: req.Query, Mode: req.Mode, EffectiveRouteMode: "web"}, nil
		},
	}
	tool := NewDeepResearchTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"mode": "analyze",
		"wait": false,
		"input": map[string]interface{}{
			"topic":         "Competitive pricing snapshot",
			"urls":          []interface{}{"https://example.com/pricing", "https://example.com/blog"},
			"searchQueries": []interface{}{"example pricing comparison", "competitor plan changes"},
			"text":          "Use internal pricing notes too.",
			"outputMode":    "report",
			"reportStyle":   "briefing",
			"language":      "en-US",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["accepted"]; got != true {
		t.Fatalf("accepted = %v, want true", got)
	}
	if got := payload["mode"]; got != "analyze" {
		t.Fatalf("mode = %v, want analyze", got)
	}
}

func TestResearchToolForwardsUIReviewModeSpecificArgs(t *testing.T) {
	service := &mockResearchService{
		create: func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
			if req.Mode != "ui_review" {
				t.Fatalf("mode = %q, want ui_review", req.Mode)
			}
			if req.Query != "https://example.com/pricing" {
				t.Fatalf("query = %q, want url fallback", req.Query)
			}
			if req.Action != "check_accessibility" {
				t.Fatalf("action = %q, want check_accessibility", req.Action)
			}
			if req.URL != "https://example.com/pricing" {
				t.Fatalf("url = %q, want forwarded url", req.URL)
			}
			if req.Device != "mobile" || req.Channel != "telegram" {
				t.Fatalf("device/channel = %q/%q, want mobile/telegram", req.Device, req.Channel)
			}
			if req.WaitMS != 1500 {
				t.Fatalf("wait_ms = %d, want 1500", req.WaitMS)
			}
			if req.Threshold != 82.5 {
				t.Fatalf("threshold = %v, want 82.5", req.Threshold)
			}
			if req.Format != "human" {
				t.Fatalf("format = %q, want human", req.Format)
			}
			if req.Profile != "ppt" {
				t.Fatalf("profile = %q, want ppt", req.Profile)
			}
			if req.Lang != "zh-CN" {
				t.Fatalf("lang = %q, want zh-CN", req.Lang)
			}
			return &ResearchJob{ID: "job-ui-review", Status: "pending", Query: req.Query, Mode: req.Mode, EffectiveRouteMode: "web"}, nil
		},
	}
	tool := NewDeepResearchTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"mode": "ui_review",
		"wait": false,
		"input": map[string]interface{}{
			"action":    "check_accessibility",
			"url":       "https://example.com/pricing",
			"device":    "mobile",
			"channel":   "telegram",
			"wait_ms":   1500,
			"threshold": 82.5,
			"format":    "human",
			"profile":   "ppt",
			"lang":      "zh-CN",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["accepted"]; got != true {
		t.Fatalf("accepted = %v, want true", got)
	}
	if got := payload["mode"]; got != "ui_review" {
		t.Fatalf("mode = %v, want ui_review", got)
	}
}

func TestResearchToolForwardsAdvisorV2Args(t *testing.T) {
	service := &mockResearchService{
		create: func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
			if req.Mode != "advisor" {
				t.Fatalf("mode = %q, want advisor", req.Mode)
			}
			if req.Query != "Go vs Python vs Node" {
				t.Fatalf("query = %q, want forwarded advisor question", req.Query)
			}
			if req.Output != "decision_pack" {
				t.Fatalf("output = %q, want decision_pack", req.Output)
			}
			if req.ScorecardPack != "solution_selection_v1" {
				t.Fatalf("scorecard pack = %q, want solution_selection_v1", req.ScorecardPack)
			}
			if got := req.ScorecardWeights["fitness"]; got != 0.4 {
				t.Fatalf("scorecard weight fitness = %v, want 0.4", got)
			}
			return &ResearchJob{ID: "job-advisor-v2", Status: "pending", Query: req.Query, Mode: req.Mode, EffectiveRouteMode: "web"}, nil
		},
	}
	tool := NewDeepResearchTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"mode": "advisor",
		"wait": false,
		"input": map[string]interface{}{
			"question":          "Go vs Python vs Node",
			"category":          "language",
			"decision_mode":     "compare",
			"candidates":        []interface{}{"Go", "Python", "Node"},
			"depth":             "deep",
			"output":            "decision_pack",
			"scorecard_pack":    "solution_selection_v1",
			"scorecard_weights": map[string]interface{}{"fitness": 0.4},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["accepted"]; got != true {
		t.Fatalf("accepted = %v, want true", got)
	}
	if got := payload["mode"]; got != "advisor" {
		t.Fatalf("mode = %v, want advisor", got)
	}
}

func TestResearchToolInfersUIReviewImageActionFromScreenshotAlias(t *testing.T) {
	service := &mockResearchService{
		create: func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
			if req.Mode != "ui_review" {
				t.Fatalf("mode = %q, want ui_review", req.Mode)
			}
			if req.Action != "review_image" {
				t.Fatalf("action = %q, want review_image", req.Action)
			}
			if req.Image != "base64-image-data" {
				t.Fatalf("image = %q, want forwarded screenshot alias", req.Image)
			}
			if req.Query != "Review provided image" {
				t.Fatalf("query = %q, want image fallback query", req.Query)
			}
			return &ResearchJob{ID: "job-ui-review-image", Status: "pending", Query: req.Query, Mode: req.Mode, EffectiveRouteMode: "web"}, nil
		},
	}
	tool := NewDeepResearchTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"mode": "ui_review",
		"wait": false,
		"input": map[string]interface{}{
			"screenshot": "base64-image-data",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["accepted"]; got != true {
		t.Fatalf("accepted = %v, want true", got)
	}
	if got := payload["mode"]; got != "ui_review" {
		t.Fatalf("mode = %v, want ui_review", got)
	}
}

func TestDeepResearchTool_StatusAction(t *testing.T) {
	service := &mockResearchService{
		get: func(id, userID string) (*ResearchJob, error) {
			if id != "job-3" {
				t.Fatalf("id = %q, want %q", id, "job-3")
			}
			return &ResearchJob{ID: id, Status: "completed", Answer: "report"}, nil
		},
	}
	tool := NewDeepResearchTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "status",
		"job_id": "job-3",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["answer"]; got != "report" {
		t.Fatalf("answer = %v, want report", got)
	}
}

func TestResearchStatusToolSupportsNestedCamelCaseArgs(t *testing.T) {
	service := &mockResearchService{
		get: func(id, userID string) (*ResearchJob, error) {
			if id != "job-3" {
				t.Fatalf("id = %q, want %q", id, "job-3")
			}
			return &ResearchJob{ID: id, Status: "completed", Answer: "report"}, nil
		},
	}
	tool := NewResearchStatusTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"jobId": "job-3",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["answer"]; got != "report" {
		t.Fatalf("answer = %v, want report", got)
	}
}

func TestResearchStatusTool(t *testing.T) {
	service := &mockResearchService{
		get: func(id, userID string) (*ResearchJob, error) {
			if id != "job-3" {
				t.Fatalf("id = %q, want %q", id, "job-3")
			}
			return &ResearchJob{ID: id, Status: "completed", Answer: "report"}, nil
		},
	}
	tool := NewResearchStatusTool(service)
	res, err := tool.Execute(context.Background(), map[string]interface{}{"job_id": "job-3"})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	payload := res.(map[string]interface{})
	if got := payload["answer"]; got != "report" {
		t.Fatalf("answer = %v, want report", got)
	}
}
