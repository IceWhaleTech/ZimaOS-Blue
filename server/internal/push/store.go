// Package push provides persistent push notification management with SQLite storage.
package push

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Status constants for push notification lifecycle.
const (
	StatusPending   = "pending"
	StatusFired     = "fired"
	StatusCancelled = "cancelled"
)

// PushNotification represents a persistent push notification entry.
type PushNotification struct {
	ID        string     `json:"id"`
	OwnerID   string     `json:"owner_id"`
	Message   string     `json:"message"`
	FireAt    time.Time  `json:"fire_at"`
	Recurring string     `json:"recurring,omitempty"` // "", "daily", "weekly", "monthly"
	UntilAt   *time.Time `json:"until_at,omitempty"`
	SessionID string     `json:"session_id,omitempty"`
	CronJobID string     `json:"cron_job_id,omitempty"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	FiredAt   *time.Time `json:"fired_at,omitempty"`
}

// Store provides SQLite-backed persistence for push notifications.
type Store struct {
	db *sql.DB
}

// NewStore creates a new push notification store and ensures the table exists.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("push store migration: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	// Check if existing table has the expected schema (fire_at column).
	// If the table exists with an old schema, drop and recreate.
	var hasFireAt bool
	var hasUntilAt bool
	rows, err := s.db.Query(`PRAGMA table_info(push)`)
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
			if name == "until_at" {
				hasUntilAt = true
			}
		}
		rows.Close()
	}

	// Table exists but missing fire_at → old schema, drop it
	if !hasFireAt {
		s.db.Exec(`DROP TABLE IF EXISTS push`)
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS push (
			id TEXT PRIMARY KEY,
			owner_id TEXT NOT NULL,
			message TEXT NOT NULL,
			fire_at DATETIME NOT NULL,
			recurring TEXT DEFAULT '',
			until_at DATETIME,
			session_id TEXT DEFAULT '',
			cron_job_id TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL,
			fired_at DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_push_owner ON push(owner_id, status);
		CREATE INDEX IF NOT EXISTS idx_push_fire_at ON push(fire_at);
	`)
	if err != nil {
		return err
	}
	if !hasUntilAt {
		if _, err := s.db.Exec(`ALTER TABLE push ADD COLUMN until_at DATETIME`); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	return nil
}

func resolveOwnerScope(ownerID []string) string {
	if len(ownerID) == 0 {
		return ""
	}
	return strings.TrimSpace(ownerID[0])
}

// Create inserts a new push notification.
func (s *Store) Create(ctx context.Context, r *PushNotification) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = timeutil.NowTime()
	}
	if r.Status == "" {
		r.Status = StatusPending
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO push (id, owner_id, message, fire_at, recurring, until_at, session_id, cron_job_id, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.OwnerID, r.Message, r.FireAt, r.Recurring, r.UntilAt, r.SessionID, r.CronJobID, r.Status, r.CreatedAt,
	)
	return err
}

// Get retrieves a push notification by ID.
func (s *Store) Get(ctx context.Context, id string, ownerID ...string) (*PushNotification, error) {
	r := &PushNotification{}
	scopedOwnerID := resolveOwnerScope(ownerID)
	var err error
	if scopedOwnerID != "" {
		err = s.db.QueryRowContext(ctx,
			`SELECT id, owner_id, message, fire_at, recurring, until_at, session_id, cron_job_id, status, created_at, fired_at
			 FROM push WHERE id = ? AND owner_id = ?`, id, scopedOwnerID,
		).Scan(&r.ID, &r.OwnerID, &r.Message, &r.FireAt, &r.Recurring, &r.UntilAt, &r.SessionID, &r.CronJobID, &r.Status, &r.CreatedAt, &r.FiredAt)
	} else {
		err = s.db.QueryRowContext(ctx,
			`SELECT id, owner_id, message, fire_at, recurring, until_at, session_id, cron_job_id, status, created_at, fired_at
			 FROM push WHERE id = ?`, id,
		).Scan(&r.ID, &r.OwnerID, &r.Message, &r.FireAt, &r.Recurring, &r.UntilAt, &r.SessionID, &r.CronJobID, &r.Status, &r.CreatedAt, &r.FiredAt)
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

// ListByOwner returns push notifications for a user, optionally filtered by status.
func (s *Store) ListByOwner(ctx context.Context, ownerID string, status ...string) ([]*PushNotification, error) {
	query := `SELECT id, owner_id, message, fire_at, recurring, until_at, session_id, cron_job_id, status, created_at, fired_at
	          FROM push WHERE owner_id = ?`
	args := []any{ownerID}

	if len(status) > 0 && status[0] != "" {
		query += " AND status = ?"
		args = append(args, status[0])
	}
	query += " ORDER BY fire_at ASC"

	return s.queryPush(ctx, query, args...)
}

// ListPending returns all pending push notifications (for restart recovery).
func (s *Store) ListPending(ctx context.Context) ([]*PushNotification, error) {
	return s.queryPush(ctx,
		`SELECT id, owner_id, message, fire_at, recurring, until_at, session_id, cron_job_id, status, created_at, fired_at
		 FROM push WHERE status = ? ORDER BY fire_at ASC`, StatusPending)
}

// UpdateStatus updates the status and optionally the fired_at timestamp.
func (s *Store) UpdateStatus(ctx context.Context, id, status string, firedAt *time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE push SET status = ?, fired_at = ? WHERE id = ?`,
		status, firedAt, id)
	return err
}

// UpdateCronJobID sets the cron_job_id for a push notification.
func (s *Store) UpdateCronJobID(ctx context.Context, id, cronJobID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE push SET cron_job_id = ? WHERE id = ?`,
		cronJobID, id)
	return err
}

// UpdateSchedule updates the next fire time, cron job ID, and status together.
func (s *Store) UpdateSchedule(ctx context.Context, id string, fireAt time.Time, cronJobID, status string, firedAt *time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE push SET fire_at = ?, cron_job_id = ?, status = ?, fired_at = ? WHERE id = ?`,
		fireAt, cronJobID, status, firedAt, id)
	return err
}

// Delete removes a push notification by ID (only if owned by ownerID).
func (s *Store) Delete(ctx context.Context, id, ownerID string) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM push WHERE id = ? AND owner_id = ?`, id, ownerID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("push notification %s not found", id)
	}
	return nil
}

// DeleteByOwner removes all push notifications for a user, returns count deleted.
func (s *Store) DeleteByOwner(ctx context.Context, ownerID string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM push WHERE owner_id = ?`, ownerID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) queryPush(ctx context.Context, query string, args ...any) ([]*PushNotification, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*PushNotification
	for rows.Next() {
		r := &PushNotification{}
		if err := rows.Scan(&r.ID, &r.OwnerID, &r.Message, &r.FireAt, &r.Recurring, &r.UntilAt, &r.SessionID, &r.CronJobID, &r.Status, &r.CreatedAt, &r.FiredAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
