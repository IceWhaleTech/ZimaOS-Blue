// Package push provides persistent push notification management with SQLite storage.
package push

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
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
	db     *sql.DB
	readDB *sql.DB
}

// NewStore creates a new push notification store and ensures the table exists.
func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

// NewStoreWithReadDB creates a new push notification store with separate
// write and read database handles.
func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	if readDB == nil {
		readDB = writeDB
	}
	s := &Store{db: writeDB, readDB: readDB}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("push store migration: %w", err)
	}
	return s, nil
}

func (s *Store) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Store) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "push")
}

func (s *Store) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "push")
}

type pushRow struct {
	ID        string     `json:"id" zorm:"id"`
	OwnerID   string     `json:"owner_id" zorm:"owner_id"`
	Message   string     `json:"message" zorm:"message"`
	FireAt    time.Time  `json:"fire_at" zorm:"fire_at"`
	Recurring string     `json:"recurring" zorm:"recurring"`
	UntilAt   *time.Time `json:"until_at" zorm:"until_at"`
	SessionID string     `json:"session_id" zorm:"session_id"`
	CronJobID string     `json:"cron_job_id" zorm:"cron_job_id"`
	Status    string     `json:"status" zorm:"status"`
	CreatedAt time.Time  `json:"created_at" zorm:"created_at"`
	FiredAt   *time.Time `json:"fired_at" zorm:"fired_at"`
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

func nullablePushTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func rowToPushNotification(row pushRow) *PushNotification {
	return &PushNotification{
		ID:        row.ID,
		OwnerID:   row.OwnerID,
		Message:   row.Message,
		FireAt:    row.FireAt,
		Recurring: row.Recurring,
		UntilAt:   row.UntilAt,
		SessionID: row.SessionID,
		CronJobID: row.CronJobID,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		FiredAt:   row.FiredAt,
	}
}

func rowsToPushNotifications(rows []pushRow) []*PushNotification {
	result := make([]*PushNotification, 0, len(rows))
	for i := range rows {
		result = append(result, rowToPushNotification(rows[i]))
	}
	return result
}

// Create inserts a new push notification.
func (s *Store) Create(ctx context.Context, r *PushNotification) error {
	if r.CreatedAt.IsZero() {
		r.CreatedAt = timeutil.NowTime()
	}
	if r.Status == "" {
		r.Status = StatusPending
	}
	_, err := s.table(ctx).Insert(map[string]interface{}{
		"id":          r.ID,
		"owner_id":    r.OwnerID,
		"message":     r.Message,
		"fire_at":     r.FireAt,
		"recurring":   r.Recurring,
		"until_at":    nullablePushTime(r.UntilAt),
		"session_id":  r.SessionID,
		"cron_job_id": r.CronJobID,
		"status":      r.Status,
		"created_at":  r.CreatedAt,
	})
	return err
}

// Get retrieves a push notification by ID.
func (s *Store) Get(ctx context.Context, id string, ownerID ...string) (*PushNotification, error) {
	scopedOwnerID := resolveOwnerScope(ownerID)
	conds := []interface{}{z.Eq("id", id)}
	if scopedOwnerID != "" {
		conds = append(conds, z.Eq("owner_id", scopedOwnerID))
	}
	var rows []pushRow
	_, err := s.readTable(ctx).Select(&rows, z.Where(conds...), z.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rowToPushNotification(rows[0]), nil
}

// ListByOwner returns push notifications for a user, optionally filtered by status.
func (s *Store) ListByOwner(ctx context.Context, ownerID string, status ...string) ([]*PushNotification, error) {
	conds := []interface{}{z.Eq("owner_id", ownerID)}
	if len(status) > 0 && status[0] != "" {
		conds = append(conds, z.Eq("status", status[0]))
	}
	return s.queryPush(ctx, z.Where(conds...), z.OrderBy("fire_at ASC"))
}

// ListPending returns all pending push notifications (for restart recovery).
func (s *Store) ListPending(ctx context.Context) ([]*PushNotification, error) {
	return s.queryPush(ctx,
		z.Where(z.Eq("status", StatusPending)),
		z.OrderBy("fire_at ASC"),
	)
}

// UpdateStatus updates the status and optionally the fired_at timestamp.
func (s *Store) UpdateStatus(ctx context.Context, id, status string, firedAt *time.Time) error {
	_, err := s.table(ctx).Update(
		z.V{
			"status":   status,
			"fired_at": nullablePushTime(firedAt),
		},
		z.Fields("status", "fired_at"),
		z.Where(z.Eq("id", id)),
	)
	return err
}

// UpdateCronJobID sets the cron_job_id for a push notification.
func (s *Store) UpdateCronJobID(ctx context.Context, id, cronJobID string) error {
	_, err := s.table(ctx).Update(
		z.V{"cron_job_id": cronJobID},
		z.Fields("cron_job_id"),
		z.Where(z.Eq("id", id)),
	)
	return err
}

// UpdateSchedule updates the next fire time, cron job ID, and status together.
func (s *Store) UpdateSchedule(ctx context.Context, id string, fireAt time.Time, cronJobID, status string, firedAt *time.Time) error {
	_, err := s.table(ctx).Update(
		z.V{
			"fire_at":     fireAt,
			"cron_job_id": cronJobID,
			"status":      status,
			"fired_at":    nullablePushTime(firedAt),
		},
		z.Fields("fire_at", "cron_job_id", "status", "fired_at"),
		z.Where(z.Eq("id", id)),
	)
	return err
}

// Delete removes a push notification by ID (only if owned by ownerID).
func (s *Store) Delete(ctx context.Context, id, ownerID string) error {
	n, err := s.table(ctx).Delete(
		z.Where(z.Eq("id", id), z.Eq("owner_id", ownerID)),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("push notification %s not found", id)
	}
	return nil
}

// DeleteByOwner removes all push notifications for a user, returns count deleted.
func (s *Store) DeleteByOwner(ctx context.Context, ownerID string) (int64, error) {
	n, err := s.table(ctx).Delete(
		z.Where(z.Eq("owner_id", ownerID)),
	)
	if err != nil {
		return 0, err
	}
	return int64(n), nil
}

func (s *Store) queryPush(ctx context.Context, opts ...z.ZormItem) ([]*PushNotification, error) {
	var rows []pushRow
	_, err := s.readTable(ctx).Select(&rows, opts...)
	if err != nil {
		return nil, err
	}
	return rowsToPushNotifications(rows), nil
}
