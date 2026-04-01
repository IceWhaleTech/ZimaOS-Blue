package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func runtimeActivationToolRegistry(services *Services) *tools.Registry {
	if services == nil {
		return nil
	}
	return services.ToolRegistry
}

func runtimeActivationImageTool(services *Services) *tools.ImageTool {
	registry := runtimeActivationToolRegistry(services)
	if registry == nil {
		return nil
	}
	if tool := registry.Get("image"); tool != nil {
		imageTool, _ := tool.(*tools.ImageTool)
		return imageTool
	}
	return nil
}

func runtimeActivationExecAutoConfirm(services *Services) runtimeAutoConfirmTarget {
	registry := runtimeActivationToolRegistry(services)
	if registry == nil {
		return nil
	}
	execTool := tools.GetExecTool(registry)
	if execTool == nil {
		return nil
	}
	return execTool
}

func runtimeActivationSessionCompaction(deps *RoutesDeps) config.SessionCompactionConfig {
	if deps == nil || deps.Config == nil {
		return config.SessionCompactionConfig{}
	}
	return deps.Config.Session.Compaction
}

func runtimeActivationSessionMaxTokens(deps *RoutesDeps) int {
	if deps == nil || deps.Config == nil {
		return 0
	}
	return deps.Config.Session.MaxTokens
}
