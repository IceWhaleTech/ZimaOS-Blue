package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/labstack/echo/v4"
)

func TestMgmtSettingsAdapterGetAllIncludesKnowledgeFixToggle(t *testing.T) {
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	e := echo.New()
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/settings",
		strings.NewReader(`{"small_model_knowledge_fix_enabled":true}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	if err := settings.Patch(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Patch() error = %v", err)
	}

	adapter := &mgmtSettingsAdapter{handler: settings}
	values, err := adapter.GetAll(context.Background())
	if err != nil {
		t.Fatalf("GetAll() error = %v", err)
	}
	if got, ok := values["small_model_knowledge_fix_enabled"].(bool); !ok || !got {
		t.Fatalf("small_model_knowledge_fix_enabled = %#v, want true", values["small_model_knowledge_fix_enabled"])
	}
}

func TestMgmtSettingsAdapterSetAllowsKnowledgeFixToggleKey(t *testing.T) {
	adapter := &mgmtSettingsAdapter{handler: serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())}

	err := adapter.Set(context.Background(), "small_model_knowledge_fix_enabled", "true")
	if err == nil {
		t.Fatal("expected error because tool-based settings.set is not wired")
	}
	if strings.Contains(err.Error(), "unknown setting key") {
		t.Fatalf("Set() should accept knowledge-fix key in allowlist, got %v", err)
	}
}
