package push

import (
	"context"
	"database/sql"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

// mockPublisher captures SSE events for testing.
type mockPublisher struct {
	mu     sync.Mutex
	events []publishedEvent
}

type publishedEvent struct {
	UserID    string
	EventType string
	Data      any
}

func (m *mockPublisher) Publish(userID string, eventType string, data any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, publishedEvent{userID, eventType, data})
}

func (m *mockPublisher) getEvents() []publishedEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]publishedEvent, len(m.events))
	copy(cp, m.events)
	return cp
}

// mockInjector captures injected messages.
type mockInjector struct {
	mu       sync.Mutex
	messages []injectedMessage
}

type injectedMessage struct {
	OwnerID   string
	SessionID string
	Content   string
}

func (m *mockInjector) InjectMessage(_ context.Context, ownerID, sessionID, content string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, injectedMessage{ownerID, sessionID, content})
	return "conv-injected", nil
}

func (m *mockInjector) getMessages() []injectedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]injectedMessage, len(m.messages))
	copy(cp, m.messages)
	return cp
}

func testServiceDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testService(t *testing.T) (*Service, *Store) {
	t.Helper()
	db := testServiceDB(t)
	store, err := NewStore(db)
	if err != nil {
		t.Fatal(err)
	}
	logger := zap.NewNop()
	svc := NewService(store, logger)
	return svc, store
}

func TestSetLocaleFunc(t *testing.T) {
	svc, _ := testService(t)

	called := false
	svc.SetLocaleFunc(func() string {
		called = true
		return "zh-CN"
	})

	// Verify the func is stored by reading it back
	svc.mu.RLock()
	fn := svc.localeFunc
	svc.mu.RUnlock()

	if fn == nil {
		t.Fatal("localeFunc should not be nil")
	}
	result := fn()
	if !called {
		t.Error("localeFunc was not called")
	}
	if result != "zh-CN" {
		t.Errorf("localeFunc() = %q, want %q", result, "zh-CN")
	}
}

func TestFirePush_TypelessCardFormat(t *testing.T) {
	svc, store := testService(t)

	inj := &mockInjector{}
	svc.SetMessageInjector(inj)

	ctx := context.Background()
	r := &PushNotification{
		ID:      "push-1",
		OwnerID: "user-1",
		Message: "Drink water",
		FireAt:  time.Now(),
		Status:  StatusPending,
	}
	store.Create(ctx, r)

	svc.firePush(ctx, r)

	msgs := inj.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 injected message, got %d", len(msgs))
	}

	content := msgs[0].Content
	// Should contain title_key for i18n, not hardcoded title
	if !strings.Contains(content, `"title_key":"push.reminder"`) {
		t.Errorf("expected title_key in card, got: %s", content)
	}
	// Should contain the message
	if !strings.Contains(content, "Drink water") {
		t.Errorf("expected message in card, got: %s", content)
	}
	// Should NOT contain hardcoded emoji title
	if strings.Contains(content, `"title":"📢"`) {
		t.Error("card should not have hardcoded emoji title")
	}
	// Should be a typeless block
	if !strings.HasPrefix(content, "```typeless\n") {
		t.Errorf("expected typeless block prefix, got: %s", content)
	}
}

func TestFirePush_SSEEventIncludesLocale(t *testing.T) {
	svc, store := testService(t)

	pub := &mockPublisher{}
	svc.SetEventPublisher(pub)
	svc.SetLocaleFunc(func() string { return "ja-JP" })

	ctx := context.Background()
	r := &PushNotification{
		ID:      "push-2",
		OwnerID: "user-1",
		Message: "Test",
		FireAt:  time.Now(),
		Status:  StatusPending,
	}
	store.Create(ctx, r)

	svc.firePush(ctx, r)

	events := pub.getEvents()
	// Should have "push" + "conversation_updated" events
	var pushEvent publishedEvent
	found := false
	for _, e := range events {
		if e.EventType == "push" {
			pushEvent = e
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'push' SSE event")
	}

	data, ok := pushEvent.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", pushEvent.Data)
	}

	// Should include locale
	if data["locale"] != "ja-JP" {
		t.Errorf("locale = %v, want %q", data["locale"], "ja-JP")
	}
	// Should include message
	if data["message"] != "Test" {
		t.Errorf("message = %v, want %q", data["message"], "Test")
	}
	// Should NOT include hardcoded title
	if _, has := data["title"]; has {
		t.Error("SSE push event should not have hardcoded 'title' field")
	}
}

