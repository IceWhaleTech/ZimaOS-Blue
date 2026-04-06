package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
)

type auditCommandExitPanic struct {
	code int
}

func resetAuditCLIState(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	oldCfgFile, oldProfile := cfgFile, profile
	oldDevMode, oldJSONOutput := devMode, jsonOutput
	oldNoColor := noColor
	oldAuditFollow, oldAuditLines := auditFollow, auditLines
	oldAuditQuery, oldAuditPollInterval := auditQuery, auditPollInterval
	oldAuditExit := auditExit

	t.Cleanup(func() {
		cfgFile, profile = oldCfgFile, oldProfile
		devMode, jsonOutput = oldDevMode, oldJSONOutput
		noColor = oldNoColor
		auditFollow, auditLines = oldAuditFollow, oldAuditLines
		auditQuery, auditPollInterval = oldAuditQuery, oldAuditPollInterval
		auditExit = oldAuditExit
	})

	cfgFile = ""
	profile = ""
	devMode = false
	jsonOutput = false
	noColor = true
	auditFollow = false
	auditLines = 20
	auditQuery = ""
	auditPollInterval = 10 * time.Millisecond
	auditExit = os.Exit

	return filepath.Join(home, ".zimaos-blue", "data")
}

func newAuditTestStore(t *testing.T, dataDir string) *sessionaudit.Store {
	t.Helper()

	store, err := sessionaudit.NewJSONLStore(dataDir, sessionaudit.DefaultStoreConfig())
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})
	return store
}

func recordAuditTestEntry(t *testing.T, store *sessionaudit.Store, entry sessionaudit.Entry) {
	t.Helper()
	if err := store.Record(context.Background(), entry); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
}

func TestRunAuditTailPrintsRecentEntriesForConversation(t *testing.T) {
	dataDir := resetAuditCLIState(t)
	store := newAuditTestStore(t, dataDir)

	base := time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)
	recordAuditTestEntry(t, store, sessionaudit.Entry{
		ConversationID: "conv-a",
		EventType:      "user_message",
		Role:           "user",
		Payload:        "first troubleshooting note",
		CreatedAt:      base,
	})
	recordAuditTestEntry(t, store, sessionaudit.Entry{
		ConversationID: "conv-b",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "other conversation should stay hidden",
		CreatedAt:      base.Add(1 * time.Second),
	})
	recordAuditTestEntry(t, store, sessionaudit.Entry{
		ConversationID: "conv-a",
		EventType:      "tool_result",
		Role:           "tool",
		ToolName:       "shell",
		Payload:        "second troubleshooting note",
		CreatedAt:      base.Add(2 * time.Second),
	})

	out := captureStdout(t, func() {
		runAuditTail(nil, []string{"conv-a"})
	})

	if !strings.Contains(out, "first troubleshooting note") {
		t.Fatalf("stdout missing first conversation entry:\n%s", out)
	}
	if !strings.Contains(out, "second troubleshooting note") {
		t.Fatalf("stdout missing second conversation entry:\n%s", out)
	}
	if strings.Contains(out, "other conversation should stay hidden") {
		t.Fatalf("stdout unexpectedly included another conversation:\n%s", out)
	}
	if strings.Index(out, "first troubleshooting note") > strings.Index(out, "second troubleshooting note") {
		t.Fatalf("stdout not in chronological order:\n%s", out)
	}
}

