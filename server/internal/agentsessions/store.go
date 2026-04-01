package agentsessions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

type SQLiteStore struct {
	db     *sql.DB
	readDB *sql.DB
}

func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	return NewSQLiteStoreWithReadDB(db, db)
}

func NewSQLiteStoreWithReadDB(writeDB, readDB *sql.DB) (*SQLiteStore, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("agent sessions store requires a database")
	}
	if readDB == nil {
		readDB = writeDB
	}
	s := &SQLiteStore{db: writeDB, readDB: readDB}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLiteStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *SQLiteStore) table(name string) *z.ZormTable {
	return z.Table(s.db, name)
}

func (s *SQLiteStore) readTable(name string) *z.ZormTable {
	return z.Table(s.reader(), name)
}

type jsonDataRow struct {
	Data string `json:"data" zorm:"data"`
}

type runEventRow struct {
	ID         int64  `json:"id" zorm:"id,auto_incr"`
	SessionID  string `json:"session_id" zorm:"session_id"`
	RunID      string `json:"run_id" zorm:"run_id"`
	EventIndex int    `json:"event_index" zorm:"event_index"`
	Type       string `json:"type" zorm:"type"`
	Payload    string `json:"payload" zorm:"payload"`
	CreatedAt  string `json:"created_at" zorm:"created_at"`
}

