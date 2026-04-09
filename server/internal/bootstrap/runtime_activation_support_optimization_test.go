package bootstrap

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/optimization"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func TestBindHarnessRuntimeOptimizationWiresControllerTriggerer(t *testing.T) {
	manager, err := optimization.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}
	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	bundle := &HarnessRuntimeBundle{Controller: &harness.Controller{}}

	bindHarnessRuntimeOptimization(settings, bundle)

	optimizationField := reflect.ValueOf(bundle.Controller).Elem().FieldByName("optimization")
	if !optimizationField.IsValid() || optimizationField.IsNil() {
		t.Fatal("expected controller optimization triggerer to be wired")
	}
}

func TestBuildOptimizationFollowupMetadata_PreservesProviderID(t *testing.T) {
	metadata := buildOptimizationFollowupMetadata(
		harness.OptimizationTrigger{
			EvalRunID:           "parent-eval-run",
			OptimizationSurface: harness.OptimizationSurfaceSkillDefinition,
			Metadata: map[string]interface{}{
				"provider_id": "openai-prod",
			},
		},
		"optimization-run-1",
		map[string]interface{}{"candidate_id": "candidate-browser-optimized"},
		&harness.SkillRevision{ID: "revision-1"},
	)

	if got := strings.TrimSpace(asStringForOptimizationTest(metadata["provider_id"])); got != "openai-prod" {
		t.Fatalf("provider_id = %q, want openai-prod", got)
	}
}

func TestHarnessOptimizationTriggererExecutesPreparedRunnerAndPersistsTranscript(t *testing.T) {
	bin := buildOptimizationTestRunnerBinary(t)
	root := filepath.Join(t.TempDir(), "agentcore-runner")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	sum := sha256FileForOptimizationTest(t, bin)
	status := optimization.Status{
		BinaryReady:    true,
		BinaryPath:     bin,
		BinarySHA256:   sum,
		ResolvedRef:    "main",
		ResolvedCommit: strings.Repeat("a", 40),
	}
	statusData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "status.json"), statusData, 0o644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	patchAgentcoreRunnerSettingsForOptimizationTest(t, settings, `{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`)

	triggerer := &harnessOptimizationTriggerer{
		manager:  manager,
		settings: settings,
	}
	event := harness.OptimizationTrigger{
		Reason:              harness.OptimizationReasonExecutionGateFailed,
		CandidateID:         "candidate-123",
		EvalRunID:           "eval-456",
		BaseEvalRunID:       "base-789",
		OptimizationSurface: harness.OptimizationSurfaceRunnerCode,
		Metadata: map[string]interface{}{
			"gate": "execution",
		},
	}

	if err := triggerer.TriggerOptimization(context.Background(), event); err != nil {
		t.Fatalf("TriggerOptimization: %v", err)
	}

	current := manager.GetStatus(context.Background())
	if strings.TrimSpace(current.LastOptimizationRunID) == "" {
		t.Fatal("expected last optimization run id to be recorded")
	}
	recordPath := filepath.Join(root, "optimization-runs", current.LastOptimizationRunID+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read optimization record: %v", err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode optimization record: %v", err)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["runner_artifact_path"])); got != bin {
		t.Fatalf("runner_artifact_path = %q, want %q", got, bin)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["runner_artifact_sha256"])); got != sum {
		t.Fatalf("runner_artifact_sha256 = %q, want %q", got, sum)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["runner_stop_reason"])); got != "completed" {
		t.Fatalf("runner_stop_reason = %q, want completed; record=%s", got, string(data))
	}
	if _, ok := record["runner_started_at"]; !ok {
		t.Fatalf("runner_started_at missing: record=%s", string(data))
	}
	if _, ok := record["runner_finished_at"]; !ok {
		t.Fatalf("runner_finished_at missing: record=%s", string(data))
	}
	duration := int64(record["runner_duration_ms"].(float64))
	if duration < 0 {
		t.Fatalf("runner_duration_ms = %d, want >= 0; record=%s", duration, string(data))
	}
	responseText := strings.TrimSpace(asStringForOptimizationTest(record["runner_response_text"]))
	if !strings.Contains(responseText, "candidate-123") {
		t.Fatalf("runner_response_text = %q, want candidate id evidence; record=%s", responseText, string(data))
	}
	transcript, ok := record["runner_transcript"].([]interface{})
	if !ok || len(transcript) == 0 {
		t.Fatalf("runner_transcript missing or empty: record=%s", string(data))
	}
}

