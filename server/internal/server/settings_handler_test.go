package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func boolPtr(v bool) *bool { return &v }

func TestGetSmartToolSelection_DefaultTrue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if !h.GetSmartToolSelection() {
		t.Fatalf("GetSmartToolSelection() = false, want true")
	}
}

func TestGetSmartToolSelection_StoredFalse(t *testing.T) {
	store := kvstore.NewMemoryStore()
	disabled := false
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		SmartToolSelection: &disabled,
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	h := NewSettingsHandler(store)
	if h.GetSmartToolSelection() {
		t.Fatalf("GetSmartToolSelection() = true, want false")
	}
}

func TestGetMemoryRecallMode_DefaultBalanced(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetMemoryRecallMode(); got != "balanced" {
		t.Fatalf("GetMemoryRecallMode() = %q, want %q", got, "balanced")
	}
}

func TestGetMemoryRecallMode_StoredValue(t *testing.T) {
	store := kvstore.NewMemoryStore()
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		MemoryRecallMode: "aggressive",
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetMemoryRecallMode(); got != "aggressive" {
		t.Fatalf("GetMemoryRecallMode() = %q, want %q", got, "aggressive")
	}
}

func TestGetMemoryRecallMode_InvalidStoredValueFallback(t *testing.T) {
	store := kvstore.NewMemoryStore()
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		MemoryRecallMode: "unexpected-mode",
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetMemoryRecallMode(); got != "balanced" {
		t.Fatalf("GetMemoryRecallMode() = %q, want %q", got, "balanced")
	}
}

func TestGetIMHistoryLimit_DefaultThree(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetIMHistoryLimit(); got != 3 {
		t.Fatalf("GetIMHistoryLimit() = %d, want 3", got)
	}
}

func TestGetIMHistoryLimit_StoredNegativeClampedToZero(t *testing.T) {
	store := kvstore.NewMemoryStore()
	v := -9
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		IMHistoryLimit: &v,
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetIMHistoryLimit(); got != 0 {
		t.Fatalf("GetIMHistoryLimit() = %d, want 0", got)
	}
}

func TestGetSmartSkillSelection_DefaultFalse(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSmartSkillSelection() {
		t.Fatalf("GetSmartSkillSelection() = true, want false")
	}
}

func TestGetSkillSelectorMode_DefaultHybrid(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetSkillSelectorMode(); got != "hybrid" {
		t.Fatalf("GetSkillSelectorMode() = %q, want %q", got, "hybrid")
	}
}

func TestGetSkillRerankEnabled_DefaultFalse(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSkillRerankEnabled() {
		t.Fatalf("GetSkillRerankEnabled() = true, want false")
	}
}

func TestGetEffectiveSkillRerankEnabled(t *testing.T) {
	tests := []struct {
		name                      string
		skillRerankEnabled        *bool
		smallModelEnabled         *bool
		smallModelRerankEnabled   *bool
		wantEffectiveRerankEnable bool
	}{
		{
			name:                      "default disabled",
			wantEffectiveRerankEnable: false,
		},
		{
			name:                      "skill rerank disabled",
			skillRerankEnabled:        boolPtr(false),
			wantEffectiveRerankEnable: false,
		},
		{
			name:                      "small model rerank disabled while small model enabled",
			skillRerankEnabled:        boolPtr(true),
			smallModelEnabled:         boolPtr(true),
			smallModelRerankEnabled:   boolPtr(false),
			wantEffectiveRerankEnable: false,
		},
		{
			name:                      "small model rerank disabled but small model off",
			skillRerankEnabled:        boolPtr(true),
			smallModelEnabled:         boolPtr(false),
			smallModelRerankEnabled:   boolPtr(false),
			wantEffectiveRerankEnable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewSettingsHandler(kvstore.NewMemoryStore())
			h.settings.SkillRerankEnabled = tt.skillRerankEnabled
			h.settings.SmallModelEnabled = tt.smallModelEnabled
			h.settings.SmallModelRerankEnabled = tt.smallModelRerankEnabled
			if got := h.GetEffectiveSkillRerankEnabled(); got != tt.wantEffectiveRerankEnable {
				t.Fatalf("GetEffectiveSkillRerankEnabled() = %v, want %v", got, tt.wantEffectiveRerankEnable)
			}
		})
	}
}