func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS agent_profiles (
		id TEXT PRIMARY KEY,
		protocol TEXT NOT NULL,
		name TEXT NOT NULL,
		builtin INTEGER NOT NULL DEFAULT 0,
		data TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS agent_sessions (
		id TEXT PRIMARY KEY,
		profile_id TEXT NOT NULL,
		protocol TEXT NOT NULL,
		user_id TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL,
		cwd TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL,
		remote_session_id TEXT NOT NULL DEFAULT '',
		last_error TEXT NOT NULL DEFAULT '',
		data TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		closed_at TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_agent_sessions_user_updated ON agent_sessions(user_id, updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_agent_sessions_profile_updated ON agent_sessions(profile_id, updated_at DESC);
	CREATE TABLE IF NOT EXISTS agent_runs (
		id TEXT PRIMARY KEY,
		session_id TEXT NOT NULL,
		status TEXT NOT NULL,
		remote_run_id TEXT NOT NULL DEFAULT '',
		prompt TEXT NOT NULL DEFAULT '',
		stop_reason TEXT NOT NULL DEFAULT '',
		error TEXT NOT NULL DEFAULT '',
		data TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		started_at TEXT NOT NULL DEFAULT '',
		completed_at TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_agent_runs_session_created ON agent_runs(session_id, created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_agent_runs_session_status ON agent_runs(session_id, status, updated_at DESC);
	CREATE TABLE IF NOT EXISTS agent_run_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		run_id TEXT NOT NULL,
		event_index INTEGER NOT NULL,
		type TEXT NOT NULL,
		payload TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_agent_run_events_session_id ON agent_run_events(session_id, id ASC);
	CREATE INDEX IF NOT EXISTS idx_agent_run_events_run_id ON agent_run_events(run_id, id ASC);
	`
	_, err := s.db.Exec(schema)
	return err
}

func cloneMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func decodeProfile(raw string) (*AgentProfile, error) {
	var profile AgentProfile
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func decodeProfiles(rows []jsonDataRow) ([]AgentProfile, error) {
	profiles := make([]AgentProfile, 0, len(rows))
	for i := range rows {
		profile, err := decodeProfile(rows[i].Data)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, *profile)
	}
	return profiles, nil
}

func decodeSession(raw string) (*ExternalSession, error) {
	var session ExternalSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func decodeSessions(rows []jsonDataRow) ([]ExternalSession, error) {
	sessions := make([]ExternalSession, 0, len(rows))
	for i := range rows {
		session, err := decodeSession(rows[i].Data)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *session)
	}
	return sessions, nil
}

func decodeRun(raw string) (*ExternalRun, error) {
	var run ExternalRun
	if err := json.Unmarshal([]byte(raw), &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func decodeRuns(rows []jsonDataRow) ([]ExternalRun, error) {
	runs := make([]ExternalRun, 0, len(rows))
	for i := range rows {
		run, err := decodeRun(rows[i].Data)
		if err != nil {
			return nil, err
		}
		runs = append(runs, *run)
	}
	return runs, nil
}

func parseStoredTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func rowToRunEvent(row runEventRow) RunEvent {
	return RunEvent{
		ID:        row.ID,
		SessionID: row.SessionID,
		RunID:     row.RunID,
		Index:     row.EventIndex,
		Type:      row.Type,
		Payload:   json.RawMessage(row.Payload),
		CreatedAt: parseStoredTime(row.CreatedAt),
	}
}

func rowsToRunEvents(rows []runEventRow) []RunEvent {
	events := make([]RunEvent, 0, len(rows))
	for i := range rows {
		events = append(events, rowToRunEvent(rows[i]))
	}
	return events
}

func (s *SQLiteStore) SaveProfile(profile *AgentProfile) error {
	if profile == nil {
		return fmt.Errorf("profile is nil")
	}
	now := timeutil.NowTime().UTC()
	if strings.TrimSpace(profile.ID) == "" {
		profile.ID = uuid.NewString()
	}
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}
	profile.UpdatedAt = now
	if strings.TrimSpace(profile.Name) == "" {
		profile.Name = profile.ID
	}
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	_, err = s.table("agent_profiles").Insert(z.V{
		"id":         profile.ID,
		"protocol":   string(profile.Protocol),
		"name":       profile.Name,
		"builtin":    boolToInt(profile.Builtin),
		"data":       string(data),
		"updated_at": now.Format(time.RFC3339Nano),
	}, z.OnConflictDoUpdateSet(
		[]string{"id"},
		[]string{"protocol", "name", "builtin", "data", "updated_at"},
	))
	return err
}

func (s *SQLiteStore) GetProfile(id string) (*AgentProfile, error) {
	var rows []jsonDataRow
	_, err := s.readTable("agent_profiles").Select(
		&rows,
		z.Where(z.Eq("id", strings.TrimSpace(id))),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrProfileNotFound
	}
	return decodeProfile(rows[0].Data)
}

func (s *SQLiteStore) ListProfiles() ([]AgentProfile, error) {
	var rows []jsonDataRow
	_, err := s.readTable("agent_profiles").Select(
		&rows,
		z.OrderBy("builtin DESC, protocol ASC, name ASC"),
	)
	if err != nil {
		return nil, err
	}
	return decodeProfiles(rows)
}

func (s *SQLiteStore) DeleteProfile(id string) error {
	affected, err := s.table("agent_profiles").Delete(z.Where(z.Eq("id", strings.TrimSpace(id))))
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrProfileNotFound
	}
	return nil
}

func (s *SQLiteStore) SaveSession(session *ExternalSession) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	now := timeutil.NowTime().UTC()
	if strings.TrimSpace(session.ID) == "" {
		session.ID = uuid.NewString()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.UpdatedAt = now
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	_, err = s.table("agent_sessions").Insert(z.V{
		"id":                session.ID,
		"profile_id":        session.ProfileID,
		"protocol":          string(session.Protocol),
		"user_id":           strings.TrimSpace(session.UserID),
		"name":              session.Name,
		"cwd":               strings.TrimSpace(session.CWD),
		"status":            string(session.Status),
		"remote_session_id": strings.TrimSpace(session.RemoteSessionID),
		"last_error":        strings.TrimSpace(session.LastError),
		"data":              string(data),
		"created_at":        session.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":        session.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"closed_at":         formatOptionalTime(session.ClosedAt),
	}, z.OnConflictDoUpdateSet(
		[]string{"id"},
		[]string{
			"profile_id",
			"protocol",
			"user_id",
			"name",
			"cwd",
			"status",
			"remote_session_id",
			"last_error",
			"data",
			"updated_at",
			"closed_at",
		},
	))
	return err
}

func (s *SQLiteStore) GetSession(id string) (*ExternalSession, error) {
	var rows []jsonDataRow
	_, err := s.readTable("agent_sessions").Select(
		&rows,
		z.Where(z.Eq("id", strings.TrimSpace(id))),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrSessionNotFound
	}
	return decodeSession(rows[0].Data)
}

func (s *SQLiteStore) ListSessions(limit, offset int, userID string, protocol ProtocolKind) ([]ExternalSession, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var conds []interface{}
	if strings.TrimSpace(userID) != "" {
		conds = append(conds, z.Eq("user_id", strings.TrimSpace(userID)))
	}
	if protocol != "" {
		conds = append(conds, z.Eq("protocol", string(protocol)))
	}
	opts := []z.ZormItem{
		z.OrderBy("updated_at DESC"),
		z.Limit(limit, offset),
	}
	if len(conds) > 0 {
		opts = append([]z.ZormItem{z.Where(conds...)}, opts...)
	}
	var rows []jsonDataRow
	if _, err := s.readTable("agent_sessions").Select(&rows, opts...); err != nil {
		return nil, err
	}
	return decodeSessions(rows)
}

func (s *SQLiteStore) SaveRun(run *ExternalRun) error {
	if run == nil {
		return fmt.Errorf("run is nil")
	}
	now := timeutil.NowTime().UTC()
	if strings.TrimSpace(run.ID) == "" {
		run.ID = uuid.NewString()
	}
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	run.UpdatedAt = now
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = s.table("agent_runs").Insert(z.V{
		"id":            run.ID,
		"session_id":    run.SessionID,
		"status":        string(run.Status),
		"remote_run_id": strings.TrimSpace(run.RemoteRunID),
		"prompt":        run.Prompt,
		"stop_reason":   strings.TrimSpace(run.StopReason),
		"error":         strings.TrimSpace(run.Error),
		"data":          string(data),
		"created_at":    run.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":    run.UpdatedAt.UTC().Format(time.RFC3339Nano),
		"started_at":    formatOptionalTime(run.StartedAt),
		"completed_at":  formatOptionalTime(run.CompletedAt),
	}, z.OnConflictDoUpdateSet(
		[]string{"id"},
		[]string{
			"session_id",
			"status",
			"remote_run_id",
			"prompt",
			"stop_reason",
			"error",
			"data",
			"updated_at",
			"started_at",
			"completed_at",
		},
	))
	return err
}

func (s *SQLiteStore) GetRun(id string) (*ExternalRun, error) {
	var rows []jsonDataRow
	_, err := s.readTable("agent_runs").Select(
		&rows,
		z.Where(z.Eq("id", strings.TrimSpace(id))),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrRunNotFound
	}
	return decodeRun(rows[0].Data)
}

func (s *SQLiteStore) ListRuns(sessionID string, limit int) ([]ExternalRun, error) {
	if limit <= 0 {
		limit = 20
	}
	var rows []jsonDataRow
	_, err := s.readTable("agent_runs").Select(
		&rows,
		z.Where(z.Eq("session_id", strings.TrimSpace(sessionID))),
		z.OrderBy("created_at DESC"),
		z.Limit(limit),
	)
	if err != nil {
		return nil, err
	}
	return decodeRuns(rows)
}

func (s *SQLiteStore) LatestActiveRun(sessionID string) (*ExternalRun, error) {
	var rows []jsonDataRow
	_, err := s.readTable("agent_runs").Select(
		&rows,
		z.Where(
			z.Eq("session_id", strings.TrimSpace(sessionID)),
			z.In("status", string(RunStatusQueued), string(RunStatusRunning)),
		),
		z.OrderBy("updated_at DESC"),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return decodeRun(rows[0].Data)
}

func (s *SQLiteStore) AppendEvent(sessionID, runID, eventType string, payload interface{}) (*RunEvent, error) {
	index, err := s.nextEventIndex(sessionID, runID)
	if err != nil {
		return nil, err
	}
	now := timeutil.NowTime().UTC()
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	row := &runEventRow{
		SessionID:  strings.TrimSpace(sessionID),
		RunID:      strings.TrimSpace(runID),
		EventIndex: index,
		Type:       strings.TrimSpace(eventType),
		Payload:    string(data),
		CreatedAt:  now.Format(time.RFC3339Nano),
	}
	_, err = s.table("agent_run_events").Insert(row)
	if err != nil {
		return nil, err
	}
	return &RunEvent{
		ID:        row.ID,
		SessionID: row.SessionID,
		RunID:     row.RunID,
		Index:     index,
		Type:      row.Type,
		Payload:   json.RawMessage(data),
		CreatedAt: now,
	}, nil
}

func (s *SQLiteStore) ListEvents(sessionID string, limit int, afterID int64) ([]RunEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	var rows []runEventRow
	_, err := s.readTable("agent_run_events").Select(
		&rows,
		z.Where(
			z.Eq("session_id", strings.TrimSpace(sessionID)),
			z.Gt("id", afterID),
		),
		z.OrderBy("id ASC"),
		z.Limit(limit),
	)
	if err != nil {
		return nil, err
	}
	return rowsToRunEvents(rows), nil
}

func (s *SQLiteStore) BuildHistory(sessionID string, limit int) ([]SessionHistoryItem, error) {
	events, err := s.ListEvents(sessionID, limit, 0)
	if err != nil {
		return nil, err
	}
	history := make([]SessionHistoryItem, 0, len(events))
	for _, event := range events {
		item := SessionHistoryItem{
			ID:        event.ID,
			RunID:     event.RunID,
			Type:      event.Type,
			CreatedAt: event.CreatedAt,
		}
		var payload map[string]interface{}
		_ = json.Unmarshal(event.Payload, &payload)
		item.Metadata = cloneMap(payload)
		if role, _ := payload["role"].(string); role != "" {
			item.Role = role
		}
		if text, _ := payload["text"].(string); text != "" {
			item.Content = text
		}
		if text, _ := payload["content"].(string); item.Content == "" && text != "" {
			item.Content = text
		}
		history = append(history, item)
	}
	return history, nil
}

func (s *SQLiteStore) nextEventIndex(sessionID, runID string) (int, error) {
	var index int64
	_, err := s.readTable("agent_run_events").Select(
		&index,
		z.Fields("coalesce(max(event_index), 0)"),
		z.Where(
			z.Eq("session_id", strings.TrimSpace(sessionID)),
			z.Eq("run_id", strings.TrimSpace(runID)),
		),
	)
	if err != nil {
		return 0, err
	}
	return int(index) + 1, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatOptionalTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}

func (s *SQLiteStore) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