func TestHarnessOptimizationTriggererPersistsRunnerErrorWhenPreparedBinaryChecksumMismatches(t *testing.T) {
	bin := buildOptimizationTestRunnerBinary(t)
	root := filepath.Join(t.TempDir(), "agentcore-runner")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	status := optimization.Status{
		BinaryReady:    true,
		BinaryPath:     bin,
		BinarySHA256:   strings.Repeat("f", 64),
		ResolvedRef:    "main",
		ResolvedCommit: strings.Repeat("b", 40),
	}
	statusData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "status.json"), statusData, 0o644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	patchAgentcoreRunnerSettingsForOptimizationTest(t, settings, `{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`)

	triggerer := &harnessOptimizationTriggerer{
		manager:  manager,
		settings: settings,
	}
	err = triggerer.TriggerOptimization(context.Background(), harness.OptimizationTrigger{
		Reason:      harness.OptimizationReasonSelectorGateFailed,
		CandidateID: "candidate-bad-sha",
	})
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("TriggerOptimization error = %v, want checksum mismatch", err)
	}

	current := manager.GetStatus(context.Background())
	if strings.TrimSpace(current.LastOptimizationRunID) == "" {
		t.Fatal("expected last optimization run id to be recorded")
	}
	recordPath := filepath.Join(root, "optimization-runs", current.LastOptimizationRunID+".json")
	data, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read optimization record: %v", err)
	}
	var record map[string]interface{}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("decode optimization record: %v", err)
	}
	runnerError := strings.TrimSpace(asStringForOptimizationTest(record["runner_error"]))
	if !strings.Contains(runnerError, "checksum mismatch") {
		t.Fatalf("runner_error = %q, want checksum mismatch; record=%s", runnerError, string(data))
	}
	if _, ok := record["runner_started_at"]; !ok {
		t.Fatalf("runner_started_at missing: record=%s", string(data))
	}
	if _, ok := record["runner_finished_at"]; !ok {
		t.Fatalf("runner_finished_at missing: record=%s", string(data))
	}
	duration := int64(record["runner_duration_ms"].(float64))
	if duration < 0 {
		t.Fatalf("runner_duration_ms = %d, want >= 0; record=%s", duration, string(data))
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["runner_stop_reason"])); got != "" {
		t.Fatalf("runner_stop_reason = %q, want empty on checksum failure; record=%s", got, string(data))
	}
}

