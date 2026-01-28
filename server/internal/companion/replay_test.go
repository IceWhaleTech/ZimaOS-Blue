package companion

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplayService_GetSessionTimeline(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)
	ctx := context.Background()

	// Create a session
	session := &Session{
		ID:        "session-1",
		Platform:  PlatformWeb,
		UserID:    "user-1",
		Status:    SessionStatusActive,
		StartedAt: time.Now(),
	}
	err = storage.SaveSession(ctx, session)
	require.NoError(t, err)

	// Create events with different timestamps
	baseTime := time.Now()
	events := []*SessionEvent{
		{
			ID:        "event-1",
			SessionID: "session-1",
			Timestamp: baseTime,
			EventType: EventSessionStart,
		},
		{
			ID:        "event-2",
			SessionID: "session-1",
			Timestamp: baseTime.Add(1 * time.Second),
			EventType: EventMessageReceived,
		},
		{
			ID:        "event-3",
			SessionID: "session-1",
			Timestamp: baseTime.Add(2 * time.Second),
			EventType: EventToolCall,
		},
		{
			ID:        "event-4",
			SessionID: "session-1",
			Timestamp: baseTime.Add(3 * time.Second),
			EventType: EventMessageSent,
		},
	}

	for _, event := range events {
		err := storage.AppendEvent(ctx, event)
		require.NoError(t, err)
	}

	// Get timeline
	timeline, err := service.GetSessionTimeline(ctx, "session-1")
	require.NoError(t, err)

	assert.Equal(t, "session-1", timeline.SessionID)
	assert.Equal(t, 4, timeline.TotalEvents)
	assert.Len(t, timeline.Events, 4)

	// Verify relative times
	assert.Equal(t, int64(0), timeline.Events[0].RelativeTime)
	assert.Equal(t, int64(1000), timeline.Events[1].RelativeTime)
	assert.Equal(t, int64(2000), timeline.Events[2].RelativeTime)
	assert.Equal(t, int64(3000), timeline.Events[3].RelativeTime)

	// Verify indices
	for i, event := range timeline.Events {
		assert.Equal(t, i, event.Index)
	}
}

func TestReplayService_GetSessionTimeline_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-empty-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)
	ctx := context.Background()

	// Create a session with no events
	session := &Session{
		ID:        "non-existent",
		Platform:  PlatformWeb,
		UserID:    "user-1",
		Status:    SessionStatusActive,
		StartedAt: time.Now(),
	}
	err = storage.SaveSession(ctx, session)
	require.NoError(t, err)

	// Get timeline for session with no events
	timeline, err := service.GetSessionTimeline(ctx, "non-existent")
	require.NoError(t, err)

	assert.Equal(t, "non-existent", timeline.SessionID)
	assert.Equal(t, 0, timeline.TotalEvents)
	assert.Empty(t, timeline.Events)
}

func TestReplayService_GetPlaybackMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-metadata-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)

	metadata := service.GetPlaybackMetadata()

	assert.Contains(t, metadata.SupportedSpeeds, 1.0)
	assert.Contains(t, metadata.SupportedSpeeds, 0.5)
	assert.Contains(t, metadata.SupportedSpeeds, 2.0)
	assert.Equal(t, 1.0, metadata.DefaultSpeed)
	assert.Equal(t, int64(100), metadata.MinEventGap)
	assert.Equal(t, int64(5000), metadata.MaxEventGap)
}

func TestReplayService_GetEventAtTime(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-eventattime-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)

	// Create a timeline manually
	timeline := &Timeline{
		SessionID: "session-1",
		Events: []*TimelineEvent{
			{SessionEvent: &SessionEvent{ID: "e1"}, RelativeTime: 0, Index: 0},
			{SessionEvent: &SessionEvent{ID: "e2"}, RelativeTime: 1000, Index: 1},
			{SessionEvent: &SessionEvent{ID: "e3"}, RelativeTime: 2000, Index: 2},
			{SessionEvent: &SessionEvent{ID: "e4"}, RelativeTime: 3000, Index: 3},
		},
	}

	tests := []struct {
		name       string
		timeMs     int64
		expectedID string
	}{
		{"at start", 0, "e1"},
		{"before first", -100, "e1"},
		{"between events", 500, "e1"},
		{"at second event", 1000, "e2"},
		{"between second and third", 1500, "e2"},
		{"at last event", 3000, "e4"},
		{"after last event", 5000, "e4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := service.GetEventAtTime(timeline, tt.timeMs)
			require.NotNil(t, event)
			assert.Equal(t, tt.expectedID, event.ID)
		})
	}
}

func TestReplayService_GetEventAtTime_Empty(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-eventattime-empty-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)

	timeline := &Timeline{
		SessionID: "session-1",
		Events:    []*TimelineEvent{},
	}

	event := service.GetEventAtTime(timeline, 1000)
	assert.Nil(t, event)
}

