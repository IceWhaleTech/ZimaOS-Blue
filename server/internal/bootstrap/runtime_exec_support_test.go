package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func TestNewRuntimeExecSelectionOptions_UsesDefaultsWithoutSettings(t *testing.T) {
	opts := newRuntimeExecSelectionOptions(nil)

	if opts.Mode != agentcore.SkillSelectorModeHybrid || !opts.EnableRerank || opts.ConfidenceThreshold != 0.78 {
		t.Fatalf("options=%#v, want hybrid/default rerank/default threshold", opts)
	}
}

func TestNewRuntimeExecSelectionOptions_UsesSettingsOverrides(t *testing.T) {
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{
		"skill_selector_mode":"ir_only",
		"skill_rerank_enabled":true,
		"skill_selector_confidence_threshold":0.42
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	if err := settings.Patch(c); err != nil {
		t.Fatalf("Patch settings: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Patch settings status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	opts := newRuntimeExecSelectionOptions(settings)
	if opts.Mode != agentcore.SkillSelectorModeIROnly || !opts.EnableRerank || opts.ConfidenceThreshold != 0.42 {
		t.Fatalf("options=%#v, want ir_only/rerank enabled/threshold override", opts)
	}
}
