package skill

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Executor handles skill execution
type Executor struct {
	registry *Registry
	logger   *slog.Logger
}

// NewExecutor creates a new skill executor
func NewExecutor(registry *Registry, logger *slog.Logger) *Executor {
	if logger == nil {
		logger = slog.Default()
	}
	return &Executor{
		registry: registry,
		logger:   logger,
	}
}

// ExecuteOptions contains options for skill execution
type ExecuteOptions struct {
	Timeout     time.Duration
	Context     *ExecutionContext
	ValidateOnly bool
}

// Execute executes a skill by ID
func (e *Executor) Execute(ctx context.Context, skillID string, input map[string]any, opts *ExecuteOptions) (*Result, error) {
	if opts == nil {
		opts = &ExecuteOptions{}
	}

	// Get skill
	skill := e.registry.Get(skillID)
	if skill == nil {
		return nil, fmt.Errorf("skill %s not found", skillID)
	}

	// Check if enabled
	if !e.registry.IsEnabled(skillID) {
		return nil, fmt.Errorf("skill %s is disabled", skillID)
	}

	// Validate input
	if err := skill.Validate(input); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// If validate only, return success
	if opts.ValidateOnly {
		return NewResult(nil), nil
	}

	// Apply timeout if specified
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Execute skill
	startTime := timeutil.NowTime()
	e.logger.Info("executing skill",
		"skill_id", skillID,
		"skill_name", skill.Manifest().Name,
	)

	result, err := skill.Execute(ctx, input)

	duration := timeutil.SinceTime(startTime)
	if err != nil {
		e.logger.Error("skill execution failed",
			"skill_id", skillID,
			"duration", duration,
			"error", err,
		)
		return nil, err
	}

	e.logger.Info("skill execution completed",
		"skill_id", skillID,
		"duration", duration,
		"success", result.Success,
	)

	return result, nil
}

// ValidateInput validates input for a skill
func (e *Executor) ValidateInput(skillID string, input map[string]any) error {
	skill := e.registry.Get(skillID)
	if skill == nil {
		return fmt.Errorf("skill %s not found", skillID)
	}

	return skill.Validate(input)
}

// GetSkillInfo returns information about a skill
func (e *Executor) GetSkillInfo(skillID string) *SkillInfo {
	return e.registry.GetInfo(skillID)
}

// ListSkills returns all available skills
func (e *Executor) ListSkills() []*SkillInfo {
	return e.registry.List()
}

// ListEnabledSkills returns all enabled skills
func (e *Executor) ListEnabledSkills() []*SkillInfo {
	return e.registry.ListEnabled()
}