func TestBuildOptimizationRunnerPromptIncludesSkillCandidateDetails(t *testing.T) {
	content := strings.TrimSpace(`
---
name: browser
---

# Browser
Use browser skill candidate.
`) + "\n"
	sum := sha256.Sum256([]byte(content))
	sha := hex.EncodeToString(sum[:])

	prompt := buildOptimizationRunnerPrompt(harness.OptimizationTrigger{
		Reason:              harness.OptimizationReasonSelectorGateFailed,
		CandidateID:         "candidate-browser-1",
		EvalRunID:           "eval-browser-1",
		BaseEvalRunID:       "eval-browser-base",
		OptimizationSurface: harness.OptimizationSurfaceSkillDefinition,
		Metadata: map[string]interface{}{
			"skill_candidate": map[string]interface{}{
				"skill_id":     "browser",
				"candidate_id": "candidate-browser-1",
				"source_path":  "assets/skills/browser/SKILL.md",
				"applied_path": "/tmp/harness/workspace/.agents/skills/browser/SKILL.md",
				"sha256":       sha,
				"content":      content,
			},
		},
	})

	for _, want := range []string{
		"Optimization Surface: skill_definition",
		"Skill ID: browser",
		"Skill Candidate ID: candidate-browser-1",
		"Skill Source Path: assets/skills/browser/SKILL.md",
		"Applied Skill Path: /tmp/harness/workspace/.agents/skills/browser/SKILL.md",
		"Skill SHA256: " + sha,
		"SKILL.md Content:",
		"Use browser skill candidate.",
		"Return JSON only with this schema:",
		`"status":"candidate_ready|no_change"`,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestHarnessOptimizationTriggererSubmitsFollowupEvalRunForSkillCandidateResponse(t *testing.T) {
	controller := newOptimizationHarnessControllerForFollowupTest(t)
	parentEvalRun := createOptimizationEvalRunForFollowupTest(t, controller, map[string]interface{}{
		"candidate_id": "candidate-parent",
		"skill_candidate": map[string]interface{}{
			"skill_id":    "browser",
			"source_path": "assets/skills/browser/SKILL.md",
		},
		"optimization_surface": string(harness.OptimizationSurfaceSkillDefinition),
	})

	content := strings.TrimSpace(`
---
name: browser
description: Improved browser skill
---

# Browser

Prefer high-signal browsing steps.
`) + "\n"

	response := "Candidate ready.\n```json\n" + strings.TrimSpace(fmt.Sprintf(`{
  "status": "candidate_ready",
  "message": "Improved browser skill candidate",
  "skill_candidate": {
    "skill_id": "browser",
    "candidate_id": "candidate-browser-optimized",
    "content": %q
  }
}`, content)) + "\n```"
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
		Reason:              harness.OptimizationReasonSelectorGateFailed,
		CandidateID:         "candidate-parent",
		EvalRunID:           parentEvalRun.ID,
		BaseEvalRunID:       "baseline-eval-run",
		OptimizationSurface: harness.OptimizationSurfaceSkillDefinition,
		Metadata: map[string]interface{}{
			"candidate_id": "candidate-parent",
			"skill_candidate": map[string]interface{}{
				"skill_id":    "browser",
				"source_path": "assets/skills/browser/SKILL.md",
			},
		},
	}

	if err := triggerer.TriggerOptimization(context.Background(), event); err != nil {
		t.Fatalf("TriggerOptimization: %v", err)
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

	if got := strings.TrimSpace(asStringForOptimizationTest(record["followup_state"])); got != "submitted" {
		t.Fatalf("followup_state = %q, want submitted; record=%s", got, string(data))
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["promotion_state"])); got != "candidate" {
		t.Fatalf("promotion_state = %q, want candidate; record=%s", got, string(data))
	}
	skillRevisionID := strings.TrimSpace(asStringForOptimizationTest(record["skill_revision_id"]))
	if skillRevisionID == "" {
		t.Fatalf("expected skill_revision_id in record: %s", string(data))
	}
	followupEvalRunID := strings.TrimSpace(asStringForOptimizationTest(record["followup_eval_run_id"]))
	if followupEvalRunID == "" {
		t.Fatalf("expected followup_eval_run_id in record: %s", string(data))
	}
	followupEvalRun, err := controller.GetEvalRun(context.Background(), followupEvalRunID)
	if err != nil {
		t.Fatalf("GetEvalRun(followup): %v", err)
	}
	if got, want := followupEvalRun.EvalSpecID, parentEvalRun.EvalSpecID; got != want {
		t.Fatalf("followup eval_spec_id = %q, want %q", got, want)
	}
	if got := strings.TrimSpace(followupEvalRun.BaselineEvalRunID); got != "baseline-eval-run" {
		t.Fatalf("followup baseline_eval_run_id = %q, want baseline-eval-run", got)
	}
	if got := strings.TrimSpace(followupEvalRun.TriggerKind); got != "optimization_followup" {
		t.Fatalf("followup trigger_kind = %q, want optimization_followup", got)
	}
	if got := strings.TrimSpace(followupEvalRun.TriggerRef); got != current.LastOptimizationRunID {
		t.Fatalf("followup trigger_ref = %q, want %q", got, current.LastOptimizationRunID)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(followupEvalRun.Metadata["optimization_parent_run_id"])); got != parentEvalRun.ID {
		t.Fatalf("optimization_parent_run_id = %q, want %q", got, parentEvalRun.ID)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(followupEvalRun.Metadata["optimization_run_id"])); got != current.LastOptimizationRunID {
		t.Fatalf("optimization_run_id = %q, want %q", got, current.LastOptimizationRunID)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(followupEvalRun.Metadata["skill_revision_id"])); got != skillRevisionID {
		t.Fatalf("skill_revision_id = %q, want %q", got, skillRevisionID)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(followupEvalRun.Metadata["candidate_id"])); got != "candidate-browser-optimized" {
		t.Fatalf("candidate_id = %q, want candidate-browser-optimized", got)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(followupEvalRun.Metadata["optimization_surface"])); got != string(harness.OptimizationSurfaceSkillDefinition) {
		t.Fatalf("optimization_surface = %q, want %q", got, harness.OptimizationSurfaceSkillDefinition)
	}
	if got, ok := followupEvalRun.Metadata["optimization_run"].(bool); !ok || !got {
		t.Fatalf("optimization_run = %#v, want true", followupEvalRun.Metadata["optimization_run"])
	}

	candidateMeta, ok := followupEvalRun.Metadata["skill_candidate"].(map[string]interface{})
	if !ok {
		t.Fatalf("skill_candidate metadata = %#v, want map", followupEvalRun.Metadata["skill_candidate"])
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(candidateMeta["skill_id"])); got != "browser" {
		t.Fatalf("skill_candidate.skill_id = %q, want browser", got)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(candidateMeta["source_path"])); got != "assets/skills/browser/SKILL.md" {
		t.Fatalf("skill_candidate.source_path = %q, want assets/skills/browser/SKILL.md", got)
	}
	gotContent, ok := candidateMeta["content"].(string)
	if !ok {
		t.Fatalf("skill_candidate.content = %#v, want string", candidateMeta["content"])
	}
	if got := gotContent; got != content {
		t.Fatalf("skill_candidate.content = %q, want %q", got, content)
	}
	revision, err := controller.GetSkillRevision(context.Background(), skillRevisionID)
	if err != nil {
		t.Fatalf("GetSkillRevision(skillRevisionID): %v", err)
	}
	if got, want := revision.Status, harness.SkillRevisionStatusCandidate; got != want {
		t.Fatalf("revision status = %q, want %q", got, want)
	}
	if got, want := revision.SkillID, "browser"; got != want {
		t.Fatalf("revision skill_id = %q, want %q", got, want)
	}
	if got, want := revision.SourcePath, "assets/skills/browser/SKILL.md"; got != want {
		t.Fatalf("revision source_path = %q, want %q", got, want)
	}
	if got, want := revision.EvalRunID, followupEvalRunID; got != want {
		t.Fatalf("revision eval_run_id = %q, want %q", got, want)
	}
	if got, want := revision.OptimizationRunID, current.LastOptimizationRunID; got != want {
		t.Fatalf("revision optimization_run_id = %q, want %q", got, want)
	}
	if got, want := revision.FollowupGate, "selector"; got != want {
		t.Fatalf("revision followup_gate = %q, want %q", got, want)
	}
	if got, want := revision.OptimizationSurface, harness.OptimizationSurfaceSkillDefinition; got != want {
		t.Fatalf("revision optimization_surface = %q, want %q", got, want)
	}
	if strings.TrimSpace(revision.OriginCaseID) == "" {
		t.Fatal("expected revision OriginCaseID to be populated")
	}
	if strings.TrimSpace(revision.BaseContentSHA256) == "" {
		t.Fatal("expected revision BaseContentSHA256 to be populated")
	}
	evolutionCases, err := controller.ListSkillEvolutionCases(context.Background(), harness.SkillEvolutionCaseFilter{
		SkillID:     "browser",
		OwnerUserID: "user-1",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListSkillEvolutionCases(browser): %v", err)
	}
	if len(evolutionCases) != 1 {
		t.Fatalf("evolution cases len = %d, want 1", len(evolutionCases))
	}
	if got, want := evolutionCases[0].RevisionID, skillRevisionID; got != want {
		t.Fatalf("evolution case revision_id = %q, want %q", got, want)
	}
	if got, want := evolutionCases[0].CandidateID, "candidate-browser-optimized"; got != want {
		t.Fatalf("evolution case candidate_id = %q, want %q", got, want)
	}
	if got, want := evolutionCases[0].Status, harness.SkillEvolutionCaseStatusCandidateCreated; got != want {
		t.Fatalf("evolution case status = %q, want %q", got, want)
	}
	if got, want := revision.OriginCaseID, evolutionCases[0].ID; got != want {
		t.Fatalf("revision OriginCaseID = %q, want %q", got, want)
	}
}

func TestHarnessOptimizationManagerAdapterReconcilesAcceptedFollowupSelectorRun(t *testing.T) {
	controller := newOptimizationHarnessControllerForFollowupTest(t)
	driver := bootstrapSelectorEvalDriver{
		responsesByCandidate: map[string]map[string]map[string]interface{}{
			"": {
				"Search the latest OpenAI Responses API documentation.":            selectorEvalResponseForBootstrapTest("web_query", false, "selected"),
				"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponseForBootstrapTest("exec", true, "clarify"),
			},
			"candidate-browser-optimized": {
				"Search the latest OpenAI Responses API documentation.":            selectorEvalResponseForBootstrapTest("web_query", false, "selected"),
				"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": selectorEvalResponseForBootstrapTest("exec", true, "clarify"),
			},
		},
	}
	controller.RegisterDriver(driver)
	evalSpec := createSelectorEvalSpecForOptimizationAssessmentTest(t, controller)
	baselineEvalRun := runSelectorEvalForOptimizationAssessmentTest(t, controller, evalSpec, "selector-baseline", map[string]interface{}{})
	followupEvalRun := runSelectorEvalForOptimizationAssessmentTest(t, controller, evalSpec, "selector-followup", map[string]interface{}{
		"candidate_id":               "candidate-browser-optimized",
		"optimization_run":           true,
		"optimization_run_id":        "opt-accepted",
		"optimization_parent_run_id": "parent-eval-run",
	})
	evolutionCase, err := controller.CreateSkillEvolutionCase(context.Background(), harness.SkillEvolutionCase{
		SkillID:           "browser",
		OwnerUserID:       "user-1",
		Mode:              harness.SkillEvolutionModeFix,
		Reason:            harness.SkillEvolutionReasonSelectorGateFailed,
		SourceKind:        "eval_run",
		SourceID:          "parent-eval-run",
		CandidateID:       "candidate-browser-optimized",
		BaseContentSHA256: "base-browser-accepted",
		FailureSignature:  "selector-gate-failed",
		Summary:           "Selector gate produced a candidate browser revision.",
		EvidenceJSON:      `{"followup":"selector"}`,
		Status:            harness.SkillEvolutionCaseStatusCandidateCreated,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateSkillEvolutionCase failed: %v", err)
	}
	revision, err := controller.CreateSkillRevision(context.Background(), harness.SkillRevision{
		SkillID:           "browser",
		Status:            harness.SkillRevisionStatusCandidate,
		SourcePath:        "assets/skills/browser/SKILL.md",
		CandidateID:       "candidate-browser-optimized",
		BaseContentSHA256: "base-browser-accepted",
		OriginCaseID:      evolutionCase.ID,
		EvalRunID:         "parent-eval-run",
		OptimizationRunID: "opt-accepted",
		Content:           "# Browser\nAccepted candidate.\n",
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}

	recordID := "opt-accepted"
	manager := newOptimizationManagerWithLastRunRecordForTest(t, filepath.Join(t.TempDir(), "agentcore-runner"), recordID, map[string]interface{}{
		"id":                   recordID,
		"reason":               string(harness.OptimizationReasonSelectorGateFailed),
		"base_eval_run_id":     baselineEvalRun.ID,
		"followup_eval_run_id": followupEvalRun.ID,
		"followup_state":       "submitted",
		"skill_revision_id":    revision.ID,
		"runner_response_text": `{"status":"candidate_ready"}`,
	})

	adapter := newHarnessOptimizationManagerAdapter(manager, controller)
	record, err := adapter.GetLastOptimizationRun(context.Background())
	if err != nil {
		t.Fatalf("GetLastOptimizationRun: %v", err)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["followup_decision"])); got != "accepted" {
		t.Fatalf("followup_decision = %q, want accepted; record=%#v", got, record)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["followup_gate"])); got != "selector" {
		t.Fatalf("followup_gate = %q, want selector", got)
	}
	if got, ok := record["followup_gate_passed"].(bool); !ok || !got {
		t.Fatalf("followup_gate_passed = %#v, want true", record["followup_gate_passed"])
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["followup_eval_status"])); got != string(harness.RunGroupStatusCompleted) {
		t.Fatalf("followup_eval_status = %q, want %q", got, harness.RunGroupStatusCompleted)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["promotion_state"])); got != "accepted" {
		t.Fatalf("promotion_state = %q, want accepted", got)
	}
	reloadedRevision, err := controller.GetSkillRevision(context.Background(), revision.ID)
	if err != nil {
		t.Fatalf("GetSkillRevision(revision.ID): %v", err)
	}
	if got, want := reloadedRevision.Status, harness.SkillRevisionStatusAccepted; got != want {
		t.Fatalf("revision status = %q, want %q", got, want)
	}
	reloadedCase, err := controller.GetSkillEvolutionCase(context.Background(), evolutionCase.ID)
	if err != nil {
		t.Fatalf("GetSkillEvolutionCase(evolutionCase.ID): %v", err)
	}
	if got, want := reloadedCase.Status, harness.SkillEvolutionCaseStatusAccepted; got != want {
		t.Fatalf("evolution case status = %q, want %q", got, want)
	}

	status := adapter.GetStatus(context.Background())
	if got := strings.TrimSpace(status.LastOptimizationState); got != "accepted" {
		t.Fatalf("LastOptimizationState = %q, want accepted", got)
	}
	if got := strings.TrimSpace(status.LastOptimizationSummary); !strings.Contains(got, "accepted") {
		t.Fatalf("LastOptimizationSummary = %q, want accepted summary", got)
	}
}

