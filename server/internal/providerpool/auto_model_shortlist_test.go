package providerpool

import (
	"fmt"
	"testing"
)

func TestShortlistModelsForAuto_KeepsSmallListsAndSortsDescending(t *testing.T) {
	models := []*Model{
		{ID: "claude-sonnet-4-5"},
		{ID: "gpt-4.1"},
		{ID: "claude-opus-4-1"},
		{ID: "claude-haiku-4-5"},
	}

	shortlisted := shortlistModelsForAuto(models)

	want := []string{
		"gpt-4.1",
		"claude-sonnet-4-5",
		"claude-opus-4-1",
		"claude-haiku-4-5",
	}
	assertAutoModelIDs(t, shortlisted, want)
}

func TestShortlistModelsForAuto_PrefersKnownFamiliesOnLongLists(t *testing.T) {
	models := []*Model{
		{ID: "zeta-1"},
		{ID: "claude-sonnet-4-5"},
		{ID: "gpt-4.1"},
		{ID: "glm-4.5"},
		{ID: "misc-1"},
		{ID: "haiku-4-5"},
		{ID: "m2.7"},
		{ID: "k2.5"},
		{ID: "misc-2"},
	}

	shortlisted := shortlistModelsForAuto(models)

	want := []string{
		"m2.7",
		"k2.5",
		"haiku-4-5",
		"gpt-4.1",
		"glm-4.5",
		"claude-sonnet-4-5",
	}
	assertAutoModelIDs(t, shortlisted, want)
}

func TestShortlistModelsForAuto_UsesSecondaryKeywordsWhenStillVeryLong(t *testing.T) {
	models := []*Model{
		{ID: "glm-4.5"},
		{ID: "claude-sonnet-4-5"},
		{ID: "gpt-5"},
		{ID: "codex-mini"},
	}
	for index := 0; index < 30; index++ {
		models = append(models, &Model{ID: fmt.Sprintf("glm-%02d", index)})
	}

	shortlisted := shortlistModelsForAuto(models)

	want := []string{
		"gpt-5",
		"codex-mini",
		"claude-sonnet-4-5",
	}
	assertAutoModelIDs(t, shortlisted, want)
}

func TestShortlistModelsForAuto_TruncatesToFirstEightWhenStillVeryLong(t *testing.T) {
	models := make([]*Model, 0, 40)
	for index := 0; index < 40; index++ {
		models = append(models, &Model{ID: fmt.Sprintf("gpt-%02d", index)})
	}

	shortlisted := shortlistModelsForAuto(models)

	if len(shortlisted) != autoModelFinalCandidateLimit {
		t.Fatalf("expected %d models after truncation, got %d", autoModelFinalCandidateLimit, len(shortlisted))
	}
	want := []string{
		"gpt-39",
		"gpt-38",
		"gpt-37",
		"gpt-36",
		"gpt-35",
		"gpt-34",
		"gpt-33",
		"gpt-32",
	}
	assertAutoModelIDs(t, shortlisted, want)
}

func assertAutoModelIDs(t *testing.T, models []*Model, want []string) {
	t.Helper()
	if len(models) != len(want) {
		t.Fatalf("expected %d models, got %d", len(want), len(models))
	}
	for index, expected := range want {
		if got := models[index].ID; got != expected {
			t.Fatalf("model %d mismatch: got %q want %q", index, got, expected)
		}
	}
}
