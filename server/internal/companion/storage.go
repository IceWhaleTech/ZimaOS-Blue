package companion

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// JSONLStorage implements the Storage interface using JSONL files.
type JSONLStorage struct {
	basePath string
	mu       sync.RWMutex

	// In-memory caches for active data
	sessions     map[string]*Session
	sessionIndex []string // sorted by start time
	alerts       map[string]*Alert
	alertIndex   []string // sorted by timestamp
}

// NewJSONLStorage creates a new JSONL storage instance.
func NewJSONLStorage(basePath string) (*JSONLStorage, error) {
	s := &JSONLStorage{
		basePath: basePath,
		sessions: make(map[string]*Session),
		alerts:   make(map[string]*Alert),
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(basePath, "sessions"),
		filepath.Join(basePath, "alerts"),
		filepath.Join(basePath, "stats"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Load existing sessions and alerts into memory
	if err := s.loadSessions(); err != nil {
		return nil, fmt.Errorf("failed to load sessions: %w", err)
	}
	if err := s.loadAlerts(); err != nil {
		return nil, fmt.Errorf("failed to load alerts: %w", err)
	}

	return s, nil
}

// sessionDir returns the directory path for a session based on its start date.
func (s *JSONLStorage) sessionDir(t time.Time) string {
	return filepath.Join(s.basePath, "sessions", t.Format("2006-01-02"))
}

// sessionFile returns the file path for a session.
func (s *JSONLStorage) sessionFile(session *Session) string {
	return filepath.Join(s.sessionDir(session.StartedAt), fmt.Sprintf("session-%s.jsonl", session.ID))
}

// alertFile returns the file path for alerts on a given date.
func (s *JSONLStorage) alertFile(t time.Time) string {
	return filepath.Join(s.basePath, "alerts", fmt.Sprintf("%s.jsonl", t.Format("2006-01-02")))
}

// statsFile returns the file path for daily stats.
func (s *JSONLStorage) statsFile() string {
	return filepath.Join(s.basePath, "stats", "daily-stats.json")
}

// SaveSession saves a session to storage.
func (s *JSONLStorage) SaveSession(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create session directory if needed
	dir := s.sessionDir(session.StartedAt)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Write session metadata as first line
	filePath := s.sessionFile(session)
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer f.Close()

	// Write session metadata
	meta := map[string]interface{}{
		"_type":   "session_meta",
		"session": session,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write session: %w", err)
	}

	// Update in-memory cache
	s.sessions[session.ID] = session
	s.updateSessionIndex()

	return nil
}

// GetSession retrieves a session by ID.
func (s *JSONLStorage) GetSession(ctx context.Context, id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// ListSessions lists sessions with filtering and pagination.
func (s *JSONLStorage) ListSessions(ctx context.Context, opts *ListOptions) ([]*Session, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter sessions
	var filtered []*Session
	for _, id := range s.sessionIndex {
		session := s.sessions[id]
		if s.matchesSessionFilter(session, opts) {
			filtered = append(filtered, session)
		}
	}

	total := len(filtered)

	// Apply sorting
	s.sortSessions(filtered, opts)

	// Apply pagination
	if opts != nil {
		start := opts.Offset
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + opts.Limit
		if opts.Limit <= 0 || end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[start:end]
	}

	return filtered, total, nil
}

// UpdateSession updates an existing session.
func (s *JSONLStorage) UpdateSession(ctx context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.sessions[session.ID]
	if !ok {
		return ErrSessionNotFound
	}

	// Update the session file by rewriting it
	filePath := s.sessionFile(existing)

	// Read existing events
	events, err := s.readEventsFromFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read events: %w", err)
	}

	// Rewrite file with updated session metadata
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer f.Close()

	// Write updated session metadata
	meta := map[string]interface{}{
		"_type":   "session_meta",
		"session": session,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write session: %w", err)
	}

	// Write events back
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}
	}

	// Update in-memory cache
	s.sessions[session.ID] = session

	return nil
}

