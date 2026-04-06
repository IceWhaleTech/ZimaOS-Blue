package server

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/chatcmd"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/labstack/echo/v4"
)

type conversationCommandStateResponse struct {
	ConversationID     string `json:"conversation_id,omitempty"`
	SelectedProviderID string `json:"selected_provider_id,omitempty"`
	SelectedModelID    string `json:"selected_model_id,omitempty"`
	Offline            bool   `json:"offline"`
}

type conversationCommandStatePatchRequest struct {
	SelectedProviderID *string `json:"selected_provider_id,omitempty"`
	SelectedModelID    *string `json:"selected_model_id,omitempty"`
	Offline            *bool   `json:"offline,omitempty"`
}

func commandStateToResponse(state memory.ConversationCommandState) conversationCommandStateResponse {
	return conversationCommandStateResponse{
		ConversationID:     state.ConversationID,
		SelectedProviderID: state.SelectedProviderID,
		SelectedModelID:    state.SelectedModelID,
		Offline:            state.Offline,
	}
}

func memoryCommandStateToCore(state memory.ConversationCommandState) chatcmd.CommandState {
	state = normalizeConversationCommandToolState(state)
	return chatcmd.CommandState{
		ConversationID:      state.ConversationID,
		SelectedProviderID:  state.SelectedProviderID,
		SelectedModelID:     state.SelectedModelID,
		Offline:             state.Offline,
		WebSearchEnabled:    state.WebSearchEnabled,
		DeepResearchEnabled: state.DeepResearchEnabled,
	}
}

func coreCommandStateToMemory(state chatcmd.CommandState) memory.ConversationCommandState {
	return normalizeConversationCommandToolState(memory.ConversationCommandState{
		ConversationID:      state.ConversationID,
		SelectedProviderID:  state.SelectedProviderID,
		SelectedModelID:     state.SelectedModelID,
		Offline:             state.Offline,
		WebSearchEnabled:    state.WebSearchEnabled,
		DeepResearchEnabled: state.DeepResearchEnabled,
	})
}

func (h *ChatHandler) getConversationCommandState(ctx context.Context, convID string) (memory.ConversationCommandState, error) {
	state, err := h.store.GetConversationCommandState(ctx, convID)
	if err != nil {
		return state, err
	}
	return normalizeConversationCommandToolState(state), nil
}

func (h *ChatHandler) conversationCommandStateOrDefault(ctx context.Context, convID string) memory.ConversationCommandState {
	state, err := h.getConversationCommandState(ctx, convID)
	if err != nil {
		return defaultConversationCommandState(convID)
	}
	return normalizeConversationCommandToolState(state)
}

func hasPersistedConversationCommandState(state memory.ConversationCommandState) bool {
	return !state.UpdatedAt.IsZero()
}

func (h *ChatHandler) applyCommandStateToRequest(ctx context.Context, convID string, req *SendMessageRequest) memory.ConversationCommandState {
	state := h.conversationCommandStateOrDefault(ctx, convID)
	if req == nil {
		return state
	}
	// Progressive local tool exposure is the sole authority now. Legacy request
	// toggles are ignored so callers cannot disable local tool families.
	req.WebSearchEnabled = nil
	req.DeepResearchEnabled = nil
	req.ResearchModeEnabled = nil
	return state
}

func (h *ChatHandler) saveConversationCommandState(ctx context.Context, state memory.ConversationCommandState) error {
	return h.store.UpsertConversationCommandState(ctx, normalizeConversationCommandToolState(state))
}

func (h *ChatHandler) clearCommandStateForRoutingChange(ctx context.Context, convID string) {
	h.clearPreviousResponseID(convID)
}

func (h *ChatHandler) resetConversationForCommand(ctx context.Context, convID string) (int, error) {
	msgs, err := h.store.GetMessages(ctx, convID, 10000, 0)
	if err != nil {
		return 0, err
	}
	ids := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		ids = append(ids, msg.ID)
	}
	if len(ids) > 0 {
		if err := h.store.DeleteMessages(ctx, convID, ids); err != nil {
			return 0, err
		}
	}
	_ = h.store.ClearConversationCommandState(ctx, convID)
	h.clearPreviousResponseID(convID)
	h.clearProviderAffinity(convID)
	h.conversationCache.Invalidate(convID)
	h.summaryCache.Del(convID)
	return len(ids), nil
}

