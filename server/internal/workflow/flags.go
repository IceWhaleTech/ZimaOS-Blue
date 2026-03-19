package workflow

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"

const workflowCheckpointResumeFlagName = "workflow_checkpoint_resume"

type workflowFlagEvaluator interface {
	HasFlag(flagName string) bool
	IsEnabled(flagName string, ctx *config.EvaluationContext) bool
}

// SetFlagEvaluator wires feature-flag evaluation into workflow runtime behavior.
func (s *WorkflowService) SetFlagEvaluator(evaluator workflowFlagEvaluator) {
	if s == nil {
		return
	}
	s.flags = evaluator
	if s.engine != nil {
		s.engine.SetCheckpointEnabledFunc(func() bool {
			return s.workflowCheckpointResumeEnabled(nil)
		})
	}
}

func (s *WorkflowService) workflowCheckpointResumeEnabled(ctx *config.EvaluationContext) bool {
	if s == nil || s.flags == nil {
		return true
	}
	if !s.flags.HasFlag(workflowCheckpointResumeFlagName) {
		return true
	}
	return s.flags.IsEnabled(workflowCheckpointResumeFlagName, ctx)
}
