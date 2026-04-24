package pdf

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func classifyNativePDFWidgetType(rawType string, isListChoice bool) string {
	raw := strings.ToLower(strings.TrimSpace(rawType))
	switch {
	case raw == "":
		return "unknown"
	case strings.Contains(raw, "text"):
		return "text"
	case strings.Contains(raw, "choice"):
		if isListChoice {
			return "list"
		}
		return "combo"
	case strings.Contains(raw, "checkbox"):
		return "checkbox"
	case strings.Contains(raw, "radio"):
		return "radio"
	case strings.Contains(raw, "button"):
		return "button"
	case strings.Contains(raw, "signature"):
		return "signature"
	default:
		return "unknown"
	}
}

func normalizeNativePDFButtonControlType(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(raw, "radio"):
		return "radio"
	case strings.Contains(raw, "check"):
		return "checkbox"
	case strings.Contains(raw, "push"):
		return "button"
	default:
		return ""
	}
}

func classifyNativePDFButtonFieldType(controlTypeHint string, exportValue string, groupSize int) string {
	if normalized := normalizeNativePDFButtonControlType(controlTypeHint); normalized != "" {
		return normalized
	}
	exportValue = strings.TrimSpace(exportValue)
	switch {
	case groupSize > 1 && exportValue != "":
		return "radio"
	case exportValue != "":
		return "checkbox"
	default:
		return "button"
	}
}

func supportsNativePDFFormFill(fieldType string) bool {
	switch strings.ToLower(strings.TrimSpace(fieldType)) {
	case "text", "combo", "list", "checkbox", "radio":
		return true
	default:
		return false
	}
}

func resolvePDFChoiceOptionIndex(field FormField, requested string) (int, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return 0, fmt.Errorf("requested option cannot be empty")
	}
	for _, option := range field.Options {
		if strings.TrimSpace(option.Label) == requested {
			return option.Index, nil
		}
	}
	if idx, err := strconv.Atoi(requested); err == nil {
		for _, option := range field.Options {
			if option.Index == idx {
				return idx, nil
			}
		}
	}
	available := make([]string, 0, len(field.Options))
	for _, option := range field.Options {
		label := strings.TrimSpace(option.Label)
		if label == "" {
			label = fmt.Sprintf("%d", option.Index)
		} else {
			label = fmt.Sprintf("%d:%s", option.Index, label)
		}
		available = append(available, label)
	}
	return 0, fmt.Errorf("unsupported option %q (available: %s)", requested, strings.Join(available, ", "))
}

func resolvePDFCheckboxState(field FormField, requested string) (bool, error) {
	requested = strings.TrimSpace(requested)
	switch strings.ToLower(requested) {
	case "1", "true", "yes", "on", "checked":
		return true, nil
	case "0", "false", "no", "off", "unchecked":
		return false, nil
	}
	if exportValue := strings.TrimSpace(field.ExportValue); exportValue != "" && strings.EqualFold(exportValue, requested) {
		return true, nil
	}
	return false, fmt.Errorf("unsupported checkbox value %q", requested)
}

func resolvePDFRadioFieldIndex(fields []FormField, requested string) (int, error) {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return 0, fmt.Errorf("requested export value cannot be empty")
	}

	available := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for index, field := range fields {
		exportValue := strings.TrimSpace(field.ExportValue)
		if exportValue == "" {
			continue
		}
		if _, ok := seen[exportValue]; !ok {
			seen[exportValue] = struct{}{}
			available = append(available, exportValue)
		}
		if strings.EqualFold(exportValue, requested) {
			return index, nil
		}
	}

	return 0, fmt.Errorf("unsupported export value %q (available: %s)", requested, strings.Join(available, ", "))
}

func ensurePDFRadioGroupWritable(fields []FormField) error {
	for _, field := range fields {
		if !field.ReadOnly {
			continue
		}
		return fmt.Errorf("radio group contains read-only option %q", pdfFormFieldOptionIdentity(field))
	}
	return nil
}

func pdfFormFieldOptionIdentity(field FormField) string {
	candidates := []string{
		strings.TrimSpace(field.ExportValue),
		strings.TrimSpace(field.Value),
		strings.TrimSpace(field.AlternateName),
		strings.TrimSpace(field.Name),
	}
	for _, candidate := range candidates {
		if candidate != "" {
			return candidate
		}
	}
	return "unnamed"
}

func sortedPDFFieldNames(values map[string]string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		names = append(names, trimmed)
	}
	sort.Strings(names)
	return names
}

func compactFormWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	out := make([]string, 0, len(warnings))
	seen := make(map[string]struct{}, len(warnings))
	for _, warning := range warnings {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		out = append(out, warning)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
