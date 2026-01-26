package builtin

import (
	"context"
	"fmt"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
)

// DateTime is a built-in date/time skill
type DateTime struct {
	manifest *skill.Manifest
}

// NewDateTime creates a new date/time skill
func NewDateTime() *DateTime {
	return &DateTime{
		manifest: &skill.Manifest{
			ID:          "datetime",
			Name:        "Date & Time",
			Version:     "1.0.0",
			Description: "Returns current date and time information",
			Category:    "utility",
			Tags:        []string{"date", "time", "utility"},
			Inputs: []skill.Parameter{
				{
					Name:        "timezone",
					Type:        "string",
					Description: "Timezone (e.g., 'America/New_York', 'UTC')",
					Required:    false,
					Default:     "Local",
				},
				{
					Name:        "format",
					Type:        "string",
					Description: "Output format: 'iso', 'unix', 'rfc3339', 'custom'",
					Required:    false,
					Default:     "iso",
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "datetime",
					Type:        "object",
					Description: "Date and time information",
				},
			},
		},
	}
}

// Manifest returns the skill manifest
func (d *DateTime) Manifest() *skill.Manifest {
	return d.manifest
}

// Validate validates the input parameters
func (d *DateTime) Validate(input map[string]any) error {
	if format, ok := input["format"]; ok {
		formatStr, ok := format.(string)
		if !ok {
			return fmt.Errorf("format must be a string")
		}
		validFormats := map[string]bool{"iso": true, "unix": true, "rfc3339": true, "custom": true}
		if !validFormats[formatStr] {
			return fmt.Errorf("invalid format: %s", formatStr)
		}
	}
	return nil
}

// Execute executes the date/time skill
func (d *DateTime) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	now := time.Now()

	// Handle timezone
	if tz, ok := input["timezone"].(string); ok && tz != "" && tz != "Local" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return skill.NewErrorResult(fmt.Errorf("invalid timezone: %s", tz)), nil
		}
		now = now.In(loc)
	}

	format := "iso"
	if f, ok := input["format"].(string); ok {
		format = f
	}

	result := map[string]any{
		"year":       now.Year(),
		"month":      int(now.Month()),
		"day":        now.Day(),
		"hour":       now.Hour(),
		"minute":     now.Minute(),
		"second":     now.Second(),
		"weekday":    now.Weekday().String(),
		"timezone":   now.Location().String(),
		"unix":       now.Unix(),
		"unix_milli": now.UnixMilli(),
	}

	switch format {
	case "iso":
		result["formatted"] = now.Format("2006-01-02T15:04:05")
	case "unix":
		result["formatted"] = now.Unix()
	case "rfc3339":
		result["formatted"] = now.Format(time.RFC3339)
	default:
		result["formatted"] = now.Format(time.RFC3339)
	}

	return skill.NewResult(result), nil
}
