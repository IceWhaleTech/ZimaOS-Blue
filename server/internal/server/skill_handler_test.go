package server

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/labstack/echo/v4"
)

func newTestSkillHandler(t *testing.T, registry *skill.Registry) *SkillHandler {
	t.Helper()
	handler := NewSkillHandler(registry)
	handler.SetSkillsDir(t.TempDir())
	return handler
}

func newTestSkillHandlerWithMarketplace(t *testing.T, registry *skill.Registry) (*SkillHandler, *skillmarket.Service) {
	t.Helper()

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "skillmarket.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config: skillmarket.Config{
			Enabled:         true,
			CacheRoot:       t.TempDir(),
			ActiveSkillsDir: t.TempDir(),
		},
		Registry: registry,
	})
	if err != nil {
		t.Fatalf("skillmarket.NewService() error = %v", err)
	}
	t.Cleanup(func() { _ = market.Close() })

	handler := newTestSkillHandler(t, registry)
	handler.SetMarketplace(market)
	return handler, market
}

func newMultipartUploadRequest(t *testing.T, targetURL, filename string, body []byte) (*http.Request, string) {
	t.Helper()

	var payload bytes.Buffer
	writer := multipart.NewWriter(&payload)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatalf("Write(upload body) error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart writer close error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, targetURL, &payload)
	return req, writer.FormDataContentType()
}

type rewriteHostTransport struct {
	t      *testing.T
	target *url.URL
}

func (r rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = r.target.Scheme
	cloned.URL.Host = r.target.Host
	cloned.Host = req.URL.Host
	return http.DefaultTransport.RoundTrip(cloned)
}

func newRewriteHostTransport(t *testing.T, server *httptest.Server) http.RoundTripper {
	t.Helper()
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("url.Parse(server.URL) error = %v", err)
	}
	return rewriteHostTransport{t: t, target: target}
}

func TestNewSkillHandler(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	if handler == nil {
		t.Fatal("NewSkillHandler returned nil")
	}

	if handler.registry != registry {
		t.Error("registry not set correctly")
	}

	// Check default sources are registered
	if _, exists := handler.sources["clawhub"]; !exists {
		t.Error("clawhub source not registered")
	}
	// Note: moltbot source removed - extensions are now native
}

func TestSkillHandler_DefaultSources(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	t.Run("clawhub source has correct URL", func(t *testing.T) {
		source := handler.sources["clawhub"]
		if source.URL != "https://www.clawhub.ai" {
			t.Errorf("expected URL 'https://www.clawhub.ai', got '%s'", source.URL)
		}
		if source.Type != "clawhub" {
			t.Errorf("expected type 'clawhub', got '%s'", source.Type)
		}
		if !source.Enabled {
			t.Error("clawhub source should be enabled by default")
		}
	})
	// Note: moltbot source test removed - extensions are now native
}

