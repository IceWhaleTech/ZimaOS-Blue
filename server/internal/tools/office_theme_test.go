package tools

import (
	"sync"
	"testing"
)

func resetOfficeThemeCatalogForTest(t *testing.T) {
	t.Helper()

	originalCatalog := officeThemeCatalog

	officeThemeCatalog = nil
	officeThemeCatalogOnce = sync.Once{}

	t.Cleanup(func() {
		officeThemeCatalog = originalCatalog
		officeThemeCatalogOnce = sync.Once{}
		if len(originalCatalog) > 0 {
			officeThemeCatalogOnce.Do(func() {})
		}
	})
}

func TestOfficeThemeCatalog_InitializesOnDemand(t *testing.T) {
	resetOfficeThemeCatalogForTest(t)

	if officeThemeCatalog != nil {
		t.Fatal("expected office theme catalog to start uninitialized")
	}

	theme := resolveOfficeTheme("", "UI audit findings")
	if theme.Name != "ui_review" {
		t.Fatalf("resolveOfficeTheme() theme = %q, want %q", theme.Name, "ui_review")
	}
	if len(officeThemeCatalog) == 0 {
		t.Fatal("expected office theme catalog to initialize on first lookup")
	}

	explicit, err := officeThemeByName("clean")
	if err != nil {
		t.Fatalf("officeThemeByName(clean) error = %v", err)
	}
	if explicit.Name != "clean" {
		t.Fatalf("officeThemeByName(clean).Name = %q, want %q", explicit.Name, "clean")
	}
}

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
