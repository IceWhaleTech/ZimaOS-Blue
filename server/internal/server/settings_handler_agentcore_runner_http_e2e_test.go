package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/optimization"
)

func TestSettingsHandlerAgentcoreRunnerRoutesHTTPE2E(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	manager := &stubAgentcoreRunnerManager{
		status: AgentcoreRunnerStatus{
			BinaryReady: true,
			BinaryPath:  "/tmp/agentcore-runner",
		},
	}
	handler.SetAgentcoreRunnerManager(manager)

	e := echo.New()
	api := e.Group("/api/v1")
	handler.RegisterRoutes(api)
	srv := httptest.NewServer(e)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/settings", strings.NewReader(`{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`))
	if err != nil {
		t.Fatalf("new patch request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("PATCH /settings: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH /settings status=%d", resp.StatusCode)
	}

	statusResp, err := srv.Client().Get(srv.URL + "/api/v1/settings/agentcore-runner/status")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status code=%d", statusResp.StatusCode)
	}
	var status AgentcoreRunnerStatus
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.Enabled || status.RepoURL != "https://github.com/IceWhaleTech/ZimaOS-Blue" || status.ResolvedRef != "main" {
		t.Fatalf("unexpected status %#v", status)
	}

	prepareResp, err := srv.Client().Post(srv.URL+"/api/v1/settings/agentcore-runner/prepare", "application/json", nil)
	if err != nil {
		t.Fatalf("POST prepare: %v", err)
	}
	defer prepareResp.Body.Close()
	if prepareResp.StatusCode != http.StatusOK {
		t.Fatalf("POST prepare code=%d", prepareResp.StatusCode)
	}
	if manager.prepareCalls != 1 {
		t.Fatalf("prepareCalls=%d, want 1", manager.prepareCalls)
	}
	if manager.lastPrepareRepo != "https://github.com/IceWhaleTech/ZimaOS-Blue" || manager.lastPrepareRef != "main" {
		t.Fatalf("prepare request repo=%q ref=%q", manager.lastPrepareRepo, manager.lastPrepareRef)
	}
}

func TestSettingsHandlerAgentcoreRunnerTagsRouteHTTPE2E(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.agentcoreRunnerTagResolver = func(_ context.Context, repoURL string) ([]string, error) {
		if repoURL != "owner/repo" {
			t.Fatalf("repoURL=%q, want owner/repo", repoURL)
		}
		return []string{"v2.0.0", "v1.9.0"}, nil
	}

	e := echo.New()
	api := e.Group("/api/v1")
	handler.RegisterRoutes(api)
	srv := httptest.NewServer(e)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/settings/agentcore-runner/tags?repo_url=owner/repo")
	if err != nil {
		t.Fatalf("GET tags: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET tags code=%d", resp.StatusCode)
	}

	var body AgentcoreRunnerTagList
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode tags: %v", err)
	}
	if body.DefaultRef != "main" {
		t.Fatalf("default_ref=%q", body.DefaultRef)
	}
	if strings.Join(body.Tags, ",") != "v2.0.0,v1.9.0" {
		t.Fatalf("tags=%#v", body.Tags)
	}
}

func TestSettingsHandlerAgentcoreRunnerStatusRouteUsesRealManagerOptimizationSummary(t *testing.T) {
	root := t.TempDir()
	layout := optimization.NewManagedLayout(root)
	writeAgentcoreRunnerHTTPStatusFixture(t, layout.StatusPath, optimization.Status{
		BinaryReady:           true,
		BinaryPath:            "/tmp/agentcore-runner",
		LastOptimizationRunID: "opt-789",
	})
	writeAgentcoreRunnerHTTPRunFixture(t, filepath.Join(layout.RootDir, "optimization-runs", "opt-789.json"), map[string]interface{}{
		"id":                   "opt-789",
		"created_at":           "2026-04-03T16:17:18Z",
		"runner_stop_reason":   "completed",
		"runner_response_text": "execution gate improved after managed runner check",
	})
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}

	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetAgentcoreRunnerManager(manager)

	e := echo.New()
	api := e.Group("/api/v1")
	handler.RegisterRoutes(api)
	srv := httptest.NewServer(e)
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/api/v1/settings", strings.NewReader(`{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`))
	if err != nil {
		t.Fatalf("new patch request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("PATCH /settings: %v", err)
	}
	_ = resp.Body.Close()

	statusResp, err := srv.Client().Get(srv.URL + "/api/v1/settings/agentcore-runner/status")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status code=%d", statusResp.StatusCode)
	}
	var status AgentcoreRunnerStatus
	if err := json.NewDecoder(statusResp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.LastOptimizationState != "completed" {
		t.Fatalf("LastOptimizationState=%q, want completed", status.LastOptimizationState)
	}
	wantAt := time.Date(2026, 4, 3, 16, 17, 18, 0, time.UTC)
	if status.LastOptimizationAt == nil || !status.LastOptimizationAt.Equal(wantAt) {
		t.Fatalf("LastOptimizationAt=%v, want %s", status.LastOptimizationAt, wantAt.Format(time.RFC3339))
	}
	if !strings.Contains(status.LastOptimizationSummary, "execution gate improved") {
		t.Fatalf("LastOptimizationSummary=%q", status.LastOptimizationSummary)
	}
}

