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

func TestToModelResponsesUsesPinchBenchOnlyCatalogEntriesWithoutAddingBuiltinModels(t *testing.T) {
	ClearOfficialProviderCatalog()
	defer ClearOfficialProviderCatalog()

	score := 84.8
	SetOfficialProviderCatalog(officialProviderCatalog{
		PinchBenchModels: map[string][]officialProviderCatalogModel{
			"moonshot": {
				{
					ID:              "moonshotai/kimi-k2.5",
					DisplayName:     "Kimi K2.5",
					PinchBenchScore: &score,
					PinchBenchURL:   "https://pinchbench.com/submission/ce9bbcbd-f78b-4655-af1f-c97781320ce6",
				},
			},
		},
	})

	for _, model := range GetBuiltinModels("moonshot") {
		if model == nil {
			continue
		}
		if model.ID == "moonshotai/kimi-k2.5" || model.ID == "kimi-k2.5" {
			t.Fatalf("pinchbench-only catalog entry should not add builtin model %q", model.ID)
		}
	}

	responses := toModelResponses([]*Model{
		{
			ID:          "kimi-k2.5",
			ProviderID:  "moonshot",
			Name:        "moonshotai/kimi-k2.5",
			DisplayName: "Kimi K2.5",
			Enabled:     true,
		},
	}, nil, nil)

	if len(responses) != 1 {
		t.Fatalf("responses len = %d, want 1", len(responses))
	}
	if responses[0].PinchBenchScore == nil || *responses[0].PinchBenchScore != score {
		t.Fatalf("pinchbench-only heuristic score = %#v, want %v", responses[0].PinchBenchScore, score)
	}
	if got := responses[0].PinchBenchURL; got != "https://pinchbench.com/submission/ce9bbcbd-f78b-4655-af1f-c97781320ce6" {
		t.Fatalf("pinchbench-only heuristic url = %q, want submission page url", got)
	}
}

func TestToModelResponsesMatchesCustomProviderModelsByInferredVendor(t *testing.T) {
	ClearOfficialProviderCatalog()
	defer ClearOfficialProviderCatalog()

	openAIScore := 80.3
	grokScore := 82.4
	SetOfficialProviderCatalog(officialProviderCatalog{
		PinchBenchModels: map[string][]officialProviderCatalogModel{
			"openai": {
				{
					ID:              "openai/gpt-5-mini",
					DisplayName:     "GPT-5 Mini",
					PinchBenchScore: &openAIScore,
					PinchBenchURL:   "https://pinchbench.com/submission/openai-gpt-5-mini",
				},
			},
			"x-ai": {
				{
					ID:              "x-ai/grok-4.1-fast",
					DisplayName:     "Grok 4.1 Fast",
					PinchBenchScore: &grokScore,
					PinchBenchURL:   "https://pinchbench.com/submission/x-ai-grok-4-1-fast",
				},
			},
		},
	})

	responses := toModelResponses([]*Model{
		{
			ID:          "gpt-5-mini",
			ProviderID:  "custom-openai",
			Name:        "gpt-5-mini",
			DisplayName: "GPT-5 Mini",
			Enabled:     true,
		},
		{
			ID:          "grok-4.1-fast",
			ProviderID:  "custom-grok",
			Name:        "grok-4.1-fast",
			DisplayName: "Grok 4.1 Fast",
			Enabled:     true,
		},
	}, nil, nil)

	if len(responses) != 2 {
		t.Fatalf("responses len = %d, want 2", len(responses))
	}

	if responses[0].PinchBenchScore == nil || *responses[0].PinchBenchScore != openAIScore {
		t.Fatalf("custom openai heuristic score = %#v, want %v", responses[0].PinchBenchScore, openAIScore)
	}
	if got := responses[0].PinchBenchURL; got != "https://pinchbench.com/submission/openai-gpt-5-mini" {
		t.Fatalf("custom openai heuristic url = %q, want submission page url", got)
	}

	if responses[1].PinchBenchScore == nil || *responses[1].PinchBenchScore != grokScore {
		t.Fatalf("custom grok heuristic score = %#v, want %v", responses[1].PinchBenchScore, grokScore)
	}
	if got := responses[1].PinchBenchURL; got != "https://pinchbench.com/submission/x-ai-grok-4-1-fast" {
		t.Fatalf("custom grok heuristic url = %q, want submission page url", got)
	}
}

