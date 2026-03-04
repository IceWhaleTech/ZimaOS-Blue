package deepresearch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockSearcher struct {
	search func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error)
}

func (m *mockSearcher) Search(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
	return m.search(ctx, query, maxResults, lang)
}

type mockPlanner struct {
	plan func(query string, mode Mode, lang string) []Task
}

func (m *mockPlanner) Plan(query string, mode Mode, lang string) []Task {
	if m == nil || m.plan == nil {
		return nil
	}
	return m.plan(query, mode, lang)
}

type mockSummarySynth struct {
	summarize func(ctx context.Context, input SummaryInput) (string, error)
}

func (m *mockSummarySynth) Summarize(ctx context.Context, input SummaryInput) (string, error) {
	if m == nil || m.summarize == nil {
		return "", nil
	}
	return m.summarize(ctx, input)
}

func waitForTerminalJob(t *testing.T, svc *Service, jobID string, timeout time.Duration) *Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		current, err := svc.GetJob(jobID)
		if err != nil {
			t.Fatalf("get job failed: %v", err)
		}
		if isTerminalStatus(current.Status) {
			return current
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for terminal status, status=%s", current.Status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestServiceCreateJobCompletes(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
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
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
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
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
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
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
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

	report := synthesizeReport("test query", "en-US", evidence)
	if got := len(report.Citations); got != len(evidence) {
		t.Fatalf("expected %d citations, got %d", len(evidence), got)
	}
}

func TestServiceCreateJob_LangAffectsPlanSearchAndReport(t *testing.T) {
	var mu sync.Mutex
	var seenQueries []string
	var seenLangs []string

	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			mu.Lock()
			seenQueries = append(seenQueries, query)
			seenLangs = append(seenLangs, lang)
			mu.Unlock()
			return []SearchHit{
				{Title: "Doc A", URL: "https://example.com/a", Description: "A"},
			}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "ZimaOS",
		Mode:  ModeStandard,
		Lang:  "zh-CN",
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
			mu.Lock()
			queries := append([]string(nil), seenQueries...)
			langs := append([]string(nil), seenLangs...)
			mu.Unlock()
			if len(langs) == 0 {
				t.Fatal("expected searcher to be called")
			}
			for _, lang := range langs {
				if lang != "zh-CN" {
					t.Fatalf("search lang = %q, want zh-CN", lang)
				}
			}
			foundChinesePlan := false
			for _, q := range queries {
				if strings.Contains(q, "最新进展") || strings.Contains(q, "官方文档") {
					foundChinesePlan = true
					break
				}
			}
			if !foundChinesePlan {
				t.Fatalf("expected localized Chinese plan queries, got %v", queries)
			}
			if current.Report == nil {
				t.Fatal("expected report")
			}
			if !strings.Contains(current.Report.Answer, "调研总结：") {
				t.Fatalf("expected localized Chinese report title, got %q", current.Report.Answer)
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

func TestServiceSubscribeUnsubscribeDoesNotPanicDuringBroadcast(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{{Title: "Doc", URL: "https://example.com", Description: "A"}}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "broadcast race",
		Mode:  ModeFast,
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	_, unsubscribe, err := svc.Subscribe(job.ID)
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 64; i++ {
			svc.broadcast(job.ID, "test_event", map[string]interface{}{"n": i})
		}
	}()

	unsubscribe()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for broadcast goroutine")
	}
}

