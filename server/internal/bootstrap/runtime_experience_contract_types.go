package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"

type routeRuntimeExperienceBinding interface {
	routeRuntimeResearchSurface
	ConfigureChatRuntime(options routeRuntimeContractChatOptions) *agentcore.AutoSkillReranker
	BindChatRuntime(options routeRuntimeContractChatBindingOptions) routeRuntimeContractChatBindingResult
	BindProductivityTools(options routeRuntimeContractProductivityOptions) routeRuntimeContractProductivityResult
	BindPlatformSurfaceRuntime(options routeRuntimeContractPlatformSurfaceOptions)
	BindSkillRuntime(options routeRuntimeContractSkillOptions) routeRuntimeContractSkillResult
}

var _ routeRuntimeExperienceBinding = (*runtimeContractBinding)(nil)

type routeRuntimeContractExperienceSurfaceOptions struct {
	research routeRuntimeContractResearchOptions
}

type routeRuntimeContractExperienceOptions struct {
	chat         routeRuntimeContractChatOptions
	chatBinding  routeRuntimeContractChatBindingOptions
	surface      routeRuntimeContractExperienceSurfaceOptions
	productivity routeRuntimeContractProductivityOptions
	platform     routeRuntimeContractPlatformSurfaceOptions
	skill        routeRuntimeContractSkillOptions
}

type routeRuntimeContractExperienceResult struct {
	skillAutoReranker *agentcore.AutoSkillReranker
	chat              routeRuntimeContractChatBindingResult
	productivity      routeRuntimeContractProductivityResult
	skill             routeRuntimeContractSkillResult
}
