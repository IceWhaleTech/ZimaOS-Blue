// Package sdk provides a comprehensive SDK for plugin development.
package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/plugin"
)

// BasePlugin provides a base implementation for plugins
type BasePlugin struct {
	id       string
	manifest *plugin.Manifest
	api      plugin.PluginAPI
	config   map[string]interface{}
	mu       sync.RWMutex
}

// NewBasePlugin creates a new base plugin with the given manifest
func NewBasePlugin(manifest *plugin.Manifest) *BasePlugin {
	return &BasePlugin{
		id:       manifest.ID,
		manifest: manifest,
		config:   make(map[string]interface{}),
	}
}

// ID returns the plugin ID
func (p *BasePlugin) ID() string {
	return p.id
}

// Manifest returns the plugin manifest
func (p *BasePlugin) Manifest() *plugin.Manifest {
	return p.manifest
}

// Init initializes the plugin with the given API
func (p *BasePlugin) Init(ctx context.Context, api plugin.PluginAPI) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.api = api
	p.config = api.PluginConfig()
	return nil
}

// Start starts the plugin (override in subclass)
func (p *BasePlugin) Start(ctx context.Context) error {
	return nil
}

// Stop stops the plugin (override in subclass)
func (p *BasePlugin) Stop(ctx context.Context) error {
	return nil
}

// IsNative returns true for native Go plugins
func (p *BasePlugin) IsNative() bool {
	return true
}

// API returns the plugin API
func (p *BasePlugin) API() plugin.PluginAPI {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.api
}

// Config returns the plugin configuration
func (p *BasePlugin) Config() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// GetConfigString gets a string configuration value
func (p *BasePlugin) GetConfigString(key string, defaultValue string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if val, ok := p.config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

// GetConfigInt gets an integer configuration value
func (p *BasePlugin) GetConfigInt(key string, defaultValue int) int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if val, ok := p.config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int64:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return defaultValue
}

// GetConfigBool gets a boolean configuration value
func (p *BasePlugin) GetConfigBool(key string, defaultValue bool) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if val, ok := p.config[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// Logger returns the plugin logger
func (p *BasePlugin) Logger() plugin.Logger {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.api != nil {
		return p.api.Logger()
	}
	return nil
}

// RegisterTool registers a tool with the plugin API
func (p *BasePlugin) RegisterTool(tool plugin.Tool) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.api == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return p.api.RegisterTool(tool)
}

// RegisterHook registers a hook handler
func (p *BasePlugin) RegisterHook(event string, handler plugin.HookHandler) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.api == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return p.api.RegisterHook(event, handler)
}

// RegisterService registers a background service
func (p *BasePlugin) RegisterService(service plugin.Service) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.api == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return p.api.RegisterService(service)
}

// RegisterCommand registers a custom command
func (p *BasePlugin) RegisterCommand(command plugin.Command) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.api == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return p.api.RegisterCommand(command)
}

// RegisterHTTPHandler registers an HTTP handler
func (p *BasePlugin) RegisterHTTPHandler(path string, handler plugin.HTTPHandler) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.api == nil {
		return fmt.Errorf("plugin not initialized")
	}
	return p.api.RegisterHTTPHandler(path, handler)
}

// ManifestBuilder helps build plugin manifests
type ManifestBuilder struct {
	manifest *plugin.Manifest
}

// NewManifestBuilder creates a new manifest builder
func NewManifestBuilder(id, name, version string) *ManifestBuilder {
	return &ManifestBuilder{
		manifest: &plugin.Manifest{
			ID:           id,
			Name:         name,
			Version:      version,
			ConfigSchema: make(map[string]interface{}),
			UIHints:      make(map[string]plugin.UIHint),
		},
	}
}

// Description sets the plugin description
func (b *ManifestBuilder) Description(desc string) *ManifestBuilder {
	b.manifest.Description = desc
	return b
}

// Kind sets the plugin kind
func (b *ManifestBuilder) Kind(kind plugin.PluginKind) *ManifestBuilder {
	b.manifest.Kind = kind
	return b
}

// AddChannel adds a channel to the manifest
func (b *ManifestBuilder) AddChannel(channel string) *ManifestBuilder {
	b.manifest.Channels = append(b.manifest.Channels, channel)
	return b
}

// AddProvider adds a provider to the manifest
func (b *ManifestBuilder) AddProvider(provider string) *ManifestBuilder {
	b.manifest.Providers = append(b.manifest.Providers, provider)
	return b
}

// AddSkill adds a skill to the manifest
func (b *ManifestBuilder) AddSkill(skill string) *ManifestBuilder {
	b.manifest.Skills = append(b.manifest.Skills, skill)
	return b
}

