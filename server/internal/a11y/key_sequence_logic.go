package a11y

import "strings"

func normalizeKeySequence(keys []string) ([]string, error) {
	cleaned := make([]string, 0, len(keys))
	for _, key := range keys {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	if len(cleaned) == 0 {
		return nil, NewError("unsupported_action", "keys are required", nil)
	}
	return cleaned, nil
}

func handleLiteralTextKeySequence(
	keys []string,
	isModifier func(string) bool,
	isNamedKey func(string) bool,
	sendText func(string) error,
) ([]string, bool, error) {
	cleaned, err := normalizeKeySequence(keys)
	if err != nil {
		return nil, false, err
	}
	if len(cleaned) != 1 {
		return cleaned, false, nil
	}
	modifier := false
	if isModifier != nil {
		modifier = isModifier(cleaned[0])
	}
	namedKey := false
	if isNamedKey != nil {
		namedKey = isNamedKey(cleaned[0])
	}
	if modifier || namedKey || len([]rune(cleaned[0])) == 0 {
		return cleaned, false, nil
	}
	if sendText != nil {
		if err := sendText(cleaned[0]); err != nil {
			return cleaned, true, err
		}
	}
	return cleaned, true, nil
}
