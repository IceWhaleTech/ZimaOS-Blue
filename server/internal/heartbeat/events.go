package heartbeat

import (
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/companion"
)

// IndicatorType represents the heartbeat status indicator for UI.
type IndicatorType string

const (
	IndicatorOK    IndicatorType = "ok"
	IndicatorAlert IndicatorType = "alert"
	IndicatorError IndicatorType = "error"
)

// HeartbeatEvent represents the outcome of a single heartbeat run.
type HeartbeatEvent struct {
	Timestamp     time.Time     `json:"timestamp"`
	Status        string        `json:"status"` // sent, ok-empty, ok-token, skipped, failed
	Channel       string        `json:"channel,omitempty"`
	Preview       string        `json:"preview,omitempty"`
	DurationMs    int64         `json:"duration_ms,omitempty"`
	Reason        string        `json:"reason,omitempty"`
	IndicatorType IndicatorType `json:"indicator_type,omitempty"`
	Silent        bool          `json:"silent,omitempty"`
}

// ResolveIndicator maps a heartbeat status to an indicator type.
func ResolveIndicator(status string) IndicatorType {
	switch status {
	case "ok-empty", "ok-token":
		return IndicatorOK
	case "sent":
		return IndicatorAlert
	case "failed":
		return IndicatorError
	default:
		return ""
	}
}

// EmitEvent publishes a heartbeat event to the companion event streamer.
// If streamer is nil, the event is silently dropped.
func EmitEvent(streamer *companion.EventStreamer, evt HeartbeatEvent) {
	if streamer == nil {
		return
	}

	preview := evt.Preview
	if len(preview) > 200 {
		preview = preview[:200]
	}

	sessionEvt := &companion.SessionEvent{
		ID:        "hb-" + evt.Timestamp.Format("20060102-150405"),
		SessionID: "heartbeat",
		EventType: companion.EventHeartbeat,
		Timestamp: evt.Timestamp,
		Status:    evt.Status,
		Duration:  companion.DurationMs(evt.DurationMs),
		Message: &companion.MessageEvent{
			Direction:   "outbound",
			Content:     preview,
			ContentType: "heartbeat",
			Length:      len(preview),
		},
	}

	streamer.Emit(sessionEvt)
}
