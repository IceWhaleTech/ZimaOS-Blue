package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newRuntimeToolGateway(
	registry *tools.Registry,
	approver tools.ToolApprover,
	observer tools.RuntimeEventObserver,
	metrics runtimeToolGatewayMetricsRecorder,
) *tools.ToolGateway {
	if registry == nil || approver == nil {
		return nil
	}
	gateway := tools.NewToolGateway(registry, tools.NewExecutor(registry))
	gateway.SetApprover(approver)
	if observer != nil {
		gateway.SetEventObserver(observer)
	}
	if metrics != nil {
		gateway.SetMetricsRecorder(metrics)
	}
	return gateway
}
