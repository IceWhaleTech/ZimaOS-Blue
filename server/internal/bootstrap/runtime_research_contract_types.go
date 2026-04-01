package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRuntimeContractResearchOptions struct {
	target       chatResearchRuntimeTarget
	broker       *sse.Broker
	registry     *tools.Registry
	workspaceDir string
}

type routeRuntimeContractResearchResult struct {
	serviceBound        bool
	turnHookBound       bool
	eventPublisherBound bool
	driverRegistered    bool
	toolRegistered      bool
}
