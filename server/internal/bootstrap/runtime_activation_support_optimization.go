package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/optimization"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func bindHarnessRuntimeOptimization(settings *serverpkg.SettingsHandler, bundle *HarnessRuntimeBundle) {
	if settings == nil || bundle == nil || bundle.Controller == nil {
		return
	}
	manager := settings.AgentcoreRunnerOptimizationManager()
	if manager == nil {
		return
	}
	if adapter := newHarnessOptimizationManagerAdapter(manager, bundle.Controller); adapter != nil {
		settings.SetAgentcoreRunnerManager(adapter)
	}
	triggerer := &harnessOptimizationTriggerer{
		manager:    manager,
		settings:   settings,
		controller: bundle.Controller,
	}
	if triggerer == nil {
		return
	}
	bundle.Controller.SetOptimizationTriggerer(triggerer)
}

type harnessOptimizationManagerAdapter struct {
	manager    *optimization.Manager
	controller *harness.Controller
}

type optimizationFollowupAssessment struct {
	Gate     string
	Passed   bool
	Decision string
	Summary  string
}

func newHarnessOptimizationManagerAdapter(manager *optimization.Manager, controller *harness.Controller) *harnessOptimizationManagerAdapter {
	if manager == nil {
		return nil
	}
	return &harnessOptimizationManagerAdapter{
		manager:    manager,
		controller: controller,
	}
}

func (a *harnessOptimizationManagerAdapter) OptimizationManager() *optimization.Manager {
	if a == nil {
		return nil
	}
	return a.manager
}

func (a *harnessOptimizationManagerAdapter) Prepare(ctx context.Context, req optimization.PrepareRequest) (optimization.Status, error) {
	if a == nil || a.manager == nil {
		return optimization.Status{}, fmt.Errorf("optimization manager is nil")
	}
	return a.manager.Prepare(ctx, req)
}

func (a *harnessOptimizationManagerAdapter) GetStatus(ctx context.Context) optimization.Status {
	if a == nil || a.manager == nil {
		return optimization.Status{}
	}
	status := a.manager.GetStatus(ctx)
	if strings.TrimSpace(status.LastOptimizationRunID) == "" {
		return status
	}
	if _, err := a.GetLastOptimizationRun(ctx); err != nil {
		return status
	}
	return a.manager.GetStatus(ctx)
}

func (a *harnessOptimizationManagerAdapter) GetLastOptimizationRun(ctx context.Context) (optimization.OptimizationRunRecord, error) {
	if a == nil || a.manager == nil {
		return nil, fmt.Errorf("optimization manager is nil")
	}
	record, err := a.manager.GetLastOptimizationRun(ctx)
	if err != nil || record == nil {
		return record, err
	}
	return a.reconcileFollowupOptimizationRecord(ctx, record)
}

func (a *harnessOptimizationManagerAdapter) reconcileFollowupOptimizationRecord(ctx context.Context, record optimization.OptimizationRunRecord) (optimization.OptimizationRunRecord, error) {
	if a == nil || a.manager == nil || a.controller == nil || len(record) == 0 {
		return record, nil
	}
	followupEvalRunID := optimizationMetadataString(record, "followup_eval_run_id")
	if followupEvalRunID == "" {
		return record, nil
	}

	updated := optimization.OptimizationRunRecord(cloneOptimizationMetadata(record))
	evalRun, err := a.controller.GetEvalRun(ctx, followupEvalRunID)
	if err != nil {
		updated["followup_state"] = "error"
		updated["followup_summary"] = strings.TrimSpace(err.Error())
		updated["followup_error"] = strings.TrimSpace(err.Error())
		return a.persistReconciledOptimizationRecord(ctx, updated)
	}
	delete(updated, "followup_error")
	if evalRun == nil {
		err := fmt.Errorf("follow-up eval run %q not found", followupEvalRunID)
		updated["followup_state"] = "error"
		updated["followup_summary"] = strings.TrimSpace(err.Error())
		updated["followup_error"] = strings.TrimSpace(err.Error())
		return a.persistReconciledOptimizationRecord(ctx, updated)
	}

	updated["followup_eval_run_id"] = strings.TrimSpace(evalRun.ID)
	updated["followup_eval_status"] = normalizeOptimizationFollowupEvalStatus(evalRun.Status)
	updated["followup_assessed_at"] = time.Now().UTC()

	if isOptimizationFollowupActive(evalRun.Status) {
		updated["followup_state"] = "running"
		updated["followup_decision"] = "running"
		updated["followup_summary"] = summarizeOptimizationFollowupActive(evalRun.Status)
		if optimizationMetadataString(updated, "promotion_state") != "promoted" {
			updated["promotion_state"] = string(harness.SkillRevisionStatusCandidate)
		}
		if err := a.reconcileSkillRevisionDecision(ctx, updated, harness.SkillRevisionStatusCandidate); err != nil {
			updated["followup_revision_error"] = strings.TrimSpace(err.Error())
		}
		return a.persistReconciledOptimizationRecord(ctx, updated)
	}

	if evalRun.Status != harness.RunGroupStatusCompleted {
		updated["followup_state"] = "rejected"
		updated["followup_decision"] = "rejected"
		updated["followup_gate_passed"] = false
		updated["followup_summary"] = summarizeOptimizationFollowupTerminal(evalRun.Status)
		if gate := optimizationFollowupGate(record); gate != "" {
			updated["followup_gate"] = gate
		}
		if optimizationMetadataString(updated, "promotion_state") != "promoted" {
			updated["promotion_state"] = string(harness.SkillRevisionStatusRejected)
		}
		if err := a.reconcileSkillRevisionDecision(ctx, updated, harness.SkillRevisionStatusRejected); err != nil {
			updated["followup_revision_error"] = strings.TrimSpace(err.Error())
		}
		return a.persistReconciledOptimizationRecord(ctx, updated)
	}

	assessment, err := a.assessCompletedFollowupEval(ctx, updated, evalRun)
	if err != nil {
		updated["followup_state"] = "completed"
		updated["followup_summary"] = strings.TrimSpace(err.Error())
		updated["followup_error"] = strings.TrimSpace(err.Error())
		if gate := optimizationFollowupGate(record); gate != "" {
			updated["followup_gate"] = gate
		}
		return a.persistReconciledOptimizationRecord(ctx, updated)
	}

	delete(updated, "followup_error")
	updated["followup_state"] = assessment.Decision
	updated["followup_decision"] = assessment.Decision
	updated["followup_gate"] = assessment.Gate
	updated["followup_gate_passed"] = assessment.Passed
	updated["followup_summary"] = assessment.Summary
	if optimizationMetadataString(updated, "promotion_state") != "promoted" {
		updated["promotion_state"] = assessment.Decision
	}
	if err := a.reconcileSkillRevisionDecision(ctx, updated, skillRevisionStatusForDecision(assessment.Decision)); err != nil {
		updated["followup_revision_error"] = strings.TrimSpace(err.Error())
	}
	return a.persistReconciledOptimizationRecord(ctx, updated)
}

