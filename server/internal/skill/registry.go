package skill

import (
	"fmt"
	"sync"
)

// Registry manages skill registration and lookup
type Registry struct {
	skills  map[string]Skill
	enabled map[string]bool
	builtin map[string]bool
	mu      sync.RWMutex
}

// NewRegistry creates a new skill registry
func NewRegistry() *Registry {
	return &Registry{
		skills:  make(map[string]Skill),
		enabled: make(map[string]bool),
		builtin: make(map[string]bool),
	}
}

// Register registers a skill
func (r *Registry) Register(skill Skill, builtin bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	manifest := skill.Manifest()
	if manifest == nil {
		return fmt.Errorf("skill manifest is nil")
	}

	if manifest.ID == "" {
		return fmt.Errorf("skill ID is required")
	}

	if _, exists := r.skills[manifest.ID]; exists {
		return fmt.Errorf("skill %s already registered", manifest.ID)
	}

	r.skills[manifest.ID] = skill
	r.enabled[manifest.ID] = true
	r.builtin[manifest.ID] = builtin

	return nil
}

// Unregister removes a skill from the registry
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[id]; !exists {
		return fmt.Errorf("skill %s not found", id)
	}

	delete(r.skills, id)
	delete(r.enabled, id)
	delete(r.builtin, id)

	return nil
}

// Get returns a skill by ID
func (r *Registry) Get(id string) Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.skills[id]
}

// GetInfo returns skill info by ID
func (r *Registry) GetInfo(id string) *SkillInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, exists := r.skills[id]
	if !exists {
		return nil
	}

	return &SkillInfo{
		Manifest: skill.Manifest(),
		Enabled:  r.enabled[id],
		Builtin:  r.builtin[id],
	}
}

// List returns all registered skills
func (r *Registry) List() []*SkillInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SkillInfo, 0, len(r.skills))
	for id, skill := range r.skills {
		result = append(result, &SkillInfo{
			Manifest: skill.Manifest(),
			Enabled:  r.enabled[id],
			Builtin:  r.builtin[id],
		})
	}

	return result
}

// ListEnabled returns all enabled skills
func (r *Registry) ListEnabled() []*SkillInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SkillInfo, 0)
	for id, skill := range r.skills {
		if r.enabled[id] {
			result = append(result, &SkillInfo{
				Manifest: skill.Manifest(),
				Enabled:  true,
				Builtin:  r.builtin[id],
			})
		}
	}

	return result
}

// Enable enables a skill
func (r *Registry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[id]; !exists {
		return fmt.Errorf("skill %s not found", id)
	}

	r.enabled[id] = true
	return nil
}

// Disable disables a skill
func (r *Registry) Disable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.skills[id]; !exists {
		return fmt.Errorf("skill %s not found", id)
	}

	r.enabled[id] = false
	return nil
}

// IsEnabled checks if a skill is enabled
func (r *Registry) IsEnabled(id string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.enabled[id]
}

// Count returns the number of registered skills
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.skills)
}