func TestRunAuditTailJSONOutputFiltersByQuery(t *testing.T) {
	dataDir := resetAuditCLIState(t)
	store := newAuditTestStore(t, dataDir)

	jsonOutput = true
	auditQuery = "rollback"

	recordAuditTestEntry(t, store, sessionaudit.Entry{
		ConversationID: "conv-a",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "rollback plan is ready",
		CreatedAt:      time.Date(2026, 4, 5, 11, 0, 0, 0, time.UTC),
	})
	recordAuditTestEntry(t, store, sessionaudit.Entry{
		ConversationID: "conv-b",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "kitchen sink output",
		CreatedAt:      time.Date(2026, 4, 5, 11, 0, 1, 0, time.UTC),
	})

	out := captureStdout(t, func() {
		runAuditTail(nil, nil)
	})

	var resp struct {
		Count   int                  `json:"count"`
		Entries []sessionaudit.Entry `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode audit json response: %v\n%s", err, out)
	}
	if resp.Count != 1 {
		t.Fatalf("count = %d, want 1\n%s", resp.Count, out)
	}
	if len(resp.Entries) != 1 {
		t.Fatalf("entries len = %d, want 1\n%s", len(resp.Entries), out)
	}
	if got := resp.Entries[0].ConversationID; got != "conv-a" {
		t.Fatalf("conversation_id = %q, want conv-a", got)
	}
	if got := resp.Entries[0].Payload; got != "rollback plan is ready" {
		t.Fatalf("payload = %q, want rollback plan is ready", got)
	}
}

func TestTailAuditEntriesFollowsAppendedEntries(t *testing.T) {
	dataDir := resetAuditCLIState(t)
	store := newAuditTestStore(t, dataDir)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	received := make(chan sessionaudit.Entry, 1)
	errCh := make(chan error, 1)
	go func() {
		errCh <- tailAuditEntries(ctx, auditTailOptions{
			DataDir:      dataDir,
			Conversation: "conv-follow",
			Lines:        5,
			Follow:       true,
			PollInterval: auditPollInterval,
		}, func(entry sessionaudit.Entry) error {
			select {
			case received <- entry:
			default:
			}
			cancel()
			return nil
		})
	}()

	time.Sleep(30 * time.Millisecond)
	recordAuditTestEntry(t, store, sessionaudit.Entry{
		ConversationID: "conv-follow",
		EventType:      "tool_result",
		Role:           "tool",
		ToolName:       "shell",
		Payload:        "followed entry",
		CreatedAt:      time.Date(2026, 4, 5, 12, 0, 0, 0, time.UTC),
	})

	select {
	case entry := <-received:
		if entry.ConversationID != "conv-follow" {
			t.Fatalf("conversation_id = %q, want conv-follow", entry.ConversationID)
		}
		if entry.Payload != "followed entry" {
			t.Fatalf("payload = %q, want followed entry", entry.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for followed audit entry")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("tailAuditEntries() error = %v", err)
	}
}

func TestRunAuditRecentCommandUsesIPC(t *testing.T) {
	dataDir := resetAuditCLIState(t)
	_ = dataDir

	oldRoundTrip := auditIPCRoundTripFunc
	defer func() { auditIPCRoundTripFunc = oldRoundTrip }()

	jsonOutput = true
	auditLines = 7
	auditQuery = "rollback"
	auditExit = func(code int) { panic(auditCommandExitPanic{code: code}) }

	var capturedReq *sockipc.Request
	auditIPCRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		capturedReq = req
		payload, _ := json.Marshal([]sessionaudit.Entry{{
			ID:             "entry-1",
			ConversationID: "conv-ipc",
			EventType:      "assistant_message",
			Role:           "assistant",
			Payload:        "rollback plan is ready",
			CreatedAt:      time.Date(2026, 4, 5, 13, 0, 0, 0, time.UTC),
		}})
		return sockipc.OkResponse(map[string]string{
			"entries": string(payload),
			"count":   "1",
		}), nil
	}

	exitCode, stdout := runAuditCommandForTest(func() {
		runAuditRecentCommand(nil, []string{"conv-ipc"})
	})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout=%q)", exitCode, stdout)
	}
	if capturedReq == nil {
		t.Fatal("expected IPC request to be sent")
	}
	if capturedReq.Cmd != "audit.recent" {
		t.Fatalf("cmd = %q, want audit.recent", capturedReq.Cmd)
	}
	for key, want := range map[string]string{
		"conversation_id": "conv-ipc",
		"limit":           "7",
		"query":           "rollback",
	} {
		if got := capturedReq.Params[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}

	var resp struct {
		Count   int                  `json:"count"`
		Entries []sessionaudit.Entry `json:"entries"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("decode json output: %v\n%s", err, stdout)
	}
	if resp.Count != 1 || len(resp.Entries) != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestRunAuditRecentCommandRequiresRunningServiceWhenIPCUnavailable(t *testing.T) {
	resetAuditCLIState(t)

	oldRoundTrip := auditIPCRoundTripFunc
	defer func() { auditIPCRoundTripFunc = oldRoundTrip }()

	auditExit = func(code int) { panic(auditCommandExitPanic{code: code}) }
	auditIPCRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		return nil, fmt.Errorf("dial unix /tmp/missing-session-audit.sock: connect: no such file or directory")
	}

	exitCode, stdout := runAuditCommandForTest(func() {
		runAuditRecentCommand(nil, []string{"conv-ipc"})
	})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1 (stdout=%q)", exitCode, stdout)
	}
	if !strings.Contains(stdout, "running Blue service is required for audit.recent") {
		t.Fatalf("stdout = %q, want missing-service error", stdout)
	}
}

