package companion

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONLStorage(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "companion-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage
	storage, err := NewJSONLStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	t.Run("SaveAndGetSession", func(t *testing.T) {
		session := &Session{
			ID:        "test-session-1",
			Platform:  PlatformTelegram,
			UserID:    "user-123",
			Status:    SessionStatusActive,
			StartedAt: time.Now(),
		}

		err := storage.SaveSession(ctx, session)
		if err != nil {
			t.Fatalf("failed to save session: %v", err)
		}

		// Verify file was created
		filePath := filepath.Join(tmpDir, "sessions", session.StartedAt.Format("2006-01-02"), "session-test-session-1.jsonl")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("session file was not created")
		}

		// Get session
		retrieved, err := storage.GetSession(ctx, "test-session-1")
		if err != nil {
			t.Fatalf("failed to get session: %v", err)
		}

		if retrieved.ID != session.ID {
			t.Errorf("expected session ID %s, got %s", session.ID, retrieved.ID)
		}
		if retrieved.Platform != session.Platform {
			t.Errorf("expected platform %s, got %s", session.Platform, retrieved.Platform)
		}
	})

	t.Run("ListSessions", func(t *testing.T) {
		// Create additional sessions
		for i := 2; i <= 5; i++ {
			session := &Session{
				ID:        "test-session-" + string(rune('0'+i)),
				Platform:  PlatformDiscord,
				UserID:    "user-456",
				Status:    SessionStatusActive,
				StartedAt: time.Now().Add(time.Duration(i) * time.Minute),
			}
			if err := storage.SaveSession(ctx, session); err != nil {
				t.Fatalf("failed to save session %d: %v", i, err)
			}
		}

		// List all sessions
		sessions, total, err := storage.ListSessions(ctx, nil)
		if err != nil {
			t.Fatalf("failed to list sessions: %v", err)
		}

		if total < 5 {
			t.Errorf("expected at least 5 sessions, got %d", total)
		}

		// List with filter
		opts := &ListOptions{
			Platform: PlatformDiscord,
		}
		sessions, total, err = storage.ListSessions(ctx, opts)
		if err != nil {
			t.Fatalf("failed to list sessions with filter: %v", err)
		}

		for _, s := range sessions {
			if s.Platform != PlatformDiscord {
				t.Errorf("expected platform Discord, got %s", s.Platform)
			}
		}
	})

	t.Run("AppendAndGetEvents", func(t *testing.T) {
		sessionID := "test-session-1"

		// Append events
		events := []*SessionEvent{
			{
				ID:        "event-1",
				SessionID: sessionID,
				Timestamp: time.Now(),
				EventType: EventMessageReceived,
				Platform:  PlatformTelegram,
				UserID:    "user-123",
				Status:    "success",
				Message: &MessageEvent{
					Direction: "inbound",
					Content:   "Hello, world!",
					Length:    13,
				},
			},
			{
				ID:        "event-2",
				SessionID: sessionID,
				Timestamp: time.Now().Add(time.Second),
				EventType: EventToolCall,
				Platform:  PlatformTelegram,
				UserID:    "user-123",
				Status:    "success",
				ToolCall: &ToolCallEvent{
					ToolName: "search",
					ToolID:   "tool-1",
					Duration: 100 * time.Millisecond,
					Status:   "completed",
				},
			},
		}

		for _, event := range events {
			if err := storage.AppendEvent(ctx, event); err != nil {
				t.Fatalf("failed to append event: %v", err)
			}
		}

		// Get events
		retrieved, total, err := storage.GetSessionEvents(ctx, sessionID, nil)
		if err != nil {
			t.Fatalf("failed to get events: %v", err)
		}

		if total != 2 {
			t.Errorf("expected 2 events, got %d", total)
		}

		if len(retrieved) != 2 {
			t.Errorf("expected 2 events, got %d", len(retrieved))
		}
	})

	t.Run("SaveAndGetAlert", func(t *testing.T) {
		alert := &Alert{
			ID:          "alert-1",
			Severity:    AlertSeverityHigh,
			Title:       "Test Alert",
			Description: "This is a test alert",
			SessionID:   "test-session-1",
			ThreatLevel: ThreatLevelHigh,
			Timestamp:   time.Now(),
		}

		err := storage.SaveAlert(ctx, alert)
		if err != nil {
			t.Fatalf("failed to save alert: %v", err)
		}

		// Get alert
		retrieved, err := storage.GetAlert(ctx, "alert-1")
		if err != nil {
			t.Fatalf("failed to get alert: %v", err)
		}

		if retrieved.ID != alert.ID {
			t.Errorf("expected alert ID %s, got %s", alert.ID, retrieved.ID)
		}
		if retrieved.Severity != alert.Severity {
			t.Errorf("expected severity %s, got %s", alert.Severity, retrieved.Severity)
		}
	})

	t.Run("UpdateAlert", func(t *testing.T) {
		alert, err := storage.GetAlert(ctx, "alert-1")
		if err != nil {
			t.Fatalf("failed to get alert: %v", err)
		}

		now := time.Now()
		alert.Acknowledged = true
		alert.AckedAt = &now
		alert.AckedBy = "admin"

		err = storage.UpdateAlert(ctx, alert)
		if err != nil {
			t.Fatalf("failed to update alert: %v", err)
		}

		// Verify update
		retrieved, err := storage.GetAlert(ctx, "alert-1")
		if err != nil {
			t.Fatalf("failed to get updated alert: %v", err)
		}

		if !retrieved.Acknowledged {
			t.Error("expected alert to be acknowledged")
		}
		if retrieved.AckedBy != "admin" {
			t.Errorf("expected acked_by admin, got %s", retrieved.AckedBy)
		}
	})

	t.Run("ListAlerts", func(t *testing.T) {
		// Add more alerts
		for i := 2; i <= 5; i++ {
			alert := &Alert{
				ID:        "alert-" + string(rune('0'+i)),
				Severity:  AlertSeverityWarning,
				Title:     "Test Alert " + string(rune('0'+i)),
				Timestamp: time.Now().Add(time.Duration(i) * time.Minute),
			}
			if err := storage.SaveAlert(ctx, alert); err != nil {
				t.Fatalf("failed to save alert %d: %v", i, err)
			}
		}

		// List all alerts
		alerts, total, err := storage.ListAlerts(ctx, nil)
		if err != nil {
			t.Fatalf("failed to list alerts: %v", err)
		}

		if total < 5 {
			t.Errorf("expected at least 5 alerts, got %d", total)
		}

		// List with filter
		opts := &ListOptions{
			Filters: map[string]string{
				"acknowledged": "false",
			},
		}
		alerts, _, err = storage.ListAlerts(ctx, opts)
		if err != nil {
			t.Fatalf("failed to list alerts with filter: %v", err)
		}

		for _, a := range alerts {
			if a.Acknowledged {
				t.Error("expected unacknowledged alerts only")
			}
		}
	})

	t.Run("SaveAndGetDailyStats", func(t *testing.T) {
		stats := &DailyStats{
			Date:          time.Now().Format("2006-01-02"),
			TotalSessions: 10,
			TotalEvents:   100,
			TotalAlerts:   5,
			SessionsByPlatform: map[Platform]int{
				PlatformTelegram: 5,
				PlatformDiscord:  5,
			},
			ThreatsByLevel: map[ThreatLevel]int{
				ThreatLevelNone: 8,
				ThreatLevelLow:  2,
			},
			AvgSessionDuration: 5 * time.Minute,
		}

		err := storage.SaveDailyStats(ctx, stats)
		if err != nil {
			t.Fatalf("failed to save daily stats: %v", err)
		}

		// Get stats
		retrieved, err := storage.GetDailyStats(ctx, stats.Date)
		if err != nil {
			t.Fatalf("failed to get daily stats: %v", err)
		}

		if retrieved.TotalSessions != stats.TotalSessions {
			t.Errorf("expected total sessions %d, got %d", stats.TotalSessions, retrieved.TotalSessions)
		}
	})
}

