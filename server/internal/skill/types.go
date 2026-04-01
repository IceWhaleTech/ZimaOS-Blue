// Package skill provides skill management and execution capabilities.
package skill

import (
	"context"
	"encoding/json"
	"fmt"
)

type contextKey string

const userIDKey contextKey = "skill_user_id"

// WithUserID returns a context carrying the user ID for skill execution.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID extracts the user ID from the context.
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(userIDKey).(string); ok {
		return v
	}
	return ""
}

// Manifest represents a skill manifest
type Manifest struct {
	ID              string            `json:"id" yaml:"id"`
	Name            string            `json:"name" yaml:"name"`
	Version         string            `json:"version" yaml:"version"`
	Description     string            `json:"description" yaml:"description"`
	Author          string            `json:"author,omitempty" yaml:"author,omitempty"`
	Category        string            `json:"category,omitempty" yaml:"category,omitempty"`
	Icon            string            `json:"icon,omitempty" yaml:"icon,omitempty"`
	Tags            []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Paths           []string          `json:"paths,omitempty" yaml:"paths,omitempty"`
	UserInvocable   bool              `json:"user_invocable" yaml:"user_invocable"`
	ModelInvocable  bool              `json:"model_invocable" yaml:"model_invocable"`
	Invocation      string            `json:"invocation,omitempty" yaml:"invocation,omitempty"`
	Examples        []string          `json:"examples,omitempty" yaml:"examples,omitempty"`
	CapabilityTags  []string          `json:"capability_tags,omitempty" yaml:"capability_tags,omitempty"`
	InteractionMode string            `json:"interaction_mode,omitempty" yaml:"interaction_mode,omitempty"`
	CardSupport     string            `json:"card_support,omitempty" yaml:"card_support,omitempty"`
	Inputs          []Parameter       `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs         []Parameter       `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	Permissions     []string          `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Config          map[string]any    `json:"config,omitempty" yaml:"config,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// Parameter represents an input or output parameter
type Parameter struct {
	Name        string `json:"name" yaml:"name"`
	Type        string `json:"type" yaml:"type"` // string, number, boolean, object, array
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Default     any    `json:"default,omitempty" yaml:"default,omitempty"`
}

// Skill is the interface that all skills must implement
type Skill interface {
	// Manifest returns the skill manifest
	Manifest() *Manifest

	// Execute executes the skill with the given input
	Execute(ctx context.Context, input map[string]any) (*Result, error)

	// Validate validates the input parameters
	Validate(input map[string]any) error
}

// Result represents the result of a skill execution
type Result struct {
	Success bool           `json:"success"`
	Data    any            `json:"data,omitempty"`
	Error   string         `json:"error,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// NewResult creates a successful result
func NewResult(data any) *Result {
	return &Result{
		Success: true,
		Data:    data,
	}
}

// NewErrorResult creates an error result
func NewErrorResult(err error) *Result {
	return &Result{
		Success: false,
		Error:   err.Error(),
	}
}

// ToJSON converts the result to JSON
func (r *Result) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ExecutionContext provides context for skill execution
type ExecutionContext struct {
	UserID      string            `json:"user_id,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
	Permissions []string          `json:"permissions,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SkillInfo contains information about a registered skill
type SkillInfo struct {
	Manifest *Manifest `json:"manifest"`
	Enabled  bool      `json:"enabled"`
	Builtin  bool      `json:"builtin"`
}

// ManifestSkill is a declarative skill created from a manifest JSON.
// It holds metadata but has no executable logic.
type ManifestSkill struct {
	manifest *Manifest
}

func NormalizeManifestDefaults(m *Manifest) {
	if m == nil {
		return
	}
	if !m.UserInvocable && !m.ModelInvocable && len(m.Paths) == 0 {
		m.UserInvocable = true
		m.ModelInvocable = true
	}
}

// NewManifestSkill creates a skill from a parsed manifest.
func NewManifestSkill(m *Manifest) *ManifestSkill {
	NormalizeManifestDefaults(m)
	return &ManifestSkill{manifest: m}
}

func (s *ManifestSkill) Manifest() *Manifest { return s.manifest }

func (s *ManifestSkill) Execute(_ context.Context, input map[string]any) (*Result, error) {
	// Declarative skills have no Go backend — the LLM handles them directly.
	// Return the input as-is so the tool adapter can surface it.
	return NewResult(map[string]any{
		"skill":   s.manifest.ID,
		"input":   input,
		"message": fmt.Sprintf("Skill %q is declarative — handled by LLM", s.manifest.ID),
	}), nil
}

func (s *ManifestSkill) Validate(_ map[string]any) error { return nil }
