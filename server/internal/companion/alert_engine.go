package companion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// DefaultAlertEngine is the default implementation of the AlertEngine interface.
type DefaultAlertEngine struct {
	mu       sync.RWMutex
	storage  Storage
	streamer Streamer
	config   *AlertEngineConfig

	// Deduplication cache: key -> last alert time
	dedupCache map[string]time.Time

	// Notification channels
	webhookClient *http.Client

	// Running state
	running bool
	stopCh  chan struct{}
}

// AlertEngineConfig holds configuration for the alert engine.
type AlertEngineConfig struct {
	// ThreatThreshold is the minimum threat level to trigger an alert
	ThreatThreshold ThreatLevel `json:"threat_threshold"`

	// DeduplicationWindow is the time window for alert deduplication
	DeduplicationWindow time.Duration `json:"deduplication_window"`

	// WebhookURL is the URL to send webhook notifications
	WebhookURL string `json:"webhook_url,omitempty"`

	// WebhookSecret is the secret for webhook authentication
	WebhookSecret string `json:"webhook_secret,omitempty"`

	// EnableWebSocket enables real-time WebSocket alert push
	EnableWebSocket bool `json:"enable_websocket"`
}

// NewDefaultAlertEngine creates a new default alert engine.
func NewDefaultAlertEngine(storage Storage, streamer Streamer, config *AlertEngineConfig) *DefaultAlertEngine {
	if config == nil {
		config = &AlertEngineConfig{
			ThreatThreshold:     ThreatLevelMedium,
			DeduplicationWindow: 5 * time.Minute,
			EnableWebSocket:     true,
		}
	}

	return &DefaultAlertEngine{
		storage:    storage,
		streamer:   streamer,
		config:     config,
		dedupCache: make(map[string]time.Time),
		webhookClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

// Start starts the alert engine.
func (e *DefaultAlertEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}
	e.running = true
	e.mu.Unlock()

	// Start cleanup goroutine
	go e.cleanupLoop(ctx)

	return nil
}

// Stop stops the alert engine.
func (e *DefaultAlertEngine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	e.running = false
	close(e.stopCh)
	return nil
}

// cleanupLoop periodically cleans up the deduplication cache.
func (e *DefaultAlertEngine) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.CleanupDedupCache()
		}
	}
}

// Trigger creates and emits a new alert.
func (e *DefaultAlertEngine) Trigger(ctx context.Context, alert *Alert) error {
	// Generate ID if not set
	if alert.ID == "" {
		alert.ID = uuid.New().String()
	}

	// Set timestamp if not set
	if alert.Timestamp.IsZero() {
		alert.Timestamp = timeutil.NowTime()
	}

	// Check deduplication
	dedupKey := e.getDedupKey(alert)
	if e.isDuplicate(dedupKey) {
		return nil // Skip duplicate alert
	}

	// Store alert
	if err := e.storage.SaveAlert(ctx, alert); err != nil {
		return fmt.Errorf("failed to save alert: %w", err)
	}

	// Update dedup cache
	e.updateDedupCache(dedupKey)

	// Send notifications
	go e.sendNotifications(alert)

	return nil
}

// CheckEvent checks if an event should trigger an alert.
func (e *DefaultAlertEngine) CheckEvent(ctx context.Context, event *SessionEvent) error {
	// Check if event has security data
	if event.Security == nil {
		return nil
	}

	// Check threat threshold
	if !e.meetsThreshold(event.Security.ThreatLevel) {
		return nil
	}

	// Create alert from event
	alert := &Alert{
		ID:          uuid.New().String(),
		Severity:    e.threatLevelToSeverity(event.Security.ThreatLevel),
		Title:       fmt.Sprintf("Security threat detected: %s", event.Security.ThreatLevel),
		Description: fmt.Sprintf("Threat types: %v", event.Security.ThreatTypes),
		SessionID:   event.SessionID,
		EventID:     event.ID,
		ThreatLevel: event.Security.ThreatLevel,
		Details: map[string]interface{}{
			"threat_score":      event.Security.ThreatScore,
			"threat_types":      event.Security.ThreatTypes,
			"detected_patterns": event.Security.DetectedPatterns,
			"action":            event.Security.Action,
		},
		Timestamp: timeutil.NowTime(),
	}

	return e.Trigger(ctx, alert)
}

// GetAlerts returns alerts with optional filtering.
func (e *DefaultAlertEngine) GetAlerts(ctx context.Context, opts *ListOptions) ([]*Alert, int, error) {
	return e.storage.ListAlerts(ctx, opts)
}