func TestHarnessOptimizationManagerAdapterMarksRunningFollowupEval(t *testing.T) {
	controller := newOptimizationHarnessControllerForFollowupTest(t)
	evalSpec := createSelectorEvalSpecForOptimizationAssessmentTest(t, controller)
	followupEvalRun, err := controller.SubmitEvalRun(context.Background(), harness.EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       "selector-followup-pending",
		Metadata: map[string]interface{}{
			"candidate_id":               "candidate-pending",
			"optimization_run":           true,
			"optimization_run_id":        "opt-running",
			"optimization_parent_run_id": "parent-eval-run",
		},
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun pending followup failed: %v", err)
	}
	evolutionCase, err := controller.CreateSkillEvolutionCase(context.Background(), harness.SkillEvolutionCase{
		SkillID:           "browser",
		OwnerUserID:       "user-1",
		Mode:              harness.SkillEvolutionModeFix,
		Reason:            harness.SkillEvolutionReasonSelectorGateFailed,
		SourceKind:        "eval_run",
		SourceID:          "parent-eval-run",
		CandidateID:       "candidate-pending",
		BaseContentSHA256: "base-browser-pending",
		FailureSignature:  "selector-gate-failed",
		Summary:           "Selector gate produced a pending candidate browser revision.",
		EvidenceJSON:      `{"followup":"selector"}`,
		Status:            harness.SkillEvolutionCaseStatusCandidateCreated,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateSkillEvolutionCase failed: %v", err)
	}
	revision, err := controller.CreateSkillRevision(context.Background(), harness.SkillRevision{
		SkillID:           "browser",
		Status:            harness.SkillRevisionStatusCandidate,
		SourcePath:        "assets/skills/browser/SKILL.md",
		CandidateID:       "candidate-pending",
		BaseContentSHA256: "base-browser-pending",
		OriginCaseID:      evolutionCase.ID,
		EvalRunID:         "parent-eval-run",
		OptimizationRunID: "opt-running",
		Content:           "# Browser\nPending candidate.\n",
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}

	recordID := "opt-running"
	manager := newOptimizationManagerWithLastRunRecordForTest(t, filepath.Join(t.TempDir(), "agentcore-runner"), recordID, map[string]interface{}{
		"id":                   recordID,
		"reason":               string(harness.OptimizationReasonSelectorGateFailed),
		"base_eval_run_id":     "baseline-eval-run",
		"followup_eval_run_id": followupEvalRun.ID,
		"followup_state":       "submitted",
		"skill_revision_id":    revision.ID,
		"runner_response_text": `{"status":"candidate_ready"}`,
	})

	adapter := newHarnessOptimizationManagerAdapter(manager, controller)
	record, err := adapter.GetLastOptimizationRun(context.Background())
	if err != nil {
		t.Fatalf("GetLastOptimizationRun: %v", err)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["followup_decision"])); got != "running" {
		t.Fatalf("followup_decision = %q, want running; record=%#v", got, record)
	}
	if got := strings.TrimSpace(asStringForOptimizationTest(record["followup_eval_status"])); got != string(harness.RunGroupStatusPending) {
		t.Fatalf("followup_eval_status = %q, want %q", got, harness.RunGroupStatusPending)
	}
	reloadedRevision, err := controller.GetSkillRevision(context.Background(), revision.ID)
	if err != nil {
		t.Fatalf("GetSkillRevision(revision.ID): %v", err)
	}
	if got, want := reloadedRevision.Status, harness.SkillRevisionStatusCandidate; got != want {
		t.Fatalf("revision status = %q, want %q", got, want)
	}
	reloadedCase, err := controller.GetSkillEvolutionCase(context.Background(), evolutionCase.ID)
	if err != nil {
		t.Fatalf("GetSkillEvolutionCase(evolutionCase.ID): %v", err)
	}
	if got, want := reloadedCase.Status, harness.SkillEvolutionCaseStatusCandidateCreated; got != want {
		t.Fatalf("evolution case status = %q, want %q", got, want)
	}

	status := adapter.GetStatus(context.Background())
	if got := strings.TrimSpace(status.LastOptimizationState); got != "running" {
		t.Fatalf("LastOptimizationState = %q, want running", got)
	}
}

func buildOptimizationTestRunnerBinary(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	serverDir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	bin := filepath.Join(t.TempDir(), "agentcore-runner")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/agentcore-runner")
	cmd.Dir = serverDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build runner failed: %v\n%s", err, output)
	}
	return bin
}

