package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/labstack/echo/v4"
)

const (
	warmupTTL             = 30 * time.Second
	providerWarmupTimeout = 10 * time.Second
	providerWarmupPrompt  = "[WARMUP_ONLY] Prime immutable prompt prefix and tool schema cache. Reply with a single period. Do not call tools."
)

var errProviderWarmupStopped = errors.New("provider warmup stopped")

type warmupResult struct {
	systemPromptMessages []llm.Message
	preloadedMessages    []memory.Message
	beforeCount          int
	createdAt            time.Time
}

type providerWarmupState struct {
	cancel    context.CancelFunc
	startedAt time.Time
	model     string
}

func (h *ChatHandler) CancelWarmup(c echo.Context) error {
	convID := c.Param("id")
	if _, err := h.checkConversationOwnership(c, convID); err != nil {
		return err
	}
	h.clearWarmupToken(convID)
	h.cancelProviderWarmup(convID, "client_cancel")
	return c.NoContent(http.StatusNoContent)
}

func (h *ChatHandler) armWarmupToken(convID string) string {
	if strings.TrimSpace(convID) == "" {
		return ""
	}
	token := strconv.FormatInt(timeutil.NowTime().UnixNano(), 36)
	h.warmupTokenMu.Lock()
	h.warmupTokens[convID] = token
	h.warmupTokenMu.Unlock()
	return token
}

func (h *ChatHandler) clearWarmupToken(convID string) {
	if strings.TrimSpace(convID) == "" {
		return
	}
	h.warmupTokenMu.Lock()
	delete(h.warmupTokens, convID)
	h.warmupTokenMu.Unlock()
}

func (h *ChatHandler) warmupModelForConversation(convID string) string {
	model := "auto"
	if h == nil {
		return model
	}
	state := h.conversationCommandStateOrDefault(context.Background(), convID)
	if selectedModelID := strings.TrimSpace(state.SelectedModelID); selectedModelID != "" {
		model = selectedModelID
	}
	return h.defaultModelForCCCLI(model)
}

func (h *ChatHandler) isWarmupTokenCurrent(convID, token string) bool {
	if strings.TrimSpace(convID) == "" || strings.TrimSpace(token) == "" {
		return false
	}
	h.warmupTokenMu.Lock()
	current := h.warmupTokens[convID]
	h.warmupTokenMu.Unlock()
	return current == token
}

func (h *ChatHandler) cancelProviderWarmup(convID, reason string) bool {
	if strings.TrimSpace(convID) == "" {
		return false
	}
	h.providerWarmupsMu.Lock()
	state := h.providerWarmups[convID]
	delete(h.providerWarmups, convID)
	h.providerWarmupsMu.Unlock()
	if state == nil {
		return false
	}
	state.cancel()
	logger.Debug().Str("conv_id", convID).Str("reason", reason).Str("model", state.model).Msg("[warmup] cancelled active provider warmup")
	return true
}

func (h *ChatHandler) replaceProviderWarmup(convID string, next *providerWarmupState) *providerWarmupState {
	h.providerWarmupsMu.Lock()
	prev := h.providerWarmups[convID]
	if next == nil {
		delete(h.providerWarmups, convID)
	} else {
		h.providerWarmups[convID] = next
	}
	h.providerWarmupsMu.Unlock()
	return prev
}

func (h *ChatHandler) clearProviderWarmupState(convID string, state *providerWarmupState) {
	h.providerWarmupsMu.Lock()
	if current := h.providerWarmups[convID]; current == state {
		delete(h.providerWarmups, convID)
	}
	h.providerWarmupsMu.Unlock()
}

func (h *ChatHandler) startProviderWarmup(convID, model, token string, warmup *warmupResult) {
	if h == nil || h.proxyBridge == nil || warmup == nil || strings.TrimSpace(token) == "" {
		return
	}
	if !h.isWarmupTokenCurrent(convID, token) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), providerWarmupTimeout)
	state := &providerWarmupState{cancel: cancel, startedAt: timeutil.NowTime(), model: model}
	if prev := h.replaceProviderWarmup(convID, state); prev != nil {
		prev.cancel()
	}
	go h.runProviderWarmup(ctx, convID, model, token, warmup, state)
}

func (h *ChatHandler) runProviderWarmup(ctx context.Context, convID, model, token string, warmup *warmupResult, state *providerWarmupState) {
	defer state.cancel()
	defer h.clearProviderWarmupState(convID, state)

	if !h.isWarmupTokenCurrent(convID, token) {
		return
	}

	chatReq, llmCtx, ok := h.buildProviderWarmupRequest(ctx, convID, model, warmup)
	if !ok {
		return
	}

	logger.Info().
		Str("conv_id", convID).
		Str("model", model).
		Int("messages", len(chatReq.Messages)).
		Int("tools", len(chatReq.Tools)).
		Msg("[warmup] provider-side warmup started")

	err := h.proxyBridge.ChatStream(llmCtx, chatReq, func(chunk llm.StreamChunk) error {
		logger.Debug().
			Str("conv_id", convID).
			Str("model", model).
			Str("provider", chunk.Provider).
			Str("provider_id", chunk.ProviderID).
			Bool("done", chunk.Done).
			Msg("[warmup] provider-side warmup received first stream signal; stopping")
		state.cancel()
		return errProviderWarmupStopped
	})
	switch {
	case err == nil,
		errors.Is(err, errProviderWarmupStopped),
		errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		logger.Debug().Str("conv_id", convID).Str("model", model).Msg("[warmup] provider-side warmup finished")
	default:
		logger.Debug().Err(err).Str("conv_id", convID).Str("model", model).Msg("[warmup] provider-side warmup failed")
	}
}

