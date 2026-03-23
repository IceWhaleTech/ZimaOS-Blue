package tools

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BrowserSiteAllowlistStore persists approved browser origins per user.
type BrowserSiteAllowlistStore struct {
	db *sql.DB
}

// BrowserSiteAllowlistEntry represents a single approved browser origin.
type BrowserSiteAllowlistEntry struct {
	ID         string
	Origin     string
	AddedAt    time.Time
	LastUsed   time.Time
	ApprovedBy string
}

// NewBrowserSiteAllowlistStore creates the browser site allowlist table.
func NewBrowserSiteAllowlistStore(db *sql.DB) (*BrowserSiteAllowlistStore, error) {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS browser_site_allowlist (
		id          TEXT PRIMARY KEY,
		origin      TEXT NOT NULL,
		added_at    TEXT NOT NULL,
		last_used   TEXT NOT NULL,
		approved_by TEXT NOT NULL DEFAULT '',
		UNIQUE(origin, approved_by)
	)`)
	if err != nil {
		return nil, err
	}
	return &BrowserSiteAllowlistStore{db: db}, nil
}

func normalizeBrowserSiteApprovedBy(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "default"
	}
	return userID
}

// NormalizeBrowserSiteOrigin converts a browser URL into a stable origin
// string (scheme + host + optional non-default port). Returns empty string
// when the input does not represent an http(s) origin.
func NormalizeBrowserSiteOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return ""
	}
	port := strings.TrimSpace(parsed.Port())
	switch {
	case port == "":
		return scheme + "://" + host
	case scheme == "http" && port == "80":
		return scheme + "://" + host
	case scheme == "https" && port == "443":
		return scheme + "://" + host
	default:
		return scheme + "://" + host + ":" + port
	}
}

// Add inserts an approved origin for a specific user.
func (s *BrowserSiteAllowlistStore) Add(rawURLOrOrigin, userID string) error {
	origin := NormalizeBrowserSiteOrigin(rawURLOrOrigin)
	if origin == "" {
		return fmt.Errorf("invalid browser site origin")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO browser_site_allowlist (id, origin, added_at, last_used, approved_by) VALUES (?, ?, ?, ?, ?)`,
		uuid.New().String(),
		origin,
		now,
		now,
		normalizeBrowserSiteApprovedBy(userID),
	)
	return err
}

// Match returns the approved origin entry for a specific user, if any.
func (s *BrowserSiteAllowlistStore) Match(rawURLOrOrigin, userID string) *BrowserSiteAllowlistEntry {
	origin := NormalizeBrowserSiteOrigin(rawURLOrOrigin)
	if origin == "" {
		return nil
	}

	candidates := []string{normalizeBrowserSiteApprovedBy(userID)}
	if candidates[0] != "default" {
		candidates = append(candidates, "default")
	}
	for _, approvedBy := range candidates {
		row := s.db.QueryRow(
			`SELECT id, origin, added_at, last_used, approved_by FROM browser_site_allowlist WHERE origin = ? AND approved_by = ? LIMIT 1`,
			origin,
			approvedBy,
		)
		var entry BrowserSiteAllowlistEntry
		var addedAt string
		var lastUsed string
		if err := row.Scan(&entry.ID, &entry.Origin, &addedAt, &lastUsed, &entry.ApprovedBy); err != nil {
			continue
		}
		entry.AddedAt, _ = time.Parse(time.RFC3339, addedAt)
		entry.LastUsed, _ = time.Parse(time.RFC3339, lastUsed)

		now := time.Now().UTC().Format(time.RFC3339)
		_, _ = s.db.Exec(`UPDATE browser_site_allowlist SET last_used = ? WHERE id = ?`, now, entry.ID)

		return &entry
	}
	return nil
}

// List returns all approved browser origins.
func (s *BrowserSiteAllowlistStore) List() ([]BrowserSiteAllowlistEntry, error) {
	rows, err := s.db.Query(
		`SELECT id, origin, added_at, last_used, approved_by FROM browser_site_allowlist ORDER BY added_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BrowserSiteAllowlistEntry
	for rows.Next() {
		var entry BrowserSiteAllowlistEntry
		var addedAt string
		var lastUsed string
		if err := rows.Scan(&entry.ID, &entry.Origin, &addedAt, &lastUsed, &entry.ApprovedBy); err != nil {
			continue
		}
		entry.AddedAt, _ = time.Parse(time.RFC3339, addedAt)
		entry.LastUsed, _ = time.Parse(time.RFC3339, lastUsed)
		entries = append(entries, entry)
	}
	return entries, nil
}

// Delete removes an approved browser origin by ID.
func (s *BrowserSiteAllowlistStore) Delete(id string) error {
	_, err := s.db.Exec(`DELETE FROM browser_site_allowlist WHERE id = ?`, strings.TrimSpace(id))
	return err
}
