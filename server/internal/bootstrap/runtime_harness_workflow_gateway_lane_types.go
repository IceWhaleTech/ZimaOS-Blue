package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type runtimeToolGatewayMetricsRecorder interface {
	RecordCounter(name string, value int64, tags map[string]string)
}

type workflowRuntimeMetricsRecorder interface {
	RecordCounter(name string, value int64, tags map[string]string)
}

type workflowRuntimeApplyTarget interface {
	ApplyToolGateway(gateway workflow.ToolRuntime)
	ApplyMetricsRecorder(recorder workflowRuntimeMetricsRecorder)
}

type workflowRuntimeHookTarget interface {
	SetServiceInitHook(func(*workflow.WorkflowService))
}

type workflowRuntimeBinding struct {
	toolRuntime workflow.ToolRuntime
	metrics     workflowRuntimeMetricsRecorder
}

type workflowMetricsRecorderAdapter struct {
	recorder workflowRuntimeMetricsRecorder
}

type workflowToolRuntimeAdapter struct {
	gateway *tools.ToolGateway
}
