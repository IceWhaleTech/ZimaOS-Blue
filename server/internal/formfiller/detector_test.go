package formfiller

import (
	"testing"
)

func TestDetectFieldType(t *testing.T) {
	detector := NewDetector(DefaultPatterns())

	tests := []struct {
		name       string
		attrs      FieldAttributes
		wantType   FieldType
		wantSource DetectionSource
		minConf    float64
	}{
		{
			name: "autocomplete email",
			attrs: FieldAttributes{
				Autocomplete: "email",
			},
			wantType:   FieldEmail,
			wantSource: DetectionAttribute,
			minConf:    0.9,
		},
		{
			name: "input type email",
			attrs: FieldAttributes{
				Type: "email",
			},
			wantType:   FieldEmail,
			wantSource: DetectionAttribute,
			minConf:    0.9,
		},
		{
			name: "input type password",
			attrs: FieldAttributes{
				Type: "password",
			},
			wantType:   FieldPassword,
			wantSource: DetectionAttribute,
			minConf:    0.9,
		},
		{
			name: "name attribute email",
			attrs: FieldAttributes{
				Name: "user_email",
			},
			wantType:   FieldEmail,
			wantSource: DetectionAttribute,
			minConf:    0.5,
		},
		{
			name: "id attribute phone",
			attrs: FieldAttributes{
				ID: "phone-number",
			},
			wantType:   FieldPhone,
			wantSource: DetectionAttribute,
			minConf:    0.5,
		},
		{
			name: "placeholder email",
			attrs: FieldAttributes{
				Placeholder: "your email",
			},
			wantType:   FieldEmail,
			wantSource: DetectionLabel,
			minConf:    0.4,
		},
		{
			name: "label text",
			attrs: FieldAttributes{
				Label: "First Name",
			},
			wantType:   FieldFirstName,
			wantSource: DetectionLabel,
			minConf:    0.4,
		},
		{
			name: "chinese email pattern",
			attrs: FieldAttributes{
				Name: "邮箱",
			},
			wantType:   FieldEmail,
			wantSource: DetectionAttribute,
			minConf:    0.5,
		},
		{
			name: "autocomplete given-name",
			attrs: FieldAttributes{
				Autocomplete: "given-name",
			},
			wantType:   FieldFirstName,
			wantSource: DetectionAttribute,
			minConf:    0.9,
		},
		{
			name: "autocomplete postal-code",
			attrs: FieldAttributes{
				Autocomplete: "postal-code",
			},
			wantType:   FieldZipCode,
			wantSource: DetectionAttribute,
			minConf:    0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fieldType, confidence, source := detector.DetectFieldType(tt.attrs)

			if fieldType != tt.wantType {
				t.Errorf("DetectFieldType() type = %v, want %v", fieldType, tt.wantType)
			}

			if source != tt.wantSource {
				t.Errorf("DetectFieldType() source = %v, want %v", source, tt.wantSource)
			}

			if confidence < tt.minConf {
				t.Errorf("DetectFieldType() confidence = %v, want >= %v", confidence, tt.minConf)
			}
		})
	}
}

func TestDetectFields(t *testing.T) {
	detector := NewDetector(DefaultPatterns())

	template := &FillTemplate{
		ID:   "test",
		Name: "Test",
		Fields: map[string]string{
			"email":     "test@example.com",
			"firstName": "John",
			"lastName":  "Doe",
		},
	}

	fields := []FieldAttributes{
		{ID: "email", Type: "email"},
		{Name: "first_name", Placeholder: "First Name"},
		{Name: "last_name", Label: "Last Name"},
	}

	detected := detector.DetectFields(fields, template)

	if len(detected) != 3 {
		t.Fatalf("Expected 3 detected fields, got %d", len(detected))
	}

	// Check email field
	if detected[0].FieldType != FieldEmail {
		t.Errorf("Expected email field type, got %v", detected[0].FieldType)
	}
	if detected[0].SuggestedValue != "test@example.com" {
		t.Errorf("Expected suggested value 'test@example.com', got '%s'", detected[0].SuggestedValue)
	}

	// Check first name field
	if detected[1].FieldType != FieldFirstName {
		t.Errorf("Expected firstName field type, got %v", detected[1].FieldType)
	}
	if detected[1].SuggestedValue != "John" {
		t.Errorf("Expected suggested value 'John', got '%s'", detected[1].SuggestedValue)
	}
}

func TestMatchAutocomplete(t *testing.T) {
	detector := NewDetector(DefaultPatterns())

	tests := []struct {
		autocomplete string
		wantType     FieldType
		wantOK       bool
	}{
		{"email", FieldEmail, true},
		{"tel", FieldPhone, true},
		{"given-name", FieldFirstName, true},
		{"family-name", FieldLastName, true},
		{"street-address", FieldAddress, true},
		{"postal-code", FieldZipCode, true},
		{"country", FieldCountry, true},
		{"username", FieldUsername, true},
		{"current-password", FieldPassword, true},
		{"new-password", FieldPassword, true},
		{"organization", FieldCompany, true},
		{"unknown-value", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.autocomplete, func(t *testing.T) {
			fieldType, ok := detector.matchAutocomplete(tt.autocomplete)
			if ok != tt.wantOK {
				t.Errorf("matchAutocomplete(%s) ok = %v, want %v", tt.autocomplete, ok, tt.wantOK)
			}
			if ok && fieldType != tt.wantType {
				t.Errorf("matchAutocomplete(%s) type = %v, want %v", tt.autocomplete, fieldType, tt.wantType)
			}
		})
	}
}

func TestGenerateSelector(t *testing.T) {
	detector := NewDetector(DefaultPatterns())

	tests := []struct {
		name     string
		attrs    FieldAttributes
		index    int
		expected string
	}{
		{
			name:     "with ID",
			attrs:    FieldAttributes{ID: "email-input"},
			index:    0,
			expected: "#email-input",
		},
		{
			name:     "with name only",
			attrs:    FieldAttributes{Name: "email"},
			index:    0,
			expected: "[name=\"email\"]",
		},
		{
			name:     "with neither",
			attrs:    FieldAttributes{Placeholder: "Email"},
			index:    2,
			expected: "input:nth-of-type(3)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := detector.generateSelector(tt.attrs, tt.index)
			if selector != tt.expected {
				t.Errorf("generateSelector() = %s, want %s", selector, tt.expected)
			}
		})
	}
}
