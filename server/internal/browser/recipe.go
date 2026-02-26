package browser

import (
	"context"
	"fmt"
	"sync"
)

// Recipe defines a reusable browser automation template.
// Recipes encapsulate multi-step browser operations (search, form fill, etc.)
// into a single call, eliminating the need for LLM to orchestrate each step.
type Recipe interface {
	// Name returns the recipe identifier (e.g. "search", "fill_form").
	Name() string
	// Description returns a human-readable description.
	Description() string
	// Validate checks that params are valid before execution.
	Validate(params map[string]string) error
	// Execute runs the recipe and returns structured results.
	Execute(ctx context.Context, svc *RodService, params map[string]string) (*RecipeResult, error)
	// KeepTab returns true if the tab should stay open after execution.
	KeepTab() bool
}

// RecipeResult is the structured output from a recipe execution.
type RecipeResult struct {
	Success  bool                   `json:"success"`
	Data     map[string]interface{} `json:"data"`
	TargetID string                 `json:"target_id,omitempty"`
	Message  string                 `json:"message"`
}

// RecipeRegistry holds all registered recipes.
type RecipeRegistry struct {
	mu      sync.RWMutex
	recipes map[string]Recipe
}

// NewRecipeRegistry creates a registry with all built-in recipes.
func NewRecipeRegistry() *RecipeRegistry {
	r := &RecipeRegistry{
		recipes: make(map[string]Recipe),
	}
	// Register built-in recipes
	r.Register(&searchRecipe{})
	r.Register(&fillFormRecipe{})
	r.Register(&extractRecipe{})
	r.Register(&loginRecipe{})
	return r
}

// Register adds a recipe to the registry.
func (r *RecipeRegistry) Register(recipe Recipe) {
	r.mu.Lock()
	r.recipes[recipe.Name()] = recipe
	r.mu.Unlock()
}

// Get returns a recipe by name.
func (r *RecipeRegistry) Get(name string) (Recipe, bool) {
	r.mu.RLock()
	recipe, ok := r.recipes[name]
	r.mu.RUnlock()
	return recipe, ok
}

// List returns all registered recipe names and descriptions.
func (r *RecipeRegistry) List() []RecipeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	infos := make([]RecipeInfo, 0, len(r.recipes))
	for _, recipe := range r.recipes {
		infos = append(infos, RecipeInfo{
			Name:        recipe.Name(),
			Description: recipe.Description(),
			KeepTab:     recipe.KeepTab(),
		})
	}
	return infos
}

// Execute runs a recipe by name with the given params.
func (r *RecipeRegistry) Execute(ctx context.Context, svc *RodService, name string, params map[string]string) (*RecipeResult, error) {
	recipe, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("unknown recipe: %s (available: search, fill_form, extract, login)", name)
	}
	if err := recipe.Validate(params); err != nil {
		return nil, fmt.Errorf("recipe %s: %w", name, err)
	}
	return recipe.Execute(ctx, svc, params)
}

// RecipeInfo describes a recipe for listing.
type RecipeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	KeepTab     bool   `json:"keep_tab"`
}
