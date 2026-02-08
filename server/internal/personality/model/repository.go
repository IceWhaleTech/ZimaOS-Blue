package model

import (
	"database/sql"
	"fmt"
)

// Repository handles personality data persistence
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Create saves a personality to database
func (r *Repository) Create(p *Personality) error {
	if err := p.Validate(); err != nil {
		return err
	}

	_, err := r.db.Exec(
		`INSERT INTO personalities (id, name, description, system_prompt, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, p.SystemPrompt, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create personality: %w", err)
	}

	// Save traits
	for _, trait := range p.Traits {
		_, err := r.db.Exec(
			`INSERT INTO personality_traits (personality_id, key, value, weight)
			 VALUES (?, ?, ?, ?)`,
			p.ID, trait.Key, trait.Value, trait.Weight,
		)
		if err != nil {
			return fmt.Errorf("failed to save trait: %w", err)
		}
	}

	return nil
}

// GetByID retrieves a personality by ID
func (r *Repository) GetByID(id string) (*Personality, error) {
	p := &Personality{}
	err := r.db.QueryRow(
		`SELECT id, name, description, system_prompt, is_active, created_at, updated_at
		 FROM personalities WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.SystemPrompt, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("personality not found: %w", err)
	}

	// Load traits
	rows, err := r.db.Query(
		`SELECT key, value, weight FROM personality_traits WHERE personality_id = ?`,
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var trait PersonalityTrait
		if err := rows.Scan(&trait.Key, &trait.Value, &trait.Weight); err != nil {
			return nil, err
		}
		p.Traits = append(p.Traits, trait)
	}

	return p, nil
}

// GetActive retrieves the active personality
func (r *Repository) GetActive() (*Personality, error) {
	p := &Personality{}
	err := r.db.QueryRow(
		`SELECT id, name, description, system_prompt, is_active, created_at, updated_at
		 FROM personalities WHERE is_active = 1 LIMIT 1`,
	).Scan(&p.ID, &p.Name, &p.Description, &p.SystemPrompt, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("no active personality: %w", err)
	}

	// Load traits
	rows, err := r.db.Query(
		`SELECT key, value, weight FROM personality_traits WHERE personality_id = ?`,
		p.ID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var trait PersonalityTrait
		if err := rows.Scan(&trait.Key, &trait.Value, &trait.Weight); err != nil {
			return nil, err
		}
		p.Traits = append(p.Traits, trait)
	}

	return p, nil
}

// Activate sets a personality as active
func (r *Repository) Activate(id string) error {
	_, err := r.db.Exec(`UPDATE personalities SET is_active = 0`)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`UPDATE personalities SET is_active = 1 WHERE id = ?`, id)
	return err
}

// List retrieves all personalities
func (r *Repository) List() ([]*Personality, error) {
	rows, err := r.db.Query(
		`SELECT id, name, description, system_prompt, created_at, updated_at
		 FROM personalities ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var personalities []*Personality
	for rows.Next() {
		p := &Personality{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.SystemPrompt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		personalities = append(personalities, p)
	}

	return personalities, nil
}

// Update updates a personality
func (r *Repository) Update(p *Personality) error {
	if err := p.Validate(); err != nil {
		return err
	}

	_, err := r.db.Exec(
		`UPDATE personalities SET name=?, description=?, system_prompt=?, updated_at=?
		 WHERE id=?`,
		p.Name, p.Description, p.SystemPrompt, p.UpdatedAt, p.ID,
	)
	return err
}

// Delete deletes a personality
func (r *Repository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM personalities WHERE id=?`, id)
	return err
}
