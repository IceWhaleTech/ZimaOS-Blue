package tools

import (
	"context"
	"database/sql"
	"sync"
	"time"

	z "github.com/IceWhaleTech/zorm"
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
	db     *sql.DB
	readDB *sql.DB
}

// NewExecAuditStore creates the audit table and returns a store.
func NewExecAuditStore(db *sql.DB) (*ExecAuditStore, error) {
	return NewExecAuditStoreWithReadDB(db, db)
}

// NewExecAuditStoreWithReadDB creates the audit table and returns a store with
// separate write and read database handles.
func NewExecAuditStoreWithReadDB(writeDB, readDB *sql.DB) (*ExecAuditStore, error) {
	if writeDB == nil {
		return nil, sql.ErrConnDone
	}
	if readDB == nil {
		readDB = writeDB
	}
	_, err := writeDB.Exec(`CREATE TABLE IF NOT EXISTS exec_audit_log (
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
	writeDB.Exec(`CREATE INDEX IF NOT EXISTS idx_exec_audit_timestamp ON exec_audit_log(timestamp)`)
	return &ExecAuditStore{db: writeDB, readDB: readDB}, nil
}

func (s *ExecAuditStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *ExecAuditStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "exec_audit_log")
}

func (s *ExecAuditStore) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "exec_audit_log")
}

type execAuditRow struct {
	ID         string `json:"id" zorm:"id"`
	Timestamp  string `json:"timestamp" zorm:"timestamp"`
	UserID     string `json:"user_id" zorm:"user_id"`
	Command    string `json:"command" zorm:"command"`
	Workdir    string `json:"workdir" zorm:"workdir"`
	RiskScore  int    `json:"risk_score" zorm:"risk_score"`
	RiskLevel  string `json:"risk_level" zorm:"risk_level"`
	PolicyMode string `json:"policy_mode" zorm:"policy_mode"`
	Decision   string `json:"decision" zorm:"decision"`
	ExitCode   *int64 `json:"exit_code" zorm:"exit_code"`
	DurationMs int64  `json:"duration_ms" zorm:"duration_ms"`
	StdoutLen  int    `json:"stdout_len" zorm:"stdout_len"`
	StderrLen  int    `json:"stderr_len" zorm:"stderr_len"`
	Error      string `json:"error" zorm:"error"`
}

func rowToExecAuditEntry(row execAuditRow) ExecAuditEntry {
	entry := ExecAuditEntry{
		ID:         row.ID,
		UserID:     row.UserID,
		Command:    row.Command,
		Workdir:    row.Workdir,
		RiskScore:  row.RiskScore,
		RiskLevel:  RiskLevel(row.RiskLevel),
		PolicyMode: row.PolicyMode,
		Decision:   row.Decision,
		Duration:   time.Duration(row.DurationMs) * time.Millisecond,
		StdoutLen:  row.StdoutLen,
		StderrLen:  row.StderrLen,
		Error:      row.Error,
	}
	entry.Timestamp, _ = time.Parse(time.RFC3339Nano, row.Timestamp)
	if row.ExitCode != nil {
		code := int(*row.ExitCode)
		entry.ExitCode = &code
	}
	return entry
}

func rowsToExecAuditEntries(rows []execAuditRow) []ExecAuditEntry {
	entries := make([]ExecAuditEntry, 0, len(rows))
	for i := range rows {
		entries = append(entries, rowToExecAuditEntry(rows[i]))
	}
	return entries
}

// Record inserts an audit entry.
func (s *ExecAuditStore) Record(entry ExecAuditEntry) error {
	var exitCode interface{}
	if entry.ExitCode != nil {
		exitCode = int64(*entry.ExitCode)
	}
	_, err := s.table(context.Background()).Insert(map[string]interface{}{
		"id":          entry.ID,
		"timestamp":   entry.Timestamp.UTC().Format(time.RFC3339Nano),
		"user_id":     entry.UserID,
		"command":     truncateStr(entry.Command, 2000),
		"workdir":     entry.Workdir,
		"risk_score":  entry.RiskScore,
		"risk_level":  string(entry.RiskLevel),
		"policy_mode": entry.PolicyMode,
		"decision":    entry.Decision,
		"exit_code":   exitCode,
		"duration_ms": entry.Duration.Milliseconds(),
		"stdout_len":  entry.StdoutLen,
		"stderr_len":  entry.StderrLen,
		"error":       entry.Error,
	})
	return err
}

// Recent returns the most recent N audit entries.
func (s *ExecAuditStore) Recent(limit int) ([]ExecAuditEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []execAuditRow
	_, err := s.readTable(context.Background()).Select(&rows,
		z.OrderBy("timestamp DESC"),
		z.Limit(limit),
	)
	if err != nil {
		return nil, err
	}
	return rowsToExecAuditEntries(rows), nil
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
