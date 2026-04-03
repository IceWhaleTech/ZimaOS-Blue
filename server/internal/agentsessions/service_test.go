package agentsessions

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
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

	store := newTestAgentSessionStore(t)
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

func newTestAgentSessionStore(t *testing.T) *SQLiteStore {
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
	return store
}

func saveTestProfile(t *testing.T, store *SQLiteStore, profile *AgentProfile) {
	t.Helper()
	if err := store.SaveProfile(profile); err != nil {
		t.Fatalf("SaveProfile(%s) error = %v", profile.ID, err)
	}
}

func saveRunnableACPProfile(t *testing.T, store *SQLiteStore, id string) {
	t.Helper()
	saveTestProfile(t, store, &AgentProfile{
		ID:                   id,
		Protocol:             ProtocolACP,
		Name:                 id,
		Title:                strings.ToUpper(id[:1]) + id[1:] + " Custom ACP",
		Description:          "Custom runnable ACP profile for tests.",
		Command:              []string{"/usr/local/bin/acp-runner", "--stdio"},
		CredentialProviderID: "openai-codex",
	})
}

func saveSessionForTest(t *testing.T, store *SQLiteStore, session *ExternalSession) {
	t.Helper()
	if err := store.SaveSession(session); err != nil {
		t.Fatalf("SaveSession(%s) error = %v", session.ID, err)
	}
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
	saveRunnableACPProfile(t, store, "custom-codex")

	detail, err := service.CreateSession(context.Background(), CreateSessionParams{
		ProfileID: "custom-codex",
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

func TestServiceMajorCLIProfilesRemainRunnableAsCustomACP(t *testing.T) {
	acpRuntime := &stubProtocolRuntime{
		ensureResult: &EnsureSessionResult{
			RemoteSessionID: "remote-session-major-cli",
		},
		submitResult: &SubmitRunResult{RemoteRunID: "remote-run-major-cli"},
		streamEvents: []RuntimeEvent{
			{Type: "assistant_delta", Role: "assistant", Text: "ok"},
		},
	}
	service, store := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: acpRuntime,
		ProtocolA2A: &stubProtocolRuntime{},
	})

	testCases := []struct {
		id                   string
		title                string
		command              []string
		credentialProviderID string
	}{
		{
			id:                   "claude-custom",
			title:                "Claude Custom ACP",
			command:              []string{"/usr/local/bin/claude"},
			credentialProviderID: "anthropic",
		},
		{
			id:                   "codex-custom",
			title:                "Codex Custom ACP",
			command:              []string{"/usr/local/bin/codex"},
			credentialProviderID: "openai-codex",
		},
		{
			id:                   "gemini-custom",
			title:                "Gemini Custom ACP",
			command:              []string{"/opt/bin/gemini", "--acp"},
			credentialProviderID: "google-gemini-cli",
		},
	}

	for _, tc := range testCases {
		saveTestProfile(t, store, &AgentProfile{
			ID:                   tc.id,
			Protocol:             ProtocolACP,
			Name:                 tc.id,
			Title:                tc.title,
			Description:          "Custom ACP profile for a major CLI runtime.",
			Command:              append([]string(nil), tc.command...),
			CredentialProviderID: tc.credentialProviderID,
		})

		if _, err := service.VerifyProfile(context.Background(), tc.id, nil); err != nil {
			t.Fatalf("VerifyProfile(%s) error = %v", tc.id, err)
		}

		detail, err := service.CreateSession(context.Background(), CreateSessionParams{
			ProfileID: tc.id,
			Name:      tc.title + " Session",
			UserID:    "user-major-cli",
		})
		if err != nil {
			t.Fatalf("CreateSession(%s) error = %v", tc.id, err)
		}

		run := &ExternalRun{
			SessionID: detail.Session.ID,
			Status:    RunStatusQueued,
			Prompt:    "hello from " + tc.id,
		}
		if err := store.SaveRun(run); err != nil {
			t.Fatalf("SaveRun(%s) error = %v", tc.id, err)
		}
		if _, err := store.AppendEvent(detail.Session.ID, run.ID, "user_message", map[string]interface{}{
			"role":    "user",
			"content": run.Prompt,
			"text":    run.Prompt,
		}); err != nil {
			t.Fatalf("AppendEvent(%s) error = %v", tc.id, err)
		}

		if err := service.processRun(context.Background(), detail.Session.ID, run.ID); err != nil {
			t.Fatalf("processRun(%s) error = %v", tc.id, err)
		}
	}

	if len(acpRuntime.ensureCalls) != len(testCases) {
		t.Fatalf("ensureCalls = %d, want %d", len(acpRuntime.ensureCalls), len(testCases))
	}
	if len(acpRuntime.submitCalls) != len(testCases) {
		t.Fatalf("submitCalls = %d, want %d", len(acpRuntime.submitCalls), len(testCases))
	}
}

func TestServiceCancelSessionMarksCancellingAndDelegates(t *testing.T) {
	acpRuntime := &stubProtocolRuntime{}
	service, store := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: acpRuntime,
		ProtocolA2A: &stubProtocolRuntime{},
	})
	saveRunnableACPProfile(t, store, "custom-claude")

	detail, err := service.CreateSession(context.Background(), CreateSessionParams{
		ProfileID: "custom-claude",
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

func TestSeedBuiltinProfilesNormalizesAcpTemplatesAndKeepsGenericA2ARunnable(t *testing.T) {
	service, store := newTestAgentSessionService(t, nil)
	if service == nil {
		t.Fatal("expected service to be initialized")
	}

	for _, id := range []string{"claude", "codex", "gemini"} {
		profile, err := store.GetProfile(id)
		if err != nil {
			t.Fatalf("GetProfile(%s) error = %v", id, err)
		}
		if !profile.Builtin {
			t.Fatalf("%s builtin = false, want true", id)
		}
		if !profile.TemplateOnly {
			t.Fatalf("%s template_only = false, want true", id)
		}
		if len(profile.Command) != 0 {
			t.Fatalf("%s command = %v, want empty", id, profile.Command)
		}
	}

	a2aProfile, err := store.GetProfile("generic-a2a")
	if err != nil {
		t.Fatalf("GetProfile(generic-a2a) error = %v", err)
	}
	if a2aProfile.Protocol != ProtocolA2A {
		t.Fatalf("generic-a2a protocol = %s, want %s", a2aProfile.Protocol, ProtocolA2A)
	}
	if a2aProfile.TemplateOnly {
		t.Fatal("generic-a2a template_only = true, want false")
	}
}

func TestNewServiceMigratesLegacyBuiltinAcpSessionsToCustomProfile(t *testing.T) {
	store := newTestAgentSessionStore(t)
	saveTestProfile(t, store, &AgentProfile{
		ID:                   "codex",
		Protocol:             ProtocolACP,
		Name:                 "codex",
		Title:                "Codex ACP",
		Description:          "Legacy runnable built-in ACP profile.",
		Builtin:              true,
		Command:              []string{"npx", "@zed-industries/codex-acp"},
		CredentialProviderID: "openai-codex",
		Metadata: map[string]interface{}{
			"reference": "openclaw/acpx",
		},
	})
	saveSessionForTest(t, store, &ExternalSession{
		ID:        "sess-legacy-codex",
		ProfileID: "codex",
		Protocol:  ProtocolACP,
		Name:      "Legacy Codex Session",
		Status:    SessionStatusIdle,
	})

	service, err := NewService(store, zap.NewNop(), map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if service == nil {
		t.Fatal("expected service to be initialized")
	}

	builtin, err := store.GetProfile("codex")
	if err != nil {
		t.Fatalf("GetProfile(codex) error = %v", err)
	}
	if !builtin.Builtin || !builtin.TemplateOnly {
		t.Fatalf("builtin codex = %+v, want builtin template-only", builtin)
	}
	if len(builtin.Command) != 0 {
		t.Fatalf("builtin codex command = %v, want empty", builtin.Command)
	}

	profiles, err := store.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles() error = %v", err)
	}
	var migrated *AgentProfile
	for i := range profiles {
		profile := profiles[i]
		if profile.Builtin || profile.Protocol != ProtocolACP {
			continue
		}
		if source, _ := profile.Metadata["migrated_from_builtin_profile_id"].(string); source == "codex" {
			migrated = &profile
			break
		}
	}
	if migrated == nil {
		t.Fatal("expected migrated custom ACP profile to be created")
	}
	if migrated.TemplateOnly {
		t.Fatal("migrated profile template_only = true, want false")
	}
	if len(migrated.Command) == 0 {
		t.Fatal("migrated profile command = empty, want preserved runnable command")
	}

	session, err := store.GetSession("sess-legacy-codex")
	if err != nil {
		t.Fatalf("GetSession(sess-legacy-codex) error = %v", err)
	}
	if session.ProfileID != migrated.ID {
		t.Fatalf("session profile_id = %q, want %q", session.ProfileID, migrated.ID)
	}
}

func TestServiceVerifyProfileRejectsTemplateOnlyACPProfile(t *testing.T) {
	service, _ := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})

	_, err := service.VerifyProfile(context.Background(), "codex", nil)
	if err == nil || !strings.Contains(err.Error(), "setup template") {
		t.Fatalf("VerifyProfile(codex) err = %v, want setup template error", err)
	}
}

