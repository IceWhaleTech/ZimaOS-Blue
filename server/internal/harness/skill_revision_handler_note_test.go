package harness

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func TestHandler_PromoteSkillRevisionPersistsReviewNoteAndReviewer(t *testing.T) {
	controller := newTestController(t)
	repoRoot := t.TempDir()
	skillDir := filepath.Join(repoRoot, "assets", "skills", "browser")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll skillDir: %v", err)
	}
	canonicalPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(canonicalPath, []byte("# Browser\nOriginal.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile canonicalPath: %v", err)
	}
	restoreWD := chdirForSkillRevisionTest(t, repoRoot)
	defer restoreWD()

	revision, err := controller.CreateSkillRevision(context.Background(), SkillRevision{
		SkillID:     "browser",
		Status:      SkillRevisionStatusAccepted,
		SourcePath:  "assets/skills/browser/SKILL.md",
		CandidateID: "candidate-browser-promote",
		Content:     "# Browser\nPromoted.\n",
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateSkillRevision failed: %v", err)
	}

	handler := NewHandler(controller)
	e := echo.New()
	req := httptest.NewRequest(
		http.MethodPost,
		"/skill-revisions/"+revision.ID+"/promote",
		strings.NewReader(`{"review_note":"Promote after explicit operator sign-off."}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(revision.ID)
	c.SetRequest(c.Request().WithContext(context.WithValue(c.Request().Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "user-promote-handler",
	})))

	if err := handler.PromoteSkillRevision(c); err != nil {
		t.Fatalf("PromoteSkillRevision returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result SkillPromoteResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got, want := result.PromotedRevisionID, revision.ID; got != want {
		t.Fatalf("PromotedRevisionID = %q, want %q", got, want)
	}

	promoted, err := controller.store.GetSkillRevision(context.Background(), revision.ID)
	if err != nil {
		t.Fatalf("GetSkillRevision(promoted) failed: %v", err)
	}
	if got, want := promoted.ReviewNote, "Promote after explicit operator sign-off."; got != want {
		t.Fatalf("promoted review_note = %q, want %q", got, want)
	}
	if got, want := promoted.ReviewedBy, "user-promote-handler"; got != want {
		t.Fatalf("promoted reviewed_by = %q, want %q", got, want)
	}
	if got, want := promoted.DecisionAction, SkillRevisionDecisionActionPromote; got != want {
		t.Fatalf("promoted decision_action = %q, want %q", got, want)
	}
	if promoted.ReviewedAt == nil {
		t.Fatal("expected promoted revision to record reviewed_at")
	}
}