func buildScriptedOptimizationRunnerBinary(t *testing.T, responseText string) string {
	t.Helper()
	source := fmt.Sprintf(`package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "acp" {
		fmt.Fprintln(os.Stderr, "expected acp subcommand")
		os.Exit(2)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var payload map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &payload); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		id := fmt.Sprint(payload["id"])
		method := fmt.Sprint(payload["method"])
		switch method {
		case "initialize":
			writeJSON(map[string]interface{}{"jsonrpc":"2.0","id":id,"result":map[string]interface{}{"protocolVersion":1}})
		case "session/new":
			writeJSON(map[string]interface{}{"jsonrpc":"2.0","id":id,"result":map[string]interface{}{"sessionId":"session-test"}})
		case "session/prompt":
			writeJSON(map[string]interface{}{
				"jsonrpc":"2.0",
				"method":"session/update",
				"params":map[string]interface{}{
					"update":map[string]interface{}{
						"content":map[string]interface{}{"text":%q},
					},
				},
			})
			writeJSON(map[string]interface{}{"jsonrpc":"2.0","id":id,"result":map[string]interface{}{"stopReason":"completed"}})
		default:
			writeJSON(map[string]interface{}{"jsonrpc":"2.0","id":id,"result":map[string]interface{}{}})
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func writeJSON(payload map[string]interface{}) {
	data, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(append(data, '\n')); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`, responseText)

	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.go")
	if err := os.WriteFile(mainPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write scripted runner source: %v", err)
	}
	bin := filepath.Join(dir, "scripted-agentcore-runner")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, mainPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build scripted runner failed: %v\n%s", err, output)
	}
	return bin
}