func TestServicePruneJobsLocked_RemovesExpiredTerminalJobs(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return nil, nil
		},
	})

	now := time.Now()
	expired := now.Add(-(deepResearchCompletedJobTTL + time.Minute))
	fresh := now.Add(-time.Minute)

	expiredID := uuid.NewString()
	freshID := uuid.NewString()
	runningID := uuid.NewString()

	svc.mu.Lock()
	svc.jobs[expiredID] = &Job{
		ID:          expiredID,
		Query:       "expired",
		Status:      JobStatusCompleted,
		CreatedAt:   expired,
		UpdatedAt:   expired,
		CompletedAt: &expired,
	}
	svc.jobs[freshID] = &Job{
		ID:          freshID,
		Query:       "fresh",
		Status:      JobStatusCompleted,
		CreatedAt:   fresh,
		UpdatedAt:   fresh,
		CompletedAt: &fresh,
	}
	svc.jobs[runningID] = &Job{
		ID:        runningID,
		Query:     "running",
		Status:    JobStatusRunning,
		CreatedAt: expired,
		UpdatedAt: expired,
	}
	svc.pruneJobsLocked(now)
	svc.mu.Unlock()

	if _, err := svc.GetJob(expiredID); err == nil {
		t.Fatalf("expected expired completed job %s to be pruned", expiredID)
	}
	if _, err := svc.GetJob(freshID); err != nil {
		t.Fatalf("expected fresh completed job %s to remain: %v", freshID, err)
	}
	if _, err := svc.GetJob(runningID); err != nil {
		t.Fatalf("expected running job %s to remain: %v", runningID, err)
	}
}

func TestServiceSearchConcurrencyCapped(t *testing.T) {
	const taskCount = 10
	tasks := make([]Task, 0, taskCount)
	for i := 0; i < taskCount; i++ {
		tasks = append(tasks, Task{
			ID:       fmt.Sprintf("task_%d", i+1),
			Question: fmt.Sprintf("q-%d", i+1),
			Priority: i + 1,
			Depth:    1,
			Status:   "pending",
		})
	}

	var inFlight int32
	var maxInFlight int32
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return tasks
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				prev := atomic.LoadInt32(&maxInFlight)
				if cur <= prev || atomic.CompareAndSwapInt32(&maxInFlight, prev, cur) {
					break
				}
			}
			time.Sleep(60 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			return []SearchHit{
				{Title: query, URL: "https://example.com/" + query, Description: "ok"},
			}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "concurrency cap",
		Mode:  ModeDeep,
		Budget: &Budget{
			MaxSteps:   20,
			MaxSources: 100,
			MaxSeconds: 10,
		},
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	current := waitForTerminalJob(t, svc, job.ID, 3*time.Second)
	if current.Status != JobStatusCompleted {
		t.Fatalf("expected completed status, got %s (err=%s)", current.Status, current.Error)
	}

	limit := int32(searchParallelism(ModeDeep))
	if got := atomic.LoadInt32(&maxInFlight); got > limit {
		t.Fatalf("max concurrent search calls = %d, want <= %d", got, limit)
	}
}

func TestServiceCanonicalURLDedupesEvidence(t *testing.T) {
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{{ID: "task_1", Question: query, Priority: 1, Depth: 1, Status: "pending"}}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{
				{Title: "A", URL: "https://EXAMPLE.com/path/?id=1&utm_source=newsletter#section", Description: "A"},
				{Title: "B", URL: "https://example.com/path?id=1&utm_medium=email", Description: "B"},
				{Title: "C", URL: "https://example.com/path/?utm_campaign=x&id=1", Description: "C"},
			}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "url canonicalization",
		Mode:  ModeStandard,
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	current := waitForTerminalJob(t, svc, job.ID, 2*time.Second)
	if current.Status != JobStatusCompleted {
		t.Fatalf("expected completed status, got %s (err=%s)", current.Status, current.Error)
	}
	if len(current.Evidence) != 1 {
		t.Fatalf("expected deduped evidence count 1, got %d", len(current.Evidence))
	}
	if got := current.Evidence[0].URL; got != "https://example.com/path?id=1" {
		t.Fatalf("canonical URL = %q, want https://example.com/path?id=1", got)
	}
}

