package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skilladvisor"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func boolPtr(v bool) *bool { return &v }

func writeSettingsSelectorSkillAtRoot(t *testing.T, workspaceDir, rootDir, dirName, manifestName, desc, invocation string, capabilityTags ...string) {
	t.Helper()

	dir := filepath.Join(workspaceDir, rootDir, "skills", dirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}

	content := strings.Builder{}
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("name: %s\n", manifestName))
	content.WriteString("version: \"1.0.0\"\n")
	content.WriteString(fmt.Sprintf("description: %q\n", desc))
	if invocation != "" {
		content.WriteString(fmt.Sprintf("invocation: %q\n", invocation))
		content.WriteString("examples:\n")
		content.WriteString(fmt.Sprintf("  - %q\n", invocation))
	}
	if len(capabilityTags) > 0 {
		content.WriteString("capability_tags:\n")
		for _, tag := range capabilityTags {
			content.WriteString("  - " + tag + "\n")
		}
	}
	content.WriteString("interaction_mode: stateless\n")
	content.WriteString("card_support: none\n")
	content.WriteString("os: [\"" + runtime.GOOS + "\"]\n")
	content.WriteString("---\n")
	content.WriteString("# " + manifestName + "\n")

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content.String()), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}

func writeSettingsSelectorSkill(t *testing.T, workspaceDir, id, desc, invocation string, capabilityTags ...string) {
	t.Helper()
	writeSettingsSelectorSkillAtRoot(t, workspaceDir, ".claude", id, id, desc, invocation, capabilityTags...)
}

func writeSettingsSelectorCanonicalWebQuerySkill(t *testing.T, workspaceDir, desc, invocation string, capabilityTags ...string) {
	t.Helper()
	writeSettingsSelectorSkillAtRoot(t, workspaceDir, ".claude", "web_query", "web_query", desc, invocation, capabilityTags...)
}

func newSelectorDryRunTestHandler(t *testing.T) *SettingsHandler {
	t.Helper()

	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "ask", Description: "Ask the user clarifying questions."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "config", Description: "Manage providers, settings, and diagnostics."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web for latest sources."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "browser", Description: "Open and interact with web pages."})

	chatHandler := NewChatHandler(nil, nil, registry)
	chatHandler.SetSettingsHandler(h)
	chatHandler.SetToolSelector(tools.DefaultToolSelector())
	chatHandler.SetToolRouter(tools.DefaultToolRouter())

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
	writeSettingsSelectorSkill(t, workspaceDir, "ask", "ask the user clarifying questions and wait for their answer", `blue ask q="Choose a deploy strategy" a='["Canary","Blue-Green"]'`, "clarify", "interactive", "question")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")
	writeSettingsSelectorSkill(t, workspaceDir, "reminder", "schedule reminders and user notifications at a specific time", `blue reminder add message="Standup" time="2026-03-01 09:00"`, "reminder", "notify", "schedule")
	writeSettingsSelectorSkill(t, workspaceDir, "browser", "browse urls and interact with web pages", "blue browser.navigate url=https://example.com", "browser", "web")
	writeSettingsSelectorSkill(t, workspaceDir, "config", "manage providers settings channels skills tools health and proxy diagnostics", "blue config.providers.list", "admin", "settings", "providers", "diagnostics")
	writeSettingsSelectorSkill(t, workspaceDir, "ui_reviewer", "review screenshots and UI layouts for accessibility and visual issues", `blue ui_reviewer target="screenshot.png"`, "ui", "review", "screenshot")

	chatHandler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
	h.SetChatHandler(chatHandler)
	return h
}

func runSelectorDryRun(t *testing.T, h *SettingsHandler, query string) map[string]any {
	t.Helper()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/selector/dry-run", strings.NewReader(`{"query":`+strconv.Quote(query)+`,"model":"auto"}`))
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
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return body
}

func assertSelectorDryRunSelected(t *testing.T, body map[string]any, skill string) {
	t.Helper()

	skillDecision, ok := body["skill_decision"].(map[string]any)
	if !ok {
		t.Fatalf("expected skill_decision payload, got=%T", body["skill_decision"])
	}
	wantSelected := skill
	wantCanonical := expectedSelectorDryRunCanonicalSkillID(skill)
	wantResearchMode := ""
	switch skill {
	case "analyze":
		wantSelected = "research"
		wantResearchMode = "analyze"
	case "deep_research":
		wantSelected = "research"
		wantResearchMode = "deep_research"
	case "ui_reviewer":
		wantSelected = "research"
		wantResearchMode = "ui_review"
	}
	if skillDecision["selected_skill"] != wantSelected {
		t.Fatalf("expected skill_decision.selected_skill=%s, got=%v", wantSelected, skillDecision["selected_skill"])
	}
	if wantResearchMode != "" && skillDecision["research_mode"] != wantResearchMode {
		t.Fatalf("expected skill_decision.research_mode=%s, got=%v", wantResearchMode, skillDecision["research_mode"])
	}
	if body["canonical_skill_id"] != wantCanonical {
		t.Fatalf("expected canonical_skill_id=%s, got=%v", wantCanonical, body["canonical_skill_id"])
	}
	if body["skill_route_outcome"] != "selected" {
		t.Fatalf("expected skill_route_outcome=selected, got=%v", body["skill_route_outcome"])
	}
	if body["skill_need_clarify"] != false {
		t.Fatalf("expected skill_need_clarify=false, got=%v", body["skill_need_clarify"])
	}
	if hint, _ := body["skill_prompt_hint"].(string); strings.TrimSpace(hint) == "" {
		t.Fatalf("expected skill_prompt_hint, got=%v", body["skill_prompt_hint"])
	}
}