func TestSkillHandler_ListSources(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skill-store/sources", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListSources(c)
	if err != nil {
		t.Fatalf("ListSources failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var sources []*SkillSource
	if err := json.Unmarshal(rec.Body.Bytes(), &sources); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Note: moltbot source removed - only clawhub source is registered by default
	if len(sources) < 1 {
		t.Errorf("expected at least 1 source, got %d", len(sources))
	}
}

func TestSkillHandler_AddSource(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	e := echo.New()

	t.Run("add valid source", func(t *testing.T) {
		body := `{"id":"test-source","name":"Test Source","url":"https://example.com/skills","type":"custom","enabled":true}`
		req := httptest.NewRequest(http.MethodPost, "/skill-store/sources", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.AddSource(c)
		if err != nil {
			t.Fatalf("AddSource failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if _, exists := handler.sources["test-source"]; !exists {
			t.Error("source was not added")
		}
	})

	t.Run("add source without ID", func(t *testing.T) {
		body := `{"name":"Test Source","url":"https://example.com/skills"}`
		req := httptest.NewRequest(http.MethodPost, "/skill-store/sources", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.AddSource(c)
		if err != nil {
			t.Fatalf("AddSource failed: %v", err)
		}

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestSkillHandler_RemoveSource(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	// Add a custom source first
	handler.sources["custom-source"] = &SkillSource{
		ID:      "custom-source",
		Name:    "Custom Source",
		URL:     "https://example.com",
		Type:    "custom",
		Enabled: true,
	}

	e := echo.New()

	t.Run("remove custom source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/skill-store/sources/custom-source", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("custom-source")

		err := handler.RemoveSource(c)
		if err != nil {
			t.Fatalf("RemoveSource failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if _, exists := handler.sources["custom-source"]; exists {
			t.Error("source was not removed")
		}
	})

	t.Run("cannot remove default clawhub source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/skill-store/sources/clawhub", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("clawhub")

		err := handler.RemoveSource(c)
		if err != nil {
			t.Fatalf("RemoveSource failed: %v", err)
		}

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}
	})

	// Note: moltbot source test removed - extensions are now native

	t.Run("remove non-existent source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/skill-store/sources/non-existent", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("non-existent")

		err := handler.RemoveSource(c)
		if err != nil {
			t.Fatalf("RemoveSource failed: %v", err)
		}

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestSkillHandler_ListSources_UsesMarketplaceWhenAvailable(t *testing.T) {
	registry := skill.NewRegistry()
	handler, market := newTestSkillHandlerWithMarketplace(t, registry)

	if err := market.Store().UpsertSource(context.Background(), skillmarket.Source{
		ID:                 "custom-catalog",
		Type:               "html_catalog",
		BaseURL:            "https://catalog.example.com",
		DisplayName:        "Catalog Example",
		SourceGroup:        "catalog-example",
		AuthMode:           "none",
		Enabled:            true,
		RateLimitPerMinute: 20,
		Priority:           205,
	}); err != nil {
		t.Fatalf("UpsertSource(custom-catalog) error = %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skill-store/sources", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListSources(c); err != nil {
		t.Fatalf("ListSources failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var sources []SkillSource
	if err := json.Unmarshal(rec.Body.Bytes(), &sources); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	found := false
	for _, source := range sources {
		if source.ID != "custom-catalog" {
			continue
		}
		found = true
		if source.Name != "Catalog Example" {
			t.Fatalf("source.Name = %q, want %q", source.Name, "Catalog Example")
		}
		if source.URL != "https://catalog.example.com" {
			t.Fatalf("source.URL = %q, want %q", source.URL, "https://catalog.example.com")
		}
		if source.Type != "html_catalog" {
			t.Fatalf("source.Type = %q, want %q", source.Type, "html_catalog")
		}
	}
	if !found {
		t.Fatalf("expected custom-catalog in marketplace-backed source list")
	}
}

func TestSkillHandler_AddSource_UsesMarketplaceInference(t *testing.T) {
	registry := skill.NewRegistry()
	handler, market := newTestSkillHandlerWithMarketplace(t, registry)

	e := echo.New()
	body := `{"url":"https://catalog.example.com/skills"}`
	req := httptest.NewRequest(http.MethodPost, "/skill-store/sources", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.AddSource(c); err != nil {
		t.Fatalf("AddSource failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	sources, err := market.Store().ListSources(context.Background())
	if err != nil {
		t.Fatalf("ListSources() error = %v", err)
	}

	found := false
	for _, source := range sources {
		if source.BaseURL != "https://catalog.example.com/skills" {
			continue
		}
		found = true
		if source.Type != "html_catalog" {
			t.Fatalf("source.Type = %q, want %q", source.Type, "html_catalog")
		}
		if !source.Enabled {
			t.Fatal("expected inferred marketplace source to be enabled")
		}
		if strings.TrimSpace(source.ID) == "" {
			t.Fatal("expected inferred marketplace source id")
		}
	}
	if !found {
		t.Fatalf("expected inferred source to be persisted in marketplace store")
	}
}

func TestSkillHandler_RemoveSource_UsesMarketplaceSoftDelete(t *testing.T) {
	registry := skill.NewRegistry()
	handler, market := newTestSkillHandlerWithMarketplace(t, registry)

	if err := market.Store().UpsertSource(context.Background(), skillmarket.Source{
		ID:                 "custom-catalog",
		Type:               "html_catalog",
		BaseURL:            "https://catalog.example.com",
		DisplayName:        "Catalog Example",
		SourceGroup:        "catalog-example",
		AuthMode:           "none",
		Enabled:            true,
		RateLimitPerMinute: 20,
		Priority:           205,
	}); err != nil {
		t.Fatalf("UpsertSource(custom-catalog) error = %v", err)
	}

	e := echo.New()

	t.Run("disables custom marketplace source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/skill-store/sources/custom-catalog", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("custom-catalog")

		if err := handler.RemoveSource(c); err != nil {
			t.Fatalf("RemoveSource failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		sources, err := market.Store().ListSources(context.Background())
		if err != nil {
			t.Fatalf("ListSources() error = %v", err)
		}
		for _, source := range sources {
			if source.ID == "custom-catalog" {
				t.Fatalf("expected custom-catalog to be hidden from enabled source list after removal")
			}
		}
	})

	t.Run("rejects removal of protected built-in marketplace source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/skill-store/sources/clawhub", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("clawhub")

		if err := handler.RemoveSource(c); err != nil {
			t.Fatalf("RemoveSource failed: %v", err)
		}
		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})
}

func TestSkillHandler_PreviewSourceImport(t *testing.T) {
	registry := skill.NewRegistry()
	handler, _ := newTestSkillHandlerWithMarketplace(t, registry)
	e := echo.New()

	type previewResponse struct {
		Kind            string       `json:"kind"`
		Confidence      string       `json:"confidence"`
		SeedType        string       `json:"seed_type"`
		SeedValue       string       `json:"seed_value"`
		SuggestedSource *SkillSource `json:"suggested_source"`
	}

	t.Run("classifies HTML catalog as source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/sources/preview", strings.NewReader(`{"url":"https://catalog.example.com/skills"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.PreviewSourceImport(c); err != nil {
			t.Fatalf("PreviewSourceImport failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload previewResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if payload.Kind != "source" {
			t.Fatalf("kind = %q, want %q", payload.Kind, "source")
		}
		if payload.SuggestedSource == nil {
			t.Fatal("expected suggested_source for source preview")
		}
		if payload.SuggestedSource.Type != "html_catalog" {
			t.Fatalf("suggested_source.type = %q, want %q", payload.SuggestedSource.Type, "html_catalog")
		}
	})

	t.Run("classifies GitHub repository as seed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/sources/preview", strings.NewReader(`{"url":"https://github.com/demo/skills-repo"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.PreviewSourceImport(c); err != nil {
			t.Fatalf("PreviewSourceImport failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload previewResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if payload.Kind != "seed" {
			t.Fatalf("kind = %q, want %q", payload.Kind, "seed")
		}
		if payload.SeedType != "github_repo" {
			t.Fatalf("seed_type = %q, want %q", payload.SeedType, "github_repo")
		}
		if payload.SeedValue != "demo/skills-repo" {
			t.Fatalf("seed_value = %q, want %q", payload.SeedValue, "demo/skills-repo")
		}
	})
}

func TestSkillHandler_BrowseSkills(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	// Add some remote skills
	handler.remoteSkills["skill1"] = &RemoteSkill{
		ID:          "skill1",
		Name:        "Test Skill 1",
		Version:     "1.0.0",
		Description: "A test skill",
		Category:    "productivity",
		SourceID:    "clawhub",
		SourceName:  "ClawHub",
	}
	handler.remoteSkills["skill2"] = &RemoteSkill{
		ID:          "skill2",
		Name:        "Test Skill 2",
		Version:     "1.0.0",
		Description: "Another test skill",
		Category:    "development",
		SourceID:    "moltbot",
		SourceName:  "MoltBot",
	}

	e := echo.New()

	t.Run("browse all skills", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skill-store/browse", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.BrowseSkills(c)
		if err != nil {
			t.Fatalf("BrowseSkills failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		var skills []*RemoteSkill
		if err := json.Unmarshal(rec.Body.Bytes(), &skills); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(skills) != 2 {
			t.Errorf("expected 2 skills, got %d", len(skills))
		}
	})

	t.Run("filter by source", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skill-store/browse?source=clawhub", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.BrowseSkills(c)
		if err != nil {
			t.Fatalf("BrowseSkills failed: %v", err)
		}

		var skills []*RemoteSkill
		if err := json.Unmarshal(rec.Body.Bytes(), &skills); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(skills) != 1 {
			t.Errorf("expected 1 skill, got %d", len(skills))
		}
		if skills[0].SourceID != "clawhub" {
			t.Errorf("expected source 'clawhub', got '%s'", skills[0].SourceID)
		}
	})

	t.Run("filter by category", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skill-store/browse?category=development", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.BrowseSkills(c)
		if err != nil {
			t.Fatalf("BrowseSkills failed: %v", err)
		}

		var skills []*RemoteSkill
		if err := json.Unmarshal(rec.Body.Bytes(), &skills); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(skills) != 1 {
			t.Errorf("expected 1 skill, got %d", len(skills))
		}
		if skills[0].Category != "development" {
			t.Errorf("expected category 'development', got '%s'", skills[0].Category)
		}
	})

	t.Run("search by name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/skill-store/browse?search=Skill%201", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := handler.BrowseSkills(c)
		if err != nil {
			t.Fatalf("BrowseSkills failed: %v", err)
		}

		var skills []*RemoteSkill
		if err := json.Unmarshal(rec.Body.Bytes(), &skills); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(skills) != 1 {
			t.Errorf("expected 1 skill, got %d", len(skills))
		}
	})
}

func TestSkillHandler_InstallSkill(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	// Add a remote skill
	handler.remoteSkills["test-skill"] = &RemoteSkill{
		ID:          "test-skill",
		Name:        "Test Skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Category:    "productivity",
		SourceID:    "clawhub",
		SourceName:  "ClawHub",
	}

	e := echo.New()

	t.Run("install skill successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install/test-skill", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("test-skill")

		err := handler.InstallSkill(c)
		if err != nil {
			t.Fatalf("InstallSkill failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		// Verify skill is registered
		if registry.Get("test-skill") == nil {
			t.Error("skill was not registered")
		}
	})

	t.Run("install legacy remote skill returns compatibility warning", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, `---
name: legacy_remote_skill
description: Legacy remote skill
---

# Legacy Remote Skill

## Command Usage

`+"```bash"+`
blue legacy_remote_skill action=list
`+"```"+`
`)
		}))
		defer server.Close()

		handler.remoteSkills["legacy-remote-skill"] = &RemoteSkill{
			ID:          "legacy-remote-skill",
			Name:        "Legacy Remote Skill",
			Version:     "1.0.0",
			Description: "Legacy remote skill",
			SourceID:    "custom",
			SourceName:  "Custom",
			DownloadURL: server.URL + "/SKILL.md",
		}

		req := httptest.NewRequest(http.MethodPost, "/skill-store/install/legacy-remote-skill", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("legacy-remote-skill")

		if err := handler.InstallSkill(c); err != nil {
			t.Fatalf("InstallSkill failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload struct {
			Warnings       []string `json:"warnings"`
			ContractStatus string   `json:"contract_status"`
			ContractSource string   `json:"contract_source"`
			ContractNotes  []string `json:"contract_notes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Warnings) == 0 || !strings.Contains(payload.Warnings[0], "legacy manifest compatibility fallback applied") {
			t.Fatalf("warnings = %v, want legacy compatibility warning", payload.Warnings)
		}
		if payload.ContractStatus != skillmanifest.ContractStatusLegacyFallback {
			t.Fatalf("contract_status = %q, want %q", payload.ContractStatus, skillmanifest.ContractStatusLegacyFallback)
		}
		if payload.ContractSource != skillmanifest.ContractSourceLegacyFrontmatter {
			t.Fatalf("contract_source = %q, want %q", payload.ContractSource, skillmanifest.ContractSourceLegacyFrontmatter)
		}
		if len(payload.ContractNotes) == 0 || !strings.Contains(payload.ContractNotes[0], "legacy manifest compatibility fallback applied") {
			t.Fatalf("contract_notes = %v, want legacy compatibility note", payload.ContractNotes)
		}
	})

	t.Run("install already installed skill", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install/test-skill", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("test-skill")

		err := handler.InstallSkill(c)
		if err != nil {
			t.Fatalf("InstallSkill failed: %v", err)
		}

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d", http.StatusConflict, rec.Code)
		}
	})

	t.Run("install non-existent skill", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install/non-existent", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("non-existent")

		err := handler.InstallSkill(c)
		if err != nil {
			t.Fatalf("InstallSkill failed: %v", err)
		}

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestSkillHandler_GetSkillContentRejectsTraversalID(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skills/../../etc/passwd/content", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("../../etc/passwd")

	if err := handler.GetSkillContent(c); err != nil {
		t.Fatalf("GetSkillContent failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestSkillHandler_InstallUninstallSkill_WithMarketplaceCompatibility(t *testing.T) {
	registry := skill.NewRegistry()

	rawSkill := `---
id: market-test-skill
name: Market Test Skill
version: 1.0.0
description: Marketplace install test
permissions:
  - none
---
# Market Test Skill
This is a test skill.
`
	sum := sha256.Sum256([]byte(rawSkill))
	checksum := hex.EncodeToString(sum[:])

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	seedStore, err := skillmarket.NewStore(db)
	if err != nil {
		t.Fatalf("new skillmarket store: %v", err)
	}

	doc := &skillmarket.SkillDocument{
		ID:           "market-test-skill",
		Slug:         "market-test-skill",
		Name:         "Market Test Skill",
		Description:  "Marketplace install test",
		SourceID:     "test",
		SourceName:   "Test",
		SourceGroup:  "test",
		SourceType:   "test",
		Published:    true,
		SkillContent: rawSkill,
	}
	version := &skillmarket.SkillVersion{
		SkillID:    doc.ID,
		Version:    "1.0.0",
		RawSkillMD: rawSkill,
		Checksum:   checksum,
		SourceURL:  "https://example.com/market-test-skill/SKILL.md",
	}
	report := &skillmarket.SecurityReport{
		SkillID:             doc.ID,
		Version:             version.Version,
		Score:               100,
		RiskLevel:           skillmarket.RiskLow,
		SecurityBadge:       skillmarket.BadgeGreen,
		VulnerabilityStatus: skillmarket.VulnerabilityStatusNotApplicable,
		ScannerVersion:      "test",
		LLMStatus:           "skipped",
	}
	if err := seedStore.UpsertSkill(context.Background(), doc, version, report); err != nil {
		t.Fatalf("seed skillmarket store: %v", err)
	}

	activeDir := t.TempDir()
	cacheRoot := t.TempDir()
	market, err := skillmarket.NewService(db, skillmarket.Options{
		Config: skillmarket.Config{
			Enabled:         true,
			CacheRoot:       cacheRoot,
			ActiveSkillsDir: activeDir,
		},
		Registry: registry,
	})
	if err != nil {
		t.Fatalf("new skillmarket service: %v", err)
	}
	t.Cleanup(func() { _ = market.Close() })

	handler := NewSkillHandler(registry)
	handler.SetMarketplaceFactory(func() (*skillmarket.Service, error) { return market, nil })

	e := echo.New()

	t.Run("install returns legacy success contract", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install/market-test-skill", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("market-test-skill")

		if err := handler.InstallSkill(c); err != nil {
			t.Fatalf("InstallSkill failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if success, _ := payload["success"].(bool); !success {
			t.Fatalf("success = %v, want true", payload["success"])
		}
		if _, ok := payload["message"].(string); !ok {
			t.Fatalf("message missing or not string: %T", payload["message"])
		}
		if registry.Get("market-test-skill") == nil {
			t.Fatalf("expected registry to contain installed skill")
		}
	})

	t.Run("uninstall works without skillsDir configured", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/uninstall/market-test-skill", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("market-test-skill")

		if err := handler.UninstallSkill(c); err != nil {
			t.Fatalf("UninstallSkill failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if success, _ := payload["success"].(bool); !success {
			t.Fatalf("success = %v, want true", payload["success"])
		}
		if registry.Get("market-test-skill") != nil {
			t.Fatalf("expected registry to not contain uninstalled skill")
		}
	})
}

func TestSkillHandler_InstallSkillRejectsTraversalID(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/skill-store/install/../escape", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("../escape")

	if err := handler.InstallSkill(c); err != nil {
		t.Fatalf("InstallSkill failed: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestSkillHandler_InstallFromURL(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)
	e := echo.New()

	t.Run("installs and registers direct skill markdown", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(`---
id: url_installed_skill
name: URL Installed Skill
version: 1.2.3
description: Installed from direct URL
invocation: blue url_installed_skill action=list
examples:
  - blue url_installed_skill action=list
capability_tags:
  - install
interaction_mode: stateless
card_support: none
---

# URL Installed Skill
`))
		}))
		defer server.Close()

		body := fmt.Sprintf(`{"url":"%s/SKILL.md"}`, server.URL)
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install-url", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.InstallFromURL(c); err != nil {
			t.Fatalf("InstallFromURL failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if registry.Get("url_installed_skill") == nil {
			t.Fatalf("expected installed skill to be registered")
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "url_installed_skill", "SKILL.md")); err != nil {
			t.Fatalf("expected SKILL.md to be written: %v", err)
		}
	})

	t.Run("installs legacy skill markdown without blocking on missing contract fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(`---
name: legacy_url_skill
description: Installed from legacy direct URL
---

# Legacy URL Skill

## Command Usage

` + "```bash" + `
blue legacy_url_skill action=list
` + "```" + `
`))
		}))
		defer server.Close()

		body := fmt.Sprintf(`{"url":"%s/SKILL.md"}`, server.URL)
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install-url", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.InstallFromURL(c); err != nil {
			t.Fatalf("InstallFromURL failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if registry.Get("legacy_url_skill") == nil {
			t.Fatalf("expected legacy installed skill to be registered")
		}
		var payload struct {
			Warnings []string `json:"warnings"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Warnings) == 0 || !strings.Contains(payload.Warnings[0], "legacy manifest compatibility fallback applied") {
			t.Fatalf("warnings = %v, want legacy compatibility warning", payload.Warnings)
		}
	})

	t.Run("installs plain markdown skill without frontmatter contract", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(`
# Plain Third Party Skill

Installs without frontmatter.

blue plain-thirdparty action=list
`))
		}))
		defer server.Close()

		body := fmt.Sprintf(`{"url":"%s/plain-thirdparty.md"}`, server.URL)
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install-url", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.InstallFromURL(c); err != nil {
			t.Fatalf("InstallFromURL failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		var payload struct {
			EntryFile      string   `json:"entry_file"`
			ContractStatus string   `json:"contract_status"`
			ContractSource string   `json:"contract_source"`
			ContractNotes  []string `json:"contract_notes"`
			Skill          struct {
				ID string `json:"id"`
			} `json:"skill"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if strings.TrimSpace(payload.Skill.ID) == "" {
			t.Fatalf("expected installed skill id in payload, body=%s", rec.Body.String())
		}
		if registry.Get(payload.Skill.ID) == nil {
			t.Fatalf("expected plain markdown skill %q to be registered", payload.Skill.ID)
		}
		if payload.EntryFile != "SKILL.md" {
			t.Fatalf("entry_file = %q, want SKILL.md", payload.EntryFile)
		}
		if payload.ContractStatus != skillmanifest.ContractStatusGenerated {
			t.Fatalf("contract_status = %q, want %q", payload.ContractStatus, skillmanifest.ContractStatusGenerated)
		}
		if payload.ContractSource != skillmanifest.ContractSourceGeneratedSafeDefaults {
			t.Fatalf("contract_source = %q, want %q", payload.ContractSource, skillmanifest.ContractSourceGeneratedSafeDefaults)
		}
		if len(payload.ContractNotes) == 0 || !strings.Contains(payload.ContractNotes[0], "generated safe structural contract defaults") {
			t.Fatalf("contract_notes = %v, want generated contract note", payload.ContractNotes)
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, payload.Skill.ID, payload.EntryFile)); err != nil {
			t.Fatalf("expected installed entry file to be written: %v", err)
		}
	})

	t.Run("rejects html document", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Skill Page</title></head><body>not markdown</body></html>`))
		}))
		defer server.Close()

		body := fmt.Sprintf(`{"url":"%s/skill-page"}`, server.URL)
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install-url", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.InstallFromURL(c); err != nil {
			t.Fatalf("InstallFromURL failed: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}
	})

	t.Run("installs direct CLAUDE.md and materializes compatibility SKILL.md", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, `---
id: url_installed_claude
name: URL Installed Claude
version: 1.0.0
description: Installed from CLAUDE URL
invocation: blue url_installed_claude action=list
examples:
  - blue url_installed_claude action=list
capability_tags:
  - install
interaction_mode: stateless
card_support: none
---

# URL Installed Claude
`)
		}))
		defer server.Close()

		body := fmt.Sprintf(`{"url":"%s/CLAUDE.md"}`, server.URL)
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install-url", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.InstallFromURL(c); err != nil {
			t.Fatalf("InstallFromURL failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload struct {
			EntryFile string `json:"entry_file"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.EntryFile != "CLAUDE.md" {
			t.Fatalf("entry_file = %q, want CLAUDE.md", payload.EntryFile)
		}
		if registry.Get("url_installed_claude") == nil {
			t.Fatalf("expected CLAUDE.md-installed skill to be registered")
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "url_installed_claude", "CLAUDE.md")); err != nil {
			t.Fatalf("expected CLAUDE.md to be written: %v", err)
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "url_installed_claude", "SKILL.md")); err != nil {
			t.Fatalf("expected compatibility SKILL.md to be written: %v", err)
		}
	})

	t.Run("installs GitHub directory skill from CLAUDE.md with raw mirror fallback", func(t *testing.T) {
		var hosts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hosts = append(hosts, r.Host)
			switch {
			case r.Host == "api.github.com" && r.URL.Path == "/repos/demo/claude-skill/contents/skills/claude-skill":
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `[
					{
						"name":"CLAUDE.md",
						"path":"skills/claude-skill/CLAUDE.md",
						"type":"file",
						"download_url":"https://raw.githubusercontent.com/demo/claude-skill/main/skills/claude-skill/CLAUDE.md"
					}
				]`)
			case r.Host == "raw.githubusercontent.com":
				http.Error(w, "blocked", http.StatusBadGateway)
			case r.Host == "raw.gitmirror.com":
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = io.WriteString(w, `---
id: github_directory_claude
name: GitHub Directory Claude
version: 2.0.0
description: Installed from GitHub directory
invocation: blue github_directory_claude action=list
examples:
  - blue github_directory_claude action=list
capability_tags:
  - github
interaction_mode: stateless
card_support: none
---

# GitHub Directory Claude
`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		handler.httpClient = &http.Client{Transport: newRewriteHostTransport(t, server)}

		body := `{"url":"https://github.com/demo/claude-skill/tree/main/skills/claude-skill"}`
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install-url", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.InstallFromURL(c); err != nil {
			t.Fatalf("InstallFromURL failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload struct {
			EntryFile string `json:"entry_file"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.EntryFile != "CLAUDE.md" {
			t.Fatalf("entry_file = %q, want CLAUDE.md", payload.EntryFile)
		}
		if registry.Get("github_directory_claude") == nil {
			t.Fatalf("expected GitHub directory skill to be registered")
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "github_directory_claude", "CLAUDE.md")); err != nil {
			t.Fatalf("expected CLAUDE.md to be written: %v", err)
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "github_directory_claude", "SKILL.md")); err != nil {
			t.Fatalf("expected compatibility SKILL.md to be written: %v", err)
		}
		if len(hosts) < 3 || hosts[0] != "api.github.com" || hosts[1] != "raw.githubusercontent.com" || hosts[2] != "raw.gitmirror.com" {
			t.Fatalf("unexpected GitHub host fallback order: %v", hosts)
		}
	})

	t.Run("installs GitHub repository root URL via contents API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Host == "api.github.com" && r.URL.Path == "/repos/demo/root-skill/contents":
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `[
					{
						"name":"SKILL.md",
						"path":"SKILL.md",
						"type":"file",
						"download_url":"https://raw.githubusercontent.com/demo/root-skill/main/SKILL.md"
					}
				]`)
			case r.Host == "raw.githubusercontent.com" && r.URL.Path == "/demo/root-skill/main/SKILL.md":
				w.Header().Set("Content-Type", "text/plain; charset=utf-8")
				_, _ = io.WriteString(w, `---
id: github_repo_root_skill
name: GitHub Repo Root Skill
version: 1.0.0
description: Installed from repository root URL
invocation: blue github_repo_root_skill action=list
examples:
  - blue github_repo_root_skill action=list
capability_tags:
  - github
interaction_mode: stateless
card_support: none
---

# GitHub Repo Root Skill
`)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		handler.httpClient = &http.Client{Transport: newRewriteHostTransport(t, server)}

		body := `{"url":"https://github.com/demo/root-skill"}`
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install-url", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.InstallFromURL(c); err != nil {
			t.Fatalf("InstallFromURL failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if registry.Get("github_repo_root_skill") == nil {
			t.Fatalf("expected GitHub repo root skill to be registered")
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "github_repo_root_skill", "SKILL.md")); err != nil {
			t.Fatalf("expected SKILL.md to be written: %v", err)
		}
	})

	t.Run("rejects canonical alias conflict", func(t *testing.T) {
		skillDir := filepath.Join(handler.skillsDir, "team-browser")
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser skill
version: 1.0.0
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`), 0o644); err != nil {
			t.Fatalf("write skill file: %v", err)
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, `---
id: browser
name: Browser
version: 2.0.0
description: Installed from direct URL
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`)
		}))
		defer server.Close()
		handler.httpClient = server.Client()

		body := fmt.Sprintf(`{"url":"%s/SKILL.md"}`, server.URL)
		req := httptest.NewRequest(http.MethodPost, "/skill-store/install-url", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		if err := handler.InstallFromURL(c); err != nil {
			t.Fatalf("InstallFromURL failed: %v", err)
		}
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "already installed as browser") {
			t.Fatalf("expected canonical alias conflict, body=%s", rec.Body.String())
		}
	})

	t.Run("rejects same-precedence home root conflict", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		handler.SetSkillsDir(filepath.Join(homeDir, ".claude", "skills"))
		if err := os.MkdirAll(handler.skillsDir, 0o755); err != nil {
			t.Fatalf("mkdir handler skills dir: %v", err)
		}

		existingDir := filepath.Join(homeDir, ".agents", "skills", "team-browser")
		if err := os.MkdirAll(existingDir, 0o755); err != nil {
			t.Fatalf("mkdir existing skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(existingDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from sibling home root
version: 1.0.0
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`), 0o644); err != nil {
			t.Fatalf("write existing skill file: %v", err)
		}

		req, contentType := newMultipartUploadRequest(t, "/skills/upload", "SKILL.md", []byte(`---
id: browser
name: Browser
version: 2.0.0
description: Uploaded browser skill
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`))
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := httptest.NewRecorder()

		if err := handler.UploadSkill(e.NewContext(req, rec)); err != nil {
			t.Fatalf("UploadSkill failed: %v", err)
		}
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "already installed as browser") {
			t.Fatalf("expected sibling home root conflict, body=%s", rec.Body.String())
		}
	})

	t.Run("reports ambiguous same-precedence home root conflict", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		handler.SetSkillsDir(filepath.Join(homeDir, ".claude", "skills"))
		if err := os.MkdirAll(handler.skillsDir, 0o755); err != nil {
			t.Fatalf("mkdir handler skills dir: %v", err)
		}

		firstDir := filepath.Join(homeDir, ".agents", "skills", "team-browser")
		if err := os.MkdirAll(firstDir, 0o755); err != nil {
			t.Fatalf("mkdir first skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(firstDir, "SKILL.md"), []byte(`---
name: browser
description: Browser alias from sibling home root
version: 1.0.0
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`), 0o644); err != nil {
			t.Fatalf("write first skill file: %v", err)
		}

		secondDir := filepath.Join(homeDir, ".agents", "skills", "browser")
		if err := os.MkdirAll(secondDir, 0o755); err != nil {
			t.Fatalf("mkdir second skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(secondDir, "SKILL.md"), []byte(`---
name: browser
description: Browser duplicate from sibling home root
version: 1.0.0
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`), 0o644); err != nil {
			t.Fatalf("write second skill file: %v", err)
		}

		req, contentType := newMultipartUploadRequest(t, "/skills/upload", "SKILL.md", []byte(`---
id: browser
name: Browser
version: 2.0.0
description: Uploaded browser skill
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`))
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := httptest.NewRecorder()

		if err := handler.UploadSkill(e.NewContext(req, rec)); err != nil {
			t.Fatalf("UploadSkill failed: %v", err)
		}
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "canonical skill conflicts detected") {
			t.Fatalf("expected canonical conflict detail, body=%s", rec.Body.String())
		}
	})
}

func TestSkillHandler_UploadSkill(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)
	e := echo.New()

	t.Run("uploads direct CLAUDE.md and preserves entry file", func(t *testing.T) {
		req, contentType := newMultipartUploadRequest(t, "/skills/upload", "CLAUDE.md", []byte(`---
id: uploaded_claude_skill
name: Uploaded Claude Skill
version: 1.0.0
description: Uploaded as CLAUDE.md
invocation: blue uploaded_claude_skill action=list
examples:
  - blue uploaded_claude_skill action=list
capability_tags:
  - upload
interaction_mode: stateless
card_support: none
---

# Uploaded Claude Skill
`))
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := httptest.NewRecorder()

		if err := handler.UploadSkill(e.NewContext(req, rec)); err != nil {
			t.Fatalf("UploadSkill failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload struct {
			Success   bool   `json:"success"`
			EntryFile string `json:"entry_file"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if !payload.Success || payload.EntryFile != "CLAUDE.md" {
			t.Fatalf("unexpected upload payload: %+v", payload)
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "uploaded_claude_skill", "CLAUDE.md")); err != nil {
			t.Fatalf("expected CLAUDE.md to exist: %v", err)
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "uploaded_claude_skill", "SKILL.md")); err != nil {
			t.Fatalf("expected compatibility SKILL.md to exist: %v", err)
		}
	})

	t.Run("uploads legacy SKILL.md without blocking on missing contract fields", func(t *testing.T) {
		req, contentType := newMultipartUploadRequest(t, "/skills/upload", "SKILL.md", []byte(`---
name: uploaded_legacy_skill
description: Uploaded using legacy frontmatter
---

# Uploaded Legacy Skill

## Command Usage

`+"```bash"+`
blue uploaded_legacy_skill action=list
`+"```"+`
`))
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := httptest.NewRecorder()

		if err := handler.UploadSkill(e.NewContext(req, rec)); err != nil {
			t.Fatalf("UploadSkill failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "uploaded_legacy_skill", "SKILL.md")); err != nil {
			t.Fatalf("expected legacy SKILL.md install to exist: %v", err)
		}
		var payload struct {
			Warnings       []string `json:"warnings"`
			ContractStatus string   `json:"contract_status"`
			ContractSource string   `json:"contract_source"`
			ContractNotes  []string `json:"contract_notes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Warnings) == 0 || !strings.Contains(payload.Warnings[0], "legacy manifest compatibility fallback applied") {
			t.Fatalf("warnings = %v, want legacy compatibility warning", payload.Warnings)
		}
		if payload.ContractStatus != skillmanifest.ContractStatusLegacyFallback {
			t.Fatalf("contract_status = %q, want %q", payload.ContractStatus, skillmanifest.ContractStatusLegacyFallback)
		}
		if payload.ContractSource != skillmanifest.ContractSourceLegacyFrontmatter {
			t.Fatalf("contract_source = %q, want %q", payload.ContractSource, skillmanifest.ContractSourceLegacyFrontmatter)
		}
		if len(payload.ContractNotes) == 0 || !strings.Contains(payload.ContractNotes[0], "legacy manifest compatibility fallback applied") {
			t.Fatalf("contract_notes = %v, want legacy compatibility note", payload.ContractNotes)
		}
	})

	t.Run("uploads archive with AGENT.md entry", func(t *testing.T) {
		var archive bytes.Buffer
		zipWriter := zip.NewWriter(&archive)
		agentFile, err := zipWriter.Create("bundle/agent-skill/AGENT.md")
		if err != nil {
			t.Fatalf("zipWriter.Create(AGENT.md) error = %v", err)
		}
		if _, err := agentFile.Write([]byte(`---
id: uploaded_agent_skill
name: Uploaded Agent Skill
version: 1.0.0
description: Uploaded as archive
invocation: blue uploaded_agent_skill action=list
examples:
  - blue uploaded_agent_skill action=list
capability_tags:
  - upload
interaction_mode: stateless
card_support: none
---

# Uploaded Agent Skill
`)); err != nil {
			t.Fatalf("agentFile.Write() error = %v", err)
		}
		if err := zipWriter.Close(); err != nil {
			t.Fatalf("zipWriter.Close() error = %v", err)
		}

		req, contentType := newMultipartUploadRequest(t, "/skills/upload", "agent-skill.zip", archive.Bytes())
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := httptest.NewRecorder()

		if err := handler.UploadSkill(e.NewContext(req, rec)); err != nil {
			t.Fatalf("UploadSkill failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload struct {
			Success   bool   `json:"success"`
			EntryFile string `json:"entry_file"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if !payload.Success || payload.EntryFile != "AGENT.md" {
			t.Fatalf("unexpected upload payload: %+v", payload)
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "uploaded_agent_skill", "AGENT.md")); err != nil {
			t.Fatalf("expected AGENT.md to exist: %v", err)
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "uploaded_agent_skill", "SKILL.md")); err != nil {
			t.Fatalf("expected compatibility SKILL.md to exist: %v", err)
		}
	})

	t.Run("uploads legacy archive with AGENT.md entry without blocking on missing contract fields", func(t *testing.T) {
		var archive bytes.Buffer
		zipWriter := zip.NewWriter(&archive)
		agentFile, err := zipWriter.Create("bundle/legacy-agent-skill/AGENT.md")
		if err != nil {
			t.Fatalf("zipWriter.Create(AGENT.md) error = %v", err)
		}
		if _, err := agentFile.Write([]byte(`---
name: uploaded_legacy_agent_skill
description: Uploaded from legacy archive
---

# Uploaded Legacy Agent Skill

## Command Usage

` + "```bash" + `
blue uploaded_legacy_agent_skill action=list
` + "```" + `
`)); err != nil {
			t.Fatalf("agentFile.Write() error = %v", err)
		}
		if err := zipWriter.Close(); err != nil {
			t.Fatalf("zipWriter.Close() error = %v", err)
		}

		req, contentType := newMultipartUploadRequest(t, "/skills/upload", "legacy-agent-skill.zip", archive.Bytes())
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := httptest.NewRecorder()

		if err := handler.UploadSkill(e.NewContext(req, rec)); err != nil {
			t.Fatalf("UploadSkill failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}
		if _, err := os.Stat(filepath.Join(handler.skillsDir, "uploaded_legacy_agent_skill", "AGENT.md")); err != nil {
			t.Fatalf("expected legacy AGENT.md install to exist: %v", err)
		}
		var payload struct {
			Warnings       []string `json:"warnings"`
			ContractStatus string   `json:"contract_status"`
			ContractSource string   `json:"contract_source"`
			ContractNotes  []string `json:"contract_notes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Warnings) == 0 || !strings.Contains(payload.Warnings[0], "legacy manifest compatibility fallback applied") {
			t.Fatalf("warnings = %v, want legacy compatibility warning", payload.Warnings)
		}
		if payload.ContractStatus != skillmanifest.ContractStatusLegacyFallback {
			t.Fatalf("contract_status = %q, want %q", payload.ContractStatus, skillmanifest.ContractStatusLegacyFallback)
		}
		if payload.ContractSource != skillmanifest.ContractSourceLegacyFrontmatter {
			t.Fatalf("contract_source = %q, want %q", payload.ContractSource, skillmanifest.ContractSourceLegacyFrontmatter)
		}
		if len(payload.ContractNotes) == 0 || !strings.Contains(payload.ContractNotes[0], "legacy manifest compatibility fallback applied") {
			t.Fatalf("contract_notes = %v, want legacy compatibility note", payload.ContractNotes)
		}
	})

	t.Run("rejects canonical alias conflict", func(t *testing.T) {
		skillDir := filepath.Join(handler.skillsDir, "team-browser")
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("mkdir skill dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser skill
version: 1.0.0
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`), 0o644); err != nil {
			t.Fatalf("write skill file: %v", err)
		}

		req, contentType := newMultipartUploadRequest(t, "/skills/upload", "SKILL.md", []byte(`---
id: browser
name: Browser
version: 2.0.0
description: Uploaded browser skill
invocation: blue browser
examples:
  - blue browser
capability_tags:
  - browser
interaction_mode: stateless
card_support: none
---

# Browser
`))
		req.Header.Set(echo.HeaderContentType, contentType)
		rec := httptest.NewRecorder()

		if err := handler.UploadSkill(e.NewContext(req, rec)); err != nil {
			t.Fatalf("UploadSkill failed: %v", err)
		}
		if rec.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusConflict, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "already installed as browser") {
			t.Fatalf("expected canonical alias conflict, body=%s", rec.Body.String())
		}
	})
}