func TestSettingsHandlerAgentcoreRunnerLastRunRouteUsesRealManagerArtifact(t *testing.T) {
	root := t.TempDir()
	layout := optimization.NewManagedLayout(root)
	writeAgentcoreRunnerHTTPStatusFixture(t, layout.StatusPath, optimization.Status{
		LastOptimizationRunID: "opt-last",
	})
	writeAgentcoreRunnerHTTPRunFixture(t, filepath.Join(layout.RootDir, "optimization-runs", "opt-last.json"), map[string]interface{}{
		"id":                   "opt-last",
		"created_at":           "2026-04-03T18:19:20Z",
		"runner_response_text": "agentcore-runner received optimization evidence",
		"runner_transcript": []map[string]interface{}{
			{
				"direction": "in",
				"method":    "session/update",
				"text":      "agentcore-runner received optimization evidence",
			},
		},
	})
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}

	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetAgentcoreRunnerManager(manager)
	e := echo.New()
	api := e.Group("/api/v1")
	handler.RegisterRoutes(api)
	srv := httptest.NewServer(e)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/settings/agentcore-runner/last-run")
	if err != nil {
		t.Fatalf("GET last-run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET last-run code=%d", resp.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode last-run: %v", err)
	}
	if got := strings.TrimSpace(body["id"].(string)); got != "opt-last" {
		t.Fatalf("id=%q, want opt-last", got)
	}
	if got := strings.TrimSpace(body["runner_response_text"].(string)); !strings.Contains(got, "optimization evidence") {
		t.Fatalf("runner_response_text=%q", got)
	}
	transcript, ok := body["runner_transcript"].([]interface{})
	if !ok || len(transcript) != 1 {
		t.Fatalf("runner_transcript=%#v", body["runner_transcript"])
	}
}

func TestSettingsHandlerAgentcoreRunnerLastRunRoutePreservesStructuredRunnerEvidence(t *testing.T) {
	root := t.TempDir()
	layout := optimization.NewManagedLayout(root)
	writeAgentcoreRunnerHTTPStatusFixture(t, layout.StatusPath, optimization.Status{
		LastOptimizationRunID: "opt-evidence",
	})
	writeAgentcoreRunnerHTTPRunFixture(t, filepath.Join(layout.RootDir, "optimization-runs", "opt-evidence.json"), map[string]interface{}{
		"id":                   "opt-evidence",
		"reason":               "execution_gate_failed",
		"candidate_id":         "candidate-456",
		"eval_run_id":          "eval-456",
		"optimization_surface": "runner_code",
		"runner_protocol":      "acp",
		"runner_session_id":    "session-123",
		"runner_stop_reason":   "completed",
		"runner_duration_ms":   612,
		"runner_response_text": "execution gate improved after managed runner check",
		"runner_error":         "",
		"runner_stderr":        "runner stderr line",
		"runner_transcript": []map[string]interface{}{
			{
				"direction": "out",
				"method":    "session/prompt",
				"text":      "Optimization trigger received.",
			},
			{
				"direction": "out",
				"method":    "session/context",
				"text":      "Constraint diff prepared.",
			},
		},
	})
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}

	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetAgentcoreRunnerManager(manager)
	e := echo.New()
	api := e.Group("/api/v1")
	handler.RegisterRoutes(api)
	srv := httptest.NewServer(e)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/settings/agentcore-runner/last-run")
	if err != nil {
		t.Fatalf("GET last-run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET last-run code=%d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode last-run: %v", err)
	}
	if got := strings.TrimSpace(body["reason"].(string)); got != "execution_gate_failed" {
		t.Fatalf("reason=%q", got)
	}
	if got := strings.TrimSpace(body["candidate_id"].(string)); got != "candidate-456" {
		t.Fatalf("candidate_id=%q", got)
	}
	if got := strings.TrimSpace(body["eval_run_id"].(string)); got != "eval-456" {
		t.Fatalf("eval_run_id=%q", got)
	}
	if got := strings.TrimSpace(body["optimization_surface"].(string)); got != "runner_code" {
		t.Fatalf("optimization_surface=%q", got)
	}
	if got := strings.TrimSpace(body["runner_protocol"].(string)); got != "acp" {
		t.Fatalf("runner_protocol=%q", got)
	}
	if got := strings.TrimSpace(body["runner_session_id"].(string)); got != "session-123" {
		t.Fatalf("runner_session_id=%q", got)
	}
	if got := strings.TrimSpace(body["runner_stop_reason"].(string)); got != "completed" {
		t.Fatalf("runner_stop_reason=%q", got)
	}
	if got, ok := body["runner_duration_ms"].(float64); !ok || got != 612 {
		t.Fatalf("runner_duration_ms=%#v", body["runner_duration_ms"])
	}
	if got := strings.TrimSpace(body["runner_response_text"].(string)); !strings.Contains(got, "execution gate improved") {
		t.Fatalf("runner_response_text=%q", got)
	}
	if got := strings.TrimSpace(body["runner_stderr"].(string)); got != "runner stderr line" {
		t.Fatalf("runner_stderr=%q", got)
	}
	transcript, ok := body["runner_transcript"].([]interface{})
	if !ok || len(transcript) != 2 {
		t.Fatalf("runner_transcript=%#v", body["runner_transcript"])
	}
	firstEntry, ok := transcript[0].(map[string]interface{})
	if !ok {
		t.Fatalf("runner_transcript[0]=%#v", transcript[0])
	}
	if got := strings.TrimSpace(firstEntry["method"].(string)); got != "session/prompt" {
		t.Fatalf("runner_transcript[0].method=%q", got)
	}
}

func writeAgentcoreRunnerHTTPStatusFixture(t *testing.T, path string, status optimization.Status) {
	t.Helper()
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir status dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
}

func writeAgentcoreRunnerHTTPRunFixture(t *testing.T, path string, payload map[string]interface{}) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal run fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write run fixture: %v", err)
	}
}
