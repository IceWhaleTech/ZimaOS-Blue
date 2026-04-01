package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

func newAgentRuntimeSupportBinding(
	reflector agent.SelfReflector,
	metrics interface {
		RecordCounter(name string, value int64, tags map[string]string)
	},
) agentRuntimeSupportBinding {
	return agentRuntimeSupportBinding{
		reflector: reflector,
		metrics:   metrics,
	}
}

func (binding agentRuntimeSupportBinding) apply(target agentRuntimeSupportTarget) {
	if target == nil {
		return
	}
	if binding.reflector != nil {
		target.SetReflector(binding.reflector)
	}
	if binding.metrics != nil {
		target.SetToolMetricsRecorder(binding.metrics)
	}
}

func bindAgentRuntimeSupport(
	target agentRuntimeSupportTarget,
	reflector agent.SelfReflector,
	metrics interface {
		RecordCounter(name string, value int64, tags map[string]string)
	},
) {
	newAgentRuntimeSupportBinding(reflector, metrics).apply(target)
}

func newReflectionRuntimeBinding(llmCaller selfreflect.LLMCaller, proposalGate func() bool) reflectionRuntimeBinding {
	return reflectionRuntimeBinding{
		llmCaller:    llmCaller,
		proposalGate: proposalGate,
	}
}

func (binding reflectionRuntimeBinding) apply(target reflectionRuntimeTarget) {
	if target == nil {
		return
	}
	if binding.llmCaller != nil {
		target.SetLLMCaller(binding.llmCaller)
	}
	if binding.proposalGate != nil {
		target.SetProposalGateFunc(binding.proposalGate)
	}
}

func (binding reflectionRuntimeBinding) applyJudge(target harnessJudgeEvaluatorTarget) {
	bindHarnessRuntimeJudgeEvaluator(target, binding.llmCaller)
}

func bindHarnessRuntimeReflection(
	bundle *HarnessRuntimeBundle,
	target reflectionRuntimeTarget,
	llmCaller selfreflect.LLMCaller,
	proposalGate func() bool,
) {
	binding := newReflectionRuntimeBinding(llmCaller, proposalGate)
	binding.apply(target)
	binding.applyJudge(harnessRuntimeController(bundle))
}

func activateHarnessRuntime(
	ctx context.Context,
	bundle *HarnessRuntimeBundle,
	target reflectionRuntimeTarget,
	llmCaller selfreflect.LLMCaller,
	proposalGate func() bool,
) {
	bindHarnessRuntimeReflection(bundle, target, llmCaller, proposalGate)
	startHarnessGroupDispatcher(ctx, bundle)
}

func startHarnessGroupDispatcher(ctx context.Context, bundle *HarnessRuntimeBundle) {
	if bundle == nil || bundle.GroupDispatcher == nil {
		return
	}
	go bundle.GroupDispatcher.Start(ctx)
}