func TestReplayService_GetEventsInRange(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-eventsinrange-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)

	timeline := &Timeline{
		SessionID: "session-1",
		Events: []*TimelineEvent{
			{SessionEvent: &SessionEvent{ID: "e1"}, RelativeTime: 0, Index: 0},
			{SessionEvent: &SessionEvent{ID: "e2"}, RelativeTime: 1000, Index: 1},
			{SessionEvent: &SessionEvent{ID: "e3"}, RelativeTime: 2000, Index: 2},
			{SessionEvent: &SessionEvent{ID: "e4"}, RelativeTime: 3000, Index: 3},
			{SessionEvent: &SessionEvent{ID: "e5"}, RelativeTime: 4000, Index: 4},
		},
	}

	tests := []struct {
		name     string
		startMs  int64
		endMs    int64
		expected []string
	}{
		{"full range", 0, 4000, []string{"e1", "e2", "e3", "e4", "e5"}},
		{"middle range", 1000, 3000, []string{"e2", "e3", "e4"}},
		{"single event", 2000, 2000, []string{"e3"}},
		{"no events", 500, 900, []string{}},
		{"partial overlap", 1500, 2500, []string{"e3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := service.GetEventsInRange(timeline, tt.startMs, tt.endMs)
			assert.Len(t, events, len(tt.expected))
			for i, event := range events {
				assert.Equal(t, tt.expected[i], event.ID)
			}
		})
	}
}

func TestReplayService_NormalizeTimeline(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-normalize-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)

	// Create timeline with large gaps
	timeline := &Timeline{
		SessionID: "session-1",
		StartTime: time.Now(),
		Events: []*TimelineEvent{
			{SessionEvent: &SessionEvent{ID: "e1"}, RelativeTime: 0, Index: 0},
			{SessionEvent: &SessionEvent{ID: "e2"}, RelativeTime: 1000, Index: 1},
			{SessionEvent: &SessionEvent{ID: "e3"}, RelativeTime: 60000, Index: 2}, // 59 second gap
			{SessionEvent: &SessionEvent{ID: "e4"}, RelativeTime: 61000, Index: 3},
		},
	}

	// Normalize with 5 second max gap
	normalized := service.NormalizeTimeline(timeline, 5000)

	assert.Equal(t, int64(0), normalized.Events[0].RelativeTime)
	assert.Equal(t, int64(1000), normalized.Events[1].RelativeTime)
	assert.Equal(t, int64(6000), normalized.Events[2].RelativeTime) // Gap capped to 5000
	assert.Equal(t, int64(7000), normalized.Events[3].RelativeTime)
}

func TestReplayService_NormalizeTimeline_SingleEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-normalize-single-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)

	timeline := &Timeline{
		SessionID: "session-1",
		Events: []*TimelineEvent{
			{SessionEvent: &SessionEvent{ID: "e1"}, RelativeTime: 0, Index: 0},
		},
	}

	normalized := service.NormalizeTimeline(timeline, 5000)
	assert.Equal(t, timeline, normalized) // Should return same timeline
}

func TestReplayService_GetEventsByType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-eventsbytype-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)

	timeline := &Timeline{
		SessionID: "session-1",
		Events: []*TimelineEvent{
			{SessionEvent: &SessionEvent{ID: "e1", EventType: EventSessionStart}, RelativeTime: 0},
			{SessionEvent: &SessionEvent{ID: "e2", EventType: EventMessageReceived}, RelativeTime: 1000},
			{SessionEvent: &SessionEvent{ID: "e3", EventType: EventToolCall}, RelativeTime: 2000},
			{SessionEvent: &SessionEvent{ID: "e4", EventType: EventMessageReceived}, RelativeTime: 3000},
			{SessionEvent: &SessionEvent{ID: "e5", EventType: EventSessionEnd}, RelativeTime: 4000},
		},
	}

	// Get message events
	messages := service.GetEventsByType(timeline, EventMessageReceived)
	assert.Len(t, messages, 2)
	assert.Equal(t, "e2", messages[0].ID)
	assert.Equal(t, "e4", messages[1].ID)

	// Get tool calls
	toolCalls := service.GetEventsByType(timeline, EventToolCall)
	assert.Len(t, toolCalls, 1)
	assert.Equal(t, "e3", toolCalls[0].ID)

	// Get non-existent type
	threats := service.GetEventsByType(timeline, EventSecurityThreat)
	assert.Empty(t, threats)
}

func TestReplayService_GetKeyEvents(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "replay-keyevents-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	service := NewReplayService(storage)

	timeline := &Timeline{
		SessionID: "session-1",
		Events: []*TimelineEvent{
			{SessionEvent: &SessionEvent{ID: "e1", EventType: EventSessionStart}, RelativeTime: 0},
			{SessionEvent: &SessionEvent{ID: "e2", EventType: EventMessageReceived}, RelativeTime: 1000},
			{SessionEvent: &SessionEvent{ID: "e3", EventType: EventSecurityThreat}, RelativeTime: 2000},
			{SessionEvent: &SessionEvent{ID: "e4", EventType: EventToolCall}, RelativeTime: 3000},
			{SessionEvent: &SessionEvent{ID: "e5", EventType: EventSessionEnd}, RelativeTime: 4000},
		},
	}

	keyEvents := service.GetKeyEvents(timeline)
	assert.Len(t, keyEvents, 3)

	// Should include session start, security threat, and session end
	ids := make([]string, len(keyEvents))
	for i, e := range keyEvents {
		ids[i] = e.ID
	}
	assert.Contains(t, ids, "e1") // session start
	assert.Contains(t, ids, "e3") // security threat
	assert.Contains(t, ids, "e5") // session end
}
