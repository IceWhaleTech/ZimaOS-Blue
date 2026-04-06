package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newAskUserQuestionRoutesTestHarness(t *testing.T) (*echo.Echo, *tools.QuestionManager, *sse.Broker, *auth.JWTService) {
	t.Helper()

	broker := sse.NewBroker()
	t.Cleanup(broker.Close)
	questionMgr := tools.NewQuestionManager(broker, nil, time.Minute)

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	authMiddleware := auth.NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	v1 := e.Group("/api/v1")
	registerAskUserQuestionRoutes(
		v1,
		authMiddleware.Authenticate(),
		permission.RequirePagePermission(nil, permission.PageChat),
		questionMgr,
	)

	return e, questionMgr, broker, jwtSvc
}

func mustAskUserQuestionAccessToken(t *testing.T, jwtSvc *auth.JWTService, userID string) string {
	t.Helper()

	token, err := jwtSvc.GenerateAccessToken(&auth.UserClaims{
		UserID:   userID,
		Username: userID,
		Role:     "user",
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	return token
}

func TestAskUserQuestionRoutesResolveAnswersAndDoNotExposeStandalonePendingRoute(t *testing.T) {
	e, questionMgr, broker, jwtSvc := newAskUserQuestionRoutesTestHarness(t)
	sub := broker.Subscribe("user-a")
	t.Cleanup(func() { broker.Unsubscribe("user-a", sub) })

	answerCh := make(chan []tools.QuestionAnswerResult, 1)
	okCh := make(chan bool, 1)
	errCh := make(chan error, 1)
	go func() {
		answers, ok, err := questionMgr.AskQuestions(context.Background(), "user-a", "session-1", []tools.QuestionItem{
			{
				ID:       "q1",
				Header:   "Confirm",
				Question: "Proceed?",
				Options: []tools.QuestionOption{
					{Label: "Continue", Value: "continue"},
					{Label: "Stop", Value: "stop"},
				},
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		answerCh <- answers
		okCh <- ok
	}()

	var pending *tools.QuestionRequest
	deadline := time.Now().Add(2 * time.Second)
	for pending == nil && time.Now().Before(deadline) {
		pending = questionMgr.GetPending("user-a")
		if pending == nil {
			time.Sleep(10 * time.Millisecond)
		}
	}
	if pending == nil {
		t.Fatal("expected pending question request")
	}

	reqPending := httptest.NewRequest(http.MethodGet, "/api/v1/ask-user-question/pending", nil)
	reqPending.Header.Set(echo.HeaderAuthorization, "Bearer "+mustAskUserQuestionAccessToken(t, jwtSvc, "user-a"))
	recPending := httptest.NewRecorder()
	e.ServeHTTP(recPending, reqPending)
	if recPending.Code != http.StatusNotFound {
		t.Fatalf("pending status=%d, want 404, body=%s", recPending.Code, recPending.Body.String())
	}

	reqAnswer := httptest.NewRequest(http.MethodPost, "/api/v1/ask-user-question/"+pending.ID+"/answer", strings.NewReader(`{"answers":[{"question_id":"q1","selected":["continue"]}]}`))
	reqAnswer.Header.Set(echo.HeaderAuthorization, "Bearer "+mustAskUserQuestionAccessToken(t, jwtSvc, "user-a"))
	reqAnswer.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	recAnswer := httptest.NewRecorder()
	e.ServeHTTP(recAnswer, reqAnswer)
	if recAnswer.Code != http.StatusOK {
		t.Fatalf("answer status=%d, want 200, body=%s", recAnswer.Code, recAnswer.Body.String())
	}

	select {
	case err := <-errCh:
		t.Fatalf("AskQuestions returned error: %v", err)
	case answers := <-answerCh:
		if len(answers) != 1 || answers[0].QuestionID != "q1" || len(answers[0].Selected) != 1 || answers[0].Selected[0] != "continue" {
			t.Fatalf("unexpected answers: %+v", answers)
		}
		if ok := <-okCh; ok {
			t.Fatalf("expected answered question flow to return silent=false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for answer resolution")
	}
}

func TestAskUserQuestionRoutesKeepSessionScopedPendingAndDoNotExposeStandalonePendingRoute(t *testing.T) {
	e, questionMgr, broker, jwtSvc := newAskUserQuestionRoutesTestHarness(t)
	sub := broker.Subscribe("user-a")
	t.Cleanup(func() { broker.Unsubscribe("user-a", sub) })

	done1 := make(chan struct{})
	done2 := make(chan struct{})
	go func() {
		defer close(done1)
		_, _, _ = questionMgr.AskQuestions(context.Background(), "user-a", "session-1", []tools.QuestionItem{
			{
				ID:       "q1",
				Header:   "First",
				Question: "First session?",
				Options:  []tools.QuestionOption{{Label: "Continue", Value: "continue"}},
			},
		})
	}()

	time.Sleep(20 * time.Millisecond)

	go func() {
		defer close(done2)
		_, _, _ = questionMgr.AskQuestions(context.Background(), "user-a", "session-2", []tools.QuestionItem{
			{
				ID:       "q2",
				Header:   "Second",
				Question: "Second session?",
				Options:  []tools.QuestionOption{{Label: "Continue", Value: "continue"}},
			},
		})
	}()

	var pending1 *tools.QuestionRequest
	var pending2 *tools.QuestionRequest
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		pending1 = questionMgr.GetPendingBySession("session-1")
		pending2 = questionMgr.GetPendingBySession("session-2")
		if pending1 != nil && pending2 != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if pending1 == nil || pending2 == nil {
		t.Fatalf("expected both session-scoped pending questions, got session-1=%+v session-2=%+v", pending1, pending2)
	}

	reqPending := httptest.NewRequest(http.MethodGet, "/api/v1/ask-user-question/pending?session_id=session-1", nil)
	reqPending.Header.Set(echo.HeaderAuthorization, "Bearer "+mustAskUserQuestionAccessToken(t, jwtSvc, "user-a"))
	recPending := httptest.NewRecorder()
	e.ServeHTTP(recPending, reqPending)
	if recPending.Code != http.StatusNotFound {
		t.Fatalf("pending status=%d, want 404, body=%s", recPending.Code, recPending.Body.String())
	}

	if !questionMgr.ResolveAnswer(pending1.ID, []tools.QuestionAnswerResult{{QuestionID: "q1", Selected: []string{"continue"}}}) {
		t.Fatalf("failed to resolve pending question %q", pending1.ID)
	}
	if !questionMgr.ResolveAnswer(pending2.ID, []tools.QuestionAnswerResult{{QuestionID: "q2", Selected: []string{"continue"}}}) {
		t.Fatalf("failed to resolve pending question %q", pending2.ID)
	}

	select {
	case <-done1:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session-1 question request to exit")
	}
	select {
	case <-done2:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session-2 question request to exit")
	}
}