type bootstrapSelectorEvalDriver struct {
	responsesByCandidate map[string]map[string]map[string]interface{}
}

func (d bootstrapSelectorEvalDriver) Kind() harness.RunKind { return harness.RunKindAgentTask }

func (d bootstrapSelectorEvalDriver) Validate(spec harness.RunSpec) error {
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d bootstrapSelectorEvalDriver) Start(ctx context.Context, run *harness.Run, env harness.RunEnv) error {
	if run == nil {
		return fmt.Errorf("run is required")
	}
	candidateID := strings.TrimSpace(asStringForOptimizationTest(run.Metadata["candidate_id"]))
	responses := d.responsesByCandidate[candidateID]
	if responses == nil {
		responses = d.responsesByCandidate[""]
	}
	response := cloneBootstrapMetadataMap(responses[run.Goal])
	if response == nil {
		response = map[string]interface{}{}
	}
	response["query"] = run.Goal
	response["model"] = strings.TrimSpace(run.Model)
	raw, err := json.Marshal(response)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	snapshot := *run
	snapshot.Status = harness.RunStatusCompleted
	snapshot.Result = string(raw)
	snapshot.UpdatedAt = now
	snapshot.FinishedAt = &now
	return env.Manager.SyncSnapshot(ctx, &snapshot)
}

func (d bootstrapSelectorEvalDriver) Cancel(context.Context, *harness.Run) error { return nil }

func newOptimizationManagerWithRunnerBinaryForTest(t *testing.T, root string, bin string) *optimization.Manager {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	status := optimization.Status{
		BinaryReady:    true,
		BinaryPath:     bin,
		BinarySHA256:   sha256FileForOptimizationTest(t, bin),
		ResolvedRef:    "main",
		ResolvedCommit: strings.Repeat("c", 40),
	}
	statusData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "status.json"), statusData, 0o644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}
	return manager
}

func newOptimizationManagerWithLastRunRecordForTest(t *testing.T, root string, recordID string, record map[string]interface{}) *optimization.Manager {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	status := optimization.Status{
		LastOptimizationRunID: strings.TrimSpace(recordID),
	}
	statusData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "status.json"), statusData, 0o644); err != nil {
		t.Fatalf("write status.json: %v", err)
	}
	manager, err := optimization.NewManager(root)
	if err != nil {
		t.Fatalf("optimization.NewManager: %v", err)
	}
	if err := manager.RecordOptimizationEvent(recordID, record); err != nil {
		t.Fatalf("RecordOptimizationEvent: %v", err)
	}
	return manager
}

func newOptimizationHarnessControllerForFollowupTest(t *testing.T) *harness.Controller {
	t.Helper()
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-harness-followup.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{}
	cfg.Harness.Enabled = true
	cfg.Harness.StorePath = filepath.Join(tmp, "blue.db")
	cfg.Harness.ArtifactRoot = filepath.Join(tmp, "artifacts")

	bundle, err := newHarnessRuntimeBundle(db, cfg, nil)
	if err != nil {
		t.Fatalf("newHarnessRuntimeBundle failed: %v", err)
	}
	if bundle == nil || bundle.Controller == nil {
		t.Fatalf("expected harness controller, got %#v", bundle)
	}
	return bundle.Controller
}

