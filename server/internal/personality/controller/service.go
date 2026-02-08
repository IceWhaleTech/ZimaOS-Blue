package controller

import (
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/personality/model"
)

// Service handles personality business logic
type Service struct {
	repo *model.Repository
}

// NewService creates a new service
func NewService(repo *model.Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a new personality
func (s *Service) Create(name, description, systemPrompt string) (*model.Personality, error) {
	p := model.NewPersonality(name, description, systemPrompt)
	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetByID retrieves a personality
func (s *Service) GetByID(id string) (*model.Personality, error) {
	return s.repo.GetByID(id)
}

// List retrieves all personalities
func (s *Service) List() ([]*model.Personality, error) {
	return s.repo.List()
}

// Update updates a personality
func (s *Service) Update(id, name, description, systemPrompt string) (*model.Personality, error) {
	p, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	p.Name = name
	p.Description = description
	p.SystemPrompt = systemPrompt
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete deletes a personality
func (s *Service) Delete(id string) error {
	return s.repo.Delete(id)
}

// AddTrait adds a trait to personality
func (s *Service) AddTrait(id, key, value string, weight float64) (*model.Personality, error) {
	if weight < 0 || weight > 1 {
		return nil, fmt.Errorf("weight must be between 0 and 1")
	}
	p, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	p.AddTrait(key, value, weight)
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Activate sets a personality as active
func (s *Service) Activate(id string) error {
	_, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	return s.repo.Activate(id)
}

// GetActive retrieves the active personality
func (s *Service) GetActive() (*model.Personality, error) {
	return s.repo.GetActive()
}
