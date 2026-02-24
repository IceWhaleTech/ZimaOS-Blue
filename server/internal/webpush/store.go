package webpush

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
	db *sql.DB
}

// NewStore creates a new push subscription store.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("webpush store migration: %w", err)
	}
	return s, nil
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
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO push_subscriptions (user_id, endpoint, key_p256dh, key_auth, created_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(endpoint) DO UPDATE SET user_id=?, key_p256dh=?, key_auth=?`,
		userID, sub.Endpoint, sub.KeyP256dh, sub.KeyAuth, timeutil.NowTime(),
		userID, sub.KeyP256dh, sub.KeyAuth,
	)
	return err
}

// Unsubscribe removes a subscription by endpoint.
func (s *Store) Unsubscribe(ctx context.Context, endpoint string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}

// ListByUser returns all subscriptions for a user.
func (s *Store) ListByUser(ctx context.Context, userID string) ([]*Subscription, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, endpoint, key_p256dh, key_auth, created_at
		 FROM push_subscriptions WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Subscription
	for rows.Next() {
		sub := &Subscription{}
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.KeyP256dh, &sub.KeyAuth, &sub.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, sub)
	}
	return result, rows.Err()
}

// ListAll returns all subscriptions (for broadcast).
func (s *Store) ListAll(ctx context.Context) ([]*Subscription, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, endpoint, key_p256dh, key_auth, created_at
		 FROM push_subscriptions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Subscription
	for rows.Next() {
		sub := &Subscription{}
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.Endpoint, &sub.KeyP256dh, &sub.KeyAuth, &sub.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, sub)
	}
	return result, rows.Err()
}

// DeleteByEndpoint removes a subscription by endpoint (for stale cleanup).
func (s *Store) DeleteByEndpoint(ctx context.Context, endpoint string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
	return err
}
