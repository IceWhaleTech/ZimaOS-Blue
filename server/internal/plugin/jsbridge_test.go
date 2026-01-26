package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSBridge_LoadJSPlugin(t *testing.T) {
	// Create temp directory with a mock JS plugin
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "test-js-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	// Create manifest file
	manifest := `{
		"id": "test-js-plugin",
		"name": "Test JS Plugin",
		"description": "A test JavaScript plugin",
		"version": "1.0.0",
		"configSchema": {}
	}`
	manifestPath := filepath.Join(pluginDir, "clawdbot.plugin.json")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Create plugin code
	pluginCode := `
// Simple plugin that registers a tool
function register(api) {
	console.log("Plugin initializing...");

	api.registerTool({
		name: "test-js-tool",
		description: "A test tool from JS",
		parameters: {},
		handler: function(params) {
			return "Hello from JS!";
		}
	});

	api.on("test-event", function(data) {
		console.log("Received event:", data);
	});

	api.logger.info("Plugin initialized");
}

module.exports = register;
`
	codePath := filepath.Join(pluginDir, "index.js")
	if err := os.WriteFile(codePath, []byte(pluginCode), 0644); err != nil {
		t.Fatalf("failed to write plugin code: %v", err)
	}

	registry := NewRegistry()
	bridge := NewJSBridge(registry)
	defer bridge.Close()

	ctx := context.Background()

	t.Run("load JS plugin", func(t *testing.T) {
		err := bridge.LoadJSPlugin(ctx, pluginDir, OriginWorkspace)
		if err != nil {
			t.Fatalf("failed to load JS plugin: %v", err)
		}

		// Check plugin was registered
		info := registry.GetPlugin("test-js-plugin")
		if info == nil {
			t.Fatal("plugin not found")
		}
		if info.Manifest.Name != "Test JS Plugin" {
			t.Errorf("expected name 'Test JS Plugin', got '%s'", info.Manifest.Name)
		}

		// Check tool was registered
		tool := registry.GetTool("test-js-tool")
		if tool == nil {
			t.Fatal("tool not found")
		}
		if tool.Description != "A test tool from JS" {
			t.Errorf("expected description 'A test tool from JS', got '%s'", tool.Description)
		}
	})
}

func TestJSBridge_LoadPluginWithDefaultExport(t *testing.T) {
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "default-export-plugin")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatalf("failed to create plugin dir: %v", err)
	}

	manifest := `{
		"id": "default-export-plugin",
		"name": "Default Export Plugin",
		"version": "1.0.0",
		"configSchema": {}
	}`
	if err := os.WriteFile(filepath.Join(pluginDir, "clawdbot.plugin.json"), []byte(manifest), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	// Plugin with object export and register method
	pluginCode := `
var plugin = {
	id: "default-export-plugin",
	name: "Default Export Plugin",
	register: function(api) {
		api.registerCommand({
			name: "test-cmd",
			description: "A test command",
			handler: function(args) {
				return "Command executed";
			}
		});
	}
};

exports.default = plugin;
`
	if err := os.WriteFile(filepath.Join(pluginDir, "index.js"), []byte(pluginCode), 0644); err != nil {
		t.Fatalf("failed to write plugin code: %v", err)
	}

	registry := NewRegistry()
	bridge := NewJSBridge(registry)
	defer bridge.Close()

	ctx := context.Background()

	t.Run("load plugin with default export", func(t *testing.T) {
		err := bridge.LoadJSPlugin(ctx, pluginDir, OriginWorkspace)
		if err != nil {
			t.Fatalf("failed to load plugin: %v", err)
		}

		// Check command was registered
		cmd := registry.GetCommand("test-cmd")
		if cmd == nil {
			t.Fatal("command not found")
		}
	})
}

func TestJSBridge_FindEntryPoint(t *testing.T) {
	bridge := NewJSBridge(nil)

	t.Run("find index.js", func(t *testing.T) {
		tmpDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(tmpDir, "index.js"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		entry, err := bridge.findEntryPoint(tmpDir)
		if err != nil {
			t.Fatalf("failed to find entry point: %v", err)
		}
		if filepath.Base(entry) != "index.js" {
			t.Errorf("expected index.js, got %s", filepath.Base(entry))
		}
	})

	t.Run("find from package.json", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create package.json with clawdbot.extensions
		pkgJSON := `{
			"name": "test-plugin",
			"clawdbot": {
				"extensions": ["./src/main.js"]
			}
		}`
		if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
			t.Fatal(err)
		}

		// Create the entry point
		srcDir := filepath.Join(tmpDir, "src")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "main.js"), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		entry, err := bridge.findEntryPoint(tmpDir)
		if err != nil {
			t.Fatalf("failed to find entry point: %v", err)
		}
		if filepath.Base(entry) != "main.js" {
			t.Errorf("expected main.js, got %s", filepath.Base(entry))
		}
	})

	t.Run("no entry point", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := bridge.findEntryPoint(tmpDir)
		if err == nil {
			t.Fatal("expected error for missing entry point")
		}
	})
}

func TestJSBridge_TranspileTypeScript(t *testing.T) {
	bridge := NewJSBridge(nil)

	t.Run("remove import type", func(t *testing.T) {
		code := `
import type { SomeType } from 'some-module';
import { something } from 'other-module';

const x = 1;
`
		result := bridge.transpileTypeScript(code)

		if strings.Contains(result, "import type") {
			t.Error("import type should be removed")
		}
		if !strings.Contains(result, "import { something }") {
			t.Error("regular import should be preserved")
		}
	})
}
