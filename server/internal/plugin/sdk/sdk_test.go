package sdk

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
)

func TestBasePlugin(t *testing.T) {
	manifest := &plugin.Manifest{
		ID:          "test-plugin",
		Name:        "Test Plugin",
		Version:     "1.0.0",
		Description: "A test plugin",
	}

	p := NewBasePlugin(manifest)

	if p.ID() != "test-plugin" {
		t.Errorf("expected ID 'test-plugin', got '%s'", p.ID())
	}

	if p.Manifest().Name != "Test Plugin" {
		t.Errorf("expected name 'Test Plugin', got '%s'", p.Manifest().Name)
	}

	if !p.IsNative() {
		t.Error("expected IsNative() to return true")
	}
}

func TestManifestBuilder(t *testing.T) {
	manifest := NewManifestBuilder("my-plugin", "My Plugin", "1.0.0").
		Description("A sample plugin").
		Kind(plugin.PluginKindTool).
		AddChannel("telegram").
		AddProvider("openai").
		AddSkill("calculator").
		AddDependency("core", "^1.0.0", false).
		AddConfigField("apiKey", map[string]interface{}{
			"type":        "string",
			"description": "API key for the service",
		}).
		AddUIHint("apiKey", plugin.UIHint{
			Label:     "API Key",
			Help:      "Enter your API key",
			Sensitive: true,
		}).
		Build()

	if manifest.ID != "my-plugin" {
		t.Errorf("expected ID 'my-plugin', got '%s'", manifest.ID)
	}

	if manifest.Kind != plugin.PluginKindTool {
		t.Errorf("expected kind 'tool', got '%s'", manifest.Kind)
	}

	if len(manifest.Channels) != 1 || manifest.Channels[0] != "telegram" {
		t.Errorf("expected channels ['telegram'], got %v", manifest.Channels)
	}

	if len(manifest.Dependencies) != 1 {
		t.Errorf("expected 1 dependency, got %d", len(manifest.Dependencies))
	}

	if hint, ok := manifest.UIHints["apiKey"]; !ok || !hint.Sensitive {
		t.Error("expected sensitive UI hint for apiKey")
	}
}

func TestConfigValidator(t *testing.T) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
			},
			"count": map[string]interface{}{
				"type": "integer",
			},
			"enabled": map[string]interface{}{
				"type": "boolean",
			},
		},
		"required": []interface{}{"name"},
	}

	validator := NewConfigValidator(schema)

	// Valid config
	validConfig := map[string]interface{}{
		"name":    "test",
		"count":   10,
		"enabled": true,
	}
	if err := validator.Validate(validConfig); err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}

	// Missing required field
	invalidConfig := map[string]interface{}{
		"count": 10,
	}
	if err := validator.Validate(invalidConfig); err == nil {
		t.Error("expected error for missing required field")
	}

	// Wrong type
	wrongTypeConfig := map[string]interface{}{
		"name":  123, // should be string
		"count": 10,
	}
	if err := validator.Validate(wrongTypeConfig); err == nil {
		t.Error("expected error for wrong type")
	}
}

func TestToolBuilder(t *testing.T) {
	handler := func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
		return "result", nil
	}

	tool := NewToolBuilder("my-tool", "A sample tool").
		AddParameter("input", "string", "Input value", true).
		AddParameter("count", "integer", "Count value", false).
		Handler(handler).
		Build()

	if tool.Name != "my-tool" {
		t.Errorf("expected name 'my-tool', got '%s'", tool.Name)
	}

	if tool.Description != "A sample tool" {
		t.Errorf("expected description 'A sample tool', got '%s'", tool.Description)
	}

	if tool.Handler == nil {
		t.Error("expected handler to be set")
	}

	properties, ok := tool.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties in parameters")
	}

	if _, ok := properties["input"]; !ok {
		t.Error("expected 'input' parameter")
	}

	if _, ok := properties["count"]; !ok {
		t.Error("expected 'count' parameter")
	}
}

func TestBasePluginConfig(t *testing.T) {
	manifest := &plugin.Manifest{
		ID:      "config-test",
		Name:    "Config Test",
		Version: "1.0.0",
	}

	p := NewBasePlugin(manifest)

	// Test default values when not initialized
	if p.GetConfigString("key", "default") != "default" {
		t.Error("expected default string value")
	}

	if p.GetConfigInt("key", 42) != 42 {
		t.Error("expected default int value")
	}

	if p.GetConfigBool("key", true) != true {
		t.Error("expected default bool value")
	}
}

func TestJSONHelper(t *testing.T) {
	helper := &JSONHelper{}

	data := map[string]interface{}{
		"name":  "test",
		"count": 10,
	}

	// Test Marshal
	jsonBytes, err := helper.Marshal(data)
	if err != nil {
		t.Errorf("Marshal failed: %v", err)
	}

	// Test Unmarshal
	var result map[string]interface{}
	if err := helper.Unmarshal(jsonBytes, &result); err != nil {
		t.Errorf("Unmarshal failed: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("expected name 'test', got '%v'", result["name"])
	}

	// Test MarshalIndent
	indentedBytes, err := helper.MarshalIndent(data)
	if err != nil {
		t.Errorf("MarshalIndent failed: %v", err)
	}

	if len(indentedBytes) <= len(jsonBytes) {
		t.Error("expected indented JSON to be longer")
	}
}
