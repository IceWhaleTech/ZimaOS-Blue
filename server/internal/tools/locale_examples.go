package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
)

var (
	localeEgPattern             = regexp.MustCompile(`(?i)e\.g\.\s*,?\s*[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8})?(?:\s*,\s*[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8})?)*`)
	localeExamplesPattern       = regexp.MustCompile(`(?i)examples?\s*:\s*[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8})?(?:\s*,\s*[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8})?)*`)
	localeDefaultContextPattern = regexp.MustCompile(`(?i)default\s*:\s*from context or [A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8})?`)
	localeDefaultPattern        = regexp.MustCompile(`(?i)default\s*:\s*[A-Za-z]{2,3}(?:[-_][A-Za-z0-9]{2,8})?`)
)

func localizeToolDefinitions(defs []ToolDefinition, locale string) []ToolDefinition {
	if len(defs) == 0 {
		return nil
	}

	exampleLocale := localizedToolLocale(locale)
	out := make([]ToolDefinition, len(defs))
	for i, def := range defs {
		out[i] = ToolDefinition{
			Name:        def.Name,
			Description: def.Description,
			Icon:        def.Icon,
			Parameters:  localizeSchemaMap(def.Parameters, "", exampleLocale),
		}
	}
	return out
}

func localizedToolLocale(locale string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(locale, "_", "-"))
	if trimmed == "" {
		return string(i18n.DefaultLanguage)
	}
	parsed := string(i18n.ParseLanguage(trimmed))
	if parsed != string(i18n.DefaultLanguage) || strings.HasPrefix(strings.ToLower(trimmed), "en") {
		return parsed
	}
	return trimmed
}

func localizeSchemaMap(in map[string]interface{}, propName, locale string) map[string]interface{} {
	if len(in) == 0 {
		return in
	}

	out := make(map[string]interface{}, len(in))
	for key, raw := range in {
		switch key {
		case "properties":
			props, ok := raw.(map[string]interface{})
			if !ok {
				out[key] = cloneSchemaValue(raw)
				continue
			}
			localizedProps := make(map[string]interface{}, len(props))
			for childName, childRaw := range props {
				if childMap, ok := childRaw.(map[string]interface{}); ok {
					localizedProps[childName] = localizeSchemaMap(childMap, childName, locale)
					continue
				}
				localizedProps[childName] = cloneSchemaValue(childRaw)
			}
			out[key] = localizedProps
		case "items", "oneOf", "anyOf", "allOf", "additionalProperties":
			out[key] = localizeSchemaValue(raw, propName, locale)
		case "description":
			desc, ok := raw.(string)
			if !ok || !isLocaleProperty(propName) {
				out[key] = cloneSchemaValue(raw)
				continue
			}
			out[key] = localizeLocaleDescription(desc, locale)
		default:
			out[key] = cloneSchemaValue(raw)
		}
	}
	return out
}

func localizeSchemaValue(v interface{}, propName, locale string) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return localizeSchemaMap(t, propName, locale)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i := range t {
			out[i] = localizeSchemaValue(t[i], propName, locale)
		}
		return out
	default:
		return cloneSchemaValue(v)
	}
}

func isLocaleProperty(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "lang", "locale", "language":
		return true
	default:
		return false
	}
}

func localizeLocaleDescription(description, locale string) string {
	out := localeEgPattern.ReplaceAllString(description, fmt.Sprintf("e.g., %s", locale))
	out = localeExamplesPattern.ReplaceAllString(out, fmt.Sprintf("Example: %s", locale))
	out = localeDefaultContextPattern.ReplaceAllString(out, fmt.Sprintf("Default: from context or %s", locale))
	out = localeDefaultPattern.ReplaceAllStringFunc(out, func(match string) string {
		if strings.Contains(strings.ToLower(match), "from context or") {
			return match
		}
		if strings.HasPrefix(match, "Default") {
			return fmt.Sprintf("Default: %s", locale)
		}
		return fmt.Sprintf("default: %s", locale)
	})
	return out
}
