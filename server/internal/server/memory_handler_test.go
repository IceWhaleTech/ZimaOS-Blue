package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
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
