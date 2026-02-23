// Package reminder provides persistent reminder management with SQLite storage.
package reminder

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Status constants for reminder lifecycle.
const (
	StatusPending   = "pending"
	StatusFired     = "fired"
	StatusCancelled = "cancelled"
)

// Reminder represents a persistent reminder entry.
type Reminder struct {
	ID        string     `json:"id"`
	OwnerID   string     `json:"owner_id"`
	Message   string     `json:"message"`
	FireAt    time.Time  `json:"fire_at"`
	Recurring string     `json:"recurring,omitempty"` // "", "daily", "weekly", "monthly"
	SessionID string     `json:"session_id,omitempty"`
	CronJobID string     `json:"cron_job_id,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	FiredAt   *time.Time `json:"fired_at,omitempty"`
}

// Store provides SQLite-backed persistence for reminders.
type Store struct {
	db *sql.DB
}

// NewStore creates a new reminder store and ensures the table exists.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("reminder store migration: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	// Check if existing table has the expected schema (fire_at column).
	// If the table exists with an old schema, drop and recreate.
	var hasFireAt bool
	rows, err := s.db.Query(`PRAGMA table_info(reminders)`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cid int
			var name, typ string
			var notNull int
			var dflt sql.NullString
			var pk int
			if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
				break
			}
			if name == "fire_at" {
				hasFireAt = true
			}
		}
		rows.Close()
	}

	// Table exists but missing fire_at → old schema, drop it
	if !hasFireAt {
		s.db.Exec(`DROP TABLE IF EXISTS reminders`)
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS reminders (
			id TEXT PRIMARY KEY,
			owner_id TEXT NOT NULL,
			message TEXT NOT NULL,
			fire_at DATETIME NOT NULL,
			recurring TEXT DEFAULT '',
			session_id TEXT DEFAULT '',
			cron_job_id TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL,
			fired_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_reminders_owner ON reminders(owner_id, status);
		CREATE INDEX IF NOT EXISTS idx_reminders_fire_at ON reminders(fire_at);
	`)
	return err
}

// Create inserts a new reminder.
func (s *Store) Create(ctx context.Context, r *Reminder) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = timeutil.NowTime()
	}
	if r.Status == "" {
		r.Status = StatusPending
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO reminders (id, owner_id, message, fire_at, recurring, session_id, cron_job_id, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.OwnerID, r.Message, r.FireAt, r.Recurring, r.SessionID, r.CronJobID, r.Status, r.CreatedAt,
	)
	return err
}

// Get retrieves a reminder by ID.
func (s *Store) Get(ctx context.Context, id string) (*Reminder, error) {
	r := &Reminder{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, message, fire_at, recurring, session_id, cron_job_id, status, created_at, fired_at
		 FROM reminders WHERE id = ?`, id,
	).Scan(&r.ID, &r.OwnerID, &r.Message, &r.FireAt, &r.Recurring, &r.SessionID, &r.CronJobID, &r.Status, &r.CreatedAt, &r.FiredAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

// ListByOwner returns reminders for a user, optionally filtered by status.
func (s *Store) ListByOwner(ctx context.Context, ownerID string, status ...string) ([]*Reminder, error) {
	query := `SELECT id, owner_id, message, fire_at, recurring, session_id, cron_job_id, status, created_at, fired_at
	          FROM reminders WHERE owner_id = ?`
	args := []any{ownerID}

	if len(status) > 0 && status[0] != "" {
		query += " AND status = ?"
		args = append(args, status[0])
	}
	query += " ORDER BY fire_at ASC"

	return s.queryReminders(ctx, query, args...)
}

// ListPending returns all pending reminders (for restart recovery).
func (s *Store) ListPending(ctx context.Context) ([]*Reminder, error) {
	return s.queryReminders(ctx,
		`SELECT id, owner_id, message, fire_at, recurring, session_id, cron_job_id, status, created_at, fired_at
		 FROM reminders WHERE status = ? ORDER BY fire_at ASC`, StatusPending)
}

// UpdateStatus updates the status and optionally the fired_at timestamp.
func (s *Store) UpdateStatus(ctx context.Context, id, status string, firedAt *time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE reminders SET status = ?, fired_at = ? WHERE id = ?`,
		status, firedAt, id)
	return err
}

// UpdateCronJobID sets the cron_job_id for a reminder.
func (s *Store) UpdateCronJobID(ctx context.Context, id, cronJobID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE reminders SET cron_job_id = ? WHERE id = ?`,
		cronJobID, id)
	return err
}

// Delete removes a reminder by ID (only if owned by ownerID).
func (s *Store) Delete(ctx context.Context, id, ownerID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM reminders WHERE id = ? AND owner_id = ?`, id, ownerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("reminder %s not found", id)
	}
	return nil
}

// DeleteByOwner removes all reminders for a user, returns count deleted.
func (s *Store) DeleteByOwner(ctx context.Context, ownerID string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM reminders WHERE owner_id = ?`, ownerID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) queryReminders(ctx context.Context, query string, args ...any) ([]*Reminder, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Reminder
	for rows.Next() {
		r := &Reminder{}
		if err := rows.Scan(&r.ID, &r.OwnerID, &r.Message, &r.FireAt, &r.Recurring, &r.SessionID, &r.CronJobID, &r.Status, &r.CreatedAt, &r.FiredAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
