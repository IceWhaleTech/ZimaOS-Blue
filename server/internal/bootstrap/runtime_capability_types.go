package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

// HarnessRuntimeBundle centralizes the shared harness runtime wiring used by
// chat, deep research, agent tasks, and harness-backed subagents.
type HarnessRuntimeBundle struct {
	Controller       *harness.Controller
	GroupDispatcher  *harness.GroupDispatcher
	RunTracer        *harness.RunTraceCollector
	RuntimeObserver  tools.RuntimeEventObserver
	SubagentExecutor tools.SubagentExecutor
	WriteGuard       tools.WritePathGuard
	ExecGuard        tools.ExecPathGuard
}

type runtimeCapabilityContract struct {
	Research *deepresearch.Service
	Reflect  *selfreflect.Service
	Harness  *HarnessRuntimeBundle
}

type runtimeWorkspaceManagerSource interface {
	Manager() *workspace.Manager
}
