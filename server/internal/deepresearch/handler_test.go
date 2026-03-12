package deepresearch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func withUserClaims(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: userID})
	return req.WithContext(ctx)
}

func TestHandlerGetJobForbidden(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{{Title: "Doc", URL: "https://example.com/a", Description: "A"}}, nil
		},
	})
	handler := NewHandler(svc)
	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:  "owner test",
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deep-research/jobs/"+job.ID, nil)
	req = withUserClaims(req, "u2")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(job.ID)

	if err := handler.GetJob(c); err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, _ := body["code"].(string); got != "forbidden" {
		t.Fatalf("error code = %q, want forbidden", got)
	}
}

func TestHandlerGetReportNotReady(t *testing.T) {
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
	handler := NewHandler(svc)
	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:  "report not ready",
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deep-research/jobs/"+job.ID+"/report", nil)
	req = withUserClaims(req, "u1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(job.ID)

	if err := handler.GetReport(c); err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if rec.Code != http.StatusTooEarly {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooEarly)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, _ := body["code"].(string); got != "not_ready" {
		t.Fatalf("error code = %q, want not_ready", got)
	}

	close(release)
	_ = waitForTerminalJob(t, svc, job.ID, 2*time.Second)
}

func TestHandlerCancelJobAlreadyTerminal(t *testing.T) {
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			return []SearchHit{{Title: "Doc", URL: "https://example.com/a", Description: "A"}}, nil
		},
	})
	handler := NewHandler(svc)
	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:  "cancel terminal",
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}
	_ = waitForTerminalJob(t, svc, job.ID, 2*time.Second)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deep-research/jobs/"+job.ID+"/cancel", bytes.NewBufferString("{}"))
	req = withUserClaims(req, "u1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(job.ID)

	if err := handler.CancelJob(c); err != nil {
		t.Fatalf("CancelJob failed: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got, _ := body["code"].(string); got != "already_terminal" {
		t.Fatalf("error code = %q, want already_terminal", got)
	}
}

func TestHandlerStreamEventsIncludesRetryAndEventID(t *testing.T) {
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
	handler := NewHandler(svc)
	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:  "events",
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}
	defer close(release)

	e := echo.New()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deep-research/jobs/"+job.ID+"/events", nil).WithContext(ctx)
	req = withUserClaims(req, "u1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(job.ID)

	if err := handler.StreamEvents(c); err != nil {
		t.Fatalf("StreamEvents failed: %v", err)
	}
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(body, "retry: 2000") {
		t.Fatalf("expected retry line in SSE stream, body=%q", body)
	}
	if !strings.Contains(body, "event: job_snapshot") {
		t.Fatalf("expected snapshot event in SSE stream, body=%q", body)
	}
	if !strings.Contains(body, "\nid: ") {
		t.Fatalf("expected event id line in SSE stream, body=%q", body)
	}
}

func TestHandlerStreamEventsIncludesDeepResearchLoopEvents(t *testing.T) {
	release := make(chan struct{})
	svc := NewService(&mockPlanner{
		plan: func(query string, mode Mode, lang string) []Task {
			return []Task{{
				ID:       "task_1",
				Question: "topic overview",
				Priority: 1,
				Depth:    1,
				Status:   "pending",
				Axis:     "official",
			}}
		},
	}, &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			if strings.Contains(strings.ToLower(query), "official") || strings.Contains(query, "primary source") {
				return []SearchHit{{Title: "Official docs", URL: "https://docs.example.com/official", Description: "official documentation"}}, nil
			}
			return []SearchHit{{Title: "Blog post", URL: "https://blog.example.com/post", Description: "analysis"}}, nil
		},
	})
	handler := NewHandler(svc)
	job, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:  "topic",
		Mode:   ModeStandard,
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("create job failed: %v", err)
	}

	e := echo.New()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deep-research/jobs/"+job.ID+"/events", nil).WithContext(ctx)
	req = withUserClaims(req, "u1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(job.ID)

	errCh := make(chan error, 1)
	go func() {
		errCh <- handler.StreamEvents(c)
	}()

	time.Sleep(50 * time.Millisecond)
	close(release)
	_ = waitForTerminalJob(t, svc, job.ID, 3*time.Second)
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("StreamEvents failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for StreamEvents to exit")
	}

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	for _, eventType := range []string{"verification_completed", "gap_detected", "followup_planned", "loop_stopped", "job_completed"} {
		if !strings.Contains(body, "event: "+eventType) {
			t.Fatalf("expected %s in SSE stream, body=%q", eventType, body)
		}
	}
	for _, snippet := range []string{"\"iteration\":1", "\"follow_up_query\":", "\"latest_action\":\"loop_stopped\"", "\"stop_reason\":"} {
		if !strings.Contains(body, snippet) {
			t.Fatalf("expected %s in SSE stream, body=%q", snippet, body)
		}
	}
}

func TestHandlerListJobs_ActiveOnlyAndConversationID(t *testing.T) {
	release := make(chan struct{})
	svc := NewService(NewHeuristicPlanner(), &mockSearcher{
		search: func(ctx context.Context, query string, maxResults int, lang string) ([]SearchHit, error) {
			if strings.Contains(query, "blocking") {
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
			return []SearchHit{{Title: "Doc", URL: "https://example.com/a", Description: "A"}}, nil
		},
	})
	handler := NewHandler(svc)

	activeJob, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:          "blocking active list",
		UserID:         "u1",
		TenantID:       "tenant-1",
		ConversationID: "conv-1",
	})
	if err != nil {
		t.Fatalf("create active job failed: %v", err)
	}
	completedJob, err := svc.CreateJob(context.Background(), CreateJobRequest{
		Query:          "completed list",
		UserID:         "u1",
		TenantID:       "tenant-1",
		ConversationID: "conv-2",
	})
	if err != nil {
		t.Fatalf("create completed job failed: %v", err)
	}
	_, err = svc.CreateJob(context.Background(), CreateJobRequest{
		Query:          "blocking hidden list",
		UserID:         "u2",
		TenantID:       "tenant-1",
		ConversationID: "conv-hidden",
	})
	if err != nil {
		t.Fatalf("create hidden job failed: %v", err)
	}
	waitForTerminalJob(t, svc, completedJob.ID, 2*time.Second)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deep-research/jobs?status=active", nil)
	req = withUserClaims(req, "u1")
	req.Header.Set("X-Tenant-ID", "tenant-1")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListJobs(c); err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var jobs []JobSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs length = %d, want 1 (%#v)", len(jobs), jobs)
	}
	if jobs[0].JobID != activeJob.ID {
		t.Fatalf("job_id = %q, want %q", jobs[0].JobID, activeJob.ID)
	}
	if jobs[0].ConversationID != "conv-1" {
		t.Fatalf("conversation_id = %q, want %q", jobs[0].ConversationID, "conv-1")
	}
	if jobs[0].Status == JobStatusCompleted {
		t.Fatalf("expected active job, got completed")
	}

	close(release)
	waitForTerminalJob(t, svc, activeJob.ID, 2*time.Second)
}
