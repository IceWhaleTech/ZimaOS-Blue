package harness

import (
	"context"
	"testing"
	"time"
)

func TestController_SkillEvolutionCaseCRUDAndFilter(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)
	now := time.Now().UTC()

	browserCase, err := controller.CreateSkillEvolutionCase(ctx, SkillEvolutionCase{
		ID:                "case-browser-fix-1",
		SkillID:           "browser",
		OwnerUserID:       "user-1",
		Mode:              SkillEvolutionModeFix,
		Reason:            SkillEvolutionReasonRuntimeFailure,
		SourceKind:        "runtime_task",
		SourceID:          "task-1",
		CandidateID:       "candidate-browser-fix-1",
		BaseContentSHA256: "base-browser-sha",
		FailureSignature:  "missing-title",
		Summary:           "Browser skill missed the page-title extraction step.",
		EvidenceJSON:      `{"final_status":"failed"}`,
		RevisionID:        "rev-browser-fix-1",
		Status:            SkillEvolutionCaseStatusCandidateCreated,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateSkillEvolutionCase(browser) failed: %v", err)
	}
	_, err = controller.CreateSkillEvolutionCase(ctx, SkillEvolutionCase{
		ID:                "case-reminder-capture-1",
		SkillID:           "reminder",
		OwnerUserID:       "user-2",
		Mode:              SkillEvolutionModeCapture,
		Reason:            SkillEvolutionReasonRuntimeCapture,
		SourceKind:        "runtime_task",
		SourceID:          "task-2",
		CandidateID:       "candidate-reminder-capture-1",
		BaseContentSHA256: "base-reminder-sha",
		FailureSignature:  "captured-recurring-example",
		Summary:           "Reminder skill surfaced a reusable scheduling example.",
		EvidenceJSON:      `{"final_status":"completed"}`,
		Status:            SkillEvolutionCaseStatusOpen,
		CreatedAt:         now.Add(time.Second),
		UpdatedAt:         now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("CreateSkillEvolutionCase(reminder) failed: %v", err)
	}

	stored, err := controller.GetSkillEvolutionCase(ctx, browserCase.ID)
	if err != nil {
		t.Fatalf("GetSkillEvolutionCase(browser) failed: %v", err)
	}
	if got, want := stored.Mode, SkillEvolutionModeFix; got != want {
		t.Fatalf("Mode = %q, want %q", got, want)
	}
	if got, want := stored.Reason, SkillEvolutionReasonRuntimeFailure; got != want {
		t.Fatalf("Reason = %q, want %q", got, want)
	}
	if got, want := stored.FailureSignature, "missing-title"; got != want {
		t.Fatalf("FailureSignature = %q, want %q", got, want)
	}
	if got, want := stored.RevisionID, "rev-browser-fix-1"; got != want {
		t.Fatalf("RevisionID = %q, want %q", got, want)
	}

	stored.Status = SkillEvolutionCaseStatusAccepted
	stored.UpdatedAt = now.Add(2 * time.Second)
	if _, err := controller.UpdateSkillEvolutionCase(ctx, *stored); err != nil {
		t.Fatalf("UpdateSkillEvolutionCase(browser) failed: %v", err)
	}

	cases, err := controller.ListSkillEvolutionCases(ctx, SkillEvolutionCaseFilter{
		SkillID:     "browser",
		OwnerUserID: "user-1",
		Statuses:    []SkillEvolutionCaseStatus{SkillEvolutionCaseStatusAccepted},
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListSkillEvolutionCases failed: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("ListSkillEvolutionCases len = %d, want 1", len(cases))
	}
	if got, want := cases[0].ID, browserCase.ID; got != want {
		t.Fatalf("case id = %q, want %q", got, want)
	}
	if got, want := cases[0].Status, SkillEvolutionCaseStatusAccepted; got != want {
		t.Fatalf("case status = %q, want %q", got, want)
	}
}

func TestController_EnsureSkillEvolutionCaseDedupsByBaseHashAndFailureSignature(t *testing.T) {
	ctx := context.Background()
	controller := newTestController(t)

	first, created, err := controller.EnsureSkillEvolutionCase(ctx, SkillEvolutionCaseSpec{
		SkillID:           "browser",
		OwnerUserID:       "user-1",
		Mode:              SkillEvolutionModeFix,
		Reason:            SkillEvolutionReasonRuntimeFailure,
		SourceKind:        "runtime_task",
		SourceID:          "task-1",
		BaseContentSHA256: "base-browser-sha",
		FailureSignature:  "missing-title",
		Summary:           "The browser skill skipped the title extraction fallback.",
		EvidenceJSON:      `{"task_id":"task-1"}`,
	})
	if err != nil {
		t.Fatalf("EnsureSkillEvolutionCase(first) failed: %v", err)
	}
	if !created {
		t.Fatal("expected first EnsureSkillEvolutionCase call to create a new case")
	}
	if got, want := first.Status, SkillEvolutionCaseStatusOpen; got != want {
		t.Fatalf("first status = %q, want %q", got, want)
	}

	second, created, err := controller.EnsureSkillEvolutionCase(ctx, SkillEvolutionCaseSpec{
		SkillID:           "browser",
		OwnerUserID:       "user-1",
		Mode:              SkillEvolutionModeFix,
		Reason:            SkillEvolutionReasonRuntimeFailure,
		SourceKind:        "runtime_task",
		SourceID:          "task-2",
		BaseContentSHA256: "base-browser-sha",
		FailureSignature:  "missing-title",
		Summary:           "A second task hit the same browser failure.",
		EvidenceJSON:      `{"task_id":"task-2"}`,
	})
	if err != nil {
		t.Fatalf("EnsureSkillEvolutionCase(second) failed: %v", err)
	}
	if created {
		t.Fatal("expected duplicate EnsureSkillEvolutionCase call to reuse the existing case")
	}
	if got, want := second.ID, first.ID; got != want {
		t.Fatalf("reused case id = %q, want %q", got, want)
	}
}