func TestGetSkillRerankONNXEnabled_DefaultFalse(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSkillRerankONNXEnabled() {
		t.Fatalf("GetSkillRerankONNXEnabled() = true, want false")
	}
}

func TestGetSkillRerankONNXAutoDownload_DefaultFalse(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSkillRerankONNXAutoDownload() {
		t.Fatalf("GetSkillRerankONNXAutoDownload() = true, want false")
	}
}

func TestGetSkillSelectorConfidenceThreshold_Default(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetSkillSelectorConfidenceThreshold(); got != 0.78 {
		t.Fatalf("GetSkillSelectorConfidenceThreshold() = %v, want 0.78", got)
	}
}

func TestSelectorDryRunReturnsDebugSignals(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	smartSkill := true
	h.settings.SmartSkillSelection = &smartSkill

	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_search", Description: "Search the web for latest sources."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files."})

	chatHandler := NewChatHandler(nil, nil, registry)
	chatHandler.SetSettingsHandler(h)
	chatHandler.SetToolSelector(tools.DefaultToolSelector())
	chatHandler.SetToolRouter(tools.DefaultToolRouter())

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	for _, tc := range []struct {
		id   string
		desc string
	}{
		{id: "web_search", desc: "search the web"},
		{id: "browser", desc: "browse urls"},
	} {
		dir := filepath.Join(workspaceDir, ".claude", "skills", tc.id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir skill: %v", err)
		}
		content := "---\nname: " + tc.id + "\ndescription: " + tc.desc + "\nos: [\"" + runtime.GOOS + "\"]\n---\n# " + tc.id + "\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}
	chatHandler.SetSkillSelector(claudecode.NewSkillSelector(workspaceDir, claudecode.NewHeuristicSkillReranker()))
	h.SetChatHandler(chatHandler)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/selector/dry-run", strings.NewReader(`{"query":"搜索最新新闻并给我来源和引用","model":"auto"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID:   "admin-1",
		Username: "admin",
		Role:     "admin",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.SelectorDryRun(c); err != nil {
		t.Fatalf("SelectorDryRun error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	selectedTools, ok := body["selected_tools"].([]any)
	if !ok || len(selectedTools) == 0 || selectedTools[0] != "web_search" {
		t.Fatalf("expected selected_tools to start with web_search, got=%v", body["selected_tools"])
	}

	toolDebug, ok := body["tool_debug"].(map[string]any)
	if !ok {
		t.Fatalf("expected tool_debug payload, got=%T", body["tool_debug"])
	}
	querySignals, ok := toolDebug["query_signals"].(map[string]any)
	if !ok || querySignals["live_web"] != true {
		t.Fatalf("expected live_web query signal, got=%v", toolDebug["query_signals"])
	}

	skillDecision, ok := body["skill_decision"].(map[string]any)
	if !ok {
		t.Fatalf("expected skill_decision payload, got=%T", body["skill_decision"])
	}
	if skillDecision["selected_skill"] != "web_search" {
		t.Fatalf("expected skill_decision.selected_skill=web_search, got=%v", skillDecision["selected_skill"])
	}
	if _, ok := skillDecision["matched_signals"].([]any); !ok {
		t.Fatalf("expected matched_signals in skill decision, got=%v", skillDecision["matched_signals"])
	}
}

func TestGetAgentAskTimeoutSeconds_Default(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetAgentAskTimeoutSeconds(); got != 120 {
		t.Fatalf("GetAgentAskTimeoutSeconds() = %d, want 120", got)
	}
}

func TestGetAgentAskTimeoutSeconds_Clamped(t *testing.T) {
	store := kvstore.NewMemoryStore()
	v := 3
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		AgentAskTimeoutSeconds: &v,
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetAgentAskTimeoutSeconds(); got != 15 {
		t.Fatalf("GetAgentAskTimeoutSeconds() = %d, want 15", got)
	}
}

func TestGetAgentAskTimeoutAction_Default(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetAgentAskTimeoutAction(); got != "default" {
		t.Fatalf("GetAgentAskTimeoutAction() = %q, want %q", got, "default")
	}
}

func TestGetAgentAskTimeoutAction_StoredError(t *testing.T) {
	store := kvstore.NewMemoryStore()
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		AgentAskTimeoutAction: "error",
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetAgentAskTimeoutAction(); got != "error" {
		t.Fatalf("GetAgentAskTimeoutAction() = %q, want %q", got, "error")
	}
}

func TestGetAgentAutoReflect_DefaultTrue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if !h.GetAgentAutoReflect() {
		t.Fatal("GetAgentAutoReflect() = false, want true")
	}
}

func TestGetAgentAutoReflect_StoredFalse(t *testing.T) {
	store := kvstore.NewMemoryStore()
	disabled := false
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		AgentAutoReflect: &disabled,
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if h.GetAgentAutoReflect() {
		t.Fatal("GetAgentAutoReflect() = true, want false")
	}
}

func TestGetSmallModelDefaults(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSmallModelEnabled() {
		t.Fatal("GetSmallModelEnabled() = true, want false")
	}
	if got := h.GetSmallModelRuntime(); got != smallmodel.RuntimeType {
		t.Fatalf("GetSmallModelRuntime() = %q, want %q", got, smallmodel.RuntimeType)
	}
	if got := h.GetSmallModelID(); got != smallmodel.ModelID {
		t.Fatalf("GetSmallModelID() = %q, want %q", got, smallmodel.ModelID)
	}
	if !h.GetSmallModelAutoDownload() {
		t.Fatal("GetSmallModelAutoDownload() = false, want true")
	}
	if h.GetSmallModelSummaryEnabled() || h.GetSmallModelDocExtractEnabled() || h.GetSmallModelRerankEnabled() || h.GetSmallModelContextPruneEnabled() {
		t.Fatal("expected phase1 enhancement switches default false")
	}
	if h.GetSmallModelMediaIntentEnabled() {
		t.Fatal("expected media intent switch default false")
	}
	if h.GetSmallModelRouteImageQAEnabled() || h.GetSmallModelRouteShortQAEnabled() || h.GetSmallModelRouteToolDispatchEnabled() {
		t.Fatal("expected image-qa/short-qa/tool-dispatch route switches default false")
	}
	if h.GetOfflineIRFallbackEnabled() {
		t.Fatal("expected offline IR fallback switch default false")
	}
	if h.GetFeatureIntentIREnabled() {
		t.Fatal("expected feature intent IR switch default false")
	}
	if h.GetDeepResearchV2Enabled() {
		t.Fatal("expected deep research v2 switch default false")
	}
}

func TestGetSmallModelRouteImageQAEnabled_InheritsShortQAWhenUnset(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	h.settings.SmallModelRouteShortQAEnabled = &enabled
	if !h.GetSmallModelRouteImageQAEnabled() {
		t.Fatal("expected image QA switch to inherit short QA setting when unset")
	}

	disabled := false
	h.settings.SmallModelRouteImageQAEnabled = &disabled
	if h.GetSmallModelRouteImageQAEnabled() {
		t.Fatal("expected explicit image QA switch to override inherited short QA setting")
	}
}

func TestPatchSmallModelRouteImageQAEnabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"small_model_route_image_qa_enabled":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !h.GetSmallModelRouteImageQAEnabled() {
		t.Fatal("expected image QA switch enabled after patch")
	}

	h2 := NewSettingsHandler(store)
	if !h2.GetSmallModelRouteImageQAEnabled() {
		t.Fatal("expected persisted image QA switch enabled")
	}
}

