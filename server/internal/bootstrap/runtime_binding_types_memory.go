package bootstrap

import (
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type layeredMemoryReadyTarget interface {
	SetOnLayeredReady(fn func(*memory.LayeredMemoryService))
}

type layeredMemoryChatTarget interface {
	SetLayeredMemory(svc *memory.LayeredMemoryService)
}

type layeredMemoryAgentTarget interface {
	SetMemory(m agent.MemoryRecaller)
}

type layeredMemoryReflectionTarget interface {
	SetMemoryWriter(writer selfreflect.MemoryWriter)
}

type agentRuntimeSupportTarget interface {
	SetReflector(reflector agent.SelfReflector)
	SetToolMetricsRecorder(recorder interface {
		RecordCounter(name string, value int64, tags map[string]string)
	})
}

type runtimeAskPolicySettingsSource interface {
	GetAgentAutoConfirm() bool
	GetAgentAskTimeoutSeconds() int
	GetAgentAskTimeoutAction() string
	GetAgentLoopPolicyMaxToolRounds() int
	GetAgentAutoReflect() bool
}

type reflectionRuntimeTarget interface {
	SetLLMCaller(llmCaller selfreflect.LLMCaller)
	SetProposalGateFunc(fn func() bool)
}

type chatAskRuntimeTarget interface {
	runtimeToolEventObserverTarget
	SetMediaDir(dir string)
	SetQuestionManager(mgr *tools.QuestionManager)
	SetBrowserCheckpointManager(mgr *tools.BrowserCheckpointManager)
	SetBrowserSiteAllowlistStore(store *tools.BrowserSiteAllowlistStore)
}

type chatAskRuntimeBinding struct {
	mediaDir             string
	questionMgr          *tools.QuestionManager
	browserCheckpointMgr *tools.BrowserCheckpointManager
	browserSiteStore     *tools.BrowserSiteAllowlistStore
	runtimeEventObserver tools.RuntimeEventObserver
}

type questionRuntimePolicyTarget interface {
	SetSilentFunc(fn func() bool)
	SetTimeoutFunc(fn func() time.Duration)
	SetTimeoutActionFunc(fn func() string)
}

type agentRuntimePolicyTarget interface {
	SetAskTimeoutFunc(fn func() time.Duration)
	SetAskTimeoutActionFunc(fn func() string)
	SetMaxToolRoundsPerStepFunc(fn func() int)
	SetAutoReflectFunc(fn func() bool)
}

type agentRuntimeSupportBinding struct {
	reflector agent.SelfReflector
	metrics   interface {
		RecordCounter(name string, value int64, tags map[string]string)
	}
}

type runtimeAskPolicyBinding struct {
	silentFunc        func() bool
	timeoutFunc       func() time.Duration
	timeoutActionFunc func() string
	maxToolRoundsFunc func() int
	autoReflectFunc   func() bool
}

type reflectionRuntimeBinding struct {
	llmCaller    selfreflect.LLMCaller
	proposalGate func() bool
}
