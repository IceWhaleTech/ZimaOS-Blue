package routingcue

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"testing"
)

func TestSupportedLocales_MatchWebLocaleCatalog(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	localeCatalogPath := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "web", "src", "i18n", "locale-catalog.ts"))
	data, err := os.ReadFile(localeCatalogPath)
	if err != nil {
		t.Fatalf("read locale catalog: %v", err)
	}

	sectionPattern := regexp.MustCompile(`(?s)export const localeKeys = \[(.*?)\] as const`)
	sectionMatch := sectionPattern.FindSubmatch(data)
	if len(sectionMatch) < 2 {
		t.Fatalf("failed to locate localeKeys in %s", localeCatalogPath)
	}
	valuePattern := regexp.MustCompile(`'([^']+)'`)
	values := valuePattern.FindAllSubmatch(sectionMatch[1], -1)
	got := make([]string, 0, len(values))
	for _, value := range values {
		got = append(got, string(value[1]))
	}

	if !reflect.DeepEqual(got, SupportedLocales()) {
		t.Fatalf("supported locales mismatch\nweb=%v\nrouting=%v", got, SupportedLocales())
	}
}

func TestLocalizedExamples_CoverAllSupportedLocales(t *testing.T) {
	for _, skill := range []string{"web_search", "analyze", "reminder", "browser", "ui_reviewer"} {
		examples := LocalizedExamples(skill)
		if len(examples) != len(SupportedLocales()) {
			t.Fatalf("%s examples = %d, want %d", skill, len(examples), len(SupportedLocales()))
		}

		seen := make(map[string]struct{}, len(examples))
		for _, example := range examples {
			if example.Locale == "" || example.Query == "" {
				t.Fatalf("%s has empty locale/query: %+v", skill, example)
			}
			if len(example.Actions) == 0 || len(example.Objects) == 0 || len(example.Context) == 0 {
				t.Fatalf("%s is missing cue terms for locale %s: %+v", skill, example.Locale, example)
			}
			if _, ok := seen[example.Locale]; ok {
				t.Fatalf("%s duplicates locale %s", skill, example.Locale)
			}
			seen[example.Locale] = struct{}{}
		}

		for _, locale := range SupportedLocales() {
			if _, ok := seen[locale]; !ok {
				t.Fatalf("%s missing locale %s", skill, locale)
			}
		}
	}
}

func TestLocalizedURLBypassExamples_CoverAllSupportedLocales(t *testing.T) {
	for _, skill := range []string{"analyze", "ui_reviewer"} {
		examples := LocalizedURLBypassExamples(skill)
		if len(examples) != len(SupportedLocales()) {
			t.Fatalf("%s URL bypass examples = %d, want %d", skill, len(examples), len(SupportedLocales()))
		}

		seen := make(map[string]struct{}, len(examples))
		for _, example := range examples {
			if example.Locale == "" || example.Query == "" {
				t.Fatalf("%s URL bypass has empty locale/query: %+v", skill, example)
			}
			if len(example.Actions) == 0 || len(example.Objects) == 0 || len(example.Context) == 0 {
				t.Fatalf("%s URL bypass is missing cue terms for locale %s: %+v", skill, example.Locale, example)
			}
			if _, ok := seen[example.Locale]; ok {
				t.Fatalf("%s URL bypass duplicates locale %s", skill, example.Locale)
			}
			seen[example.Locale] = struct{}{}
		}

		for _, locale := range SupportedLocales() {
			if _, ok := seen[locale]; !ok {
				t.Fatalf("%s URL bypass missing locale %s", skill, locale)
			}
		}
	}
}