func createOptimizationEvalRunForFollowupTest(t *testing.T, controller *harness.Controller, metadata map[string]interface{}) *harness.EvalRun {
	t.Helper()
	dataset, err := controller.CreateDataset(context.Background(), harness.DatasetSpec{
		Name:           "Optimization Follow-up Dataset",
		OwnerUserID:    "user-1",
		Subject:        "agent_task",
		DefaultRunKind: harness.RunKindAgentTask,
		DefaultProfile: "agent_task",
	})
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}
	version, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, harness.DatasetVersionSpec{
		Version: "v1",
		Manifest: map[string]interface{}{
			"defaults": map[string]interface{}{
				"run_kind": "agent_task",
				"profile":  "agent_task",
			},
			"items": []interface{}{
				map[string]interface{}{
					"id": "case-1",
					"input": map[string]interface{}{
						"goal": "check optimization candidate",
					},
				},
			},
		},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}
	evalSpec, err := controller.CreateEvalSpec(context.Background(), harness.EvalSpecSpec{
		Name:             "Optimization Follow-up Eval",
		OwnerUserID:      "user-1",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RunKind:          harness.RunKindAgentTask,
		Profile:          "agent_task",
		ScoringConfig: harness.GroupScoringConfig{
			Mode:          harness.ScoringModeRule,
			PassThreshold: 0.5,
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}
	evalRun, err := controller.SubmitEvalRun(context.Background(), harness.EvalRunSpec{
		EvalSpecID:        evalSpec.ID,
		BaselineEvalRunID: "embedded-base-eval-run",
		OwnerUserID:       "user-1",
		Title:             "optimization-parent-run",
		Metadata:          metadata,
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}
	return evalRun
}

func createSelectorEvalSpecForOptimizationAssessmentTest(t *testing.T, controller *harness.Controller) *harness.EvalSpec {
	t.Helper()
	dataset, err := controller.CreateDataset(context.Background(), harness.DatasetSpec{
		Name:           "Optimization Selector Assessment",
		OwnerUserID:    "user-1",
		Subject:        harness.SelectorCuratedDatasetSubject,
		DefaultRunKind: harness.RunKindAgentTask,
		DefaultProfile: "selector_dry_run",
	})
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}
	version, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, harness.DatasetVersionSpec{
		Version: "v1",
		Manifest: map[string]interface{}{
			"dataset": map[string]interface{}{
				"name":    "optimization-selector-assessment",
				"subject": harness.SelectorCuratedDatasetSubject,
			},
			"defaults": map[string]interface{}{
				"run_kind": "agent_task",
				"profile":  "selector_dry_run",
			},
			"items": []interface{}{
				map[string]interface{}{
					"id": "selected-web_query-en-us",
					"input": map[string]interface{}{
						"goal": "Search the latest OpenAI Responses API documentation.",
					},
					"expected": map[string]interface{}{
						"canonical_skill_id":  "web_query",
						"skill_route_outcome": "selected",
						"skill_need_clarify":  false,
					},
					"metadata": map[string]interface{}{
						"locale":                     "en-US",
						"primary_route":              "web_query",
						"critical":                   true,
						"compare_fields":             []interface{}{"canonical_skill_id", "skill_route_outcome", "skill_need_clarify"},
						"allowed_alternative_routes": []interface{}{},
					},
				},
				map[string]interface{}{
					"id": "clarify-mixed-local-web-zh-cn",
					"input": map[string]interface{}{
						"goal": "看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？",
					},
					"expected": map[string]interface{}{
						"skill_route_outcome": "clarify",
						"skill_need_clarify":  true,
					},
					"metadata": map[string]interface{}{
						"locale":                     "zh-CN",
						"primary_route":              "exec",
						"critical":                   true,
						"compare_fields":             []interface{}{"canonical_skill_id", "skill_route_outcome", "skill_need_clarify"},
						"allowed_alternative_routes": []interface{}{"web_query", "analyze"},
					},
				},
			},
		},
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}
	evalSpec, err := controller.CreateEvalSpec(context.Background(), harness.EvalSpecSpec{
		Name:             "Optimization Selector Assessment Eval",
		OwnerUserID:      "user-1",
		Subject:          harness.SelectorCuratedDatasetSubject,
		RunKind:          harness.RunKindAgentTask,
		Profile:          "selector_dry_run",
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RuntimePolicy: map[string]interface{}{
			"driver":          "selector_dry_run",
			"target_endpoint": "/api/settings/selector/dry-run",
			"request_method":  "POST",
			"request_defaults": map[string]interface{}{
				"model": "auto",
			},
			"compare_fields": []interface{}{"canonical_skill_id", "skill_route_outcome", "skill_need_clarify"},
			"required_fields": []interface{}{
				"selected_tools",
				"skill_decision",
				"skill_prompt_hint",
				"canonical_skill_id",
				"skill_need_clarify",
				"skill_route_outcome",
			},
		},
		ScoringConfig: harness.GroupScoringConfig{
			Mode:          harness.ScoringModeRule,
			PassThreshold: 0.5,
		},
		Metadata: map[string]interface{}{
			"gate_type": "selection",
		},
	})
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}
	return evalSpec
}

