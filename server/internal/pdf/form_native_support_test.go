package pdf

import "testing"

func TestClassifyNativePDFWidgetType(t *testing.T) {
	tests := []struct {
		name         string
		rawType      string
		isListChoice bool
		want         string
	}{
		{name: "text", rawType: "text", want: "text"},
		{name: "choice combo", rawType: "choice", want: "combo"},
		{name: "choice list", rawType: "choice", isListChoice: true, want: "list"},
		{name: "checkbox", rawType: "checkbox", want: "checkbox"},
		{name: "radio", rawType: "radio", want: "radio"},
		{name: "button", rawType: "button", want: "button"},
		{name: "signature", rawType: "signature", want: "signature"},
		{name: "unknown", rawType: "mystery", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyNativePDFWidgetType(tt.rawType, tt.isListChoice)
			if got != tt.want {
				t.Fatalf("classifyNativePDFWidgetType(%q, %v) = %q, want %q", tt.rawType, tt.isListChoice, got, tt.want)
			}
		})
	}
}

func TestClassifyNativePDFButtonFieldType(t *testing.T) {
	tests := []struct {
		name            string
		controlTypeHint string
		exportValue     string
		groupSize       int
		want            string
	}{
		{name: "control type radio", controlTypeHint: "radioButtonControl", exportValue: "", groupSize: 1, want: "radio"},
		{name: "control type checkbox", controlTypeHint: "checkBoxControl", exportValue: "", groupSize: 1, want: "checkbox"},
		{name: "control type push", controlTypeHint: "pushButtonControl", exportValue: "Yes", groupSize: 1, want: "button"},
		{name: "single checkbox fallback", exportValue: "Yes", groupSize: 1, want: "checkbox"},
		{name: "radio group fallback", exportValue: "Email", groupSize: 2, want: "radio"},
		{name: "push button fallback", exportValue: "", groupSize: 1, want: "button"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyNativePDFButtonFieldType(tt.controlTypeHint, tt.exportValue, tt.groupSize)
			if got != tt.want {
				t.Fatalf("classifyNativePDFButtonFieldType(%q, %q, %d) = %q, want %q", tt.controlTypeHint, tt.exportValue, tt.groupSize, got, tt.want)
			}
		})
	}
}

func TestNormalizeNativePDFButtonControlType(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "radio", raw: "radioButtonControl", want: "radio"},
		{name: "checkbox", raw: "checkBoxControl", want: "checkbox"},
		{name: "push", raw: "pushButtonControl", want: "button"},
		{name: "empty", raw: "", want: ""},
		{name: "unknown", raw: "mysteryControl", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeNativePDFButtonControlType(tt.raw)
			if got != tt.want {
				t.Fatalf("normalizeNativePDFButtonControlType(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSupportsNativePDFFormFill(t *testing.T) {
	tests := []struct {
		fieldType string
		want      bool
	}{
		{fieldType: "text", want: true},
		{fieldType: "combo", want: true},
		{fieldType: "list", want: true},
		{fieldType: "checkbox", want: true},
		{fieldType: "radio", want: true},
		{fieldType: "button", want: false},
		{fieldType: "unknown", want: false},
	}

	for _, tt := range tests {
		if got := supportsNativePDFFormFill(tt.fieldType); got != tt.want {
			t.Fatalf("supportsNativePDFFormFill(%q) = %v, want %v", tt.fieldType, got, tt.want)
		}
	}
}

func TestResolvePDFRadioFieldIndex(t *testing.T) {
	fields := []FormField{
		{Type: "radio", ExportValue: "Email"},
		{Type: "radio", ExportValue: "Phone"},
	}

	index, err := resolvePDFRadioFieldIndex(fields, "Phone")
	if err != nil {
		t.Fatalf("resolvePDFRadioFieldIndex() returned error: %v", err)
	}
	if index != 1 {
		t.Fatalf("resolvePDFRadioFieldIndex() = %d, want 1", index)
	}
}

func TestResolvePDFRadioFieldIndexRejectsUnknownExportValue(t *testing.T) {
	fields := []FormField{
		{Type: "radio", ExportValue: "Email"},
		{Type: "radio", ExportValue: "Phone"},
	}

	_, err := resolvePDFRadioFieldIndex(fields, "SMS")
	if err == nil {
		t.Fatal("expected unknown export value to fail")
	}
	if got := err.Error(); got == "" || got == "SMS" {
		t.Fatalf("resolvePDFRadioFieldIndex() error = %q, want descriptive message", got)
	}
}

func TestEnsurePDFRadioGroupWritable(t *testing.T) {
	t.Run("writable group", func(t *testing.T) {
		fields := []FormField{
			{Type: "radio", ExportValue: "Email"},
			{Type: "radio", ExportValue: "Phone"},
		}

		if err := ensurePDFRadioGroupWritable(fields); err != nil {
			t.Fatalf("ensurePDFRadioGroupWritable() returned error: %v", err)
		}
	})

	t.Run("rejects readonly candidate", func(t *testing.T) {
		fields := []FormField{
			{Type: "radio", ExportValue: "Email", ReadOnly: true},
			{Type: "radio", ExportValue: "Phone"},
		}

		err := ensurePDFRadioGroupWritable(fields)
		if err == nil {
			t.Fatal("expected readonly radio group to fail")
		}
		if got := err.Error(); got == "" || got == "Email" {
			t.Fatalf("ensurePDFRadioGroupWritable() error = %q, want descriptive message", got)
		}
	})
}
