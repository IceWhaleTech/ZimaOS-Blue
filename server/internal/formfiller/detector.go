package formfiller

import (
	"regexp"
	"strings"
)

// Detector handles field detection logic.
type Detector struct {
	patterns *FieldPatterns
}

// NewDetector creates a new field detector.
func NewDetector(patterns *FieldPatterns) *Detector {
	return &Detector{
		patterns: patterns,
	}
}

// DetectFieldType detects the field type from attributes.
func (d *Detector) DetectFieldType(attrs FieldAttributes) (FieldType, float64, DetectionSource) {
	// Try autocomplete attribute first (highest confidence)
	if attrs.Autocomplete != "" {
		if fieldType, ok := d.matchAutocomplete(attrs.Autocomplete); ok {
			return fieldType, 0.95, DetectionAttribute
		}
	}

	// Try type attribute for specific types
	if attrs.Type != "" {
		if fieldType, ok := d.matchInputType(attrs.Type); ok {
			return fieldType, 0.90, DetectionAttribute
		}
	}

	// Try name/id attributes
	if attrs.Name != "" {
		if fieldType, confidence := d.matchPattern(attrs.Name); fieldType != "" {
			return fieldType, confidence, DetectionAttribute
		}
	}
	if attrs.ID != "" {
		if fieldType, confidence := d.matchPattern(attrs.ID); fieldType != "" {
			return fieldType, confidence, DetectionAttribute
		}
	}

	// Try placeholder
	if attrs.Placeholder != "" {
		if fieldType, confidence := d.matchPattern(attrs.Placeholder); fieldType != "" {
			return fieldType, confidence * 0.9, DetectionLabel
		}
	}

	// Try label
	if attrs.Label != "" {
		if fieldType, confidence := d.matchPattern(attrs.Label); fieldType != "" {
			return fieldType, confidence * 0.85, DetectionLabel
		}
	}

	// Try format patterns
	if fieldType, confidence := d.matchFormatPattern(attrs); fieldType != "" {
		return fieldType, confidence, DetectionPattern
	}

	return FieldCustom, 0.0, DetectionAttribute
}

// matchAutocomplete matches autocomplete attribute values.
func (d *Detector) matchAutocomplete(autocomplete string) (FieldType, bool) {
	autocomplete = strings.ToLower(strings.TrimSpace(autocomplete))

	mapping := map[string]FieldType{
		"email":              FieldEmail,
		"tel":                FieldPhone,
		"given-name":         FieldFirstName,
		"family-name":        FieldLastName,
		"name":               FieldFullName,
		"street-address":     FieldAddress,
		"address-line1":      FieldAddress,
		"address-level2":     FieldCity,
		"address-level1":     FieldState,
		"postal-code":        FieldZipCode,
		"country":            FieldCountry,
		"country-name":       FieldCountry,
		"username":           FieldUsername,
		"current-password":   FieldPassword,
		"new-password":       FieldPassword,
		"bday":               FieldBirthDate,
		"organization":       FieldCompany,
		"organization-title": FieldTitle,
	}

	if fieldType, ok := mapping[autocomplete]; ok {
		return fieldType, true
	}
	return "", false
}

// matchInputType matches input type attribute.
func (d *Detector) matchInputType(inputType string) (FieldType, bool) {
	inputType = strings.ToLower(strings.TrimSpace(inputType))

	mapping := map[string]FieldType{
		"email":    FieldEmail,
		"tel":      FieldPhone,
		"password": FieldPassword,
	}

	if fieldType, ok := mapping[inputType]; ok {
		return fieldType, true
	}
	return "", false
}

// matchPattern matches a string against field patterns.
func (d *Detector) matchPattern(text string) (FieldType, float64) {
	text = strings.ToLower(text)
	// Remove common separators for better matching
	normalizedText := strings.NewReplacer("_", "", "-", "", " ", "").Replace(text)

	var bestMatch FieldType
	var bestConfidence float64

	for fieldType, patterns := range d.patterns.Patterns {
		for _, pattern := range patterns {
			pattern = strings.ToLower(pattern)
			normalizedPattern := strings.NewReplacer("_", "", "-", "", " ", "").Replace(pattern)

			// Exact match (highest confidence)
			if text == pattern || normalizedText == normalizedPattern {
				return fieldType, 0.95
			}

			// Contains match - check if pattern is contained in text
			if strings.Contains(text, pattern) || strings.Contains(normalizedText, normalizedPattern) {
				// Calculate confidence based on how much of the text the pattern covers
				confidence := float64(len(pattern)) / float64(len(text))
				if confidence > 1.0 {
					confidence = 1.0
				}
				// Boost confidence for longer patterns (more specific)
				confidence = 0.6 + (confidence * 0.35)
				if confidence > bestConfidence {
					bestMatch = fieldType
					bestConfidence = confidence
				}
			}
		}
	}

	if bestConfidence >= 0.5 {
		return bestMatch, bestConfidence
	}
	return "", 0
}

// matchFormatPattern matches format patterns (email, phone, etc.).
func (d *Detector) matchFormatPattern(attrs FieldAttributes) (FieldType, float64) {
	// Check for email format hints
	if attrs.Type == "email" {
		return FieldEmail, 0.90
	}

	// Check for phone format hints
	if attrs.Type == "tel" {
		return FieldPhone, 0.90
	}

	// Check placeholder for format hints
	placeholder := strings.ToLower(attrs.Placeholder)

	// Email pattern
	if strings.Contains(placeholder, "@") || strings.Contains(placeholder, "example.com") {
		return FieldEmail, 0.80
	}

	// Phone pattern (contains digits and common separators)
	phonePattern := regexp.MustCompile(`[\d\-\+\(\)\s]{7,}`)
	if phonePattern.MatchString(placeholder) {
		return FieldPhone, 0.75
	}

	// Date pattern
	datePatterns := []string{"yyyy", "mm/dd", "dd/mm", "年", "月", "日"}
	for _, p := range datePatterns {
		if strings.Contains(placeholder, p) {
			return FieldBirthDate, 0.70
		}
	}

	return "", 0
}

// DetectFields detects fields from a list of field attributes.
func (d *Detector) DetectFields(fields []FieldAttributes, template *FillTemplate) []DetectedField {
	result := make([]DetectedField, 0, len(fields))

	for i, attrs := range fields {
		fieldType, confidence, source := d.DetectFieldType(attrs)

		detected := DetectedField{
			Selector:        d.generateSelector(attrs, i),
			FieldType:       fieldType,
			Confidence:      confidence,
			DetectionSource: source,
			Attributes:      attrs,
		}

		// Add suggested value from template if available
		if template != nil && fieldType != FieldCustom && fieldType != FieldPassword {
			if value, ok := template.Fields[string(fieldType)]; ok {
				detected.SuggestedValue = value
			}
		}

		result = append(result, detected)
	}

	return result
}

// generateSelector generates a CSS selector for a field.
func (d *Detector) generateSelector(attrs FieldAttributes, index int) string {
	if attrs.ID != "" {
		return "#" + attrs.ID
	}
	if attrs.Name != "" {
		return "[name=\"" + attrs.Name + "\"]"
	}
	return "input:nth-of-type(" + string(rune('0'+index+1)) + ")"
}
