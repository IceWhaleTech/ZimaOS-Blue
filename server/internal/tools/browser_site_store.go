package tools

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
)

// BrowserSiteAllowlistStore persists approved browser origins per user.
type BrowserSiteAllowlistStore struct {
	db     *sql.DB
	readDB *sql.DB
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
	return NewBrowserSiteAllowlistStoreWithReadDB(db, db)
}

// NewBrowserSiteAllowlistStoreWithReadDB creates the browser site allowlist
// table with separate write and read database handles.
func NewBrowserSiteAllowlistStoreWithReadDB(writeDB, readDB *sql.DB) (*BrowserSiteAllowlistStore, error) {
	if readDB == nil {
		readDB = writeDB
	}
	_, err := writeDB.Exec(`CREATE TABLE IF NOT EXISTS browser_site_allowlist (
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
	return &BrowserSiteAllowlistStore{db: writeDB, readDB: readDB}, nil
}

func (s *BrowserSiteAllowlistStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *BrowserSiteAllowlistStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "browser_site_allowlist")
}

func (s *BrowserSiteAllowlistStore) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "browser_site_allowlist")
}

type browserSiteAllowlistRow struct {
	ID         string `json:"id" zorm:"id"`
	Origin     string `json:"origin" zorm:"origin"`
	AddedAt    string `json:"added_at" zorm:"added_at"`
	LastUsed   string `json:"last_used" zorm:"last_used"`
	ApprovedBy string `json:"approved_by" zorm:"approved_by"`
}

func normalizeBrowserSiteApprovedBy(userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "default"
	}
	return userID
}

func rowToBrowserSiteAllowlistEntry(row browserSiteAllowlistRow) BrowserSiteAllowlistEntry {
	entry := BrowserSiteAllowlistEntry{
		ID:         row.ID,
		Origin:     row.Origin,
		ApprovedBy: row.ApprovedBy,
	}
	entry.AddedAt, _ = time.Parse(time.RFC3339, row.AddedAt)
	entry.LastUsed, _ = time.Parse(time.RFC3339, row.LastUsed)
	return entry
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
	_, err := s.table(context.Background()).InsertIgnore(map[string]interface{}{
		"id":          uuid.New().String(),
		"origin":      origin,
		"added_at":    now,
		"last_used":   now,
		"approved_by": normalizeBrowserSiteApprovedBy(userID),
	})
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
	ctx := context.Background()
	for _, approvedBy := range candidates {
		var rows []browserSiteAllowlistRow
		_, err := s.readTable(ctx).Select(&rows,
			z.Where(z.Eq("origin", origin), z.Eq("approved_by", approvedBy)),
			z.Limit(1),
		)
		if err != nil || len(rows) == 0 {
			continue
		}
		entry := rowToBrowserSiteAllowlistEntry(rows[0])

		now := time.Now().UTC().Format(time.RFC3339)
		_, _ = s.table(ctx).Update(
			z.V{"last_used": now},
			z.Fields("last_used"),
			z.Where(z.Eq("id", entry.ID)),
		)

		return &entry
	}
	return nil
}

// List returns all approved browser origins.
func (s *BrowserSiteAllowlistStore) List() ([]BrowserSiteAllowlistEntry, error) {
	var rows []browserSiteAllowlistRow
	_, err := s.readTable(context.Background()).Select(&rows, z.OrderBy("added_at"))
	if err != nil {
		return nil, err
	}

	entries := make([]BrowserSiteAllowlistEntry, 0, len(rows))
	for i := range rows {
		entries = append(entries, rowToBrowserSiteAllowlistEntry(rows[i]))
	}
	return entries, nil
}

// Delete removes an approved browser origin by ID.
func (s *BrowserSiteAllowlistStore) Delete(id string) error {
	_, err := s.table(context.Background()).Delete(z.Where(z.Eq("id", strings.TrimSpace(id))))
	return err
}
