package providerpool

import "testing"

func TestSetOfficialProviderCatalogAppliesPinchBenchMetadataToBuiltinModels(t *testing.T) {
	ClearOfficialProviderCatalog()
	defer ClearOfficialProviderCatalog()

	score := 88.6
	SetOfficialProviderCatalog(officialProviderCatalog{
		Models: map[string][]officialProviderCatalogModel{
			"anthropic": {
				{
					ID:              "claude-sonnet-4-5-20250929",
					DisplayName:     "Claude Sonnet 4.5",
					PinchBenchScore: &score,
					PinchBenchURL:   "https://pinchbench.com/model/anthropic/anthropic/claude-sonnet-4.5",
				},
			},
		},
	})

	models := GetBuiltinModels("anthropic")
	if len(models) == 0 {
		t.Fatal("expected anthropic models")
	}

	model := models[0]
	if model.PinchBenchScore == nil {
		t.Fatal("expected pinchbench score to be applied")
	}
	if got := *model.PinchBenchScore; got != score {
		t.Fatalf("pinchbench score = %v, want %v", got, score)
	}
	if got := model.PinchBenchURL; got != "https://pinchbench.com/model/anthropic/anthropic/claude-sonnet-4.5" {
		t.Fatalf("pinchbench url = %q, want model page url", got)
	}
}

func TestToModelResponsesIncludesPinchBenchMetadata(t *testing.T) {
	score := 75.0
	responses := toModelResponses([]*Model{
		{
			ID:              "gpt-4o-mini",
			ProviderID:      "openai",
			Name:            "gpt-4o-mini",
			DisplayName:     "GPT-4o Mini",
			Enabled:         true,
			PinchBenchScore: &score,
			PinchBenchURL:   "https://pinchbench.com/model/openai/openai/gpt-4o-mini",
		},
	}, nil, nil)

	if len(responses) != 1 {
		t.Fatalf("responses len = %d, want 1", len(responses))
	}
	if responses[0].PinchBenchScore == nil {
		t.Fatal("expected pinchbench score in response")
	}
	if got := *responses[0].PinchBenchScore; got != score {
		t.Fatalf("response pinchbench score = %v, want %v", got, score)
	}
	if got := responses[0].PinchBenchURL; got != "https://pinchbench.com/model/openai/openai/gpt-4o-mini" {
		t.Fatalf("response pinchbench url = %q, want model page url", got)
	}
}

func TestToModelResponsesHeuristicallyMatchesPinchBenchMetadata(t *testing.T) {
	ClearOfficialProviderCatalog()
	defer ClearOfficialProviderCatalog()

	openAIScore := 75.0
	anthropicScore := 88.6
	SetOfficialProviderCatalog(officialProviderCatalog{
		Models: map[string][]officialProviderCatalogModel{
			"openai": {
				{
					ID:              "gpt-4o-mini",
					DisplayName:     "GPT-4o Mini",
					PinchBenchScore: &openAIScore,
					PinchBenchURL:   "https://pinchbench.com/model/openai/openai/gpt-4o-mini",
				},
			},
			"anthropic": {
				{
					ID:              "claude-sonnet-4-5-20250929",
					DisplayName:     "Claude Sonnet 4.5",
					PinchBenchScore: &anthropicScore,
					PinchBenchURL:   "https://pinchbench.com/model/anthropic/anthropic/claude-sonnet-4.5",
				},
			},
		},
	})

	responses := toModelResponses([]*Model{
		{
			ID:          "openai/gpt-4o-mini",
			ProviderID:  "openai",
			Name:        "openai/gpt-4o-mini",
			DisplayName: "openai/gpt-4o-mini",
			Enabled:     true,
		},
		{
			ID:          "claude-sonnet-4.5",
			ProviderID:  "anthropic",
			Name:        "claude-sonnet-4.5",
			DisplayName: "Claude Sonnet 4.5",
			Enabled:     true,
		},
	}, nil, nil)

	if len(responses) != 2 {
		t.Fatalf("responses len = %d, want 2", len(responses))
	}

	if responses[0].PinchBenchScore == nil || *responses[0].PinchBenchScore != openAIScore {
		t.Fatalf("openai heuristic score = %#v, want %v", responses[0].PinchBenchScore, openAIScore)
	}
	if got := responses[0].PinchBenchURL; got != "https://pinchbench.com/model/openai/openai/gpt-4o-mini" {
		t.Fatalf("openai heuristic url = %q, want model page url", got)
	}

	if responses[1].PinchBenchScore == nil || *responses[1].PinchBenchScore != anthropicScore {
		t.Fatalf("anthropic heuristic score = %#v, want %v", responses[1].PinchBenchScore, anthropicScore)
	}
	if got := responses[1].PinchBenchURL; got != "https://pinchbench.com/model/anthropic/anthropic/claude-sonnet-4.5" {
		t.Fatalf("anthropic heuristic url = %q, want model page url", got)
	}
}
