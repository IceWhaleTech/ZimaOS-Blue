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