// DeleteSession deletes a session and its events.
func (s *JSONLStorage) DeleteSession(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}

	// Delete the session file
	filePath := s.sessionFile(session)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	// Remove from in-memory cache
	delete(s.sessions, id)
	s.updateSessionIndex()

	return nil
}

// AppendEvent appends an event to a session file.
func (s *JSONLStorage) AppendEvent(ctx context.Context, event *SessionEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[event.SessionID]
	if !ok {
		return ErrSessionNotFound
	}

	// Open file in append mode
	filePath := s.sessionFile(session)
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	// Write event
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write event: %w", err)
	}

	// Update session event count
	session.EventCount++

	return nil
}

// GetSessionEvents retrieves events for a session.
func (s *JSONLStorage) GetSessionEvents(ctx context.Context, sessionID string, opts *ListOptions) ([]*SessionEvent, int, error) {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return nil, 0, ErrSessionNotFound
	}

	filePath := s.sessionFile(session)
	events, err := s.readEventsFromFile(filePath)
	if err != nil {
		return nil, 0, err
	}

	total := len(events)

	// Apply pagination
	if opts != nil {
		start := opts.Offset
		if start > len(events) {
			start = len(events)
		}
		end := start + opts.Limit
		if opts.Limit <= 0 || end > len(events) {
			end = len(events)
		}
		events = events[start:end]
	}

	return events, total, nil
}

// SaveAlert saves an alert to storage.
func (s *JSONLStorage) SaveAlert(ctx context.Context, alert *Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Append to daily alert file
	filePath := s.alertFile(alert.Timestamp)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create alerts directory: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open alerts file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write alert: %w", err)
	}

	// Update in-memory cache
	s.alerts[alert.ID] = alert
	s.updateAlertIndex()

	return nil
}

// GetAlert retrieves an alert by ID.
func (s *JSONLStorage) GetAlert(ctx context.Context, id string) (*Alert, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alert, ok := s.alerts[id]
	if !ok {
		return nil, ErrAlertNotFound
	}
	return alert, nil
}

// ListAlerts lists alerts with filtering and pagination.
func (s *JSONLStorage) ListAlerts(ctx context.Context, opts *ListOptions) ([]*Alert, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Filter alerts
	var filtered []*Alert
	for _, id := range s.alertIndex {
		alert := s.alerts[id]
		if s.matchesAlertFilter(alert, opts) {
			filtered = append(filtered, alert)
		}
	}

	total := len(filtered)

	// Sort by timestamp descending (most recent first)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// Apply pagination
	if opts != nil {
		start := opts.Offset
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + opts.Limit
		if opts.Limit <= 0 || end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[start:end]
	}

	return filtered, total, nil
}

// UpdateAlert updates an existing alert.
func (s *JSONLStorage) UpdateAlert(ctx context.Context, alert *Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.alerts[alert.ID]; !ok {
		return ErrAlertNotFound
	}

	// Update in-memory cache
	s.alerts[alert.ID] = alert

	// Rewrite the alert file for the day
	filePath := s.alertFile(alert.Timestamp)
	return s.rewriteAlertsFile(filePath)
}

// SaveDailyStats saves daily statistics.
func (s *JSONLStorage) SaveDailyStats(ctx context.Context, stats *DailyStats) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read existing stats
	allStats, err := s.readAllDailyStats()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing stats: %w", err)
	}

	// Update or add stats for the date
	found := false
	for i, existing := range allStats {
		if existing.Date == stats.Date {
			allStats[i] = stats
			found = true
			break
		}
	}
	if !found {
		allStats = append(allStats, stats)
	}

	// Write all stats
	filePath := s.statsFile()
	data, err := json.MarshalIndent(allStats, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal stats: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write stats: %w", err)
	}

	return nil
}

// GetDailyStats retrieves daily statistics for a date.
func (s *JSONLStorage) GetDailyStats(ctx context.Context, date string) (*DailyStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allStats, err := s.readAllDailyStats()
	if err != nil {
		return nil, err
	}

	for _, stats := range allStats {
		if stats.Date == date {
			return stats, nil
		}
	}

	return nil, fmt.Errorf("stats not found for date: %s", date)
}