func (h *ChatHandler) getLatestAssistantCommandMetadata(ctx context.Context, convID string) (*chatcmd.LatestAssistantMessage, error) {
	msg, err := h.store.GetLatestAssistantMessage(ctx, convID)
	if err != nil || msg == nil {
		return nil, err
	}
	if strings.TrimSpace(msg.Provider) == "local" && (strings.TrimSpace(msg.Model) == "command" || strings.TrimSpace(msg.Model) == "offline") {
		return nil, nil
	}
	return &chatcmd.LatestAssistantMessage{Provider: strings.TrimSpace(msg.Provider), Model: strings.TrimSpace(msg.Model)}, nil
}

func (h *ChatHandler) listCommandProviders(_ context.Context) ([]chatcmd.ProviderInfo, error) {
	providers := make([]chatcmd.ProviderInfo, 0)
	modelCounts := make(map[string]int)
	for _, model := range h.listCommandModels(context.Background()) {
		modelCounts[strings.ToLower(model.ProviderID)]++
	}
	if h.providerPool != nil && h.providerPool.Registry != nil {
		for _, provider := range h.providerPool.Registry.ListEnabled() {
			if provider == nil {
				continue
			}
			providers = append(providers, chatcmd.ProviderInfo{
				ID:         provider.ID,
				Name:       provider.Name,
				Enabled:    provider.Enabled,
				Status:     string(provider.Status),
				Location:   string(provider.Location),
				BaseURL:    provider.EffectiveBaseURL(),
				APIFormat:  string(provider.APIFormat),
				ModelCount: modelCounts[strings.ToLower(provider.ID)],
			})
		}
		sort.Slice(providers, func(i, j int) bool { return providers[i].ID < providers[j].ID })
		return providers, nil
	}
	if h.providers != nil {
		for _, name := range h.providers.List() {
			providers = append(providers, chatcmd.ProviderInfo{
				ID:         name,
				Name:       name,
				Enabled:    true,
				Status:     "active",
				Location:   "unknown",
				ModelCount: modelCounts[strings.ToLower(name)],
			})
		}
		sort.Slice(providers, func(i, j int) bool { return providers[i].ID < providers[j].ID })
	}
	return providers, nil
}

func (h *ChatHandler) listCommandModels(_ context.Context) []chatcmd.ModelInfo {
	models := make([]chatcmd.ModelInfo, 0)
	if h.providerPool != nil && h.providerPool.Discovery != nil {
		for _, model := range h.providerPool.Discovery.GetAllModels() {
			if model == nil || strings.TrimSpace(model.ID) == "" {
				continue
			}
			models = append(models, chatcmd.ModelInfo{
				ID:          model.ID,
				ProviderID:  model.ProviderID,
				DisplayName: model.DisplayName,
				Enabled:     model.Enabled,
			})
		}
		return models
	}
	if h.providers != nil {
		for _, name := range h.providers.List() {
			provider := h.providers.Get(name)
			if provider == nil {
				continue
			}
			for _, modelID := range provider.Models() {
				trimmed := strings.TrimSpace(modelID)
				if trimmed == "" {
					continue
				}
				models = append(models, chatcmd.ModelInfo{ID: trimmed, ProviderID: name, DisplayName: trimmed, Enabled: true})
			}
		}
	}
	return models
}