func TestServiceHealthProfileRejectsTemplateOnlyACPProfile(t *testing.T) {
	service, _ := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})

	_, err := service.HealthProfile(context.Background(), "codex")
	if err == nil || !strings.Contains(err.Error(), "setup template") {
		t.Fatalf("HealthProfile(codex) err = %v, want setup template error", err)
	}
}

func TestServiceDeleteProfileDeletesMigratedCustomACPProfile(t *testing.T) {
	service, store := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})
	saveTestProfile(t, store, &AgentProfile{
		ID:          "codex-migrated",
		Protocol:    ProtocolACP,
		Name:        "codex-migrated",
		Title:       "Codex ACP (Migrated)",
		Description: "Legacy runnable ACP profile migrated from built-in Codex.",
		Command:     []string{"npx", "@zed-industries/codex-acp"},
		Metadata: map[string]interface{}{
			"migrated_from_builtin_profile_id": "codex",
		},
	})

	if err := service.DeleteProfile("codex-migrated"); err != nil {
		t.Fatalf("DeleteProfile(codex-migrated) error = %v", err)
	}
	if _, err := store.GetProfile("codex-migrated"); err != ErrProfileNotFound {
		t.Fatalf("GetProfile(codex-migrated) err = %v, want %v", err, ErrProfileNotFound)
	}
}