func TestServiceStopsSearchEarlyWhenMaxSourcesReached(t *testing.T) {
	tasks := make([]Task, 0, 20)
	for i := 1; i <= 20; i++ {
		tasks = append(tasks, Task{
			ID:       fmt.Sprintf("task_%d", i),
			Question: fmt.Sprintf("task_%d", i),
			Priority: i,
			Depth:    1,
			Status:   "pending",
		})
	}

	var cancelledCalls int32
	var startedCalls int32
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return tasks
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			atomic.AddInt32(&startedCalls, 1)
			if query == "task_1" {
				return []SearchHit{
					{Title: "Fast hit", URL: "https://example.com/fast", Description: "fast"},
				}, nil
			}

			select {
			case <-ctx.Done():
				atomic.AddInt32(&cancelledCalls, 1)
				return nil, ctx.Err()
			case <-time.After(2 * time.Second):
				return []SearchHit{
					{Title: "Slow hit", URL: "https://example.com/" + query, Description: "slow"},
				}, nil
			}
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "stop early",
		Mode:  ModeDeep,
		Budget: &Budget{
			MaxSteps:   20,
			MaxSources: 1,
			MaxSeconds: 10,
		},
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	current := waitForTerminalJob(t, svc, job.ID, 3*time.Second)
	if current.Status != JobStatusCompleted {
		t.Fatalf("expected completed status, got %s (err=%s)", current.Status, current.Error)
	}
	if len(current.Evidence) != 1 {
		t.Fatalf("expected exactly 1 evidence item, got %d", len(current.Evidence))
	}
	if got := atomic.LoadInt32(&cancelledCalls); got == 0 {
		t.Fatalf("expected at least one cancelled search call after reaching max sources")
	}
	if got := atomic.LoadInt32(&startedCalls); got >= int32(len(tasks)) {
		t.Fatalf("expected early-stop scheduler to avoid starting all tasks, started=%d total=%d", got, len(tasks))
	}
}

func TestServiceSearchCacheDeduplicatesConcurrentIdenticalQuery(t *testing.T) {
	var calls int32
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{
				{ID: "task_1", Question: "same query", Priority: 1, Depth: 1, Status: "pending"},
				{ID: "task_2", Question: "same query", Priority: 2, Depth: 1, Status: "pending"},
			}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			atomic.AddInt32(&calls, 1)
			time.Sleep(80 * time.Millisecond)
			return []SearchHit{
				{Title: "Doc", URL: "https://example.com/cache", Description: "cache"},
			}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "cache in-flight dedupe",
		Mode:  ModeStandard,
		Budget: &Budget{
			MaxSteps:   10,
			MaxSources: 10,
			MaxSeconds: 10,
		},
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	current := waitForTerminalJob(t, svc, job.ID, 2*time.Second)
	if current.Status != JobStatusCompleted {
		t.Fatalf("expected completed status, got %s (err=%s)", current.Status, current.Error)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("underlying search calls = %d, want 1", got)
	}
}

func TestServiceSearchCacheTTLHitAndExpire(t *testing.T) {
	var calls int32
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{{ID: "task_1", Question: "stable query", Priority: 1, Depth: 1, Status: "pending"}}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			n := atomic.AddInt32(&calls, 1)
			return []SearchHit{
				{Title: fmt.Sprintf("Doc %d", n), URL: fmt.Sprintf("https://example.com/cache/%d", n), Description: "cache"},
			}, nil
		},
	})
	svc.searchCacheTTL = 500 * time.Millisecond

	createAndWait := func() {
		job, err := svc.CreateJob(context.Background(), CreateJobRequest{
			Query: "cache ttl",
			Mode:  ModeFast,
		})
		if err != nil {
			t.Fatalf("create job failed: %v", err)
		}
		current := waitForTerminalJob(t, svc, job.ID, 2*time.Second)
		if current.Status != JobStatusCompleted {
			t.Fatalf("expected completed status, got %s (err=%s)", current.Status, current.Error)
		}
	}

	createAndWait()
	createAndWait()
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("underlying search calls before ttl expiry = %d, want 1", got)
	}

	time.Sleep(650 * time.Millisecond)
	createAndWait()
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("underlying search calls after ttl expiry = %d, want 2", got)
	}
}

