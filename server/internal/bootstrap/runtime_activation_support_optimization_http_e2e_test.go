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

func TestHarnessOptimizationHTTPE2E_OptimizeSkillSurfacesRunnerLastRunEvidence(t *testing.T) {
	controller := newBootstrapOptimizationHTTPController(t)
	controller.RegisterDriver(bootstrapHTTPE2ESelectorDriver{
		responsesByCandidate: map[string]map[string]map[string]interface{}{
			"": {
				"Search the latest OpenAI Responses API documentation.":            bootstrapHTTPEESelectorResponse("web_query", false, "selected"),
				"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？": bootstrapHTTPEESelectorResponse("exec", true, "clarify"),
			},
		},
	})

	evalSpec := createBootstrapOptimizationHTTPE2EEvalSpec(t, controller)
	parentEvalRun := runBootstrapOptimizationHTTPE2EParentEval(t, controller, evalSpec, "manual-http-optimize-parent", map[string]interface{}{
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

	managerRoot := filepath.Join(t.TempDir(), "agentcore-runner")
	manager := newBootstrapOptimizationManagerWithRunnerBinary(t, managerRoot, buildBootstrapScriptedOptimizationRunnerBinary(t, response))

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	patchBootstrapOptimizationHTTPSettings(t, settings, `{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`)
	bindHarnessRuntimeOptimization(settings, &HarnessRuntimeBundle{Controller: controller})

	e := echo.New()
	api := e.Group("/api/v1")
	settings.RegisterRoutes(api)
	harness.NewHandler(controller).RegisterRoutes(api.Group("/harness"))
	srv := httptest.NewServer(e)
	defer srv.Close()

	optimizeReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/harness/skills/browser/optimize", strings.NewReader(fmt.Sprintf(`{"eval_run_id":%q}`, parentEvalRun.ID)))
	if err != nil {
		t.Fatalf("new optimize request: %v", err)
	}
	optimizeReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	optimizeResp, err := srv.Client().Do(optimizeReq)
	if err != nil {
		t.Fatalf("POST optimize: %v", err)
	}
	defer optimizeResp.Body.Close()
	if optimizeResp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST optimize code=%d body=%s", optimizeResp.StatusCode, readBootstrapHTTPE2EBody(t, optimizeResp))
	}

	var event harness.OptimizationTrigger
	if err := json.NewDecoder(optimizeResp.Body).Decode(&event); err != nil {
		t.Fatalf("decode optimize response: %v", err)
	}
	if got, want := strings.TrimSpace(string(event.Reason)), string(harness.OptimizationReasonManualSkillOptimize); got != want {
		t.Fatalf("optimize reason=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(event.EvalRunID), parentEvalRun.ID; got != want {
		t.Fatalf("optimize eval_run_id=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(string(event.OptimizationSurface)), string(harness.OptimizationSurfaceSkillDefinition); got != want {
		t.Fatalf("optimize surface=%q, want %q", got, want)
	}

	lastRunResp, err := srv.Client().Get(srv.URL + "/api/v1/settings/agentcore-runner/last-run")
	if err != nil {
		t.Fatalf("GET last-run: %v", err)
	}
	defer lastRunResp.Body.Close()
	if lastRunResp.StatusCode != http.StatusOK {
		t.Fatalf("GET last-run code=%d body=%s", lastRunResp.StatusCode, readBootstrapHTTPE2EBody(t, lastRunResp))
	}

	var lastRun map[string]interface{}
	if err := json.NewDecoder(lastRunResp.Body).Decode(&lastRun); err != nil {
		t.Fatalf("decode last-run: %v", err)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["reason"])), string(harness.OptimizationReasonManualSkillOptimize); got != want {
		t.Fatalf("last-run reason=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["source_eval_run_id"])), parentEvalRun.ID; got != want {
		t.Fatalf("last-run source_eval_run_id=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["followup_gate"])), "selector"; got != want {
		t.Fatalf("last-run followup_gate=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["followup_state"])), "running"; got != want {
		t.Fatalf("last-run followup_state=%q, want %q; payload=%#v", got, want, lastRun)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["followup_eval_status"])), string(harness.RunGroupStatusPending); got != want {
		t.Fatalf("last-run followup_eval_status=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["promotion_state"])), string(harness.SkillRevisionStatusCandidate); got != want {
		t.Fatalf("last-run promotion_state=%q, want %q", got, want)
	}
	if got := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["runner_response_text"])); !strings.Contains(got, "candidate_ready") {
		t.Fatalf("last-run runner_response_text=%q, want candidate_ready", got)
	}

	skillRevisionID := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["skill_revision_id"]))
	if skillRevisionID == "" {
		t.Fatalf("last-run missing skill_revision_id: %#v", lastRun)
	}
	evolutionCaseID := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["skill_evolution_case_id"]))
	if evolutionCaseID == "" {
		t.Fatalf("last-run missing skill_evolution_case_id: %#v", lastRun)
	}
	followupEvalRunID := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["followup_eval_run_id"]))
	if followupEvalRunID == "" {
		t.Fatalf("last-run missing followup_eval_run_id: %#v", lastRun)
	}

	materialized, ok := lastRun["materialized_skill_candidate"].(map[string]interface{})
	if !ok {
		t.Fatalf("materialized_skill_candidate=%#v, want map", lastRun["materialized_skill_candidate"])
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(materialized["skill_id"])), "browser"; got != want {
		t.Fatalf("materialized skill_id=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(materialized["candidate_id"])), "candidate-browser-optimized"; got != want {
		t.Fatalf("materialized candidate_id=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(materialized["source_path"])), "assets/skills/browser/SKILL.md"; got != want {
		t.Fatalf("materialized source_path=%q, want %q", got, want)
	}
	if got, want := materialized["content"].(string), content; got != want {
		t.Fatalf("materialized content=%q, want %q", got, want)
	}

	revisionsResp, err := srv.Client().Get(srv.URL + "/api/v1/harness/skills/browser/revisions?limit=10")
	if err != nil {
		t.Fatalf("GET revisions: %v", err)
	}
	defer revisionsResp.Body.Close()
	if revisionsResp.StatusCode != http.StatusOK {
		t.Fatalf("GET revisions code=%d body=%s", revisionsResp.StatusCode, readBootstrapHTTPE2EBody(t, revisionsResp))
	}

	var revisions []harness.SkillRevision
	if err := json.NewDecoder(revisionsResp.Body).Decode(&revisions); err != nil {
		t.Fatalf("decode revisions: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("revisions len=%d, want 1", len(revisions))
	}
	if got, want := revisions[0].ID, skillRevisionID; got != want {
		t.Fatalf("revision id=%q, want %q", got, want)
	}
	if got, want := revisions[0].OriginCaseID, evolutionCaseID; got != want {
		t.Fatalf("revision origin_case_id=%q, want %q", got, want)
	}
	if got, want := revisions[0].Status, harness.SkillRevisionStatusCandidate; got != want {
		t.Fatalf("revision status=%q, want %q", got, want)
	}
	if got, want := revisions[0].FollowupGate, "selector"; got != want {
		t.Fatalf("revision followup_gate=%q, want %q", got, want)
	}
	if got, want := revisions[0].Content, content; got != want {
		t.Fatalf("revision content=%q, want %q", got, want)
	}

	casesResp, err := srv.Client().Get(srv.URL + "/api/v1/harness/skills/browser/evolution-cases?limit=10")
	if err != nil {
		t.Fatalf("GET evolution-cases: %v", err)
	}
	defer casesResp.Body.Close()
	if casesResp.StatusCode != http.StatusOK {
		t.Fatalf("GET evolution-cases code=%d body=%s", casesResp.StatusCode, readBootstrapHTTPE2EBody(t, casesResp))
	}

	var cases []harness.SkillEvolutionCase
	if err := json.NewDecoder(casesResp.Body).Decode(&cases); err != nil {
		t.Fatalf("decode evolution-cases: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("evolution cases len=%d, want 1", len(cases))
	}
	if got, want := cases[0].ID, evolutionCaseID; got != want {
		t.Fatalf("evolution case id=%q, want %q", got, want)
	}
	if got, want := cases[0].RevisionID, skillRevisionID; got != want {
		t.Fatalf("evolution case revision_id=%q, want %q", got, want)
	}
	if got, want := cases[0].CandidateID, "candidate-browser-optimized"; got != want {
		t.Fatalf("evolution case candidate_id=%q, want %q", got, want)
	}
	if got, want := cases[0].Status, harness.SkillEvolutionCaseStatusCandidateCreated; got != want {
		t.Fatalf("evolution case status=%q, want %q", got, want)
	}
	if got, want := cases[0].Reason, harness.SkillEvolutionReasonManual; got != want {
		t.Fatalf("evolution case reason=%q, want %q", got, want)
	}
	if got, want := cases[0].SourceID, parentEvalRun.ID; got != want {
		t.Fatalf("evolution case source_id=%q, want %q", got, want)
	}
}

func TestHarnessOptimizationHTTPE2E_RuntimeSkillFailureAutoCreatesEvolutionCase(t *testing.T) {
	repoRoot, canonicalContent := createRuntimeSkillEvolutionRepoForTest(t, "browser")
	restoreWD := chdirRuntimeSkillEvolutionTest(t, repoRoot)
	defer restoreWD()

	controller := newBootstrapOptimizationHTTPController(t)
	controller.RegisterDriver(&runtimeTerminalHarnessDriver{
		kind:           harness.RunKindAgentTask,
		terminalStatus: harness.RunStatusFailed,
		terminalError:  "Page title extraction failed after browser navigation.",
		terminalResult: "Browser task failed after partial navigation recovery.",
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
	})

	content := strings.TrimSpace(`
---
name: browser
description: Improved browser skill
---

# Browser

Prefer validating page titles before extracting summaries.
`) + "\n"
	response := "Candidate ready.\n```json\n" + strings.TrimSpace(fmt.Sprintf(`{
  "status": "candidate_ready",
  "message": "Improved browser runtime fix candidate",
  "skill_candidate": {
    "skill_id": "browser",
    "candidate_id": "candidate-browser-runtime-optimized",
    "content": %q
  }
}`, content)) + "\n```"

	managerRoot := filepath.Join(t.TempDir(), "agentcore-runner")
	manager := newBootstrapOptimizationManagerWithRunnerBinary(t, managerRoot, buildBootstrapScriptedOptimizationRunnerBinary(t, response))

	settings := serverpkg.NewSettingsHandler(kvstore.NewMemoryStore())
	settings.SetAgentcoreRunnerManager(manager)
	patchBootstrapOptimizationHTTPSettings(t, settings, `{
		"experimental_agentcore_runner_enabled": true,
		"experimental_agentcore_runner_repo_url": "https://github.com/IceWhaleTech/ZimaOS-Blue",
		"experimental_agentcore_runner_ref": "main"
	}`)
	bindHarnessRuntimeOptimization(settings, &HarnessRuntimeBundle{Controller: controller})

	e := echo.New()
	api := e.Group("/api/v1")
	settings.RegisterRoutes(api)
	harness.NewHandler(controller).RegisterRoutes(api.Group("/harness"))
	srv := httptest.NewServer(e)
	defer srv.Close()

	createReq, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/harness/runs", strings.NewReader(`{
		"kind": "agent_task",
		"goal": "Investigate browser regression",
		"user_id": "user-1",
		"metadata": {
			"selected_canonical_skill": "browser"
		}
	}`))
	if err != nil {
		t.Fatalf("new create run request: %v", err)
	}
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createResp, err := srv.Client().Do(createReq)
	if err != nil {
		t.Fatalf("POST create run: %v", err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("POST create run code=%d body=%s", createResp.StatusCode, readBootstrapHTTPE2EBody(t, createResp))
	}

	var runtimeRun harness.Run
	if err := json.NewDecoder(createResp.Body).Decode(&runtimeRun); err != nil {
		t.Fatalf("decode create run response: %v", err)
	}
	if strings.TrimSpace(runtimeRun.ID) == "" {
		t.Fatalf("runtime run id is empty: %#v", runtimeRun)
	}
	if got, want := runtimeRun.Status, harness.RunStatusFailed; got != want {
		t.Fatalf("runtime run status=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(runtimeRun.Error), "Page title extraction failed after browser navigation."; got != want {
		t.Fatalf("runtime run error=%q, want %q", got, want)
	}

	lastRunResp, err := srv.Client().Get(srv.URL + "/api/v1/settings/agentcore-runner/last-run")
	if err != nil {
		t.Fatalf("GET last-run: %v", err)
	}
	defer lastRunResp.Body.Close()
	if lastRunResp.StatusCode != http.StatusOK {
		t.Fatalf("GET last-run code=%d body=%s", lastRunResp.StatusCode, readBootstrapHTTPE2EBody(t, lastRunResp))
	}

	var lastRun map[string]interface{}
	if err := json.NewDecoder(lastRunResp.Body).Decode(&lastRun); err != nil {
		t.Fatalf("decode last-run: %v", err)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["reason"])), string(harness.OptimizationReasonRuntimeSkillFailure); got != want {
		t.Fatalf("last-run reason=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["followup_gate"])), "execution"; got != want {
		t.Fatalf("last-run followup_gate=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["followup_state"])), "running"; got != want {
		t.Fatalf("last-run followup_state=%q, want %q; payload=%#v", got, want, lastRun)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["followup_eval_status"])), string(harness.RunGroupStatusPending); got != want {
		t.Fatalf("last-run followup_eval_status=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["promotion_state"])), string(harness.SkillRevisionStatusCandidate); got != want {
		t.Fatalf("last-run promotion_state=%q, want %q", got, want)
	}

	skillRevisionID := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["skill_revision_id"]))
	if skillRevisionID == "" {
		t.Fatalf("last-run missing skill_revision_id: %#v", lastRun)
	}
	evolutionCaseID := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["skill_evolution_case_id"]))
	if evolutionCaseID == "" {
		t.Fatalf("last-run missing skill_evolution_case_id: %#v", lastRun)
	}
	followupEvalRunID := strings.TrimSpace(bootstrapHTTPE2EString(lastRun["followup_eval_run_id"]))
	if followupEvalRunID == "" {
		t.Fatalf("last-run missing followup_eval_run_id: %#v", lastRun)
	}

	materialized, ok := lastRun["materialized_skill_candidate"].(map[string]interface{})
	if !ok {
		t.Fatalf("materialized_skill_candidate=%#v, want map", lastRun["materialized_skill_candidate"])
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(materialized["skill_id"])), "browser"; got != want {
		t.Fatalf("materialized skill_id=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(materialized["candidate_id"])), "candidate-browser-runtime-optimized"; got != want {
		t.Fatalf("materialized candidate_id=%q, want %q", got, want)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(materialized["source_path"])), "assets/skills/browser/SKILL.md"; got != want {
		t.Fatalf("materialized source_path=%q, want %q", got, want)
	}
	if got, want := materialized["content"].(string), content; got != want {
		t.Fatalf("materialized content=%q, want %q", got, want)
	}

	revisionsResp, err := srv.Client().Get(srv.URL + "/api/v1/harness/skills/browser/revisions?limit=10")
	if err != nil {
		t.Fatalf("GET revisions: %v", err)
	}
	defer revisionsResp.Body.Close()
	if revisionsResp.StatusCode != http.StatusOK {
		t.Fatalf("GET revisions code=%d body=%s", revisionsResp.StatusCode, readBootstrapHTTPE2EBody(t, revisionsResp))
	}

	var revisions []harness.SkillRevision
	if err := json.NewDecoder(revisionsResp.Body).Decode(&revisions); err != nil {
		t.Fatalf("decode revisions: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("revisions len=%d, want 1", len(revisions))
	}
	if got, want := revisions[0].ID, skillRevisionID; got != want {
		t.Fatalf("revision id=%q, want %q", got, want)
	}
	if got, want := revisions[0].OriginCaseID, evolutionCaseID; got != want {
		t.Fatalf("revision origin_case_id=%q, want %q", got, want)
	}
	if got, want := revisions[0].Status, harness.SkillRevisionStatusCandidate; got != want {
		t.Fatalf("revision status=%q, want %q", got, want)
	}
	if got, want := revisions[0].FollowupGate, "execution"; got != want {
		t.Fatalf("revision followup_gate=%q, want %q", got, want)
	}
	if got, want := revisions[0].BaseContentSHA256, sha256HexForOptimizationRuntimeTest(canonicalContent); got != want {
		t.Fatalf("revision base_content_sha256=%q, want %q", got, want)
	}
	if got, want := revisions[0].EvalRunID, followupEvalRunID; got != want {
		t.Fatalf("revision eval_run_id=%q, want %q", got, want)
	}
	if got, want := revisions[0].Content, content; got != want {
		t.Fatalf("revision content=%q, want %q", got, want)
	}

	casesResp, err := srv.Client().Get(srv.URL + "/api/v1/harness/skills/browser/evolution-cases?limit=10")
	if err != nil {
		t.Fatalf("GET evolution-cases: %v", err)
	}
	defer casesResp.Body.Close()
	if casesResp.StatusCode != http.StatusOK {
		t.Fatalf("GET evolution-cases code=%d body=%s", casesResp.StatusCode, readBootstrapHTTPE2EBody(t, casesResp))
	}

	var cases []harness.SkillEvolutionCase
	if err := json.NewDecoder(casesResp.Body).Decode(&cases); err != nil {
		t.Fatalf("decode evolution-cases: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("evolution cases len=%d, want 1", len(cases))
	}
	if got, want := cases[0].ID, evolutionCaseID; got != want {
		t.Fatalf("evolution case id=%q, want %q", got, want)
	}
	if got, want := cases[0].RevisionID, skillRevisionID; got != want {
		t.Fatalf("evolution case revision_id=%q, want %q", got, want)
	}
	if got, want := cases[0].CandidateID, "candidate-browser-runtime-optimized"; got != want {
		t.Fatalf("evolution case candidate_id=%q, want %q", got, want)
	}
	if got, want := cases[0].Status, harness.SkillEvolutionCaseStatusCandidateCreated; got != want {
		t.Fatalf("evolution case status=%q, want %q", got, want)
	}
	if got, want := cases[0].Reason, harness.SkillEvolutionReasonRuntimeFailure; got != want {
		t.Fatalf("evolution case reason=%q, want %q", got, want)
	}
	if got, want := cases[0].SourceKind, "runtime_run"; got != want {
		t.Fatalf("evolution case source_kind=%q, want %q", got, want)
	}
	if got, want := cases[0].SourceID, runtimeRun.ID; got != want {
		t.Fatalf("evolution case source_id=%q, want %q", got, want)
	}

	detailResp, err := srv.Client().Get(srv.URL + "/api/v1/harness/skill-evolution-cases/" + evolutionCaseID)
	if err != nil {
		t.Fatalf("GET evolution-case detail: %v", err)
	}
	defer detailResp.Body.Close()
	if detailResp.StatusCode != http.StatusOK {
		t.Fatalf("GET evolution-case detail code=%d body=%s", detailResp.StatusCode, readBootstrapHTTPE2EBody(t, detailResp))
	}

	var detail harness.SkillEvolutionCaseDetail
	if err := json.NewDecoder(detailResp.Body).Decode(&detail); err != nil {
		t.Fatalf("decode evolution-case detail: %v", err)
	}
	if got, want := detail.SourceRunID, runtimeRun.ID; got != want {
		t.Fatalf("detail source_run_id=%q, want %q", got, want)
	}
	if detail.SourceRun == nil || detail.SourceRun.ID != runtimeRun.ID {
		t.Fatalf("detail source_run=%#v, want run %q", detail.SourceRun, runtimeRun.ID)
	}
	if detail.LinkedRevision == nil || detail.LinkedRevision.ID != skillRevisionID {
		t.Fatalf("detail linked_revision=%#v, want %q", detail.LinkedRevision, skillRevisionID)
	}
	if got, want := detail.LinkedEvalRunID, followupEvalRunID; got != want {
		t.Fatalf("detail linked_eval_run_id=%q, want %q", got, want)
	}
	if detail.LinkedEvalRun == nil || detail.LinkedEvalRun.ID != followupEvalRunID {
		t.Fatalf("detail linked_eval_run=%#v, want eval run %q", detail.LinkedEvalRun, followupEvalRunID)
	}

	var evidence map[string]interface{}
	if err := json.Unmarshal([]byte(detail.EvidenceJSON), &evidence); err != nil {
		t.Fatalf("decode evolution evidence: %v", err)
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(evidence["runtime_run_id"])), runtimeRun.ID; got != want {
		t.Fatalf("evidence runtime_run_id=%q, want %q", got, want)
	}
	runtimeUsage, ok := evidence["runtime_usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("evidence runtime_usage=%#v, want map", evidence["runtime_usage"])
	}
	if got, want := int(runtimeUsage["total_tokens"].(float64)), 165; got != want {
		t.Fatalf("evidence runtime_usage.total_tokens=%d, want %d", got, want)
	}
	runtimeQuality, ok := evidence["runtime_quality"].(map[string]interface{})
	if !ok {
		t.Fatalf("evidence runtime_quality=%#v, want map", evidence["runtime_quality"])
	}
	if got, ok := runtimeQuality["verification_passed"].(bool); !ok || got {
		t.Fatalf("evidence runtime_quality.verification_passed=%#v, want false", runtimeQuality["verification_passed"])
	}
	if got, want := strings.TrimSpace(bootstrapHTTPE2EString(runtimeQuality["failure_label"])), "missing_title"; got != want {
		t.Fatalf("evidence runtime_quality.failure_label=%q, want %q", got, want)
	}
}

func readBootstrapHTTPE2EBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	if resp == nil || resp.Body == nil {
		return ""
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil {
		data, marshalErr := json.Marshal(body)
		if marshalErr == nil {
			return string(data)
		}
	}
	return ""
}

type bootstrapHTTPE2ESelectorDriver struct {
	responsesByCandidate map[string]map[string]map[string]interface{}
}

func (d bootstrapHTTPE2ESelectorDriver) Kind() harness.RunKind { return harness.RunKindAgentTask }

func (d bootstrapHTTPE2ESelectorDriver) Validate(spec harness.RunSpec) error {
	if strings.TrimSpace(spec.Goal) == "" {
		return fmt.Errorf("goal is required")
	}
	return nil
}

func (d bootstrapHTTPE2ESelectorDriver) Start(ctx context.Context, run *harness.Run, env harness.RunEnv) error {
	if run == nil {
		return fmt.Errorf("run is required")
	}
	candidateID := strings.TrimSpace(bootstrapHTTPE2EString(run.Metadata["candidate_id"]))
	responses := d.responsesByCandidate[candidateID]
	if responses == nil {
		responses = d.responsesByCandidate[""]
	}
	response := bootstrapHTTPE2ECloneMap(responses[run.Goal])
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

func (d bootstrapHTTPE2ESelectorDriver) Cancel(context.Context, *harness.Run) error { return nil }

func newBootstrapOptimizationHTTPController(t *testing.T) *harness.Controller {
	t.Helper()
	tmpDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmpDir, "runtime-harness-http-e2e.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store, err := harness.NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore failed: %v", err)
	}
	harnessCfg := *config.DefaultHarnessConfig()
	harnessCfg.Enabled = true
	harnessCfg.StorePath = filepath.Join(tmpDir, "blue.db")
	harnessCfg.ArtifactRoot = filepath.Join(tmpDir, "artifacts")
	return harness.NewController(store, harness.NewPolicyResolver(harnessCfg, nil))
}

func createBootstrapOptimizationHTTPE2EEvalSpec(t *testing.T, controller *harness.Controller) *harness.EvalSpec {
	t.Helper()
	dataset, err := controller.CreateDataset(context.Background(), harness.DatasetSpec{
		Name:           "Optimization HTTP E2E Selector Dataset",
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
				"name":    "optimization-http-e2e-selector",
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
		Name:             "Optimization HTTP E2E Selector Eval",
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

func runBootstrapOptimizationHTTPE2EParentEval(t *testing.T, controller *harness.Controller, evalSpec *harness.EvalSpec, title string, metadata map[string]interface{}) *harness.EvalRun {
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
	bootstrapHTTPE2EWaitForCondition(t, "parent eval completion", func() bool {
		got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
		return err == nil && got != nil && got.Status == harness.RunGroupStatusCompleted
	})
	got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun failed: %v", err)
	}
	return got
}

func buildBootstrapScriptedOptimizationRunnerBinary(t *testing.T, responseText string) string {
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
	cmd.Env = optimizationTestGoBuildEnv(t, dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build scripted runner failed: %v\n%s", err, output)
	}
	return bin
}

func newBootstrapOptimizationManagerWithRunnerBinary(t *testing.T, root string, bin string) *optimization.Manager {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	status := optimization.Status{
		BinaryReady:    true,
		BinaryPath:     bin,
		BinarySHA256:   bootstrapHTTPE2ESHA256File(t, bin),
		RepoURL:        optimizationTestRunnerRepoURL,
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

func patchBootstrapOptimizationHTTPSettings(t *testing.T, settings *serverpkg.SettingsHandler, body string) {
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

func bootstrapHTTPE2EWaitForCondition(t *testing.T, label string, check func() bool) {
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

func bootstrapHTTPEESelectorResponse(skill string, clarify bool, outcome string) map[string]interface{} {
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
		"selected_tool_surface":          bootstrapHTTPE2EToolSurface(selectedTools),
		"selected_native_tools":          nativeToolPayload,
		"selected_native_tool_surface":   bootstrapHTTPE2EToolSurface(selectedNativeTools),
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
		"decision_reason":     "bootstrap_http_e2e_test",
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

func bootstrapHTTPE2EToolSurface(selectedTools []string) map[string]interface{} {
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

func bootstrapHTTPE2ECloneMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func bootstrapHTTPE2ESHA256File(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func bootstrapHTTPE2EString(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