func expectedSelectorDryRunCanonicalSkillID(skill string) string {
	switch skill {
	case "analyze", "deep_research", "ui_reviewer":
		return string(agentcore.CanonicalResearch)
	default:
		if canonical, ok := agentcore.ResolveCanonicalSkill(skill); ok {
			return string(canonical)
		}
		return skill
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

func TestGetTimezone_UsesStoredValue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	h.settings.Timezone = "Asia/Shanghai"

	if got := h.GetTimezone(); got != "Asia/Shanghai" {
		t.Fatalf("GetTimezone() = %q, want %q", got, "Asia/Shanghai")
	}
}

func TestGetTimezone_EmptyWhenUnset(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())

	if got := h.GetTimezone(); got != "" {
		t.Fatalf("GetTimezone() = %q, want empty string", got)
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

func TestGetSmartSkillSelection_DefaultTrue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if !h.GetSmartSkillSelection() {
		t.Fatalf("GetSmartSkillSelection() = false, want true")
	}
}

func TestGetSkillSelectorMode_DefaultHybrid(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if got := h.GetSkillSelectorMode(); got != "hybrid" {
		t.Fatalf("GetSkillSelectorMode() = %q, want %q", got, "hybrid")
	}
}

func TestGetSkillDynamicExposure_DefaultTrue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if !h.GetSkillDynamicExposure() {
		t.Fatalf("GetSkillDynamicExposure() = false, want true")
	}
}

func TestGetSkillRerankEnabled_DefaultFalse(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSkillRerankEnabled() {
		t.Fatalf("GetSkillRerankEnabled() = true, want false")
	}
}

func TestDirectoryWhitelistSnapshot_DefaultTmpEnabled(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())

	enabled, entries := h.DirectoryWhitelistSnapshot()
	if runtime.GOOS == "windows" {
		if enabled || len(entries) != 0 {
			t.Fatalf("windows default directory whitelist = enabled:%v entries:%v, want disabled with no entries", enabled, entries)
		}
		return
	}

	if !enabled {
		t.Fatal("expected default directory whitelist enabled")
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %v, want one default entry", entries)
	}
	if entries[0].Path != defaultDirectoryWhitelistPath || entries[0].Alias != "" {
		t.Fatalf("entry[0] = %+v, want %s without alias", entries[0], defaultDirectoryWhitelistPath)
	}
}