func TestServiceSearchCacheDoesNotCacheErrors(t *testing.T) {
	var calls int32
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{{ID: "task_1", Question: "error query", Priority: 1, Depth: 1, Status: "pending"}}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			if atomic.AddInt32(&calls, 1) <= 3 {
				return nil, errors.New("temporary upstream error")
			}
			return []SearchHit{
				{Title: "Recovered", URL: "https://example.com/recovered", Description: "ok"},
			}, nil
		},
	})

	job1, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "cache error first",
		Mode:  ModeFast,
	})
	if err != nil {
		t.Fatalf("create first job failed: %v", err)
	}
	first := waitForTerminalJob(t, svc, job1.ID, 2*time.Second)
	if first.Status != JobStatusFailed {
		t.Fatalf("expected first job failed status, got %s", first.Status)
	}

	job2, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query: "cache error first",
		Mode:  ModeFast,
	})
	if err != nil {
		t.Fatalf("create second job failed: %v", err)
	}
	second := waitForTerminalJob(t, svc, job2.ID, 2*time.Second)
	if second.Status != JobStatusCompleted {
		t.Fatalf("expected second job completed status, got %s (err=%s)", second.Status, second.Error)
	}
	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Fatalf("underlying search calls = %d, want 4 (all failed attempts should not be cached)", got)
	}
}

func TestServiceSearchParallelismForMode_DefaultWhenSamplesInsufficient(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return nil, nil
		},
	})

	for i := 0; i < deepResearchSearchPerfMinN-1; i++ {
		svc.recordSearchOutcome(120*time.Millisecond, nil)
	}

	if got, want := svc.searchParallelismForMode(ModeDeep), searchParallelism(ModeDeep); got != want {
		t.Fatalf("parallelism with insufficient samples = %d, want %d", got, want)
	}
}

func TestServiceSearchParallelismForMode_ScalesDownOnTimeouts(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return nil, nil
		},
	})

	for i := 0; i < 12; i++ {
		svc.recordSearchOutcome(900*time.Millisecond, context.DeadlineExceeded)
	}

	want := maxInt(1, searchParallelism(ModeDeep)-2)
	if got := svc.searchParallelismForMode(ModeDeep); got != want {
		t.Fatalf("parallelism under timeout pressure = %d, want %d", got, want)
	}
}

func TestServiceSearchParallelismForMode_ScalesUpWhenFastAndHealthy(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return nil, nil
		},
	})

	for i := 0; i < 12; i++ {
		svc.recordSearchOutcome(90*time.Millisecond, nil)
	}

	want := minInt(searchParallelismCeiling(ModeDeep), searchParallelism(ModeDeep)+1)
	if got := svc.searchParallelismForMode(ModeDeep); got != want {
		t.Fatalf("parallelism in healthy fast mode = %d, want %d", got, want)
	}
}

func TestServiceSearchParallelismForMode_IgnoresCanceledSamples(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return nil, nil
		},
	})

	for i := 0; i < 20; i++ {
		svc.recordSearchOutcome(200*time.Millisecond, context.Canceled)
	}

	if got, want := svc.searchParallelismForMode(ModeStandard), searchParallelism(ModeStandard); got != want {
		t.Fatalf("parallelism after canceled-only samples = %d, want %d", got, want)
	}
}

func TestServiceOwnershipAndActorScoping(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			time.Sleep(30 * time.Millisecond)
			return []SearchHit{{Title: "Doc", URL: "https://example.com/a", Description: "A"}}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:  "ownership",
		UserID: "alice",
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	if _, err := svc.GetJobForUser(job.ID, "bob", ""); !errors.Is(err, ErrJobForbidden) {
		t.Fatalf("GetJobForUser wrong actor err = %v, want %v", err, ErrJobForbidden)
	}
	if _, _, err := svc.SubscribeForUser(job.ID, "bob", ""); !errors.Is(err, ErrJobForbidden) {
		t.Fatalf("SubscribeForUser wrong actor err = %v, want %v", err, ErrJobForbidden)
	}
	if err := svc.CancelJobForUser(job.ID, "bob", ""); !errors.Is(err, ErrJobForbidden) {
		t.Fatalf("CancelJobForUser wrong actor err = %v, want %v", err, ErrJobForbidden)
	}
	if _, err := svc.GetJobForUser(job.ID, "alice", ""); err != nil {
		t.Fatalf("GetJobForUser owner should be allowed, got %v", err)
	}
}