func TestRunAuditRecentCommandReportsDisabledAuditIPC(t *testing.T) {
	resetAuditCLIState(t)

	oldRoundTrip := auditIPCRoundTripFunc
	defer func() { auditIPCRoundTripFunc = oldRoundTrip }()

	auditExit = func(code int) { panic(auditCommandExitPanic{code: code}) }
	auditIPCRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		return &sockipc.Response{Status: "error", Error: "unknown cmd: audit.recent"}, nil
	}

	exitCode, stdout := runAuditCommandForTest(func() {
		runAuditRecentCommand(nil, nil)
	})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1 (stdout=%q)", exitCode, stdout)
	}
	if !strings.Contains(stdout, "session.audit.ipc_enabled") {
		t.Fatalf("stdout = %q, want enable hint", stdout)
	}
}

func TestRunAuditWatchCommandStreamsEntriesFromIPC(t *testing.T) {
	resetAuditCLIState(t)

	oldWatch := auditIPCWatchFunc
	defer func() { auditIPCWatchFunc = oldWatch }()

	auditLines = 7
	auditQuery = "rollback"
	auditIPCWatchFunc = func(ctx context.Context, req *sockipc.Request, onResp func(*sockipc.Response) error) error {
		_ = ctx
		snapshotPayload, _ := json.Marshal([]sessionaudit.Entry{{
			ID:             "snapshot-1",
			ConversationID: "conv-ipc",
			EventType:      "assistant_message",
			Role:           "assistant",
			Payload:        "rollback plan is ready",
			CreatedAt:      time.Date(2026, 4, 6, 14, 0, 0, 0, time.UTC),
		}})
		if req.Cmd != "audit.watch" {
			t.Fatalf("cmd = %q, want audit.watch", req.Cmd)
		}
		for key, want := range map[string]string{
			"conversation_id": "conv-ipc",
			"limit":           "7",
			"query":           "rollback",
		} {
			if got := req.Params[key]; got != want {
				t.Fatalf("%s = %q, want %q", key, got, want)
			}
		}
		if err := onResp(sockipc.OkResponse(map[string]string{
			"mode":    "snapshot",
			"entries": string(snapshotPayload),
			"count":   "1",
		})); err != nil {
			return err
		}

		livePayload, _ := json.Marshal(sessionaudit.Entry{
			ID:             "live-1",
			ConversationID: "conv-ipc",
			EventType:      "tool_result",
			Role:           "tool",
			ToolName:       "shell",
			Payload:        "rollback finished cleanly",
			CreatedAt:      time.Date(2026, 4, 6, 14, 0, 1, 0, time.UTC),
		})
		return onResp(sockipc.OkResponse(map[string]string{
			"mode":  "event",
			"entry": string(livePayload),
		}))
	}

	exitCode, stdout := runAuditCommandForTest(func() {
		runAuditWatchCommand(nil, []string{"conv-ipc"})
	})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout=%q)", exitCode, stdout)
	}
	if !strings.Contains(stdout, "rollback plan is ready") {
		t.Fatalf("stdout missing snapshot entry:\n%s", stdout)
	}
	if !strings.Contains(stdout, "rollback finished cleanly") {
		t.Fatalf("stdout missing live entry:\n%s", stdout)
	}
}

func TestRunAuditWatchCommandRequiresRunningServiceWhenIPCUnavailable(t *testing.T) {
	resetAuditCLIState(t)

	oldWatch := auditIPCWatchFunc
	defer func() { auditIPCWatchFunc = oldWatch }()

	auditExit = func(code int) { panic(auditCommandExitPanic{code: code}) }
	auditIPCWatchFunc = func(ctx context.Context, req *sockipc.Request, onResp func(*sockipc.Response) error) error {
		_, _, _ = ctx, req, onResp
		return fmt.Errorf("dial unix /tmp/missing-session-audit.sock: connect: no such file or directory")
	}

	exitCode, stdout := runAuditCommandForTest(func() {
		runAuditWatchCommand(nil, []string{"conv-ipc"})
	})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1 (stdout=%q)", exitCode, stdout)
	}
	if !strings.Contains(stdout, "running Blue service is required for audit.watch") {
		t.Fatalf("stdout = %q, want missing-service error", stdout)
	}
}

func runAuditCommandForTest(fn func()) (exitCode int, stdout string) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()
		stdout = buf.String()
		if rec := recover(); rec != nil {
			if exit, ok := rec.(auditCommandExitPanic); ok {
				exitCode = exit.code
				return
			}
			panic(rec)
		}
	}()

	fn()
	return 0, stdout
}