func TestToModelResponsesMatchesPinchBenchScoreWithoutURL(t *testing.T) {
	ClearOfficialProviderCatalog()
	defer ClearOfficialProviderCatalog()

	openAIScore := 80.3
	SetOfficialProviderCatalog(officialProviderCatalog{
		PinchBenchModels: map[string][]officialProviderCatalogModel{
			"openai": {
				{
					ID:              "openai/gpt-5-mini",
					DisplayName:     "GPT-5 Mini",
					PinchBenchScore: &openAIScore,
				},
			},
		},
	})

	responses := toModelResponses([]*Model{
		{
			ID:          "gpt-5-mini",
			ProviderID:  "custom-openai",
			Name:        "gpt-5-mini",
			DisplayName: "GPT-5 Mini",
			Enabled:     true,
		},
	}, nil, nil)

	if len(responses) != 1 {
		t.Fatalf("responses len = %d, want 1", len(responses))
	}
	if responses[0].PinchBenchScore == nil || *responses[0].PinchBenchScore != openAIScore {
		t.Fatalf("custom openai score-only heuristic score = %#v, want %v", responses[0].PinchBenchScore, openAIScore)
	}
	if got := responses[0].PinchBenchURL; got != "" {
		t.Fatalf("custom openai score-only heuristic url = %q, want empty", got)
	}
}

func TestToModelResponsesMatchesCustomProviderModelsByExtendedVendorPrefixes(t *testing.T) {
	ClearOfficialProviderCatalog()
	defer ClearOfficialProviderCatalog()

	mistralScore := 82.0
	stepScore := 85.3
	xiaomiScore := 84.0
	SetOfficialProviderCatalog(officialProviderCatalog{
		PinchBenchModels: map[string][]officialProviderCatalogModel{
			"mistralai": {
				{
					ID:              "mistralai/devstral-2512",
					DisplayName:     "Devstral 2512",
					PinchBenchScore: &mistralScore,
				},
			},
			"stepfun": {
				{
					ID:              "stepfun/step-3.5-flash",
					DisplayName:     "Step 3.5 Flash",
					PinchBenchScore: &stepScore,
				},
			},
			"xiaomi": {
				{
					ID:              "xiaomi/mimo-v2-pro",
					DisplayName:     "Mimo V2 Pro",
					PinchBenchScore: &xiaomiScore,
				},
			},
		},
	})

	responses := toModelResponses([]*Model{
		{
			ID:          "devstral-2512",
			ProviderID:  "custom-mistral",
			Name:        "devstral-2512",
			DisplayName: "Devstral 2512",
			Enabled:     true,
		},
		{
			ID:          "step-3.5-flash",
			ProviderID:  "custom-step",
			Name:        "step-3.5-flash",
			DisplayName: "Step 3.5 Flash",
			Enabled:     true,
		},
		{
			ID:          "mimo-v2-pro",
			ProviderID:  "custom-xiaomi",
			Name:        "mimo-v2-pro",
			DisplayName: "Mimo V2 Pro",
			Enabled:     true,
		},
	}, nil, nil)

	if len(responses) != 3 {
		t.Fatalf("responses len = %d, want 3", len(responses))
	}

	if responses[0].PinchBenchScore == nil || *responses[0].PinchBenchScore != mistralScore {
		t.Fatalf("custom mistral heuristic score = %#v, want %v", responses[0].PinchBenchScore, mistralScore)
	}
	if responses[1].PinchBenchScore == nil || *responses[1].PinchBenchScore != stepScore {
		t.Fatalf("custom step heuristic score = %#v, want %v", responses[1].PinchBenchScore, stepScore)
	}
	if responses[2].PinchBenchScore == nil || *responses[2].PinchBenchScore != xiaomiScore {
		t.Fatalf("custom xiaomi heuristic score = %#v, want %v", responses[2].PinchBenchScore, xiaomiScore)
	}
}