func TestGet_DefaultDirectoryWhitelistIncludedInResponse(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()

	if err := h.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var settings Settings
	if err := json.Unmarshal(rec.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings response: %v", err)
	}

	if runtime.GOOS == "windows" {
		if settings.DirectoryWhitelistEnabled != nil || len(settings.DirectoryWhitelist) != 0 {
			t.Fatalf("windows settings response = %+v, want no default directory whitelist", settings)
		}
		return
	}

	if settings.DirectoryWhitelistEnabled == nil || !*settings.DirectoryWhitelistEnabled {
		t.Fatalf("directory_whitelist_enabled = %v, want true", settings.DirectoryWhitelistEnabled)
	}
	if len(settings.DirectoryWhitelist) != 1 || settings.DirectoryWhitelist[0].Path != defaultDirectoryWhitelistPath {
		t.Fatalf("directory_whitelist = %v, want [%s]", settings.DirectoryWhitelist, defaultDirectoryWhitelistPath)
	}
	if settings.SmallModelContextPruneEnabled == nil || !*settings.SmallModelContextPruneEnabled {
		t.Fatalf("small_model_context_prune_enabled = %v, want true", settings.SmallModelContextPruneEnabled)
	}
	if settings.SmallModelMediaIntentEnabled == nil || !*settings.SmallModelMediaIntentEnabled {
		t.Fatalf("small_model_media_intent_enabled = %v, want true", settings.SmallModelMediaIntentEnabled)
	}
	if settings.OfflineIRFallbackEnabled == nil || !*settings.OfflineIRFallbackEnabled {
		t.Fatalf("offline_ir_fallback_enabled = %v, want true", settings.OfflineIRFallbackEnabled)
	}
	if settings.FeatureIntentIREnabled == nil || !*settings.FeatureIntentIREnabled {
		t.Fatalf("feature_intent_ir_enabled = %v, want true", settings.FeatureIntentIREnabled)
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

func TestSelectorDryRunReturnsSelectedTools(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web for latest sources."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files."})

	chatHandler := NewChatHandler(nil, nil, registry)
	chatHandler.SetSettingsHandler(h)
	chatHandler.SetToolSelector(tools.DefaultToolSelector())
	chatHandler.SetToolRouter(tools.DefaultToolRouter())

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web", `blue web_query input="OpenAI Responses API docs"`, "search", "web")
	writeSettingsSelectorSkill(t, workspaceDir, "browser", "browse urls", "blue browser.navigate url=https://example.com", "browser", "web")
	chatHandler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
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
	if !ok {
		t.Fatalf("expected selected_tools payload, got=%T", body["selected_tools"])
	}
	selectedNames := make(map[string]bool, len(selectedTools))
	for _, item := range selectedTools {
		name, ok := item.(string)
		if !ok {
			t.Fatalf("expected string tool name, got=%T", item)
		}
		selectedNames[name] = true
	}
	for _, required := range []string{"read", "web_query", "write"} {
		if !selectedNames[required] {
			t.Fatalf("expected %q in selected_tools, got=%v", required, body["selected_tools"])
		}
	}
	toolSurface, ok := body["selected_tool_surface"].(map[string]any)
	if !ok {
		t.Fatalf("expected selected_tool_surface payload, got=%T", body["selected_tool_surface"])
	}
	if got, ok := toolSurface["tool_count"].(float64); !ok || int(got) != len(selectedTools) {
		t.Fatalf("selected_tool_surface.tool_count = %#v, want %d", toolSurface["tool_count"], len(selectedTools))
	}
	if got, ok := toolSurface["schema_bytes"].(float64); !ok || int(got) <= 0 {
		t.Fatalf("selected_tool_surface.schema_bytes = %#v, want > 0", toolSurface["schema_bytes"])
	}
	selectedNativeTools, ok := body["selected_native_tools"].([]any)
	if !ok {
		t.Fatalf("expected selected_native_tools payload, got=%T", body["selected_native_tools"])
	}
	if len(selectedNativeTools) != 1 || selectedNativeTools[0] != "exec" {
		t.Fatalf("selected_native_tools = %#v, want [exec]", selectedNativeTools)
	}
	if body["selected_native_surface_mode"] != "skill_exec" {
		t.Fatalf("selected_native_surface_mode = %#v, want skill_exec", body["selected_native_surface_mode"])
	}
	nativeToolSurface, ok := body["selected_native_tool_surface"].(map[string]any)
	if !ok {
		t.Fatalf("expected selected_native_tool_surface payload, got=%T", body["selected_native_tool_surface"])
	}
	if got, ok := nativeToolSurface["tool_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("selected_native_tool_surface.tool_count = %#v, want 1", nativeToolSurface["tool_count"])
	}
	if got, ok := nativeToolSurface["schema_bytes"].(float64); !ok || int(got) <= 0 {
		t.Fatalf("selected_native_tool_surface.schema_bytes = %#v, want > 0", nativeToolSurface["schema_bytes"])
	}
	toolDebug, ok := body["tool_debug"].(map[string]any)
	if !ok {
		t.Fatalf("expected tool_debug payload, got=%T", body["tool_debug"])
	}
	if _, ok := toolDebug["query_signals"].(map[string]any); !ok {
		t.Fatalf("expected tool_debug.query_signals, got=%v", toolDebug["query_signals"])
	}

	skillDecision, ok := body["skill_decision"].(map[string]any)
	if !ok {
		t.Fatalf("expected skill_decision payload, got=%T", body["skill_decision"])
	}
	if skillDecision["selected_skill"] != "web_query" {
		t.Fatalf("expected skill_decision.selected_skill=web_query, got=%v", skillDecision["selected_skill"])
	}
	if body["canonical_skill_id"] != "web_query" {
		t.Fatalf("expected canonical_skill_id=web_query, got=%v", body["canonical_skill_id"])
	}
	if body["skill_need_clarify"] != false {
		t.Fatalf("expected skill_need_clarify=false, got=%v", body["skill_need_clarify"])
	}
	if body["skill_route_outcome"] != "selected" {
		t.Fatalf("expected skill_route_outcome=selected, got=%v", body["skill_route_outcome"])
	}
	if body["decision_reason"] != "ir_ranked" {
		t.Fatalf("expected decision_reason=ir_ranked, got=%v", body["decision_reason"])
	}
	if body["decision_stage"] != "ir" {
		t.Fatalf("expected decision_stage=ir, got=%v", body["decision_stage"])
	}
	if body["fallback_reason"] != "ir_ranked" {
		t.Fatalf("expected fallback_reason=ir_ranked, got=%v", body["fallback_reason"])
	}
	if _, ok := skillDecision["matched_signals"].([]any); !ok {
		t.Fatalf("expected matched_signals in skill decision, got=%v", skillDecision["matched_signals"])
	}
}

func TestSelectorDryRunIncludesSkillAdvice(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files."})
	chatHandler := NewChatHandler(nil, nil, registry)
	chatHandler.SetSettingsHandler(h)
	chatHandler.SetToolSelector(tools.DefaultToolSelector())
	chatHandler.SetToolRouter(tools.DefaultToolRouter())
	h.SetChatHandler(chatHandler)
	h.SetSkillAdvisor(skilladvisor.NewService(skilladvisor.SearchFunc(func(ctx context.Context, query skillmarket.SearchQuery) (*skillmarket.SearchResponse, error) {
		return &skillmarket.SearchResponse{
			Skills: []skillmarket.SearchResult{{
				Skill: skillmarket.SkillDocument{
					ID:            "gh-release-bot",
					Name:          "GitHub Release Bot",
					Description:   "Automate GitHub Actions releases and changelog generation.",
					Installable:   true,
					SecurityBadge: skillmarket.BadgeGreen,
					RiskLevel:     skillmarket.RiskLow,
					CuratedRank:   1,
				},
				Score: 42,
			}},
			Total: 1,
		}, nil
	})))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/selector/dry-run", strings.NewReader(`{"query":"帮我做 GitHub Actions 自动发版","model":"auto"}`))
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
	advice, ok := body["skill_advice"].(map[string]any)
	if !ok {
		t.Fatalf("expected skill_advice payload, got=%T", body["skill_advice"])
	}
	if advice["need_store_search"] != true {
		t.Fatalf("expected need_store_search=true, got=%v", advice["need_store_search"])
	}
	recommended, ok := advice["recommended_ids"].([]any)
	if !ok || len(recommended) == 0 || recommended[0] != "gh-release-bot" {
		t.Fatalf("expected recommended_ids with gh-release-bot, got=%v", advice["recommended_ids"])
	}
}