func TestServiceDeleteProfileRejectsBuiltinProfile(t *testing.T) {
	service, _ := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})

	err := service.DeleteProfile("codex")
	if err == nil || !strings.Contains(err.Error(), "built-in profiles cannot be deleted") {
		t.Fatalf("DeleteProfile(codex) err = %v, want built-in delete error", err)
	}
}

func TestServiceDeleteProfileRejectsProfileInUse(t *testing.T) {
	service, store := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})
	saveTestProfile(t, store, &AgentProfile{
		ID:          "codex-migrated-in-use",
		Protocol:    ProtocolACP,
		Name:        "codex-migrated-in-use",
		Title:       "Codex ACP (Migrated)",
		Description: "Legacy runnable ACP profile still referenced by a session.",
		Command:     []string{"npx", "@zed-industries/codex-acp"},
		Metadata: map[string]interface{}{
			"migrated_from_builtin_profile_id": "codex",
		},
	})
	saveSessionForTest(t, store, &ExternalSession{
		ID:        "sess-migrated-codex",
		ProfileID: "codex-migrated-in-use",
		Protocol:  ProtocolACP,
		Name:      "Migrated Codex Session",
		Status:    SessionStatusIdle,
	})

	err := service.DeleteProfile("codex-migrated-in-use")
	if err == nil || !strings.Contains(err.Error(), "still used by 1 session") {
		t.Fatalf("DeleteProfile(codex-migrated-in-use) err = %v, want in-use error", err)
	}
	if _, err := store.GetProfile("codex-migrated-in-use"); err != nil {
		t.Fatalf("GetProfile(codex-migrated-in-use) error = %v, want profile preserved", err)
	}
}

func TestServiceCreateSessionRejectsTemplateOnlyACPProfile(t *testing.T) {
	service, _ := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})

	_, err := service.CreateSession(context.Background(), CreateSessionParams{
		ProfileID: "codex",
		Name:      "Template Session",
	})
	if err == nil || !strings.Contains(err.Error(), "setup template") {
		t.Fatalf("CreateSession(codex) err = %v, want setup template error", err)
	}
}

func TestServiceSendMessageRejectsTemplateOnlyACPProfileSession(t *testing.T) {
	service, store := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})
	saveSessionForTest(t, store, &ExternalSession{
		ID:        "sess-template-codex",
		ProfileID: "codex",
		Protocol:  ProtocolACP,
		Name:      "Template Codex Session",
		Status:    SessionStatusIdle,
	})

	_, err := service.SendMessage(context.Background(), SendMessageParams{
		SessionID: "sess-template-codex",
		Message:   "hello",
	})
	if err == nil || !strings.Contains(err.Error(), "setup template") {
		t.Fatalf("SendMessage(template session) err = %v, want setup template error", err)
	}

	runs, err := store.ListRuns("sess-template-codex", 10)
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("runs len = %d, want 0", len(runs))
	}
}

func TestServiceProcessRunFailsTemplateOnlyACPProfileSession(t *testing.T) {
	service, store := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{},
		ProtocolA2A: &stubProtocolRuntime{},
	})
	saveSessionForTest(t, store, &ExternalSession{
		ID:        "sess-template-process",
		ProfileID: "codex",
		Protocol:  ProtocolACP,
		Name:      "Template Process Session",
		Status:    SessionStatusIdle,
	})
	run := &ExternalRun{
		ID:        "run-template-process",
		SessionID: "sess-template-process",
		Status:    RunStatusQueued,
		Prompt:    "hello",
	}
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	err := service.processRun(context.Background(), "sess-template-process", "run-template-process")
	if err == nil || !strings.Contains(err.Error(), "setup template") {
		t.Fatalf("processRun(template session) err = %v, want setup template error", err)
	}

	session, err := store.GetSession("sess-template-process")
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}
	if session.Status != SessionStatusError {
		t.Fatalf("session status = %s, want %s", session.Status, SessionStatusError)
	}
	storedRun, err := store.GetRun("run-template-process")
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if storedRun.Status != RunStatusFailed {
		t.Fatalf("run status = %s, want %s", storedRun.Status, RunStatusFailed)
	}
}
