package tools

import (
	"context"
	"encoding/json"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// SkillToolAdapter wraps a skill.Skill as a tools.Tool so the LLM can call it.
type SkillToolAdapter struct {
	skill skill.Skill
}

// NewSkillToolAdapter creates a Tool that delegates to the given Skill.
func NewSkillToolAdapter(s skill.Skill) *SkillToolAdapter {
	return &SkillToolAdapter{skill: s}
}

// Definition converts the skill manifest into a ToolDefinition.
func (a *SkillToolAdapter) Definition() ToolDefinition {
	m := a.skill.Manifest()

	// Build JSON Schema properties from manifest inputs.
	properties := make(map[string]interface{}, len(m.Inputs))
	var required []string
	for _, p := range m.Inputs {
		prop := map[string]interface{}{
			"type":        paramTypeToJSONSchema(p.Type),
			"description": p.Description,
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		properties[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}

	params := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		params["required"] = required
	}

	return ToolDefinition{
		Name:        m.ID,
		Description: m.Description,
		Icon:        m.Icon,
		Parameters:  params,
	}
}

// Execute delegates to the underlying skill and returns JSON.
func (a *SkillToolAdapter) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// Convert map[string]interface{} to map[string]any (same type, just pass through).
	result, err := a.skill.Execute(ctx, args)
	if err != nil {
		errResp := map[string]interface{}{"error": err.Error()}
		b, _ := json.Marshal(errResp)
		return string(b), nil // Return error as JSON so the LLM sees it
	}

	b, err := result.ToJSON()
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// paramTypeToJSONSchema maps skill parameter types to JSON Schema types.
func paramTypeToJSONSchema(t string) string {
	switch t {
	case "string":
		return "string"
	case "number":
		return "number"
	case "boolean":
		return "boolean"
	case "object":
		return "object"
	case "array":
		return "array"
	default:
		return "string"
	}
}

// RegisterSkill registers a skill as a tool in the registry.
func RegisterSkill(registry *Registry, s skill.Skill) {
	registry.Register(NewSkillToolAdapter(s))
}
