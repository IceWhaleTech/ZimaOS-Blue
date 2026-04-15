package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

type runtimeTerminalHarnessDriver struct {
	kind           harness.RunKind
	terminalStatus harness.RunStatus
	terminalError  string
	terminalResult string
	events         []harness.RunEvent
}

func (d *runtimeTerminalHarnessDriver) Kind() harness.RunKind { return d.kind }

func (d *runtimeTerminalHarnessDriver) Validate(spec harness.RunSpec) error {
	if spec.Goal == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d *runtimeTerminalHarnessDriver) Start(ctx context.Context, run *harness.Run, env harness.RunEnv) error {
	for _, event := range d.events {
		event.RunID = run.ID
		event.RootRunID = run.RootRunID
		event.ParentRunID = run.ParentRunID
		if event.CreatedAt.IsZero() {
			event.CreatedAt = time.Now().UTC()
		}
		if err := env.Manager.AppendEvent(ctx, event); err != nil {
			return err
		}
	}
	snapshot := *run
	snapshot.Metadata = cloneRuntimeSkillEvolutionMap(run.Metadata)
	snapshot.Status = d.terminalStatus
	snapshot.Error = d.terminalError
	snapshot.Result = d.terminalResult
	snapshot.RuntimeState = agentpkg.RuntimeStateExecute
	return env.Manager.SyncSnapshot(ctx, &snapshot)
}

func (d *runtimeTerminalHarnessDriver) Cancel(context.Context, *harness.Run) error { return nil }

type runtimeRecordingOptimizationTriggerer struct {
	events []harness.OptimizationTrigger
}

func (r *runtimeRecordingOptimizationTriggerer) TriggerOptimization(_ context.Context, event harness.OptimizationTrigger) error {
	r.events = append(r.events, event)
	return nil
}

