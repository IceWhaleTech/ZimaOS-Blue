package webpush

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

// Subscription represents a Web Push subscription from a browser.
type Subscription struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Endpoint  string    `json:"endpoint"`
	KeyP256dh string    `json:"key_p256dh"`
	KeyAuth   string    `json:"key_auth"`
	CreatedAt time.Time `json:"created_at"`
}

// Store provides SQLite-backed persistence for push subscriptions.
type Store struct {
	db     *sql.DB
	readDB *sql.DB
}

// NewStore creates a new push subscription store.
func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

// NewStoreWithReadDB creates a new push subscription store with separate
// write and read database handles.
func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	if readDB == nil {
		readDB = writeDB
	}
	s := &Store{db: writeDB, readDB: readDB}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("webpush store migration: %w", err)
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
	return z.TableContext(ctx, s.db, "push_subscriptions")
}

func (s *Store) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "push_subscriptions")
}

type subscriptionRow struct {
	ID        int64  `json:"id" zorm:"id"`
	UserID    string `json:"user_id" zorm:"user_id"`
	Endpoint  string `json:"endpoint" zorm:"endpoint"`
	KeyP256dh string `json:"key_p256dh" zorm:"key_p256dh"`
	KeyAuth   string `json:"key_auth" zorm:"key_auth"`
	CreatedAt string `json:"created_at" zorm:"created_at"`
}

func formatSubscriptionTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseSubscriptionTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func rowToSubscription(row subscriptionRow) *Subscription {
	return &Subscription{
		ID:        row.ID,
		UserID:    row.UserID,
		Endpoint:  row.Endpoint,
		KeyP256dh: row.KeyP256dh,
		KeyAuth:   row.KeyAuth,
		CreatedAt: parseSubscriptionTime(row.CreatedAt),
	}
}

func rowsToSubscriptions(rows []subscriptionRow) []*Subscription {
	result := make([]*Subscription, 0, len(rows))
	for i := range rows {
		result = append(result, rowToSubscription(rows[i]))
	}
	return result
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS push_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT NOT NULL,
			endpoint TEXT NOT NULL UNIQUE,
			key_p256dh TEXT NOT NULL,
			key_auth TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_push_sub_user ON push_subscriptions(user_id);
	`)
	return err
}

// Subscribe upserts a push subscription for a user.
func (s *Store) Subscribe(ctx context.Context, userID string, sub *Subscription) error {
	_, err := s.table(ctx).Insert(
		map[string]interface{}{
			"user_id":    userID,
			"endpoint":   sub.Endpoint,
			"key_p256dh": sub.KeyP256dh,
			"key_auth":   sub.KeyAuth,
			"created_at": formatSubscriptionTime(timeutil.NowTime()),
		},
		z.OnConflictDoUpdateSet(
			[]string{"endpoint"},
			[]string{"user_id", "key_p256dh", "key_auth"},
		),
	)
	return err
}

// Unsubscribe removes a subscription by endpoint.
func (s *Store) Unsubscribe(ctx context.Context, endpoint string) error {
	_, err := s.table(ctx).Delete(z.Where(z.Eq("endpoint", endpoint)))
	return err
}

// ListByUser returns all subscriptions for a user.
func (s *Store) ListByUser(ctx context.Context, userID string) ([]*Subscription, error) {
	var rows []subscriptionRow
	_, err := s.readTable(ctx).Select(&rows, z.Where(z.Eq("user_id", userID)))
	if err != nil {
		return nil, err
	}
	return rowsToSubscriptions(rows), nil
}

// ListAll returns all subscriptions (for broadcast).
func (s *Store) ListAll(ctx context.Context) ([]*Subscription, error) {
	var rows []subscriptionRow
	_, err := s.readTable(ctx).Select(&rows)
	if err != nil {
		return nil, err
	}
	return rowsToSubscriptions(rows), nil
}

// DeleteByEndpoint removes a subscription by endpoint (for stale cleanup).
func (s *Store) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	_, err := s.table(ctx).Delete(z.Where(z.Eq("endpoint", endpoint)))
	return err
}
