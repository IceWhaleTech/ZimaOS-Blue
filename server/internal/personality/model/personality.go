package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Personality represents an AI assistant personality
type Personality struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	SystemPrompt string           `json:"system_prompt"`
	IsActive     bool             `json:"is_active"`
	Traits       []PersonalityTrait `json:"traits"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// PersonalityTrait represents a personality characteristic
type PersonalityTrait struct {
	Key    string  `json:"key"`
	Value  string  `json:"value"`
	Weight float64 `json:"weight"`
}

// NewPersonality creates a new personality
func NewPersonality(name, description, systemPrompt string) *Personality {
	return &Personality{
		ID:           uuid.New().String(),
		Name:         name,
		Description:  description,
		SystemPrompt: systemPrompt,
		Traits:       []PersonalityTrait{},
		CreatedAt:    timeutil.NowTime(),
		UpdatedAt:    timeutil.NowTime(),
	}
}

// AddTrait adds a trait to the personality
func (p *Personality) AddTrait(key, value string, weight float64) {
	p.Traits = append(p.Traits, PersonalityTrait{
		Key:    key,
		Value:  value,
		Weight: weight,
	})
	p.UpdatedAt = timeutil.NowTime()
}

// Validate validates the personality
func (p *Personality) Validate() error {
	if p.Name == "" {
		return fmt.Errorf("personality name cannot be empty")
	}
	if p.SystemPrompt == "" {
		return fmt.Errorf("system prompt cannot be empty")
	}
	return nil
}
