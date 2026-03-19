package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// MockNativePlugin is a mock native plugin for testing
type MockNativePlugin struct {
	id          string
	manifest    *Manifest
	initialized bool
	started     bool
	stopped     bool
}

func NewMockNativePlugin(id, name string) *MockNativePlugin {
	return &MockNativePlugin{
		id: id,
		manifest: &Manifest{
			ID:           id,
			Name:         name,
			Description:  "A mock plugin for testing",
			Version:      "1.0.0",
			ConfigSchema: map[string]interface{}{},
		},
	}
}

func (p *MockNativePlugin) ID() string {
	return p.id
}

func (p *MockNativePlugin) Manifest() *Manifest {
	return p.manifest
}

func (p *MockNativePlugin) Init(ctx context.Context, api PluginAPI) error {
	p.initialized = true
	return nil
}

func (p *MockNativePlugin) Start(ctx context.Context) error {
	p.started = true
	return nil
}

func (p *MockNativePlugin) Stop(ctx context.Context) error {
	p.stopped = true
	return nil
}

func (p *MockNativePlugin) IsNative() bool {
	return true
}

func TestRegistry_RegisterNativePlugin(t *testing.T) {
	registry := NewRegistry()

	t.Run("register native plugin", func(t *testing.T) {
		plugin := NewMockNativePlugin("test-plugin", "Test Plugin")
		err := registry.RegisterNativePlugin(plugin)
		if err != nil {
			t.Fatalf("failed to register plugin: %v", err)
		}

		info := registry.GetPlugin("test-plugin")
		if info == nil {
			t.Fatal("plugin not found")
		}
		if info.Manifest.ID != "test-plugin" {
			t.Errorf("expected ID 'test-plugin', got '%s'", info.Manifest.ID)
		}
		if !info.IsNative {
			t.Error("expected IsNative to be true")
		}
		if info.Status != StatusLoaded {
			t.Errorf("expected status 'loaded', got '%s'", info.Status)
		}
	})

	t.Run("register duplicate plugin", func(t *testing.T) {
		plugin := NewMockNativePlugin("test-plugin", "Test Plugin 2")
		err := registry.RegisterNativePlugin(plugin)
		if err == nil {
			t.Fatal("expected error for duplicate plugin")
		}
	})
}

func TestRegistry_InitializeAndStartPlugins(t *testing.T) {
	registry := NewRegistry()
	plugin := NewMockNativePlugin("test-plugin", "Test Plugin")
	_ = registry.RegisterNativePlugin(plugin)

	ctx := context.Background()

	t.Run("initialize plugins", func(t *testing.T) {
		err := registry.InitializePlugins(ctx)
		if err != nil {
			t.Fatalf("failed to initialize plugins: %v", err)
		}
		if !plugin.initialized {
			t.Error("plugin should be initialized")
		}
	})

	t.Run("start plugins", func(t *testing.T) {
		err := registry.StartPlugins(ctx)
		if err != nil {
			t.Fatalf("failed to start plugins: %v", err)
		}
		if !plugin.started {
			t.Error("plugin should be started")
		}
	})

	t.Run("stop plugins", func(t *testing.T) {
		err := registry.StopPlugins(ctx)
		if err != nil {
			t.Fatalf("failed to stop plugins: %v", err)
		}
		if !plugin.stopped {
			t.Error("plugin should be stopped")
		}
	})
}

func TestRegistry_ListPlugins(t *testing.T) {
	registry := NewRegistry()

	// Register multiple plugins
	for i := 0; i < 3; i++ {
		plugin := NewMockNativePlugin(
			"test-plugin-"+string(rune('a'+i)),
			"Test Plugin "+string(rune('A'+i)),
		)
		_ = registry.RegisterNativePlugin(plugin)
	}

	plugins := registry.ListPlugins()
	if len(plugins) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(plugins))
	}
}