func TestSelectorDryRunIncludesDiscoverFirstMetadata(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	body := runSelectorDryRun(t, h, "搜索最新 OpenAI Responses API 文档。")

	if body["selected_alias"] != "web_query" {
		t.Fatalf("selected_alias = %#v, want web_query", body["selected_alias"])
	}
	if body["selected_canonical_skill"] != "web_query" {
		t.Fatalf("selected_canonical_skill = %#v, want web_query", body["selected_canonical_skill"])
	}
	if body["execution_profile"] != "prefer_fork" {
		t.Fatalf("execution_profile = %#v, want prefer_fork", body["execution_profile"])
	}
	if body["skill_exec_cutover"] != true {
		t.Fatalf("skill_exec_cutover = %#v, want true", body["skill_exec_cutover"])
	}
	if body["forked_skill_execution"] != true {
		t.Fatalf("forked_skill_execution = %#v, want true", body["forked_skill_execution"])
	}
	if body["selected_native_surface_reason"] != "discover_first_cutover" {
		t.Fatalf("selected_native_surface_reason = %#v, want discover_first_cutover", body["selected_native_surface_reason"])
	}
	discoveryDecision, ok := body["discovery_decision"].(map[string]any)
	if !ok {
		t.Fatalf("expected discovery_decision payload, got=%T", body["discovery_decision"])
	}
	if discoveryDecision["canonical_target"] != "web_query" {
		t.Fatalf("discovery_decision.canonical_target = %#v, want web_query", discoveryDecision["canonical_target"])
	}
	if discoveryDecision["execution_profile"] != "prefer_fork" {
		t.Fatalf("discovery_decision.execution_profile = %#v, want prefer_fork", discoveryDecision["execution_profile"])
	}
	canonicalEntry, ok := body["selected_canonical_entry"].(map[string]any)
	if !ok {
		t.Fatalf("expected selected_canonical_entry payload, got=%T", body["selected_canonical_entry"])
	}
	if canonicalEntry["canonical_id"] != "web_query" {
		t.Fatalf("selected_canonical_entry.canonical_id = %#v, want web_query", canonicalEntry["canonical_id"])
	}
	if body["selected_canonical_cutover_eligible"] != true {
		t.Fatalf("selected_canonical_cutover_eligible = %#v, want true", body["selected_canonical_cutover_eligible"])
	}
	discoveryRuntime, ok := body["discovery_runtime"].(map[string]any)
	if !ok {
		t.Fatalf("expected discovery_runtime payload, got=%T", body["discovery_runtime"])
	}
	if discoveryRuntime["surface_reason"] != "discover_first_cutover" {
		t.Fatalf("discovery_runtime.surface_reason = %#v, want discover_first_cutover", discoveryRuntime["surface_reason"])
	}
	if discoveryRuntime["entry_kind"] != "skill" {
		t.Fatalf("discovery_runtime.entry_kind = %#v, want skill", discoveryRuntime["entry_kind"])
	}
	aliases, ok := discoveryRuntime["aliases"].([]any)
	if !ok || len(aliases) == 0 {
		t.Fatalf("expected discovery_runtime.aliases, got=%v", discoveryRuntime["aliases"])
	}
}

func TestSelectorDryRunOmitsRemovedCutoverSwitchesAfterCutover(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	body := runSelectorDryRun(t, h, "搜索最新 OpenAI Responses API 文档。")

	if _, exists := body["smart_skill_selection"]; exists {
		t.Fatalf("expected smart_skill_selection to be removed from selector dry-run, got=%v", body["smart_skill_selection"])
	}
	if _, exists := body["skill_dynamic_exposure"]; exists {
		t.Fatalf("expected skill_dynamic_exposure to be removed from selector dry-run, got=%v", body["skill_dynamic_exposure"])
	}

	if body["selected_native_surface_mode"] != "skill_exec" {
		t.Fatalf("selected_native_surface_mode = %#v, want skill_exec", body["selected_native_surface_mode"])
	}
	if body["selected_native_surface_reason"] != "discover_first_cutover" {
		t.Fatalf("selected_native_surface_reason = %#v, want discover_first_cutover", body["selected_native_surface_reason"])
	}
	discoveryRuntime, ok := body["discovery_runtime"].(map[string]any)
	if !ok {
		t.Fatalf("expected discovery_runtime payload, got=%T", body["discovery_runtime"])
	}
	if _, exists := discoveryRuntime["dynamic_exposure_enabled"]; exists {
		t.Fatalf("expected discovery_runtime.dynamic_exposure_enabled to be removed, got=%v", discoveryRuntime["dynamic_exposure_enabled"])
	}
	if discoveryRuntime["surface_reason"] != "discover_first_cutover" {
		t.Fatalf("discovery_runtime.surface_reason = %#v, want discover_first_cutover", discoveryRuntime["surface_reason"])
	}
}