// AcknowledgeAlert marks an alert as acknowledged.
func (e *DefaultAlertEngine) AcknowledgeAlert(ctx context.Context, alertID, userID string) (*Alert, error) {
	alert, err := e.storage.GetAlert(ctx, alertID)
	if err != nil {
		return nil, err
	}

	now := timeutil.NowTime()
	alert.Acknowledged = true
	alert.AckedAt = &now
	alert.AckedBy = userID

	if err := e.storage.UpdateAlert(ctx, alert); err != nil {
		return nil, err
	}

	return alert, nil
}

// BulkAcknowledge acknowledges multiple alerts at once.
func (e *DefaultAlertEngine) BulkAcknowledge(ctx context.Context, alertIDs []string, userID string) (int, error) {
	count := 0
	for _, id := range alertIDs {
		_, err := e.AcknowledgeAlert(ctx, id, userID)
		if err == nil {
			count++
		}
	}
	return count, nil
}

// getDedupKey generates a deduplication key for an alert.
func (e *DefaultAlertEngine) getDedupKey(alert *Alert) string {
	return fmt.Sprintf("%s:%s:%s", alert.SessionID, alert.Severity, alert.Title)
}

// isDuplicate checks if an alert is a duplicate within the deduplication window.
func (e *DefaultAlertEngine) isDuplicate(key string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	lastTime, exists := e.dedupCache[key]
	if !exists {
		return false
	}

	return timeutil.SinceTime(lastTime) < e.config.DeduplicationWindow
}

// updateDedupCache updates the deduplication cache.
func (e *DefaultAlertEngine) updateDedupCache(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.dedupCache[key] = timeutil.NowTime()

	// Clean old entries
	cutoff := timeutil.NowTime().Add(-e.config.DeduplicationWindow * 2)
	for k, t := range e.dedupCache {
		if t.Before(cutoff) {
			delete(e.dedupCache, k)
		}
	}
}

// meetsThreshold checks if a threat level meets the configured threshold.
func (e *DefaultAlertEngine) meetsThreshold(level ThreatLevel) bool {
	levelPriority := map[ThreatLevel]int{
		ThreatLevelNone:     0,
		ThreatLevelLow:      1,
		ThreatLevelMedium:   2,
		ThreatLevelHigh:     3,
		ThreatLevelCritical: 4,
	}

	return levelPriority[level] >= levelPriority[e.config.ThreatThreshold]
}

// threatLevelToSeverity converts a threat level to an alert severity.
func (e *DefaultAlertEngine) threatLevelToSeverity(level ThreatLevel) AlertSeverity {
	switch level {
	case ThreatLevelCritical:
		return AlertSeverityCritical
	case ThreatLevelHigh:
		return AlertSeverityHigh
	case ThreatLevelMedium:
		return AlertSeverityWarning
	default:
		return AlertSeverityInfo
	}
}

// sendNotifications sends alert notifications through configured channels.
func (e *DefaultAlertEngine) sendNotifications(alert *Alert) {
	// WebSocket notification
	if e.config.EnableWebSocket && e.streamer != nil {
		e.sendWebSocketNotification(alert)
	}

	// Webhook notification
	if e.config.WebhookURL != "" {
		e.sendWebhookNotification(alert)
	}
}

// sendWebSocketNotification sends an alert via WebSocket.
func (e *DefaultAlertEngine) sendWebSocketNotification(alert *Alert) {
	// Create a special alert event
	event := &SessionEvent{
		ID:        uuid.New().String(),
		SessionID: alert.SessionID,
		Timestamp: alert.Timestamp,
		EventType: "alert",
		Status:    string(alert.Severity),
	}

	e.streamer.Emit(event)
}

// sendWebhookNotification sends an alert via webhook.
func (e *DefaultAlertEngine) sendWebhookNotification(alert *Alert) {
	payload := map[string]interface{}{
		"type":      "companion_alert",
		"alert":     alert,
		"timestamp": timeutil.NowTime().Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequest(http.MethodPost, e.config.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", "application/json")
	if e.config.WebhookSecret != "" {
		req.Header.Set("X-Webhook-Secret", e.config.WebhookSecret)
	}

	resp, err := e.webhookClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// CleanupDedupCache removes expired entries from the deduplication cache.
func (e *DefaultAlertEngine) CleanupDedupCache() {
	e.mu.Lock()
	defer e.mu.Unlock()

	cutoff := timeutil.NowTime().Add(-e.config.DeduplicationWindow)
	for k, t := range e.dedupCache {
		if t.Before(cutoff) {
			delete(e.dedupCache, k)
		}
	}
}

// Ensure DefaultAlertEngine implements AlertEngine interface
var _ AlertEngine = (*DefaultAlertEngine)(nil)