func runSelectorEvalForOptimizationAssessmentTest(t *testing.T, controller *harness.Controller, evalSpec *harness.EvalSpec, title string, metadata map[string]interface{}) *harness.EvalRun {
	t.Helper()
	evalRun, err := controller.SubmitEvalRun(context.Background(), harness.EvalRunSpec{
		EvalSpecID:  evalSpec.ID,
		OwnerUserID: "user-1",
		Title:       title,
		Metadata:    metadata,
	})
	if err != nil {
		t.Fatalf("SubmitEvalRun failed: %v", err)
	}
	dispatcher := harness.NewGroupDispatcher(controller)
	dispatcher.SetPollInterval(10 * time.Millisecond)
	dispatcher.SetRunPollInterval(10 * time.Millisecond)
	if err := dispatcher.DispatchOnce(context.Background()); err != nil {
		t.Fatalf("DispatchOnce failed: %v", err)
	}
	waitForBootstrapCondition(t, "selector eval completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
		return err == nil && got != nil && got.Status == harness.RunGroupStatusCompleted
	})
	got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun failed: %v", err)
	}
	return got
}

func waitForBootstrapCondition(t *testing.T, label string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}

func selectorEvalResponseForBootstrapTest(skill string, clarify bool, outcome string) map[string]interface{} {
	selectedTools := []string{skill}
	selectedNativeTools := append([]string(nil), selectedTools...)
	nativeSurfaceMode := "legacy"
	nativeSurfaceReason := "legacy_native_surface"
	if clarify {
		selectedNativeTools = nil
		nativeSurfaceMode = "clarify_none"
		nativeSurfaceReason = "clarify_required"
	} else if len(selectedNativeTools) == 1 && selectedNativeTools[0] == "exec" {
		nativeSurfaceMode = "skill_exec"
		nativeSurfaceReason = "legacy_exec_collapse_compat"
	}
	toolPayload := make([]interface{}, 0, len(selectedTools))
	for _, tool := range selectedTools {
		toolPayload = append(toolPayload, tool)
	}
	nativeToolPayload := make([]interface{}, 0, len(selectedNativeTools))
	for _, tool := range selectedNativeTools {
		nativeToolPayload = append(nativeToolPayload, tool)
	}
	response := map[string]interface{}{
		"selected_tools":                 toolPayload,
		"selected_tool_surface":          selectorEvalToolSurfaceForBootstrapTest(selectedTools),
		"selected_native_tools":          nativeToolPayload,
		"selected_native_tool_surface":   selectorEvalToolSurfaceForBootstrapTest(selectedNativeTools),
		"selected_native_surface_mode":   nativeSurfaceMode,
		"selected_native_surface_reason": nativeSurfaceReason,
		"selected_canonical_skill":       skill,
		"selected_alias":                 skill,
		"execution_profile":              "inline",
		"skill_exec_cutover":             nativeSurfaceMode == "skill_exec",
		"forked_skill_execution":         false,
		"skill_decision":                 map[string]interface{}{"selected_skill": skill, "need_clarify": clarify},
		"discovery_decision": map[string]interface{}{
			"canonical_target":    skill,
			"alias_resolved":      skill,
			"need_clarify":        clarify,
			"execution_profile":   "inline",
			"native_surface_mode": nativeSurfaceMode,
		},
		"discovery_runtime": map[string]interface{}{
			"canonical_target":       skill,
			"selected_alias":         skill,
			"selected_native_mode":   nativeSurfaceMode,
			"native_surface_mode":    nativeSurfaceMode,
			"surface_reason":         nativeSurfaceReason,
			"execution_profile":      "inline",
			"skill_exec_cutover":     nativeSurfaceMode == "skill_exec",
			"forked_skill_execution": false,
		},
		"skill_prompt_hint":   "Use the curated selector route.",
		"canonical_skill_id":  skill,
		"skill_need_clarify":  clarify,
		"skill_route_outcome": outcome,
		"decision_reason":     "bootstrap_test",
		"decision_stage":      "rerank",
	}
	if clarify {
		response["clarify_reason"] = "The request mixes local-workspace and live-web intents."
	}
	raw, _ := json.Marshal(response)
	var cloned map[string]interface{}
	_ = json.Unmarshal(raw, &cloned)
	return cloned
}

func selectorEvalToolSurfaceForBootstrapTest(selectedTools []string) map[string]interface{} {
	tools := make([]interface{}, 0, len(selectedTools))
	for _, tool := range selectedTools {
		if strings.TrimSpace(tool) != "" {
			tools = append(tools, strings.TrimSpace(tool))
		}
	}
	return map[string]interface{}{
		"tools": tools,
		"mode":  "native",
	}
}

func cloneBootstrapMetadataMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func patchAgentcoreRunnerSettingsForOptimizationTest(t *testing.T, settings *serverpkg.SettingsHandler, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e := echo.New()
	if err := settings.Patch(e.NewContext(req, rec)); err != nil {
		t.Fatalf("settings.Patch: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("settings.Patch status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func sha256FileForOptimizationTest(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func asStringForOptimizationTest(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
