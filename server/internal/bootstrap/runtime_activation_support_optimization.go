package bootstrap

import (
	"context"
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
	triggerer := &harnessOptimizationTriggerer{
		manager:  manager,
		settings: settings,
	}
	if triggerer == nil {
		return
	}
	bundle.Controller.SetOptimizationTriggerer(triggerer)
}

type harnessOptimizationTriggerer struct {
	manager  *optimization.Manager
	settings *serverpkg.SettingsHandler
}

func (t *harnessOptimizationTriggerer) TriggerOptimization(ctx context.Context, event harness.OptimizationTrigger) error {
	if t == nil || t.manager == nil || t.settings == nil {
		return nil
	}
	if !t.settings.GetExperimentalAgentcoreRunnerEnabled() {
		return nil
	}
	status := t.manager.GetStatus(ctx)
	if !status.BinaryReady {
		return nil
	}
	runID := uuid.NewString()
	record := map[string]interface{}{
		"id":                     runID,
		"reason":                 event.Reason,
		"candidate_id":           strings.TrimSpace(event.CandidateID),
		"eval_run_id":            strings.TrimSpace(event.EvalRunID),
		"base_eval_run_id":       strings.TrimSpace(event.BaseEvalRunID),
		"optimization_surface":   defaultOptimizationSurface(event.OptimizationSurface),
		"repo_url":               t.settings.GetExperimentalAgentcoreRunnerRepoURL(),
		"ref":                    t.settings.GetExperimentalAgentcoreRunnerRef(),
		"runner_artifact_path":   status.BinaryPath,
		"runner_artifact_sha256": status.BinarySHA256,
		"created_at":             time.Now().UTC(),
		"metadata":               cloneOptimizationMetadata(event.Metadata),
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
	if err := t.manager.RecordOptimizationEvent(runID, record); err != nil {
		return err
	}
	return execErr
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
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
