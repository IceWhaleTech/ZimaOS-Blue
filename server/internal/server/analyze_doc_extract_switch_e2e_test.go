package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type analyzeBridgeCapture struct {
	calls    []string
	response string
}

func (b *analyzeBridgeCapture) Chat(_ context.Context, prompt string, _ int) (string, error) {
	b.calls = append(b.calls, prompt)
	if strings.TrimSpace(b.response) == "" {
		return `{"summary":"ok","stats":[],"themes":[],"quotes":[],"insights":[],"recommendations":[]}`, nil
	}
	return b.response, nil
}

func TestAnalyzeDocExtractSwitchE2E_UsesSettingsToggle(t *testing.T) {
	settings := NewSettingsHandler(kvstore.NewMemoryStore())

	patch := func(t *testing.T, body string) {
		t.Helper()
		e := echo.New()
		req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		if err := settings.Patch(e.NewContext(req, rec)); err != nil {
			t.Fatalf("settings Patch() error = %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("settings Patch() status=%d, want 200", rec.Code)
		}
	}

	bridge := &analyzeBridgeCapture{}
	sm := &smallModelRuntimeMock{
		respText: "Objective: summarize\nInputs: text\nSteps: extract\nOutputs: summary\nRisks: missing context",
	}

	tool := tools.NewAnalyzeTool()
	tool.SetLLMBridge(bridge)
	tool.SetMediaDir(t.TempDir())
	tool.SetSmallModelRuntime(sm)
	tool.SetSmallModelSwitchFuncs(
		func() bool { return settings.GetSmallModelEnabled() },
		func() bool { return settings.GetSmallModelDocExtractEnabled() },
	)

	args := map[string]interface{}{
		"topic": "Doc Extract E2E",
		"text":  strings.Repeat("content line\n", 20),
		"lang":  "en-US",
	}

	t.Run("disabled: does not call small model and does not inject extraction block", func(t *testing.T) {
		sm.calls = 0
		bridge.calls = nil
		patch(t, `{"small_model_enabled":true,"small_model_doc_extract_enabled":false}`)

		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("AnalyzeTool Execute: %v", err)
		}
		if sm.calls != 0 {
			t.Fatalf("small model calls=%d, want 0 when small_model_doc_extract_enabled=false", sm.calls)
		}
		if len(bridge.calls) != 1 {
			t.Fatalf("bridge calls=%d, want 1", len(bridge.calls))
		}
		if strings.Contains(bridge.calls[0], "=== Small-model structured extraction ===") {
			t.Fatalf("expected no extraction block when disabled, got prompt=%q", bridge.calls[0])
		}
	})

	t.Run("enabled: calls small model and injects extraction block into analysis prompt", func(t *testing.T) {
		sm.calls = 0
		bridge.calls = nil
		patch(t, `{"small_model_enabled":true,"small_model_doc_extract_enabled":true}`)

		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("AnalyzeTool Execute: %v", err)
		}
		if sm.calls != 1 {
			t.Fatalf("small model calls=%d, want 1 when enabled", sm.calls)
		}
		if len(bridge.calls) != 1 {
			t.Fatalf("bridge calls=%d, want 1", len(bridge.calls))
		}
		if !strings.Contains(bridge.calls[0], "=== Small-model structured extraction ===") {
			t.Fatalf("expected extraction block when enabled, got prompt=%q", bridge.calls[0])
		}
	})
}
