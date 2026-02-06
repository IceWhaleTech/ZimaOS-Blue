package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSBridge_LoadJSPlugin_Disabled(t *testing.T) {
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

	t.Run("JS plugin loading is disabled", func(t *testing.T) {
		err := bridge.LoadJSPlugin(ctx, pluginDir, OriginWorkspace)
		if err == nil {
			t.Fatal("expected error since JS support is disabled")
		}
		if !strings.Contains(err.Error(), "disabled") {
			t.Errorf("expected error about JS being disabled, got: %v", err)
		}
	})
}

func TestJSBridge_Close(t *testing.T) {
	bridge := NewJSBridge(nil)
	// Close should not panic
	bridge.Close()
}
