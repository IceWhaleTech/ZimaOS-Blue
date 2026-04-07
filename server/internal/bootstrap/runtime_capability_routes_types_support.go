package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

type runtimeTaskSurfaceKnowledgeDeps struct {
	memoryHandler   *serverpkg.MemoryHandler
	cronHandler     *cron.Handler
	sseBroker       *sse.Broker
	knowledgeAuthor knowledge.KnowledgeAuthor
}

type runtimeTaskKnowledgeSurfaceRegistration struct {
	routesRegistered bool
	cronRegistered   bool
}

func newRuntimeTaskSurfaceKnowledgeDeps(state *routeRegistrationState) runtimeTaskSurfaceKnowledgeDeps {
	if state == nil {
		return runtimeTaskSurfaceKnowledgeDeps{}
	}
	deps := state.deps
	if deps == nil {
		return runtimeTaskSurfaceKnowledgeDeps{
			knowledgeAuthor: newRuntimeKnowledgeAuthor(newRuntimeKnowledgeAuthorCaller(state.runtimeLLM, state.bootstrapSupport.smallModelManager)),
		}
	}
	return runtimeTaskSurfaceKnowledgeDeps{
		memoryHandler:   deps.MemoryHandler,
		cronHandler:     deps.CronHandler,
		sseBroker:       deps.SSEBroker,
		knowledgeAuthor: newRuntimeKnowledgeAuthor(newRuntimeKnowledgeAuthorCaller(state.runtimeLLM, state.bootstrapSupport.smallModelManager)),
	}
}

func withRuntimeTaskSurfaceKnowledgeDeps(state *routeRegistrationState, options runtimeTaskSurfaceOptions) runtimeTaskSurfaceOptions {
	options.runtimeTaskSurfaceKnowledgeDeps = newRuntimeTaskSurfaceKnowledgeDeps(state)
	return options
}
