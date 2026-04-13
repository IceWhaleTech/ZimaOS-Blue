package tools

import "testing"

func TestOfficeParseNumericString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   float64
		wantOK bool
	}{
		{name: "integer", input: "42", want: 42, wantOK: true},
		{name: "decimal", input: "  -12.5  ", want: -12.5, wantOK: true},
		{name: "thousands separators", input: "1,234.75", want: 1234.75, wantOK: true},
		{name: "leading decimal", input: ".75", want: 0.75, wantOK: true},
		{name: "scientific notation", input: "6.02e3", want: 6020, wantOK: true},
		{name: "team label", input: "Team 7", wantOK: false},
		{name: "percent suffix", input: "+12%", wantOK: false},
		{name: "date string", input: "2024-01-01", wantOK: false},
		{name: "empty", input: "   ", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := officeParseNumericString(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("officeParseNumericString(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("officeParseNumericString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
