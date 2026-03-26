package agentsessions

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

type stubProtocolRuntime struct {
	verifyResult  *ProfileVerifyResult
	verifyErr     error
	healthResult  *ProfileHealthResult
	healthErr     error
	ensureResult  *EnsureSessionResult
	ensureErr     error
	submitResult  *SubmitRunResult
	submitErr     error
	streamEvents  []RuntimeEvent
	streamErr     error
	streamWaitErr error
	cancelErr     error
	closeErr      error

	ensureCalls []EnsureSessionRequest
	submitCalls []SubmitRunRequest
	cancelCalls []struct {
		Session ExternalSession
		Run     ExternalRun
	}
	closeCalls []struct {
		Profile AgentProfile
		Session ExternalSession
	}
}

func (r *stubProtocolRuntime) VerifyProfile(_ context.Context, profile AgentProfile) (*ProfileVerifyResult, error) {
	if r.verifyErr != nil {
		return nil, r.verifyErr
	}
	if r.verifyResult != nil {
		return r.verifyResult, nil
	}
	return &ProfileVerifyResult{OK: true, Message: profile.Name + " verified"}, nil
}

func (r *stubProtocolRuntime) EnsureSession(_ context.Context, req EnsureSessionRequest) (*EnsureSessionResult, error) {
	r.ensureCalls = append(r.ensureCalls, req)
	if r.ensureErr != nil {
		return nil, r.ensureErr
	}
	if r.ensureResult != nil {
		return r.ensureResult, nil
	}
	return &EnsureSessionResult{}, nil
}

func (r *stubProtocolRuntime) SubmitRun(_ context.Context, req SubmitRunRequest) (*SubmitRunResult, error) {
	r.submitCalls = append(r.submitCalls, req)
	if r.submitErr != nil {
		return nil, r.submitErr
	}
	if r.submitResult != nil {
		return r.submitResult, nil
	}
	return &SubmitRunResult{}, nil
}

func (r *stubProtocolRuntime) StreamRun(_ context.Context, _ StreamRunRequest) (RuntimeStream, error) {
	if r.streamErr != nil {
		return nil, r.streamErr
	}
	stream := newRuntimeStream(len(r.streamEvents) + 1)
	go func() {
		for _, event := range r.streamEvents {
			stream.push(event)
		}
		stream.finish(r.streamWaitErr)
	}()
	return stream, nil
}

func (r *stubProtocolRuntime) CancelRun(_ context.Context, session ExternalSession, run ExternalRun) error {
	r.cancelCalls = append(r.cancelCalls, struct {
		Session ExternalSession
		Run     ExternalRun
	}{Session: session, Run: run})
	return r.cancelErr
}

func (r *stubProtocolRuntime) CloseSession(_ context.Context, profile AgentProfile, session ExternalSession) error {
	r.closeCalls = append(r.closeCalls, struct {
		Profile AgentProfile
		Session ExternalSession
	}{Profile: profile, Session: session})
	return r.closeErr
}

func (r *stubProtocolRuntime) Health(_ context.Context, profile AgentProfile) (*ProfileHealthResult, error) {
	if r.healthErr != nil {
		return nil, r.healthErr
	}
	if r.healthResult != nil {
		return r.healthResult, nil
	}
	return &ProfileHealthResult{Healthy: true, Message: profile.Name + " healthy"}, nil
}

func newTestAgentSessionService(t *testing.T, runtimes map[ProtocolKind]ProtocolRuntime) (*Service, *SQLiteStore) {
	t.Helper()

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "agent-sessions.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	if runtimes == nil {
		runtimes = map[ProtocolKind]ProtocolRuntime{
			ProtocolACP: &stubProtocolRuntime{},
			ProtocolA2A: &stubProtocolRuntime{},
		}
	}
	service, err := NewService(store, zap.NewNop(), runtimes)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service, store
}

