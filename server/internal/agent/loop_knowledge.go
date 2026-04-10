package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/knowledge"
)

type LoopKnowledgeResolver interface {
	ResolveLoopContext(ctx context.Context, req knowledge.LoopContextRequest) (*knowledge.LoopContextResult, error)
}

type loopKnowledgeTrace struct {
	Context    string
	SkipReason string
	UsedCount  int
	UsedSlugs  []string
}

func (r *Runner) SetKnowledgeResolver(resolver LoopKnowledgeResolver) {
	if r == nil {
		return
	}
	r.knowledgeResolver = resolver
	if r.groundedRuntime != nil {
		r.groundedRuntime.knowledgeResolver = resolver
	}
}

func resolvePlanningLoopKnowledge(ctx context.Context, resolver LoopKnowledgeResolver, task *Task, goal string) loopKnowledgeTrace {
	return resolveLoopKnowledgeTrace(ctx, resolver, knowledge.LoopContextRequest{
		Stage:       knowledge.LoopContextStagePlanning,
		Goal:        goal,
		TokenBudget: 220,
	})
}

func resolveExecutionLoopKnowledge(ctx context.Context, resolver LoopKnowledgeResolver, task *Task, step PlanStep, plan []PlanStep) loopKnowledgeTrace {
	goal := ""
	if task != nil {
		goal = task.Goal
	}
	return resolveLoopKnowledgeTrace(ctx, resolver, knowledge.LoopContextRequest{
		Stage:       knowledge.LoopContextStageExecution,
		Goal:        goal,
		CurrentStep: step.Description,
		PlanSummary: buildGroundedPlanSummary(plan),
		TokenBudget: 120,
	})
}

func resolveRecoveryLoopKnowledge(ctx context.Context, resolver LoopKnowledgeResolver, task *Task, verificationCtx VerificationContext, verificationResult *VerificationResult) loopKnowledgeTrace {
	failureHints := make([]string, 0, 4)
	if verificationResult != nil {
		if summary := strings.TrimSpace(verificationResult.Summary); summary != "" {
			failureHints = append(failureHints, summary)
		}
		if suggested := strings.TrimSpace(verificationResult.SuggestedRecovery); suggested != "" {
			failureHints = append(failureHints, suggested)
		}
		for _, item := range verificationResult.CriteriaResults {
			if evidence := strings.TrimSpace(item.Evidence); evidence != "" {
				failureHints = append(failureHints, evidence)
			}
		}
	}
	return resolveLoopKnowledgeTrace(ctx, resolver, knowledge.LoopContextRequest{
		Stage:             knowledge.LoopContextStageRecovery,
		Goal:              verificationCtx.Goal,
		CurrentStep:       recoveryStepDescription(verificationCtx.TaskKind),
		PlanSummary:       strings.Join(verificationCtx.FallbackPlan, "\n"),
		TokenBudget:       140,
		PriorFailureHints: failureHints,
	})
}

func resolveLoopKnowledgeTrace(ctx context.Context, resolver LoopKnowledgeResolver, req knowledge.LoopContextRequest) loopKnowledgeTrace {
	if resolver == nil {
		return loopKnowledgeTrace{}
	}
	result, err := resolver.ResolveLoopContext(ctx, req)
	if err != nil {
		return loopKnowledgeTrace{SkipReason: "resolver_error"}
	}
	if result == nil {
		return loopKnowledgeTrace{}
	}
	return loopKnowledgeTrace{
		Context:    strings.TrimSpace(result.Context),
		SkipReason: strings.TrimSpace(result.SkipReason),
		UsedCount:  result.UsedCount,
		UsedSlugs:  append([]string(nil), result.UsedSlugs...),
	}
}

func (r *Runner) publishLoopKnowledgeTrace(task *Task, trace loopKnowledgeTrace) {
	if r == nil || task == nil {
		return
	}
	if trace.SkipReason != "" {
		r.publishEvent(task.UserID, TaskEvent{
			TaskID:    task.ID,
			EventType: "task_loop_knowledge_skipped",
			Message:   trace.SkipReason,
		})
	}
	if trace.UsedCount > 0 {
		message := fmt.Sprintf("%d", trace.UsedCount)
		if len(trace.UsedSlugs) > 0 {
			message += ":" + strings.Join(trace.UsedSlugs, ",")
		}
		r.publishEvent(task.UserID, TaskEvent{
			TaskID:    task.ID,
			EventType: "task_loop_knowledge_used",
			Message:   message,
		})
	}
}
