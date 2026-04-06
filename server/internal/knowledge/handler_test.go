package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

type timeoutKnowledgeJobCreator struct{}

func (timeoutKnowledgeJobCreator) CreateJob(ctx context.Context, req CreateJobRequest) (*KnowledgeJob, error) {
	_ = req
	return nil, fmt.Errorf("waiting for knowledge slot: %w", context.DeadlineExceeded)
}

type blockingMemorySink struct {
	release <-chan struct{}
}

func (s blockingMemorySink) Remember(ctx context.Context, content string, tags []string) error {
	_, _ = content, tags
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return nil
	}
}

func TestHandlerCreateListGetAndReportKnowledgeJobs(t *testing.T) {
	release := make(chan struct{})
	service, _ := newKnowledgeHandlerTestService(t, ServiceOptions{
		MemorySink: blockingMemorySink{release: release},
	})
	handler := NewHandler(service)

	e := echo.New()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/jobs", bytes.NewBufferString(`{"kind":"compile"}`))
	createReq = withKnowledgeUserClaims(createReq, "u1")
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	if err := handler.CreateJob(e.NewContext(createReq, createRec)); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", createRec.Code, http.StatusCreated)
	}

	var created KnowledgeJob
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if strings.TrimSpace(created.ID) == "" {
		t.Fatal("created job id is empty")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/jobs?status=active", nil)
	listReq = withKnowledgeUserClaims(listReq, "u1")
	listRec := httptest.NewRecorder()
	if err := handler.ListJobs(e.NewContext(listReq, listRec)); err != nil {
		t.Fatalf("ListJobs failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}
	var listed []KnowledgeJobSummary
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("active jobs = %#v, want one job %q", listed, created.ID)
	}

	close(release)
	waitForTerminalKnowledgeJob(t, service, created.ID, 2*time.Second)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/jobs/"+created.ID, nil)
	getReq = withKnowledgeUserClaims(getReq, "u1")
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetParamNames("id")
	getCtx.SetParamValues(created.ID)
	if err := handler.GetJob(getCtx); err != nil {
		t.Fatalf("GetJob failed: %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d", getRec.Code, http.StatusOK)
	}
	var got KnowledgeJob
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if got.Status != JobStatusCompleted {
		t.Fatalf("job status = %q, want %q", got.Status, JobStatusCompleted)
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/jobs/"+created.ID+"/report", nil)
	reportReq = withKnowledgeUserClaims(reportReq, "u1")
	reportRec := httptest.NewRecorder()
	reportCtx := e.NewContext(reportReq, reportRec)
	reportCtx.SetParamNames("id")
	reportCtx.SetParamValues(created.ID)
	if err := handler.GetReport(reportCtx); err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if reportRec.Code != http.StatusOK {
		t.Fatalf("report status = %d, want %d", reportRec.Code, http.StatusOK)
	}
	var report KnowledgeJobReport
	if err := json.Unmarshal(reportRec.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode report response: %v", err)
	}
	if report.Kind != JobKindIngest || report.Ingest == nil || report.Compile == nil {
		t.Fatalf("report = %#v, want ingest report with compile alias", report)
	}
}

func TestHandlerGetJobForbidden(t *testing.T) {
	service, _ := newKnowledgeHandlerTestService(t, ServiceOptions{})
	handler := NewHandler(service)
	job, err := service.CreateJob(context.Background(), CreateJobRequest{
		Kind:   JobKindCompile,
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	waitForTerminalKnowledgeJob(t, service, job.ID, 2*time.Second)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/jobs/"+job.ID, nil)
	req = withKnowledgeUserClaims(req, "u2")
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

func TestHandlerCreateJobTimeoutReturnsRequestTimeout(t *testing.T) {
	service, _ := newKnowledgeHandlerTestService(t, ServiceOptions{})
	handler := NewHandler(service)
	handler.SetJobCreator(timeoutKnowledgeJobCreator{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/jobs", bytes.NewBufferString(`{"kind":"compile"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	if err := handler.CreateJob(e.NewContext(req, rec)); err != nil {
		t.Fatalf("CreateJob failed: %v", err)
	}
	if rec.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestTimeout)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["code"] != "request_timeout" {
		t.Fatalf("code = %q, want request_timeout", payload["code"])
	}
}

func TestHandlerGetReportNotReady(t *testing.T) {
	release := make(chan struct{})
	service, _ := newKnowledgeHandlerTestService(t, ServiceOptions{
		MemorySink: blockingMemorySink{release: release},
	})
	handler := NewHandler(service)
	job, err := service.CreateJob(context.Background(), CreateJobRequest{
		Kind:   JobKindCompile,
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/jobs/"+job.ID+"/report", nil)
	req = withKnowledgeUserClaims(req, "u1")
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
	waitForTerminalKnowledgeJob(t, service, job.ID, 2*time.Second)
}

func TestHandlerCancelJobAlreadyTerminal(t *testing.T) {
	service, _ := newKnowledgeHandlerTestService(t, ServiceOptions{})
	handler := NewHandler(service)
	job, err := service.CreateJob(context.Background(), CreateJobRequest{
		Kind:   JobKindCompile,
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	waitForTerminalKnowledgeJob(t, service, job.ID, 2*time.Second)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/jobs/"+job.ID+"/cancel", bytes.NewBufferString("{}"))
	req = withKnowledgeUserClaims(req, "u1")
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
	service, _ := newKnowledgeHandlerTestService(t, ServiceOptions{
		MemorySink: blockingMemorySink{release: release},
	})
	handler := NewHandler(service)
	job, err := service.CreateJob(context.Background(), CreateJobRequest{
		Kind:   JobKindCompile,
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	defer close(release)

	e := echo.New()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/jobs/"+job.ID+"/events", nil).WithContext(ctx)
	req = withKnowledgeUserClaims(req, "u1")
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

func TestHandlerPageIndexAndLatestLintEndpoints(t *testing.T) {
	service, compileReport := newKnowledgeHandlerTestService(t, ServiceOptions{})
	handler := NewHandler(service)
	if _, err := service.Lint(context.Background(), LintRequest{}); err != nil {
		t.Fatalf("Lint() error = %v", err)
	}

	e := echo.New()

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/pages", nil)
	listRec := httptest.NewRecorder()
	if err := handler.ListPages(e.NewContext(listReq, listRec)); err != nil {
		t.Fatalf("ListPages failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("list pages status = %d, want %d", listRec.Code, http.StatusOK)
	}
	var pages []KnowledgePageSummary
	if err := json.Unmarshal(listRec.Body.Bytes(), &pages); err != nil {
		t.Fatalf("decode pages: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("expected non-empty pages response")
	}

	pageReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/pages/"+compileReport.Pages[0].Slug, nil)
	pageRec := httptest.NewRecorder()
	pageCtx := e.NewContext(pageReq, pageRec)
	pageCtx.SetParamNames("slug")
	pageCtx.SetParamValues(compileReport.Pages[0].Slug)
	if err := handler.GetPage(pageCtx); err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}
	if pageRec.Code != http.StatusOK {
		t.Fatalf("page status = %d, want %d", pageRec.Code, http.StatusOK)
	}

	indexReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/index", nil)
	indexRec := httptest.NewRecorder()
	if err := handler.GetIndex(e.NewContext(indexReq, indexRec)); err != nil {
		t.Fatalf("GetIndex failed: %v", err)
	}
	if indexRec.Code != http.StatusOK {
		t.Fatalf("index status = %d, want %d", indexRec.Code, http.StatusOK)
	}
	if !strings.Contains(indexRec.Body.String(), "content") {
		t.Fatalf("expected index content payload, got %q", indexRec.Body.String())
	}

	lintReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/lint/latest", nil)
	lintRec := httptest.NewRecorder()
	if err := handler.GetLatestLint(e.NewContext(lintReq, lintRec)); err != nil {
		t.Fatalf("GetLatestLint failed: %v", err)
	}
	if lintRec.Code != http.StatusOK {
		t.Fatalf("lint status = %d, want %d", lintRec.Code, http.StatusOK)
	}
}

func TestHandlerSchemaLogAndPromoteEndpoints(t *testing.T) {
	service, compileReport := newKnowledgeHandlerTestService(t, ServiceOptions{})
	handler := NewHandler(service)

	answerJob, err := service.CreateJob(context.Background(), CreateJobRequest{
		Kind:          JobKindAnswer,
		Query:         "How does Blue knowledge work?",
		PageSlug:      compileReport.Pages[0].Slug,
		ArchiveAnswer: true,
		UserID:        "u1",
	})
	if err != nil {
		t.Fatalf("CreateJob(answer) error = %v", err)
	}
	waitForTerminalKnowledgeJob(t, service, answerJob.ID, 2*time.Second)

	e := echo.New()

	getSchemaReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/schema", nil)
	getSchemaRec := httptest.NewRecorder()
	if err := handler.GetSchema(e.NewContext(getSchemaReq, getSchemaRec)); err != nil {
		t.Fatalf("GetSchema failed: %v", err)
	}
	if getSchemaRec.Code != http.StatusOK {
		t.Fatalf("schema status = %d, want %d", getSchemaRec.Code, http.StatusOK)
	}
	if !strings.Contains(getSchemaRec.Body.String(), "Knowledge Space Schema") {
		t.Fatalf("expected schema content, got %q", getSchemaRec.Body.String())
	}

	updateSchemaReq := httptest.NewRequest(http.MethodPut, "/api/v1/knowledge/schema", bytes.NewBufferString(`{"content":"# Updated Schema\n\nKeep syntheses durable.\n"}`))
	updateSchemaReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updateSchemaRec := httptest.NewRecorder()
	if err := handler.UpdateSchema(e.NewContext(updateSchemaReq, updateSchemaRec)); err != nil {
		t.Fatalf("UpdateSchema failed: %v", err)
	}
	if updateSchemaRec.Code != http.StatusOK {
		t.Fatalf("update schema status = %d, want %d", updateSchemaRec.Code, http.StatusOK)
	}

	logReq := httptest.NewRequest(http.MethodGet, "/api/v1/knowledge/log", nil)
	logRec := httptest.NewRecorder()
	if err := handler.GetLog(e.NewContext(logReq, logRec)); err != nil {
		t.Fatalf("GetLog failed: %v", err)
	}
	if logRec.Code != http.StatusOK {
		t.Fatalf("log status = %d, want %d", logRec.Code, http.StatusOK)
	}
	if !strings.Contains(logRec.Body.String(), "\"operation\":\"schema\"") {
		t.Fatalf("expected schema log entry, got %q", logRec.Body.String())
	}

	promoteReq := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/query/"+answerJob.ID+"/promote", nil)
	promoteReq = withKnowledgeUserClaims(promoteReq, "u1")
	promoteRec := httptest.NewRecorder()
	promoteCtx := e.NewContext(promoteReq, promoteRec)
	promoteCtx.SetParamNames("id")
	promoteCtx.SetParamValues(answerJob.ID)
	if err := handler.PromoteQuery(promoteCtx); err != nil {
		t.Fatalf("PromoteQuery failed: %v", err)
	}
	if promoteRec.Code != http.StatusCreated {
		t.Fatalf("promote status = %d, want %d", promoteRec.Code, http.StatusCreated)
	}
	if !strings.Contains(promoteRec.Body.String(), "\"page_type\":\"synthesis\"") {
		t.Fatalf("expected promoted synthesis payload, got %q", promoteRec.Body.String())
	}
}

func TestHandlerUpdateSchemaRejectsEmptyContent(t *testing.T) {
	service, _ := newKnowledgeHandlerTestService(t, ServiceOptions{})
	handler := NewHandler(service)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPut, "/api/v1/knowledge/schema", bytes.NewBufferString(`{"content":"   "}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := handler.UpdateSchema(e.NewContext(req, rec)); err != nil {
		t.Fatalf("UpdateSchema failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "schema content is required") {
		t.Fatalf("expected schema validation error, got %q", rec.Body.String())
	}
}

func TestHandlerPromoteQueryReturnsTooEarlyWhenReportNotReady(t *testing.T) {
	release := make(chan struct{})
	service, _ := newKnowledgeHandlerTestService(t, ServiceOptions{
		MemorySink: blockingMemorySink{release: release},
	})
	handler := NewHandler(service)
	e := echo.New()

	job, err := service.CreateJob(context.Background(), CreateJobRequest{
		Kind:   JobKindIngest,
		UserID: "u1",
	})
	if err != nil {
		t.Fatalf("CreateJob(ingest) error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/knowledge/query/"+job.ID+"/promote", nil)
	req = withKnowledgeUserClaims(req, "u1")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetParamNames("id")
	ctx.SetParamValues(job.ID)
	if err := handler.PromoteQuery(ctx); err != nil {
		t.Fatalf("PromoteQuery failed: %v", err)
	}
	if rec.Code != http.StatusTooEarly {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooEarly)
	}
	if !strings.Contains(rec.Body.String(), "\"code\":\"not_ready\"") {
		t.Fatalf("expected not_ready error payload, got %q", rec.Body.String())
	}

	close(release)
	waitForTerminalKnowledgeJob(t, service, job.ID, 2*time.Second)
}

func newKnowledgeHandlerTestService(t *testing.T, options ServiceOptions) (*Service, *KnowledgeCompileReport) {
	t.Helper()

	repoRoot := t.TempDir()
	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "README.md"), "# Blue Knowledge\n\nBlue compiled knowledge pages.\n")
	writeKnowledgeTestFile(t, filepath.Join(repoRoot, "ARCHITECTURE.md"), "# Blue Architecture\n\nKnowledge architecture and runtime context.\n")

	options.WorkspaceDir = workspaceDir
	options.RepoRoot = repoRoot
	service := NewService(options)

	var report *KnowledgeCompileReport
	if options.MemorySink == nil {
		var err error
		report, err = service.Compile(context.Background(), CompileRequest{})
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
	}
	return service, report
}

func withKnowledgeUserClaims(req *http.Request, userID string) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: userID})
	return req.WithContext(ctx)
}

func waitForTerminalKnowledgeJob(t *testing.T, service *Service, jobID string, timeout time.Duration) *KnowledgeJob {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		job, err := service.GetJob(jobID)
		if err != nil {
			t.Fatalf("GetJob() error = %v", err)
		}
		switch job.Status {
		case JobStatusCompleted, JobStatusFailed, JobStatusCancelled:
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for terminal job %q", jobID)
	return nil
}