func TestRegistry_LoadPluginsFromDir(t *testing.T) {
	// Create temp directory with a mock plugin
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "test-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create manifest file
	manifest := `{
		"id": "test-plugin",
		"name": "Test Plugin",
		"description": "A test plugin",
		"version": "1.0.0",
		"configSchema": {}
	}`
	manifestPath := filepath.Join(pluginDir, "clawdbot.plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	registry := NewRegistry()
	ctx := context.Background()

	t.Run("load plugins from directory", func(t *testing.T) {
		err := registry.LoadPluginsFromDir(ctx, tmpDir)
		if err != nil {
			t.Fatalf("failed to load plugins: %v", err)
		}

		info := registry.GetPlugin("test-plugin")
		if info == nil {
			t.Fatal("plugin not found")
		}
		if info.Manifest.Name != "Test Plugin" {
			t.Errorf("expected name 'Test Plugin', got '%s'", info.Manifest.Name)
		}
		if info.Origin != OriginWorkspace {
			t.Errorf("expected origin 'workspace', got '%s'", info.Origin)
		}
	})

	t.Run("load from non-existent directory", func(t *testing.T) {
		err := registry.LoadPluginsFromDir(ctx, "/non/existent/path")
		if err != nil {
			t.Errorf("should not error for non-existent directory: %v", err)
		}
	})
}

func TestPluginAPI_RegisterTool(t *testing.T) {
	registry := NewRegistry()
	plugin := NewMockNativePlugin("test-plugin", "Test Plugin")
	_ = registry.RegisterNativePlugin(plugin)

	api := registry.createPluginAPI("test-plugin")

	t.Run("register tool", func(t *testing.T) {
		tool := Tool{
			Name:        "test-tool",
			Description: "A test tool",
			Parameters:  map[string]interface{}{},
			Handler: func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
				return "result", nil
			},
		}

		err := api.RegisterTool(tool)
		if err != nil {
			t.Fatalf("failed to register tool: %v", err)
		}

		registered := registry.GetTool("test-tool")
		if registered == nil {
			t.Fatal("tool not found")
		}
		if registered.Name != "test-tool" {
			t.Errorf("expected name 'test-tool', got '%s'", registered.Name)
		}
	})

	t.Run("register duplicate tool", func(t *testing.T) {
		tool := Tool{
			Name:        "test-tool",
			Description: "Another test tool",
		}

		err := api.RegisterTool(tool)
		if err == nil {
			t.Fatal("expected error for duplicate tool")
		}
	})

	t.Run("reject invalid schema", func(t *testing.T) {
		err := api.RegisterTool(Tool{
			Name:        "bad-tool",
			Description: "bad",
			Parameters: map[string]interface{}{
				"type":       "object",
				"properties": "not-an-object",
			},
		})
		if err == nil {
			t.Fatal("expected invalid schema error")
		}
	})
}

func TestPluginAPI_RegisterHook(t *testing.T) {
	registry := NewRegistry()
	plugin := NewMockNativePlugin("test-plugin", "Test Plugin")
	_ = registry.RegisterNativePlugin(plugin)

	api := registry.createPluginAPI("test-plugin")

	t.Run("register and trigger hook", func(t *testing.T) {
		triggered := false
		err := api.RegisterHook("test-event", func(ctx context.Context, data interface{}) error {
			triggered = true
			return nil
		})
		if err != nil {
			t.Fatalf("failed to register hook: %v", err)
		}

		ctx := context.Background()
		_ = registry.TriggerHook(ctx, "test-event", nil)

		if !triggered {
			t.Error("hook should have been triggered")
		}
	})
}

func TestPluginAPI_RegisterCommand(t *testing.T) {
	registry := NewRegistry()
	plugin := NewMockNativePlugin("test-plugin", "Test Plugin")
	_ = registry.RegisterNativePlugin(plugin)

	api := registry.createPluginAPI("test-plugin")

	t.Run("register command", func(t *testing.T) {
		cmd := Command{
			Name:        "test-cmd",
			Description: "A test command",
			Handler: func(ctx context.Context, args []string) (string, error) {
				return "executed", nil
			},
		}

		err := api.RegisterCommand(cmd)
		if err != nil {
			t.Fatalf("failed to register command: %v", err)
		}

		registered := registry.GetCommand("test-cmd")
		if registered == nil {
			t.Fatal("command not found")
		}
	})
}

func TestPluginAPI_RegisterHTTPHandler(t *testing.T) {
	registry := NewRegistry()
	plugin := NewMockNativePlugin("test-plugin", "Test Plugin")
	_ = registry.RegisterNativePlugin(plugin)

	api := registry.createPluginAPI("test-plugin")

	t.Run("register HTTP handler", func(t *testing.T) {
		handler := func(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error) {
			return &HTTPResponse{StatusCode: 200}, nil
		}

		err := api.RegisterHTTPHandler("/test", handler)
		if err != nil {
			t.Fatalf("failed to register HTTP handler: %v", err)
		}

		registered := registry.GetHTTPHandler("/test")
		if registered == nil {
			t.Fatal("HTTP handler not found")
		}
	})
}

// MockService is a mock service for testing
type MockService struct {
	name    string
	started bool
	stopped bool
}

func (s *MockService) Name() string {
	return s.name
}

func (s *MockService) Start(ctx context.Context) error {
	s.started = true
	return nil
}

func (s *MockService) Stop(ctx context.Context) error {
	s.stopped = true
	return nil
}

func TestPluginAPI_RegisterService(t *testing.T) {
	registry := NewRegistry()
	plugin := NewMockNativePlugin("test-plugin", "Test Plugin")
	_ = registry.RegisterNativePlugin(plugin)

	api := registry.createPluginAPI("test-plugin")

	t.Run("register and start service", func(t *testing.T) {
		service := &MockService{name: "test-service"}

		err := api.RegisterService(service)
		if err != nil {
			t.Fatalf("failed to register service: %v", err)
		}

		ctx := context.Background()
		_ = registry.StartPlugins(ctx)

		if !service.started {
			t.Error("service should have been started")
		}

		_ = registry.StopPlugins(ctx)

		if !service.stopped {
			t.Error("service should have been stopped")
		}
	})
}
