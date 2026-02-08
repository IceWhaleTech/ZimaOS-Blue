package controller

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/personality/model"
)

// Service handles personality business logic
type Service struct {
	storage *model.FileStorage
}

// NewService creates a new service
func NewService(storage *model.FileStorage) *Service {
	return &Service{storage: storage}
}

// Create creates a new personality
func (s *Service) Create(name, description, systemPrompt string) (*model.Personality, error) {
	p := model.NewPersonality(name, description, systemPrompt)
	if err := s.storage.Create(p); err != nil {
		return nil, err
	}
	return p, nil
}

// GetByID retrieves a personality
func (s *Service) GetByID(id string) (*model.Personality, error) {
	return s.storage.GetByID(id)
}

// List retrieves all personalities
func (s *Service) List() ([]*model.Personality, error) {
	return s.storage.List()
}

// Update updates a personality
func (s *Service) Update(id, name, description, systemPrompt string) (*model.Personality, error) {
	p, err := s.storage.GetByID(id)
	if err != nil {
		return nil, err
	}
	p.Name = name
	p.Description = description
	p.SystemPrompt = systemPrompt
	if err := s.storage.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Delete deletes a personality
func (s *Service) Delete(id string) error {
	return s.storage.Delete(id)
}

// AddTrait adds a trait to personality
func (s *Service) AddTrait(id, key, value string, weight float64) (*model.Personality, error) {
	if weight < 0 || weight > 1 {
		return nil, fmt.Errorf("weight must be between 0 and 1")
	}
	p, err := s.storage.GetByID(id)
	if err != nil {
		return nil, err
	}
	p.AddTrait(key, value, weight)
	if err := s.storage.Update(p); err != nil {
		return nil, err
	}
	return p, nil
}

// Activate sets a personality as active (stores in a metadata file)
func (s *Service) Activate(id string) error {
	_, err := s.storage.GetByID(id)
	if err != nil {
		return err
	}
	// Store active personality ID in metadata file
	metadataPath := filepath.Join(filepath.Dir(s.storage.GetDataDir()), "active_personality.txt")
	return os.WriteFile(metadataPath, []byte(id), 0644)
}

// GetActive retrieves the active personality
func (s *Service) GetActive() (*model.Personality, error) {
	metadataPath := filepath.Join(filepath.Dir(s.storage.GetDataDir()), "active_personality.txt")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		// Default to "default" personality if no active personality set
		return s.storage.GetByID("default")
	}
	return s.storage.GetByID(string(data))
}
