package plugin

import (
	"context"
)

// PluginKind represents the type of plugin
type PluginKind string

const (
	PluginKindGeneral PluginKind = ""
	PluginKindMemory  PluginKind = "memory"
	PluginKindChannel PluginKind = "channel"
	PluginKindTool    PluginKind = "tool"
)

// PluginOrigin represents where the plugin was loaded from
type PluginOrigin string

const (
	OriginBundled   PluginOrigin = "bundled"
	OriginWorkspace PluginOrigin = "workspace"
	OriginConfig    PluginOrigin = "config"
	OriginNative    PluginOrigin = "native" // Go native plugins
)

// PluginStatus represents the current status of a plugin
type PluginStatus string

const (
	StatusLoaded   PluginStatus = "loaded"
	StatusDisabled PluginStatus = "disabled"
	StatusError    PluginStatus = "error"
)

// Manifest represents the plugin manifest (clawdbot.plugin.json compatible)
type Manifest struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Version      string                 `json:"version"`
	Kind         PluginKind             `json:"kind,omitempty"`
	ConfigSchema map[string]interface{} `json:"configSchema"`
	Channels     []string               `json:"channels,omitempty"`
	Providers    []string               `json:"providers,omitempty"`
	Skills       []string               `json:"skills,omitempty"`
	UIHints      map[string]UIHint      `json:"uiHints,omitempty"`
}

// UIHint provides UI hints for configuration fields
type UIHint struct {
	Label     string `json:"label,omitempty"`
	Help      string `json:"help,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
	Advanced  bool   `json:"advanced,omitempty"`
}

// PluginInfo contains metadata about a loaded plugin
type PluginInfo struct {
	Manifest   *Manifest              `json:"manifest"`
	Origin     PluginOrigin           `json:"origin"`
	Status     PluginStatus           `json:"status"`
	Path       string                 `json:"path"`
	Config     map[string]interface{} `json:"config,omitempty"`
	Error      string                 `json:"error,omitempty"`
	IsNative   bool                   `json:"is_native"`
}

// Plugin is the interface that all plugins must implement
type Plugin interface {
	// ID returns the unique identifier of the plugin
	ID() string

	// Manifest returns the plugin manifest
	Manifest() *Manifest

	// Init initializes the plugin with the given API
	Init(ctx context.Context, api PluginAPI) error

	// Start starts the plugin
	Start(ctx context.Context) error

	// Stop stops the plugin
	Stop(ctx context.Context) error
}

// NativePlugin is the interface for Go native plugins
type NativePlugin interface {
	Plugin

	// IsNative returns true for native Go plugins
	IsNative() bool
}

// PluginAPI is the API provided to plugins for registration
type PluginAPI interface {
	// Plugin metadata
	PluginID() string
	PluginConfig() map[string]interface{}

	// Tool registration
	RegisterTool(tool Tool) error

	// Hook registration
	RegisterHook(event string, handler HookHandler) error

	// Service registration
	RegisterService(service Service) error

	// Command registration
	RegisterCommand(command Command) error

	// HTTP handler registration
	RegisterHTTPHandler(path string, handler HTTPHandler) error

	// Logger
	Logger() Logger
}

// Tool represents an agent tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Handler     ToolHandler            `json:"-"`
}

// ToolHandler is the function signature for tool handlers
type ToolHandler func(ctx context.Context, params map[string]interface{}) (interface{}, error)

// HookHandler is the function signature for hook handlers
type HookHandler func(ctx context.Context, data interface{}) error

// Service represents a background service
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Command represents a custom command
type Command struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Handler     CommandHandler `json:"-"`
}

// CommandHandler is the function signature for command handlers
type CommandHandler func(ctx context.Context, args []string) (string, error)

// HTTPHandler is the function signature for HTTP handlers
type HTTPHandler func(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)

// HTTPRequest represents an HTTP request
type HTTPRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
}

// HTTPResponse represents an HTTP response
type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// Logger interface for plugin logging
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}
