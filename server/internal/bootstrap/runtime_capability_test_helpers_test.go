package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

func newTestRuntimeCapabilityContract(bundle *HarnessRuntimeBundle) runtimeCapabilityAdapter {
	contract := runtimeCapabilityContract{
		Research: deepresearch.NewService(nil, nil),
		Reflect:  selfreflect.NewService(nil, nil),
	}
	contract.Harness = bundle
	return newRuntimeCapabilityAdapter(contract)
}

type stubRuntimeWorkspaceManagerSource struct {
	mgr *workspace.Manager
}

func (s *stubRuntimeWorkspaceManagerSource) Manager() *workspace.Manager {
	if s == nil {
		return nil
	}
	return s.mgr
}

type stubResearchPublisherTarget struct {
	publisher deepresearch.EventPublisher
}

func (s *stubResearchPublisherTarget) SetEventPublisher(publisher deepresearch.EventPublisher) {
	s.publisher = publisher
}

type stubChatResearchRuntimeTarget struct {
	service      *deepresearch.Service
	hook         serverpkg.TurnHook
	serviceCalls int
	hookCalls    int
}

func (s *stubChatResearchRuntimeTarget) SetDeepResearchService(service *deepresearch.Service) {
	s.service = service
	s.serviceCalls++
}

func (s *stubChatResearchRuntimeTarget) RegisterTurnHook(hook serverpkg.TurnHook) {
	s.hook = hook
	s.hookCalls++
}