func (h *ChatHandler) buildProviderWarmupRequest(ctx context.Context, convID, model string, warmup *warmupResult) (llm.ChatRequest, context.Context, bool) {
	var req llm.ChatRequest
	if h == nil || warmup == nil {
		return req, ctx, false
	}

	messages := make([]llm.Message, 0, len(warmup.systemPromptMessages)+1)
	messages = append(messages, warmup.systemPromptMessages...)

	state := h.conversationCommandStateOrDefault(ctx, convID)
	targetProviderID, targetProvider, ok := h.providerWarmupTarget(convID, state)
	if !ok {
		return req, ctx, false
	}
	if !supportsProviderSidePromptWarmup(targetProvider) {
		logger.Debug().
			Str("conv_id", convID).
			Str("provider_id", targetProviderID).
			Str("api_format", string(targetProvider.APIFormat)).
			Msg("[warmup] provider-side warmup skipped: explicit prefix cache unavailable on this provider format")
		return req, ctx, false
	}
	selectedTools := h.selectTools("", model)
	webSearchEnabled := state.WebSearchEnabled
	deepResearchEnabled := state.DeepResearchEnabled
	selectedTools = applyWebSearchPreference(selectedTools, &webSearchEnabled)
	selectedTools = applyDeepResearchPreference(selectedTools, &deepResearchEnabled)
	toolDefs := defsToLLMTools(selectedTools)

	if len(messages) == 0 && len(toolDefs) == 0 {
		return req, ctx, false
	}

	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: providerWarmupPrompt})
	llmCtx := proxy.WithPinnedProvider(ctx, targetProviderID)
	var resolvedRoute proxy.ResolvedRoute
	llmCtx = proxy.WithResolvedRoute(llmCtx, &resolvedRoute)

	req = llm.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0,
		MaxTokens:   1,
		Tools:       toolDefs,
		Stream:      true,
	}
	if supportsPromptCacheKey(targetProvider) {
		req.PromptCacheKey = buildSystemPromptCacheKey(targetProviderID, model, messages, req.Tools)
	}
	return req, llmCtx, true
}

func (h *ChatHandler) providerWarmupTarget(convID string, state memory.ConversationCommandState) (string, *providerpool.Provider, bool) {
	return h.promptCacheTargetProvider(convID, "", state)
}

func supportsProviderSidePromptWarmup(provider *providerpool.Provider) bool {
	if provider == nil {
		return false
	}
	return provider.APIFormat == providerpool.APIFormatAnthropic || providerpool.UsesResponsesIntegration(provider)
}

func (h *ChatHandler) applyPromptCacheKeyForRequest(convID, explicitProviderID string, state memory.ConversationCommandState, req *llm.ChatRequest) {
	if h == nil || req == nil {
		return
	}
	targetProviderID, targetProvider, ok := h.promptCacheTargetProvider(convID, explicitProviderID, state)
	if !ok || !supportsPromptCacheKey(targetProvider) {
		return
	}
	req.PromptCacheKey = buildSystemPromptCacheKey(targetProviderID, req.Model, req.Messages, req.Tools)
}

func (h *ChatHandler) promptCacheTargetProvider(convID, explicitProviderID string, state memory.ConversationCommandState) (string, *providerpool.Provider, bool) {
	if h == nil || h.providerPool == nil || h.providerPool.Registry == nil {
		return "", nil, false
	}
	providerID := strings.TrimSpace(explicitProviderID)
	if providerID == "" {
		providerID = strings.TrimSpace(state.SelectedProviderID)
	}
	if providerID == "" {
		if aff := h.getProviderAffinity(convID); aff != nil {
			providerID = strings.TrimSpace(aff.ProviderID)
		}
	}
	if providerID == "" {
		return "", nil, false
	}
	provider, err := h.providerPool.Registry.Get(providerID)
	if err != nil || provider == nil || !provider.Enabled {
		return "", nil, false
	}
	return providerID, provider, true
}

func supportsPromptCacheKey(provider *providerpool.Provider) bool {
	return providerpool.UsesResponsesIntegration(provider)
}

func buildSystemPromptCacheKey(providerID, model string, messages []llm.Message, tools []llm.Tool) string {
	system := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Role != llm.RoleSystem {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		system = append(system, content)
	}
	if len(system) == 0 {
		return ""
	}
	payload := struct {
		Version    string     `json:"version"`
		ProviderID string     `json:"provider_id"`
		Model      string     `json:"model"`
		System     []string   `json:"system"`
		Tools      []llm.Tool `json:"tools,omitempty"`
	}{
		Version:    "v1",
		ProviderID: strings.TrimSpace(providerID),
		Model:      strings.TrimSpace(model),
		System:     system,
		Tools:      tools,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "blue:prompt-cache:" + hex.EncodeToString(sum[:16])
}
