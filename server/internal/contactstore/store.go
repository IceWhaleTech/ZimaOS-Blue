// Package contactstore provides SQLite persistence for contacts.
package contactstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"
)

// Contact represents a persisted contact.
type Contact struct {
	ID           string            `json:"id"`
	OwnerID      string            `json:"owner_id"`
	FirstName    string            `json:"first_name"`
	LastName     string            `json:"last_name"`
	DisplayName  string            `json:"display_name"`
	Email        []string          `json:"email,omitempty"`
	Phone        []string          `json:"phone,omitempty"`
	Address      string            `json:"address,omitempty"`
	Organization string            `json:"organization,omitempty"`
	Title        string            `json:"title,omitempty"`
	Birthday     string            `json:"birthday,omitempty"`
	Notes        string            `json:"notes,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Custom       map[string]string `json:"custom,omitempty"`
	Created      time.Time         `json:"created"`
	Updated      time.Time         `json:"updated"`
}

// Store provides SQLite-backed contact persistence.
type Store struct {
	db     *sql.DB
	readDB *sql.DB
}

// NewStore creates the contacts table and returns a Store.
func NewStore(db *sql.DB) (*Store, error) {
	return NewStoreWithReadDB(db, db)
}

// NewStoreWithReadDB creates the contacts table and returns a Store with
// separate write and read database handles.
func NewStoreWithReadDB(writeDB, readDB *sql.DB) (*Store, error) {
	if readDB == nil {
		readDB = writeDB
	}
	_, err := writeDB.Exec(`
		CREATE TABLE IF NOT EXISTS contacts (
			id TEXT PRIMARY KEY,
			owner_id TEXT NOT NULL DEFAULT 'default',
			first_name TEXT DEFAULT '',
			last_name TEXT DEFAULT '',
			display_name TEXT NOT NULL,
			email TEXT DEFAULT '[]',
			phone TEXT DEFAULT '[]',
			address TEXT DEFAULT '',
			organization TEXT DEFAULT '',
			title TEXT DEFAULT '',
			birthday TEXT DEFAULT '',
			notes TEXT DEFAULT '',
			tags TEXT DEFAULT '[]',
			custom TEXT DEFAULT '{}',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_contacts_owner ON contacts(owner_id);
	`)
	if err != nil {
		return nil, err
	}
	return &Store{db: writeDB, readDB: readDB}, nil
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
	return z.TableContext(ctx, s.db, "contacts")
}

func (s *Store) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "contacts")
}

type contactRow struct {
	ID           string `json:"id" zorm:"id"`
	OwnerID      string `json:"owner_id" zorm:"owner_id"`
	FirstName    string `json:"first_name" zorm:"first_name"`
	LastName     string `json:"last_name" zorm:"last_name"`
	DisplayName  string `json:"display_name" zorm:"display_name"`
	Email        string `json:"email" zorm:"email"`
	Phone        string `json:"phone" zorm:"phone"`
	Address      string `json:"address" zorm:"address"`
	Organization string `json:"organization" zorm:"organization"`
	Title        string `json:"title" zorm:"title"`
	Birthday     string `json:"birthday" zorm:"birthday"`
	Notes        string `json:"notes" zorm:"notes"`
	Tags         string `json:"tags" zorm:"tags"`
	Custom       string `json:"custom" zorm:"custom"`
	CreatedAt    string `json:"created_at" zorm:"created_at"`
	UpdatedAt    string `json:"updated_at" zorm:"updated_at"`
}

func resolveOwnerScope(ownerID []string, fallback string) string {
	if len(ownerID) > 0 {
		return strings.TrimSpace(ownerID[0])
	}
	return strings.TrimSpace(fallback)
}

func formatContactTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseContactTime(raw string) time.Time {
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

func rowToContact(row contactRow) (*Contact, error) {
	contact := &Contact{
		ID:           row.ID,
		OwnerID:      row.OwnerID,
		FirstName:    row.FirstName,
		LastName:     row.LastName,
		DisplayName:  row.DisplayName,
		Address:      row.Address,
		Organization: row.Organization,
		Title:        row.Title,
		Birthday:     row.Birthday,
		Notes:        row.Notes,
		Created:      parseContactTime(row.CreatedAt),
		Updated:      parseContactTime(row.UpdatedAt),
	}
	if strings.TrimSpace(row.Email) != "" {
		if err := json.Unmarshal([]byte(row.Email), &contact.Email); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(row.Phone) != "" {
		if err := json.Unmarshal([]byte(row.Phone), &contact.Phone); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(row.Tags) != "" {
		if err := json.Unmarshal([]byte(row.Tags), &contact.Tags); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(row.Custom) != "" {
		if err := json.Unmarshal([]byte(row.Custom), &contact.Custom); err != nil {
			return nil, err
		}
	}
	return contact, nil
}

func rowsToContacts(rows []contactRow) ([]*Contact, error) {
	contacts := make([]*Contact, 0, len(rows))
	for i := range rows {
		contact, err := rowToContact(rows[i])
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, contact)
	}
	return contacts, nil
}

// Create inserts a new contact.
func (s *Store) Create(ctx context.Context, c *Contact) error {
	emailJSON, err := json.Marshal(c.Email)
	if err != nil {
		return err
	}
	phoneJSON, err := json.Marshal(c.Phone)
	if err != nil {
		return err
	}
	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return err
	}
	customJSON, err := json.Marshal(c.Custom)
	if err != nil {
		return err
	}
	_, err = s.table(ctx).Insert(map[string]interface{}{
		"id":           c.ID,
		"owner_id":     c.OwnerID,
		"first_name":   c.FirstName,
		"last_name":    c.LastName,
		"display_name": c.DisplayName,
		"email":        string(emailJSON),
		"phone":        string(phoneJSON),
		"address":      c.Address,
		"organization": c.Organization,
		"title":        c.Title,
		"birthday":     c.Birthday,
		"notes":        c.Notes,
		"tags":         string(tagsJSON),
		"custom":       string(customJSON),
		"created_at":   formatContactTime(c.Created),
		"updated_at":   formatContactTime(c.Updated),
	})
	return err
}

// Get retrieves a contact by ID.
func (s *Store) Get(ctx context.Context, id string, ownerID ...string) (*Contact, error) {
	scopedOwnerID := resolveOwnerScope(ownerID, "")
	conds := []interface{}{z.Eq("id", id)}
	if scopedOwnerID != "" {
		conds = append(conds, z.Eq("owner_id", scopedOwnerID))
	}

	var rows []contactRow
	_, err := s.readTable(ctx).Select(&rows, z.Where(conds...), z.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rowToContact(rows[0])
}

// Update modifies an existing contact.
func (s *Store) Update(ctx context.Context, c *Contact, ownerID ...string) error {
	emailJSON, err := json.Marshal(c.Email)
	if err != nil {
		return err
	}
	phoneJSON, err := json.Marshal(c.Phone)
	if err != nil {
		return err
	}
	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return err
	}
	customJSON, err := json.Marshal(c.Custom)
	if err != nil {
		return err
	}
	scopedOwnerID := resolveOwnerScope(ownerID, c.OwnerID)
	conds := []interface{}{z.Eq("id", c.ID)}
	if scopedOwnerID != "" {
		conds = append(conds, z.Eq("owner_id", scopedOwnerID))
	}
	affected, err := s.table(ctx).Update(
		z.V{
			"first_name":   c.FirstName,
			"last_name":    c.LastName,
			"display_name": c.DisplayName,
			"email":        string(emailJSON),
			"phone":        string(phoneJSON),
			"address":      c.Address,
			"organization": c.Organization,
			"title":        c.Title,
			"birthday":     c.Birthday,
			"notes":        c.Notes,
			"tags":         string(tagsJSON),
			"custom":       string(customJSON),
			"updated_at":   formatContactTime(c.Updated),
		},
		z.Fields("first_name", "last_name", "display_name", "email", "phone", "address", "organization", "title", "birthday", "notes", "tags", "custom", "updated_at"),
		z.Where(conds...),
	)
	if err != nil {
		return err
	}
	if scopedOwnerID != "" && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// Delete removes a contact by ID.
func (s *Store) Delete(ctx context.Context, id string, ownerID ...string) error {
	scopedOwnerID := resolveOwnerScope(ownerID, "")
	conds := []interface{}{z.Eq("id", id)}
	if scopedOwnerID != "" {
		conds = append(conds, z.Eq("owner_id", scopedOwnerID))
	}
	affected, err := s.table(ctx).Delete(z.Where(conds...))
	if err != nil {
		return err
	}
	if scopedOwnerID != "" && affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListByOwner returns all contacts for an owner.
func (s *Store) ListByOwner(ctx context.Context, ownerID string) ([]*Contact, error) {
	var rows []contactRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("owner_id", ownerID)),
		z.OrderBy("display_name ASC"),
	)
	if err != nil {
		return nil, err
	}
	return rowsToContacts(rows)
}

// Search finds contacts matching a query.
func (s *Store) Search(ctx context.Context, ownerID, query string) ([]*Contact, error) {
	like := "%" + query + "%"
	var rows []contactRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(
			z.Eq("owner_id", ownerID),
			z.Or(
				z.Like("display_name", like),
				z.Like("first_name", like),
				z.Like("last_name", like),
				z.Like("email", like),
				z.Like("phone", like),
				z.Like("organization", like),
				z.Like("notes", like),
			),
		),
		z.OrderBy("display_name ASC"),
	)
	if err != nil {
		return nil, err
	}
	return rowsToContacts(rows)
}
