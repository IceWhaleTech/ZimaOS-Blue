package cron

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func setupCronHTTPHandler(t *testing.T) (*Service, *echo.Echo) {
	t.Helper()

	svc := NewService(DefaultConfig(), zap.NewNop())
	svc.RegisterBuiltinHandlers()
	svc.RegisterHandler("test-handler", func(ctx context.Context, job *Job) (interface{}, error) {
		return map[string]string{"job_id": job.ID}, nil
	})
	if err := svc.Start(); err != nil {
		t.Fatalf("failed to start cron service: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = svc.Stop(ctx)
	})

	e := echo.New()
	h := NewHandler(svc, zap.NewNop())
	h.RegisterRoutes(e.Group("/api"))
	return svc, e
}

func TestHandlerListWrappedJobsAlias(t *testing.T) {
	svc, e := setupCronHTTPHandler(t)
	if _, err := svc.Create("job-1", "", "* * * * *", "test-handler", nil); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/cron/jobs", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(resp.Jobs))
	}
	if resp.Jobs[0].Name != "job-1" {
		t.Fatalf("unexpected job name: %s", resp.Jobs[0].Name)
	}
}

func TestHandlerCreateViaJobsAlias(t *testing.T) {
	_, e := setupCronHTTPHandler(t)

	body := []byte(`{"name":"http-job","schedule":"*/5 * * * *","handler":"http","payload":{"url":"https://example.com"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/cron/jobs", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var job Job
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if job.Name != "http-job" {
		t.Fatalf("unexpected job name: %s", job.Name)
	}
	if job.Schedule != "0 */5 * * * *" {
		t.Fatalf("unexpected normalized schedule: %s", job.Schedule)
	}
}

func TestHandlerStatus(t *testing.T) {
	svc, e := setupCronHTTPHandler(t)
	job1, err := svc.Create("job-1", "", "* * * * *", "test-handler", nil)
	if err != nil {
		t.Fatalf("failed to create first job: %v", err)
	}
	if _, err := svc.Create("job-2", "", "*/5 * * * *", "test-handler", nil); err != nil {
		t.Fatalf("failed to create second job: %v", err)
	}
	if err := svc.Disable(job1.ID); err != nil {
		t.Fatalf("failed to disable job: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/cron/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Running    bool `json:"running"`
		JobCount   int  `json:"job_count"`
		ActiveJobs int  `json:"active_jobs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Running {
		t.Fatal("expected running=true")
	}
	if resp.JobCount != 2 {
		t.Fatalf("expected job_count=2, got %d", resp.JobCount)
	}
	if resp.ActiveJobs != 1 {
		t.Fatalf("expected active_jobs=1, got %d", resp.ActiveJobs)
	}
}

func TestHandlerExecutionsWrappedJobsAlias(t *testing.T) {
	svc, e := setupCronHTTPHandler(t)
	job, err := svc.Create("job-1", "", "0 0 0 1 1 *", "test-handler", nil)
	if err != nil {
		t.Fatalf("failed to create job: %v", err)
	}
	if err := svc.Trigger(job.ID); err != nil {
		t.Fatalf("failed to trigger job: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		executions, err := svc.GetExecutions(job.ID, 1)
		if err == nil && len(executions) == 1 && executions[0].Status != "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/cron/jobs/"+job.ID+"/executions", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Executions []JobExecution `json:"executions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(resp.Executions))
	}
	if resp.Executions[0].Status == "running" {
		t.Fatalf("expected execution to finish, got status=%s", resp.Executions[0].Status)
	}
}
