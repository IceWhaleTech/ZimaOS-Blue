package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/skill"
)

// Translate is a built-in translation skill
type Translate struct {
	manifest *skill.Manifest
}

// NewTranslate creates a new translate skill
func NewTranslate() *Translate {
	return &Translate{
		manifest: &skill.Manifest{
			ID:          "translate",
			Name:        "Translate",
			Version:     "1.0.0",
			Description: "Text translation between languages",
			Category:    "information",
			Icon:        "translate",
			Tags:        []string{"translate", "language", "i18n", "localization"},
			Inputs: []skill.Parameter{
				{
					Name:        "text",
					Type:        "string",
					Description: "Text to translate",
					Required:    true,
				},
				{
					Name:        "from",
					Type:        "string",
					Description: "Source language code (e.g., 'en', 'zh', 'es')",
					Required:    false,
					Default:     "auto",
				},
				{
					Name:        "to",
					Type:        "string",
					Description: "Target language code",
					Required:    true,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "translated",
					Type:        "string",
					Description: "Translated text",
				},
				{
					Name:        "detected_language",
					Type:        "string",
					Description: "Detected source language",
				},
			},
		},
	}
}

// Manifest returns the skill manifest
func (t *Translate) Manifest() *skill.Manifest {
	return t.manifest
}

// Validate validates the input parameters
func (t *Translate) Validate(input map[string]any) error {
	if _, ok := input["text"]; !ok {
		return fmt.Errorf("text is required")
	}
	if _, ok := input["to"]; !ok {
		return fmt.Errorf("target language (to) is required")
	}
	return nil
}

// Execute executes the translate skill
// Note: This is a placeholder implementation. In production, integrate with
// a translation API like Google Translate, DeepL, or LibreTranslate.
func (t *Translate) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	text := input["text"].(string)
	to := input["to"].(string)
	from := "auto"
	if f, ok := input["from"].(string); ok {
		from = f
	}

	// Placeholder: In production, call actual translation API
	// For now, return a message indicating the translation request
	return skill.NewResult(map[string]any{
		"text":              text,
		"from":              from,
		"to":                to,
		"translated":        fmt.Sprintf("[Translation to %s]: %s", to, text),
		"detected_language": from,
		"message":           "Translation service placeholder. Configure translation API for actual translations.",
		"supported_languages": []string{
			"en", "zh", "es", "fr", "de", "ja", "ko", "ru", "pt", "it",
			"ar", "hi", "th", "vi", "nl", "pl", "tr", "sv", "da", "fi",
		},
	}), nil
}

// Notifications is a built-in notifications skill
type Notifications struct {
	manifest      *skill.Manifest
	notifications []map[string]any
}

// NewNotifications creates a new notifications skill
func NewNotifications() *Notifications {
	return &Notifications{
		manifest: &skill.Manifest{
			ID:          "notifications",
			Name:        "Notifications",
			Version:     "1.0.0",
			Description: "System notifications management",
			Category:    "communication",
			Icon:        "notifications",
			Tags:        []string{"notifications", "alerts", "system"},
			Inputs: []skill.Parameter{
				{
					Name:        "action",
					Type:        "string",
					Description: "Action: send, list, clear",
					Required:    true,
				},
				{
					Name:        "title",
					Type:        "string",
					Description: "Notification title",
					Required:    false,
				},
				{
					Name:        "message",
					Type:        "string",
					Description: "Notification message",
					Required:    false,
				},
				{
					Name:        "priority",
					Type:        "string",
					Description: "Priority: low, normal, high, urgent",
					Required:    false,
					Default:     "normal",
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "notification",
					Type:        "object",
					Description: "Notification object",
				},
				{
					Name:        "notifications",
					Type:        "array",
					Description: "List of notifications",
				},
			},
		},
		notifications: make([]map[string]any, 0),
	}
}

// Manifest returns the skill manifest
func (n *Notifications) Manifest() *skill.Manifest {
	return n.manifest
}

// Validate validates the input parameters
func (n *Notifications) Validate(input map[string]any) error {
	action, ok := input["action"]
	if !ok {
		return fmt.Errorf("action is required")
	}

	actionStr, ok := action.(string)
	if !ok {
		return fmt.Errorf("action must be a string")
	}

	if actionStr == "send" {
		if _, ok := input["message"]; !ok {
			return fmt.Errorf("message is required for send action")
		}
	}

	return nil
}

// Execute executes the notifications skill
func (n *Notifications) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	action := input["action"].(string)

	switch action {
	case "send":
		notification := map[string]any{
			"title":    input["title"],
			"message":  input["message"],
			"priority": input["priority"],
		}
		if notification["priority"] == nil {
			notification["priority"] = "normal"
		}
		n.notifications = append(n.notifications, notification)
		return skill.NewResult(map[string]any{
			"sent":         true,
			"notification": notification,
			"message":      "Notification sent",
		}), nil

	case "list":
		return skill.NewResult(map[string]any{
			"notifications": n.notifications,
			"count":         len(n.notifications),
		}), nil

	case "clear":
		count := len(n.notifications)
		n.notifications = make([]map[string]any, 0)
		return skill.NewResult(map[string]any{
			"cleared": count,
			"message": fmt.Sprintf("Cleared %d notifications", count),
		}), nil
	}

	return skill.NewErrorResult(fmt.Errorf("unknown action: %s", action)), nil
}