func TestServiceCreateJob_ConcurrencyLimitPerUser(t *testing.T) {
	release := make(chan struct{})
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{{ID: "task_1", Question: "q", Priority: 1, Depth: 1, Status: "pending"}}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			select {
			case <-release:
				return []SearchHit{{Title: "Doc", URL: "https://example.com/a", Description: "A"}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	})
	svc.maxConcurrentPerUser = 1
	svc.maxCreatesPerWindow = 10

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{Query: "first", UserID: "u1"})
	if err != nil {
		t.Fatalf("create first job failed: %v", err)
	}
	if _, err := svc.CreateJob(context.Background(), CreateJobRequest{Query: "second", UserID: "u1"}); !errors.Is(err, ErrTooManyActiveJobs) {
		t.Fatalf("create second job err = %v, want %v", err, ErrTooManyActiveJobs)
	}

	close(release)
	_ = waitForTerminalJob(t, svc, job.ID, 2*time.Second)
}

func TestServiceCreateJob_RateLimitPerUser(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{{Title: "Doc", URL: "https://example.com/a", Description: "A"}}, nil
		},
	})
	svc.maxConcurrentPerUser = 10
	svc.maxCreatesPerWindow = 1
	svc.createRateWindow = time.Minute

	if _, err := svc.CreateJob(context.Background(), CreateJobRequest{Query: "one", UserID: "u1"}); err != nil {
		t.Fatalf("create first job failed: %v", err)
	}
	if _, err := svc.CreateJob(context.Background(), CreateJobRequest{Query: "two", UserID: "u1"}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("create second job err = %v, want %v", err, ErrRateLimited)
	}
}

func TestServiceSubscribeForUser_ReplaysTerminalEventWithID(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{{Title: "Doc", URL: "https://example.com/a", Description: "A"}}, nil
		},
	})

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:  "terminal replay",
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}
	current := waitForTerminalJob(t, svc, job.ID, 2*time.Second)
	if current.Status != JobStatusCompleted {
		t.Fatalf("expected completed status, got %s", current.Status)
	}

	events, unsubscribe, err := svc.SubscribeForUser(job.ID, "u1", "")
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsubscribe()

	gotSnapshot := false
	gotTerminal := false
	timeout := time.After(2 * time.Second)
	for !gotSnapshot || !gotTerminal {
		select {
		case ev := <-events:
			if ev.ID == "" {
				t.Fatalf("expected event id to be non-empty, event=%s", ev.Type)
			}
			if ev.Type == "job_snapshot" {
				gotSnapshot = true
			}
			if ev.Type == "job_completed" || ev.Type == "job_failed" || ev.Type == "job_cancelled" {
				gotTerminal = true
			}
		case <-timeout:
			t.Fatalf("timeout waiting events, snapshot=%v terminal=%v", gotSnapshot, gotTerminal)
		}
	}
}

func TestSynthesizeReport_ConflictMetrics(t *testing.T) {
	report := synthesizeReport("feature", "en-US", []Evidence{
		{
			ID:               "ev1",
			Title:            "Official docs confirm feature rollout",
			URL:              "https://example.com/1",
			Domain:           "example.com",
			RelevanceScore:   0.9,
			CredibilityScore: 0.8,
			ClaimKey:         "feature|rollout",
		},
		{
			ID:               "ev2",
			Title:            "Rumor says feature is not available",
			URL:              "https://example.com/2",
			Domain:           "example.com",
			RelevanceScore:   0.8,
			CredibilityScore: 0.7,
			ClaimKey:         "feature|rollout",
		},
	})
	if !report.HasConflict {
		t.Fatalf("expected HasConflict=true")
	}
	if report.SupportCount == 0 {
		t.Fatalf("expected SupportCount > 0")
	}
	if report.ConflictCount == 0 {
		t.Fatalf("expected ConflictCount > 0")
	}
	if len(report.OpenQuestions) < 2 {
		t.Fatalf("expected conflict open question appended, got %v", report.OpenQuestions)
	}
}

func TestServiceSynthesizeReport_SummaryOverride(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return nil, nil
		},
	})
	svc.SetSummarySynthesizer(&mockSummarySynth{
		summarize: func(ctx context.Context, input SummaryInput) (string, error) {
			if strings.TrimSpace(input.Draft) == "" {
				t.Fatalf("expected non-empty draft answer")
			}
			return "override summary", nil
		},
	})

	report := svc.synthesizeReport(context.Background(), "q", "en-US", []Evidence{
		{
			ID:               "ev1",
			Title:            "Doc",
			URL:              "https://example.com/1",
			Domain:           "example.com",
			RelevanceScore:   0.8,
			CredibilityScore: 0.9,
		},
	})
	if got := report.Answer; got != "override summary" {
		t.Fatalf("report answer = %q, want override summary", got)
	}
}

