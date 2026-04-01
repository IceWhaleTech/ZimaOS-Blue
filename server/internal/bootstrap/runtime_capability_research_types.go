package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

type researchEventPublisherTarget interface {
	SetEventPublisher(publisher deepresearch.EventPublisher)
}

type researchRuntimeBinding struct {
	eventPublisher deepresearch.EventPublisher
	driver         *harnessdrivers.ResearchDriver
}

type chatResearchRuntimeTarget interface {
	autoHarnessTurnHookTarget
	SetDeepResearchService(svc *deepresearch.Service)
}

type chatResearchRuntimeBinding struct {
	service     *deepresearch.Service
	turnHook    serverpkg.TurnHook
	research    researchRuntimeBinding
	toolAdapter *deepResearchToolAdapter
}