// CleanupExpired removes expired data based on retention policy.
func (s *JSONLStorage) CleanupExpired(ctx context.Context, retention *RetentionConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeutil.NowTime()

	// Cleanup old session directories
	sessionsDir := filepath.Join(s.basePath, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read sessions directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		date, err := time.Parse("2006-01-02", entry.Name())
		if err != nil {
			continue
		}
		if now.Sub(date) > time.Duration(retention.SessionsDays)*24*time.Hour {
			dirPath := filepath.Join(sessionsDir, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				return fmt.Errorf("failed to remove old session directory: %w", err)
			}
			// Remove from in-memory cache
			for id, session := range s.sessions {
				if session.StartedAt.Format("2006-01-02") == entry.Name() {
					delete(s.sessions, id)
				}
			}
		}
	}

	// Cleanup old alert files
	alertsDir := filepath.Join(s.basePath, "alerts")
	entries, err = os.ReadDir(alertsDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read alerts directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".jsonl")
		date, err := time.Parse("2006-01-02", name)
		if err != nil {
			continue
		}
		if now.Sub(date) > time.Duration(retention.AlertsDays)*24*time.Hour {
			filePath := filepath.Join(alertsDir, entry.Name())
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("failed to remove old alert file: %w", err)
			}
			// Remove from in-memory cache
			for id, alert := range s.alerts {
				if alert.Timestamp.Format("2006-01-02") == name {
					delete(s.alerts, id)
				}
			}
		}
	}

	s.updateSessionIndex()
	s.updateAlertIndex()

	return nil
}

// Close closes the storage.
func (s *JSONLStorage) Close() error {
	return nil
}

// Helper methods

func (s *JSONLStorage) loadSessions() error {
	sessionsDir := filepath.Join(s.basePath, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dayDir := filepath.Join(sessionsDir, entry.Name())
		files, err := os.ReadDir(dayDir)
		if err != nil {
			continue
		}
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".jsonl") {
				continue
			}
			filePath := filepath.Join(dayDir, file.Name())
			session, err := s.readSessionFromFile(filePath)
			if err != nil {
				continue
			}
			s.sessions[session.ID] = session
		}
	}

	s.updateSessionIndex()
	return nil
}

func (s *JSONLStorage) loadAlerts() error {
	alertsDir := filepath.Join(s.basePath, "alerts")
	entries, err := os.ReadDir(alertsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		filePath := filepath.Join(alertsDir, entry.Name())
		alerts, err := s.readAlertsFromFile(filePath)
		if err != nil {
			continue
		}
		for _, alert := range alerts {
			s.alerts[alert.ID] = alert
		}
	}

	s.updateAlertIndex()
	return nil
}

func (s *JSONLStorage) readSessionFromFile(filePath string) (*Session, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if scanner.Scan() {
		line := scanner.Bytes()
		var meta struct {
			Type    string   `json:"_type"`
			Session *Session `json:"session"`
		}
		if err := json.Unmarshal(line, &meta); err != nil {
			return nil, err
		}
		if meta.Type == "session_meta" && meta.Session != nil {
			return meta.Session, nil
		}
	}

	return nil, fmt.Errorf("session metadata not found in file")
}

func (s *JSONLStorage) readEventsFromFile(filePath string) ([]*SessionEvent, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []*SessionEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Skip session metadata line
		var meta struct {
			Type string `json:"_type"`
		}
		if err := json.Unmarshal(line, &meta); err == nil && meta.Type == "session_meta" {
			continue
		}

		var event SessionEvent
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		events = append(events, &event)
	}

	return events, scanner.Err()
}

func (s *JSONLStorage) readAlertsFromFile(filePath string) ([]*Alert, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var alerts []*Alert
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var alert Alert
		if err := json.Unmarshal(scanner.Bytes(), &alert); err != nil {
			continue
		}
		alerts = append(alerts, &alert)
	}

	return alerts, scanner.Err()
}

