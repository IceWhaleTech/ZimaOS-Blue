package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/optimization"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type stubAgentcoreRunnerManager struct {
	status           AgentcoreRunnerStatus
	lastRun          optimization.OptimizationRunRecord
	prepareCalls     int
	lastPrepareRepo  string
	lastPrepareRef   string
	lastPrepareParts []string
}

func (m *stubAgentcoreRunnerManager) GetStatus(_ context.Context) AgentcoreRunnerStatus {
	return m.status
}

func (m *stubAgentcoreRunnerManager) Prepare(_ context.Context, req AgentcoreRunnerPrepareRequest) (AgentcoreRunnerStatus, error) {
	m.prepareCalls++
	m.lastPrepareRepo = req.RepoURL
	m.lastPrepareRef = req.Ref
	m.lastPrepareParts = append([]string(nil), req.RequestedParts...)
	m.status.RepoURL = req.RepoURL
	m.status.ResolvedRef = req.Ref
	m.status.LastPrepareState = "preparing"
	return m.status, nil
}

func (m *stubAgentcoreRunnerManager) GetLastOptimizationRun(_ context.Context) (optimization.OptimizationRunRecord, error) {
	return m.lastRun, nil
}

func TestSettingsHandlerPatchStoresAgentcoreRunnerSettings(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e := echo.New()

	if err := handler.Patch(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Patch returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Patch status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !handler.GetExperimentalAgentcoreRunnerEnabled() {
		t.Fatal("expected experimental agentcore runner to be enabled")
	}
	if got := handler.GetExperimentalAgentcoreRunnerRepoURL(); got != "https://github.com/IceWhaleTech/ZimaOS-Blue" {
		t.Fatalf("repo url = %q", got)
	}
	if got := handler.GetExperimentalAgentcoreRunnerRef(); got != "main" {
		t.Fatalf("ref = %q", got)
	}
}

func TestSettingsHandlerGetDefaultsAgentcoreRunnerRepoURLWhenUnset(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	e := echo.New()

	if err := handler.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body Settings
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got := handler.GetExperimentalAgentcoreRunnerRepoURL(); got != "https://github.com/IceWhaleTech/ZimaOS-Blue" {
		t.Fatalf("default getter repo url = %q", got)
	}
	if got := strings.TrimSpace(body.ExperimentalAgentcoreRunnerRepoURL); got != "https://github.com/IceWhaleTech/ZimaOS-Blue" {
		t.Fatalf("default response repo url = %q", got)
	}
}

func TestSettingsHandlerGetDefaultsAgentcoreRunnerRefWhenUnset(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())

	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	e := echo.New()

	if err := handler.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body Settings
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got := handler.GetExperimentalAgentcoreRunnerRef(); got != "main" {
		t.Fatalf("default getter ref = %q", got)
	}
	if got := strings.TrimSpace(body.ExperimentalAgentcoreRunnerRef); got != "main" {
		t.Fatalf("default response ref = %q", got)
	}
}

func TestSettingsHandlerResolveExperimentalAgentcoreRunnerRefUsesConversationOverride(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Runner Ref", "user-1")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if err := store.UpsertConversationCommandState(context.Background(), memory.ConversationCommandState{
		ConversationID:     conv.ID,
		AgentcoreRunnerRef: "release/v2",
	}); err != nil {
		t.Fatalf("UpsertConversationCommandState: %v", err)
	}

	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.settings.ExperimentalAgentcoreRunnerRef = "main"
	handler.SetChatHandler(NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry()))

	if got := handler.ResolveExperimentalAgentcoreRunnerRef(context.Background(), conv.ID); got != "release/v2" {
		t.Fatalf("ResolveExperimentalAgentcoreRunnerRef = %q, want release/v2", got)
	}
}

func TestSettingsHandlerResolveExperimentalAgentcoreRunnerRefFallsBackToStoredSetting(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.settings.ExperimentalAgentcoreRunnerRef = "main"

	if got := handler.ResolveExperimentalAgentcoreRunnerRef(context.Background(), ""); got != "main" {
		t.Fatalf("ResolveExperimentalAgentcoreRunnerRef(empty) = %q, want main", got)
	}
	if got := handler.ResolveExperimentalAgentcoreRunnerRef(context.Background(), "missing-conversation"); got != "main" {
		t.Fatalf("ResolveExperimentalAgentcoreRunnerRef(missing) = %q, want main", got)
	}
}

