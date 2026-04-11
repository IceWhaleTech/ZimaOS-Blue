package server

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestProviderAccelerationWarmupDecision_RequiresTwoStableSlowResponsesTurns(t *testing.T) {
	handler := newProviderAccelerationTestHandler(t)
	convID := "conv-accel-stable"
	state := memory.ConversationCommandState{SelectedProviderID: "prov-responses"}

	handler.recordProviderAccelerationObservation(convID, providerAccelerationObservation{
		Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
		TTFTMs:          1850,
		InputTokens:     1680,
		CacheReuseRatio: 0.18,
		CacheReuseKnown: true,
	})
	handler.recordProviderAccelerationObservation(convID, providerAccelerationObservation{
		Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
		TTFTMs:          2100,
		InputTokens:     1940,
		CacheReuseRatio: 0.23,
		CacheReuseKnown: true,
	})

	decision := handler.providerAccelerationWarmupDecision(convID, "auto", state)
	if !decision.Eligible {
		t.Fatalf("decision.Eligible = false, want true (reason=%q)", decision.Reason)
	}
	if decision.Route.ProviderID != "prov-responses" || decision.Route.Model != "gpt-5-codex-responses" {
		t.Fatalf("decision.Route = %#v, want prov-responses/gpt-5-codex-responses", decision.Route)
	}
}

func TestProviderAccelerationWarmupDecision_SkipsShortFastOrHighCacheReuseTurns(t *testing.T) {
	tests := []struct {
		name string
		obs  []providerAccelerationObservation
	}{
		{
			name: "short input",
			obs: []providerAccelerationObservation{
				{
					Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
					TTFTMs:          1900,
					InputTokens:     900,
					CacheReuseRatio: 0.10,
					CacheReuseKnown: true,
				},
				{
					Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
					TTFTMs:          2200,
					InputTokens:     1600,
					CacheReuseRatio: 0.15,
					CacheReuseKnown: true,
				},
			},
		},
		{
			name: "fast ttft",
			obs: []providerAccelerationObservation{
				{
					Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
					TTFTMs:          900,
					InputTokens:     1500,
					CacheReuseRatio: 0.10,
					CacheReuseKnown: true,
				},
				{
					Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
					TTFTMs:          2100,
					InputTokens:     1700,
					CacheReuseRatio: 0.12,
					CacheReuseKnown: true,
				},
			},
		},
		{
			name: "high cache reuse",
			obs: []providerAccelerationObservation{
				{
					Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
					TTFTMs:          1900,
					InputTokens:     1500,
					CacheReuseRatio: 0.72,
					CacheReuseKnown: true,
				},
				{
					Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
					TTFTMs:          2200,
					InputTokens:     1800,
					CacheReuseRatio: 0.68,
					CacheReuseKnown: true,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := newProviderAccelerationTestHandler(t)
			convID := "conv-accel-skip"
			state := memory.ConversationCommandState{SelectedProviderID: "prov-responses"}
			for _, obs := range tc.obs {
				handler.recordProviderAccelerationObservation(convID, obs)
			}

			decision := handler.providerAccelerationWarmupDecision(convID, "auto", state)
			if decision.Eligible {
				t.Fatalf("decision.Eligible = true, want false for %s", tc.name)
			}
		})
	}
}

func TestProviderAccelerationWarmupDecision_SuppressesAfterRouteDrift(t *testing.T) {
	handler := newProviderAccelerationTestHandler(t)
	convID := "conv-accel-drift"
	state := memory.ConversationCommandState{SelectedProviderID: "prov-responses"}

	handler.recordProviderAccelerationObservation(convID, providerAccelerationObservation{
		Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
		TTFTMs:          1850,
		InputTokens:     1680,
		CacheReuseRatio: 0.18,
		CacheReuseKnown: true,
	})
	handler.recordProviderAccelerationObservation(convID, providerAccelerationObservation{
		Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
		TTFTMs:          2100,
		InputTokens:     1940,
		CacheReuseRatio: 0.23,
		CacheReuseKnown: true,
	})
	handler.noteProviderAccelerationWarmupStarted(convID, providerAccelerationRoute{
		ProviderID: "prov-responses",
		Model:      "gpt-5-codex-responses",
	})

	turn, ok := handler.claimProviderAccelerationTurn(convID, "auto", state)
	if !ok {
		t.Fatal("claimProviderAccelerationTurn = false, want true")
	}

	handler.recordProviderAccelerationTurnResult(convID, turn, "prov-backup", "gpt-5-codex-responses", 2400, 2000, 0, 0, true)

	decision := handler.providerAccelerationWarmupDecision(convID, "auto", state)
	if decision.Eligible {
		t.Fatalf("decision.Eligible = true, want false after route drift")
	}
}

func TestBuildProviderWarmupRequest_DisablesResponsesContinuation(t *testing.T) {
	handler := newProviderAccelerationTestHandler(t)
	convID := "conv-accel-warmup"

	handler.setPreviousResponseID(convID, "resp_prev_live")
	handler.setProviderAffinity(convID, "prov-responses", "")
	handler.recordProviderAccelerationObservation(convID, providerAccelerationObservation{
		Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
		TTFTMs:          1850,
		InputTokens:     1680,
		CacheReuseRatio: 0.18,
		CacheReuseKnown: true,
	})
	handler.recordProviderAccelerationObservation(convID, providerAccelerationObservation{
		Route:           providerAccelerationRoute{ProviderID: "prov-responses", Model: "gpt-5-codex-responses"},
		TTFTMs:          2100,
		InputTokens:     1940,
		CacheReuseRatio: 0.23,
		CacheReuseKnown: true,
	})

	ctx := context.Background()
	warmup := &warmupResult{
		systemPromptMessages: []llm.Message{{Role: llm.RoleSystem, Content: "system"}},
	}
	req, llmCtx, ok := handler.buildProviderWarmupRequest(ctx, convID, "auto", warmup)
	if !ok {
		t.Fatal("buildProviderWarmupRequest = false, want true")
	}
	if got := req.PreviousResponseID; got != "" {
		t.Fatalf("req.PreviousResponseID = %q, want empty", got)
	}
	if !proxy.DisableResponsesContinuationFromContext(llmCtx) {
		t.Fatal("warmup context should disable responses continuation")
	}
}

func newProviderAccelerationTestHandler(t *testing.T) *ChatHandler {
	t.Helper()

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	t.Cleanup(func() {
		handler.Close()
		_ = store.Close()
	})

	registryStorage, err := providerpool.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("providerpool.NewFileStorage: %v", err)
	}
	registry, err := providerpool.NewRegistry(registryStorage)
	if err != nil {
		t.Fatalf("providerpool.NewRegistry: %v", err)
	}
	if err := registry.Register(&providerpool.Provider{
		ID:        "prov-responses",
		Name:      "prov-responses",
		Type:      providerpool.ProviderTypeCustom,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		BaseURL:   "https://example.com/backend-api/codex/responses",
		APIFormat: providerpool.APIFormatResponses,
	}); err != nil {
		t.Fatalf("registry.Register prov-responses: %v", err)
	}
	if err := registry.Register(&providerpool.Provider{
		ID:        "prov-backup",
		Name:      "prov-backup",
		Type:      providerpool.ProviderTypeCustom,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		BaseURL:   "https://example.com/backend-api/codex/responses",
		APIFormat: providerpool.APIFormatResponses,
	}); err != nil {
		t.Fatalf("registry.Register prov-backup: %v", err)
	}

	handler.SetProviderPool(&providerpool.Pool{Registry: registry})
	return handler
}
