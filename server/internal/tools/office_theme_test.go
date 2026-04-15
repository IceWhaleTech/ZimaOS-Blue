package tools

import (
	"strings"
	"testing"
)

func TestThemeColorDominance(t *testing.T) {
	ensureOfficeThemeCatalog()

	tests := []struct {
		name          string
		expectedMood  string
		expectedCount int
	}{
		{"midnight", "trustworthy", 4},
		{"terracotta", "warm", 3},
		{"forest", "natural", 4},
		{"coral", "energetic", 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			theme, err := officeThemeByName(tc.name)
			if err != nil {
				t.Fatalf("theme %q not found: %v", tc.name, err)
			}

			if theme.Mood != tc.expectedMood {
				t.Errorf("theme %q mood = %q, want %q", tc.name, theme.Mood, tc.expectedMood)
			}

			if len(theme.UseCases) < tc.expectedCount {
				t.Errorf("theme %q has %d use cases, want at least %d",
					tc.name, len(theme.UseCases), tc.expectedCount)
			}

			// Verify new fields exist
			if theme.Personality == "" {
				t.Errorf("theme %q missing Personality", tc.name)
			}

			// Verify color dominance methods
			if theme.PrimaryDominance() != 0.65 {
				t.Errorf("theme %q primary dominance = %v, want 0.65", tc.name, theme.PrimaryDominance())
			}
		})
	}
}

func TestThemeInference(t *testing.T) {
	tests := []struct {
		hint string
		want string
	}{
		// New Grapwork-inspired matches
		{"startup pitch deck", "coral"},
		{"growth report", "coral"},
		{"marketing deck", "coral"},
		{"sustainability report", "forest"},
		{"carbon footprint", "forest"},
		{"wellness program", "forest"},
		{"creative brief", "terracotta"},
		{"design review", "terracotta"},
		{"brand guidelines", "terracotta"},
		{"board presentation", "midnight"},
		{"investor deck", "midnight"},
		{"annual report", "midnight"},

		// Original matches preserved
		{"ui review", "ui_review"},
		{"executive summary", "executive"},
		{"minimal doc", "clean"},
		{"unknown topic", "analysis"}, // fallback
	}

	for _, tc := range tests {
		t.Run(tc.hint, func(t *testing.T) {
			got := resolveOfficeTheme("", tc.hint)
			if got.Name != tc.want {
				t.Errorf("hint=%q got=%q want=%q", tc.hint, got.Name, tc.want)
			}
		})
	}
}

func TestListThemes(t *testing.T) {
	themes := ListThemes()
	expected := []string{"analysis", "ui_review", "executive", "clean", "midnight", "terracotta", "forest", "coral"}

	if len(themes) != len(expected) {
		t.Errorf("got %d themes, want %d", len(themes), len(expected))
	}

	// Check all expected themes exist
	themeMap := make(map[string]bool)
	for _, name := range themes {
		themeMap[name] = true
	}

	for _, want := range expected {
		if !themeMap[want] {
			t.Errorf("missing theme: %q", want)
		}
	}
}

func TestGetThemeByMood(t *testing.T) {
	tests := []struct {
		mood     string
		expected int
	}{
		{"trustworthy", 1}, // midnight
		{"energetic", 1},   // coral
		{"warm", 1},        // terracotta
		{"natural", 1},     // forest
		{"focused", 1},     // analysis
		{"balanced", 1},    // clean
	}

	for _, tc := range tests {
		t.Run(tc.mood, func(t *testing.T) {
			got := GetThemeByMood(tc.mood)
			if len(got) != tc.expected {
				t.Errorf("mood=%q got %d themes, want %d", tc.mood, len(got), tc.expected)
			}
		})
	}
}

func TestNewThemeFonts(t *testing.T) {
	// Verify new themes have diverse fonts (not just Aptos)
	newThemes := []string{"midnight", "terracotta", "forest", "coral"}
	aptosCount := 0

	for _, name := range newThemes {
		theme, _ := officeThemeByName(name)
		if theme.DisplayFont == "Aptos Display" && theme.BodyFont == "Aptos" {
			aptosCount++
		}
	}

	// At most 1 new theme should use Aptos/Aptos Display
	if aptosCount > 1 {
		t.Errorf("too many new themes using Aptos fonts: %d, want diversity", aptosCount)
	}
}

func TestGetThemePreview(t *testing.T) {
	preview, err := GetThemePreview("midnight")
	if err != nil {
		t.Fatalf("GetThemePreview(midnight) error = %v", err)
	}

	if preview.Name != "midnight" {
		t.Fatalf("preview.Name = %q, want midnight", preview.Name)
	}
	if preview.Personality != "professional yet distinctive" {
		t.Fatalf("preview.Personality = %q", preview.Personality)
	}
	if preview.Mood != "trustworthy" {
		t.Fatalf("preview.Mood = %q, want trustworthy", preview.Mood)
	}
	if preview.Fonts.Display != "Georgia" || preview.Fonts.Body != "Segoe UI" {
		t.Fatalf("preview.Fonts = %+v, want Georgia/Segoe UI", preview.Fonts)
	}
	if preview.Dominance.Primary != 0.65 || preview.Dominance.Secondary != 0.25 || preview.Dominance.Accent != 0.10 {
		t.Fatalf("preview.Dominance = %+v, want 0.65/0.25/0.10", preview.Dominance)
	}
	if len(preview.Swatches) < 3 {
		t.Fatalf("len(preview.Swatches) = %d, want at least 3", len(preview.Swatches))
	}
	if preview.Swatches[0].Role != "primary" || preview.Swatches[0].Hex != "#1E3A5F" {
		t.Fatalf("preview.Swatches[0] = %+v, want primary #1E3A5F", preview.Swatches[0])
	}
	for _, needle := range []string{"#1E3A5F", "#00D4AA", "Georgia", "professional yet distinctive"} {
		if !strings.Contains(preview.HTML, needle) {
			t.Fatalf("preview.HTML missing %q in %s", needle, preview.HTML)
		}
	}
}
