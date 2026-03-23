package tools

import "testing"

func TestResolveOfficeThemeFallsBackFromStyleHint(t *testing.T) {
	tests := []struct {
		name      string
		styleHint string
		want      string
	}{
		{name: "ui review", styleHint: "UI audit findings", want: "ui_review"},
		{name: "executive", styleHint: "Board briefing deck", want: "executive"},
		{name: "clean", styleHint: "极简 方案", want: "clean"},
		{name: "default", styleHint: "something else", want: "analysis"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveOfficeTheme("", tc.styleHint)
			if got.Name != tc.want {
				t.Fatalf("resolveOfficeTheme(%q).Name = %q, want %q", tc.styleHint, got.Name, tc.want)
			}
		})
	}
}
