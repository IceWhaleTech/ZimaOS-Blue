package builtin

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// Calculator is a built-in calculator skill
type Calculator struct {
	manifest *skill.Manifest
}

// NewCalculator creates a new calculator skill
func NewCalculator() *Calculator {
	return &Calculator{
		manifest: &skill.Manifest{
			ID:          "calculator",
			Name:        "Calculator",
			Version:     "1.0.0",
			Description: "Performs basic arithmetic calculations",
			Category:    "utility",
			Icon:        "calculator",
			Tags:        []string{"math", "calculator", "utility"},
			Inputs: []skill.Parameter{
				{
					Name:        "expression",
					Type:        "string",
					Description: "Mathematical expression to evaluate (e.g., '2 + 3 * 4')",
					Required:    true,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "result",
					Type:        "number",
					Description: "The calculated result",
				},
			},
		},
	}
}

// Manifest returns the skill manifest
func (c *Calculator) Manifest() *skill.Manifest {
	return c.manifest
}

// Validate validates the input parameters
func (c *Calculator) Validate(input map[string]any) error {
	expr, ok := input["expression"]
	if !ok {
		return fmt.Errorf("expression is required")
	}

	if _, ok := expr.(string); !ok {
		return fmt.Errorf("expression must be a string")
	}

	return nil
}

// Execute executes the calculator skill
func (c *Calculator) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	expr := input["expression"].(string)

	result, err := c.evaluate(expr)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"result":     result,
		"expression": expr,
	}), nil
}

// evaluate evaluates a simple arithmetic expression
func (c *Calculator) evaluate(expr string) (float64, error) {
	// Simple expression parser for basic operations
	// Supports: +, -, *, /
	expr = strings.TrimSpace(expr)
	expr = strings.ReplaceAll(expr, " ", "")

	// Try to parse as a simple number first
	if num, err := strconv.ParseFloat(expr, 64); err == nil {
		return num, nil
	}

	// Find the last + or - (lowest precedence)
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '+' || expr[i] == '-' {
			if i == 0 {
				continue // Unary operator
			}
			left, err := c.evaluate(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := c.evaluate(expr[i+1:])
			if err != nil {
				return 0, err
			}
			if expr[i] == '+' {
				return left + right, nil
			}
			return left - right, nil
		}
	}

	// Find the last * or / (higher precedence)
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '*' || expr[i] == '/' {
			left, err := c.evaluate(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := c.evaluate(expr[i+1:])
			if err != nil {
				return 0, err
			}
			if expr[i] == '*' {
				return left * right, nil
			}
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		}
	}

	return 0, fmt.Errorf("invalid expression: %s", expr)
}