func (s *JSONLStorage) readAllDailyStats() ([]*DailyStats, error) {
	filePath := s.statsFile()
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var stats []*DailyStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *JSONLStorage) rewriteAlertsFile(filePath string) error {
	// Get all alerts for this file's date
	date := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")
	var alerts []*Alert
	for _, alert := range s.alerts {
		if alert.Timestamp.Format("2006-01-02") == date {
			alerts = append(alerts, alert)
		}
	}

	// Sort by timestamp
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Timestamp.Before(alerts[j].Timestamp)
	})

	// Rewrite file
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, alert := range alerts {
		data, err := json.Marshal(alert)
		if err != nil {
			continue
		}
		if _, err := f.Write(append(data, '\n')); err != nil {
			return err
		}
	}

	return nil
}

func (s *JSONLStorage) updateSessionIndex() {
	s.sessionIndex = make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		s.sessionIndex = append(s.sessionIndex, id)
	}
	sort.Slice(s.sessionIndex, func(i, j int) bool {
		return s.sessions[s.sessionIndex[i]].StartedAt.After(s.sessions[s.sessionIndex[j]].StartedAt)
	})
}

func (s *JSONLStorage) updateAlertIndex() {
	s.alertIndex = make([]string, 0, len(s.alerts))
	for id := range s.alerts {
		s.alertIndex = append(s.alertIndex, id)
	}
	sort.Slice(s.alertIndex, func(i, j int) bool {
		return s.alerts[s.alertIndex[i]].Timestamp.After(s.alerts[s.alertIndex[j]].Timestamp)
	})
}

func (s *JSONLStorage) matchesSessionFilter(session *Session, opts *ListOptions) bool {
	if opts == nil {
		return true
	}

	if opts.Platform != "" && session.Platform != opts.Platform {
		return false
	}
	if opts.UserID != "" && session.UserID != opts.UserID {
		return false
	}
	if opts.Status != "" && session.Status != opts.Status {
		return false
	}
	if opts.From != nil && session.StartedAt.Before(*opts.From) {
		return false
	}
	if opts.To != nil && session.StartedAt.After(*opts.To) {
		return false
	}

	return true
}

func (s *JSONLStorage) matchesAlertFilter(alert *Alert, opts *ListOptions) bool {
	if opts == nil {
		return true
	}

	if opts.From != nil && alert.Timestamp.Before(*opts.From) {
		return false
	}
	if opts.To != nil && alert.Timestamp.After(*opts.To) {
		return false
	}

	// Check filters map
	if opts.Filters != nil {
		if severity, ok := opts.Filters["severity"]; ok && string(alert.Severity) != severity {
			return false
		}
		if acked, ok := opts.Filters["acknowledged"]; ok {
			if acked == "true" && !alert.Acknowledged {
				return false
			}
			if acked == "false" && alert.Acknowledged {
				return false
			}
		}
	}

	return true
}

func (s *JSONLStorage) sortSessions(sessions []*Session, opts *ListOptions) {
	if opts == nil || opts.Sort == "" {
		// Default: sort by start time descending
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].StartedAt.After(sessions[j].StartedAt)
		})
		return
	}

	ascending := opts.Order != "desc"

	switch opts.Sort {
	case "started_at":
		sort.Slice(sessions, func(i, j int) bool {
			if ascending {
				return sessions[i].StartedAt.Before(sessions[j].StartedAt)
			}
			return sessions[i].StartedAt.After(sessions[j].StartedAt)
		})
	case "duration":
		sort.Slice(sessions, func(i, j int) bool {
			if ascending {
				return sessions[i].Duration < sessions[j].Duration
			}
			return sessions[i].Duration > sessions[j].Duration
		})
	case "threat_score":
		sort.Slice(sessions, func(i, j int) bool {
			if ascending {
				return sessions[i].ThreatScore < sessions[j].ThreatScore
			}
			return sessions[i].ThreatScore > sessions[j].ThreatScore
		})
	case "event_count":
		sort.Slice(sessions, func(i, j int) bool {
			if ascending {
				return sessions[i].EventCount < sessions[j].EventCount
			}
			return sessions[i].EventCount > sessions[j].EventCount
		})
	}
}