func TestFirePush_SSEEventDefaultLocale(t *testing.T) {
	svc, store := testService(t)

	pub := &mockPublisher{}
	svc.SetEventPublisher(pub)
	// No SetLocaleFunc — should default to "en-US"

	ctx := context.Background()
	r := &PushNotification{
		ID:      "push-3",
		OwnerID: "user-1",
		Message: "Default locale test",
		FireAt:  time.Now(),
		Status:  StatusPending,
	}
	store.Create(ctx, r)

	svc.firePush(ctx, r)

	events := pub.getEvents()
	for _, e := range events {
		if e.EventType == "push" {
			data := e.Data.(map[string]any)
			if data["locale"] != "en-US" {
				t.Errorf("default locale = %v, want %q", data["locale"], "en-US")
			}
			return
		}
	}
	t.Fatal("expected 'push' SSE event")
}

func TestFirePush_ConversationIDInSSE(t *testing.T) {
	svc, store := testService(t)

	inj := &mockInjector{} // returns "conv-injected"
	pub := &mockPublisher{}
	svc.SetMessageInjector(inj)
	svc.SetEventPublisher(pub)

	ctx := context.Background()
	r := &PushNotification{
		ID:        "push-4",
		OwnerID:   "user-1",
		Message:   "With session",
		SessionID: "sess-1",
		FireAt:    time.Now(),
		Status:    StatusPending,
	}
	store.Create(ctx, r)

	svc.firePush(ctx, r)

	events := pub.getEvents()
	for _, e := range events {
		if e.EventType == "push" {
			data := e.Data.(map[string]any)
			if data["conversation_id"] != "conv-injected" {
				t.Errorf("conversation_id = %v, want %q", data["conversation_id"], "conv-injected")
			}
			return
		}
	}
	t.Fatal("expected 'push' SSE event")
}

func TestServiceAdd(t *testing.T) {
	svc, _ := testService(t)

	ctx := context.Background()
	r, err := svc.Add(ctx, "user-1", "Take medicine", time.Now().Add(time.Hour), "", "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected notification, got nil")
	}
	if r.Message != "Take medicine" {
		t.Errorf("message = %q, want %q", r.Message, "Take medicine")
	}
	if r.SessionID != "sess-1" {
		t.Errorf("session_id = %q, want %q", r.SessionID, "sess-1")
	}
	if r.Status != StatusPending {
		t.Errorf("status = %q, want %q", r.Status, StatusPending)
	}
	if !strings.HasPrefix(r.ID, "push_") {
		t.Errorf("ID = %q, expected push_ prefix", r.ID)
	}
}

func TestTimeToCron(t *testing.T) {
	tests := []struct {
		name      string
		fireAt    time.Time
		recurring string
		wantParts int // number of space-separated parts
	}{
		{"one-shot", time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC), "", 6},
		{"daily", time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC), "daily", 6},
		{"weekly", time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC), "weekly", 6},
		{"monthly", time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC), "monthly", 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cron := timeToCron(tt.fireAt, tt.recurring)
			parts := strings.Fields(cron)
			if len(parts) != tt.wantParts {
				t.Errorf("timeToCron() = %q, has %d parts, want %d", cron, len(parts), tt.wantParts)
			}
			// Daily should have wildcards for day/month
			if tt.recurring == "daily" {
				if !strings.Contains(cron, "* * *") {
					t.Errorf("daily cron should have '* * *', got %q", cron)
				}
			}
		})
	}
}

func TestNextOccurrence(t *testing.T) {
	base := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)

	daily := nextOccurrence(base, "daily")
	if daily.Day() != 16 {
		t.Errorf("daily next = day %d, want 16", daily.Day())
	}

	weekly := nextOccurrence(base, "weekly")
	if weekly.Day() != 22 {
		t.Errorf("weekly next = day %d, want 22", weekly.Day())
	}

	monthly := nextOccurrence(base, "monthly")
	if monthly.Month() != 4 {
		t.Errorf("monthly next = month %d, want 4", monthly.Month())
	}

	// Unknown recurring returns same time
	same := nextOccurrence(base, "")
	if !same.Equal(base) {
		t.Errorf("empty recurring should return same time")
	}
}
