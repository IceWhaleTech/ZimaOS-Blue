package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

// mockNativePlugin is a mock native plugin for testing
type mockNativePlugin struct {
	id       string
	manifest *plugin.Manifest
}

func newMockNativePlugin(id, name string) *mockNativePlugin {
	return &mockNativePlugin{
		id: id,
		manifest: &plugin.Manifest{
			ID:           id,
			Name:         name,
			Description:  "A mock plugin for testing",
			Version:      "1.0.0",
			ConfigSchema: map[string]interface{}{},
		},
	}
}

func (p *mockNativePlugin) ID() string {
	return p.id
}

func (p *mockNativePlugin) Manifest() *plugin.Manifest {
	return p.manifest
}

func (p *mockNativePlugin) Init(ctx context.Context, api plugin.PluginAPI) error {
	return nil
}

func (p *mockNativePlugin) Start(ctx context.Context) error {
	return nil
}

func (p *mockNativePlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *mockNativePlugin) IsNative() bool {
	return true
}

func TestPluginHandler_DisablePlugin(t *testing.T) {
	t.Run("disable native plugin successfully", func(t *testing.T) {
		e := echo.New()
		registry := plugin.NewRegistry()

		// Register a native plugin
		nativePlugin := newMockNativePlugin("native-test", "Native Test Plugin")
		err := registry.RegisterNativePlugin(nativePlugin)
		assert.NoError(t, err)

		h := NewPluginHandler(registry)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/native-test/disable", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("native-test")

		err = h.DisablePlugin(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"success":true`)
	})

	t.Run("disable javascript plugin should not fail", func(t *testing.T) {
		e := echo.New()
		registry := plugin.NewRegistry()

		// Simulate a JavaScript plugin by creating a temporary plugin directory
		// with a manifest file, then loading it
		tmpDir := t.TempDir()
		pluginDir := tmpDir + "/js-test"
		err := os.MkdirAll(pluginDir, 0755)
		assert.NoError(t, err)

		// Create a manifest file
		manifest := `{
			"id": "js-test",
			"name": "JavaScript Test Plugin",
			"description": "A test JavaScript plugin",
			"version": "1.0.0",
			"configSchema": {}
		}`
		err = os.WriteFile(pluginDir+"/clawdbot.plugin.json", []byte(manifest), 0644)
		assert.NoError(t, err)

		// Load the plugin
		err = registry.LoadPluginsFromDir(nil, tmpDir)
		assert.NoError(t, err)

		// Verify the plugin was loaded
		pluginInfo := registry.GetPlugin("js-test")
		assert.NotNil(t, pluginInfo)
		assert.False(t, pluginInfo.IsNative)

		h := NewPluginHandler(registry)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/js-test/disable", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("js-test")

		err = h.DisablePlugin(c)
		// This should NOT return an error for JavaScript plugins
		// Currently it fails with "plugin js-test not found or not a native plugin"
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"success":true`)
	})

	t.Run("disable non-existent plugin returns 404", func(t *testing.T) {
		e := echo.New()
		registry := plugin.NewRegistry()
		h := NewPluginHandler(registry)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/non-existent/disable", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("non-existent")

		err := h.DisablePlugin(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error":"plugin not found"`)
	})
}

func TestPluginHandler_EnablePlugin(t *testing.T) {
	t.Run("enable native plugin successfully", func(t *testing.T) {
		e := echo.New()
		registry := plugin.NewRegistry()

		// Register a native plugin
		nativePlugin := newMockNativePlugin("native-test", "Native Test Plugin")
		err := registry.RegisterNativePlugin(nativePlugin)
		assert.NoError(t, err)

		h := NewPluginHandler(registry)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/native-test/enable", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("native-test")

		err = h.EnablePlugin(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"success":true`)
	})

	t.Run("enable javascript plugin should not fail", func(t *testing.T) {
		e := echo.New()
		registry := plugin.NewRegistry()

		// Simulate a JavaScript plugin by creating a temporary plugin directory
		tmpDir := t.TempDir()
		pluginDir := tmpDir + "/js-test"
		err := os.MkdirAll(pluginDir, 0755)
		assert.NoError(t, err)

		// Create a manifest file
		manifest := `{
			"id": "js-test",
			"name": "JavaScript Test Plugin",
			"description": "A test JavaScript plugin",
			"version": "1.0.0",
			"configSchema": {}
		}`
		err = os.WriteFile(pluginDir+"/clawdbot.plugin.json", []byte(manifest), 0644)
		assert.NoError(t, err)

		// Load the plugin
		err = registry.LoadPluginsFromDir(nil, tmpDir)
		assert.NoError(t, err)

		// Verify the plugin was loaded
		pluginInfo := registry.GetPlugin("js-test")
		assert.NotNil(t, pluginInfo)
		assert.False(t, pluginInfo.IsNative)

		h := NewPluginHandler(registry)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/js-test/enable", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("js-test")

		err = h.EnablePlugin(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"success":true`)
	})

	t.Run("enable non-existent plugin returns 404", func(t *testing.T) {
		e := echo.New()
		registry := plugin.NewRegistry()
		h := NewPluginHandler(registry)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins/non-existent/enable", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues("non-existent")

		err := h.EnablePlugin(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error":"plugin not found"`)
	})
}