func (a *harnessOptimizationManagerAdapter) assessCompletedFollowupEval(ctx context.Context, record optimization.OptimizationRunRecord, evalRun *harness.EvalRun) (*optimizationFollowupAssessment, error) {
	if a == nil || a.controller == nil {
		return nil, fmt.Errorf("harness controller is nil")
	}
	gate := optimizationFollowupGate(record)
	if gate == "" {
		return nil, fmt.Errorf("follow-up gate is unknown")
	}
	baseEvalRunID := firstNonEmptyOptimizationValue(
		optimizationMetadataString(record, "base_eval_run_id"),
		strings.TrimSpace(evalRun.BaselineEvalRunID),
	)

	assessment := &optimizationFollowupAssessment{Gate: gate}
	switch gate {
	case "selector":
		report, err := a.controller.EvaluateSelectorGate(ctx, evalRun.ID, harness.SelectorGateRequest{
			BaseEvalRunID: baseEvalRunID,
		})
		if err != nil {
			return nil, err
		}
		assessment.Passed = report != nil && report.Passed
	case "execution":
		report, err := a.controller.EvaluateExecutionEquivalence(ctx, evalRun.ID, harness.ExecutionEquivalenceRequest{
			BaseEvalRunID: baseEvalRunID,
		})
		if err != nil {
			return nil, err
		}
		assessment.Passed = report != nil && report.Passed
	case "budget":
		report, err := a.controller.EvaluateSkillCutoverBudgetGate(ctx, evalRun.ID, harness.SkillCutoverBudgetRequest{
			BaseEvalRunID: baseEvalRunID,
		})
		if err != nil {
			return nil, err
		}
		assessment.Passed = report != nil && report.Passed
	default:
		return nil, fmt.Errorf("follow-up gate %q is unsupported", gate)
	}

	if assessment.Passed {
		assessment.Decision = "accepted"
		assessment.Summary = fmt.Sprintf("Follow-up %s gate accepted the skill candidate.", gate)
		return assessment, nil
	}
	assessment.Decision = "rejected"
	assessment.Summary = fmt.Sprintf("Follow-up %s gate rejected the skill candidate.", gate)
	return assessment, nil
}

