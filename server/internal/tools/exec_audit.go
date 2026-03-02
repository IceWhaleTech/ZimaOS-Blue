package tools

import (
	"database/sql"
	"sync"
	"time"
)

// ExecAuditEntry is a single audit record for an exec invocation.
type ExecAuditEntry struct {
	ID         string        `json:"id"`
	Timestamp  time.Time     `json:"timestamp"`
	UserID     string        `json:"user_id"`
	Command    string        `json:"command"`
	Workdir    string        `json:"workdir"`
	RiskScore  int           `json:"risk_score"`
	RiskLevel  RiskLevel     `json:"risk_level"`
	PolicyMode string        `json:"policy_mode"`
	Decision   string        `json:"decision"` // "allowed", "blocked", "approved"
	ExitCode   *int          `json:"exit_code,omitempty"`
	Duration   time.Duration `json:"duration"`
	StdoutLen  int           `json:"stdout_len"`
	StderrLen  int           `json:"stderr_len"`
	Error      string        `json:"error,omitempty"`
}

// ExecAuditStore persists exec audit entries in SQLite.
type ExecAuditStore struct {
	db *sql.DB
}

// NewExecAuditStore creates the audit table and returns a store.
func NewExecAuditStore(db *sql.DB) (*ExecAuditStore, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS exec_audit_log (
		id          TEXT PRIMARY KEY,
		timestamp   TEXT NOT NULL,
		user_id     TEXT NOT NULL DEFAULT '',
		command     TEXT NOT NULL,
		workdir     TEXT NOT NULL DEFAULT '',
		risk_score  INTEGER NOT NULL DEFAULT 0,
		risk_level  TEXT NOT NULL DEFAULT 'low',
		policy_mode TEXT NOT NULL DEFAULT 'full',
		decision    TEXT NOT NULL DEFAULT 'allowed',
		exit_code   INTEGER,
		duration_ms INTEGER NOT NULL DEFAULT 0,
		stdout_len  INTEGER NOT NULL DEFAULT 0,
		stderr_len  INTEGER NOT NULL DEFAULT 0,
		error       TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return nil, err
	}
	// Index for time-range queries.
	db.Exec(`CREATE INDEX IF NOT EXISTS idx_exec_audit_timestamp ON exec_audit_log(timestamp)`)
	return &ExecAuditStore{db: db}, nil
}

// Record inserts an audit entry.
func (s *ExecAuditStore) Record(entry ExecAuditEntry) error {
	var exitCode sql.NullInt64
	if entry.ExitCode != nil {
		exitCode = sql.NullInt64{Int64: int64(*entry.ExitCode), Valid: true}
	}
	_, err := s.db.Exec(
		`INSERT INTO exec_audit_log (id, timestamp, user_id, command, workdir, risk_score, risk_level, policy_mode, decision, exit_code, duration_ms, stdout_len, stderr_len, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entry.ID,
		entry.Timestamp.UTC().Format(time.RFC3339Nano),
		entry.UserID,
		truncateStr(entry.Command, 2000),
		entry.Workdir,
		entry.RiskScore,
		string(entry.RiskLevel),
		entry.PolicyMode,
		entry.Decision,
		exitCode,
		entry.Duration.Milliseconds(),
		entry.StdoutLen,
		entry.StderrLen,
		entry.Error,
	)
	return err
}

// Recent returns the most recent N audit entries.
func (s *ExecAuditStore) Recent(limit int) ([]ExecAuditEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, timestamp, user_id, command, workdir, risk_score, risk_level, policy_mode, decision, exit_code, duration_ms, stdout_len, stderr_len, error
		 FROM exec_audit_log ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAuditRows(rows)
}

func scanAuditRows(rows *sql.Rows) ([]ExecAuditEntry, error) {
	var entries []ExecAuditEntry
	for rows.Next() {
		var e ExecAuditEntry
		var ts string
		var exitCode sql.NullInt64
		var durationMs int64
		if err := rows.Scan(&e.ID, &ts, &e.UserID, &e.Command, &e.Workdir,
			&e.RiskScore, &e.RiskLevel, &e.PolicyMode, &e.Decision,
			&exitCode, &durationMs, &e.StdoutLen, &e.StderrLen, &e.Error); err != nil {
			continue
		}
		e.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		if exitCode.Valid {
			code := int(exitCode.Int64)
			e.ExitCode = &code
		}
		e.Duration = time.Duration(durationMs) * time.Millisecond
		entries = append(entries, e)
	}
	return entries, nil
}

// retryRecord tracks recent invocations of the same normalized command.
type retryRecord struct {
	count   int
	firstAt time.Time
	lastErr string
}

// RetryTracker prevents the LLM from retrying the same failing command
// in a tight loop. It tracks normalized command strings within a time window.
type RetryTracker struct {
	mu         sync.Mutex
	records    map[string]*retryRecord // key = normalized command
	maxRetries int
	window     time.Duration
}

// NewRetryTracker creates a new retry tracker.
func NewRetryTracker(maxRetries int, window time.Duration) *RetryTracker {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if window <= 0 {
		window = 5 * time.Minute
	}
	return &RetryTracker{
		records:    make(map[string]*retryRecord),
		maxRetries: maxRetries,
		window:     window,
	}
}

// Check returns an error if the command has been retried too many times
// within the window. Call Record() after execution to track it.
func (rt *RetryTracker) Check(normalizedCmd string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.pruneExpired()

	rec, ok := rt.records[normalizedCmd]
	if !ok {
		return nil
	}
	if rec.count >= rt.maxRetries {
		return &RetryLimitError{
			Command: normalizedCmd,
			Count:   rec.count,
			Max:     rt.maxRetries,
			LastErr: rec.lastErr,
		}
	}
	return nil
}

// Record tracks a command execution. Only call for failed commands
// (successful commands reset the counter).
func (rt *RetryTracker) Record(normalizedCmd string, failed bool, errMsg string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if !failed {
		// Success resets the retry counter.
		delete(rt.records, normalizedCmd)
		return
	}

	rec, ok := rt.records[normalizedCmd]
	if !ok {
		rec = &retryRecord{firstAt: time.Now()}
		rt.records[normalizedCmd] = rec
	}
	rec.count++
	rec.lastErr = errMsg
}

// pruneExpired removes records outside the window. Must be called with lock held.
func (rt *RetryTracker) pruneExpired() {
	cutoff := time.Now().Add(-rt.window)
	for cmd, rec := range rt.records {
		if !rec.firstAt.After(cutoff) {
			delete(rt.records, cmd)
		}
	}
}

// RetryLimitError is returned when a command has been retried too many times.
type RetryLimitError struct {
	Command string
	Count   int
	Max     int
	LastErr string
}

func (e *RetryLimitError) Error() string {
	msg := "exec blocked: command has been retried " + itoa(e.Count) + " times (max " + itoa(e.Max) + ")"
	if e.LastErr != "" {
		msg += ". Last error: " + e.LastErr
	}
	msg += ". Analyze the error and try a different approach instead of retrying the same command."
	return msg
}

func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}
