package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestSQLiteStore_ListSkillDecisionHistoryFiltersAndOrdersByDecisionTime(t *testing.T) {
	ctx := context.Background()
	controller := newMinimalTestController(t)
	now := time.Now().UTC()

	noDecision, err := controller.CreateSkillRevision(ctx, SkillRevision{
		SkillID:     "browser",
		Status:      SkillRevisionStatusAccepted,
		SourcePath:  "assets/skills/browser/SKILL.md",
		CandidateID: "candidate-browser-no-decision",
		Content:     "# Browser\nNo decision metadata.\n",
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision(noDecision) failed: %v", err)
	}

	promoteReviewedAt := now.Add(2 * time.Minute)
	promote, err := controller.CreateSkillRevision(ctx, SkillRevision{
		SkillID:        "browser",
		Status:         SkillRevisionStatusPromoted,
		SourcePath:     "assets/skills/browser/SKILL.md",
		CandidateID:    "candidate-browser-promote",
		DecisionAction: SkillRevisionDecisionActionPromote,
		ReviewNote:     "Promoted after manual review",
		ReviewedBy:     "user-1",
		ReviewedAt:     skillDecisionHistoryTimePtr(promoteReviewedAt),
		DecisionLogJSON: `{
			"action":"promote",
			"target_revision_id":"rev-promote"
		}`,
		Content:   "# Browser\nPromoted revision.\n",
		CreatedAt: now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision(promote) failed: %v", err)
	}

	rollbackReviewedAt := now.Add(3 * time.Minute)
	rollback, err := controller.CreateSkillRevision(ctx, SkillRevision{
		SkillID:            "browser",
		Status:             SkillRevisionStatusPromoted,
		SourcePath:         "assets/skills/browser/SKILL.md",
		CandidateID:        "candidate-browser-rollback",
		BackupOfRevisionID: promote.ID,
		DecisionAction:     SkillRevisionDecisionActionRollback,
		ReviewNote:         "Rolled back after regression review",
		ReviewedBy:         "user-2",
		ReviewedAt:         skillDecisionHistoryTimePtr(rollbackReviewedAt),
		DecisionLogJSON: `{
			"action":"rollback",
			"source_revision_id":"rev-backup"
		}`,
		Content:   "# Browser\nRollback revision.\n",
		CreatedAt: now.Add(90 * time.Second),
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision(rollback) failed: %v", err)
	}

	_, err = controller.CreateSkillRevision(ctx, SkillRevision{
		SkillID:        "reminder",
		Status:         SkillRevisionStatusPromoted,
		SourcePath:     "assets/skills/reminder/SKILL.md",
		CandidateID:    "candidate-reminder-promote",
		DecisionAction: SkillRevisionDecisionActionPromote,
		ReviewNote:     "Other skill decision",
		ReviewedBy:     "user-3",
		ReviewedAt:     skillDecisionHistoryTimePtr(now.Add(4 * time.Minute)),
		DecisionLogJSON: `{
			"action":"promote",
			"target_revision_id":"rev-reminder"
		}`,
		Content:   "# Reminder\nPromoted revision.\n",
		CreatedAt: now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision(reminder) failed: %v", err)
	}

	history, err := controller.store.ListSkillDecisionHistory(ctx, SkillDecisionHistoryFilter{
		SkillID: "browser",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("ListSkillDecisionHistory failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("ListSkillDecisionHistory len = %d, want 2", len(history))
	}
	if got, want := history[0].RevisionID, rollback.ID; got != want {
		t.Fatalf("history[0].RevisionID = %q, want %q", got, want)
	}
	if got, want := history[0].DecisionAction, SkillRevisionDecisionActionRollback; got != want {
		t.Fatalf("history[0].DecisionAction = %q, want %q", got, want)
	}
	if got, want := history[0].DecisionAt, rollbackReviewedAt; !got.Equal(want) {
		t.Fatalf("history[0].DecisionAt = %v, want %v", got, want)
	}
	if history[0].DecisionLog["action"] != "rollback" {
		t.Fatalf("history[0].DecisionLog = %#v, want rollback action", history[0].DecisionLog)
	}
	if got, want := history[1].RevisionID, promote.ID; got != want {
		t.Fatalf("history[1].RevisionID = %q, want %q", got, want)
	}
	if got, want := history[1].DecisionAction, SkillRevisionDecisionActionPromote; got != want {
		t.Fatalf("history[1].DecisionAction = %q, want %q", got, want)
	}
	if got, want := history[1].DecisionAt, promoteReviewedAt; !got.Equal(want) {
		t.Fatalf("history[1].DecisionAt = %v, want %v", got, want)
	}
	for _, entry := range history {
		if entry.RevisionID == noDecision.ID {
			t.Fatalf("unexpected no-decision revision in history: %#v", entry)
		}
		if entry.SkillID != "browser" {
			t.Fatalf("history entry skill_id = %q, want browser", entry.SkillID)
		}
	}
}

func TestHandler_ListSkillDecisionHistoryReturnsSkillScopedResults(t *testing.T) {
	controller := newMinimalTestController(t)
	_, err := controller.CreateSkillRevision(context.Background(), SkillRevision{
		SkillID:        "browser",
		Status:         SkillRevisionStatusPromoted,
		SourcePath:     "assets/skills/browser/SKILL.md",
		CandidateID:    "candidate-browser-promote",
		DecisionAction: SkillRevisionDecisionActionPromote,
		ReviewNote:     "Promoted browser skill",
		ReviewedBy:     "user-1",
		ReviewedAt:     skillDecisionHistoryTimePtr(time.Now().UTC()),
		DecisionLogJSON: `{
			"action":"promote",
			"target_revision_id":"rev-browser"
		}`,
		Content:   "# Browser\nPromoted.\n",
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision(browser decision) failed: %v", err)
	}
	_, err = controller.CreateSkillRevision(context.Background(), SkillRevision{
		SkillID:     "browser",
		Status:      SkillRevisionStatusAccepted,
		SourcePath:  "assets/skills/browser/SKILL.md",
		CandidateID: "candidate-browser-candidate",
		Content:     "# Browser\nNo decision.\n",
		CreatedAt:   time.Now().UTC().Add(time.Second),
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision(browser no decision) failed: %v", err)
	}
	_, err = controller.CreateSkillRevision(context.Background(), SkillRevision{
		SkillID:        "reminder",
		Status:         SkillRevisionStatusPromoted,
		SourcePath:     "assets/skills/reminder/SKILL.md",
		CandidateID:    "candidate-reminder-promote",
		DecisionAction: SkillRevisionDecisionActionRollback,
		ReviewNote:     "Rollback reminder skill",
		ReviewedBy:     "user-2",
		ReviewedAt:     skillDecisionHistoryTimePtr(time.Now().UTC().Add(2 * time.Second)),
		DecisionLogJSON: `{
			"action":"rollback",
			"source_revision_id":"rev-reminder"
		}`,
		Content:   "# Reminder\nRollback.\n",
		CreatedAt: time.Now().UTC().Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision(reminder decision) failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/skills/browser/decision-history", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("skill_id")
	c.SetParamValues("browser")

	if err := handler.ListSkillDecisionHistory(c); err != nil {
		t.Fatalf("ListSkillDecisionHistory returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var history []SkillDecisionHistoryEntry
	if err := json.Unmarshal(rec.Body.Bytes(), &history); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history = %#v, want one browser decision entry", history)
	}
	if history[0].SkillID != "browser" || history[0].DecisionAction != SkillRevisionDecisionActionPromote {
		t.Fatalf("history[0] = %#v, want browser promote entry", history[0])
	}
	if history[0].DecisionLog["action"] != "promote" {
		t.Fatalf("history[0].DecisionLog = %#v, want promote action", history[0].DecisionLog)
	}
}

func skillDecisionHistoryTimePtr(ts time.Time) *time.Time {
	return &ts
}
