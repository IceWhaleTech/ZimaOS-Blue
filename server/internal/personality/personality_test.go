package personality

import (
	"testing"
)

func TestPersonalityCreation(t *testing.T) {
	// Arrange
	name := "Helpful Assistant"
	description := "A helpful and friendly assistant"
	systemPrompt := "You are a helpful assistant"

	// Act
	p := NewPersonality(name, description, systemPrompt)

	// Assert
	if p.Name != name {
		t.Errorf("Expected name %s, got %s", name, p.Name)
	}
	if p.Description != description {
		t.Errorf("Expected description %s, got %s", description, p.Description)
	}
	if p.SystemPrompt != systemPrompt {
		t.Errorf("Expected systemPrompt %s, got %s", systemPrompt, p.SystemPrompt)
	}
	if p.ID == "" {
		t.Error("Expected ID to be generated")
	}
	if p.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestPersonalityAddTrait(t *testing.T) {
	// Arrange
	p := NewPersonality("Test", "Test personality", "Test prompt")

	// Act
	p.AddTrait("tone", "friendly", 0.8)

	// Assert
	if len(p.Traits) != 1 {
		t.Errorf("Expected 1 trait, got %d", len(p.Traits))
	}
	if p.Traits[0].Key != "tone" {
		t.Errorf("Expected trait key 'tone', got %s", p.Traits[0].Key)
	}
	if p.Traits[0].Value != "friendly" {
		t.Errorf("Expected trait value 'friendly', got %s", p.Traits[0].Value)
	}
	if p.Traits[0].Weight != 0.8 {
		t.Errorf("Expected trait weight 0.8, got %f", p.Traits[0].Weight)
	}
}

func TestPersonalityValidation(t *testing.T) {
	tests := []struct {
		name        string
		personality *Personality
		shouldFail  bool
	}{
		{
			name:        "Valid personality",
			personality: NewPersonality("Valid", "Valid description", "Valid prompt"),
			shouldFail:  false,
		},
		{
			name:        "Empty name",
			personality: &Personality{Name: "", Description: "Desc", SystemPrompt: "Prompt"},
			shouldFail:  true,
		},
		{
			name:        "Empty system prompt",
			personality: &Personality{Name: "Name", Description: "Desc", SystemPrompt: ""},
			shouldFail:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.personality.Validate()
			if (err != nil) != tt.shouldFail {
				t.Errorf("Expected shouldFail=%v, got error=%v", tt.shouldFail, err)
			}
		})
	}
}
