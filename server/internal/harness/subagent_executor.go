package harness

import (
	"context"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const defaultSubagentPollInterval = 250 * time.Millisecond

type HarnessSubagentExecutor struct {
	manager      *Controller
	agents       *config.AgentsConfig
	pollInterval time.Duration
}

type isolatedSubagentExecution struct {
	parentRun *Run
	localCtx  context.Context
	resultCh  chan isolatedSubagentOutcome
}

type isolatedSubagentOutcome struct {
	result *tools.SubagentResult
	err    error
}

func NewSubagentExecutor(manager *Controller, agents *config.AgentsConfig) *HarnessSubagentExecutor {
	if manager == nil {
		return nil
	}
	pollInterval := defaultSubagentPollInterval
	if pollInterval <= 0 {
		pollInterval = 250 * time.Millisecond
	}
	return &HarnessSubagentExecutor{
		manager:      manager,
		agents:       agents,
		pollInterval: pollInterval,
	}
}

func (e *HarnessSubagentExecutor) ExecuteSubagent(ctx context.Context, req tools.SubagentRequest) (*tools.SubagentResult, error) {
	return e.ExecuteIsolated(ctx, req)
}

func (e *HarnessSubagentExecutor) ExecuteIsolated(ctx context.Context, req tools.SubagentRequest) (*tools.SubagentResult, error) {
	execution, immediate, err := e.startIsolatedExecution(ctx, req)
	if err != nil || !req.Wait {
		return immediate, err
	}
	outcome := <-execution.resultCh
	return outcome.result, outcome.err
}

func (e *HarnessSubagentExecutor) startIsolatedExecution(ctx context.Context, req tools.SubagentRequest) (*isolatedSubagentExecution, *tools.SubagentResult, error) {
	if e == nil || e.manager == nil {
		return nil, nil, newGuardPipelineError(RuntimeStageExecute, "runtime_unavailable", "subagent executor is not configured", nil)
	}
	controlCtx := subagentControlContext(ctx)
	req.Goal = strings.TrimSpace(req.Goal)
	req.AgentID = strings.TrimSpace(req.AgentID)
	req.Model = strings.TrimSpace(req.Model)
	req.Context = strings.TrimSpace(req.Context)
	if req.Goal == "" {
		return nil, nil, newGuardPipelineError(RuntimeStageNormalize, "goal_required", "goal is required", nil)
	}

	parentID := strings.TrimSpace(tools.GetRunID(ctx))
	if parentID == "" {
		return nil, nil, newGuardPipelineError(RuntimeStagePolicy, "parent_required", "subagents require a harness-backed parent run", nil)
	}
	parent, err := e.manager.GetStored(controlCtx, parentID)
	if err != nil {
		if errorsIsNoRows(err) {
			return nil, nil, newGuardPipelineError(RuntimeStagePolicy, "parent_not_found", "parent harness run was not found", map[string]interface{}{
				"parent_run_id": parentID,
			})
		}
		return nil, nil, err
	}

	parentCfg := e.effectiveAgentConfig(parent.AgentID)
	if !parentCfg.Subagents.Enabled {
		return nil, nil, newGuardPipelineError(RuntimeStagePolicy, "subagent_disabled", "subagents are disabled for the current agent", map[string]interface{}{
			"agent_id": strings.TrimSpace(parent.AgentID),
		})
	}

	spec := RunSpec{
		Kind:          RunKindSubagent,
		Goal:          req.Goal,
		AgentID:       req.AgentID,
		Model:         req.Model,
		MaxDuration:   req.MaxDuration,
		MaxSteps:      req.MaxSteps,
		MaxToolRounds: req.MaxToolRounds,
		MaxSubagents:  0,
		MaxDepth:      0,
		Metadata:      cloneMetadataMap(req.Metadata),
	}
	if spec.Metadata == nil {
		spec.Metadata = make(map[string]interface{})
	}
	if req.Context != "" {
		spec.Metadata["context"] = req.Context
	}
	spec.Metadata["subagent_isolated"] = true
	spec.Metadata["subagent_parent_run_id"] = parent.ID
	if strings.TrimSpace(parentCfg.Subagents.CallbackMode) != "" {
		spec.Metadata["callback_mode"] = strings.TrimSpace(parentCfg.Subagents.CallbackMode)
	}
	if spec.Model == "" && strings.TrimSpace(parentCfg.Subagents.DefaultModel) != "" {
		spec.Model = strings.TrimSpace(parentCfg.Subagents.DefaultModel)
	}

	e.applyChildBudgetDefaults(parent, parentCfg.Subagents, &spec)

	child, err := e.manager.SpawnChild(controlCtx, parent.ID, spec)
	if err != nil {
		return nil, nil, err
	}
	e.appendSubagentEvent(controlCtx, child, "subagent_start", "isolated subagent started", map[string]interface{}{
		"context_isolated": true,
		"wait":             req.Wait,
	})
	immediate := runToSubagentResult(child, false)
	if !req.Wait {
		return nil, immediate, nil
	}

	execution := &isolatedSubagentExecution{
		parentRun: parent,
		localCtx:  controlCtx,
		resultCh:  make(chan isolatedSubagentOutcome, 1),
	}
	go func() {
		waited, waitErr := e.waitForTerminal(ctx, child.ID)
		if waitErr != nil {
			execution.resultCh <- isolatedSubagentOutcome{err: waitErr}
			return
		}
		e.appendSubagentEvent(context.Background(), waited, "subagent_complete", "isolated subagent reached terminal state", map[string]interface{}{
			"context_isolated": true,
			"status":           string(waited.Status),
		})
		execution.resultCh <- isolatedSubagentOutcome{result: runToSubagentResult(waited, true)}
	}()
	return execution, immediate, nil
}

func (e *HarnessSubagentExecutor) waitForTerminal(ctx context.Context, runID string) (*Run, error) {
	waitCtx := ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	controlCtx := subagentControlContext(waitCtx)
	interval := e.pollInterval
	if interval <= 0 {
		interval = defaultSubagentPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	missingReads := 0

	for {
		run, err := e.manager.Get(controlCtx, runID)
		if err != nil {
			if !errorsIsNoRows(err) {
				return nil, err
			}
			missingReads++
			run = nil
		} else {
			missingReads = 0
			if isTerminalRunStatus(run.Status) {
				return run, nil
			}
		}

		select {
		case <-waitCtx.Done():
			cancelErr := e.manager.Cancel(controlCtx, runID, "parent context cancelled while waiting on subagent")
			if cancelErr != nil && !errorsIsNoRows(cancelErr) {
				return nil, cancelErr
			}
			code := "subagent_aborted"
			switch waitCtx.Err() {
			case context.Canceled:
				code = "subagent_cancelled"
			case context.DeadlineExceeded:
				code = "subagent_timeout"
			}
			if run == nil {
				stored, storedErr := e.manager.GetStored(controlCtx, runID)
				if storedErr == nil {
					run = stored
				}
			}
			e.appendSubagentEvent(controlCtx, run, code, waitCtx.Err().Error(), map[string]interface{}{
				"context_isolated": true,
			})
			return nil, newGuardPipelineErrorWithCause(RuntimeStageExecute, code, waitCtx.Err().Error(), waitCtx.Err(), map[string]interface{}{
				"run_id":     runID,
				"wait_state": "pending_subagent_completion",
			})
		case <-ticker.C:
			if missingReads >= 3 {
				return nil, newGuardPipelineError(RuntimeStageExecute, "subagent_not_found", "subagent run was not found", map[string]interface{}{
					"run_id": runID,
				})
			}
		}
	}
}

func subagentControlContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func (e *HarnessSubagentExecutor) applyChildBudgetDefaults(parent *Run, policy config.AgentSubagentPolicyConfig, spec *RunSpec) {
	if parent == nil || spec == nil {
		return
	}
	if spec.MaxSteps <= 0 {
		spec.MaxSteps = tighterPositive(halfPositive(parent.MaxSteps), parent.MaxSteps)
	}
	if spec.MaxToolRounds <= 0 {
		spec.MaxToolRounds = tighterPositive(halfPositive(parent.MaxToolRounds), parent.MaxToolRounds)
	}
	if spec.MaxSubagents <= 0 {
		spec.MaxSubagents = tighterPositive(parent.MaxSubagents, policy.MaxParallel)
	}
	if spec.MaxDepth <= 0 {
		spec.MaxDepth = tighterPositive(parent.MaxDepth, policy.MaxDepth)
	}
	if spec.MaxDuration <= 0 {
		spec.MaxDuration = tighterDuration(halfDuration(parent.MaxDuration), policy.Timeout)
	}
	if spec.MaxDuration <= 0 {
		spec.MaxDuration = tighterDuration(parent.MaxDuration, policy.Timeout)
	}
}

func (e *HarnessSubagentExecutor) effectiveAgentConfig(agentID string) config.AgentConfig {
	if e == nil || e.agents == nil {
		return config.DefaultAgentsConfig().Defaults
	}
	defaults := e.agents.Defaults
	id := strings.TrimSpace(agentID)
	if id == "" {
		return defaults
	}
	for _, item := range e.agents.List {
		if !strings.EqualFold(strings.TrimSpace(item.ID), id) {
			continue
		}
		return mergeEffectiveAgentConfig(defaults, item)
	}
	return defaults
}

func mergeEffectiveAgentConfig(defaults, agent config.AgentConfig) config.AgentConfig {
	out := agent
	if out.ID == "" {
		out.ID = defaults.ID
	}
	if out.Description == "" {
		out.Description = defaults.Description
	}
	if out.Thinking == "" {
		out.Thinking = defaults.Thinking
	}
	if out.Model == "" {
		out.Model = defaults.Model
	}
	if out.ToolPolicy.Profile == "" {
		out.ToolPolicy.Profile = defaults.ToolPolicy.Profile
	}
	if len(out.ToolPolicy.Allow) == 0 && len(defaults.ToolPolicy.Allow) > 0 {
		out.ToolPolicy.Allow = append([]string(nil), defaults.ToolPolicy.Allow...)
	}
	if len(out.ToolPolicy.Deny) == 0 && len(defaults.ToolPolicy.Deny) > 0 {
		out.ToolPolicy.Deny = append([]string(nil), defaults.ToolPolicy.Deny...)
	}
	if len(out.ToolPolicy.ByProvider) == 0 && len(defaults.ToolPolicy.ByProvider) > 0 {
		out.ToolPolicy.ByProvider = defaults.ToolPolicy.ByProvider
	}
	if out.Sandbox.Mode == "" {
		out.Sandbox.Mode = defaults.Sandbox.Mode
	}
	if out.Sandbox.Scope == "" {
		out.Sandbox.Scope = defaults.Sandbox.Scope
	}
	if out.Browser.Profile == "" {
		out.Browser.Profile = defaults.Browser.Profile
	}
	if isZeroSubagentConfig(out.Subagents) {
		out.Subagents = defaults.Subagents
	} else {
		if out.Subagents.MaxParallel == 0 {
			out.Subagents.MaxParallel = defaults.Subagents.MaxParallel
		}
		if out.Subagents.MaxDepth == 0 {
			out.Subagents.MaxDepth = defaults.Subagents.MaxDepth
		}
		if out.Subagents.DefaultModel == "" {
			out.Subagents.DefaultModel = defaults.Subagents.DefaultModel
		}
		if out.Subagents.CheapModel == "" {
			out.Subagents.CheapModel = defaults.Subagents.CheapModel
		}
		if out.Subagents.Timeout == 0 {
			out.Subagents.Timeout = defaults.Subagents.Timeout
		}
		if out.Subagents.CallbackMode == "" {
			out.Subagents.CallbackMode = defaults.Subagents.CallbackMode
		}
	}
	return out
}

func isZeroSubagentConfig(policy config.AgentSubagentPolicyConfig) bool {
	return !policy.Enabled &&
		policy.MaxParallel == 0 &&
		policy.MaxDepth == 0 &&
		policy.DefaultModel == "" &&
		policy.CheapModel == "" &&
		policy.Timeout == 0 &&
		policy.CallbackMode == ""
}

func tighterPositive(values ...int) int {
	best := 0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if best == 0 || value < best {
			best = value
		}
	}
	return best
}

func halfPositive(value int) int {
	if value <= 0 {
		return 0
	}
	if value == 1 {
		return 1
	}
	if value == 2 {
		return 1
	}
	return value / 2
}

func tighterDuration(values ...time.Duration) time.Duration {
	best := time.Duration(0)
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if best == 0 || value < best {
			best = value
		}
	}
	return best
}