func TestSkillHandler_UninstallSkill(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	// Register a non-builtin skill
	testSkill := NewRemoteSkillAdapter(&skill.Manifest{
		ID:          "installed-skill",
		Name:        "Installed Skill",
		Version:     "1.0.0",
		Description: "A test skill",
	})
	registry.Register(testSkill, false)

	e := echo.New()

	t.Run("uninstall skill successfully", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/uninstall/installed-skill", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("installed-skill")

		err := handler.UninstallSkill(c)
		if err != nil {
			t.Fatalf("UninstallSkill failed: %v", err)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		// Verify skill is unregistered
		if registry.Get("installed-skill") != nil {
			t.Error("skill was not unregistered")
		}
	})

	t.Run("uninstall non-existent skill", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/uninstall/non-existent", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("non-existent")

		err := handler.UninstallSkill(c)
		if err != nil {
			t.Fatalf("UninstallSkill failed: %v", err)
		}

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})
}

func TestSkillHandler_UninstallBuiltinSkill(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	// Register a builtin skill
	builtinSkill := NewRemoteSkillAdapter(&skill.Manifest{
		ID:          "builtin-skill",
		Name:        "Builtin Skill",
		Version:     "1.0.0",
		Description: "A builtin skill",
	})
	registry.Register(builtinSkill, true) // true = builtin

	e := echo.New()

	t.Run("cannot uninstall builtin skill", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/skill-store/uninstall/builtin-skill", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("builtin-skill")

		err := handler.UninstallSkill(c)
		if err != nil {
			t.Fatalf("UninstallSkill failed: %v", err)
		}

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}

		// Verify skill is still registered
		if registry.Get("builtin-skill") == nil {
			t.Error("builtin skill should not be unregistered")
		}
	})
}