// AddDependency adds a dependency to the manifest
func (b *ManifestBuilder) AddDependency(id, version string, optional bool) *ManifestBuilder {
	b.manifest.Dependencies = append(b.manifest.Dependencies, plugin.Dependency{
		ID:       id,
		Version:  version,
		Optional: optional,
	})
	return b
}

// AddConfigField adds a configuration field with schema
func (b *ManifestBuilder) AddConfigField(name string, schema map[string]interface{}) *ManifestBuilder {
	if b.manifest.ConfigSchema == nil {
		b.manifest.ConfigSchema = make(map[string]interface{})
	}
	properties, ok := b.manifest.ConfigSchema["properties"].(map[string]interface{})
	if !ok {
		properties = make(map[string]interface{})
		b.manifest.ConfigSchema["type"] = "object"
		b.manifest.ConfigSchema["properties"] = properties
	}
	properties[name] = schema
	return b
}

// AddUIHint adds a UI hint for a configuration field
func (b *ManifestBuilder) AddUIHint(field string, hint plugin.UIHint) *ManifestBuilder {
	b.manifest.UIHints[field] = hint
	return b
}

// Build returns the built manifest
func (b *ManifestBuilder) Build() *plugin.Manifest {
	return b.manifest
}

// ConfigValidator validates plugin configuration against schema
type ConfigValidator struct {
	schema map[string]interface{}
}

// NewConfigValidator creates a new config validator
func NewConfigValidator(schema map[string]interface{}) *ConfigValidator {
	return &ConfigValidator{schema: schema}
}

// Validate validates the configuration against the schema
func (v *ConfigValidator) Validate(config map[string]interface{}) error {
	if v.schema == nil {
		return nil
	}

	properties, ok := v.schema["properties"].(map[string]interface{})
	if !ok {
		return nil
	}

	required, _ := v.schema["required"].([]interface{})
	requiredSet := make(map[string]bool)
	for _, r := range required {
		if s, ok := r.(string); ok {
			requiredSet[s] = true
		}
	}

	// Check required fields
	for field := range requiredSet {
		if _, ok := config[field]; !ok {
			return fmt.Errorf("required field '%s' is missing", field)
		}
	}

	// Validate field types
	for field, value := range config {
		fieldSchema, ok := properties[field].(map[string]interface{})
		if !ok {
			continue
		}

		expectedType, _ := fieldSchema["type"].(string)
		if err := v.validateType(field, value, expectedType); err != nil {
			return err
		}
	}

	return nil
}

func (v *ConfigValidator) validateType(field string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field '%s' must be a string", field)
		}
	case "number", "integer":
		switch value.(type) {
		case int, int64, float64:
			// OK
		default:
			return fmt.Errorf("field '%s' must be a number", field)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field '%s' must be a boolean", field)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("field '%s' must be an array", field)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("field '%s' must be an object", field)
		}
	}
	return nil
}

// ToolBuilder helps build plugin tools
type ToolBuilder struct {
	tool plugin.Tool
}

// NewToolBuilder creates a new tool builder
func NewToolBuilder(name, description string) *ToolBuilder {
	return &ToolBuilder{
		tool: plugin.Tool{
			Name:        name,
			Description: description,
			Parameters:  make(map[string]interface{}),
		},
	}
}

// AddParameter adds a parameter to the tool
func (b *ToolBuilder) AddParameter(name, paramType, description string, required bool) *ToolBuilder {
	properties, ok := b.tool.Parameters["properties"].(map[string]interface{})
	if !ok {
		properties = make(map[string]interface{})
		b.tool.Parameters["type"] = "object"
		b.tool.Parameters["properties"] = properties
	}

	properties[name] = map[string]interface{}{
		"type":        paramType,
		"description": description,
	}

	if required {
		requiredList, _ := b.tool.Parameters["required"].([]string)
		b.tool.Parameters["required"] = append(requiredList, name)
	}

	return b
}

// Handler sets the tool handler
func (b *ToolBuilder) Handler(handler plugin.ToolHandler) *ToolBuilder {
	b.tool.Handler = handler
	return b
}

// Build returns the built tool
func (b *ToolBuilder) Build() plugin.Tool {
	return b.tool
}

// EventEmitter provides event emission capabilities
type EventEmitter struct {
	api plugin.PluginAPI
}

// NewEventEmitter creates a new event emitter
func NewEventEmitter(api plugin.PluginAPI) *EventEmitter {
	return &EventEmitter{api: api}
}

// Emit emits an event (triggers hooks)
func (e *EventEmitter) Emit(ctx context.Context, event string, data interface{}) error {
	// Events are handled through the hook system
	// This is a convenience wrapper
	return nil
}

// JSONHelper provides JSON utilities for plugins
type JSONHelper struct{}

// Marshal marshals data to JSON
func (h *JSONHelper) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal unmarshals JSON to data
func (h *JSONHelper) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// MarshalIndent marshals data to indented JSON
func (h *JSONHelper) MarshalIndent(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