func halfDuration(value time.Duration) time.Duration {
	if value <= 0 {
		return 0
	}
	if value <= time.Second {
		return value
	}
	return value / 2
}

func runToSubagentResult(run *Run, waited bool) *tools.SubagentResult {
	if run == nil {
		return nil
	}
	return &tools.SubagentResult{
		RunID:           run.ID,
		RootRunID:       run.RootRunID,
		ParentRunID:     run.ParentRunID,
		Status:          string(run.Status),
		Goal:            run.Goal,
		Result:          run.Result,
		Error:           run.Error,
		AgentID:         run.AgentID,
		Model:           run.Model,
		Depth:           run.Depth,
		Waited:          waited,
		Terminal:        isTerminalRunStatus(run.Status),
		Completed:       run.Status == RunStatusCompleted,
		ContextIsolated: true,
	}
}

func (e *HarnessSubagentExecutor) appendSubagentEvent(ctx context.Context, run *Run, eventType, message string, payload map[string]interface{}) {
	if e == nil || e.manager == nil || run == nil {
		return
	}
	_ = e.manager.AppendEvent(ctx, RunEvent{
		RunID:       run.ID,
		RootRunID:   run.RootRunID,
		ParentRunID: run.ParentRunID,
		Type:        strings.TrimSpace(eventType),
		Message:     strings.TrimSpace(message),
		PayloadJSON: observerPayloadJSON(payload),
		CreatedAt:   time.Now().UTC(),
	})
}

func isTerminalRunStatus(status RunStatus) bool {
	switch status {
	case RunStatusCompleted, RunStatusFailed, RunStatusCancelled, RunStatusAborted:
		return true
	default:
		return false
	}
}
