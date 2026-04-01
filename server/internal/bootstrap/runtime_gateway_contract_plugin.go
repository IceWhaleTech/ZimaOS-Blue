package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type pluginToolAdapter struct {
	registry *plugin.Registry
	name     string
}

func (t *pluginToolAdapter) Definition() tools.ToolDefinition {
	if t == nil || t.registry == nil {
		return tools.ToolDefinition{Name: strings.TrimSpace(t.name)}
	}
	tool := t.registry.GetTool(strings.TrimSpace(t.name))
	if tool == nil {
		return tools.ToolDefinition{Name: strings.TrimSpace(t.name)}
	}
	return tools.ToolDefinition{
		Name:                strings.TrimSpace(tool.Name),
		Description:         strings.TrimSpace(tool.Description),
		Icon:                "extension",
		Parameters:          tool.Parameters,
		RiskLevel:           strings.TrimSpace(tool.RiskLevel),
		VisibilityAllowlist: append([]string(nil), tool.VisibilityAllowlist...),
	}
}

func (t *pluginToolAdapter) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.registry == nil {
		return nil, fmt.Errorf("plugin tool registry is not configured")
	}
	tool := t.registry.GetTool(strings.TrimSpace(t.name))
	if tool == nil || tool.Handler == nil {
		return nil, fmt.Errorf("plugin tool %q is not available", strings.TrimSpace(t.name))
	}
	result, err := tool.Handler(ctx, args)
	if err != nil || result == nil {
		return result, err
	}
	if _, marshalErr := json.Marshal(result); marshalErr != nil {
		slog.Warn("plugin tool result required safe normalization",
			"tool", strings.TrimSpace(t.name),
			"cause", marshalErr.Error(),
		)
		return tools.SafeToolPayloadValue(result, 64*1024), nil
	}
	return result, nil
}

func registerPluginTools(toolRegistry *tools.Registry, pluginRegistry *plugin.Registry) {
	if toolRegistry == nil || pluginRegistry == nil {
		return
	}
	for _, tool := range pluginRegistry.ListTools() {
		if tool == nil || strings.TrimSpace(tool.Name) == "" {
			continue
		}
		if toolRegistry.Get(tool.Name) != nil {
			continue
		}
		toolRegistry.Register(&pluginToolAdapter{
			registry: pluginRegistry,
			name:     tool.Name,
		})
	}
}
