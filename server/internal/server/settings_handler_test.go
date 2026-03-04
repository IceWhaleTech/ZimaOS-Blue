package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
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

func TestGetSkillRerankEnabled_DefaultTrue(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if !h.GetSkillRerankEnabled() {
		t.Fatalf("GetSkillRerankEnabled() = false, want true")
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
			name:                      "default enabled",
			wantEffectiveRerankEnable: true,
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

func TestGetSmallModelDefaults(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	if h.GetSmallModelEnabled() {
		t.Fatal("GetSmallModelEnabled() = true, want false")
	}
	if got := h.GetSmallModelRuntime(); got != "onnx_genai_python" {
		t.Fatalf("GetSmallModelRuntime() = %q, want onnx_genai_python", got)
	}
	if got := h.GetSmallModelID(); got != "qwen3.5-0.8b-onnx-q4" {
		t.Fatalf("GetSmallModelID() = %q, want qwen3.5-0.8b-onnx-q4", got)
	}
	if !h.GetSmallModelAutoDownload() {
		t.Fatal("GetSmallModelAutoDownload() = false, want true")
	}
	if !h.GetSmallModelSummaryEnabled() || !h.GetSmallModelDocExtractEnabled() || !h.GetSmallModelRerankEnabled() || !h.GetSmallModelContextPruneEnabled() {
		t.Fatal("expected phase1 enhancement switches default true")
	}
	if !h.GetSmallModelMediaIntentEnabled() {
		t.Fatal("expected media intent switch default true")
	}
	if !h.GetSmallModelRouteShortQAEnabled() || !h.GetSmallModelRouteToolDispatchEnabled() {
		t.Fatal("expected short-qa/tool-dispatch route switches default true")
	}
	if !h.GetDeepResearchV2Enabled() {
		t.Fatal("expected deep research v2 switch default true")
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

func TestSoulProposalLifecycleAndPersistence(t *testing.T) {
	store := kvstore.NewMemoryStore()
	h := NewSettingsHandler(store)

	first, err := h.AddSoulProposal("First", "First content", "unit")
	if err != nil {
		t.Fatalf("AddSoulProposal first failed: %v", err)
	}
	second, err := h.AddSoulProposal("Second", "Second content", "unit")
	if err != nil {
		t.Fatalf("AddSoulProposal second failed: %v", err)
	}
	// Force deterministic sort order for list assertions.
	h.mu.Lock()
	h.soulProposals[first.ID].CreatedAt = time.Now().UTC().Add(-1 * time.Hour)
	h.soulProposals[second.ID].CreatedAt = time.Now().UTC()
	if err := h.saveSoulProposalsLocked(); err != nil {
		h.mu.Unlock()
		t.Fatalf("saveSoulProposalsLocked failed: %v", err)
	}
	h.mu.Unlock()

	e := echo.New()
	listReq := httptest.NewRequest(http.MethodGet, "/api/settings/soul/proposals", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	if err := h.ListSoulProposals(listCtx); err != nil {
		t.Fatalf("ListSoulProposals failed: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200", listRec.Code)
	}
	var listResp struct {
		Proposals []SoulProposal `json:"proposals"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode proposals list: %v", err)
	}
	if len(listResp.Proposals) != 2 {
		t.Fatalf("proposal count = %d, want 2", len(listResp.Proposals))
	}
	if listResp.Proposals[0].ID != second.ID {
		t.Fatalf("expected latest proposal first, got %q want %q", listResp.Proposals[0].ID, second.ID)
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/settings/soul/proposals/"+first.ID+"/approve", nil)
	approveRec := httptest.NewRecorder()
	approveCtx := e.NewContext(approveReq, approveRec)
	approveCtx.SetParamNames("id")
	approveCtx.SetParamValues(first.ID)
	if err := h.ApproveSoulProposal(approveCtx); err != nil {
		t.Fatalf("ApproveSoulProposal failed: %v", err)
	}
	if approveRec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, want 200", approveRec.Code)
	}
	var approved SoulProposal
	if err := json.Unmarshal(approveRec.Body.Bytes(), &approved); err != nil {
		t.Fatalf("decode approved proposal: %v", err)
	}
	if approved.Status != "approved" || approved.ReviewedAt == nil {
		t.Fatalf("unexpected approved proposal: %+v", approved)
	}

	h2 := NewSettingsHandler(store)
	if got, ok := h2.soulProposals[first.ID]; !ok || got.Status != "approved" {
		t.Fatalf("expected approved proposal persisted, got exists=%v value=%+v", ok, got)
	}
}

func TestReviewSoulProposal_NotFound(t *testing.T) {
	h := NewSettingsHandler(kvstore.NewMemoryStore())
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/settings/soul/proposals/not-found/reject", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("not-found")

	if err := h.RejectSoulProposal(c); err != nil {
		t.Fatalf("RejectSoulProposal failed: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
