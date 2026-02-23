// Package contactstore provides SQLite persistence for contacts.
package contactstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
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
	db *sql.DB
}

// NewStore creates the contacts table and returns a Store.
func NewStore(db *sql.DB) (*Store, error) {
	_, err := db.Exec(`
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
	return &Store{db: db}, nil
}

// Create inserts a new contact.
func (s *Store) Create(ctx context.Context, c *Contact) error {
	emailJSON, _ := json.Marshal(c.Email)
	phoneJSON, _ := json.Marshal(c.Phone)
	tagsJSON, _ := json.Marshal(c.Tags)
	customJSON, _ := json.Marshal(c.Custom)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contacts (id, owner_id, first_name, last_name, display_name, email, phone,
		 address, organization, title, birthday, notes, tags, custom, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.OwnerID, c.FirstName, c.LastName, c.DisplayName,
		string(emailJSON), string(phoneJSON), c.Address, c.Organization, c.Title,
		c.Birthday, c.Notes, string(tagsJSON), string(customJSON), c.Created, c.Updated)
	return err
}

// Get retrieves a contact by ID.
func (s *Store) Get(ctx context.Context, id string) (*Contact, error) {
	var c Contact
	var emailJSON, phoneJSON, tagsJSON, customJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, owner_id, first_name, last_name, display_name, email, phone,
		 address, organization, title, birthday, notes, tags, custom, created_at, updated_at
		 FROM contacts WHERE id = ?`, id).
		Scan(&c.ID, &c.OwnerID, &c.FirstName, &c.LastName, &c.DisplayName,
			&emailJSON, &phoneJSON, &c.Address, &c.Organization, &c.Title,
			&c.Birthday, &c.Notes, &tagsJSON, &customJSON, &c.Created, &c.Updated)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(emailJSON), &c.Email)
	json.Unmarshal([]byte(phoneJSON), &c.Phone)
	json.Unmarshal([]byte(tagsJSON), &c.Tags)
	json.Unmarshal([]byte(customJSON), &c.Custom)
	return &c, nil
}

// Update modifies an existing contact.
func (s *Store) Update(ctx context.Context, c *Contact) error {
	emailJSON, _ := json.Marshal(c.Email)
	phoneJSON, _ := json.Marshal(c.Phone)
	tagsJSON, _ := json.Marshal(c.Tags)
	customJSON, _ := json.Marshal(c.Custom)
	_, err := s.db.ExecContext(ctx,
		`UPDATE contacts SET first_name = ?, last_name = ?, display_name = ?, email = ?, phone = ?,
		 address = ?, organization = ?, title = ?, birthday = ?, notes = ?, tags = ?, custom = ?,
		 updated_at = ? WHERE id = ?`,
		c.FirstName, c.LastName, c.DisplayName, string(emailJSON), string(phoneJSON),
		c.Address, c.Organization, c.Title, c.Birthday, c.Notes,
		string(tagsJSON), string(customJSON), c.Updated, c.ID)
	return err
}

// Delete removes a contact by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM contacts WHERE id = ?`, id)
	return err
}

// ListByOwner returns all contacts for an owner.
func (s *Store) ListByOwner(ctx context.Context, ownerID string) ([]*Contact, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_id, first_name, last_name, display_name, email, phone,
		 address, organization, title, birthday, notes, tags, custom, created_at, updated_at
		 FROM contacts WHERE owner_id = ? ORDER BY display_name ASC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContacts(rows)
}

// Search finds contacts matching a query.
func (s *Store) Search(ctx context.Context, ownerID, query string) ([]*Contact, error) {
	like := "%" + query + "%"
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, owner_id, first_name, last_name, display_name, email, phone,
		 address, organization, title, birthday, notes, tags, custom, created_at, updated_at
		 FROM contacts WHERE owner_id = ? AND (display_name LIKE ? OR first_name LIKE ? OR last_name LIKE ?
		 OR email LIKE ? OR phone LIKE ? OR organization LIKE ? OR notes LIKE ?)
		 ORDER BY display_name ASC`,
		ownerID, like, like, like, like, like, like, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanContacts(rows)
}

func scanContacts(rows *sql.Rows) ([]*Contact, error) {
	var contacts []*Contact
	for rows.Next() {
		var c Contact
		var emailJSON, phoneJSON, tagsJSON, customJSON string
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.FirstName, &c.LastName, &c.DisplayName,
			&emailJSON, &phoneJSON, &c.Address, &c.Organization, &c.Title,
			&c.Birthday, &c.Notes, &tagsJSON, &customJSON, &c.Created, &c.Updated); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(emailJSON), &c.Email)
		json.Unmarshal([]byte(phoneJSON), &c.Phone)
		json.Unmarshal([]byte(tagsJSON), &c.Tags)
		json.Unmarshal([]byte(customJSON), &c.Custom)
		contacts = append(contacts, &c)
	}
	return contacts, rows.Err()
}
