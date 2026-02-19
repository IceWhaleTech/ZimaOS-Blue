package model

import (
	"database/sql"
	"fmt"
	"time"

	z "github.com/IceWhaleTech/zorm"
)

// Repository handles personality data persistence
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) personalities() *z.ZormTable { return z.Table(r.db, "personalities") }
func (r *Repository) traits() *z.ZormTable        { return z.Table(r.db, "personality_traits") }

type personalityRow struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type traitRow struct {
	PersonalityID string  `json:"personality_id"`
	Key           string  `json:"key"`
	Value         string  `json:"value"`
	Weight        float64 `json:"weight"`
}

func parsePersonalityTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z", s)
	}
	return t
}

func rowToPersonality(row personalityRow) *Personality {
	return &Personality{
		ID:           row.ID,
		Name:         row.Name,
		Description:  row.Description,
		SystemPrompt: row.SystemPrompt,
		IsActive:     row.IsActive,
		CreatedAt:    parsePersonalityTime(row.CreatedAt),
		UpdatedAt:    parsePersonalityTime(row.UpdatedAt),
	}
}

// Create saves a personality to database
func (r *Repository) Create(p *Personality) error {
	if err := p.Validate(); err != nil {
		return err
	}

	_, err := r.personalities().Insert(map[string]interface{}{
		"id":            p.ID,
		"name":          p.Name,
		"description":   p.Description,
		"system_prompt": p.SystemPrompt,
		"created_at":    p.CreatedAt,
		"updated_at":    p.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("failed to create personality: %w", err)
	}

	for _, trait := range p.Traits {
		_, err := r.traits().Insert(map[string]interface{}{
			"personality_id": p.ID,
			"key":            trait.Key,
			"value":          trait.Value,
			"weight":         trait.Weight,
		})
		if err != nil {
			return fmt.Errorf("failed to save trait: %w", err)
		}
	}

	return nil
}

// GetByID retrieves a personality by ID
func (r *Repository) GetByID(id string) (*Personality, error) {
	var rows []personalityRow
	_, err := r.personalities().Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("personality not found: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("personality not found: %s", id)
	}

	p := rowToPersonality(rows[0])

	var traitRows []traitRow
	_, err = r.traits().Select(&traitRows,
		z.Where(z.Eq("personality_id", id)),
	)
	if err != nil {
		return nil, err
	}
	for _, tr := range traitRows {
		p.Traits = append(p.Traits, PersonalityTrait{
			Key: tr.Key, Value: tr.Value, Weight: tr.Weight,
		})
	}

	return p, nil
}

// GetActive retrieves the active personality
func (r *Repository) GetActive() (*Personality, error) {
	var rows []personalityRow
	_, err := r.personalities().Select(&rows,
		z.Where(z.Eq("is_active", 1)),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("no active personality: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no active personality")
	}

	p := rowToPersonality(rows[0])

	var traitRows []traitRow
	_, err = r.traits().Select(&traitRows,
		z.Where(z.Eq("personality_id", p.ID)),
	)
	if err != nil {
		return nil, err
	}
	for _, tr := range traitRows {
		p.Traits = append(p.Traits, PersonalityTrait{
			Key: tr.Key, Value: tr.Value, Weight: tr.Weight,
		})
	}

	return p, nil
}

// Activate sets a personality as active
func (r *Repository) Activate(id string) error {
	_, err := r.personalities().Update(z.V{
		"is_active": 0,
	})
	if err != nil {
		return err
	}

	_, err = r.personalities().Update(z.V{
		"is_active": 1,
	}, z.Where(z.Eq("id", id)))
	return err
}

// List retrieves all personalities
func (r *Repository) List() ([]*Personality, error) {
	var rows []personalityRow
	_, err := r.personalities().Select(&rows,
		z.Fields("id", "name", "description", "system_prompt", "created_at", "updated_at"),
		z.OrderBy("created_at DESC"),
	)
	if err != nil {
		return nil, err
	}

	personalities := make([]*Personality, 0, len(rows))
	for _, row := range rows {
		personalities = append(personalities, rowToPersonality(row))
	}

	return personalities, nil
}

// Update updates a personality
func (r *Repository) Update(p *Personality) error {
	if err := p.Validate(); err != nil {
		return err
	}

	_, err := r.personalities().Update(z.V{
		"name":          p.Name,
		"description":   p.Description,
		"system_prompt": p.SystemPrompt,
		"updated_at":    p.UpdatedAt,
	}, z.Where(z.Eq("id", p.ID)))
	return err
}

// Delete deletes a personality
func (r *Repository) Delete(id string) error {
	_, err := r.personalities().Delete(z.Where(z.Eq("id", id)))
	return err
}
