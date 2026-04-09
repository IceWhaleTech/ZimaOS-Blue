package bootstrap

import (
	"bytes"
	"database/sql"
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
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeAgentSessionsBootstrapHTTPFlowUsesBinaryFirstFallback(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "bootstrap-codex-npx.log")
	writeBootstrapACPConversationStub(t, filepath.Join(tempDir, "npx"), logPath, "hello from bootstrap fallback")
	t.Setenv("PATH", tempDir)
	t.Setenv("HOME", tempDir)

	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "agent-sessions.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	service, err := newRuntimeAgentSessionsService(
		db,
		zap.NewNop(),
		[]string{tempDir},
		tools.DefaultExecConfig(),
		tools.NewApprovalManager(sse.NewBroker()),
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("newRuntimeAgentSessionsService() error = %v", err)
	}
	if service == nil {
		t.Fatal("expected runtime agent sessions service")
	}

	e := echo.New()
	bindRuntimeAgentSessions(nil, nil, service, e.Group("/profile"), e.Group("/chat"))

	listReq := httptest.NewRequest(http.MethodGet, "/profile/agent-sessions/profiles", nil)
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list profiles status = %d, want %d", listRec.Code, http.StatusOK)
	}
	var listPayload struct {
		Profiles []agentsessions.AgentProfile `json:"profiles"`
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
		t.Fatal("expected builtin codex profile from bootstrap-bound routes")
	}

	saveReq := httptest.NewRequest(http.MethodPost, "/profile/agent-sessions/profiles", bytes.NewBufferString(`{
		"protocol": "acp",
		"name": "bootstrap-codex",
		"title": "Bootstrap Codex",
		"command": ["codex-acp"]
	}`))
	saveReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	saveRec := httptest.NewRecorder()
	e.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save profile status = %d, want %d, body=%s", saveRec.Code, http.StatusOK, saveRec.Body.String())
	}
	var saved agentsessions.AgentProfile
	if err := json.Unmarshal(saveRec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("json.Unmarshal(save profile) error = %v", err)
	}
	if strings.TrimSpace(saved.ID) == "" {
		t.Fatal("saved profile id = empty, want generated id")
	}

	createReq := httptest.NewRequest(http.MethodPost, "/chat/agent-sessions/sessions", bytes.NewBufferString(fmt.Sprintf(`{
		"profile_id": %q,
		"name": "Bootstrap Session"
	}`, saved.ID)))
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	e.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create session status = %d, want %d, body=%s", createRec.Code, http.StatusOK, createRec.Body.String())
	}
	var created struct {
		Session agentsessions.ExternalSession `json:"session"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create session) error = %v", err)
	}
	if created.Session.Status != agentsessions.SessionStatusIdle {
		t.Fatalf("created session status = %s, want %s", created.Session.Status, agentsessions.SessionStatusIdle)
	}

	sendReq := httptest.NewRequest(http.MethodPost, "/chat/agent-sessions/sessions/"+created.Session.ID+"/messages", bytes.NewBufferString(`{
		"message": "say hello"
	}`))
	sendReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	sendRec := httptest.NewRecorder()
	e.ServeHTTP(sendRec, sendReq)
	if sendRec.Code != http.StatusOK {
		t.Fatalf("send message status = %d, want %d, body=%s", sendRec.Code, http.StatusOK, sendRec.Body.String())
	}

	deadline := time.Now().Add(8 * time.Second)
	for {
		historyReq := httptest.NewRequest(http.MethodGet, "/chat/agent-sessions/sessions/"+created.Session.ID+"/history?limit=20", nil)
		historyRec := httptest.NewRecorder()
		e.ServeHTTP(historyRec, historyReq)
		if historyRec.Code != http.StatusOK {
			t.Fatalf("history status = %d, want %d, body=%s", historyRec.Code, http.StatusOK, historyRec.Body.String())
		}
		var payload struct {
			History []agentsessions.SessionHistoryItem `json:"history"`
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
		if assistantMessage == "hello from bootstrap fallback" && sawCompleted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for bootstrap session completion, history=%+v", payload.History)
		}
		time.Sleep(25 * time.Millisecond)
	}

	closeReq := httptest.NewRequest(http.MethodPost, "/chat/agent-sessions/sessions/"+created.Session.ID+"/close", nil)
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

func writeBootstrapACPConversationStub(t *testing.T, path, logPath, replyText string) {
	t.Helper()
	script := fmt.Sprintf(`#!/usr/bin/python3
import json
import sys
import time

with open(%q, "w", encoding="utf-8") as handle:
    handle.write(" ".join(sys.argv[1:]))

session_id = "bootstrap-stub-session"
for line in sys.stdin:
    if not line.strip():
        continue
    request = json.loads(line)
    method = request.get("method")
    request_id = request.get("id")
    if method == "initialize":
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {
                "protocolVersion": 1,
                "agentCapabilities": {"loadSession": False},
                "agentInfo": {"name": "bootstrap-stub"},
            },
        }
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()
    elif method == "session/new":
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {"sessionId": session_id},
        }
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()
    elif method == "session/prompt":
        time.sleep(0.05)
        update = {
            "jsonrpc": "2.0",
            "method": "session/update",
            "params": {
                "sessionId": session_id,
                "update": {
                    "sessionUpdate": "agent_message_chunk",
                    "content": {"text": %q},
                },
            },
        }
        response = {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {"stopReason": "end_turn"},
        }
        sys.stdout.write(json.dumps(update) + "\n")
        sys.stdout.write(json.dumps(response) + "\n")
        sys.stdout.flush()
`, logPath, replyText)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
