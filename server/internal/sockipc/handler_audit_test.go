package sockipc

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"go.uber.org/zap"
)

type mockAuditBackend struct {
	entries         []sessionaudit.Entry
	watchCh         chan sessionaudit.Entry
	lastConvID      string
	lastLimit       int
	lastWatchConvID string
}

func (m *mockAuditBackend) Recent(ctx context.Context, conversationID string, limit int) ([]sessionaudit.Entry, error) {
	m.lastConvID = conversationID
	m.lastLimit = limit
	if conversationID == "" {
		return append([]sessionaudit.Entry(nil), m.entries...), nil
	}

	filtered := make([]sessionaudit.Entry, 0, len(m.entries))
	for _, entry := range m.entries {
		if entry.ConversationID == conversationID {
			filtered = append(filtered, entry)
		}
	}
	return filtered, nil
}

func (m *mockAuditBackend) Subscribe(conversationID string) (<-chan sessionaudit.Entry, func()) {
	m.lastWatchConvID = conversationID
	if m.watchCh == nil {
		m.watchCh = make(chan sessionaudit.Entry, 8)
	}
	return m.watchCh, func() {}
}

func TestRegisterAuditHandlersRecentReturnsEntries(t *testing.T) {
	backend := &mockAuditBackend{
		entries: []sessionaudit.Entry{
			{ID: "1", ConversationID: "conv-a", EventType: "user_message", Payload: "hello", CreatedAt: time.Now()},
			{ID: "2", ConversationID: "conv-b", EventType: "tool_result", Payload: "done", CreatedAt: time.Now()},
		},
	}
	srv := NewServer(t.TempDir()+"/audit.sock", zap.NewNop())
	RegisterAuditHandlers(srv, backend, zap.NewNop())

	handler := srv.handlers["audit.recent"]
	if handler == nil {
		t.Fatal("expected audit.recent handler to be registered")
	}

	resp := handler(context.Background(), &Request{Params: map[string]string{"limit": "10"}})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if got := backend.lastLimit; got != 10 {
		t.Fatalf("limit = %d, want 10", got)
	}
	if got := resp.Data["count"]; got != "2" {
		t.Fatalf("count = %q, want 2", got)
	}

	var entries []sessionaudit.Entry
	if err := json.Unmarshal([]byte(resp.Data["entries"]), &entries); err != nil {
		t.Fatalf("unmarshal entries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
}

func TestRegisterAuditHandlersRecentFiltersConversationAndQuery(t *testing.T) {
	backend := &mockAuditBackend{
		entries: []sessionaudit.Entry{
			{ID: "1", ConversationID: "conv-a", EventType: "assistant_message", Payload: "rollback plan is ready"},
			{ID: "2", ConversationID: "conv-a", EventType: "assistant_message", Payload: "kitchen sink output"},
			{ID: "3", ConversationID: "conv-b", EventType: "assistant_message", Payload: "rollback elsewhere"},
		},
	}
	srv := NewServer(t.TempDir()+"/audit.sock", zap.NewNop())
	RegisterAuditHandlers(srv, backend, zap.NewNop())

	resp := srv.handlers["audit.recent"](context.Background(), &Request{Params: map[string]string{
		"conversation_id": "conv-a",
		"query":           "rollback ready",
	}})
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if got := backend.lastConvID; got != "conv-a" {
		t.Fatalf("conversation_id = %q, want conv-a", got)
	}

	var entries []sessionaudit.Entry
	if err := json.Unmarshal([]byte(resp.Data["entries"]), &entries); err != nil {
		t.Fatalf("unmarshal entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if got := entries[0].Payload; got != "rollback plan is ready" {
		t.Fatalf("payload = %q, want rollback plan is ready", got)
	}
}

func TestParseAuditRecentLimitClampsInvalidValues(t *testing.T) {
	if got := parseAuditRecentLimit(""); got != defaultAuditRecentLimit {
		t.Fatalf("empty limit = %d, want %d", got, defaultAuditRecentLimit)
	}
	if got := parseAuditRecentLimit("0"); got != defaultAuditRecentLimit {
		t.Fatalf("zero limit = %d, want %d", got, defaultAuditRecentLimit)
	}
	if got := parseAuditRecentLimit("9999"); got != maxAuditRecentLimit {
		t.Fatalf("clamped limit = %d, want %d", got, maxAuditRecentLimit)
	}
}

func TestRegisterAuditHandlersWatchStreamsSnapshotAndNewEntries(t *testing.T) {
	backend := &mockAuditBackend{
		entries: []sessionaudit.Entry{
			{
				ID:             "1",
				ConversationID: "conv-a",
				EventType:      "assistant_message",
				Role:           "assistant",
				Payload:        "rollback plan is ready",
				CreatedAt:      time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC),
			},
			{
				ID:             "2",
				ConversationID: "conv-b",
				EventType:      "assistant_message",
				Role:           "assistant",
				Payload:        "other conversation snapshot",
				CreatedAt:      time.Date(2026, 4, 6, 10, 0, 1, 0, time.UTC),
			},
		},
		watchCh: make(chan sessionaudit.Entry, 8),
	}

	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	RegisterAuditHandlers(srv, backend, zap.NewNop())
	startServerOrSkip(t, srv)
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := WriteJSON(conn, &Request{
		Cmd: "audit.watch",
		Params: map[string]string{
			"conversation_id": "conv-a",
			"limit":           "5",
			"query":           "rollback ready",
		},
	}); err != nil {
		t.Fatalf("WriteJSON(request) error = %v", err)
	}

	snapshotResp, err := ReadJSON[Response](conn)
	if err != nil {
		t.Fatalf("ReadJSON(snapshot) error = %v", err)
	}
	if got := backend.lastWatchConvID; got != "conv-a" {
		t.Fatalf("watch conversation_id = %q, want conv-a", got)
	}
	if got := snapshotResp.Data["mode"]; got != "snapshot" {
		t.Fatalf("snapshot mode = %q, want snapshot", got)
	}

	var snapshot []sessionaudit.Entry
	if err := json.Unmarshal([]byte(snapshotResp.Data["entries"]), &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot entries: %v", err)
	}
	if len(snapshot) != 1 {
		t.Fatalf("snapshot len = %d, want 1", len(snapshot))
	}
	if got := snapshot[0].Payload; got != "rollback plan is ready" {
		t.Fatalf("snapshot payload = %q, want rollback plan is ready", got)
	}

	backend.watchCh <- sessionaudit.Entry{
		ID:             "live-ignore",
		ConversationID: "conv-a",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "kitchen sink output",
		CreatedAt:      time.Date(2026, 4, 6, 10, 0, 2, 0, time.UTC),
	}
	backend.watchCh <- sessionaudit.Entry{
		ID:             "live-match",
		ConversationID: "conv-a",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "rollback ready and finished cleanly",
		CreatedAt:      time.Date(2026, 4, 6, 10, 0, 3, 0, time.UTC),
	}

	eventResp, err := ReadJSON[Response](conn)
	if err != nil {
		t.Fatalf("ReadJSON(event) error = %v", err)
	}
	if got := eventResp.Data["mode"]; got != "event" {
		t.Fatalf("event mode = %q, want event", got)
	}

	var live sessionaudit.Entry
	if err := json.Unmarshal([]byte(eventResp.Data["entry"]), &live); err != nil {
		t.Fatalf("unmarshal live entry: %v", err)
	}
	if got := live.ID; got != "live-match" {
		t.Fatalf("live id = %q, want live-match", got)
	}
	if got := live.Payload; got != "rollback ready and finished cleanly" {
		t.Fatalf("live payload = %q, want rollback ready and finished cleanly", got)
	}
}
