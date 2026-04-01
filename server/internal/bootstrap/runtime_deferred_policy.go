package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func bindRuntimeAskPolicy(settings runtimeAskPolicySettingsSource, questionMgr questionRuntimePolicyTarget, runner agentRuntimePolicyTarget) {
	binding := newRuntimeAskPolicyBinding(settings)
	binding.applyQuestionManager(questionMgr)
	binding.applyAgent(runner)
}

func bindRuntimeLayeredMemory(
	handler layeredMemoryReadyTarget,
	chat layeredMemoryChatTarget,
	runner layeredMemoryAgentTarget,
	reflector layeredMemoryReflectionTarget,
) {
	if handler == nil {
		return
	}
	handler.SetOnLayeredReady(func(svc *memory.LayeredMemoryService) {
		if chat != nil {
			chat.SetLayeredMemory(svc)
		}
		if runner != nil {
			runner.SetMemory(newAgentMemoryAdapter(svc))
		}
		if reflector != nil {
			reflector.SetMemoryWriter(newAgentReflectionMemoryWriter(svc))
		}
	})
}

func bindRuntimeSettingsTargets(
	settings *serverpkg.SettingsHandler,
	provider runtimeSettingsHandlerTarget,
	chat runtimeSettingsHandlerTarget,
	research runtimeResearchSettingsTarget,
	exec runtimeAutoConfirmTarget,
	prompt runtimePromptSettingsTarget,
	push runtimeLocaleTarget,
	masker runtimeLocaleTarget,
	mgmt runtimeAdminSettingsTarget,
) {
	if settings == nil {
		return
	}
	if provider != nil {
		provider.SetSettingsHandler(settings)
	}
	if chat != nil {
		chat.SetSettingsHandler(settings)
	}
	if research != nil {
		research.SetV2Enabled(settings.GetDeepResearchV2Enabled())
	}
	if exec != nil {
		exec.SetAutoConfirmFunc(settings.GetAgentAutoConfirm)
	}
	if prompt != nil {
		prompt.SetLocaleFunc(settings.GetLocale)
		prompt.SetAgentModeFunc(settings.GetAgentMode)
		prompt.SetAgentAutoConfirmFunc(settings.GetAgentAutoConfirm)
	}
	if push != nil {
		push.SetLocaleFunc(settings.GetLocale)
	}
	if masker != nil {
		masker.SetLocaleFunc(settings.GetLocale)
	}
	if mgmt != nil {
		mgmt.SetSettings(&mgmtSettingsAdapter{handler: settings})
	}
}
