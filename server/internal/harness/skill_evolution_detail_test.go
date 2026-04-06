package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func TestController_GetSkillEvolutionCaseDetailIncludesLinkedRevisionAndEvalRuns(t *testing.T) {
	ctx := context.Background()
	controller := newMinimalTestController(t)
	now := time.Now().UTC()

	sourceEvalRun := createSkillEvolutionDetailEvalRun(t, controller, EvalRun{
		ID:          "eval-source",
		Title:       "source eval",
		OwnerUserID: "user-1",
		Status:      RunGroupStatusCompleted,
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now.Add(-2 * time.Minute),
	})
	linkedEvalRun := createSkillEvolutionDetailEvalRun(t, controller, EvalRun{
		ID:          "eval-linked",
		Title:       "linked eval",
		OwnerUserID: "user-1",
		Status:      RunGroupStatusCompleted,
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
	})

	revision, err := controller.CreateSkillRevision(ctx, SkillRevision{
		ID:                "rev-browser-candidate",
		SkillID:           "browser",
		Status:            SkillRevisionStatusCandidate,
		SourcePath:        "assets/skills/browser/SKILL.md",
		CandidateID:       "candidate-browser-fix",
		BaseContentSHA256: "base-browser-sha",
		OriginCaseID:      "case-browser-fix",
		EvalRunID:         linkedEvalRun.ID,
		Content:           "# Browser\nCandidate revision.\n",
		CreatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}

	evolutionCase, err := controller.CreateSkillEvolutionCase(ctx, SkillEvolutionCase{
		ID:                "case-browser-fix",
		SkillID:           "browser",
		OwnerUserID:       "user-1",
		Mode:              SkillEvolutionModeFix,
		Reason:            SkillEvolutionReasonRuntimeFailure,
		SourceKind:        "eval_run",
		SourceID:          sourceEvalRun.ID,
		CandidateID:       "candidate-browser-fix",
		BaseContentSHA256: "base-browser-sha",
		FailureSignature:  "missing-title",
		Summary:           "Browser skill missed the title extraction step.",
		EvidenceJSON:      `{"task_id":"task-1"}`,
		RevisionID:        revision.ID,
		Status:            SkillEvolutionCaseStatusCandidateCreated,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateSkillEvolutionCase failed: %v", err)
	}

	detail, err := controller.GetSkillEvolutionCaseDetail(ctx, evolutionCase.ID)
	if err != nil {
		t.Fatalf("GetSkillEvolutionCaseDetail failed: %v", err)
	}
	if got, want := detail.ID, evolutionCase.ID; got != want {
		t.Fatalf("detail.ID = %q, want %q", got, want)
	}
	if detail.LinkedRevision == nil || detail.LinkedRevision.ID != revision.ID {
		t.Fatalf("detail.LinkedRevision = %#v, want revision %q", detail.LinkedRevision, revision.ID)
	}
	if got, want := detail.SourceEvalRunID, sourceEvalRun.ID; got != want {
		t.Fatalf("detail.SourceEvalRunID = %q, want %q", got, want)
	}
	if detail.SourceEvalRun == nil || detail.SourceEvalRun.ID != sourceEvalRun.ID {
		t.Fatalf("detail.SourceEvalRun = %#v, want eval run %q", detail.SourceEvalRun, sourceEvalRun.ID)
	}
	if got, want := detail.LinkedEvalRunID, linkedEvalRun.ID; got != want {
		t.Fatalf("detail.LinkedEvalRunID = %q, want %q", got, want)
	}
	if detail.LinkedEvalRun == nil || detail.LinkedEvalRun.ID != linkedEvalRun.ID {
		t.Fatalf("detail.LinkedEvalRun = %#v, want eval run %q", detail.LinkedEvalRun, linkedEvalRun.ID)
	}
}

func TestController_GetSkillEvolutionCaseDetailIncludesSourceRunForRuntimeCases(t *testing.T) {
	ctx := context.Background()
	controller := newMinimalTestController(t)
	now := time.Now().UTC()
	startedAt := now.Add(-3 * time.Second)
	finishedAt := now.Add(-500 * time.Millisecond)

	sourceRun := &Run{
		ID:           "run-browser-runtime-source",
		RootRunID:    "run-browser-runtime-source",
		Kind:         RunKindAgentTask,
		Status:       RunStatusFailed,
		RuntimeState: "execute",
		UserID:       "user-1",
		Goal:         "Investigate browser regression",
		Result:       "Browser task produced partial output before failure.",
		Error:        "navigation timed out",
		Metadata: map[string]interface{}{
			"selected_canonical_skill": "browser",
		},
		CreatedAt:  now.Add(-4 * time.Second),
		UpdatedAt:  now,
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
	}
	if err := controller.store.CreateRun(ctx, sourceRun); err != nil {
		t.Fatalf("CreateRun(source) failed: %v", err)
	}

	evolutionCase, err := controller.CreateSkillEvolutionCase(ctx, SkillEvolutionCase{
		ID:                "case-browser-runtime-source",
		SkillID:           "browser",
		OwnerUserID:       "user-1",
		Mode:              SkillEvolutionModeFix,
		Reason:            SkillEvolutionReasonRuntimeFailure,
		SourceKind:        "runtime_run",
		SourceID:          sourceRun.ID,
		CandidateID:       "candidate-browser-runtime",
		BaseContentSHA256: "base-browser-runtime",
		FailureSignature:  "navigation-timeout",
		Summary:           "Browser runtime run exposed a reusable recovery gap.",
		EvidenceJSON:      `{"runtime_run_id":"run-browser-runtime-source"}`,
		Status:            SkillEvolutionCaseStatusOpen,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateSkillEvolutionCase failed: %v", err)
	}

	detail, err := controller.GetSkillEvolutionCaseDetail(ctx, evolutionCase.ID)
	if err != nil {
		t.Fatalf("GetSkillEvolutionCaseDetail failed: %v", err)
	}
	if got, want := detail.SourceRunID, sourceRun.ID; got != want {
		t.Fatalf("detail.SourceRunID = %q, want %q", got, want)
	}
	if detail.SourceRun == nil || detail.SourceRun.ID != sourceRun.ID {
		t.Fatalf("detail.SourceRun = %#v, want run %q", detail.SourceRun, sourceRun.ID)
	}
}

func TestHandler_GetSkillEvolutionCaseReturnsDetailPayload(t *testing.T) {
	ctx := context.Background()
	controller := newMinimalTestController(t)
	now := time.Now().UTC()

	sourceEvalRun := createSkillEvolutionDetailEvalRun(t, controller, EvalRun{
		ID:          "eval-source-detail",
		Title:       "source eval detail",
		OwnerUserID: "user-1",
		Status:      RunGroupStatusCompleted,
		CreatedAt:   now.Add(-2 * time.Minute),
		UpdatedAt:   now.Add(-2 * time.Minute),
	})
	linkedEvalRun := createSkillEvolutionDetailEvalRun(t, controller, EvalRun{
		ID:          "eval-linked-detail",
		Title:       "linked eval detail",
		OwnerUserID: "user-1",
		Status:      RunGroupStatusCompleted,
		CreatedAt:   now.Add(-time.Minute),
		UpdatedAt:   now.Add(-time.Minute),
	})
	revision, err := controller.CreateSkillRevision(ctx, SkillRevision{
		ID:          "rev-browser-detail",
		SkillID:     "browser",
		Status:      SkillRevisionStatusAccepted,
		SourcePath:  "assets/skills/browser/SKILL.md",
		CandidateID: "candidate-browser-detail",
		EvalRunID:   linkedEvalRun.ID,
		Content:     "# Browser\nAccepted revision.\n",
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}
	evolutionCase, err := controller.CreateSkillEvolutionCase(ctx, SkillEvolutionCase{
		ID:            "case-browser-detail",
		SkillID:       "browser",
		OwnerUserID:   "user-1",
		Mode:          SkillEvolutionModeFix,
		Reason:        SkillEvolutionReasonRuntimeFailure,
		SourceKind:    "eval_run",
		SourceID:      sourceEvalRun.ID,
		CandidateID:   "candidate-browser-detail",
		Summary:       "Browser detail case.",
		EvidenceJSON:  `{"task_id":"task-detail"}`,
		RevisionID:    revision.ID,
		Status:        SkillEvolutionCaseStatusAccepted,
		SkippedReason: "",
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("CreateSkillEvolutionCase failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skill-evolution-cases/"+evolutionCase.ID, nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(evolutionCase.ID)

	if err := handler.GetSkillEvolutionCase(c); err != nil {
		t.Fatalf("GetSkillEvolutionCase returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var got SkillEvolutionCaseDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.ID != evolutionCase.ID || got.SkillID != "browser" || got.OwnerUserID != "user-1" {
		t.Fatalf("detail = %#v, want browser case %q", got, evolutionCase.ID)
	}
	if got.LinkedRevision == nil || got.LinkedRevision.ID != revision.ID {
		t.Fatalf("detail.LinkedRevision = %#v, want %q", got.LinkedRevision, revision.ID)
	}
	if got.SourceEvalRun == nil || got.SourceEvalRun.ID != sourceEvalRun.ID {
		t.Fatalf("detail.SourceEvalRun = %#v, want %q", got.SourceEvalRun, sourceEvalRun.ID)
	}
	if got.LinkedEvalRun == nil || got.LinkedEvalRun.ID != linkedEvalRun.ID {
		t.Fatalf("detail.LinkedEvalRun = %#v, want %q", got.LinkedEvalRun, linkedEvalRun.ID)
	}
}

func createSkillEvolutionDetailEvalRun(t *testing.T, controller *Controller, evalRun EvalRun) *EvalRun {
	t.Helper()
	if evalRun.ID == "" {
		t.Fatal("eval run id is required")
	}
	if evalRun.GroupID == "" {
		evalRun.GroupID = "group-" + evalRun.ID
	}
	if evalRun.Status == "" {
		evalRun.Status = RunGroupStatusCompleted
	}
	if err := controller.store.CreateEvalRun(context.Background(), &evalRun); err != nil {
		t.Fatalf("CreateEvalRun(%q) failed: %v", evalRun.ID, err)
	}
	got, err := controller.GetEvalRun(context.Background(), evalRun.ID)
	if err != nil {
		t.Fatalf("GetEvalRun(%q) failed: %v", evalRun.ID, err)
	}
	return got
}
