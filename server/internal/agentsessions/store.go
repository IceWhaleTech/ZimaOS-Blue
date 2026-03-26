package agentsessions

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) (*SQLiteStore, error) {
	if db == nil {
		return nil, fmt.Errorf("agent sessions store requires a database")
	}
	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
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
	_, err = s.db.Exec(
		`INSERT INTO agent_profiles (id, protocol, name, builtin, data, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   protocol=excluded.protocol,
		   name=excluded.name,
		   builtin=excluded.builtin,
		   data=excluded.data,
		   updated_at=excluded.updated_at`,
		profile.ID,
		profile.Protocol,
		profile.Name,
		boolToInt(profile.Builtin),
		string(data),
		now.Format(time.RFC3339Nano),
	)
	return err
}

func (s *SQLiteStore) GetProfile(id string) (*AgentProfile, error) {
	row := s.db.QueryRow(`SELECT data FROM agent_profiles WHERE id = ?`, strings.TrimSpace(id))
	var raw string
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProfileNotFound
		}
		return nil, err
	}
	var profile AgentProfile
	if err := json.Unmarshal([]byte(raw), &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *SQLiteStore) ListProfiles() ([]AgentProfile, error) {
	rows, err := s.db.Query(`SELECT data FROM agent_profiles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []AgentProfile
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var profile AgentProfile
		if err := json.Unmarshal([]byte(raw), &profile); err != nil {
			return nil, err
		}
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].Builtin != profiles[j].Builtin {
			return profiles[i].Builtin
		}
		if profiles[i].Protocol != profiles[j].Protocol {
			return profiles[i].Protocol < profiles[j].Protocol
		}
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, rows.Err()
}

func (s *SQLiteStore) DeleteProfile(id string) error {
	res, err := s.db.Exec(`DELETE FROM agent_profiles WHERE id = ?`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
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
	_, err = s.db.Exec(
		`INSERT INTO agent_sessions (
			id, profile_id, protocol, user_id, name, cwd, status, remote_session_id, last_error, data, created_at, updated_at, closed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  profile_id=excluded.profile_id,
		  protocol=excluded.protocol,
		  user_id=excluded.user_id,
		  name=excluded.name,
		  cwd=excluded.cwd,
		  status=excluded.status,
		  remote_session_id=excluded.remote_session_id,
		  last_error=excluded.last_error,
		  data=excluded.data,
		  updated_at=excluded.updated_at,
		  closed_at=excluded.closed_at`,
		session.ID,
		session.ProfileID,
		session.Protocol,
		strings.TrimSpace(session.UserID),
		session.Name,
		strings.TrimSpace(session.CWD),
		session.Status,
		strings.TrimSpace(session.RemoteSessionID),
		strings.TrimSpace(session.LastError),
		string(data),
		session.CreatedAt.UTC().Format(time.RFC3339Nano),
		session.UpdatedAt.UTC().Format(time.RFC3339Nano),
		formatOptionalTime(session.ClosedAt),
	)
	return err
}

func (s *SQLiteStore) GetSession(id string) (*ExternalSession, error) {
	row := s.db.QueryRow(`SELECT data FROM agent_sessions WHERE id = ?`, strings.TrimSpace(id))
	var raw string
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	var session ExternalSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *SQLiteStore) ListSessions(limit, offset int, userID string, protocol ProtocolKind) ([]ExternalSession, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := `SELECT data FROM agent_sessions`
	var (
		args       []interface{}
		conditions []string
	)
	if strings.TrimSpace(userID) != "" {
		conditions = append(conditions, "user_id = ?")
		args = append(args, strings.TrimSpace(userID))
	}
	if protocol != "" {
		conditions = append(conditions, "protocol = ?")
		args = append(args, protocol)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY updated_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []ExternalSession
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var session ExternalSession
		if err := json.Unmarshal([]byte(raw), &session); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
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
	_, err = s.db.Exec(
		`INSERT INTO agent_runs (
			id, session_id, status, remote_run_id, prompt, stop_reason, error, data, created_at, updated_at, started_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  session_id=excluded.session_id,
		  status=excluded.status,
		  remote_run_id=excluded.remote_run_id,
		  prompt=excluded.prompt,
		  stop_reason=excluded.stop_reason,
		  error=excluded.error,
		  data=excluded.data,
		  updated_at=excluded.updated_at,
		  started_at=excluded.started_at,
		  completed_at=excluded.completed_at`,
		run.ID,
		run.SessionID,
		run.Status,
		strings.TrimSpace(run.RemoteRunID),
		run.Prompt,
		strings.TrimSpace(run.StopReason),
		strings.TrimSpace(run.Error),
		string(data),
		run.CreatedAt.UTC().Format(time.RFC3339Nano),
		run.UpdatedAt.UTC().Format(time.RFC3339Nano),
		formatOptionalTime(run.StartedAt),
		formatOptionalTime(run.CompletedAt),
	)
	return err
}

func (s *SQLiteStore) GetRun(id string) (*ExternalRun, error) {
	row := s.db.QueryRow(`SELECT data FROM agent_runs WHERE id = ?`, strings.TrimSpace(id))
	var raw string
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrRunNotFound
		}
		return nil, err
	}
	var run ExternalRun
	if err := json.Unmarshal([]byte(raw), &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *SQLiteStore) ListRuns(sessionID string, limit int) ([]ExternalRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT data FROM agent_runs WHERE session_id = ? ORDER BY created_at DESC LIMIT ?`,
		strings.TrimSpace(sessionID), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var runs []ExternalRun
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var run ExternalRun
		if err := json.Unmarshal([]byte(raw), &run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *SQLiteStore) LatestActiveRun(sessionID string) (*ExternalRun, error) {
	row := s.db.QueryRow(
		`SELECT data FROM agent_runs WHERE session_id = ? AND status IN (?, ?) ORDER BY updated_at DESC LIMIT 1`,
		strings.TrimSpace(sessionID), RunStatusQueued, RunStatusRunning,
	)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	var run ExternalRun
	if err := json.Unmarshal([]byte(raw), &run); err != nil {
		return nil, err
	}
	return &run, nil
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
	res, err := s.db.Exec(
		`INSERT INTO agent_run_events (session_id, run_id, event_index, type, payload, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(sessionID),
		strings.TrimSpace(runID),
		index,
		strings.TrimSpace(eventType),
		string(data),
		now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &RunEvent{
		ID:        id,
		SessionID: strings.TrimSpace(sessionID),
		RunID:     strings.TrimSpace(runID),
		Index:     index,
		Type:      strings.TrimSpace(eventType),
		Payload:   json.RawMessage(data),
		CreatedAt: now,
	}, nil
}

func (s *SQLiteStore) ListEvents(sessionID string, limit int, afterID int64) ([]RunEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(
		`SELECT id, session_id, run_id, event_index, type, payload, created_at
		 FROM agent_run_events
		 WHERE session_id = ? AND id > ?
		 ORDER BY id ASC
		 LIMIT ?`,
		strings.TrimSpace(sessionID), afterID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []RunEvent
	for rows.Next() {
		var (
			event     RunEvent
			payload   string
			createdAt string
		)
		if err := rows.Scan(&event.ID, &event.SessionID, &event.RunID, &event.Index, &event.Type, &payload, &createdAt); err != nil {
			return nil, err
		}
		event.Payload = json.RawMessage(payload)
		event.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		events = append(events, event)
	}
	return events, rows.Err()
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
	row := s.db.QueryRow(
		`SELECT COALESCE(MAX(event_index), 0) FROM agent_run_events WHERE session_id = ? AND run_id = ?`,
		strings.TrimSpace(sessionID),
		strings.TrimSpace(runID),
	)
	var index int
	if err := row.Scan(&index); err != nil {
		return 0, err
	}
	return index + 1, nil
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