func TestPatchDeepResearchV2Enabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"deep_research_v2_enabled":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !h.GetDeepResearchV2Enabled() {
		t.Fatal("expected deep research v2 switch enabled after patch")
	}

	h2 := NewSettingsHandler(store)
	if !h2.GetDeepResearchV2Enabled() {
		t.Fatal("expected persisted deep research v2 switch enabled")
	}
}

func TestPatchDeepResearchV2Disabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"deep_research_v2_enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if h.GetDeepResearchV2Enabled() {
		t.Fatal("expected deep research v2 switch disabled after patch")
	}

	h2 := NewSettingsHandler(store)
	if h2.GetDeepResearchV2Enabled() {
		t.Fatal("expected persisted deep research v2 switch disabled")
	}
}

func TestPatchLegacyThemeStyleIgnored(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"locale":"en-US","theme_style":"minimal"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if h.settings.Locale != "en-US" {
		t.Fatalf("Locale = %q, want %q", h.settings.Locale, "en-US")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, exists := resp["theme_style"]; exists {
		t.Fatalf("unexpected theme_style field in response: %s", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "theme_style") {
		t.Fatalf("response should not contain theme_style: %s", rec.Body.String())
	}
}

func TestPatchSmallModelMediaIntentEnabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"small_model_media_intent_enabled":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if h.GetSmallModelMediaIntentEnabled() {
		t.Fatal("expected media intent switch disabled after patch")
	}

	h2 := NewSettingsHandler(store)
	if h2.GetSmallModelMediaIntentEnabled() {
		t.Fatal("expected persisted media intent switch disabled")
	}
}

func TestPatchSmallModelContextPruneToolRules_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	body := `{
		"small_model_context_prune_tool_allow":[" exec ","web_*","exec",""],
		"small_model_context_prune_tool_deny":["web_search","web_search"," "]
	}`
	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	allow := h.GetSmallModelContextPruneToolAllow()
	deny := h.GetSmallModelContextPruneToolDeny()
	if len(allow) != 2 || allow[0] != "exec" || allow[1] != "web_*" {
		t.Fatalf("allow = %v, want [exec web_*]", allow)
	}
	if len(deny) != 1 || deny[0] != "web_search" {
		t.Fatalf("deny = %v, want [web_search]", deny)
	}

	h2 := NewSettingsHandler(store)
	allow2 := h2.GetSmallModelContextPruneToolAllow()
	deny2 := h2.GetSmallModelContextPruneToolDeny()
	if len(allow2) != 2 || allow2[0] != "exec" || allow2[1] != "web_*" {
		t.Fatalf("persisted allow = %v, want [exec web_*]", allow2)
	}
	if len(deny2) != 1 || deny2[0] != "web_search" {
		t.Fatalf("persisted deny = %v, want [web_search]", deny2)
	}
}

