package model

import (
	"os"
	"path/filepath"
	"time"
)

// InitializeDefaultPersonality creates the default personality from SOUL.md
func InitializeDefaultPersonality(dataDir string) error {
	storage, err := NewFileStorage(dataDir)
	if err != nil {
		return err
	}

	// Check if default personality already exists
	if _, err := storage.GetByID("default"); err == nil {
		return nil // Already exists
	}

	// Read SOUL.md
	soulPath := filepath.Join(dataDir, "..", "..", "SOUL.md")
	soulContent, err := os.ReadFile(soulPath)
	if err != nil {
		// Fallback if SOUL.md not found
		soulContent = []byte(defaultSoulContent)
	}

	// Create default personality
	now := time.Now()
	defaultPersonality := &Personality{
		ID:           "default",
		Name:         "Blue",
		Description:  "Default ZimaOS Blue AI Assistant",
		SystemPrompt: string(soulContent),
		CreatedAt:    now,
		UpdatedAt:    now,
		Traits: []PersonalityTrait{
			{Key: "tone", Value: "friendly but professional", Weight: 1.0},
			{Key: "style", Value: "clear and concise", Weight: 1.0},
			{Key: "focus", Value: "helpful and practical", Weight: 1.0},
		},
	}

	return storage.Create(defaultPersonality)
}

const defaultSoulContent = `# Blue - ZimaOS AI Assistant

## Identity
You are Blue, the AI assistant for ZimaOS - a personal cloud operating system.

## Core Values
- Clarity over complexity
- Efficiency and respect for user's time
- Proactive helpfulness
- Transparency about limitations

## Communication Style
- Friendly but professional
- Confident but humble
- Patient and encouraging
- Use plain language for general users, technical terms for developers

## Response Structure
1. Direct answer first
2. Context if needed
3. Next steps when appropriate
4. Keep it concise
`
