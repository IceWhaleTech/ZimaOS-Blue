// Package skill provides skill management and execution capabilities.
package skill

import (
	"context"
	"encoding/json"
)

// Manifest represents a skill manifest
type Manifest struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Version     string            `json:"version" yaml:"version"`
	Description string            `json:"description" yaml:"description"`
	Author      string            `json:"author,omitempty" yaml:"author,omitempty"`
	Category    string            `json:"category,omitempty" yaml:"category,omitempty"`
	Icon        string            `json:"icon,omitempty" yaml:"icon,omitempty"`
	Tags        []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Inputs      []Parameter       `json:"inputs,omitempty" yaml:"inputs,omitempty"`
	Outputs     []Parameter       `json:"outputs,omitempty" yaml:"outputs,omitempty"`
	Permissions []string          `json:"permissions,omitempty" yaml:"permissions,omitempty"`
	Config      map[string]any    `json:"config,omitempty" yaml:"config,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
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
