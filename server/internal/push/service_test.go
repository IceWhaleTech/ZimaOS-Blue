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

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
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

// mockWebPushSender captures web push sends.
type mockWebPushSender struct {
	mu    sync.Mutex
	calls []webPushCall
	err   error
}

type webPushCall struct {
	UserID string
	Title  string
	Body   string
}

func (m *mockWebPushSender) SendToUser(_ context.Context, userID, title, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, webPushCall{
		UserID: userID,
		Title:  title,
		Body:   body,
	})
	return m.err
}

func (m *mockWebPushSender) getCalls() []webPushCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]webPushCall, len(m.calls))
	copy(cp, m.calls)
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

func TestFirePush_SendsWebPushToOwner(t *testing.T) {
	svc, store := testService(t)

	wp := &mockWebPushSender{}
	svc.SetWebPushSender(wp)

	ctx := context.Background()
	r := &PushNotification{
		ID:      "push-web-1",
		OwnerID: "user-web-1",
		Message: "Drink water now",
		FireAt:  time.Now(),
		Status:  StatusPending,
	}
	store.Create(ctx, r)

	svc.firePush(ctx, r)

	calls := wp.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 web push call, got %d", len(calls))
	}
	if calls[0].UserID != "user-web-1" {
		t.Fatalf("web push userID = %q, want %q", calls[0].UserID, "user-web-1")
	}
	if calls[0].Title != "🔔 Reminder" {
		t.Fatalf("web push title = %q, want %q", calls[0].Title, "🔔 Reminder")
	}
	if calls[0].Body != "Drink water now" {
		t.Fatalf("web push body = %q, want %q", calls[0].Body, "Drink water now")
	}
}

func TestFirePush_DeliversAllChannels(t *testing.T) {
	svc, store := testService(t)

	inj := &mockInjector{}
	pub := &mockPublisher{}
	wp := &mockWebPushSender{}
	svc.SetMessageInjector(inj)
	svc.SetEventPublisher(pub)
	svc.SetWebPushSender(wp)
	svc.SetLocaleFunc(func() string { return "zh-CN" })

	ctx := context.Background()
	r := &PushNotification{
		ID:        "push-all-1",
		OwnerID:   "user-all-1",
		Message:   "10秒后喝水",
		SessionID: "conv-all-1",
		FireAt:    time.Now(),
		Status:    StatusPending,
	}
	store.Create(ctx, r)

	svc.firePush(ctx, r)

	msgs := inj.getMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 injected message, got %d", len(msgs))
	}
	if msgs[0].SessionID != "conv-all-1" {
		t.Fatalf("injected session_id = %q, want %q", msgs[0].SessionID, "conv-all-1")
	}
	if !strings.Contains(msgs[0].Content, "10秒后喝水") {
		t.Fatalf("injected content missing reminder message: %s", msgs[0].Content)
	}

	events := pub.getEvents()
	var pushEvt, convEvt bool
	for _, e := range events {
		if e.EventType == "push" {
			data, ok := e.Data.(map[string]any)
			if !ok {
				t.Fatalf("push event data type = %T, want map[string]any", e.Data)
			}
			if data["message"] != "10秒后喝水" {
				t.Fatalf("push event message = %v, want %q", data["message"], "10秒后喝水")
			}
			if data["locale"] != "zh-CN" {
				t.Fatalf("push event locale = %v, want %q", data["locale"], "zh-CN")
			}
			pushEvt = true
		}
		if e.EventType == "conversation_updated" {
			convEvt = true
		}
	}
	if !pushEvt {
		t.Fatal("expected push SSE event")
	}
	if !convEvt {
		t.Fatal("expected conversation_updated SSE event")
	}

	calls := wp.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 web push call, got %d", len(calls))
	}
	if calls[0].UserID != "user-all-1" {
		t.Fatalf("web push userID = %q, want %q", calls[0].UserID, "user-all-1")
	}
	if calls[0].Body != "10秒后喝水" {
		t.Fatalf("web push body = %q, want %q", calls[0].Body, "10秒后喝水")
	}

	stored, err := store.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("get reminder from store: %v", err)
	}
	if stored == nil {
		t.Fatalf("stored reminder not found: %s", r.ID)
	}
	if stored.Status != StatusFired {
		t.Fatalf("stored reminder status = %q, want %q", stored.Status, StatusFired)
	}
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

func TestServiceAdd_FiresAndInjectsIntoTargetSession(t *testing.T) {
	svc, store := testService(t)

	// Wire real cron service so we exercise actual scheduling path.
	cronSvc := cron.NewService(cron.DefaultConfig(), zap.NewNop())
	if err := cronSvc.Start(); err != nil {
		t.Fatalf("start cron service: %v", err)
	}
	defer cronSvc.Stop(context.Background())
	svc.SetCron(NewCronAdapter(func() *cron.Service { return cronSvc }))

	inj := &mockInjector{}
	svc.SetMessageInjector(inj)

	ctx := context.Background()
	fireAt := time.Now().Add(2 * time.Second)
	r, err := svc.Add(ctx, "user-1", "10秒后喝水", fireAt, "", "conv-target")
	if err != nil {
		t.Fatalf("add reminder: %v", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		msgs := inj.getMessages()
		if len(msgs) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	msgs := inj.getMessages()
	if len(msgs) == 0 {
		t.Fatalf("expected injected message after reminder fired, got none")
	}
	if msgs[0].SessionID != "conv-target" {
		t.Fatalf("injected session_id = %q, want %q", msgs[0].SessionID, "conv-target")
	}
	if !strings.Contains(msgs[0].Content, "push.reminder") {
		t.Fatalf("injected content missing typeless reminder card: %s", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "10秒后喝水") {
		t.Fatalf("injected content missing reminder message: %s", msgs[0].Content)
	}

	stored, err := store.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("get reminder from store: %v", err)
	}
	if stored == nil {
		t.Fatalf("stored reminder not found: %s", r.ID)
	}
	if stored.Status != StatusFired {
		t.Fatalf("stored reminder status = %q, want %q", stored.Status, StatusFired)
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