func TestSettingsHandlerAgentcoreRunnerStatusEndpoint(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	prepareAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	handler.settings.ExperimentalAgentcoreRunnerEnabled = boolPtr(true)
	handler.settings.ExperimentalAgentcoreRunnerRepoURL = "https://github.com/IceWhaleTech/ZimaOS-Blue"
	handler.settings.ExperimentalAgentcoreRunnerRef = "main"
	handler.SetAgentcoreRunnerManager(&stubAgentcoreRunnerManager{
		status: AgentcoreRunnerStatus{
			ResolvedCommit:        "abcdef123456",
			RequiredGoVersion:     "1.24.0",
			InstalledGoVersion:    "1.24.0",
			ToolchainReady:        true,
			BinaryReady:           true,
			BinaryPath:            "/tmp/agentcore-runner",
			BinarySHA256:          "deadbeef",
			LastPrepareAt:         &prepareAt,
			LastPrepareState:      "ready",
			LastOptimizationRunID: "opt-123",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/settings/agentcore-runner/status", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := handler.GetAgentcoreRunnerStatus(e.NewContext(req, rec)); err != nil {
		t.Fatalf("GetAgentcoreRunnerStatus returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body AgentcoreRunnerStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !body.Enabled || !body.BinaryReady || body.ResolvedCommit != "abcdef123456" {
		t.Fatalf("unexpected body %#v", body)
	}
	if body.LastOptimizationRunID != "opt-123" {
		t.Fatalf("unexpected optimization summary %#v", body)
	}
}

func TestSettingsHandlerAgentcoreRunnerPrepareEndpointUsesStoredSettings(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	manager := &stubAgentcoreRunnerManager{}
	handler.SetAgentcoreRunnerManager(manager)
	handler.settings.ExperimentalAgentcoreRunnerRepoURL = "https://github.com/IceWhaleTech/ZimaOS-Blue"
	handler.settings.ExperimentalAgentcoreRunnerRef = "main"

	req := httptest.NewRequest(http.MethodPost, "/api/settings/agentcore-runner/prepare", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := handler.PrepareAgentcoreRunner(e.NewContext(req, rec)); err != nil {
		t.Fatalf("PrepareAgentcoreRunner returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if manager.prepareCalls != 1 {
		t.Fatalf("prepareCalls = %d, want 1", manager.prepareCalls)
	}
	if manager.lastPrepareRepo != "https://github.com/IceWhaleTech/ZimaOS-Blue" || manager.lastPrepareRef != "main" {
		t.Fatalf("prepare request = repo %q ref %q", manager.lastPrepareRepo, manager.lastPrepareRef)
	}
	if len(manager.lastPrepareParts) != 0 {
		t.Fatalf("prepare requested parts = %#v, want empty", manager.lastPrepareParts)
	}
}

func TestSettingsHandlerAgentcoreRunnerPrepareEndpointUsesDefaultRepoWhenUnset(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	manager := &stubAgentcoreRunnerManager{}
	handler.SetAgentcoreRunnerManager(manager)
	handler.settings.ExperimentalAgentcoreRunnerRef = "main"

	req := httptest.NewRequest(http.MethodPost, "/api/settings/agentcore-runner/prepare", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := handler.PrepareAgentcoreRunner(e.NewContext(req, rec)); err != nil {
		t.Fatalf("PrepareAgentcoreRunner returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if manager.prepareCalls != 1 {
		t.Fatalf("prepareCalls = %d, want 1", manager.prepareCalls)
	}
	if manager.lastPrepareRepo != "https://github.com/IceWhaleTech/ZimaOS-Blue" {
		t.Fatalf("prepare request repo = %q", manager.lastPrepareRepo)
	}
}

func TestSettingsHandlerAgentcoreRunnerPrepareEndpointPassesRequestedParts(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	manager := &stubAgentcoreRunnerManager{}
	handler.SetAgentcoreRunnerManager(manager)
	handler.settings.ExperimentalAgentcoreRunnerRepoURL = "https://github.com/IceWhaleTech/ZimaOS-Blue"
	handler.settings.ExperimentalAgentcoreRunnerRef = "main"

	req := httptest.NewRequest(http.MethodPost, "/api/settings/agentcore-runner/prepare", strings.NewReader(`{
		"requested_parts": ["context_assembly", "prompt_template"]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := handler.PrepareAgentcoreRunner(e.NewContext(req, rec)); err != nil {
		t.Fatalf("PrepareAgentcoreRunner returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if want := []string{"prompt_template", "context_assembly"}; !reflect.DeepEqual(manager.lastPrepareParts, want) {
		t.Fatalf("prepare requested parts = %#v, want %#v", manager.lastPrepareParts, want)
	}
}

func TestSettingsHandlerAgentcoreRunnerPrepareEndpointRejectsUnknownRequestedParts(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	manager := &stubAgentcoreRunnerManager{}
	handler.SetAgentcoreRunnerManager(manager)

	req := httptest.NewRequest(http.MethodPost, "/api/settings/agentcore-runner/prepare", strings.NewReader(`{
		"requested_parts": ["prompt_template", "mystery_surface"]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := handler.PrepareAgentcoreRunner(e.NewContext(req, rec)); err != nil {
		t.Fatalf("PrepareAgentcoreRunner returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if manager.prepareCalls != 0 {
		t.Fatalf("prepareCalls = %d, want 0", manager.prepareCalls)
	}
}

func TestSettingsHandlerAgentcoreRunnerTagsEndpointUsesRequestedRepo(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.agentcoreRunnerTagResolver = func(_ context.Context, repoURL string) ([]string, error) {
		if repoURL != "owner/repo" {
			t.Fatalf("repoURL = %q, want owner/repo", repoURL)
		}
		return []string{"v1.2.0", "v1.1.0"}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/api/settings/agentcore-runner/tags?repo_url=owner/repo", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := handler.GetAgentcoreRunnerTags(e.NewContext(req, rec)); err != nil {
		t.Fatalf("GetAgentcoreRunnerTags returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body AgentcoreRunnerTagList
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.RepoURL != "https://github.com/owner/repo" {
		t.Fatalf("repo_url = %q", body.RepoURL)
	}
	if body.DefaultRef != "main" {
		t.Fatalf("default_ref = %q", body.DefaultRef)
	}
	if strings.Join(body.Tags, ",") != "v1.2.0,v1.1.0" {
		t.Fatalf("tags = %#v", body.Tags)
	}
}

func TestSettingsHandlerAgentcoreRunnerLastRunEndpointAddsEvolvablePartsCompatibilityFields(t *testing.T) {
	handler := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetAgentcoreRunnerManager(&stubAgentcoreRunnerManager{
		lastRun: optimization.OptimizationRunRecord{
			"id":                   "opt-legacy",
			"optimization_surface": "runner_code",
			"offline_value_report": map[string]interface{}{
				"offline_recommendation": "hold",
				"value_summary":          "Accepted execution candidate but cutover readiness is still pending.",
			},
			"runtime_value_report": map[string]interface{}{
				"status":        "provisional",
				"value_summary": "Promoted candidate has not collected enough runtime evidence yet.",
			},
			"proposal_set": []interface{}{
				map[string]interface{}{"candidate_id": "candidate-a"},
			},
			"pareto_frontier": []interface{}{"candidate-a"},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/settings/agentcore-runner/last-run", nil)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := handler.GetAgentcoreRunnerLastRun(e.NewContext(req, rec)); err != nil {
		t.Fatalf("GetAgentcoreRunnerLastRun returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got := strings.TrimSpace(body["primary_part"].(string)); got != "runner_code" {
		t.Fatalf("primary_part = %q, want runner_code", got)
	}
	optimized, ok := body["optimized_parts"].([]interface{})
	if !ok || len(optimized) != 1 || strings.TrimSpace(optimized[0].(string)) != "runner_code" {
		t.Fatalf("optimized_parts = %#v", body["optimized_parts"])
	}
	supported, ok := body["supported_parts"].([]interface{})
	if !ok || len(supported) == 0 {
		t.Fatalf("supported_parts = %#v", body["supported_parts"])
	}
	offline, ok := body["offline_value_report"].(map[string]interface{})
	if !ok || strings.TrimSpace(offline["offline_recommendation"].(string)) != "hold" {
		t.Fatalf("offline_value_report = %#v", body["offline_value_report"])
	}
	runtime, ok := body["runtime_value_report"].(map[string]interface{})
	if !ok || strings.TrimSpace(runtime["status"].(string)) != "provisional" {
		t.Fatalf("runtime_value_report = %#v", body["runtime_value_report"])
	}
	if frontier, ok := body["pareto_frontier"].([]interface{}); !ok || len(frontier) != 1 {
		t.Fatalf("pareto_frontier = %#v", body["pareto_frontier"])
	}
}