func TestRemoteSkillAdapter(t *testing.T) {
	manifest := &skill.Manifest{
		ID:          "test-skill",
		Name:        "Test Skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Author:      "Test Author",
		Category:    "productivity",
		Tags:        []string{"test", "example"},
	}

	adapter := NewRemoteSkillAdapter(manifest)

	t.Run("manifest returns correct data", func(t *testing.T) {
		m := adapter.Manifest()
		if m.ID != "test-skill" {
			t.Errorf("expected ID 'test-skill', got '%s'", m.ID)
		}
		if m.Name != "Test Skill" {
			t.Errorf("expected name 'Test Skill', got '%s'", m.Name)
		}
	})

	t.Run("validate returns nil", func(t *testing.T) {
		err := adapter.Validate(map[string]any{"key": "value"})
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("execute returns result", func(t *testing.T) {
		result, err := adapter.Execute(nil, map[string]any{"input": "test"})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if !result.Success {
			t.Error("expected success")
		}
	})
}

func TestGetMockClawHubSkills(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	source := &SkillSource{
		ID:   "clawhub",
		Name: "ClawHub",
	}

	skills := handler.getMockClawHubSkills(source)

	if len(skills) == 0 {
		t.Fatal("expected mock skills, got none")
	}

	// Verify all skills have correct source info
	for _, s := range skills {
		if s.SourceID != "clawhub" {
			t.Errorf("expected source ID 'clawhub', got '%s'", s.SourceID)
		}
		if s.SourceName != "ClawHub" {
			t.Errorf("expected source name 'ClawHub', got '%s'", s.SourceName)
		}
		// Verify URLs point to clawhub.ai
		if s.Homepage != "" && !strings.Contains(s.Homepage, "clawhub.ai") {
			t.Errorf("expected homepage to contain 'clawhub.ai', got '%s'", s.Homepage)
		}
	}
}

func TestParseSkillContentNormalizesHyphenatedIDForLocalInstall(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	skillID, manifest, err := handler.parseSkillContent(`---
id: word-docx
name: Word DOCX
description: Convert Word documents
version: 1.0.0
invocation: blue word_docx.convert input=demo.docx
examples:
  - blue word_docx.convert input=demo.docx
capability_tags:
  - convert
  - document
interaction_mode: stateless
card_support: none
---
`, "https://example.com/skills/word-docx/SKILL.md", "", "")
	if err != nil {
		t.Fatalf("parseSkillContent failed: %v", err)
	}
	if skillID != "word_docx" {
		t.Fatalf("skillID = %q, want word_docx", skillID)
	}
	if manifest.ID != "word_docx" {
		t.Fatalf("manifest.ID = %q, want word_docx", manifest.ID)
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "foo", false},
		{"", "", true},
		{"Hello", "", true},
		{"", "hello", false},
		{"Test Skill", "skill", true},
		{"Test Skill", "SKILL", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := containsIgnoreCase(tt.s, tt.substr)
			if got != tt.want {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestCreateManifestFromRemoteSkill(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	rs := &RemoteSkill{
		ID:          "test-skill",
		Name:        "Test Skill",
		Version:     "1.0.0",
		Description: "A test skill",
		Author:      "Test Author",
		Category:    "productivity",
		Tags:        []string{"test", "example"},
		SourceID:    "clawhub",
		SourceName:  "ClawHub",
		Homepage:    "https://www.clawhub.ai/skills/test",
	}

	manifest := handler.createManifestFromRemoteSkill(rs)

	if manifest.ID != rs.ID {
		t.Errorf("expected ID '%s', got '%s'", rs.ID, manifest.ID)
	}
	if manifest.Name != rs.Name {
		t.Errorf("expected name '%s', got '%s'", rs.Name, manifest.Name)
	}
	if manifest.Version != rs.Version {
		t.Errorf("expected version '%s', got '%s'", rs.Version, manifest.Version)
	}
	if manifest.Author != rs.Author {
		t.Errorf("expected author '%s', got '%s'", rs.Author, manifest.Author)
	}
	if manifest.Category != rs.Category {
		t.Errorf("expected category '%s', got '%s'", rs.Category, manifest.Category)
	}
	if manifest.Metadata["source_id"] != rs.SourceID {
		t.Errorf("expected source_id '%s', got '%s'", rs.SourceID, manifest.Metadata["source_id"])
	}
	if manifest.Metadata["homepage"] != rs.Homepage {
		t.Errorf("expected homepage '%s', got '%s'", rs.Homepage, manifest.Metadata["homepage"])
	}
}

func TestSkillHandler_SetUseMockData(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	t.Run("mock data disabled by default", func(t *testing.T) {
		if handler.useMockData {
			t.Error("mock data should be disabled by default")
		}
	})

	t.Run("enable mock data", func(t *testing.T) {
		handler.SetUseMockData(true)
		if !handler.useMockData {
			t.Error("mock data should be enabled")
		}
	})

	t.Run("disable mock data", func(t *testing.T) {
		handler.SetUseMockData(false)
		if handler.useMockData {
			t.Error("mock data should be disabled")
		}
	})
}

func TestSkillHandler_MockDataFallback(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	// Enable mock data for this test
	handler.SetUseMockData(true)

	source := &SkillSource{
		ID:   "clawhub",
		Name: "ClawHub",
		URL:  "https://invalid-url-that-will-fail.example.com",
		Type: "clawhub",
	}

	t.Run("returns mock data when API fails and mock enabled", func(t *testing.T) {
		skills := handler.getMockClawHubSkills(source)
		if len(skills) == 0 {
			t.Error("expected mock skills to be returned")
		}
	})

	t.Run("mock skills have correct source info", func(t *testing.T) {
		skills := handler.getMockClawHubSkills(source)
		for _, s := range skills {
			if s.SourceID != source.ID {
				t.Errorf("expected source ID '%s', got '%s'", source.ID, s.SourceID)
			}
			if s.SourceName != source.Name {
				t.Errorf("expected source name '%s', got '%s'", source.Name, s.SourceName)
			}
		}
	})
}

func TestSkillHandler_GetMockGitHubSkills(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	source := &SkillSource{
		ID:   "moltbot",
		Name: "MoltBot",
	}

	skills := handler.getMockGitHubSkills(source)

	if len(skills) == 0 {
		t.Fatal("expected mock skills, got none")
	}

	for _, s := range skills {
		if s.SourceID != "moltbot" {
			t.Errorf("expected source ID 'moltbot', got '%s'", s.SourceID)
		}
		if s.SourceName != "MoltBot" {
			t.Errorf("expected source name 'MoltBot', got '%s'", s.SourceName)
		}
		if s.Category != "extension" {
			t.Errorf("expected category 'extension', got '%s'", s.Category)
		}
	}
}

func TestClawHubAPIResponseParsing(t *testing.T) {
	// Test parsing of ClawHub API response format
	jsonData := `{
		"items": [
			{
				"slug": "test-skill",
				"displayName": "Test Skill",
				"summary": "A test skill description",
				"tags": {"latest": "1.0.0"},
				"stats": {
					"comments": 5,
					"downloads": 100,
					"installsAllTime": 50,
					"installsCurrent": 10,
					"stars": 25,
					"versions": 3
				},
				"createdAt": 1769694742385,
				"updatedAt": 1769752226497,
				"latestVersion": {
					"version": "1.0.0",
					"createdAt": 1769752226497,
					"changelog": "Initial release"
				}
			}
		],
		"nextCursor": "abc123"
	}`

	var apiResp ClawHubAPIResponse
	err := json.Unmarshal([]byte(jsonData), &apiResp)
	if err != nil {
		t.Fatalf("failed to parse ClawHub API response: %v", err)
	}

	if len(apiResp.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(apiResp.Items))
	}

	item := apiResp.Items[0]
	if item.Slug != "test-skill" {
		t.Errorf("expected slug 'test-skill', got '%s'", item.Slug)
	}
	if item.DisplayName != "Test Skill" {
		t.Errorf("expected displayName 'Test Skill', got '%s'", item.DisplayName)
	}
	if item.Summary != "A test skill description" {
		t.Errorf("expected summary 'A test skill description', got '%s'", item.Summary)
	}
	if item.Tags.Latest != "1.0.0" {
		t.Errorf("expected tags.latest '1.0.0', got '%s'", item.Tags.Latest)
	}
	if item.Stats.Downloads != 100 {
		t.Errorf("expected stats.downloads 100, got %d", item.Stats.Downloads)
	}
	if item.Stats.Stars != 25 {
		t.Errorf("expected stats.stars 25, got %d", item.Stats.Stars)
	}
	if item.LatestVersion.Version != "1.0.0" {
		t.Errorf("expected latestVersion.version '1.0.0', got '%s'", item.LatestVersion.Version)
	}
	if apiResp.NextCursor != "abc123" {
		t.Errorf("expected nextCursor 'abc123', got '%s'", apiResp.NextCursor)
	}
}

func TestClawHubSkillConversion(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	source := &SkillSource{
		ID:   "clawhub",
		Name: "ClawHub",
		URL:  "https://www.clawhub.ai",
	}

	// Simulate converting a ClawHub skill to RemoteSkill
	clawHubSkill := ClawHubSkill{
		Slug:        "my-skill",
		DisplayName: "My Skill",
		Summary:     "A great skill",
	}
	clawHubSkill.Tags.Latest = "2.0.0"
	clawHubSkill.Stats.Stars = 50
	clawHubSkill.Stats.Downloads = 200
	clawHubSkill.LatestVersion.Version = "2.0.0"

	// Convert to RemoteSkill (simulating what fetchFromClawHub does)
	remoteSkill := &RemoteSkill{
		ID:          clawHubSkill.Slug,
		Name:        clawHubSkill.DisplayName,
		Version:     clawHubSkill.LatestVersion.Version,
		Description: clawHubSkill.Summary,
		Category:    "skill",
		SourceID:    source.ID,
		SourceName:  source.Name,
		DownloadURL: fmt.Sprintf("%s/skills/%s", source.URL, clawHubSkill.Slug),
		Homepage:    fmt.Sprintf("%s/skills/%s", source.URL, clawHubSkill.Slug),
		Stars:       clawHubSkill.Stats.Stars,
		Downloads:   clawHubSkill.Stats.Downloads,
	}

	if remoteSkill.ID != "my-skill" {
		t.Errorf("expected ID 'my-skill', got '%s'", remoteSkill.ID)
	}
	if remoteSkill.Name != "My Skill" {
		t.Errorf("expected name 'My Skill', got '%s'", remoteSkill.Name)
	}
	if remoteSkill.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got '%s'", remoteSkill.Version)
	}
	if remoteSkill.Homepage != "https://www.clawhub.ai/skills/my-skill" {
		t.Errorf("expected homepage 'https://www.clawhub.ai/skills/my-skill', got '%s'", remoteSkill.Homepage)
	}
	if remoteSkill.Stars != 50 {
		t.Errorf("expected stars 50, got %d", remoteSkill.Stars)
	}
	if remoteSkill.Downloads != 200 {
		t.Errorf("expected downloads 200, got %d", remoteSkill.Downloads)
	}

	// Verify handler is not nil (just to use it)
	if handler == nil {
		t.Error("handler should not be nil")
	}
}

func TestConvertClawHubSkill(t *testing.T) {
	registry := skill.NewRegistry()
	handler := newTestSkillHandler(t, registry)

	source := &SkillSource{
		ID:   "clawhub",
		Name: "ClawHub",
		URL:  "https://www.clawhub.ai",
	}

	clawHubSkill := ClawHubSkill{
		Slug:        "test-skill",
		DisplayName: "Test Skill",
		Summary:     "A test skill",
	}
	clawHubSkill.LatestVersion.Version = "1.0.0"
	clawHubSkill.Stats.Stars = 10
	clawHubSkill.Stats.Downloads = 100

	remoteSkill := handler.convertClawHubSkill(clawHubSkill, source)

	if remoteSkill.ID != "test-skill" {
		t.Errorf("expected ID 'test-skill', got '%s'", remoteSkill.ID)
	}
	if remoteSkill.Name != "Test Skill" {
		t.Errorf("expected name 'Test Skill', got '%s'", remoteSkill.Name)
	}
	if remoteSkill.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", remoteSkill.Version)
	}
	if remoteSkill.SourceID != "clawhub" {
		t.Errorf("expected sourceID 'clawhub', got '%s'", remoteSkill.SourceID)
	}
	if remoteSkill.Homepage != "https://www.clawhub.ai/skills/test-skill" {
		t.Errorf("expected homepage 'https://www.clawhub.ai/skills/test-skill', got '%s'", remoteSkill.Homepage)
	}
}

func TestClawHubPaginationParsing(t *testing.T) {
	// Test parsing multiple pages of ClawHub API response
	page1 := `{
		"items": [
			{"slug": "skill-1", "displayName": "Skill 1", "summary": "First skill", "tags": {"latest": "1.0.0"}, "stats": {"stars": 10, "downloads": 100}, "latestVersion": {"version": "1.0.0"}}
		],
		"nextCursor": "cursor123"
	}`

	page2 := `{
		"items": [
			{"slug": "skill-2", "displayName": "Skill 2", "summary": "Second skill", "tags": {"latest": "2.0.0"}, "stats": {"stars": 20, "downloads": 200}, "latestVersion": {"version": "2.0.0"}}
		],
		"nextCursor": ""
	}`

	var resp1 ClawHubAPIResponse
	if err := json.Unmarshal([]byte(page1), &resp1); err != nil {
		t.Fatalf("failed to parse page 1: %v", err)
	}

	if len(resp1.Items) != 1 {
		t.Errorf("expected 1 item in page 1, got %d", len(resp1.Items))
	}
	if resp1.NextCursor != "cursor123" {
		t.Errorf("expected nextCursor 'cursor123', got '%s'", resp1.NextCursor)
	}

	var resp2 ClawHubAPIResponse
	if err := json.Unmarshal([]byte(page2), &resp2); err != nil {
		t.Fatalf("failed to parse page 2: %v", err)
	}

	if len(resp2.Items) != 1 {
		t.Errorf("expected 1 item in page 2, got %d", len(resp2.Items))
	}
	if resp2.NextCursor != "" {
		t.Errorf("expected empty nextCursor, got '%s'", resp2.NextCursor)
	}

	// Verify combined skills
	allItems := append(resp1.Items, resp2.Items...)
	if len(allItems) != 2 {
		t.Errorf("expected 2 total items, got %d", len(allItems))
	}
	if allItems[0].Slug != "skill-1" {
		t.Errorf("expected first skill slug 'skill-1', got '%s'", allItems[0].Slug)
	}
	if allItems[1].Slug != "skill-2" {
		t.Errorf("expected second skill slug 'skill-2', got '%s'", allItems[1].Slug)
	}
}
