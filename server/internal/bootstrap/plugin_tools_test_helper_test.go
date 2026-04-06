package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type pluginToolRegistryAdapter struct {
	definition tools.ToolDefinition
	handler    plugin.ToolHandler
}

func (a pluginToolRegistryAdapter) Definition() tools.ToolDefinition {
	return a.definition
}

func (a pluginToolRegistryAdapter) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	result, err := a.handler(ctx, args)
	if err != nil {
		return nil, err
	}
	return tools.SafeToolPayloadValue(result, 64*1024), nil
}

func registerPluginTools(registry *tools.Registry, pluginRegistry *plugin.Registry) {
	if registry == nil || pluginRegistry == nil {
		return
	}
	for _, registered := range pluginRegistry.ListTools() {
		if registered == nil || registered.Handler == nil {
			continue
		}
		registry.Register(pluginToolRegistryAdapter{
			definition: tools.ToolDefinition{
				Name:                registered.Name,
				Description:         registered.Description,
				Parameters:          registered.Parameters,
				RiskLevel:           registered.RiskLevel,
				VisibilityAllowlist: append([]string(nil), registered.VisibilityAllowlist...),
			},
			handler: registered.Handler,
		})
	}
}