func TestEventStreamer(t *testing.T) {
	config := DefaultConfig()
	streamer := NewEventStreamer(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := streamer.Start(ctx); err != nil {
		t.Fatalf("failed to start streamer: %v", err)
	}
	defer streamer.Stop()

	t.Run("SubscribeAndReceive", func(t *testing.T) {
		// Subscribe to all events
		ch, cleanup := streamer.Subscribe("")
		defer cleanup()

		// Emit event
		event := &SessionEvent{
			ID:        "test-event-1",
			SessionID: "session-1",
			Timestamp: time.Now(),
			EventType: EventMessageReceived,
			Status:    "success",
		}
		streamer.Emit(event)

		// Wait for event
		select {
		case received := <-ch:
			if received.ID != event.ID {
				t.Errorf("expected event ID %s, got %s", event.ID, received.ID)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
		}
	})

	t.Run("SubscribeToSession", func(t *testing.T) {
		// Subscribe to specific session
		ch, cleanup := streamer.Subscribe("session-2")
		defer cleanup()

		// Emit event for different session (should not receive)
		event1 := &SessionEvent{
			ID:        "test-event-2",
			SessionID: "session-1",
			Timestamp: time.Now(),
			EventType: EventMessageReceived,
			Status:    "success",
		}
		streamer.Emit(event1)

		// Emit event for subscribed session (should receive)
		event2 := &SessionEvent{
			ID:        "test-event-3",
			SessionID: "session-2",
			Timestamp: time.Now(),
			EventType: EventToolCall,
			Status:    "success",
		}
		streamer.Emit(event2)

		// Should receive only event2
		select {
		case received := <-ch:
			if received.ID != event2.ID {
				t.Errorf("expected event ID %s, got %s", event2.ID, received.ID)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
		}
	})

	t.Run("SubscriberCount", func(t *testing.T) {
		initialCount := streamer.SubscriberCount()

		_, cleanup1 := streamer.Subscribe("")
		_, cleanup2 := streamer.Subscribe("session-1")

		if streamer.SubscriberCount() != initialCount+2 {
			t.Errorf("expected %d subscribers, got %d", initialCount+2, streamer.SubscriberCount())
		}

		cleanup1()
		cleanup2()

		// Give time for cleanup
		time.Sleep(10 * time.Millisecond)

		if streamer.SubscriberCount() != initialCount {
			t.Errorf("expected %d subscribers after cleanup, got %d", initialCount, streamer.SubscriberCount())
		}
	})
}

func TestManager(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "companion-manager-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create storage
	storage, err := NewJSONLStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create streamer
	config := DefaultConfig()
	streamer := NewEventStreamer(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := streamer.Start(ctx); err != nil {
		t.Fatalf("failed to start streamer: %v", err)
	}
	defer streamer.Stop()

	// Create manager
	manager := NewManager(storage, streamer, config)

	t.Run("CreateSession", func(t *testing.T) {
		session := &Session{
			Platform: PlatformTelegram,
			UserID:   "user-123",
		}

		created, err := manager.CreateSession(ctx, session)
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		if created.ID == "" {
			t.Error("expected session ID to be generated")
		}
		if created.Status != SessionStatusActive {
			t.Errorf("expected status active, got %s", created.Status)
		}
	})

	t.Run("GetSession", func(t *testing.T) {
		sessions := manager.GetActiveSessions()
		if len(sessions) == 0 {
			t.Fatal("expected at least one active session")
		}

		session, err := manager.GetSession(ctx, sessions[0].ID)
		if err != nil {
			t.Fatalf("failed to get session: %v", err)
		}

		if session.ID != sessions[0].ID {
			t.Errorf("expected session ID %s, got %s", sessions[0].ID, session.ID)
		}
	})

	t.Run("EmitEvent", func(t *testing.T) {
		sessions := manager.GetActiveSessions()
		if len(sessions) == 0 {
			t.Fatal("expected at least one active session")
		}

		event := &SessionEvent{
			SessionID: sessions[0].ID,
			EventType: EventMessageReceived,
			Platform:  PlatformTelegram,
			UserID:    "user-123",
			Status:    "success",
			Message: &MessageEvent{
				Direction: "inbound",
				Content:   "Test message",
				Length:    12,
			},
		}

		err := manager.EmitEvent(ctx, event)
		if err != nil {
			t.Fatalf("failed to emit event: %v", err)
		}

		// Verify event was saved
		events, total, err := manager.GetSessionEvents(ctx, sessions[0].ID, nil)
		if err != nil {
			t.Fatalf("failed to get events: %v", err)
		}

		if total == 0 {
			t.Error("expected at least one event")
		}

		found := false
		for _, e := range events {
			if e.ID == event.ID {
				found = true
				break
			}
		}
		if !found {
			t.Error("emitted event not found in session events")
		}
	})

	t.Run("GetSessionFlow", func(t *testing.T) {
		sessions := manager.GetActiveSessions()
		if len(sessions) == 0 {
			t.Fatal("expected at least one active session")
		}

		flow, err := manager.GetSessionFlow(ctx, sessions[0].ID)
		if err != nil {
			t.Fatalf("failed to get session flow: %v", err)
		}

		if flow.SessionID != sessions[0].ID {
			t.Errorf("expected session ID %s, got %s", sessions[0].ID, flow.SessionID)
		}
	})

	t.Run("EndSession", func(t *testing.T) {
		sessions := manager.GetActiveSessions()
		if len(sessions) == 0 {
			t.Fatal("expected at least one active session")
		}

		sessionID := sessions[0].ID
		err := manager.EndSession(ctx, sessionID)
		if err != nil {
			t.Fatalf("failed to end session: %v", err)
		}

		// Verify session is ended
		session, err := manager.GetSession(ctx, sessionID)
		if err != nil {
			t.Fatalf("failed to get session: %v", err)
		}

		if session.Status != SessionStatusEnded {
			t.Errorf("expected status ended, got %s", session.Status)
		}
		if session.EndedAt == nil {
			t.Error("expected ended_at to be set")
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		stats, err := manager.GetStats(ctx)
		if err != nil {
			t.Fatalf("failed to get stats: %v", err)
		}

		if stats.TotalSessions == 0 {
			t.Error("expected at least one total session")
		}
	})
}

func TestThreatLevelPriority(t *testing.T) {
	tests := []struct {
		level    ThreatLevel
		expected int
	}{
		{ThreatLevelNone, 0},
		{ThreatLevelLow, 1},
		{ThreatLevelMedium, 2},
		{ThreatLevelHigh, 3},
		{ThreatLevelCritical, 4},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			result := threatLevelPriority(tt.level)
			if result != tt.expected {
				t.Errorf("expected priority %d for %s, got %d", tt.expected, tt.level, result)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 2, "hi"},
		{"hello", 5, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
