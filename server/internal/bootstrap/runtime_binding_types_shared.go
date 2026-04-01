package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/promptguard"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeToolEventObserverTarget interface {
	SetToolEventObserver(observer tools.RuntimeEventObserver)
}

type runtimeObserverTarget interface {
	SetObserver(observer tools.RuntimeEventObserver)
}

type runtimeToolApproverTarget interface {
	SetToolApprover(approver tools.ToolApprover)
}

type runtimeSettingsHandlerTarget interface {
	SetSettingsHandler(handler *serverpkg.SettingsHandler)
}

type runtimePromptGuardTarget interface {
	SetPromptGuard(detector *promptguard.Detector)
}

type runtimeCounterRecorder interface {
	RecordCounter(name string, value int64, tags map[string]string)
}

type runtimeResearchSettingsTarget interface {
	SetV2Enabled(enabled bool)
}

type runtimeAutoConfirmTarget interface {
	SetAutoConfirmFunc(fn func() bool)
}

type runtimeLocaleTarget interface {
	SetLocaleFunc(fn func() string)
}

type runtimePromptSettingsTarget interface {
	SetLocaleFunc(fn func() string)
	SetAgentModeFunc(fn func() bool)
	SetAgentAutoConfirmFunc(fn func() bool)
}

type runtimeAdminSettingsTarget interface {
	SetSettings(svc tools.AdminSettingsService)
}

type runtimeChatFlagEvaluator interface {
	HasFlag(flagName string) bool
	IsEnabled(flagName string, ctx *config.EvaluationContext) bool
}