func TestPatchIMHistoryLimit_PersistedAndClamped(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"im_history_limit":5}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := h.GetIMHistoryLimit(); got != 5 {
		t.Fatalf("GetIMHistoryLimit() = %d, want 5", got)
	}

	h2 := NewSettingsHandler(store)
	if got := h2.GetIMHistoryLimit(); got != 5 {
		t.Fatalf("persisted GetIMHistoryLimit() = %d, want 5", got)
	}

	req2 := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"im_history_limit":-2}`))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	if err := h2.Patch(c2); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec2.Code)
	}
	if got := h2.GetIMHistoryLimit(); got != 0 {
		t.Fatalf("GetIMHistoryLimit() = %d, want 0", got)
	}

	h3 := NewSettingsHandler(store)
	if got := h3.GetIMHistoryLimit(); got != 0 {
		t.Fatalf("persisted GetIMHistoryLimit() = %d, want 0", got)
	}
}

func TestSetSmallModelRouteShortQAEnabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	changed, err := h.SetSmallModelRouteShortQAEnabled(true)
	if err != nil {
		t.Fatalf("SetSmallModelRouteShortQAEnabled(true) failed: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first update")
	}
	if !h.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short QA route enabled")
	}

	h2 := NewSettingsHandler(store)
	if !h2.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected persisted short QA route enabled")
	}

	changed, err = h2.SetSmallModelRouteShortQAEnabled(true)
	if err != nil {
		t.Fatalf("SetSmallModelRouteShortQAEnabled(true) second call failed: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false when setting unchanged")
	}
}

func TestSetSmallModelRouteToolDispatchEnabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	changed, err := h.SetSmallModelRouteToolDispatchEnabled(true)
	if err != nil {
		t.Fatalf("SetSmallModelRouteToolDispatchEnabled(true) failed: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first update")
	}
	if !h.GetSmallModelRouteToolDispatchEnabled() {
		t.Fatal("expected tool-dispatch route enabled")
	}

	h2 := NewSettingsHandler(store)
	if !h2.GetSmallModelRouteToolDispatchEnabled() {
		t.Fatal("expected persisted tool-dispatch route enabled")
	}

	changed, err = h2.SetSmallModelRouteToolDispatchEnabled(true)
	if err != nil {
		t.Fatalf("SetSmallModelRouteToolDispatchEnabled(true) second call failed: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false when setting unchanged")
	}
}

func TestSetSmallModelSummaryEnabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	changed, err := h.SetSmallModelSummaryEnabled(false)
	if err != nil {
		t.Fatalf("SetSmallModelSummaryEnabled(false) failed: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first update")
	}
	if h.GetSmallModelSummaryEnabled() {
		t.Fatal("expected summary switch disabled")
	}

	h2 := NewSettingsHandler(store)
	if h2.GetSmallModelSummaryEnabled() {
		t.Fatal("expected persisted summary switch disabled")
	}

	changed, err = h2.SetSmallModelSummaryEnabled(false)
	if err != nil {
		t.Fatalf("SetSmallModelSummaryEnabled(false) second call failed: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false when setting unchanged")
	}
}

func TestSetSmallModelDocExtractEnabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	changed, err := h.SetSmallModelDocExtractEnabled(false)
	if err != nil {
		t.Fatalf("SetSmallModelDocExtractEnabled(false) failed: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first update")
	}
	if h.GetSmallModelDocExtractEnabled() {
		t.Fatal("expected doc-extract switch disabled")
	}

	h2 := NewSettingsHandler(store)
	if h2.GetSmallModelDocExtractEnabled() {
		t.Fatal("expected persisted doc-extract switch disabled")
	}

	changed, err = h2.SetSmallModelDocExtractEnabled(false)
	if err != nil {
		t.Fatalf("SetSmallModelDocExtractEnabled(false) second call failed: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false when setting unchanged")
	}
}

func TestGetNoLLMDegradeMode_DefaultDeepResearch(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetNoLLMDegradeMode(); got != "deepresearch" {
		t.Fatalf("GetNoLLMDegradeMode() = %q, want %q", got, "deepresearch")
	}
}

func TestGetSmallModelUnavailablePolicy_DefaultIRFirst(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetSmallModelUnavailablePolicy(); got != "ir_first" {
		t.Fatalf("GetSmallModelUnavailablePolicy() = %q, want %q", got, "ir_first")
	}
}

func TestGetSmallModelUnavailablePolicy_StoredLLMFirstIgnored(t *testing.T) {
	store := kvstore.NewMemoryStore()
	if err := store.SetJSON(context.Background(), settingsKVKey, &Settings{
		SmallModelUnavailablePolicy: "llm_first",
	}, 0); err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	h := NewSettingsHandler(store)
	if got := h.GetSmallModelUnavailablePolicy(); got != "ir_first" {
		t.Fatalf("GetSmallModelUnavailablePolicy() = %q, want %q", got, "ir_first")
	}
}

func TestGetSmallModelStatus_NoManager(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/settings/small-model/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetSmallModelStatus(c); err != nil {
		t.Fatalf("GetSmallModelStatus failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var st smallmodel.Status
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if st.Ready {
		t.Fatal("status.Ready = true, want false without manager")
	}
	if st.ModelID != smallmodel.ModelID {
		t.Fatalf("status.ModelID = %q, want %q", st.ModelID, smallmodel.ModelID)
	}
	if st.Runtime != smallmodel.RuntimeType {
		t.Fatalf("status.Runtime = %q, want %q", st.Runtime, smallmodel.RuntimeType)
	}
}

func TestSmallModelDownloadEndpoints_NoManager(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	e := echo.New()

	startReq := httptest.NewRequest(http.MethodPost, "/api/settings/small-model/download", nil)
	startRec := httptest.NewRecorder()
	startCtx := e.NewContext(startReq, startRec)
	if err := h.StartSmallModelDownload(startCtx); err != nil {
		t.Fatalf("StartSmallModelDownload failed: %v", err)
	}
	if startRec.Code != http.StatusBadRequest {
		t.Fatalf("start status = %d, want 400", startRec.Code)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/api/settings/small-model/cancel", nil)
	cancelRec := httptest.NewRecorder()
	cancelCtx := e.NewContext(cancelReq, cancelRec)
	if err := h.CancelSmallModelDownload(cancelCtx); err != nil {
		t.Fatalf("CancelSmallModelDownload failed: %v", err)
	}
	if cancelRec.Code != http.StatusBadRequest {
		t.Fatalf("cancel status = %d, want 400", cancelRec.Code)
	}
}