func (h *ChatHandler) commandExecutor() *chatcmd.Executor {
	return chatcmd.NewExecutor(chatcmd.Deps{
		GetConversationTitle: func(ctx context.Context, conversationID string) (string, error) {
			conv, err := h.store.GetConversation(ctx, conversationID)
			if err != nil || conv == nil {
				return "", err
			}
			return conv.Title, nil
		},
		UpdateConversationTitle: func(ctx context.Context, conversationID, title string) error {
			if err := h.store.UpdateConversationTitle(ctx, conversationID, title); err != nil {
				return err
			}
			h.conversationCache.Invalidate(conversationID)
			return nil
		},
		CountMessages: func(ctx context.Context, conversationID string) (int, error) {
			return h.store.CountMessages(ctx, conversationID)
		},
		GetLatestAssistant: h.getLatestAssistantCommandMetadata,
		GetCommandState: func(ctx context.Context, conversationID string) (chatcmd.CommandState, error) {
			state, err := h.getConversationCommandState(ctx, conversationID)
			if err != nil {
				return memoryCommandStateToCore(defaultConversationCommandState(conversationID)), err
			}
			return memoryCommandStateToCore(state), nil
		},
		SaveCommandState: func(ctx context.Context, state chatcmd.CommandState) error {
			current, err := h.getConversationCommandState(ctx, state.ConversationID)
			if err != nil {
				return err
			}
			next := coreCommandStateToMemory(state)
			if err := h.saveConversationCommandState(ctx, next); err != nil {
				return err
			}
			if current.SelectedProviderID != next.SelectedProviderID || current.SelectedModelID != next.SelectedModelID {
				h.clearCommandStateForRoutingChange(ctx, state.ConversationID)
			}
			return nil
		},
		ResetConversation: h.resetConversationForCommand,
		HasActiveStream: func(conversationID string) bool {
			h.convStreamMu.RLock()
			_, ok := h.convToStream[conversationID]
			h.convStreamMu.RUnlock()
			return ok
		},
		StopActiveStream: func(conversationID string) bool {
			h.convStreamMu.RLock()
			streamID, ok := h.convToStream[conversationID]
			h.convStreamMu.RUnlock()
			if !ok || strings.TrimSpace(streamID) == "" {
				return false
			}
			if !h.streamController.Cancel(streamID) {
				return false
			}
			h.markConversationCancelledForResponsesContinuation(conversationID)
			return true
		},
		ListProviders: h.listCommandProviders,
		ListModels: func(ctx context.Context) ([]chatcmd.ModelInfo, error) {
			return h.listCommandModels(ctx), nil
		},
	})
}

func (h *ChatHandler) executeChatCommand(ctx context.Context, convID, message string) (content string, handled bool) {
	result, handled := h.commandExecutor().Execute(ctx, convID, message)
	if !handled {
		return "", false
	}
	return result.Content, true
}

func (h *ChatHandler) executeSlashCommand(ctx context.Context, convID, message string) (content string, handled bool) {
	return h.executeChatCommand(ctx, convID, message)
}

func (h *ChatHandler) PatchConversationCommandState(c echo.Context) error {
	convID := c.Param("id")
	if _, err := h.checkConversationOwnership(c, convID); err != nil {
		return err
	}
	var req conversationCommandStatePatchRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	state, err := h.store.GetConversationCommandState(c.Request().Context(), convID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load command state")
	}
	beforeProvider := state.SelectedProviderID
	beforeModel := state.SelectedModelID
	if req.SelectedProviderID != nil {
		state.SelectedProviderID = strings.TrimSpace(*req.SelectedProviderID)
	}
	if req.SelectedModelID != nil {
		state.SelectedModelID = strings.TrimSpace(*req.SelectedModelID)
	}
	if req.Offline != nil {
		state.Offline = *req.Offline
	}
	state = normalizeConversationCommandToolState(state)
	if err := h.store.UpsertConversationCommandState(c.Request().Context(), state); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save command state")
	}
	if beforeProvider != state.SelectedProviderID || beforeModel != state.SelectedModelID {
		h.clearCommandStateForRoutingChange(c.Request().Context(), convID)
	}
	return c.JSON(http.StatusOK, commandStateToResponse(state))
}

func normalizeConversationCommandToolState(state memory.ConversationCommandState) memory.ConversationCommandState {
	state.WebSearchEnabled = true
	state.DeepResearchEnabled = true
	return state
}
