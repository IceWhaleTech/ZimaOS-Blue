package deepresearch

import (
	"context"
	"sync"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestSkillExecutorExecute_UsesUserFromContextAndEmitsProgress(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{
				{Title: "Doc A", URL: "https://example.com/a", Description: "A"},
				{Title: "Doc B", URL: "https://example.com/b", Description: "B"},
			}, nil
		},
	})
	exec := NewSkillExecutor(svc)

	var (
		mu    sync.Mutex
		cards []map[string]interface{}
	)
	ctx := tools.WithUserID(context.Background(), "user-42")
	ctx = tools.WithCardEmitter(ctx, func(card map[string]interface{}) {
		mu.Lock()
		defer mu.Unlock()
		cp := make(map[string]interface{}, len(card))
		for k, v := range card {
			cp[k] = v
		}
		cards = append(cards, cp)
	})

	out, err := exec.Execute(ctx, map[string]interface{}{
		"query": "deep research skill executor",
		"mode":  "standard",
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	data, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map", out)
	}
	jobID, _ := data["job_id"].(string)
	if jobID == "" {
		t.Fatalf("expected non-empty job_id in result")
	}
	if _, err := svc.GetJobForUser(jobID, "user-42", ""); err != nil {
		t.Fatalf("owner should be able to fetch job: %v", err)
	}
	if _, err := svc.GetJobForUser(jobID, "other-user", ""); err != ErrJobForbidden {
		t.Fatalf("non-owner access err = %v, want %v", err, ErrJobForbidden)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(cards) == 0 {
		t.Fatalf("expected progress cards to be emitted")
	}
}
