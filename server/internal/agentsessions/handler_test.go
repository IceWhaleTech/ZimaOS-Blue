package agentsessions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func TestHandlerListSessionsFiltersByRuntime(t *testing.T) {
	service, store := newTestAgentSessionService(t, nil)
	saveRunnableACPProfile(t, store, "custom-codex")

	if _, err := service.CreateSession(t.Context(), CreateSessionParams{
		ProfileID: "custom-codex",
		Name:      "Codex Session",
		UserID:    "user-a",
	}); err != nil {
		t.Fatalf("CreateSession(custom-codex) error = %v", err)
	}
	if _, err := service.CreateSession(t.Context(), CreateSessionParams{
		ProfileID: "generic-a2a",
		Name:      "A2A Session",
		UserID:    "user-a",
	}); err != nil {
		t.Fatalf("CreateSession(generic-a2a) error = %v", err)
	}

	e := echo.New()
	NewHandler(service).RegisterSessionRoutes(e.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent-sessions/sessions?runtime=a2a", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Sessions []ExternalSession `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(payload.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(payload.Sessions))
	}
	if payload.Sessions[0].Protocol != ProtocolA2A {
		t.Fatalf("session protocol = %s, want %s", payload.Sessions[0].Protocol, ProtocolA2A)
	}
}

func TestHandlerListSessionsRejectsInvalidRuntimeFilter(t *testing.T) {
	service, _ := newTestAgentSessionService(t, nil)

	e := echo.New()
	NewHandler(service).RegisterSessionRoutes(e.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent-sessions/sessions?runtime=bogus", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandlerVerifyProfileReturnsMessageCode(t *testing.T) {
	service, _ := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{
			verifyResult: &ProfileVerifyResult{OK: true, Message: "builtin verified"},
		},
		ProtocolA2A: &stubProtocolRuntime{},
	})

	e := echo.New()
	NewHandler(service).RegisterProfileRoutes(e.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-sessions/profiles/verify", bytes.NewBufferString(`{"id":"codex"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload ProfileVerifyResult
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.MessageCode != ProfileVerifyMessageCodeProfileVerified {
		t.Fatalf("message_code = %q, want %q", payload.MessageCode, ProfileVerifyMessageCodeProfileVerified)
	}
}

func TestHandlerHealthProfileReturnsMessageCode(t *testing.T) {
	service, _ := newTestAgentSessionService(t, map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: &stubProtocolRuntime{
			healthResult: &ProfileHealthResult{Healthy: true, Message: "builtin healthy"},
		},
		ProtocolA2A: &stubProtocolRuntime{},
	})

	e := echo.New()
	NewHandler(service).RegisterProfileRoutes(e.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-sessions/profiles/codex/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload ProfileHealthResult
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.MessageCode != ProfileHealthMessageCodeRuntimeHealthy {
		t.Fatalf("message_code = %q, want %q", payload.MessageCode, ProfileHealthMessageCodeRuntimeHealthy)
	}
}

