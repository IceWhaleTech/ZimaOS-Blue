package skill

import (
	"fmt"
	"strings"
)

// HelpData renders a concise, generic help payload for a skill manifest.
func HelpData(manifest *Manifest, skillID string) map[string]string {
	if manifest == nil {
		manifest = &Manifest{}
	}

	canonicalID := firstNonBlank(
		strings.TrimSpace(skillID),
		strings.TrimSpace(manifest.ID),
		strings.TrimSpace(manifest.Name),
		"skill",
	)

	usage := buildHelpUsage(manifest, canonicalID)
	data := map[string]string{
		"skill": canonicalID,
		"usage": usage,
	}
	if desc := strings.TrimSpace(manifest.Description); desc != "" {
		data["description"] = desc
	}
	if required := formatHelpParameters(manifest.Inputs, true); required != "" {
		data["required_inputs"] = required
	}
	if optional := formatHelpParameters(manifest.Inputs, false); optional != "" {
		data["optional_inputs"] = optional
	}
	if outputs := formatHelpParameters(manifest.Outputs, false); outputs != "" {
		data["outputs"] = outputs
	}
	if examples := buildHelpExamples(manifest, usage); examples != "" {
		data["examples"] = examples
	}
	return data
}

func buildHelpUsage(manifest *Manifest, skillID string) string {
	if manifest != nil {
		if invocation := strings.TrimSpace(manifest.Invocation); invocation != "" {
			return invocation
		}
	}

	parts := []string{"blue", strings.TrimSpace(skillID)}
	if manifest != nil {
		for _, param := range manifest.Inputs {
			part := fmt.Sprintf("%s=%s", strings.TrimSpace(param.Name), helpPlaceholder(param))
			if param.Required {
				parts = append(parts, part)
				continue
			}
			parts = append(parts, "["+part+"]")
		}
	}
	return strings.Join(parts, " ")
}

func buildHelpExamples(manifest *Manifest, usage string) string {
	if manifest != nil && len(manifest.Examples) > 0 {
		examples := make([]string, 0, len(manifest.Examples))
		for _, example := range manifest.Examples {
			example = strings.TrimSpace(example)
			if example == "" {
				continue
			}
			examples = append(examples, example)
			if len(examples) >= 3 {
				break
			}
		}
		if len(examples) > 0 {
			return strings.Join(examples, "\n")
		}
	}
	return strings.TrimSpace(usage)
}

func formatHelpParameters(params []Parameter, required bool) string {
	lines := make([]string, 0, len(params))
	for _, param := range params {
		if param.Required != required {
			continue
		}
		name := strings.TrimSpace(param.Name)
		if name == "" {
			continue
		}
		line := name
		if typ := strings.TrimSpace(param.Type); typ != "" {
			line += " (" + typ + ")"
		}
		if desc := strings.TrimSpace(param.Description); desc != "" {
			line += ": " + desc
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func helpPlaceholder(param Parameter) string {
	lower := strings.ToLower(strings.TrimSpace(param.Name + " " + param.Description))
	switch {
	case strings.Contains(lower, "path"), strings.Contains(lower, "file"):
		return "<path>"
	}

	switch strings.ToLower(strings.TrimSpace(param.Type)) {
	case "number", "integer":
		return "<number>"
	case "boolean", "bool":
		return "<true|false>"
	case "array", "object":
		return "<json>"
	default:
		return "<value>"
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
