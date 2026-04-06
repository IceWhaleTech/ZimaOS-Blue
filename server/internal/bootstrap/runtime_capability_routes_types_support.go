package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

type runtimeTaskSurfaceKnowledgeDeps struct {
	memoryHandler *serverpkg.MemoryHandler
	cronHandler   *cron.Handler
	sseBroker     *sse.Broker
}

type runtimeTaskKnowledgeSurfaceRegistration struct {
	routesRegistered bool
	cronRegistered   bool
}

func newRuntimeTaskSurfaceKnowledgeDeps(deps *RoutesDeps) runtimeTaskSurfaceKnowledgeDeps {
	if deps == nil {
		return runtimeTaskSurfaceKnowledgeDeps{}
	}
	return runtimeTaskSurfaceKnowledgeDeps{
		memoryHandler: deps.MemoryHandler,
		cronHandler:   deps.CronHandler,
		sseBroker:     deps.SSEBroker,
	}
}

func withRuntimeTaskSurfaceKnowledgeDeps(deps *RoutesDeps, options runtimeTaskSurfaceOptions) runtimeTaskSurfaceOptions {
	options.runtimeTaskSurfaceKnowledgeDeps = newRuntimeTaskSurfaceKnowledgeDeps(deps)
	return options
}
