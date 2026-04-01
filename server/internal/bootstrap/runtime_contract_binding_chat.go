package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"

func (binding *runtimeContractBinding) BindChatRuntime(
	options routeRuntimeContractChatBindingOptions,
) routeRuntimeContractChatBindingResult {
	if binding == nil {
		return routeRuntimeContractChatBindingResult{}
	}
	hook := newHarnessRuntimeAutoHarnessTurnHook(binding.runtime.HarnessRuntime(), options.handler)
	bindHarnessRuntimeAutoHarnessTurnHook(options.target, hook)
	return routeRuntimeContractChatBindingResult{
		autoHarnessHookBound: options.target != nil && hook != nil,
	}
}

func (binding *runtimeContractBinding) ConfigureChatRuntime(options routeRuntimeContractChatOptions) *agentcore.AutoSkillReranker {
	if binding == nil {
		return nil
	}
	promptChat := options.chatPrompt
	if promptChat == nil {
		promptChat = options.chat
	}
	bindRuntimePromptGuard(promptChat, options.security, options.disabled, options.logger)
	return bindRuntimeToolSelection(options.chat, options.config, options.dataDir, options.workspaceDir, options.flagEvaluator)
}
