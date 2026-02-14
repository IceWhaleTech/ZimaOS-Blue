package plugin

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
)

// JSBridge handles loading and executing JavaScript/TypeScript plugins
// Note: JavaScript plugin support is disabled to reduce binary size.
// Use native Go plugins instead.
type JSBridge struct {
	registry *Registry
}

// NewJSBridge creates a new JavaScript bridge
func NewJSBridge(registry *Registry) *JSBridge {
	return &JSBridge{
		registry: registry,
	}
}

// LoadJSPlugin loads a JavaScript plugin from a path
// Note: JavaScript plugin support is disabled to reduce binary size.
func (b *JSBridge) LoadJSPlugin(ctx context.Context, pluginPath string, origin PluginOrigin) error {
	logger.Warn().
		Str("path", pluginPath).
		Msg("JavaScript plugin support is disabled. Use native Go plugins instead.")
	return fmt.Errorf("JavaScript plugin support is disabled to reduce binary size")
}

// Close closes all VMs
func (b *JSBridge) Close() {
	// No-op: JavaScript support disabled
}
