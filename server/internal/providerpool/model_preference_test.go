package providerpool

import (
	"fmt"
	"testing"
)

func TestSortModelIDsByPreference_LeavesSmallListsUnchanged(t *testing.T) {
	models := []string{
		"o3-mini",
		"gemini-2.5-pro",
		"gpt-4o-mini",
		"claude-haiku-4-5",
	}

	sorted := sortModelIDsByPreference(models)

	for index, want := range models {
		if got := sorted[index]; got != want {
			t.Fatalf("expected small list to preserve order at %d: got %q want %q", index, got, want)
		}
	}
}

func TestSortModelIDsByPreference_PrioritizesClaudeAndGptPrefixesForLargeLists(t *testing.T) {
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

	if got := sorted[0]; got != "claude-haiku-4-5" {
		t.Fatalf("expected claude prefix model first, got %q", got)
	}
	if got := sorted[1]; got != "gpt-4o-mini" {
		t.Fatalf("expected gpt prefix model second, got %q", got)
	}
}

func TestSortModelIDsByPreference_PrioritizesNamespacedClaudeAndGptPrefixesForLargeLists(t *testing.T) {
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

	if got := sorted[0]; got != "anthropic/claude-sonnet-4-5" {
		t.Fatalf("expected namespaced claude model first, got %q", got)
	}
	if got := sorted[1]; got != "openai/gpt-4.1-mini" {
		t.Fatalf("expected namespaced gpt model second, got %q", got)
	}
}
