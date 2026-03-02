package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMemoryHandlerHTTPE2EUserFlow(t *testing.T) {
	_, e := newTestMemoryHandler(t)

	type statsResp struct {
		TotalChunks       int `json:"total_chunks"`
		TotalDisplayCount int `json:"total_display_count"`
	}
	type storeResp struct {
		ID string `json:"id"`
	}
	type searchItem struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	type searchResp struct {
		Results []searchItem `json:"results"`
		Total   int          `json:"total"`
	}
	type importResp struct {
		Imported int      `json:"imported"`
		Skipped  int      `json:"skipped"`
		Errors   []string `json:"errors"`
	}

	requestJSON := func(method, path string, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set(echoHeaderContentType, echoMIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	// 1) initial stats
	{
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/memory/stats", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("initial stats status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		var s statsResp
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatalf("decode initial stats failed: %v", err)
		}
		if s.TotalChunks != 0 {
			t.Fatalf("initial total_chunks = %d, want 0", s.TotalChunks)
		}
	}

	// 2) store
	var storedID string
	{
		rec := requestJSON(http.MethodPost, "/api/v1/memory/store", `{"content":"User prefers concise UI copy","tags":["pref","ui"]}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("store status = %d, want 201, body=%s", rec.Code, rec.Body.String())
		}
		var s storeResp
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatalf("decode store response failed: %v", err)
		}
		if strings.TrimSpace(s.ID) == "" {
			t.Fatalf("stored id should not be empty, body=%s", rec.Body.String())
		}
		storedID = s.ID
	}

	// 3) search and capture searchable id (path-based id used by markdown backend)
	var searchableID string
	{
		rec := requestJSON(http.MethodPost, "/api/v1/memory/search", `{"query":"concise UI","limit":10}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("search status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		var s searchResp
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatalf("decode search response failed: %v", err)
		}
		if s.Total < 1 || len(s.Results) < 1 {
			t.Fatalf("search expected >=1 result, got total=%d body=%s", s.Total, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(s.Results[0].Content), "concise") {
			t.Fatalf("search first result mismatch: %+v", s.Results[0])
		}
		searchableID = s.Results[0].ID
	}

	// 4) get by searchable id
	{
		getPath := "/api/v1/memory/" + url.PathEscape(searchableID)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, getPath, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("get status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "concise") {
			t.Fatalf("get response should contain stored content, body=%s", rec.Body.String())
		}
	}

	// 5) delete by searchable id (which removes the markdown file entry source)
	{
		delPath := "/api/v1/memory/" + url.PathEscape(searchableID)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, delPath, nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("delete status = %d, want 204, body=%s", rec.Code, rec.Body.String())
		}
	}

	// store id is currently opaque for markdown backend; it should not crash later calls.
	_ = storedID

	// 6) import append
	{
		rec := requestJSON(http.MethodPost, "/api/v1/memory/import", `{"content":"first imported\n\n---\n\nsecond imported","mode":"append"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("import append status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		var r importResp
		if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
			t.Fatalf("decode import append response failed: %v", err)
		}
		if r.Imported < 2 || len(r.Errors) != 0 {
			t.Fatalf("import append unexpected response: %+v", r)
		}
	}

	// 7) export markdown
	{
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/memory/export", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("export status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/markdown") {
			t.Fatalf("export content-type = %q, want text/markdown", ct)
		}
		if !strings.Contains(rec.Body.String(), "# ZimaOS-Blue Memory Export") {
			t.Fatalf("export body missing header, body=%s", rec.Body.String())
		}
	}

	// 8) import replace and verify old/imported replaced
	{
		rec := requestJSON(http.MethodPost, "/api/v1/memory/import", `{"content":"replacement only","mode":"replace"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("import replace status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	{
		rec := requestJSON(http.MethodPost, "/api/v1/memory/search", `{"query":"first imported","limit":10}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("search old after replace status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "first imported") {
			t.Fatalf("old imported content should be absent after replace, body=%s", rec.Body.String())
		}
	}
	{
		rec := requestJSON(http.MethodPost, "/api/v1/memory/search", `{"query":"replacement only","limit":10}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("search replacement status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "replacement only") {
			t.Fatalf("replacement content should be searchable, body=%s", rec.Body.String())
		}
	}

	// 9) prune + clear + final stats
	{
		rec := requestJSON(http.MethodPost, "/api/v1/memory/prune", `{}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("prune status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	{
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/memory", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("clear status = %d, want 204, body=%s", rec.Code, rec.Body.String())
		}
	}
	{
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/memory/stats", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("final stats status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		var s statsResp
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatalf("decode final stats failed: %v", err)
		}
		if s.TotalChunks != 0 {
			t.Fatalf("final total_chunks = %d, want 0", s.TotalChunks)
		}
	}
}

const (
	echoHeaderContentType   = "Content-Type"
	echoMIMEApplicationJSON = "application/json"
)
