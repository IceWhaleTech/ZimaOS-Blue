package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

func TestToolStoreHandlerListToolsHidesCompatFamiliesFromUI(t *testing.T) {
	registry := tools.NewRegistry()

	for _, name := range []string{
		"memory",
		"memory_search",
		"sessions",
		"sessions_list",
		"session_status",
		"web",
		"web_fetch",
		"file_write",
		"write_begin",
		"image",
		"image_generation",
		"generate_image",
		"generateImage",
		"custom_tool",
	} {
		registry.Register(&staticToolMock{
			def: tools.ToolDefinition{
				Name:        name,
				Description: name + " description",
			},
		})
	}

	handler := NewToolStoreHandler(registry)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListTools(c); err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp []ToolResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	got := make(map[string]ToolResponse, len(resp))
	for _, item := range resp {
		got[item.Name] = item
	}

	for _, hidden := range []string{
		"memory",
		"memory_search",
		"sessions_list",
		"session_status",
		"web_fetch",
		"write_begin",
		"image_generation",
		"generate_image",
		"generateImage",
	} {
		if _, ok := got[hidden]; ok {
			t.Fatalf("expected %q to be hidden from tools UI", hidden)
		}
	}

	for _, visible := range []string{
		"sessions",
		"web",
		"file_write",
		"image",
		"custom_tool",
		"mediagen",
	} {
		if _, ok := got[visible]; !ok {
			t.Fatalf("expected %q to remain visible in tools UI", visible)
		}
	}
}