func (a *harnessOptimizationManagerAdapter) persistReconciledOptimizationRecord(ctx context.Context, record optimization.OptimizationRunRecord) (optimization.OptimizationRunRecord, error) {
	if a == nil || a.manager == nil || len(record) == 0 {
		return record, nil
	}
	recordID := optimizationMetadataString(record, "id")
	if recordID == "" {
		recordID = strings.TrimSpace(a.manager.GetStatus(ctx).LastOptimizationRunID)
	}
	if recordID == "" {
		return record, nil
	}
	record["id"] = recordID
	if err := a.manager.RecordOptimizationEvent(recordID, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (a *harnessOptimizationManagerAdapter) reconcileSkillRevisionDecision(ctx context.Context, record optimization.OptimizationRunRecord, status harness.SkillRevisionStatus) error {
	if a == nil || a.controller == nil || status == "" {
		return nil
	}
	revisionID := optimizationMetadataString(record, "skill_revision_id")
	if revisionID == "" {
		return nil
	}
	revision, err := a.controller.GetSkillRevision(ctx, revisionID)
	if err != nil {
		return err
	}
	if revision == nil {
		return nil
	}
	if revision.Status != status {
		revision.Status = status
		revision, err = a.controller.UpdateSkillRevision(ctx, *revision)
		if err != nil {
			return err
		}
	}
	return a.reconcileSkillEvolutionCaseDecision(ctx, revision, skillEvolutionCaseStatusForRevisionStatus(status))
}

func (a *harnessOptimizationManagerAdapter) reconcileSkillEvolutionCaseDecision(ctx context.Context, revision *harness.SkillRevision, status harness.SkillEvolutionCaseStatus) error {
	if a == nil || a.controller == nil || revision == nil || status == "" {
		return nil
	}
	caseID := strings.TrimSpace(revision.OriginCaseID)
	if caseID == "" {
		return nil
	}
	evolutionCase, err := a.controller.GetSkillEvolutionCase(ctx, caseID)
	if err != nil {
		return err
	}
	if evolutionCase == nil {
		return nil
	}
	changed := false
	if evolutionCase.Status != status {
		evolutionCase.Status = status
		changed = true
	}
	if strings.TrimSpace(evolutionCase.RevisionID) == "" {
		evolutionCase.RevisionID = strings.TrimSpace(revision.ID)
		changed = true
	}
	if !changed {
		return nil
	}
	evolutionCase.UpdatedAt = time.Now().UTC()
	_, err = a.controller.UpdateSkillEvolutionCase(ctx, *evolutionCase)
	return err
}

func skillRevisionStatusForDecision(decision string) harness.SkillRevisionStatus {
	switch strings.TrimSpace(decision) {
	case "accepted":
		return harness.SkillRevisionStatusAccepted
	case "rejected":
		return harness.SkillRevisionStatusRejected
	default:
		return harness.SkillRevisionStatusCandidate
	}
}

func skillEvolutionCaseStatusForRevisionStatus(status harness.SkillRevisionStatus) harness.SkillEvolutionCaseStatus {
	switch status {
	case harness.SkillRevisionStatusAccepted:
		return harness.SkillEvolutionCaseStatusAccepted
	case harness.SkillRevisionStatusRejected:
		return harness.SkillEvolutionCaseStatusRejected
	case harness.SkillRevisionStatusPromoted:
		return harness.SkillEvolutionCaseStatusPromoted
	case harness.SkillRevisionStatusCandidate:
		return harness.SkillEvolutionCaseStatusCandidateCreated
	default:
		return harness.SkillEvolutionCaseStatusOpen
	}
}

func optimizationFollowupGate(record map[string]interface{}) string {
	if gate := optimizationMetadataString(record, "followup_gate"); gate != "" {
		return gate
	}
	switch harness.OptimizationReason(optimizationMetadataString(record, "reason")) {
	case harness.OptimizationReasonRuntimeSkillFailure, harness.OptimizationReasonRuntimeSkillCapture:
		return "execution"
	case harness.OptimizationReasonSelectorGateFailed, harness.OptimizationReasonSelectorGatePassed:
		return "selector"
	case harness.OptimizationReasonExecutionGateFailed, harness.OptimizationReasonExecutionGatePassed:
		return "execution"
	case harness.OptimizationReasonBudgetGateFailed, harness.OptimizationReasonBudgetGatePassed:
		return "budget"
	default:
		return ""
	}
}

func isOptimizationFollowupActive(status harness.RunGroupStatus) bool {
	switch status {
	case harness.RunGroupStatusPending,
		harness.RunGroupStatusQueued,
		harness.RunGroupStatusRunning,
		harness.RunGroupStatusScoring:
		return true
	default:
		return false
	}
}

func summarizeOptimizationFollowupActive(status harness.RunGroupStatus) string {
	if state := normalizeOptimizationFollowupEvalStatus(status); state != "" {
		return fmt.Sprintf("Follow-up eval is still running (%s).", state)
	}
	return "Follow-up eval is still running."
}

func summarizeOptimizationFollowupTerminal(status harness.RunGroupStatus) string {
	state := normalizeOptimizationFollowupEvalStatus(status)
	if state == "" {
		state = "unknown"
	}
	return fmt.Sprintf("Follow-up eval ended with status %s and was rejected.", state)
}

func normalizeOptimizationFollowupEvalStatus(status harness.RunGroupStatus) string {
	switch status {
	case harness.RunGroupStatusPending, harness.RunGroupStatusQueued:
		return string(harness.RunGroupStatusPending)
	case harness.RunGroupStatusRunning, harness.RunGroupStatusScoring:
		return string(harness.RunGroupStatusRunning)
	default:
		return strings.TrimSpace(string(status))
	}
}

type harnessOptimizationTriggerer struct {
	manager    harnessOptimizationRunnerManager
	settings   *serverpkg.SettingsHandler
	controller *harness.Controller
}

type harnessOptimizationRunnerManager interface {
	GetStatus(ctx context.Context) optimization.Status
	Prepare(ctx context.Context, req optimization.PrepareRequest) (optimization.Status, error)
	SetLastOptimizationRunID(id string) error
	RecordOptimizationEvent(id string, payload interface{}) error
	GetOptimizationRun(ctx context.Context, id string) (optimization.OptimizationRunRecord, error)
	ExecutePreparedRunnerACP(ctx context.Context, prompt string) (optimization.RunnerExecutionResult, error)
}

func (t *harnessOptimizationTriggerer) RecordSkillRevisionPromotion(ctx context.Context, promotedRevision *harness.SkillRevision, backupRevision *harness.SkillRevision, writtenSourcePath string) error {
	if t == nil || t.manager == nil || promotedRevision == nil {
		return nil
	}
	runID := strings.TrimSpace(promotedRevision.OptimizationRunID)
	if runID == "" {
		return nil
	}
	record, err := t.manager.GetOptimizationRun(ctx, runID)
	if err != nil {
		return err
	}
	if len(record) == 0 {
		return nil
	}
	updated := optimization.OptimizationRunRecord(cloneOptimizationMetadata(record))
	updated["id"] = runID
	updated["skill_revision_id"] = strings.TrimSpace(promotedRevision.ID)
	updated["promotion_state"] = "promoted"
	updated["written_source_path"] = strings.TrimSpace(writtenSourcePath)
	if backupRevision != nil {
		updated["backup_revision_id"] = strings.TrimSpace(backupRevision.ID)
	}
	return t.manager.RecordOptimizationEvent(runID, updated)
}

func (t *harnessOptimizationTriggerer) TriggerOptimization(ctx context.Context, event harness.OptimizationTrigger) error {
	if t == nil || t.manager == nil || t.settings == nil {
		return nil
	}
	if !t.settings.GetExperimentalAgentcoreRunnerEnabled() {
		return nil
	}
	runID := uuid.NewString()
	optimizedParts, _ := optimization.NormalizeRequestedEvolvableParts([]string{defaultOptimizationSurface(event.OptimizationSurface)})
	primaryPart := ""
	if len(optimizedParts) > 0 {
		primaryPart = optimizedParts[0]
	}
	repoURL := t.settings.GetExperimentalAgentcoreRunnerRepoURL()
	runnerRef := t.settings.ResolveExperimentalAgentcoreRunnerRef(ctx, optimizationConversationID(event.Metadata))
	status, err := t.prepareOptimizationRunnerIfNeeded(ctx, repoURL, runnerRef, optimizedParts)
	if err != nil {
		return err
	}
	if !status.BinaryReady {
		return fmt.Errorf("runner binary is not ready for ref %q", runnerRef)
	}
	manifestPath := strings.TrimSpace(status.ManifestPath)
	if manifestPath == "" {
		manifestPath = optimization.RunnerArtifactManifestPath(status.BinaryPath)
	}
	record := map[string]interface{}{
		"id":                         runID,
		"reason":                     event.Reason,
		"candidate_id":               strings.TrimSpace(event.CandidateID),
		"eval_run_id":                strings.TrimSpace(event.EvalRunID),
		"base_eval_run_id":           strings.TrimSpace(event.BaseEvalRunID),
		"optimization_surface":       defaultOptimizationSurface(event.OptimizationSurface),
		"supported_parts":            optimization.DefaultSupportedEvolvableParts(),
		"optimized_parts":            optimizedParts,
		"primary_part":               primaryPart,
		"source_optimization_run_id": runID,
		"source_eval_run_id":         strings.TrimSpace(event.EvalRunID),
		"repo_url":                   firstNonEmptyOptimizationValue(strings.TrimSpace(status.RepoURL), repoURL),
		"ref":                        firstNonEmptyOptimizationValue(strings.TrimSpace(status.ResolvedRef), runnerRef),
		"runner_artifact_path":       status.BinaryPath,
		"runner_artifact_sha256":     status.BinarySHA256,
		"manifest_path":              manifestPath,
		"created_at":                 time.Now().UTC(),
		"metadata":                   cloneOptimizationMetadata(event.Metadata),
	}
	if gate := optimizationMetadataString(event.Metadata, "followup_gate"); gate != "" {
		record["followup_gate"] = gate
	}
	if err := t.manager.SetLastOptimizationRunID(runID); err != nil {
		return err
	}
	runnerStartedAt := time.Now().UTC()
	result, execErr := t.manager.ExecutePreparedRunnerACP(ctx, buildOptimizationRunnerPrompt(event))
	runnerFinishedAt := time.Now().UTC()
	record["runner_protocol"] = result.Protocol
	record["runner_session_id"] = result.SessionID
	record["runner_stop_reason"] = result.StopReason
	record["runner_response_text"] = result.ResponseText
	record["runner_stderr"] = result.Stderr
	record["runner_transcript"] = result.Transcript
	record["runner_started_at"] = runnerStartedAt
	record["runner_finished_at"] = runnerFinishedAt
	record["runner_duration_ms"] = runnerFinishedAt.Sub(runnerStartedAt).Milliseconds()
	if execErr != nil {
		record["runner_error"] = strings.TrimSpace(execErr.Error())
	}
	var followupErr error
	if execErr == nil {
		followup, err := t.maybeSubmitOptimizationFollowupEval(ctx, runID, event, result.ResponseText)
		if followup != nil {
			if state := strings.TrimSpace(followup.State); state != "" {
				record["followup_state"] = state
			}
			if message := strings.TrimSpace(followup.Message); message != "" {
				record["followup_message"] = message
			}
			if len(followup.SkillCandidate) > 0 {
				record["materialized_skill_candidate"] = followup.SkillCandidate
			}
			if skippedReason := strings.TrimSpace(followup.SkippedReason); skippedReason != "" {
				record["followup_skipped_reason"] = skippedReason
			}
			if followup.EvolutionCase != nil {
				record["skill_evolution_case_id"] = strings.TrimSpace(followup.EvolutionCase.ID)
			}
			if followup.SkillRevision != nil {
				record["skill_revision_id"] = strings.TrimSpace(followup.SkillRevision.ID)
				record["promotion_state"] = string(harness.SkillRevisionStatusCandidate)
			}
			if followup.EvalRun != nil {
				record["followup_eval_run_id"] = strings.TrimSpace(followup.EvalRun.ID)
				record["followup_group_id"] = strings.TrimSpace(followup.EvalRun.GroupID)
				record["followup_eval_spec_id"] = strings.TrimSpace(followup.EvalRun.EvalSpecID)
			}
		}
		if err != nil {
			followupErr = err
			record["followup_error"] = strings.TrimSpace(err.Error())
		}
	}
	if err := t.manager.RecordOptimizationEvent(runID, record); err != nil {
		return err
	}
	if execErr != nil {
		return execErr
	}
	return followupErr
}

func (t *harnessOptimizationTriggerer) prepareOptimizationRunnerIfNeeded(
	ctx context.Context,
	repoURL string,
	ref string,
	requestedParts []string,
) (optimization.Status, error) {
	if t == nil || t.manager == nil {
		return optimization.Status{}, fmt.Errorf("optimization manager is nil")
	}

	repoURL = strings.TrimSpace(repoURL)
	ref = strings.TrimSpace(ref)
	status := t.manager.GetStatus(ctx)
	currentRepoURL := strings.TrimSpace(status.RepoURL)
	currentRef := strings.TrimSpace(status.ResolvedRef)

	if status.BinaryReady && currentRepoURL == repoURL && currentRef == ref {
		return status, nil
	}

	prepared, err := t.manager.Prepare(ctx, optimization.PrepareRequest{
		RepoURL:        repoURL,
		Ref:            ref,
		RequestedParts: requestedParts,
	})
	if err != nil {
		return prepared, err
	}
	if strings.TrimSpace(prepared.RepoURL) == "" {
		prepared.RepoURL = repoURL
	}
	if strings.TrimSpace(prepared.ResolvedRef) == "" {
		prepared.ResolvedRef = ref
	}
	return prepared, nil
}

func optimizationConversationID(meta map[string]interface{}) string {
	return workflowTriggerString(meta, "conversation_id", "conversationId", "session_id", "sessionId", "session")
}

func defaultOptimizationSurface(surface harness.OptimizationSurface) string {
	if strings.TrimSpace(string(surface)) != "" {
		return string(surface)
	}
	return string(harness.OptimizationSurfaceConstraints)
}

func cloneOptimizationMetadata(source map[string]interface{}) map[string]interface{} {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(source))
	for key, value := range source {
		out[key] = value
	}
	return out
}

func buildOptimizationRunnerPrompt(event harness.OptimizationTrigger) string {
	parts := []string{
		"Optimization trigger received.",
		fmt.Sprintf("Reason: %s", strings.TrimSpace(string(event.Reason))),
		fmt.Sprintf("Candidate ID: %s", strings.TrimSpace(event.CandidateID)),
		fmt.Sprintf("Eval Run ID: %s", strings.TrimSpace(event.EvalRunID)),
		fmt.Sprintf("Base Eval Run ID: %s", strings.TrimSpace(event.BaseEvalRunID)),
		fmt.Sprintf("Optimization Surface: %s", defaultOptimizationSurface(event.OptimizationSurface)),
	}
	if metadata := cloneOptimizationMetadata(event.Metadata); len(metadata) > 0 {
		if data, err := json.Marshal(metadata); err == nil {
			parts = append(parts, "Metadata: "+string(data))
		}
		if skillParts := buildOptimizationSkillPromptParts(metadata); len(skillParts) > 0 {
			parts = append(parts, skillParts...)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func buildOptimizationSkillPromptParts(metadata map[string]interface{}) []string {
	candidate := optimizationMetadataMap(metadata, "skill_candidate")
	if len(candidate) == 0 {
		return nil
	}

	parts := []string{
		fmt.Sprintf("Skill ID: %s", optimizationMetadataString(candidate, "skill_id")),
		fmt.Sprintf("Skill Candidate ID: %s", firstNonEmptyOptimizationValue(
			optimizationMetadataString(candidate, "candidate_id"),
			optimizationMetadataString(metadata, "candidate_id"),
		)),
		fmt.Sprintf("Skill Source Path: %s", optimizationMetadataString(candidate, "source_path")),
		fmt.Sprintf("Applied Skill Path: %s", optimizationMetadataString(candidate, "applied_path")),
		fmt.Sprintf("Skill SHA256: %s", optimizationMetadataString(candidate, "sha256")),
	}
	if content := optimizationMetadataRawString(candidate, "content"); content != "" {
		parts = append(parts, "SKILL.md Content:\n"+content)
	}
	parts = append(parts,
		"Return JSON only with this schema:",
		`{"status":"candidate_ready|no_change","message":"...","skill_candidate":{"skill_id":"...","candidate_id":"...","source_path":"...","content":"..."}}`,
		"When status is candidate_ready, include the full replacement SKILL.md text in skill_candidate.content.",
	)
	return filterEmptyOptimizationPromptParts(parts)
}

func optimizationMetadataMap(meta map[string]interface{}, key string) map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	value, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	return value
}

func optimizationMetadataString(meta map[string]interface{}, key string) string {
	if len(meta) == 0 {
		return ""
	}
	raw, ok := meta[key]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func optimizationMetadataRawString(meta map[string]interface{}, key string) string {
	if len(meta) == 0 {
		return ""
	}
	raw, ok := meta[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return value
}

func firstNonEmptyOptimizationValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func filterEmptyOptimizationPromptParts(parts []string) []string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(strings.TrimPrefix(part, "SKILL.md Content:\n")) == "" {
			continue
		}
		if strings.HasSuffix(part, ": ") {
			continue
		}
		filtered = append(filtered, part)
	}
	return filtered
}

type optimizationFollowupOutcome struct {
	State          string
	Message        string
	SkippedReason  string
	SkillCandidate map[string]interface{}
	EvolutionCase  *harness.SkillEvolutionCase
	SkillRevision  *harness.SkillRevision
	EvalRun        *harness.EvalRun
}

type optimizationSkillCandidateResponse struct {
	Status         string                            `json:"status,omitempty"`
	Message        string                            `json:"message,omitempty"`
	SkillCandidate *optimizationSkillCandidateResult `json:"skill_candidate,omitempty"`
}

type optimizationSkillCandidateResult struct {
	SkillID     string `json:"skill_id,omitempty"`
	CandidateID string `json:"candidate_id,omitempty"`
	SourcePath  string `json:"source_path,omitempty"`
	Content     string `json:"content,omitempty"`
}

func (t *harnessOptimizationTriggerer) maybeSubmitOptimizationFollowupEval(ctx context.Context, optimizationRunID string, event harness.OptimizationTrigger, responseText string) (*optimizationFollowupOutcome, error) {
	outcome := &optimizationFollowupOutcome{}
	if t == nil || t.controller == nil {
		return outcome, nil
	}
	if defaultOptimizationSurface(event.OptimizationSurface) != string(harness.OptimizationSurfaceSkillDefinition) {
		return outcome, nil
	}

	response, err := parseOptimizationSkillCandidateResponse(responseText)
	if err != nil {
		outcome.State = "parse_error"
		return outcome, err
	}
	if response == nil {
		outcome.State = "skipped"
		outcome.Message = "empty response"
		return outcome, nil
	}
	outcome.State = strings.TrimSpace(response.Status)
	outcome.Message = strings.TrimSpace(response.Message)
	if outcome.State == "" {
		if response.SkillCandidate != nil {
			outcome.State = "candidate_ready"
		} else {
			outcome.State = "no_change"
		}
	}
	if outcome.State != "candidate_ready" || response.SkillCandidate == nil {
		if outcome.State == "" {
			outcome.State = "no_change"
		}
		return outcome, nil
	}

	skillCandidate, err := materializeOptimizationSkillCandidate(event, response.SkillCandidate)
	if err != nil {
		outcome.State = "candidate_invalid"
		return outcome, err
	}
	outcome.SkillCandidate = skillCandidate

	var parentEvalRun *harness.EvalRun
	if strings.TrimSpace(event.EvalRunID) != "" {
		parentEvalRun, err = t.controller.GetEvalRun(ctx, event.EvalRunID)
		if err != nil {
			return outcome, err
		}
	}
	sourceState, err := harness.ResolveWritableSkillSourceState(
		optimizationMetadataString(skillCandidate, "skill_id"),
		optimizationMetadataString(skillCandidate, "source_path"),
	)
	if err != nil {
		outcome.State = "skipped"
		outcome.SkippedReason = strings.TrimSpace(err.Error())
		outcome.Message = firstNonEmptyOptimizationValue(
			outcome.Message,
			"skill evolution v1 only supports writable built-in skills and managed installed skills",
		)
		return outcome, nil
	}
	evolutionCase, created, err := t.ensureOptimizationSkillEvolutionCase(ctx, optimizationRunID, event, parentEvalRun, skillCandidate, sourceState)
	if err != nil {
		outcome.State = "candidate_invalid"
		return outcome, err
	}
	outcome.EvolutionCase = evolutionCase
	if !created && evolutionCase != nil && strings.TrimSpace(evolutionCase.RevisionID) != "" {
		revision, getErr := t.controller.GetSkillRevision(ctx, evolutionCase.RevisionID)
		if getErr != nil {
			return outcome, getErr
		}
		outcome.SkillRevision = revision
		outcome.State = "deduped"
		outcome.Message = firstNonEmptyOptimizationValue(outcome.Message, "reused existing skill evolution candidate")
		return outcome, nil
	}
	originCaseID := ""
	if evolutionCase != nil {
		originCaseID = strings.TrimSpace(evolutionCase.ID)
	}
	followupGate := firstNonEmptyOptimizationValue(
		optimizationMetadataString(event.Metadata, "followup_gate"),
		optimizationFollowupGate(map[string]interface{}{"reason": string(event.Reason)}),
	)
	followupTarget, err := t.resolveOptimizationFollowupTarget(ctx, event, parentEvalRun, followupGate)
	if err != nil {
		outcome.State = "skipped"
		outcome.SkippedReason = strings.TrimSpace(err.Error())
		outcome.Message = firstNonEmptyOptimizationValue(outcome.Message, "runtime follow-up assets are not ready")
		return outcome, nil
	}
	revisionID := optimizationMetadataString(event.Metadata, "candidate_revision_id")
	var revision *harness.SkillRevision
	if revisionID != "" {
		existingRevision, getErr := t.controller.GetSkillRevision(ctx, revisionID)
		if getErr != nil {
			return outcome, getErr
		}
		if existingRevision != nil {
			existingRevision.SkillID = optimizationMetadataString(skillCandidate, "skill_id")
			existingRevision.Status = harness.SkillRevisionStatusCandidate
			existingRevision.SourcePath = strings.TrimSpace(sourceState.NormalizedPath)
			existingRevision.CandidateID = optimizationMetadataString(skillCandidate, "candidate_id")
			existingRevision.BaseContentSHA256 = strings.TrimSpace(sourceState.ContentSHA256)
			existingRevision.OriginCaseID = originCaseID
			existingRevision.EvalRunID = strings.TrimSpace(event.EvalRunID)
			existingRevision.OptimizationRunID = strings.TrimSpace(optimizationRunID)
			existingRevision.FollowupGate = followupGate
			existingRevision.OptimizationSurface = harness.OptimizationSurface(defaultOptimizationSurface(event.OptimizationSurface))
			existingRevision.Content = optimizationMetadataRawString(skillCandidate, "content")
			revision, err = t.controller.UpdateSkillRevision(ctx, *existingRevision)
			if err != nil {
				outcome.State = "candidate_invalid"
				return outcome, err
			}
		}
	}
	if revision == nil {
		revision, err = t.controller.CreateSkillRevision(ctx, harness.SkillRevision{
			SkillID:             optimizationMetadataString(skillCandidate, "skill_id"),
			Status:              harness.SkillRevisionStatusCandidate,
			SourcePath:          strings.TrimSpace(sourceState.NormalizedPath),
			CandidateID:         optimizationMetadataString(skillCandidate, "candidate_id"),
			BaseContentSHA256:   strings.TrimSpace(sourceState.ContentSHA256),
			OriginCaseID:        originCaseID,
			EvalRunID:           strings.TrimSpace(event.EvalRunID),
			OptimizationRunID:   strings.TrimSpace(optimizationRunID),
			FollowupGate:        followupGate,
			OptimizationSurface: harness.OptimizationSurface(defaultOptimizationSurface(event.OptimizationSurface)),
			Content:             optimizationMetadataRawString(skillCandidate, "content"),
		})
		if err != nil {
			outcome.State = "candidate_invalid"
			return outcome, err
		}
	}
	outcome.SkillRevision = revision
	if evolutionCase != nil {
		evolutionCase.CandidateID = optimizationMetadataString(skillCandidate, "candidate_id")
		evolutionCase.BaseContentSHA256 = strings.TrimSpace(sourceState.ContentSHA256)
		evolutionCase.RevisionID = strings.TrimSpace(revision.ID)
		evolutionCase.Status = harness.SkillEvolutionCaseStatusCandidateCreated
		evolutionCase.UpdatedAt = time.Now().UTC()
		evolutionCase, err = t.controller.UpdateSkillEvolutionCase(ctx, *evolutionCase)
		if err != nil {
			return outcome, err
		}
		outcome.EvolutionCase = evolutionCase
	}
	followupEvalRun, err := t.controller.SubmitEvalRun(ctx, harness.EvalRunSpec{
		EvalSpecID:        strings.TrimSpace(followupTarget.EvalSpecID),
		BaselineEvalRunID: strings.TrimSpace(followupTarget.BaseEvalRunID),
		Title:             buildOptimizationFollowupTitle(parentEvalRun, followupTarget.TitleBase, optimizationMetadataString(skillCandidate, "candidate_id")),
		OwnerUserID:       strings.TrimSpace(followupTarget.OwnerUserID),
		TriggerKind:       "optimization_followup",
		TriggerRef:        strings.TrimSpace(optimizationRunID),
		Metadata:          buildOptimizationFollowupMetadata(event, optimizationRunID, skillCandidate, revision),
	})
	if err != nil {
		return outcome, err
	}
	if revision != nil && strings.TrimSpace(revision.EvalRunID) != strings.TrimSpace(followupEvalRun.ID) {
		revision.EvalRunID = strings.TrimSpace(followupEvalRun.ID)
		revision, err = t.controller.UpdateSkillRevision(ctx, *revision)
		if err != nil {
			return outcome, err
		}
		outcome.SkillRevision = revision
	}
	outcome.State = "submitted"
	outcome.EvalRun = followupEvalRun
	return outcome, nil
}

func (t *harnessOptimizationTriggerer) ensureOptimizationSkillEvolutionCase(
	ctx context.Context,
	optimizationRunID string,
	event harness.OptimizationTrigger,
	parentEvalRun *harness.EvalRun,
	skillCandidate map[string]interface{},
	sourceState *harness.CanonicalSkillSourceState,
) (*harness.SkillEvolutionCase, bool, error) {
	if t == nil || t.controller == nil || len(skillCandidate) == 0 || sourceState == nil {
		return nil, false, nil
	}
	evidence, err := json.Marshal(buildOptimizationSkillEvolutionEvidence(optimizationRunID, event, parentEvalRun, skillCandidate, sourceState))
	if err != nil {
		return nil, false, err
	}
	return t.controller.EnsureSkillEvolutionCase(ctx, harness.SkillEvolutionCaseSpec{
		SkillID:           optimizationMetadataString(skillCandidate, "skill_id"),
		OwnerUserID:       optimizationFollowupOwnerUserID(event, parentEvalRun),
		Mode:              skillEvolutionModeForOptimizationReason(event.Reason),
		Reason:            skillEvolutionReasonForOptimizationReason(event.Reason),
		SourceKind:        optimizationSkillEvolutionSourceKind(event, parentEvalRun),
		SourceID:          optimizationSkillEvolutionSourceID(event, optimizationRunID, parentEvalRun),
		CandidateID:       optimizationMetadataString(skillCandidate, "candidate_id"),
		BaseContentSHA256: strings.TrimSpace(sourceState.ContentSHA256),
		FailureSignature:  optimizationSkillEvolutionFailureSignature(event),
		Summary:           buildOptimizationSkillEvolutionSummary(event, skillCandidate),
		EvidenceJSON:      string(evidence),
	})
}

func parseOptimizationSkillCandidateResponse(responseText string) (*optimizationSkillCandidateResponse, error) {
	trimmed := strings.TrimSpace(responseText)
	if trimmed == "" {
		return nil, fmt.Errorf("optimization runner response is empty")
	}
	var parsed optimizationSkillCandidateResponse
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return &parsed, nil
	}
	jsonObject := extractOptimizationJSONObject(trimmed)
	if jsonObject == "" {
		return nil, fmt.Errorf("optimization runner response did not contain a JSON object")
	}
	if err := json.Unmarshal([]byte(jsonObject), &parsed); err != nil {
		return nil, fmt.Errorf("decode optimization runner JSON candidate: %w", err)
	}
	return &parsed, nil
}

func extractOptimizationJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}

func materializeOptimizationSkillCandidate(event harness.OptimizationTrigger, candidate *optimizationSkillCandidateResult) (map[string]interface{}, error) {
	source := optimizationMetadataMap(event.Metadata, "skill_candidate")
	skillID := firstNonEmptyOptimizationValue(strings.TrimSpace(candidate.SkillID), optimizationMetadataString(source, "skill_id"))
	if strings.TrimSpace(skillID) == "" {
		return nil, fmt.Errorf("optimization skill candidate skill_id is required")
	}
	content := candidate.Content
	if content == "" {
		content = optimizationMetadataRawString(source, "content")
	}
	if content == "" {
		return nil, fmt.Errorf("optimization skill candidate content is required")
	}
	sum := sha256.Sum256([]byte(content))
	sha := hex.EncodeToString(sum[:])
	candidateID := firstNonEmptyOptimizationValue(
		strings.TrimSpace(candidate.CandidateID),
		strings.TrimSpace(event.CandidateID),
		optimizationMetadataString(source, "candidate_id"),
		fmt.Sprintf("%s-%s", skillID, sha[:12]),
	)
	materialized := map[string]interface{}{
		"skill_id":     skillID,
		"candidate_id": candidateID,
		"source_path": firstNonEmptyOptimizationValue(
			strings.TrimSpace(candidate.SourcePath),
			optimizationMetadataString(source, "source_path"),
		),
		"content": content,
		"sha256":  sha,
	}
	return materialized, nil
}

func buildOptimizationSkillEvolutionEvidence(
	optimizationRunID string,
	event harness.OptimizationTrigger,
	parentEvalRun *harness.EvalRun,
	skillCandidate map[string]interface{},
	sourceState *harness.CanonicalSkillSourceState,
) map[string]interface{} {
	evidence := map[string]interface{}{
		"optimization_run_id": strings.TrimSpace(optimizationRunID),
		"reason":              strings.TrimSpace(string(event.Reason)),
		"eval_run_id":         strings.TrimSpace(event.EvalRunID),
		"base_eval_run_id":    strings.TrimSpace(event.BaseEvalRunID),
		"followup_gate": firstNonEmptyOptimizationValue(
			optimizationMetadataString(event.Metadata, "followup_gate"),
			optimizationFollowupGate(map[string]interface{}{"reason": string(event.Reason)}),
		),
		"skill_candidate":     skillCandidate,
		"base_content_sha256": strings.TrimSpace(sourceState.ContentSHA256),
		"source_path":         strings.TrimSpace(sourceState.NormalizedPath),
		"owner_user_id":       optimizationFollowupOwnerUserID(event, parentEvalRun),
	}
	if parentEvalRun != nil && strings.TrimSpace(parentEvalRun.Title) != "" {
		evidence["parent_eval_title"] = strings.TrimSpace(parentEvalRun.Title)
	}
	for _, key := range []string{
		"runtime_run_id",
		"runtime_kind",
		"runtime_model",
		"runtime_status",
		"runtime_state",
		"goal",
		"result_summary",
		"error",
		"selected_canonical_skill",
		"failure_signature",
		"runtime_failure_signature",
		"capture_signature",
		"runtime_reason_summary",
	} {
		if value := optimizationMetadataString(event.Metadata, key); value != "" {
			evidence[key] = value
		}
	}
	for _, key := range []string{
		"runtime_metrics",
		"runtime_usage",
		"runtime_quality",
		"validation",
		"selector_dry_run_response",
		"skill_candidate",
	} {
		if value := optimizationMetadataMap(event.Metadata, key); len(value) > 0 {
			evidence[key] = cloneOptimizationMetadata(value)
		}
	}
	for _, key := range []string{
		"runtime_event_summaries",
		"runtime_capture_lessons",
	} {
		if raw, ok := event.Metadata[key]; ok && raw != nil {
			evidence[key] = raw
		}
	}
	if occurrences := optimizationMetadataString(event.Metadata, "runtime_capture_occurrences"); occurrences != "" {
		evidence["runtime_capture_occurrences"] = occurrences
	}
	if threshold := optimizationMetadataString(event.Metadata, "runtime_capture_threshold"); threshold != "" {
		evidence["runtime_capture_threshold"] = threshold
	}
	return evidence
}

func skillEvolutionModeForOptimizationReason(reason harness.OptimizationReason) harness.SkillEvolutionMode {
	switch reason {
	case harness.OptimizationReasonRuntimeSkillCapture:
		return harness.SkillEvolutionModeCapture
	default:
		return harness.SkillEvolutionModeFix
	}
}

func skillEvolutionReasonForOptimizationReason(reason harness.OptimizationReason) harness.SkillEvolutionReason {
	switch reason {
	case harness.OptimizationReasonRuntimeSkillFailure:
		return harness.SkillEvolutionReasonRuntimeFailure
	case harness.OptimizationReasonRuntimeSkillCapture:
		return harness.SkillEvolutionReasonRuntimeCapture
	case harness.OptimizationReasonSelectorGateFailed, harness.OptimizationReasonSelectorGatePassed:
		return harness.SkillEvolutionReasonSelectorGateFailed
	case harness.OptimizationReasonExecutionGateFailed, harness.OptimizationReasonExecutionGatePassed:
		return harness.SkillEvolutionReasonExecutionGateFailed
	case harness.OptimizationReasonBudgetGateFailed, harness.OptimizationReasonBudgetGatePassed:
		return harness.SkillEvolutionReasonBudgetGateFailed
	default:
		return harness.SkillEvolutionReasonManual
	}
}

func optimizationSkillEvolutionFailureSignature(event harness.OptimizationTrigger) string {
	for _, key := range []string{"failure_signature", "runtime_failure_signature", "capture_signature"} {
		if value := optimizationMetadataString(event.Metadata, key); value != "" {
			return value
		}
	}
	switch event.Reason {
	case harness.OptimizationReasonSelectorGateFailed, harness.OptimizationReasonSelectorGatePassed:
		return "selector-gate-failed"
	case harness.OptimizationReasonExecutionGateFailed, harness.OptimizationReasonExecutionGatePassed:
		return "execution-gate-failed"
	case harness.OptimizationReasonBudgetGateFailed, harness.OptimizationReasonBudgetGatePassed:
		return "budget-gate-failed"
	case harness.OptimizationReasonRuntimeSkillCapture:
		return "runtime-capture"
	case harness.OptimizationReasonRuntimeSkillFailure:
		return "runtime-failure"
	default:
		return strings.ReplaceAll(strings.TrimSpace(string(event.Reason)), "_", "-")
	}
}

func buildOptimizationSkillEvolutionSummary(event harness.OptimizationTrigger, skillCandidate map[string]interface{}) string {
	skillID := optimizationMetadataString(skillCandidate, "skill_id")
	switch skillEvolutionModeForOptimizationReason(event.Reason) {
	case harness.SkillEvolutionModeCapture:
		return fmt.Sprintf("Captured new grounded runtime experience for skill %s.", skillID)
	default:
		return fmt.Sprintf("Generated a candidate revision to repair skill %s.", skillID)
	}
}

func optimizationEvalOwnerUserID(evalRun *harness.EvalRun) string {
	if evalRun == nil {
		return ""
	}
	return strings.TrimSpace(evalRun.OwnerUserID)
}

func optimizationFollowupOwnerUserID(event harness.OptimizationTrigger, evalRun *harness.EvalRun) string {
	if ownerUserID := optimizationEvalOwnerUserID(evalRun); ownerUserID != "" {
		return ownerUserID
	}
	return strings.TrimSpace(optimizationMetadataString(event.Metadata, "owner_user_id"))
}

func optimizationSkillEvolutionSourceKind(event harness.OptimizationTrigger, parentEvalRun *harness.EvalRun) string {
	if runtimeRunID := optimizationMetadataString(event.Metadata, "runtime_run_id"); runtimeRunID != "" {
		return "runtime_run"
	}
	if parentEvalRun != nil || strings.TrimSpace(event.EvalRunID) != "" {
		return "eval_run"
	}
	return "optimization_run"
}

func optimizationSkillEvolutionSourceID(event harness.OptimizationTrigger, optimizationRunID string, parentEvalRun *harness.EvalRun) string {
	if runtimeRunID := optimizationMetadataString(event.Metadata, "runtime_run_id"); runtimeRunID != "" {
		return runtimeRunID
	}
	if parentEvalRun != nil && strings.TrimSpace(parentEvalRun.ID) != "" {
		return strings.TrimSpace(parentEvalRun.ID)
	}
	if strings.TrimSpace(event.EvalRunID) != "" {
		return strings.TrimSpace(event.EvalRunID)
	}
	return strings.TrimSpace(optimizationRunID)
}

func buildOptimizationFollowupMetadata(event harness.OptimizationTrigger, optimizationRunID string, skillCandidate map[string]interface{}, revision *harness.SkillRevision) map[string]interface{} {
	metadata := cloneOptimizationMetadata(event.Metadata)
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	optimizedParts, _ := optimization.NormalizeRequestedEvolvableParts([]string{defaultOptimizationSurface(event.OptimizationSurface)})
	metadata["optimization_run"] = true
	metadata["optimization_parent_run_id"] = firstNonEmptyOptimizationValue(
		strings.TrimSpace(event.EvalRunID),
		optimizationMetadataString(metadata, "runtime_run_id"),
	)
	metadata["optimization_run_id"] = strings.TrimSpace(optimizationRunID)
	metadata["optimization_surface"] = defaultOptimizationSurface(event.OptimizationSurface)
	if gate := firstNonEmptyOptimizationValue(
		optimizationMetadataString(metadata, "followup_gate"),
		optimizationFollowupGate(map[string]interface{}{"reason": string(event.Reason)}),
	); gate != "" {
		metadata["followup_gate"] = gate
	}
	metadata["supported_parts"] = optimization.DefaultSupportedEvolvableParts()
	metadata["optimized_parts"] = optimizedParts
	if len(optimizedParts) > 0 {
		metadata["primary_part"] = optimizedParts[0]
	}
	if candidateID := optimizationMetadataString(skillCandidate, "candidate_id"); candidateID != "" {
		metadata["candidate_id"] = candidateID
	}
	if revision != nil && strings.TrimSpace(revision.ID) != "" {
		metadata["skill_revision_id"] = strings.TrimSpace(revision.ID)
		metadata["candidate_revision_id"] = strings.TrimSpace(revision.ID)
	}
	if revision != nil && strings.TrimSpace(revision.OriginCaseID) != "" {
		metadata["origin_case_id"] = strings.TrimSpace(revision.OriginCaseID)
	}
	if revision != nil && strings.TrimSpace(revision.BaseContentSHA256) != "" {
		metadata["base_content_sha256"] = strings.TrimSpace(revision.BaseContentSHA256)
	}
	metadata["skill_candidate"] = skillCandidate
	return metadata
}

func buildOptimizationFollowupTitle(parentEvalRun *harness.EvalRun, titleBase string, candidateID string) string {
	base := strings.TrimSpace(titleBase)
	if base == "" {
		base = "optimization follow-up"
	}
	if parentEvalRun != nil && strings.TrimSpace(parentEvalRun.Title) != "" {
		base = strings.TrimSpace(parentEvalRun.Title)
	}
	if strings.TrimSpace(candidateID) == "" {
		return base
	}
	return fmt.Sprintf("%s / %s", base, strings.TrimSpace(candidateID))
}

type optimizationFollowupTarget struct {
	EvalSpecID    string
	BaseEvalRunID string
	OwnerUserID   string
	TitleBase     string
}

func (t *harnessOptimizationTriggerer) resolveOptimizationFollowupTarget(ctx context.Context, event harness.OptimizationTrigger, parentEvalRun *harness.EvalRun, followupGate string) (*optimizationFollowupTarget, error) {
	if parentEvalRun != nil {
		return &optimizationFollowupTarget{
			EvalSpecID:    strings.TrimSpace(parentEvalRun.EvalSpecID),
			BaseEvalRunID: firstNonEmptyOptimizationValue(strings.TrimSpace(event.BaseEvalRunID), strings.TrimSpace(parentEvalRun.BaselineEvalRunID)),
			OwnerUserID:   strings.TrimSpace(parentEvalRun.OwnerUserID),
			TitleBase:     strings.TrimSpace(parentEvalRun.Title),
		}, nil
	}
	ownerUserID := strings.TrimSpace(optimizationMetadataString(event.Metadata, "owner_user_id"))
	if ownerUserID == "" {
		return nil, fmt.Errorf("runtime follow-up requires owner_user_id")
	}
	switch strings.TrimSpace(followupGate) {
	case "", "execution":
		assets, err := t.controller.EnsureBatch1ExecutionAssets(ctx, ownerUserID)
		if err != nil {
			return nil, err
		}
		if assets == nil || assets.EvalSpec == nil || strings.TrimSpace(assets.EvalSpec.ID) == "" {
			return nil, fmt.Errorf("runtime execution follow-up eval spec is unavailable")
		}
		return &optimizationFollowupTarget{
			EvalSpecID:  strings.TrimSpace(assets.EvalSpec.ID),
			OwnerUserID: ownerUserID,
			TitleBase:   "runtime skill optimization follow-up",
		}, nil
	case "selector":
		assets, err := t.controller.EnsureSelectorCuratedAssets(ctx, ownerUserID)
		if err != nil {
			return nil, err
		}
		if assets == nil || assets.EvalSpec == nil || strings.TrimSpace(assets.EvalSpec.ID) == "" {
			return nil, fmt.Errorf("runtime selector follow-up eval spec is unavailable")
		}
		return &optimizationFollowupTarget{
			EvalSpecID:  strings.TrimSpace(assets.EvalSpec.ID),
			OwnerUserID: ownerUserID,
			TitleBase:   "runtime skill optimization follow-up",
		}, nil
	default:
		return nil, fmt.Errorf("runtime follow-up gate %q is unsupported", followupGate)
	}
}
