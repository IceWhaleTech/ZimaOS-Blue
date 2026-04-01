package bootstrap

import (
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newChatAskRuntimeBinding(
	bundle *HarnessRuntimeBundle,
	mediaDir string,
	questionMgr *tools.QuestionManager,
	browserCheckpointMgr *tools.BrowserCheckpointManager,
	browserSiteStore *tools.BrowserSiteAllowlistStore,
) chatAskRuntimeBinding {
	return chatAskRuntimeBinding{
		mediaDir:             strings.TrimSpace(mediaDir),
		questionMgr:          questionMgr,
		browserCheckpointMgr: browserCheckpointMgr,
		browserSiteStore:     browserSiteStore,
		runtimeEventObserver: harnessRuntimeObserver(bundle),
	}
}

func (binding chatAskRuntimeBinding) applyChat(target chatAskRuntimeTarget) {
	if target == nil {
		return
	}
	target.SetMediaDir(binding.mediaDir)
	target.SetQuestionManager(binding.questionMgr)
	target.SetBrowserCheckpointManager(binding.browserCheckpointMgr)
	target.SetBrowserSiteAllowlistStore(binding.browserSiteStore)
	if binding.runtimeEventObserver != nil {
		target.SetToolEventObserver(binding.runtimeEventObserver)
	}
}

func (binding chatAskRuntimeBinding) applyQuestionManager(target runtimeObserverTarget) {
	if target == nil || binding.runtimeEventObserver == nil {
		return
	}
	target.SetObserver(binding.runtimeEventObserver)
}

func bindHarnessRuntimeAskSupport(
	bundle *HarnessRuntimeBundle,
	handler chatAskRuntimeTarget,
	mediaDir string,
	questionMgr *tools.QuestionManager,
	browserCheckpointMgr *tools.BrowserCheckpointManager,
	browserSiteStore *tools.BrowserSiteAllowlistStore,
) {
	binding := newChatAskRuntimeBinding(bundle, mediaDir, questionMgr, browserCheckpointMgr, browserSiteStore)
	binding.applyChat(handler)
	binding.applyQuestionManager(questionMgr)
}

func newRuntimeAskPolicyBinding(settings runtimeAskPolicySettingsSource) runtimeAskPolicyBinding {
	if settings == nil {
		return runtimeAskPolicyBinding{}
	}
	return runtimeAskPolicyBinding{
		silentFunc: settings.GetAgentAutoConfirm,
		timeoutFunc: func() time.Duration {
			seconds := settings.GetAgentAskTimeoutSeconds()
			if seconds <= 0 {
				seconds = 120
			}
			return time.Duration(seconds) * time.Second
		},
		timeoutActionFunc: settings.GetAgentAskTimeoutAction,
		maxToolRoundsFunc: settings.GetAgentLoopPolicyMaxToolRounds,
		autoReflectFunc:   settings.GetAgentAutoReflect,
	}
}

func (binding runtimeAskPolicyBinding) applyQuestionManager(target questionRuntimePolicyTarget) {
	if target == nil {
		return
	}
	if binding.silentFunc != nil {
		target.SetSilentFunc(binding.silentFunc)
	}
	if binding.timeoutFunc != nil {
		target.SetTimeoutFunc(binding.timeoutFunc)
	}
	if binding.timeoutActionFunc != nil {
		target.SetTimeoutActionFunc(binding.timeoutActionFunc)
	}
}

func (binding runtimeAskPolicyBinding) applyAgent(target agentRuntimePolicyTarget) {
	if target == nil {
		return
	}
	if binding.timeoutFunc != nil {
		target.SetAskTimeoutFunc(binding.timeoutFunc)
	}
	if binding.timeoutActionFunc != nil {
		target.SetAskTimeoutActionFunc(binding.timeoutActionFunc)
	}
	if binding.maxToolRoundsFunc != nil {
		target.SetMaxToolRoundsPerStepFunc(binding.maxToolRoundsFunc)
	}
	if binding.autoReflectFunc != nil {
		target.SetAutoReflectFunc(binding.autoReflectFunc)
	}
}