func TestClampBudget(t *testing.T) {
	fast := clampBudget(ModeFast, Budget{MaxSteps: 999, MaxSources: 999, MaxSeconds: 999})
	fastDef := defaultBudget(ModeFast)
	if fast.MaxSteps != fastDef.MaxSteps*2 {
		t.Fatalf("fast MaxSteps clamp = %d, want %d", fast.MaxSteps, fastDef.MaxSteps*2)
	}
	if fast.MaxSources != fastDef.MaxSources*2 {
		t.Fatalf("fast MaxSources clamp = %d, want %d", fast.MaxSources, fastDef.MaxSources*2)
	}
	if fast.MaxSeconds != fastDef.MaxSeconds*2 {
		t.Fatalf("fast MaxSeconds clamp = %d, want %d", fast.MaxSeconds, fastDef.MaxSeconds*2)
	}
}

func TestSearchWithRetryEventuallySucceeds(t *testing.T) {
	var calls int32
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			if atomic.AddInt32(&calls, 1) < 3 {
				return nil, errors.New("temporary")
			}
			return []SearchHit{{Title: "ok", URL: "https://example.com", Description: "ok"}}, nil
		},
	})
	hits, err := svc.searchWithRetry(context.Background(), "job-1", "retry test", 5, "en-US")
	if err != nil {
		t.Fatalf("searchWithRetry failed: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected non-empty hits")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
}

func TestSynthesizeReportWithOptions_TimelineAndCoverage(t *testing.T) {
	ts := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	report := synthesizeReportWithOptions("付强 观点调研", "zh-CN", []Evidence{
		{
			ID:               "ev1",
			Title:            "2024 访谈观点",
			URL:              "https://example.com/1",
			Snippet:          "观点一",
			PublishedAt:      &ts,
			RelevanceScore:   0.9,
			CredibilityScore: 0.8,
		},
	}, reportBuildOptions{
		ReportStyle:      "timeline",
		UseTimelineStyle: true,
		StageErrors:      []string{"search failed: test"},
	})
	if len(report.TimelineSections) == 0 {
		t.Fatalf("expected timeline sections")
	}
	if report.CitationCoverage <= 0 {
		t.Fatalf("expected citation coverage > 0")
	}
	if len(report.StageErrors) == 0 {
		t.Fatalf("expected stage errors to be preserved")
	}
}

func TestServiceRunJob_V2StrictEntityFiltersNoise(t *testing.T) {
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{{
				ID:          "task_1",
				Question:    "付强 蓝驰 观点",
				Priority:    1,
				Depth:       1,
				Status:      "pending",
				NegKeywords: []string{"汽车", "无关"},
			}}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{
				{Title: "蓝驰创投付强观点", URL: "https://example.com/ok", Description: "付强 蓝驰 合伙人"},
				{Title: "某汽车公司付强", URL: "https://example.com/noise", Description: "与蓝驰无关 汽车"},
			}, nil
		},
	})
	svc.SetV2Enabled(true)
	strict := true

	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:        "蓝驰创投 付强 合伙人 观点",
		Mode:         ModeStandard,
		StrictEntity: &strict,
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}
	current := waitForTerminalJob(t, svc, job.ID, 3*time.Second)
	if current.Status != JobStatusCompleted {
		t.Fatalf("expected completed status, got %s", current.Status)
	}
	if len(current.Evidence) != 1 {
		t.Fatalf("expected 1 evidence after strict filtering, got %d", len(current.Evidence))
	}
	if current.Report == nil || current.Report.EntityDisambiguation == nil {
		t.Fatalf("expected entity disambiguation report section")
	}
	if current.Report.EntityDisambiguation.FilteredCount == 0 {
		t.Fatalf("expected filtered_count > 0")
	}
}
