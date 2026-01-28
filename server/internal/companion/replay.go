package companion

import (
	"context"
	"sort"
	"time"
)

// ReplayService provides session replay functionality.
type ReplayService struct {
	storage Storage
}

// NewReplayService creates a new replay service.
func NewReplayService(storage Storage) *ReplayService {
	return &ReplayService{
		storage: storage,
	}
}

// TimelineEvent represents an event in the replay timeline.
type TimelineEvent struct {
	*SessionEvent

	// RelativeTime is the time offset from session start in milliseconds
	RelativeTime int64 `json:"relative_time"`

	// Index is the event's position in the timeline
	Index int `json:"index"`
}

// Timeline represents a session's event timeline for replay.
type Timeline struct {
	SessionID   string           `json:"session_id"`
	StartTime   time.Time        `json:"start_time"`
	EndTime     time.Time        `json:"end_time"`
	Duration    time.Duration    `json:"duration"`
	TotalEvents int              `json:"total_events"`
	Events      []*TimelineEvent `json:"events"`
}

// PlaybackMetadata contains metadata for replay playback.
type PlaybackMetadata struct {
	// SupportedSpeeds lists available playback speeds
	SupportedSpeeds []float64 `json:"supported_speeds"`

	// DefaultSpeed is the default playback speed
	DefaultSpeed float64 `json:"default_speed"`

	// MinEventGap is the minimum gap between events in milliseconds
	MinEventGap int64 `json:"min_event_gap"`

	// MaxEventGap is the maximum gap between events in milliseconds (for capping long waits)
	MaxEventGap int64 `json:"max_event_gap"`
}

// GetSessionTimeline retrieves the complete timeline for a session.
func (r *ReplayService) GetSessionTimeline(ctx context.Context, sessionID string) (*Timeline, error) {
	// Get all events for the session
	events, _, err := r.storage.GetSessionEvents(ctx, sessionID, &ListOptions{
		Limit: 10000, // Get all events
		Sort:  "timestamp",
		Order: "asc",
	})
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return &Timeline{
			SessionID:   sessionID,
			TotalEvents: 0,
			Events:      []*TimelineEvent{},
		}, nil
	}

	// Sort events by timestamp
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Calculate timeline
	startTime := events[0].Timestamp
	endTime := events[len(events)-1].Timestamp

	// Convert to timeline events
	timelineEvents := make([]*TimelineEvent, len(events))
	for i, event := range events {
		timelineEvents[i] = &TimelineEvent{
			SessionEvent: event,
			RelativeTime: event.Timestamp.Sub(startTime).Milliseconds(),
			Index:        i,
		}
	}

	return &Timeline{
		SessionID:   sessionID,
		StartTime:   startTime,
		EndTime:     endTime,
		Duration:    endTime.Sub(startTime),
		TotalEvents: len(events),
		Events:      timelineEvents,
	}, nil
}

// GetPlaybackMetadata returns metadata for replay playback.
func (r *ReplayService) GetPlaybackMetadata() *PlaybackMetadata {
	return &PlaybackMetadata{
		SupportedSpeeds: []float64{0.25, 0.5, 1.0, 1.5, 2.0, 4.0},
		DefaultSpeed:    1.0,
		MinEventGap:     100,  // 100ms minimum between events
		MaxEventGap:     5000, // Cap gaps at 5 seconds
	}
}

// GetEventAtTime returns the event at or before the specified relative time.
func (r *ReplayService) GetEventAtTime(timeline *Timeline, relativeTimeMs int64) *TimelineEvent {
	if len(timeline.Events) == 0 {
		return nil
	}

	// Binary search for the event at or before the given time
	idx := sort.Search(len(timeline.Events), func(i int) bool {
		return timeline.Events[i].RelativeTime > relativeTimeMs
	})

	if idx == 0 {
		return timeline.Events[0]
	}

	return timeline.Events[idx-1]
}

// GetEventsInRange returns events within a time range.
func (r *ReplayService) GetEventsInRange(timeline *Timeline, startMs, endMs int64) []*TimelineEvent {
	var result []*TimelineEvent

	for _, event := range timeline.Events {
		if event.RelativeTime >= startMs && event.RelativeTime <= endMs {
			result = append(result, event)
		}
	}

	return result
}

// NormalizeTimeline adjusts event timestamps to remove long gaps.
func (r *ReplayService) NormalizeTimeline(timeline *Timeline, maxGapMs int64) *Timeline {
	if len(timeline.Events) <= 1 {
		return timeline
	}

	normalized := &Timeline{
		SessionID:   timeline.SessionID,
		StartTime:   timeline.StartTime,
		TotalEvents: timeline.TotalEvents,
		Events:      make([]*TimelineEvent, len(timeline.Events)),
	}

	// First event stays at 0
	normalized.Events[0] = &TimelineEvent{
		SessionEvent: timeline.Events[0].SessionEvent,
		RelativeTime: 0,
		Index:        0,
	}

	currentTime := int64(0)
	for i := 1; i < len(timeline.Events); i++ {
		gap := timeline.Events[i].RelativeTime - timeline.Events[i-1].RelativeTime

		// Cap the gap
		if gap > maxGapMs {
			gap = maxGapMs
		}

		currentTime += gap
		normalized.Events[i] = &TimelineEvent{
			SessionEvent: timeline.Events[i].SessionEvent,
			RelativeTime: currentTime,
			Index:        i,
		}
	}

	// Update duration
	if len(normalized.Events) > 0 {
		lastEvent := normalized.Events[len(normalized.Events)-1]
		normalized.Duration = time.Duration(lastEvent.RelativeTime) * time.Millisecond
		normalized.EndTime = normalized.StartTime.Add(normalized.Duration)
	}

	return normalized
}

// GetEventsByType filters timeline events by type.
func (r *ReplayService) GetEventsByType(timeline *Timeline, eventType SessionEventType) []*TimelineEvent {
	var result []*TimelineEvent

	for _, event := range timeline.Events {
		if event.EventType == eventType {
			result = append(result, event)
		}
	}

	return result
}

// GetKeyEvents returns significant events (session start/end, security threats).
func (r *ReplayService) GetKeyEvents(timeline *Timeline) []*TimelineEvent {
	var result []*TimelineEvent

	keyTypes := map[SessionEventType]bool{
		EventSessionStart:   true,
		EventSessionEnd:     true,
		EventSecurityThreat: true,
	}

	for _, event := range timeline.Events {
		if keyTypes[event.EventType] {
			result = append(result, event)
		}
	}

	return result
}