func TestHarnessControllerEmitsRuntimeSkillFailureOptimizationTrigger(t *testing.T) {
	repoRoot, canonicalContent := createRuntimeSkillEvolutionRepoForTest(t, "browser")
	restoreWD := chdirRuntimeSkillEvolutionTest(t, repoRoot)
	defer restoreWD()

	controller := newOptimizationHarnessControllerForFollowupTest(t)
	driver := &runtimeTerminalHarnessDriver{
		kind:           harness.RunKindAgentTask,
		terminalStatus: harness.RunStatusFailed,
		terminalError:  "Page title extraction failed after browser navigation.",
		events: []harness.RunEvent{
			{
				Type:        "tool_result",
				ToolName:    "browser.navigate",
				Message:     "browser navigation returned partial output",
				PayloadJSON: `{"usage":{"input_tokens":120,"output_tokens":45,"total_tokens":165},"verification":{"verification_passed":false,"failure_label":"missing_title"},"outcome_score":0.42,"evidence_score":0.31,"execution_score":0.66}`,
			},
			{
				Type:        "task_step_completed",
				Message:     "navigation verification step finished",
				PayloadJSON: `{"duration_ms":780}`,
			},
		},
	}
	controller.RegisterDriver(driver)
	triggerer := &runtimeRecordingOptimizationTriggerer{}
	controller.SetOptimizationTriggerer(triggerer)

	run, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind:   harness.RunKindAgentTask,
		Goal:   "Investigate browser regression",
		UserID: "user-1",
		Metadata: map[string]interface{}{
			"selected_canonical_skill": "browser",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if len(triggerer.events) != 1 {
		t.Fatalf("trigger count = %d, want 1", len(triggerer.events))
	}
	event := triggerer.events[0]
	if got, want := event.Reason, harness.OptimizationReasonRuntimeSkillFailure; got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
	if got, want := event.OptimizationSurface, harness.OptimizationSurfaceSkillDefinition; got != want {
		t.Fatalf("optimization_surface = %q, want %q", got, want)
	}
	if got := stringsTrim(event.Metadata, "runtime_run_id"); got != run.ID {
		t.Fatalf("runtime_run_id = %q, want %q", got, run.ID)
	}
	if got := stringsTrim(event.Metadata, "followup_gate"); got != "execution" {
		t.Fatalf("followup_gate = %q, want execution", got)
	}
	if got := stringsTrim(event.Metadata, "failure_signature"); got == "" {
		t.Fatal("expected failure_signature to be populated")
	}
	runtimeUsage, ok := event.Metadata["runtime_usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("runtime_usage = %#v, want map", event.Metadata["runtime_usage"])
	}
	if got, want := intValueForRuntimeEvolutionTest(runtimeUsage["total_tokens"]), 165; got != want {
		t.Fatalf("runtime_usage.total_tokens = %d, want %d", got, want)
	}
	runtimeQuality, ok := event.Metadata["runtime_quality"].(map[string]interface{})
	if !ok {
		t.Fatalf("runtime_quality = %#v, want map", event.Metadata["runtime_quality"])
	}
	if got, ok := runtimeQuality["verification_passed"].(bool); !ok || got {
		t.Fatalf("runtime_quality.verification_passed = %#v, want false", runtimeQuality["verification_passed"])
	}
	if got, want := runtimeQuality["failure_label"], "missing_title"; got != want {
		t.Fatalf("runtime_quality.failure_label = %#v, want %q", got, want)
	}
	runtimeMetrics, ok := event.Metadata["runtime_metrics"].(map[string]interface{})
	if !ok {
		t.Fatalf("runtime_metrics = %#v, want map", event.Metadata["runtime_metrics"])
	}
	if got := intValueForRuntimeEvolutionTest(runtimeMetrics["event_count"]); got < 2 {
		t.Fatalf("runtime_metrics.event_count = %d, want >= 2", got)
	}
	if got := intValueForRuntimeEvolutionTest(runtimeMetrics["duration_ms"]); got < 0 {
		t.Fatalf("runtime_metrics.duration_ms = %#v, want >= 0", runtimeMetrics["duration_ms"])
	}
	skillCandidate, ok := event.Metadata["skill_candidate"].(map[string]interface{})
	if !ok {
		t.Fatalf("skill_candidate = %#v, want map", event.Metadata["skill_candidate"])
	}
	if got := stringsTrim(skillCandidate, "skill_id"); got != "browser" {
		t.Fatalf("skill_candidate.skill_id = %q, want browser", got)
	}
	if got := stringsTrim(skillCandidate, "source_path"); got != "assets/skills/browser/SKILL.md" {
		t.Fatalf("skill_candidate.source_path = %q, want assets/skills/browser/SKILL.md", got)
	}
	reflectivePacket, ok := event.Metadata["reflective_evidence_packet"].(map[string]interface{})
	if !ok {
		t.Fatalf("reflective_evidence_packet = %#v, want map", event.Metadata["reflective_evidence_packet"])
	}
	if got := stringsTrim(reflectivePacket, "kind"); got != "runtime_failure" {
		t.Fatalf("reflective_evidence_packet.kind = %q, want runtime_failure", got)
	}
	if got := stringsTrim(reflectivePacket, "failure_signature"); got == "" {
		t.Fatal("expected reflective_evidence_packet.failure_signature to be populated")
	}
	if diagnostics, ok := reflectivePacket["diagnostic_signals"].(map[string]interface{}); !ok {
		t.Fatalf("reflective_evidence_packet.diagnostic_signals = %#v, want map", reflectivePacket["diagnostic_signals"])
	} else if _, ok := diagnostics["runtime_quality"].(map[string]interface{}); !ok {
		t.Fatalf("reflective_evidence_packet.diagnostic_signals.runtime_quality = %#v, want map", diagnostics["runtime_quality"])
	}
	gotContent, ok := skillCandidate["content"].(string)
	if !ok {
		t.Fatalf("skill_candidate.content = %#v, want string", skillCandidate["content"])
	}
	if gotContent != canonicalContent {
		t.Fatalf("skill_candidate.content = %q, want %q", gotContent, canonicalContent)
	}
}

func TestHarnessControllerEmitsRuntimeSkillCaptureOnlyAfterRepeatedLessons(t *testing.T) {
	repoRoot, _ := createRuntimeSkillEvolutionRepoForTest(t, "browser")
	restoreWD := chdirRuntimeSkillEvolutionTest(t, repoRoot)
	defer restoreWD()

	reflectionPayload, err := json.Marshal(agentpkg.TaskEvent{
		TaskID:    "task-placeholder",
		EventType: "task_reflection_completed",
		Message:   "Reflection extracted repeated browser lessons.",
		Output:    "Reflection extracted repeated browser lessons.\n\nLearned:\n- [heuristic] Validate page title extraction before summarizing\n",
	})
	if err != nil {
		t.Fatalf("Marshal reflection payload: %v", err)
	}

	controller := newOptimizationHarnessControllerForFollowupTest(t)
	driver := &runtimeTerminalHarnessDriver{
		kind:           harness.RunKindAgentTask,
		terminalStatus: harness.RunStatusCompleted,
		terminalResult: "Browser task completed successfully.",
		events: []harness.RunEvent{
			{
				Type:        "task_reflection_completed",
				Message:     "Reflection extracted repeated browser lessons.",
				PayloadJSON: string(reflectionPayload),
			},
		},
	}
	controller.RegisterDriver(driver)
	triggerer := &runtimeRecordingOptimizationTriggerer{}
	controller.SetOptimizationTriggerer(triggerer)

	for i := 0; i < 2; i++ {
		_, submitErr := controller.Submit(context.Background(), harness.RunSpec{
			Kind:   harness.RunKindAgentTask,
			Goal:   "Capture browser lesson",
			UserID: "user-1",
			Metadata: map[string]interface{}{
				"selected_canonical_skill": "browser",
			},
		})
		if submitErr != nil {
			t.Fatalf("Submit(%d) failed: %v", i, submitErr)
		}
	}
	if len(triggerer.events) != 1 {
		t.Fatalf("trigger count = %d, want 1", len(triggerer.events))
	}
	event := triggerer.events[0]
	if got, want := event.Reason, harness.OptimizationReasonRuntimeSkillCapture; got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
	if got := stringsTrim(event.Metadata, "capture_signature"); got == "" {
		t.Fatal("expected capture_signature to be populated")
	}
	if got, ok := event.Metadata["runtime_capture_occurrences"].(int); !ok || got != 2 {
		t.Fatalf("runtime_capture_occurrences = %#v, want 2", event.Metadata["runtime_capture_occurrences"])
	}
	reflectivePacket, ok := event.Metadata["reflective_evidence_packet"].(map[string]interface{})
	if !ok {
		t.Fatalf("reflective_evidence_packet = %#v, want map", event.Metadata["reflective_evidence_packet"])
	}
	if got := stringsTrim(reflectivePacket, "kind"); got != "runtime_capture" {
		t.Fatalf("reflective_evidence_packet.kind = %q, want runtime_capture", got)
	}
	lessons, ok := reflectivePacket["grounded_lessons"].([]interface{})
	if !ok || len(lessons) == 0 {
		t.Fatalf("reflective_evidence_packet.grounded_lessons = %#v, want non-empty list", reflectivePacket["grounded_lessons"])
	}
}

func TestHarnessControllerEmitsRuntimeSkillCaptureFromRepeatedRuntimeReflectionSignature(t *testing.T) {
	repoRoot, _ := createRuntimeSkillEvolutionRepoForTest(t, "browser")
	restoreWD := chdirRuntimeSkillEvolutionTest(t, repoRoot)
	defer restoreWD()

	reflectionPayload := `{
		"record_id":"rr-1",
		"run_id":"task-placeholder",
		"trigger_kind":"interval_tool_finishes",
		"review_seq":1,
		"step_index":2,
		"tool_count_window":10,
		"evidence_event_ids":["ev-1","ev-2"],
		"signals":{"tool_finishes_since_review":10},
		"summary":"Runtime review captured a reusable browser validation heuristic.",
		"lessons":[{"kind":"heuristic","lesson":"Validate page title extraction before summarizing","when_to_apply":"After browser navigation returns partial content","evidence":"Two runtime evidence events showed missing title extraction before summarization."}],
		"mutation_suggestions":[{"kind":"guardrail_add","target_skill_id":"browser","rationale":"Add a title-validation guardrail before summarization.","suggested_text":"Validate page title extraction before summarizing browser results.","evidence_ids":["ev-1","ev-2"],"signature":"mut-title-guardrail"}],
		"reflection_signature":"runtime-browser-title-validation",
		"status":"recorded"
	}`

	controller := newOptimizationHarnessControllerForFollowupTest(t)
	driver := &runtimeTerminalHarnessDriver{
		kind:           harness.RunKindAgentTask,
		terminalStatus: harness.RunStatusCompleted,
		terminalResult: "Browser task completed successfully.",
		events: []harness.RunEvent{
			{
				Type:        "runtime_reflection_recorded",
				Message:     "Runtime review captured a reusable browser validation heuristic.",
				PayloadJSON: reflectionPayload,
			},
		},
	}
	controller.RegisterDriver(driver)
	triggerer := &runtimeRecordingOptimizationTriggerer{}
	controller.SetOptimizationTriggerer(triggerer)

	for i := 0; i < 2; i++ {
		_, submitErr := controller.Submit(context.Background(), harness.RunSpec{
			Kind:   harness.RunKindAgentTask,
			Goal:   "Capture browser lesson from runtime reflection",
			UserID: "user-1",
			Metadata: map[string]interface{}{
				"selected_canonical_skill": "browser",
			},
		})
		if submitErr != nil {
			t.Fatalf("Submit(%d) failed: %v", i, submitErr)
		}
	}
	if len(triggerer.events) != 1 {
		t.Fatalf("trigger count = %d, want 1", len(triggerer.events))
	}
	event := triggerer.events[0]
	if got, want := event.Reason, harness.OptimizationReasonRuntimeSkillCapture; got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
	if got := stringsTrim(event.Metadata, "capture_signature"); got != "runtime-browser-title-validation" {
		t.Fatalf("capture_signature = %q, want runtime-browser-title-validation", got)
	}
	if got, ok := event.Metadata["runtime_capture_occurrences"].(int); !ok || got != 2 {
		t.Fatalf("runtime_capture_occurrences = %#v, want 2", event.Metadata["runtime_capture_occurrences"])
	}
	reflectivePacket, ok := event.Metadata["reflective_evidence_packet"].(map[string]interface{})
	if !ok {
		t.Fatalf("reflective_evidence_packet = %#v, want map", event.Metadata["reflective_evidence_packet"])
	}
	if got := stringsTrim(reflectivePacket, "capture_signature"); got != "runtime-browser-title-validation" {
		t.Fatalf("reflective_evidence_packet.capture_signature = %q, want runtime-browser-title-validation", got)
	}
	lessons, ok := reflectivePacket["grounded_lessons"].([]interface{})
	if !ok || len(lessons) == 0 {
		t.Fatalf("reflective_evidence_packet.grounded_lessons = %#v, want non-empty list", reflectivePacket["grounded_lessons"])
	}
}

func TestHarnessControllerFailureReflectivePacketIncludesRuntimeReflections(t *testing.T) {
	repoRoot, _ := createRuntimeSkillEvolutionRepoForTest(t, "browser")
	restoreWD := chdirRuntimeSkillEvolutionTest(t, repoRoot)
	defer restoreWD()

	reflectionPayload := `{
		"record_id":"rr-2",
		"run_id":"task-placeholder",
		"trigger_kind":"repeated_fingerprint",
		"review_seq":2,
		"step_index":3,
		"tool_count_window":7,
		"evidence_event_ids":["ev-3","ev-4"],
		"signals":{"repeated_fingerprint":"browser.navigate|missing_title"},
		"summary":"Runtime review captured a browser anti-pattern.",
		"lessons":[{"kind":"anti_pattern","lesson":"Do not summarize browser results before title extraction succeeds","when_to_apply":"When browser navigation returns partial content","evidence":"Repeated runtime events showed missing_title failures before summarization."}],
		"mutation_suggestions":[{"kind":"guardrail_add","target_skill_id":"browser","rationale":"Prevent early summarization on missing title.","suggested_text":"Abort summary generation when browser title extraction is missing.","evidence_ids":["ev-3","ev-4"],"signature":"guardrail-missing-title"}],
		"reflection_signature":"runtime-browser-missing-title",
		"status":"recorded"
	}`

	controller := newOptimizationHarnessControllerForFollowupTest(t)
	driver := &runtimeTerminalHarnessDriver{
		kind:           harness.RunKindAgentTask,
		terminalStatus: harness.RunStatusFailed,
		terminalError:  "Page title extraction failed after browser navigation.",
		events: []harness.RunEvent{
			{
				Type:        "runtime_reflection_recorded",
				Message:     "Runtime review captured a browser anti-pattern.",
				PayloadJSON: reflectionPayload,
			},
			{
				Type:        "tool_result",
				ToolName:    "browser.navigate",
				Message:     "browser navigation returned partial output",
				PayloadJSON: `{"verification":{"verification_passed":false,"failure_label":"missing_title"}}`,
			},
		},
	}
	controller.RegisterDriver(driver)
	triggerer := &runtimeRecordingOptimizationTriggerer{}
	controller.SetOptimizationTriggerer(triggerer)

	_, err := controller.Submit(context.Background(), harness.RunSpec{
		Kind:   harness.RunKindAgentTask,
		Goal:   "Investigate browser regression with runtime reflection",
		UserID: "user-1",
		Metadata: map[string]interface{}{
			"selected_canonical_skill": "browser",
		},
	})
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if len(triggerer.events) != 1 {
		t.Fatalf("trigger count = %d, want 1", len(triggerer.events))
	}
	reflectivePacket, ok := triggerer.events[0].Metadata["reflective_evidence_packet"].(map[string]interface{})
	if !ok {
		t.Fatalf("reflective_evidence_packet = %#v, want map", triggerer.events[0].Metadata["reflective_evidence_packet"])
	}
	reflections, ok := reflectivePacket["runtime_reflections"].([]interface{})
	if !ok || len(reflections) == 0 {
		t.Fatalf("runtime_reflections = %#v, want non-empty list", reflectivePacket["runtime_reflections"])
	}
	suggestions, ok := reflectivePacket["mutation_suggestions"].([]interface{})
	if !ok || len(suggestions) == 0 {
		t.Fatalf("mutation_suggestions = %#v, want non-empty list", reflectivePacket["mutation_suggestions"])
	}
}

func TestHarnessOptimizationTriggererSubmitsRuntimeSkillFollowupEval(t *testing.T) {
	repoRoot, canonicalContent := createRuntimeSkillEvolutionRepoForTest(t, "browser")
	restoreWD := chdirRuntimeSkillEvolutionTest(t, repoRoot)
	defer restoreWD()

	controller := newOptimizationHarnessControllerForFollowupTest(t)
	response := "Candidate ready.\n```json\n" + strings.TrimSpace(`{
  "status": "candidate_ready",
  "message": "Runtime browser fix candidate",
  "skill_candidate": {
    "skill_id": "browser",
    "candidate_id": "candidate-runtime-browser",
    "source_path": "assets/skills/browser/SKILL.md",
    "content": "# Browser\nPrefer validating titles before extracting page summaries.\n"
  }
}`) + "\n```"
	bin := buildScriptedOptimizationRunnerBinary(t, response)
	root := filepath.Join(t.TempDir(), "agentcore-runner")
	manager := newOptimizationManagerWithRunnerBinaryForTest(t, root, bin)

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	patchAgentcoreRunnerSettingsForOptimizationTest(t, settings, `{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`)

	triggerer := &harnessOptimizationTriggerer{
		manager:    manager,
		settings:   settings,
		controller: controller,
	}
	event := harness.OptimizationTrigger{
		Reason:              harness.OptimizationReasonRuntimeSkillFailure,
		CandidateID:         "runtime-browser-source",
		OptimizationSurface: harness.OptimizationSurfaceSkillDefinition,
		Metadata: map[string]interface{}{
			"runtime_run_id":    "runtime-run-1",
			"owner_user_id":     "user-1",
			"followup_gate":     "execution",
			"failure_signature": "failed-page-title-extraction",
			"runtime_status":    "failed",
			"runtime_state":     "execute",
			"goal":              "Investigate browser regression",
			"result_summary":    "Browser task failed after partial navigation recovery.",
			"runtime_metrics": map[string]interface{}{
				"duration_ms": 3200,
				"event_count": 6,
			},
			"runtime_usage": map[string]interface{}{
				"input_tokens":  320,
				"output_tokens": 220,
				"total_tokens":  540,
			},
			"runtime_quality": map[string]interface{}{
				"verification_passed": false,
				"failure_label":       "missing_title",
				"outcome_score":       0.42,
				"evidence_score":      0.31,
				"execution_score":     0.66,
			},
			"runtime_event_summaries": []interface{}{
				map[string]interface{}{
					"type":      "tool_result",
					"tool_name": "browser.navigate",
					"message":   "browser navigation returned partial output",
				},
			},
			"skill_candidate": map[string]interface{}{
				"skill_id":     "browser",
				"candidate_id": "runtime-browser-source",
				"source_path":  "assets/skills/browser/SKILL.md",
				"content":      canonicalContent,
				"sha256":       sha256HexForOptimizationRuntimeTest(canonicalContent),
			},
		},
	}

	if err := triggerer.TriggerOptimization(context.Background(), event); err != nil {
		t.Fatalf("TriggerOptimization failed: %v", err)
	}

	current := manager.GetStatus(context.Background())
	recordPath := filepath.Join(root, "optimization-runs", current.LastOptimizationRunID+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read optimization record: %v", err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode optimization record: %v", err)
	}
	if got := stringsTrim(record, "followup_state"); got != "submitted" {
		t.Fatalf("followup_state = %q, want submitted", got)
	}
	followupEvalRunID := stringsTrim(record, "followup_eval_run_id")
	if followupEvalRunID == "" {
		t.Fatalf("expected followup_eval_run_id in record: %s", string(data))
	}
	followupEvalRun, err := controller.GetEvalRun(context.Background(), followupEvalRunID)
	if err != nil {
		t.Fatalf("GetEvalRun(followup) failed: %v", err)
	}
	if got := followupEvalRun.TriggerKind; got != "optimization_followup" {
		t.Fatalf("followup trigger_kind = %q, want optimization_followup", got)
	}
	if got := stringsTrim(followupEvalRun.Metadata, "optimization_parent_run_id"); got != "runtime-run-1" {
		t.Fatalf("optimization_parent_run_id = %q, want runtime-run-1", got)
	}
	evolutionCases, err := controller.ListSkillEvolutionCases(context.Background(), harness.SkillEvolutionCaseFilter{
		SkillID:     "browser",
		OwnerUserID: "user-1",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListSkillEvolutionCases failed: %v", err)
	}
	if len(evolutionCases) != 1 {
		t.Fatalf("evolution cases len = %d, want 1", len(evolutionCases))
	}
	if got, want := evolutionCases[0].Reason, harness.SkillEvolutionReasonRuntimeFailure; got != want {
		t.Fatalf("evolution case reason = %q, want %q", got, want)
	}
	if got, want := evolutionCases[0].SourceKind, "runtime_run"; got != want {
		t.Fatalf("evolution case source_kind = %q, want %q", got, want)
	}
	if got, want := evolutionCases[0].SourceID, "runtime-run-1"; got != want {
		t.Fatalf("evolution case source_id = %q, want %q", got, want)
	}
	var evidence map[string]interface{}
	if err := json.Unmarshal([]byte(evolutionCases[0].EvidenceJSON), &evidence); err != nil {
		t.Fatalf("decode evolution evidence: %v", err)
	}
	if got := stringsTrim(evidence, "runtime_run_id"); got != "runtime-run-1" {
		t.Fatalf("evidence runtime_run_id = %q, want runtime-run-1", got)
	}
	runtimeMetrics, ok := evidence["runtime_metrics"].(map[string]interface{})
	if !ok {
		t.Fatalf("evidence runtime_metrics = %#v, want map", evidence["runtime_metrics"])
	}
	if got, want := int(runtimeMetrics["duration_ms"].(float64)), 3200; got != want {
		t.Fatalf("evidence runtime_metrics.duration_ms = %d, want %d", got, want)
	}
	runtimeUsage, ok := evidence["runtime_usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("evidence runtime_usage = %#v, want map", evidence["runtime_usage"])
	}
	if got, want := int(runtimeUsage["total_tokens"].(float64)), 540; got != want {
		t.Fatalf("evidence runtime_usage.total_tokens = %d, want %d", got, want)
	}
	runtimeQuality, ok := evidence["runtime_quality"].(map[string]interface{})
	if !ok {
		t.Fatalf("evidence runtime_quality = %#v, want map", evidence["runtime_quality"])
	}
	if got, ok := runtimeQuality["verification_passed"].(bool); !ok || got {
		t.Fatalf("evidence runtime_quality.verification_passed = %#v, want false", runtimeQuality["verification_passed"])
	}
}

func createRuntimeSkillEvolutionRepoForTest(t *testing.T, skillID string) (string, string) {
	t.Helper()
	repoRoot := t.TempDir()
	skillDir := filepath.Join(repoRoot, "assets", "skills", skillID)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skillDir: %v", err)
	}
	content := "# " + skillID + "\nCanonical runtime skill fixture.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile SKILL.md: %v", err)
	}
	return repoRoot, content
}

func chdirRuntimeSkillEvolutionTest(t *testing.T, dir string) func() {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) failed: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd failed: %v", err)
		}
	}
}

func cloneRuntimeSkillEvolutionMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func stringsTrim(meta map[string]interface{}, key string) string {
	if len(meta) == 0 {
		return ""
	}
	return strings.TrimSpace(asStringForOptimizationTest(meta[key]))
}

func sha256HexForOptimizationRuntimeTest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func intValueForRuntimeEvolutionTest(raw interface{}) int {
	switch typed := raw.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return -1
	}
}