func TestServiceProcessRunCompletesAndPersistsHistory(t *testing.T) {
	acpRuntime := &stubProtocolRuntime{
		ensureResult: &EnsureSessionResult{
			RemoteSessionID: "remote-session-1",
			Metadata: map[string]interface{}{
				"transport": "stub-acp",
			},
		},
		submitResult: &SubmitRunResult{RemoteRunID: "remote-run-1"},
		streamEvents: []RuntimeEvent{
			{Type: "status", Status: "running", Text: "running"},
			{Type: "assistant_delta", Role: "assistant", Text: "hello "},
			{Type: "assistant_delta", Role: "assistant", Text: "world"},
		},
	}
	service, store := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: acpRuntime,
		ProtocolA2A: &stubProtocolRuntime{},
	})

	detail, err := service.CreateSession(context.Background(), CreateSessionParams{
		ProfileID: "codex",
		Name:      "Codex ACP Session",
		UserID:    "user-a",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	run := &ExternalRun{
		SessionID: detail.Session.ID,
		Status:    RunStatusQueued,
		Prompt:    "say hello",
	}
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
	if _, err := store.AppendEvent(detail.Session.ID, run.ID, "user_message", map[string]interface{}{
		"role":    "user",
		"content": run.Prompt,
		"text":    run.Prompt,
	}); err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}

	if err := service.processRun(context.Background(), detail.Session.ID, run.ID); err != nil {
		t.Fatalf("processRun() error = %v", err)
	}

	if len(acpRuntime.ensureCalls) != 1 {
		t.Fatalf("ensureCalls = %d, want 1", len(acpRuntime.ensureCalls))
	}
	if len(acpRuntime.submitCalls) != 1 {
		t.Fatalf("submitCalls = %d, want 1", len(acpRuntime.submitCalls))
	}
	if acpRuntime.submitCalls[0].Prompt != "say hello" {
		t.Fatalf("submit prompt = %q, want %q", acpRuntime.submitCalls[0].Prompt, "say hello")
	}

	session, err := store.GetSession(detail.Session.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != SessionStatusIdle {
		t.Fatalf("session status = %s, want %s", session.Status, SessionStatusIdle)
	}
	if session.RemoteSessionID != "remote-session-1" {
		t.Fatalf("remote session id = %q, want %q", session.RemoteSessionID, "remote-session-1")
	}
	if got := session.Metadata["transport"]; got != "stub-acp" {
		t.Fatalf("session metadata transport = %v, want %q", got, "stub-acp")
	}

	storedRun, err := store.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if storedRun.Status != RunStatusCompleted {
		t.Fatalf("run status = %s, want %s", storedRun.Status, RunStatusCompleted)
	}
	if storedRun.RemoteRunID != "remote-run-1" {
		t.Fatalf("remote run id = %q, want %q", storedRun.RemoteRunID, "remote-run-1")
	}

	history, err := service.GetSessionHistory(detail.Session.ID, 20)
	if err != nil {
		t.Fatalf("GetSessionHistory() error = %v", err)
	}
	var assistantMessage string
	var sawCompleted bool
	for _, item := range history {
		if item.Type == "assistant_message" {
			assistantMessage = item.Content
		}
		if item.Type == "run_completed" {
			sawCompleted = true
		}
	}
	if assistantMessage != "hello world" {
		t.Fatalf("assistant message = %q, want %q", assistantMessage, "hello world")
	}
	if !sawCompleted {
		t.Fatal("expected run_completed event in history")
	}
}

func TestServiceCancelSessionMarksCancellingAndDelegates(t *testing.T) {
	acpRuntime := &stubProtocolRuntime{}
	service, store := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: acpRuntime,
		ProtocolA2A: &stubProtocolRuntime{},
	})

	detail, err := service.CreateSession(context.Background(), CreateSessionParams{
		ProfileID: "claude",
		Name:      "Claude ACP Session",
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	session, err := store.GetSession(detail.Session.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	session.Status = SessionStatusRunning
	if err := store.SaveSession(session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	run := &ExternalRun{
		SessionID:   detail.Session.ID,
		Status:      RunStatusRunning,
		Prompt:      "build this",
		RemoteRunID: "remote-run-cancel",
	}
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	if err := service.CancelSession(context.Background(), detail.Session.ID); err != nil {
		t.Fatalf("CancelSession() error = %v", err)
	}

	session, err = store.GetSession(detail.Session.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != SessionStatusCancelling {
		t.Fatalf("session status = %s, want %s", session.Status, SessionStatusCancelling)
	}
	if len(acpRuntime.cancelCalls) != 1 {
		t.Fatalf("cancelCalls = %d, want 1", len(acpRuntime.cancelCalls))
	}
	if acpRuntime.cancelCalls[0].Run.RemoteRunID != "remote-run-cancel" {
		t.Fatalf("cancel run id = %q, want %q", acpRuntime.cancelCalls[0].Run.RemoteRunID, "remote-run-cancel")
	}

	events, err := service.ListEvents(detail.Session.ID, 20, 0)
	if err != nil {
		t.Fatalf("ListEvents() error = %v", err)
	}
	found := false
	for _, event := range events {
		if event.Type == "cancel_requested" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected cancel_requested event to be recorded")
	}
}
