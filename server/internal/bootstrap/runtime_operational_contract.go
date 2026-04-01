package bootstrap

import (
	"context"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/heartbeat"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

func (binding *runtimeContractBinding) BindHeartbeatRuntime(options routeRuntimeContractHeartbeatOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimeHeartbeat(options)
}

func bindRouteRuntimeHeartbeat(options routeRuntimeContractHeartbeatOptions) {
	if options.apiProtected == nil || options.config == nil || options.runtimeLLM == nil {
		return
	}
	hbCfg := cloneRouteRuntimeHeartbeatConfig(options.config)
	if hbCfg.Interval == 0 {
		hbCfg.Interval = heartbeat.DefaultInterval
	}
	if hbCfg.WorkspaceDir == "" {
		hbCfg.WorkspaceDir = options.dataDir
	}
	hbRunner := heartbeat.NewRunner(heartbeat.RunnerDeps{
		Config: hbCfg,
		ChatFn: func(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
			return options.runtimeLLM.Chat(ctx, req)
		},
		Logger: options.logger,
	})
	ctx := options.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	go hbRunner.Run(ctx)
	heartbeat.NewHandler(hbRunner).RegisterRoutes(options.apiProtected)
	if options.logger != nil {
		options.logger.Info("Heartbeat routes registered", zap.Bool("enabled", hbCfg.Enabled))
	}
}

func (binding *runtimeContractBinding) BindOperationalRuntime(options routeRuntimeContractOperationalOptions) routeRuntimeContractOperationalResult {
	if binding == nil {
		return routeRuntimeContractOperationalResult{}
	}
	return bindRouteRuntimeOperational(binding, options)
}

func bindRouteRuntimeOperational(binding routeRuntimeOperationalBinding, options routeRuntimeContractOperationalOptions) routeRuntimeContractOperationalResult {
	if binding == nil {
		return routeRuntimeContractOperationalResult{}
	}
	taskSurface := binding.RegisterTaskSurface(options.taskSurface)
	result := routeRuntimeContractOperationalResult{
		taskSurface: taskSurface,
	}
	result.activation = binding.ActivateRouteRuntime(options.activation)
	result.support = bindRouteRuntimeOperationalSupport(binding, result.activation, options)
	return result
}