// UnitConverter is a built-in unit conversion skill
type UnitConverter struct {
	manifest *skill.Manifest
}

// NewUnitConverter creates a new unit converter skill
func NewUnitConverter() *UnitConverter {
	return &UnitConverter{
		manifest: &skill.Manifest{
			ID:          "unit-converter",
			Name:        "Unit Converter",
			Version:     "1.0.0",
			Description: "Convert between different units of measurement",
			Category:    "utility",
			Icon:        "unit-converter",
			Tags:        []string{"convert", "units", "measurement", "utility"},
			Inputs: []skill.Parameter{
				{
					Name:        "value",
					Type:        "number",
					Description: "Value to convert",
					Required:    true,
				},
				{
					Name:        "from",
					Type:        "string",
					Description: "Source unit (e.g., 'km', 'mi', 'kg', 'lb', 'c', 'f')",
					Required:    true,
				},
				{
					Name:        "to",
					Type:        "string",
					Description: "Target unit",
					Required:    true,
				},
			},
			Outputs: []skill.Parameter{
				{
					Name:        "result",
					Type:        "number",
					Description: "Converted value",
				},
				{
					Name:        "formula",
					Type:        "string",
					Description: "Conversion formula used",
				},
			},
		},
	}
}

// Manifest returns the skill manifest
func (u *UnitConverter) Manifest() *skill.Manifest {
	return u.manifest
}

// Validate validates the input parameters
func (u *UnitConverter) Validate(input map[string]any) error {
	if _, ok := input["value"]; !ok {
		return fmt.Errorf("value is required")
	}
	if _, ok := input["from"]; !ok {
		return fmt.Errorf("from unit is required")
	}
	if _, ok := input["to"]; !ok {
		return fmt.Errorf("to unit is required")
	}
	return nil
}

// Execute executes the unit converter skill
func (u *UnitConverter) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	var value float64
	switch v := input["value"].(type) {
	case float64:
		value = v
	case int:
		value = float64(v)
	case int64:
		value = float64(v)
	default:
		return skill.NewErrorResult(fmt.Errorf("value must be a number")), nil
	}

	from := strings.ToLower(input["from"].(string))
	to := strings.ToLower(input["to"].(string))

	result, formula, err := u.convert(value, from, to)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	return skill.NewResult(map[string]any{
		"value":   value,
		"from":    from,
		"to":      to,
		"result":  result,
		"formula": formula,
	}), nil
}

func (u *UnitConverter) convert(value float64, from, to string) (float64, string, error) {
	// Length conversions
	lengthToMeters := map[string]float64{
		"m": 1, "km": 1000, "cm": 0.01, "mm": 0.001,
		"mi": 1609.344, "yd": 0.9144, "ft": 0.3048, "in": 0.0254,
	}

	// Weight conversions
	weightToKg := map[string]float64{
		"kg": 1, "g": 0.001, "mg": 0.000001,
		"lb": 0.453592, "oz": 0.0283495,
	}

	// Temperature conversions (special case)
	if (from == "c" || from == "f" || from == "k") && (to == "c" || to == "f" || to == "k") {
		return u.convertTemperature(value, from, to)
	}

	// Try length conversion
	if fromFactor, ok := lengthToMeters[from]; ok {
		if toFactor, ok := lengthToMeters[to]; ok {
			result := value * fromFactor / toFactor
			return result, fmt.Sprintf("%s -> meters -> %s", from, to), nil
		}
	}

	// Try weight conversion
	if fromFactor, ok := weightToKg[from]; ok {
		if toFactor, ok := weightToKg[to]; ok {
			result := value * fromFactor / toFactor
			return result, fmt.Sprintf("%s -> kg -> %s", from, to), nil
		}
	}

	return 0, "", fmt.Errorf("unsupported conversion from %s to %s", from, to)
}

func (u *UnitConverter) convertTemperature(value float64, from, to string) (float64, string, error) {
	// Convert to Celsius first
	var celsius float64
	switch from {
	case "c":
		celsius = value
	case "f":
		celsius = (value - 32) * 5 / 9
	case "k":
		celsius = value - 273.15
	}

	// Convert from Celsius to target
	var result float64
	switch to {
	case "c":
		result = celsius
	case "f":
		result = celsius*9/5 + 32
	case "k":
		result = celsius + 273.15
	}

	return result, fmt.Sprintf("%s -> Celsius -> %s", from, to), nil
}