func TestSelectorDryRunIncludesToolSurfaceAuditCounts(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	initial := runSelectorDryRun(t, h, "搜索最新 OpenAI Responses API 文档。")
	for _, key := range []string{
		"tool_surface_alias_rewrite_count",
		"tool_surface_cache_invalidation_count",
		"tool_surface_exec_cutover_count",
	} {
		if got, ok := initial[key].(float64); !ok || int(got) != 0 {
			t.Fatalf("%s = %#v, want 0", key, initial[key])
		}
	}

	h.chatHandler.toolSurfaceAudit.RecordAliasRewrite()
	h.chatHandler.toolSurfaceAudit.RecordCacheInvalidation()
	h.chatHandler.toolSurfaceAudit.RecordExecCutover()

	body := runSelectorDryRun(t, h, "搜索最新 OpenAI Responses API 文档。")
	if got, ok := body["tool_surface_alias_rewrite_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("tool_surface_alias_rewrite_count = %#v, want 1", body["tool_surface_alias_rewrite_count"])
	}
	if got, ok := body["tool_surface_cache_invalidation_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("tool_surface_cache_invalidation_count = %#v, want 1", body["tool_surface_cache_invalidation_count"])
	}
	if got, ok := body["tool_surface_exec_cutover_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("tool_surface_exec_cutover_count = %#v, want 1", body["tool_surface_exec_cutover_count"])
	}
	discoveryRuntime, ok := body["discovery_runtime"].(map[string]any)
	if !ok {
		t.Fatalf("expected discovery_runtime payload, got=%T", body["discovery_runtime"])
	}
	if got, ok := discoveryRuntime["tool_surface_alias_rewrite_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("discovery_runtime.tool_surface_alias_rewrite_count = %#v, want 1", discoveryRuntime["tool_surface_alias_rewrite_count"])
	}
	if got, ok := discoveryRuntime["tool_surface_cache_invalidation_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("discovery_runtime.tool_surface_cache_invalidation_count = %#v, want 1", discoveryRuntime["tool_surface_cache_invalidation_count"])
	}
	if got, ok := discoveryRuntime["tool_surface_exec_cutover_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("discovery_runtime.tool_surface_exec_cutover_count = %#v, want 1", discoveryRuntime["tool_surface_exec_cutover_count"])
	}
}

func TestSelectorDryRun_MultilingualCuratedRoutes(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	cases := []struct {
		skill string
	}{
		{skill: "web_query"},
		{skill: "analyze"},
		{skill: "reminder"},
		{skill: "browser"},
		{skill: "ui_reviewer"},
	}

	for _, tc := range cases {
		for _, example := range routingcue.LocalizedExamples(tc.skill) {
			body := runSelectorDryRun(t, h, example.Query)
			wantCanonical := expectedSelectorDryRunCanonicalSkillID(tc.skill)
			if body["canonical_skill_id"] != wantCanonical {
				t.Fatalf("%s locale=%s expected canonical_skill_id=%s got=%v", tc.skill, example.Locale, wantCanonical, body["canonical_skill_id"])
			}
			if body["skill_route_outcome"] != "selected" {
				t.Fatalf("%s locale=%s expected skill_route_outcome=selected got=%v body=%v", tc.skill, example.Locale, body["skill_route_outcome"], body)
			}
			if body["skill_need_clarify"] != false {
				t.Fatalf("%s locale=%s expected skill_need_clarify=false got=%v", tc.skill, example.Locale, body["skill_need_clarify"])
			}
			if hint, _ := body["skill_prompt_hint"].(string); strings.TrimSpace(hint) == "" {
				t.Fatalf("%s locale=%s expected skill_prompt_hint, got=%v", tc.skill, example.Locale, body["skill_prompt_hint"])
			}
		}
	}
}

func TestSelectorDryRun_CriticalPlanRoutes(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	tests := []struct {
		name  string
		query string
		skill string
	}{
		{
			name:  "latest_openai_docs_go_to_web_query",
			query: "最新 OpenAI Responses API 文档",
			skill: "web_query",
		},
		{
			name:  "workspace_readme_stays_local",
			query: "看下 workspace 里的 README",
			skill: "exec",
		},
		{
			name:  "ask_routes_to_ask",
			query: "ask me two clarifying questions before continuing",
			skill: "ask",
		},
		{
			name:  "config_routes_to_config",
			query: "config providers.list",
			skill: "config",
		},
		{
			name:  "reminder_request_goes_to_reminder",
			query: "帮我明早 9 点提醒",
			skill: "reminder",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertSelectorDryRunSelected(t, runSelectorDryRun(t, h, tc.query), tc.skill)
		})
	}
}

func TestSelectorDryRun_DynamicExposureCollapsesAskConfigAndWorkspaceToExec(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	tests := []struct {
		name          string
		query         string
		wantCanonical string
	}{
		{name: "workspace", query: "看下 workspace 里的 README", wantCanonical: "exec"},
		{name: "ask", query: "ask me two clarifying questions before continuing", wantCanonical: "ask"},
		{name: "config", query: "config providers.list", wantCanonical: "config"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := runSelectorDryRun(t, h, tc.query)
			if body["canonical_skill_id"] != tc.wantCanonical {
				t.Fatalf("canonical_skill_id = %#v, want %s", body["canonical_skill_id"], tc.wantCanonical)
			}
			selectedNativeTools, ok := body["selected_native_tools"].([]any)
			if !ok {
				t.Fatalf("expected selected_native_tools payload, got=%T", body["selected_native_tools"])
			}
			if len(selectedNativeTools) != 1 || selectedNativeTools[0] != "exec" {
				t.Fatalf("selected_native_tools = %#v, want [exec]", selectedNativeTools)
			}
			if body["selected_native_surface_mode"] != "skill_exec" {
				t.Fatalf("selected_native_surface_mode = %#v, want skill_exec", body["selected_native_surface_mode"])
			}
			if body["skill_exec_cutover"] != true {
				t.Fatalf("skill_exec_cutover = %#v, want true", body["skill_exec_cutover"])
			}
		})
	}
}

