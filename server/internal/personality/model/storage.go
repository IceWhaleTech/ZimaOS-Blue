package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// FileStorage handles personality data persistence to user data directory
type FileStorage struct {
	dataDir string
}

// GetDataDir returns the data directory path
func (fs *FileStorage) GetDataDir() string {
	return fs.dataDir
}

// NewFileStorage creates a new file storage
func NewFileStorage(dataDir string) (*FileStorage, error) {
	personalitiesDir := filepath.Join(dataDir, "personalities")
	if err := os.MkdirAll(personalitiesDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create personalities directory: %w", err)
	}
	return &FileStorage{dataDir: personalitiesDir}, nil
}

// Create saves a personality to file
func (fs *FileStorage) Create(p *Personality) error {
	if err := p.Validate(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal personality: %w", err)
	}

	filePath := filepath.Join(fs.dataDir, p.ID+".json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write personality file: %w", err)
	}

	return nil
}

// GetByID retrieves a personality by ID
func (fs *FileStorage) GetByID(id string) (*Personality, error) {
	filePath := filepath.Join(fs.dataDir, id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("personality not found: %s", id)
		}
		return nil, fmt.Errorf("failed to read personality file: %w", err)
	}

	var p Personality
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal personality: %w", err)
	}

	return &p, nil
}

// List retrieves all personalities
func (fs *FileStorage) List() ([]*Personality, error) {
	entries, err := os.ReadDir(fs.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read personalities directory: %w", err)
	}

	var personalities []*Personality
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json
		p, err := fs.GetByID(id)
		if err != nil {
			continue
		}
		personalities = append(personalities, p)
	}

	return personalities, nil
}

// Update updates a personality
func (fs *FileStorage) Update(p *Personality) error {
	return fs.Create(p)
}

// Delete deletes a personality
func (fs *FileStorage) Delete(id string) error {
	filePath := filepath.Join(fs.dataDir, id+".json")
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("personality not found: %s", id)
		}
		return fmt.Errorf("failed to delete personality file: %w", err)
	}
	return nil
}
