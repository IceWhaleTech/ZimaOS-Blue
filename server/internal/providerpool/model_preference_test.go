package providerpool

import (
	"fmt"
	"testing"
)

func TestPreferredAPIFormatsForModel_GPT54ProPrefersResponses(t *testing.T) {
	formats := PreferredAPIFormatsForModel("gpt-5.4-pro")
	if len(formats) == 0 {
		t.Fatal("PreferredAPIFormatsForModel returned no formats")
	}
	if formats[0] != APIFormatResponses {
		t.Fatalf("first preferred format = %q, want %q", formats[0], APIFormatResponses)
	}
}

func TestSortModelIDsByPreference_SortsSmallListsDescending(t *testing.T) {
	models := []string{
		"o3-mini",
		"gemini-2.5-pro",
		"gpt-4o-mini",
		"claude-haiku-4-5",
	}

	sorted := sortModelIDsByPreference(models)

	want := []string{"o3-mini", "gpt-4o-mini", "gemini-2.5-pro", "claude-haiku-4-5"}
	for index, expected := range want {
		if got := sorted[index]; got != expected {
			t.Fatalf("unexpected order at %d: got %q want %q", index, got, expected)
		}
	}
}

func TestSortModelIDsByPreference_SortsLargeListsDescending(t *testing.T) {
	models := []string{
		"o3-mini",
		"gemini-2.5-pro",
		"gpt-4o-mini",
		"claude-haiku-4-5",
	}
	for i := 0; i < preferredModelPriorityThreshold; i++ {
		models = append(models, fmt.Sprintf("misc-%03d", i))
	}

	sorted := sortModelIDsByPreference(models)

	if got := sorted[0]; got != "o3-mini" {
		t.Fatalf("expected descending sort to keep o3-mini first, got %q", got)
	}
	if got := sorted[1]; got != "misc-099" {
		t.Fatalf("expected highest misc entry second, got %q", got)
	}
}

func TestSortModelIDsByPreference_SortsNamespacedModelsByNormalizedName(t *testing.T) {
	models := []string{
		"openai/o3-mini",
		"google/gemini-2.5-pro",
		"openai/gpt-4.1-mini",
		"anthropic/claude-sonnet-4-5",
	}
	for i := 0; i < preferredModelPriorityThreshold; i++ {
		models = append(models, fmt.Sprintf("provider/misc-%03d", i))
	}

	sorted := sortModelIDsByPreference(models)

	if got := sorted[0]; got != "openai/o3-mini" {
		t.Fatalf("expected namespaced o3 model first, got %q", got)
	}
	if got := sorted[1]; got != "provider/misc-099" {
		t.Fatalf("expected highest namespaced misc model second, got %q", got)
	}
}