func TestSelectorDryRun_DynamicExposureCollapsesReminderUIReviewerAndHimalayaToExec(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	tests := []struct {
		name             string
		query            string
		wantCanonical    string
		wantResearchMode string
	}{
		{name: "reminder", query: "帮我明早 9 点提醒", wantCanonical: "reminder"},
		{name: "ui_reviewer", query: "帮我 review 一下 https://example.com 的 UI", wantCanonical: "research", wantResearchMode: "ui_review"},
		{name: "email", query: "帮我回复最新那封邮件", wantCanonical: "email"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := runSelectorDryRun(t, h, tc.query)
			if body["canonical_skill_id"] != tc.wantCanonical {
				t.Fatalf("canonical_skill_id = %#v, want %s", body["canonical_skill_id"], tc.wantCanonical)
			}
			if tc.wantResearchMode != "" {
				skillDecision, ok := body["skill_decision"].(map[string]any)
				if !ok {
					t.Fatalf("expected skill_decision payload, got=%T", body["skill_decision"])
				}
				if skillDecision["research_mode"] != tc.wantResearchMode {
					t.Fatalf("skill_decision.research_mode = %#v, want %s", skillDecision["research_mode"], tc.wantResearchMode)
				}
			}
			selectedNativeTools, ok := body["selected_native_tools"].([]any)
			if !ok {
				t.Fatalf("expected selected_native_tools payload, got=%T", body["selected_native_tools"])
			}
			if len(selectedNativeTools) != 1 || selectedNativeTools[0] != "exec" {
				t.Fatalf("selected_native_tools = %#v, want [exec]", selectedNativeTools)
			}
			if body["selected_native_surface_mode"] != "skill_exec" {
				t.Fatalf("selected_native_surface_mode = %#v, want skill_exec", body["selected_native_surface_mode"])
			}
			if body["selected_native_surface_reason"] != "discover_first_cutover" {
				t.Fatalf("selected_native_surface_reason = %#v, want discover_first_cutover", body["selected_native_surface_reason"])
			}
			if body["skill_exec_cutover"] != true {
				t.Fatalf("skill_exec_cutover = %#v, want true", body["skill_exec_cutover"])
			}
		})
	}
}

func TestSelectorDryRun_MultilingualURLBypassRoutes(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	for _, tc := range []struct {
		skill string
	}{
		{skill: "analyze"},
		{skill: "ui_reviewer"},
	} {
		for _, example := range routingcue.LocalizedURLBypassExamples(tc.skill) {
			t.Run(tc.skill+"_"+example.Locale, func(t *testing.T) {
				assertSelectorDryRunSelected(t, runSelectorDryRun(t, h, example.Query), tc.skill)
			})
		}
	}
}

func TestSelectorDryRun_ClarifyMetadataForMixedIntent(t *testing.T) {
	h := newSelectorDryRunTestHandler(t)

	body := runSelectorDryRun(t, h, "看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？")
	if body["skill_need_clarify"] != true {
		t.Fatalf("expected skill_need_clarify=true, got=%v", body["skill_need_clarify"])
	}
	if body["skill_route_outcome"] != "clarify" {
		t.Fatalf("expected skill_route_outcome=clarify, got=%v", body["skill_route_outcome"])
	}
	if _, ok := body["clarify_reason"].(string); !ok {
		t.Fatalf("expected clarify_reason string, got=%T", body["clarify_reason"])
	}
	if _, ok := body["canonical_skill_id"].(string); !ok {
		t.Fatalf("expected canonical_skill_id string, got=%T", body["canonical_skill_id"])
	}
	selectedNativeTools, ok := body["selected_native_tools"].([]any)
	if !ok {
		t.Fatalf("expected selected_native_tools payload, got=%T", body["selected_native_tools"])
	}
	if len(selectedNativeTools) != 0 {
		t.Fatalf("selected_native_tools = %#v, want empty on clarify", selectedNativeTools)
	}
	if body["selected_native_surface_mode"] != "clarify_none" {
		t.Fatalf("selected_native_surface_mode = %#v, want clarify_none", body["selected_native_surface_mode"])
	}
}

