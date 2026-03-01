package deepresearch

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type mockSearcher struct {
	search func(ctx context.Context, query string, maxResults int) ([]SearchHit, error)
}

func (m *mockSearcher) Search(ctx context.Context, query string, maxResults int) ([]SearchHit, error) {
	return m.search(ctx, query, maxResults)
}

func TestServiceCreateJobCompletes(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int) ([]SearchHit, error) {
			return []SearchHit{
				{Title: "Doc A", URL: "https://example.com/a", Description: "A"},
				{Title: "Doc B", URL: "https://docs.example.org/b", Description: "B"},
			}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "golang deep research",
		Mode:  ModeStandard,
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		current, err := svc.GetJob(job.ID)
		if err != nil {
			t.Fatalf("get job failed: %v", err)
		}
		if current.Status == JobStatusCompleted {
			if current.Report == nil {
				t.Fatal("expected report")
			}
			if len(current.Evidence) == 0 {
				t.Fatal("expected evidence")
			}
			if len(current.Report.Citations) == 0 {
				t.Fatal("expected citations")
			}
			return
		}
		if current.Status == JobStatusFailed {
			t.Fatalf("unexpected failed job: %s", current.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for completion, status=%s", current.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestServiceCreateJobFailure(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int) ([]SearchHit, error) {
			return nil, errors.New("upstream down")
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "test query",
		Mode:  ModeFast,
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		current, err := svc.GetJob(job.ID)
		if err != nil {
			t.Fatalf("get job failed: %v", err)
		}
		if current.Status == JobStatusFailed {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for fail status, status=%s", current.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestServiceSubscribe(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int) ([]SearchHit, error) {
			time.Sleep(80 * time.Millisecond)
			return []SearchHit{
				{Title: "Doc", URL: "https://example.com/a", Description: "A"},
			}, nil
		},
	})
	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "subscribe test",
		Mode:  ModeFast,
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	events, unsubscribe, err := svc.Subscribe(job.ID)
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsubscribe()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev := <-events:
			if ev.Type == "job_completed" {
				return
			}
		case <-timeout:
			t.Fatal("timeout waiting for job_completed event")
		}
	}
}

func TestServiceHonorsMaxStepsBudget(t *testing.T) {
	var calls int32
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int) ([]SearchHit, error) {
			atomic.AddInt32(&calls, 1)
			return []SearchHit{
				{Title: "Doc", URL: "https://example.com/a", Description: "A"},
			}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "budget check",
		Mode:  ModeDeep, // planner would normally produce 5 tasks
		Budget: &Budget{
			MaxSteps: 3, // retrieve budget should become 1
		},
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		current, err := svc.GetJob(job.ID)
		if err != nil {
			t.Fatalf("get job failed: %v", err)
		}
		if current.Status == JobStatusCompleted {
			if len(current.Tasks) != 1 {
				t.Fatalf("expected 1 planned task after max_steps budget, got %d", len(current.Tasks))
			}
			if got := atomic.LoadInt32(&calls); got != 1 {
				t.Fatalf("expected 1 search call after max_steps budget, got %d", got)
			}
			return
		}
		if current.Status == JobStatusFailed {
			t.Fatalf("unexpected fail: %s", current.Error)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting completion, status=%s", current.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSynthesizeReportKeepsAllCitations(t *testing.T) {
	evidence := make([]Evidence, 0, 7)
	for i := 0; i < 7; i++ {
		evidence = append(evidence, Evidence{
			ID:               fmt.Sprintf("ev-%d", i+1),
			Title:            fmt.Sprintf("Doc %d", i+1),
			URL:              fmt.Sprintf("https://example.com/%d", i+1),
			Domain:           "example.com",
			RelevanceScore:   0.9,
			CredibilityScore: 0.8,
		})
	}

	report := synthesizeReport("test query", evidence)
	if got := len(report.Citations); got != len(evidence) {
		t.Fatalf("expected %d citations, got %d", len(evidence), got)
	}
}
