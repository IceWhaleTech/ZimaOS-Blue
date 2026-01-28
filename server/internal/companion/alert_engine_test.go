package companion

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultAlertEngine_Trigger(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	config := DefaultConfig()
	streamer := NewEventStreamer(config)
	ctx := context.Background()
	require.NoError(t, streamer.Start(ctx))
	defer streamer.Stop()

	engine := NewDefaultAlertEngine(storage, streamer, nil)

	// Start the engine
	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	// Create an alert
	alert := &Alert{
		Severity:    AlertSeverityCritical,
		Title:       "Test Alert",
		Description: "This is a test alert",
		SessionID:   "session-1",
		ThreatLevel: ThreatLevelHigh,
	}

	// Trigger the alert
	err = engine.Trigger(ctx, alert)
	require.NoError(t, err)

	// Verify alert was saved
	assert.NotEmpty(t, alert.ID)
	assert.False(t, alert.Timestamp.IsZero())

	// Retrieve the alert
	savedAlert, err := storage.GetAlert(ctx, alert.ID)
	require.NoError(t, err)
	assert.Equal(t, alert.Title, savedAlert.Title)
	assert.Equal(t, alert.Severity, savedAlert.Severity)
}

func TestDefaultAlertEngine_Deduplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-dedup-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	ctx := context.Background()
	require.NoError(t, streamer.Start(ctx))
	defer streamer.Stop()

	config := &AlertEngineConfig{
		ThreatThreshold:     ThreatLevelMedium,
		DeduplicationWindow: 5 * time.Minute,
		EnableWebSocket:     false,
	}
	engine := NewDefaultAlertEngine(storage, streamer, config)

	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	// Create first alert
	alert1 := &Alert{
		Severity:    AlertSeverityHigh,
		Title:       "Duplicate Alert",
		SessionID:   "session-1",
		ThreatLevel: ThreatLevelHigh,
	}
	err = engine.Trigger(ctx, alert1)
	require.NoError(t, err)

	// Create duplicate alert (same session, severity, title)
	alert2 := &Alert{
		Severity:    AlertSeverityHigh,
		Title:       "Duplicate Alert",
		SessionID:   "session-1",
		ThreatLevel: ThreatLevelHigh,
	}
	err = engine.Trigger(ctx, alert2)
	require.NoError(t, err)

	// Only one alert should be saved (duplicate should be skipped)
	alerts, total, err := storage.ListAlerts(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, alerts, 1)
}

func TestDefaultAlertEngine_CheckEvent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-check-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	ctx := context.Background()
	require.NoError(t, streamer.Start(ctx))
	defer streamer.Stop()

	config := &AlertEngineConfig{
		ThreatThreshold:     ThreatLevelMedium,
		DeduplicationWindow: 5 * time.Minute,
		EnableWebSocket:     false,
	}
	engine := NewDefaultAlertEngine(storage, streamer, config)

	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	// Event without security data should not trigger alert
	event1 := &SessionEvent{
		ID:        "event-1",
		SessionID: "session-1",
		EventType: EventMessageReceived,
	}
	err = engine.CheckEvent(ctx, event1)
	require.NoError(t, err)

	alerts, total, _ := storage.ListAlerts(ctx, nil)
	assert.Equal(t, 0, total)
	assert.Len(t, alerts, 0)

	// Event with low threat level should not trigger alert (below threshold)
	event2 := &SessionEvent{
		ID:        "event-2",
		SessionID: "session-1",
		EventType: EventSecurityThreat,
		Security: &SecurityEvent{
			ThreatLevel: ThreatLevelLow,
			ThreatScore: 20,
			ThreatTypes: []string{"suspicious"},
		},
	}
	err = engine.CheckEvent(ctx, event2)
	require.NoError(t, err)

	alerts, total, _ = storage.ListAlerts(ctx, nil)
	assert.Equal(t, 0, total)

	// Event with high threat level should trigger alert
	event3 := &SessionEvent{
		ID:        "event-3",
		SessionID: "session-1",
		EventType: EventSecurityThreat,
		Security: &SecurityEvent{
			ThreatLevel: ThreatLevelHigh,
			ThreatScore: 80,
			ThreatTypes: []string{"injection", "malicious"},
		},
	}
	err = engine.CheckEvent(ctx, event3)
	require.NoError(t, err)

	alerts, total, _ = storage.ListAlerts(ctx, nil)
	assert.Equal(t, 1, total)
	assert.Len(t, alerts, 1)
	assert.Equal(t, AlertSeverityHigh, alerts[0].Severity)
}

func TestDefaultAlertEngine_AcknowledgeAlert(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-ack-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	ctx := context.Background()
	require.NoError(t, streamer.Start(ctx))
	defer streamer.Stop()

	engine := NewDefaultAlertEngine(storage, streamer, nil)

	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	// Create and trigger an alert
	alert := &Alert{
		Severity:    AlertSeverityWarning,
		Title:       "Test Alert",
		SessionID:   "session-1",
		ThreatLevel: ThreatLevelMedium,
	}
	err = engine.Trigger(ctx, alert)
	require.NoError(t, err)

	// Acknowledge the alert
	acked, err := engine.AcknowledgeAlert(ctx, alert.ID, "user-1")
	require.NoError(t, err)
	assert.True(t, acked.Acknowledged)
	assert.NotNil(t, acked.AckedAt)
	assert.Equal(t, "user-1", acked.AckedBy)

	// Verify in storage
	savedAlert, err := storage.GetAlert(ctx, alert.ID)
	require.NoError(t, err)
	assert.True(t, savedAlert.Acknowledged)
}

