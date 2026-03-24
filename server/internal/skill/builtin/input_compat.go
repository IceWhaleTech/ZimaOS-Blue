package builtin

import "strings"

func firstTrimmedStringValue(input map[string]any, keys ...string) string {
	if input == nil {
		return ""
	}
	for _, key := range keys {
		raw, ok := input[key]
		if !ok {
			continue
		}
		value, ok := raw.(string)
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeStringAlias(input map[string]any, canonical string, aliases ...string) {
	if input == nil {
		return
	}
	if current := firstTrimmedStringValue(input, canonical); current != "" {
		input[canonical] = current
		return
	}
	if value := firstTrimmedStringValue(input, aliases...); value != "" {
		input[canonical] = value
	}
}

func normalizeUniqueStringAlias(input map[string]any, canonical string, aliases ...string) {
	if input == nil {
		return
	}
	if current := firstTrimmedStringValue(input, canonical); current != "" {
		input[canonical] = current
		return
	}
	distinct := make(map[string]struct{}, len(aliases))
	var normalized string
	for _, key := range aliases {
		value := firstTrimmedStringValue(input, key)
		if value == "" {
			continue
		}
		distinct[value] = struct{}{}
		normalized = value
		if len(distinct) > 1 {
			return
		}
	}
	if len(distinct) == 1 {
		input[canonical] = normalized
	}
}