func TestHandlerVerifyProfileHTTPUsesBinaryFirstTemplateAndFallsBackToNpx(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "codex-npx-http.log")
	writeACPInitializeStub(t, filepath.Join(tempDir, "npx"), logPath)
	t.Setenv("PATH", tempDir)
	t.Setenv("HOME", tempDir)

	store := newTestAgentSessionStore(t)
	service, err := NewService(store, zap.NewNop(), map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: NewACPRuntime(nil, zap.NewNop(), nil),
		ProtocolA2A: &stubProtocolRuntime{},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	e := echo.New()
	NewHandler(service).RegisterProfileRoutes(e.Group("/api/v1"))

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/agent-sessions/profiles", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list profiles status = %d, want %d", listRec.Code, http.StatusOK)
	}
	var listPayload struct {
		Profiles []AgentProfile `json:"profiles"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("json.Unmarshal(list profiles) error = %v", err)
	}
	foundCodex := false
	for _, profile := range listPayload.Profiles {
		if profile.ID != "codex" {
			continue
		}
		foundCodex = true
		if got := strings.Join(profile.Command, "\n"); got != "codex-acp" {
			t.Fatalf("codex builtin command = %q, want %q", got, "codex-acp")
		}
	}
	if !foundCodex {
		t.Fatal("expected builtin codex profile in list response")
	}

	verifyReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent-sessions/profiles/verify", bytes.NewBufferString(`{
		"profile": {
			"protocol": "acp",
			"name": "codex-http-e2e",
			"title": "Codex HTTP E2E",
			"command": ["codex-acp"]
		}
	}`))
	verifyReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	verifyRec := httptest.NewRecorder()
	e.ServeHTTP(verifyRec, verifyReq)

	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify status = %d, want %d, body=%s", verifyRec.Code, http.StatusOK, verifyRec.Body.String())
	}
	var verifyPayload ProfileVerifyResult
	if err := json.Unmarshal(verifyRec.Body.Bytes(), &verifyPayload); err != nil {
		t.Fatalf("json.Unmarshal(verify) error = %v", err)
	}
	if !verifyPayload.OK {
		t.Fatalf("verify ok = false, want true: %+v", verifyPayload)
	}
	if verifyPayload.MessageCode != ProfileVerifyMessageCodeProfileVerified {
		t.Fatalf("verify message_code = %q, want %q", verifyPayload.MessageCode, ProfileVerifyMessageCodeProfileVerified)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", logPath, err)
	}
	if got := strings.TrimSpace(string(data)); got != "@zed-industries/codex-acp" {
		t.Fatalf("npx args = %q, want %q", got, "@zed-industries/codex-acp")
	}
}

func TestHandlerSessionMessageFlowHTTPUsesBinaryFirstCommandAndFallback(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "codex-npx-session.log")
	replyText := "hello from codex fallback"
	writeACPConversationStub(t, filepath.Join(tempDir, "npx"), logPath, replyText)
	t.Setenv("PATH", tempDir)
	t.Setenv("HOME", tempDir)

	store := newTestAgentSessionStore(t)
	service, err := NewService(store, zap.NewNop(), map[ProtocolKind]ProtocolRuntime{
		ProtocolACP: NewACPRuntime(nil, zap.NewNop(), nil),
		ProtocolA2A: &stubProtocolRuntime{},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	e := echo.New()
	handler := NewHandler(service)
	api := e.Group("/api/v1")
	handler.RegisterProfileRoutes(api)
	handler.RegisterSessionRoutes(api)

	saveReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent-sessions/profiles", bytes.NewBufferString(`{
		"protocol": "acp",
		"name": "codex-http-session",
		"title": "Codex HTTP Session",
		"command": ["codex-acp"]
	}`))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save profile status = %d, want %d, body=%s", saveRec.Code, http.StatusOK, saveRec.Body.String())
	}
	var savedProfile AgentProfile
	if err := json.Unmarshal(saveRec.Body.Bytes(), &savedProfile); err != nil {
		t.Fatalf("json.Unmarshal(save profile) error = %v", err)
	}
	if strings.TrimSpace(savedProfile.ID) == "" {
		t.Fatal("saved profile id = empty, want generated id")
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent-sessions/sessions", bytes.NewBufferString(fmt.Sprintf(`{
		"profile_id": %q,
		"name": "Codex Session HTTP"
	}`, savedProfile.ID)))
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create session status = %d, want %d, body=%s", createRec.Code, http.StatusOK, createRec.Body.String())
	}
	var created SessionDetail
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create session) error = %v", err)
	}
	if created.Session.Status != SessionStatusIdle {
		t.Fatalf("created session status = %s, want %s", created.Session.Status, SessionStatusIdle)
	}

	sendReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent-sessions/sessions/"+created.Session.ID+"/messages", bytes.NewBufferString(`{
		"message": "say hello"
	}`))
	sendReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	sendRec := httptest.NewRecorder()
	e.ServeHTTP(sendRec, sendReq)
	if sendRec.Code != http.StatusOK {
		t.Fatalf("send message status = %d, want %d, body=%s", sendRec.Code, http.StatusOK, sendRec.Body.String())
	}
	var sentRun ExternalRun
	if err := json.Unmarshal(sendRec.Body.Bytes(), &sentRun); err != nil {
		t.Fatalf("json.Unmarshal(send message) error = %v", err)
	}
	if sentRun.Status != RunStatusQueued {
		t.Fatalf("send run status = %s, want %s", sentRun.Status, RunStatusQueued)
	}

	deadline := time.Now().Add(15 * time.Second)
	for {
		historyReq := httptest.NewRequest(http.MethodGet, "/api/v1/agent-sessions/sessions/"+created.Session.ID+"/history?limit=20", nil)
		historyRec := httptest.NewRecorder()
		e.ServeHTTP(historyRec, historyReq)
		if historyRec.Code != http.StatusOK {
			t.Fatalf("history status = %d, want %d, body=%s", historyRec.Code, http.StatusOK, historyRec.Body.String())
		}
		var payload struct {
			History []SessionHistoryItem `json:"history"`
		}
		if err := json.Unmarshal(historyRec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(history) error = %v", err)
		}
		var assistantMessage string
		var sawCompleted bool
		for _, item := range payload.History {
			if item.Type == "assistant_message" {
				assistantMessage = item.Content
			}
			if item.Type == "run_completed" {
				sawCompleted = true
			}
		}
		if assistantMessage == replyText && sawCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for assistant history, history=%+v", payload.History)
		}
		time.Sleep(25 * time.Millisecond)
	}

	closeReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent-sessions/sessions/"+created.Session.ID+"/close", nil)
	closeRec := httptest.NewRecorder()
	e.ServeHTTP(closeRec, closeReq)
	if closeRec.Code != http.StatusOK {
		t.Fatalf("close session status = %d, want %d, body=%s", closeRec.Code, http.StatusOK, closeRec.Body.String())
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", logPath, err)
	}
	if got := strings.TrimSpace(string(data)); got != "@zed-industries/codex-acp" {
		t.Fatalf("npx args = %q, want %q", got, "@zed-industries/codex-acp")
	}
}