func TestDefaultAlertEngine_BulkAcknowledge(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-bulk-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	ctx := context.Background()
	require.NoError(t, streamer.Start(ctx))
	defer streamer.Stop()

	engine := NewDefaultAlertEngine(storage, streamer, nil)

	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	// Create multiple alerts with unique dedup keys
	alertIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		alert := &Alert{
			Severity:    AlertSeverityInfo,
			Title:       "Bulk Test Alert",
			SessionID:   "session-bulk-" + string(rune('a'+i)),
			ThreatLevel: ThreatLevelLow,
		}
		err = engine.Trigger(ctx, alert)
		require.NoError(t, err)
		alertIDs[i] = alert.ID
	}

	// Bulk acknowledge
	count, err := engine.BulkAcknowledge(ctx, alertIDs, "admin")
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify all are acknowledged
	for _, id := range alertIDs {
		alert, err := storage.GetAlert(ctx, id)
		require.NoError(t, err)
		assert.True(t, alert.Acknowledged)
		assert.Equal(t, "admin", alert.AckedBy)
	}
}

func TestDefaultAlertEngine_GetAlerts(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-list-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	ctx := context.Background()
	require.NoError(t, streamer.Start(ctx))
	defer streamer.Stop()

	engine := NewDefaultAlertEngine(storage, streamer, nil)

	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()

	// Create alerts with different severities
	severities := []AlertSeverity{AlertSeverityCritical, AlertSeverityHigh, AlertSeverityWarning, AlertSeverityInfo}
	for i, sev := range severities {
		alert := &Alert{
			Severity:    sev,
			Title:       "Test Alert",
			SessionID:   "session-" + string(rune('a'+i)),
			ThreatLevel: ThreatLevelMedium,
		}
		err = engine.Trigger(ctx, alert)
		require.NoError(t, err)
	}

	// Get all alerts
	alerts, total, err := engine.GetAlerts(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 4, total)
	assert.Len(t, alerts, 4)

	// Get with limit
	alerts, total, err = engine.GetAlerts(ctx, &ListOptions{Limit: 2})
	require.NoError(t, err)
	assert.Equal(t, 4, total)
	assert.Len(t, alerts, 2)
}

func TestDefaultAlertEngine_ThreatLevelToSeverity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-severity-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	engine := NewDefaultAlertEngine(storage, streamer, nil)

	tests := []struct {
		threatLevel ThreatLevel
		expected    AlertSeverity
	}{
		{ThreatLevelCritical, AlertSeverityCritical},
		{ThreatLevelHigh, AlertSeverityHigh},
		{ThreatLevelMedium, AlertSeverityWarning},
		{ThreatLevelLow, AlertSeverityInfo},
		{ThreatLevelNone, AlertSeverityInfo},
	}

	for _, tt := range tests {
		t.Run(string(tt.threatLevel), func(t *testing.T) {
			result := engine.threatLevelToSeverity(tt.threatLevel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultAlertEngine_MeetsThreshold(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-threshold-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	config := &AlertEngineConfig{
		ThreatThreshold: ThreatLevelMedium,
	}
	engine := NewDefaultAlertEngine(storage, streamer, config)

	tests := []struct {
		level    ThreatLevel
		expected bool
	}{
		{ThreatLevelCritical, true},
		{ThreatLevelHigh, true},
		{ThreatLevelMedium, true},
		{ThreatLevelLow, false},
		{ThreatLevelNone, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			result := engine.meetsThreshold(tt.level)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultAlertEngine_CleanupDedupCache(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-cleanup-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	config := &AlertEngineConfig{
		ThreatThreshold:     ThreatLevelMedium,
		DeduplicationWindow: 100 * time.Millisecond,
		EnableWebSocket:     false,
	}
	engine := NewDefaultAlertEngine(storage, streamer, config)

	// Add entry to dedup cache
	engine.updateDedupCache("test-key")

	// Verify it's in cache
	assert.True(t, engine.isDuplicate("test-key"))

	// Wait for dedup window to expire
	time.Sleep(150 * time.Millisecond)

	// Cleanup
	engine.CleanupDedupCache()

	// Should no longer be duplicate
	assert.False(t, engine.isDuplicate("test-key"))
}

func TestDefaultAlertEngine_StartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "alert-engine-startstop-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	storage, err := NewJSONLStorage(tmpDir)
	require.NoError(t, err)
	defer storage.Close()

	cfg := DefaultConfig()
	streamer := NewEventStreamer(cfg)
	engine := NewDefaultAlertEngine(storage, streamer, nil)

	ctx := context.Background()

	// Start
	err = engine.Start(ctx)
	require.NoError(t, err)
	assert.True(t, engine.running)

	// Start again (should be no-op)
	err = engine.Start(ctx)
	require.NoError(t, err)

	// Stop
	err = engine.Stop()
	require.NoError(t, err)
	assert.False(t, engine.running)

	// Stop again (should be no-op)
	err = engine.Stop()
	require.NoError(t, err)
}