func TestSelectorDryRunStillCollapsesToExecAfterRemovedSmartSkillSwitch(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web for latest sources."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files."})
	registry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files."})

	chatHandler := NewChatHandler(nil, nil, registry)
	chatHandler.SetSettingsHandler(h)
	chatHandler.SetToolSelector(tools.DefaultToolSelector())
	chatHandler.SetToolRouter(tools.DefaultToolRouter())

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze reports and urls", `blue analyze topic="url report" --json`, "analysis", "report", "url")

	chatHandler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
	h.SetChatHandler(chatHandler)

	body := runSelectorDryRun(t, h, "搜索最新 OpenAI Responses API 文档。")
	assertSelectorDryRunSelected(t, body, "web_query")

	if _, exists := body["smart_skill_selection"]; exists {
		t.Fatalf("expected smart_skill_selection to be removed from selector dry-run, got=%v", body["smart_skill_selection"])
	}
	selectedNativeTools, ok := body["selected_native_tools"].([]any)
	if !ok {
		t.Fatalf("expected selected_native_tools payload, got=%T", body["selected_native_tools"])
	}
	if len(selectedNativeTools) != 1 || selectedNativeTools[0] != "exec" {
		t.Fatalf("selected_native_tools = %#v, want [exec]", selectedNativeTools)
	}
	if body["selected_native_surface_mode"] != "skill_exec" {
		t.Fatalf("selected_native_surface_mode = %#v, want skill_exec", body["selected_native_surface_mode"])
	}
}

