package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	sessionctx "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
)

func newTestMemoryHandler(t *testing.T) (*MemoryHandler, *echo.Echo) {
	t.Helper()

	baseDir := t.TempDir()

	backend, err := memory.NewPureMarkdownBackend(baseDir)
	if err != nil {
		t.Fatalf("create markdown backend failed: %v", err)
	}

	unified := memory.NewUnifiedMemoryService(backend)
	layered, err := memory.NewLayeredMemoryService(unified, memory.LayeredMemoryConfig{
		BaseDir:            baseDir,
		LongTermDir:        baseDir,
		DailyRetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("create layered memory service failed: %v", err)
	}

	handler := NewMemoryHandler()
	handler.SetUnifiedService(unified)
	handler.SetLayeredService(layered)

	e := echo.New()
	handler.RegisterRoutes(e.Group("/api/v1"))

	return handler, e
}

func newDreamSessionForHandlerTest() *session.Session {
	sess := session.NewSession(session.SessionID{
		AgentID:   "chat",
		ChannelID: "web",
		PeerID:    "conv-handler",
	}, 4096)
	sess.SetSummary("Important decision: dream memory should compress and archive historical context outside the primary storage.")
	sess.AddMessage(sessionctx.Message{Role: sessionctx.RoleUser, Content: "Please remember the important dream archive design."})
	sess.AddMessage(sessionctx.Message{Role: sessionctx.RoleAssistant, Content: "Confirmed. Dream memory will archive historical context outside the primary storage."})
	return sess
}

func TestMemoryHandlerStatsIncludesDailyDisplayCounters(t *testing.T) {
	h, e := newTestMemoryHandler(t)

	ctx := context.Background()
	if _, err := h.unifiedService.Remember(ctx, "user likes black coffee", []string{"pref"}); err != nil {
		t.Fatalf("remember failed: %v", err)
	}
	if err := h.layeredService.AppendToDaily(ctx, "always show concise answers", []string{"style"}); err != nil {
		t.Fatalf("append daily #1 failed: %v", err)
	}
	if err := h.layeredService.AppendToDaily(ctx, "prefers keyboard shortcuts", []string{"ux"}); err != nil {
		t.Fatalf("append daily #2 failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/stats", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	totalChunks := int(payload["total_chunks"].(float64))
	dailyEntries := int(payload["daily_entries_count"].(float64))
	totalDisplay := int(payload["total_display_count"].(float64))

	if dailyEntries < 2 {
		t.Fatalf("daily_entries_count = %d, want >= 2", dailyEntries)
	}
	if totalDisplay != totalChunks+dailyEntries {
		t.Fatalf("total_display_count = %d, want %d (total_chunks + daily_entries_count)", totalDisplay, totalChunks+dailyEntries)
	}
}

func TestCountDailyLogEntries(t *testing.T) {
	content := `# Daily Log - 2026-03-02

## 09:15:00

one

## invalid

ignored

## 10:45:30

two
`

	got := countDailyLogEntries(content)
	if got != 2 {
		t.Fatalf("countDailyLogEntries() = %d, want 2", got)
	}
}

func TestMemoryHandlerImportMarkdownAppendAndReplace(t *testing.T) {
	_, e := newTestMemoryHandler(t)

	appendBody := `{"content":"# ZimaOS-Blue Memory Export\n> Exported at: 2026-03-02 10:00:00\n\nfirst imported memory\n\n---\n\nsecond imported memory","mode":"append"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/import", bytes.NewBufferString(appendBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("append import status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var appendResp struct {
		Imported int      `json:"imported"`
		Skipped  int      `json:"skipped"`
		Errors   []string `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &appendResp); err != nil {
		t.Fatalf("decode append import response failed: %v", err)
	}
	if appendResp.Imported < 2 {
		t.Fatalf("append imported = %d, want >= 2", appendResp.Imported)
	}
	if len(appendResp.Errors) > 0 {
		t.Fatalf("append import errors = %v, want empty", appendResp.Errors)
	}

	searchReq := httptest.NewRequest(http.MethodPost, "/api/v1/memory/search", strings.NewReader(`{"query":"first imported memory","limit":10}`))
	searchReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	searchRec := httptest.NewRecorder()
	e.ServeHTTP(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("search after append status = %d, want 200, body=%s", searchRec.Code, searchRec.Body.String())
	}
	if !strings.Contains(searchRec.Body.String(), "first imported memory") {
		t.Fatalf("search after append should contain imported content, body=%s", searchRec.Body.String())
	}

	replaceBody := `{"content":"replacement memory only","mode":"replace"}`
	replaceReq := httptest.NewRequest(http.MethodPost, "/api/v1/memory/import", bytes.NewBufferString(replaceBody))
	replaceReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	replaceRec := httptest.NewRecorder()
	e.ServeHTTP(replaceRec, replaceReq)
	if replaceRec.Code != http.StatusOK {
		t.Fatalf("replace import status = %d, want 200, body=%s", replaceRec.Code, replaceRec.Body.String())
	}

	oldSearchReq := httptest.NewRequest(http.MethodPost, "/api/v1/memory/search", strings.NewReader(`{"query":"first imported memory","limit":10}`))
	oldSearchReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	oldSearchRec := httptest.NewRecorder()
	e.ServeHTTP(oldSearchRec, oldSearchReq)
	if oldSearchRec.Code != http.StatusOK {
		t.Fatalf("old search after replace status = %d, want 200, body=%s", oldSearchRec.Code, oldSearchRec.Body.String())
	}
	if strings.Contains(oldSearchRec.Body.String(), "first imported memory") {
		t.Fatalf("old content should be removed after replace, body=%s", oldSearchRec.Body.String())
	}

	newSearchReq := httptest.NewRequest(http.MethodPost, "/api/v1/memory/search", strings.NewReader(`{"query":"replacement memory only","limit":10}`))
	newSearchReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	newSearchRec := httptest.NewRecorder()
	e.ServeHTTP(newSearchRec, newSearchReq)
	if newSearchRec.Code != http.StatusOK {
		t.Fatalf("new search after replace status = %d, want 200, body=%s", newSearchRec.Code, newSearchRec.Body.String())
	}
	if !strings.Contains(newSearchRec.Body.String(), "replacement memory only") {
		t.Fatalf("replacement content should be searchable, body=%s", newSearchRec.Body.String())
	}
}

func TestMemoryHandlerImportMarkdownValidation(t *testing.T) {
	_, e := newTestMemoryHandler(t)

	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "empty content",
			body:       `{"content":"   ","mode":"append"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid mode",
			body:       `{"content":"valid content","mode":"overwrite"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/memory/import", bytes.NewBufferString(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestMemoryHandlerRejectsTraversalIDs(t *testing.T) {
	_, e := newTestMemoryHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/..%2Foutside.md", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestMemoryHandlerRejectsInvalidDailyDate(t *testing.T) {
	_, e := newTestMemoryHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memory/daily/..%2F2026-03-17", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestMemoryHandlerDreamStatusAndRun(t *testing.T) {
	h, e := newTestMemoryHandler(t)

	cfg := memory.DefaultDreamConfig()
	cfg.ArchiveDir = t.TempDir()
	cfg.SessionMinMessages = 2
	cfg.PromoteDailyAfterDays = 2
	cfg.ArchiveDailyAfterDays = 2
	dream, err := memory.NewDreamService(h.layeredService, h.layeredService.GetConfig().LongTermDir, cfg)
	if err != nil {
		t.Fatalf("NewDreamService: %v", err)
	}
	h.SetDreamService(dream)

	if _, err := dream.ArchiveSession(context.Background(), newDreamSessionForHandlerTest(), session.EndReasonArchive); err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}

	oldDate := time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02")
	oldLogPath := h.layeredService.GetConfig().BaseDir + "/daily/" + oldDate + ".md"
	if err := os.WriteFile(oldLogPath, []byte("# Daily Log - "+oldDate+"\n\n## 08:00:00\n\n**Tags:** important, decision\n\nImportant decision: dream memory should compress and archive historical context outside the primary storage.\n\n---\n"), 0o644); err != nil {
		t.Fatalf("write old daily log: %v", err)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/memory/dream/status", nil)
	statusRec := httptest.NewRecorder()
	e.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200, body=%s", statusRec.Code, statusRec.Body.String())
	}

	var statusResp struct {
		Enabled         bool `json:"enabled"`
		PendingCapsules int  `json:"pending_capsules"`
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}
	if !statusResp.Enabled || statusResp.PendingCapsules != 1 {
		t.Fatalf("status response = %+v, want enabled with pending_capsules=1", statusResp)
	}

	runReq := httptest.NewRequest(http.MethodPost, "/api/v1/memory/dream/run", bytes.NewBufferString(`{}`))
	runReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	runRec := httptest.NewRecorder()
	e.ServeHTTP(runRec, runReq)
	if runRec.Code != http.StatusOK {
		t.Fatalf("run code = %d, want 200, body=%s", runRec.Code, runRec.Body.String())
	}

	var runResp struct {
		RunID              string `json:"run_id"`
		PromotedCount      int    `json:"promoted_count"`
		ArchivedDailyCount int    `json:"archived_daily_count"`
		ProcessedCapsules  int    `json:"processed_capsules"`
	}
	if err := json.Unmarshal(runRec.Body.Bytes(), &runResp); err != nil {
		t.Fatalf("decode run response: %v", err)
	}
	if runResp.RunID == "" || runResp.PromotedCount == 0 || runResp.ArchivedDailyCount != 1 || runResp.ProcessedCapsules != 1 {
		t.Fatalf("run response = %+v", runResp)
	}

	statusReq = httptest.NewRequest(http.MethodGet, "/api/v1/memory/dream/status", nil)
	statusRec = httptest.NewRecorder()
	e.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("status code after run = %d, want 200, body=%s", statusRec.Code, statusRec.Body.String())
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode status response after run: %v", err)
	}
	if statusResp.PendingCapsules != 0 {
		t.Fatalf("pending_capsules after run = %d, want 0", statusResp.PendingCapsules)
	}
}

func TestParseMarkdownImportEntries(t *testing.T) {
	content := `# ZimaOS-Blue Memory Export
> Exported at: 2026-03-02 10:00:00
> Backend: markdown

first line
second line

---

## section

third line
`

	entries := parseMarkdownImportEntries(content)
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if strings.Contains(entries[0], "Exported at") || strings.Contains(entries[0], "Backend:") {
		t.Fatalf("entry[0] should strip export metadata, got=%q", entries[0])
	}
	if !strings.Contains(entries[0], "first line") || !strings.Contains(entries[0], "second line") {
		t.Fatalf("entry[0] content mismatch: %q", entries[0])
	}
	if !strings.Contains(entries[1], "third line") {
		t.Fatalf("entry[1] content mismatch: %q", entries[1])
	}
}
