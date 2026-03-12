package bootstrap

import (
	"context"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"go.uber.org/zap"
)

type stubDeepResearchSearcher struct{}

func (stubDeepResearchSearcher) Search(context.Context, string, int, string) ([]deepresearch.SearchHit, error) {
	return []deepresearch.SearchHit{}, nil
}

func TestDeepResearchCronHandlerRequiresQuery(t *testing.T) {
	researchSvc := deepresearch.NewService(nil, stubDeepResearchSearcher{})
	handler := newDeepResearchCronHandler(researchSvc, zap.NewNop())

	_, err := handler(context.Background(), &cron.Job{
		ID:      "cron_job_1",
		Payload: map[string]interface{}{},
	})
	if err == nil {
		t.Fatal("expected error when query is missing")
	}
	if !strings.Contains(err.Error(), "query not specified in payload") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeepResearchCronHandlerCreatesJob(t *testing.T) {
	researchSvc := deepresearch.NewService(nil, stubDeepResearchSearcher{})
	handler := newDeepResearchCronHandler(researchSvc, zap.NewNop())

	result, err := handler(context.Background(), &cron.Job{
		ID: "cron_job_2",
		Payload: map[string]interface{}{
			"query":      "research blue roadmap",
			"user_id":    "u-1",
			"route_mode": "web",
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map[string]interface{}", result)
	}
	if got := payload["status"]; got != "accepted" {
		t.Fatalf("status = %v, want accepted", got)
	}
	jobID, _ := payload["research_job_id"].(string)
	if strings.TrimSpace(jobID) == "" {
		t.Fatalf("research_job_id is empty: %#v", payload)
	}
}

func TestRegisterDeepResearchCronHandlerEnablesCronCreate(t *testing.T) {
	cronSvc := cron.NewService(cron.DefaultConfig(), zap.NewNop())
	researchSvc := deepresearch.NewService(nil, stubDeepResearchSearcher{})
	registerDeepResearchCronHandler(cronSvc, researchSvc, zap.NewNop())

	_, err := cronSvc.Create(
		"deep-research-job",
		"run research and notify",
		"*/5 * * * *",
		researchAndNotifyHandlerName,
		map[string]interface{}{"query": "research blue release plan"},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
}