func TestSelectorDryRun_SurfacesCanonicalSkillConflict(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	registry := tools.NewRegistry()
	registry.ExposeDefinition(tools.ToolDefinition{Name: "browser", Description: "Open and interact with web pages."})

	chatHandler := NewChatHandler(nil, nil, registry)
	chatHandler.SetSettingsHandler(h)
	chatHandler.SetToolSelector(tools.DefaultToolSelector())
	chatHandler.SetToolRouter(tools.DefaultToolRouter())

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSettingsSelectorSkillAtRoot(t, workspaceDir, ".agents", "team-browser", "browser", "browser from workspace agents", "blue browser.navigate url=https://example.com", "browser", "web")
	writeSettingsSelectorSkillAtRoot(t, workspaceDir, ".agents", "browser", "browser", "browser duplicate from workspace agents", "blue browser.navigate url=https://example.com", "browser", "web")

	chatHandler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
	h.SetChatHandler(chatHandler)

	body := runSelectorDryRun(t, h, "在浏览器里打开 https://example.com")
	errMsg, ok := body["skill_selector_error"].(string)
	if !ok || !strings.Contains(errMsg, `workspace/.agents canonical skill "browser"`) {
		t.Fatalf("expected canonical skill conflict error, got=%v", body["skill_selector_error"])
	}
	if _, exists := body["canonical_skill_id"]; exists {
		t.Fatalf("did not expect canonical_skill_id on selector conflict, got=%v", body["canonical_skill_id"])
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
	if got := h.GetAgentAskTimeoutSeconds(); got != 20 {
		t.Fatalf("GetAgentAskTimeoutSeconds() = %d, want 20", got)
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
	if got := h.GetContextCompressionMode(); got != "auto" {
		t.Fatalf("GetContextCompressionMode() = %q, want %q", got, "auto")
	}
	if h.GetSmallModelSummaryEnabled() || h.GetSmallModelContextCompressEnabled() || h.GetSmallModelDocExtractEnabled() || h.GetSmallModelRerankEnabled() {
		t.Fatal("expected phase1 enhancement switches default false")
	}
	if h.GetSmallModelKnowledgeFixEnabled() {
		t.Fatal("expected knowledge-fix acceleration switch default false")
	}
	if !h.GetSmallModelContextPruneEnabled() {
		t.Fatal("expected context prune switch default true")
	}
	if !h.GetSmallModelMediaIntentEnabled() {
		t.Fatal("expected media intent switch default true")
	}
	if h.GetSmallModelRouteImageQAEnabled() || h.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected image-qa/short-qa route switches default false")
	}
	if !h.GetOfflineIRFallbackEnabled() {
		t.Fatal("expected offline IR fallback switch default true")
	}
	if !h.GetFeatureIntentIREnabled() {
		t.Fatal("expected feature intent IR switch default true")
	}
	if h.GetDeepResearchV2Enabled() {
		t.Fatal("expected deep research v2 switch default false")
	}
}

func TestGetContextCompressionMode_LegacyOffNormalizesToAuto(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	h.settings.ContextCompressionMode = "off"
	if got := h.GetContextCompressionMode(); got != "auto" {
		t.Fatalf("GetContextCompressionMode() = %q, want %q for legacy off", got, "auto")
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
		"small_model_context_prune_tool_deny":["web_query","web_query"," "]
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
	if len(deny) != 1 || deny[0] != "web_query" {
		t.Fatalf("deny = %v, want [web_query]", deny)
	}

	h2 := NewSettingsHandler(store)
	allow2 := h2.GetSmallModelContextPruneToolAllow()
	deny2 := h2.GetSmallModelContextPruneToolDeny()
	if len(allow2) != 2 || allow2[0] != "exec" || allow2[1] != "web_*" {
		t.Fatalf("persisted allow = %v, want [exec web_*]", allow2)
	}
	if len(deny2) != 1 || deny2[0] != "web_query" {
		t.Fatalf("persisted deny = %v, want [web_query]", deny2)
	}
}

func TestPatchDirectoryWhitelist_PersistedAndSanitized(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	body := `{
		"directory_whitelist_enabled": true,
		"directory_whitelist": [
			{"path":" /tmp/project-a ","alias":"docs"},
			{"path":"/tmp/project-a","alias":"duplicate"},
			{"path":"relative/path","alias":"bad"},
			{"path":"/tmp/project-b","alias":"docs"},
			{"path":"/tmp/project-c","alias":"reports"}
		]
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

	enabled, entries := h.DirectoryWhitelistSnapshot()
	if !enabled {
		t.Fatal("expected directory whitelist enabled")
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %v, want 3 sanitized entries", entries)
	}
	if entries[0].Path != "/tmp/project-a" || entries[0].Alias != "docs" {
		t.Fatalf("entry[0] = %+v, want /tmp/project-a docs", entries[0])
	}
	if entries[1].Path != "/tmp/project-b" || entries[1].Alias != "" {
		t.Fatalf("entry[1] = %+v, want /tmp/project-b with duplicate alias cleared", entries[1])
	}
	if entries[2].Path != "/tmp/project-c" || entries[2].Alias != "reports" {
		t.Fatalf("entry[2] = %+v, want /tmp/project-c reports", entries[2])
	}

	h2 := NewSettingsHandler(store)
	enabled2, entries2 := h2.DirectoryWhitelistSnapshot()
	if !enabled2 {
		t.Fatal("expected persisted directory whitelist enabled")
	}
	if len(entries2) != 3 {
		t.Fatalf("persisted entries = %v, want 3", entries2)
	}
}

func TestPatchDirectoryWhitelist_DisableOverridesDefault(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/settings",
		strings.NewReader(`{"directory_whitelist_enabled":false,"directory_whitelist":[]}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	if err := h.Patch(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	enabled, entries := h.DirectoryWhitelistSnapshot()
	if enabled {
		t.Fatal("expected directory whitelist disabled")
	}
	if len(entries) != 0 {
		t.Fatalf("entries = %v, want no entries after disabling", entries)
	}

	h2 := NewSettingsHandler(store)
	enabled2, entries2 := h2.DirectoryWhitelistSnapshot()
	if enabled2 {
		t.Fatal("expected persisted directory whitelist disabled")
	}
	if len(entries2) != 0 {
		t.Fatalf("persisted entries = %v, want no entries", entries2)
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

func TestPatchSmallModelKnowledgeFixEnabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/settings",
		strings.NewReader(`{"small_model_knowledge_fix_enabled":true}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !h.GetSmallModelKnowledgeFixEnabled() {
		t.Fatal("expected knowledge-fix acceleration switch enabled after patch")
	}

	h2 := NewSettingsHandler(store)
	if !h2.GetSmallModelKnowledgeFixEnabled() {
		t.Fatal("expected persisted knowledge-fix acceleration switch enabled")
	}
}

func TestSetSmallModelKnowledgeFixEnabled_Persisted(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	changed, err := h.SetSmallModelKnowledgeFixEnabled(true)
	if err != nil {
		t.Fatalf("SetSmallModelKnowledgeFixEnabled(true) failed: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first update")
	}
	if !h.GetSmallModelKnowledgeFixEnabled() {
		t.Fatal("expected knowledge-fix acceleration switch enabled")
	}

	h2 := NewSettingsHandler(store)
	if !h2.GetSmallModelKnowledgeFixEnabled() {
		t.Fatal("expected persisted knowledge-fix acceleration switch enabled")
	}

	changed, err = h2.SetSmallModelKnowledgeFixEnabled(true)
	if err != nil {
		t.Fatalf("SetSmallModelKnowledgeFixEnabled(true) second call failed: %v", err)
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

func TestPatchRejectsRemovedSettingsKeys(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	for _, tc := range []struct {
		name string
		body string
		key  string
	}{
		{
			name: "smart tool selection",
			body: `{"smart_tool_selection":true,"locale":"en-US"}`,
			key:  "smart_tool_selection",
		},
		{
			name: "small model tool dispatch",
			body: `{"small_model_route_tool_dispatch_enabled":true,"locale":"en-US"}`,
			key:  "small_model_route_tool_dispatch_enabled",
		},
		{
			name: "smart skill selection",
			body: `{"smart_skill_selection":false,"locale":"en-US"}`,
			key:  "smart_skill_selection",
		},
		{
			name: "skill dynamic exposure",
			body: `{"skill_dynamic_exposure":false,"locale":"en-US"}`,
			key:  "skill_dynamic_exposure",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := h.Patch(c); err != nil {
				t.Fatalf("Patch failed: %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "locale") {
				t.Fatalf("expected request to fail before applying updates, body=%s", rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.key) {
				t.Fatalf("expected error body to mention %q, got=%s", tc.key, rec.Body.String())
			}
		})
	}
}

func TestUpdateRejectsRemovedSettingsKeys(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)
	e := echo.New()

	for _, tc := range []struct {
		name string
		body string
		key  string
	}{
		{
			name: "small model tool dispatch",
			body: `{"locale":"en-US","small_model_route_tool_dispatch_enabled":true}`,
			key:  "small_model_route_tool_dispatch_enabled",
		},
		{
			name: "smart skill selection",
			body: `{"locale":"en-US","smart_skill_selection":false}`,
			key:  "smart_skill_selection",
		},
		{
			name: "skill dynamic exposure",
			body: `{"locale":"en-US","skill_dynamic_exposure":false}`,
			key:  "skill_dynamic_exposure",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if err := h.Update(c); err != nil {
				t.Fatalf("Update failed: %v", err)
			}
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 body=%s", rec.Code, rec.Body.String())
			}
			if h.settings.Locale != "" {
				t.Fatalf("expected settings to remain unchanged, locale=%q", h.settings.Locale)
			}
			if !strings.Contains(rec.Body.String(), tc.key) {
				t.Fatalf("expected error body to mention removed key, got=%s", rec.Body.String())
			}
		})
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
