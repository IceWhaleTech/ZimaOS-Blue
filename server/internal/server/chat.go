package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// estimateTokens estimates the number of tokens in a text.
// This is a rough estimation: ~4 characters per token for English,
// ~1.5 characters per token for CJK languages.
// Used as fallback when provider doesn't return usage info.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}

	// Count characters and CJK characters
	totalChars := 0
	cjkChars := 0

	for _, r := range text {
		totalChars++
		// Check if character is CJK (Chinese, Japanese, Korean)
		if isCJK(r) {
			cjkChars++
		}
	}

	// Estimate tokens:
	// - CJK characters: ~1.5 chars per token
	// - Other characters: ~4 chars per token
	nonCJKChars := totalChars - cjkChars
	cjkTokens := float64(cjkChars) / 1.5
	nonCJKTokens := float64(nonCJKChars) / 4.0

	return int(cjkTokens + nonCJKTokens + 0.5) // Round to nearest int
}

// isCJK checks if a rune is a CJK character.
func isCJK(r rune) bool {
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK Unified Ideographs Extension B-F
	if r >= 0x20000 && r <= 0x2CEAF {
		return true
	}
	// Hiragana
	if r >= 0x3040 && r <= 0x309F {
		return true
	}
	// Katakana
	if r >= 0x30A0 && r <= 0x30FF {
		return true
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	return false
}

// estimateInputTokens estimates input tokens from messages.
func estimateInputTokens(messages []llm.Message) int {
	total := 0
	for _, msg := range messages {
		// Add overhead for role and formatting (~4 tokens per message)
		total += 4
		total += estimateTokens(msg.Content)
	}
	return total
}

// providerPoolToLLM maps Provider Pool IDs to LLM provider names.
// This allows the Chat API to work with both Provider Pool IDs and legacy LLM provider names.
var providerPoolToLLM = map[string]string{
	"anthropic": "claude",
	"openai":    "openai",
	"ollama":    "ollama",
	"custom":    "custom",
	"grok":      "grok",
	"qwen":      "qwen",
	"venice":    "venice",
	"bedrock":   "bedrock",
	"glm":       "glm",
	"claude":    "claude",
}

// mapProviderID converts a Provider Pool ID to an LLM provider name.
// If no mapping exists, returns the original ID (for custom providers).
func mapProviderID(providerID string) string {
	if mapped, ok := providerPoolToLLM[providerID]; ok {
		return mapped
	}
	// For unknown providers, return as-is (might be a direct LLM provider name)
	return providerID
}

// getProviderFromPool retrieves a provider from the Provider Pool and creates an LLM provider instance.
// This handles custom providers (prov_xxx IDs) by looking up their configuration in the pool.
func (h *ChatHandler) getProviderFromPool(providerID string) (llm.Provider, error) {
	if h.providerPool == nil {
		return nil, fmt.Errorf("provider pool not configured")
	}

	// Get provider configuration from pool
	poolProvider, err := h.providerPool.Registry.Get(providerID)
	if err != nil {
		return nil, fmt.Errorf("provider not found in pool: %s", providerID)
	}

	// Check if provider is enabled
	if !poolProvider.Enabled {
		return nil, fmt.Errorf("provider is disabled: %s", providerID)
	}

	// Get API key from provider configuration
	apiKey := ""
	for _, key := range poolProvider.APIKeys {
		if key.Enabled && key.Key != "" {
			apiKey = key.Key
			break
		}
	}

	// Create LLM provider based on API format
	switch poolProvider.APIFormat {
	case providerpool.APIFormatAnthropic:
		return llm.NewClaudeProvider(apiKey, poolProvider.BaseURL), nil
	case providerpool.APIFormatOllama:
		return llm.NewOllamaProvider(poolProvider.BaseURL), nil
	default:
		// Default to OpenAI-compatible format (covers OpenAI, Google, and custom providers)
		return llm.NewCustomProvider(apiKey, poolProvider.BaseURL), nil
	}
}

// getDefaultProvider returns the best available provider from the pool.
// It selects the first enabled provider with the highest priority and creates
// a dynamic LLM provider instance using the pool configuration.
// Falls back to the legacy provider registry if no pool is configured.
// Note: This function no longer fetches models to avoid I/O overhead.
// The caller should use req.Model directly if available.
func (h *ChatHandler) getDefaultProvider() (llm.Provider, string, string, error) {
	// Try provider pool first
	if h.providerPool != nil {
		// Get enabled providers from pool, sorted by priority
		poolProviders := h.providerPool.Registry.ListEnabled()
		if len(poolProviders) > 0 {
			// Sort by priority (higher first)
			sort.Slice(poolProviders, func(i, j int) bool {
				return poolProviders[i].Priority > poolProviders[j].Priority
			})

			// Use the highest priority enabled provider
			poolProvider := poolProviders[0]

			// Check if this is a trial provider and if quota is exhausted
			if providerpool.IsTrialProvider(poolProvider.ID) {
				if h.providerPool.TrialQuotaManager != nil && h.providerPool.TrialQuotaManager.IsExhausted() {
					// Skip trial provider if quota is exhausted
					// Try to find next available provider
					for i := 1; i < len(poolProviders); i++ {
						nextProvider := poolProviders[i]
						if !providerpool.IsTrialProvider(nextProvider.ID) {
							provider, err := h.getProviderFromPool(nextProvider.ID)
							if err == nil {
								return provider, nextProvider.ID, "", nil
							}
						}
					}
					// No other providers available, return quota exhausted error
					return nil, "", "", providerpool.ErrTrialQuotaExhausted
				}
			}

			// Create LLM provider from pool configuration
			provider, err := h.getProviderFromPool(poolProvider.ID)
			if err == nil {
				// Return empty model - caller will use req.Model if available
				// This avoids expensive GetModels() call on every request
				return provider, poolProvider.ID, "", nil
			}
		}
	}

	// Fallback to legacy provider registry
	providerNames := h.providers.List()
	if len(providerNames) == 0 {
		return nil, "", "", fmt.Errorf("no available providers")
	}

	// Use first available provider from registry
	providerName := providerNames[0]
	provider := h.providers.Get(providerName)
	if provider == nil {
		return nil, "", "", fmt.Errorf("provider not found: %s", providerName)
	}

	// Get first available model
	models := provider.Models()
	model := ""
	if len(models) > 0 {
		model = models[0]
	}

	return provider, providerName, model, nil
}

// ChatHandler handles chat-related API endpoints.
type ChatHandler struct {
	store             *memory.Store
	providers         *llm.ProviderRegistry
	providerPool      *providerpool.Pool
	toolRegistry      *tools.Registry
	streamController  *claudecode.StreamController
	compactionConfig  claudecode.CompactionConfig
	claudeCodeHandler *claudecode.Handler
	metricsRecorder   MetricsRecorder
	companionManager  *companion.Manager
	promptGuard       *promptguard.Detector
	cache             *proxy.CCCache
	convToSession     map[string]string
	convMu            sync.RWMutex
}

// MetricsRecorder is an interface for recording API call metrics.
type MetricsRecorder interface {
	RecordAPICall(model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string)
	RecordSpeed(model string, tokensPerSecond, ttftMs, decodeSpeed float64)
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(store *memory.Store, providers *llm.ProviderRegistry, toolRegistry *tools.Registry) *ChatHandler {
	return &ChatHandler{
		store:            store,
		providers:        providers,
		toolRegistry:     toolRegistry,
		streamController: claudecode.NewStreamController(),
		compactionConfig: claudecode.DefaultCompactionConfig(),
		convToSession:    make(map[string]string),
	}
}

// SetMetricsRecorder sets the metrics recorder for tracking API call metrics.
func (h *ChatHandler) SetMetricsRecorder(recorder MetricsRecorder) {
	h.metricsRecorder = recorder
}

// SetClaudeCodeHandler sets the Claude Code handler for checking enabled status.
func (h *ChatHandler) SetClaudeCodeHandler(handler *claudecode.Handler) {
	h.claudeCodeHandler = handler
}

// SetCompanionManager sets the companion manager for session tracking.
func (h *ChatHandler) SetCompanionManager(manager *companion.Manager) {
	h.companionManager = manager
}

// SetPromptGuard sets the prompt guard detector for security checks.
func (h *ChatHandler) SetPromptGuard(detector *promptguard.Detector) {
	h.promptGuard = detector
}

// SetProviderPool sets the provider pool for auto-selecting providers.
func (h *ChatHandler) SetProviderPool(pool *providerpool.Pool) {
	h.providerPool = pool
}

// SetCache sets the response cache for caching non-streaming responses.
func (h *ChatHandler) SetCache(cache *proxy.CCCache) {
	h.cache = cache
}

// getCompanionSessionID returns the companion session ID for a conversation.
func (h *ChatHandler) getCompanionSessionID(convID string) string {
	h.convMu.RLock()
	defer h.convMu.RUnlock()
	return h.convToSession[convID]
}

// GetProviderRegistry returns the provider registry.
func (h *ChatHandler) GetProviderRegistry() *llm.ProviderRegistry {
	return h.providers
}

// CreateConversationRequest represents a request to create a conversation.
type CreateConversationRequest struct {
	Title string `json:"title"`
}

// CreateConversation creates a new conversation.
func (h *ChatHandler) CreateConversation(c echo.Context) error {
	var req CreateConversationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Title == "" {
		req.Title = "New Conversation"
	}

	conv, err := h.store.CreateConversation(c.Request().Context(), req.Title)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create conversation")
	}

	// Create companion session for web chat
	if h.companionManager != nil {
		session := &companion.Session{
			Platform: companion.PlatformWeb,
			UserID:   "web-user",
			Metadata: companion.SessionMeta{
				ClientIP: c.RealIP(),
			},
		}
		created, err := h.companionManager.CreateSession(c.Request().Context(), session)
		if err == nil && created != nil {
			h.convMu.Lock()
			h.convToSession[conv.ID] = created.ID
			h.convMu.Unlock()
		}
	}

	return c.JSON(http.StatusCreated, conv)
}

// ListConversations lists all conversations or searches by query.
func (h *ChatHandler) ListConversations(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 50
	}

	query := c.QueryParam("q")
	var convs []memory.Conversation
	var err error

	if query != "" {
		// Search conversations by title
		convs, err = h.store.SearchConversations(c.Request().Context(), query, limit)
	} else {
		// List all conversations with pagination
		offset, _ := strconv.Atoi(c.QueryParam("offset"))
		convs, err = h.store.ListConversations(c.Request().Context(), limit, offset)
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list conversations")
	}

	if convs == nil {
		convs = []memory.Conversation{}
	}

	return c.JSON(http.StatusOK, convs)
}

// GetConversation retrieves a conversation by ID.
func (h *ChatHandler) GetConversation(c echo.Context) error {
	id := c.Param("id")

	conv, err := h.store.GetConversation(c.Request().Context(), id)
	if err != nil {
		if err == memory.ErrNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "conversation not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
	}

	return c.JSON(http.StatusOK, conv)
}

// DeleteConversation deletes a conversation.
func (h *ChatHandler) DeleteConversation(c echo.Context) error {
	id := c.Param("id")

	err := h.store.DeleteConversation(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete conversation")
	}

	return c.NoContent(http.StatusNoContent)
}

// GetMessages retrieves messages for a conversation.
func (h *ChatHandler) GetMessages(c echo.Context) error {
	id := c.Param("id")

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 100
	}

	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	messages, err := h.store.GetMessages(c.Request().Context(), id, limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get messages")
	}

	if messages == nil {
		messages = []memory.Message{}
	}

	return c.JSON(http.StatusOK, messages)
}

// MessageAttachment represents a file or image attachment.
type MessageAttachment struct {
	Type     string `json:"type"`      // "image" or "file"
	Name     string `json:"name"`      // filename
	MimeType string `json:"mime_type"` // MIME type
	Data     string `json:"data"`      // base64 encoded content
}

// SendMessageRequest represents a request to send a message.
type SendMessageRequest struct {
	Message     string              `json:"message"`
	Provider    string              `json:"provider"`
	Model       string              `json:"model"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Attachments []MessageAttachment `json:"attachments,omitempty"`
}

// SendMessageResponse represents a response from sending a message.
type SendMessageResponse struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SendMessage sends a message and gets a response from the LLM.
func (h *ChatHandler) SendMessage(c echo.Context) error {
	convID := c.Param("id")

	var req SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Allow empty message if attachments are provided
	if req.Message == "" && len(req.Attachments) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "message or attachments required")
	}

	// Check for prompt injection
	if h.promptGuard != nil {
		result := h.promptGuard.Detect(req.Message)
		if result.IsThreat {
			// Record security event to companion
			if h.companionManager != nil {
				sessionID := h.getCompanionSessionID(convID)
				if sessionID != "" {
					h.emitSecurityEvent(c.Request().Context(), sessionID, result)
				}
			}
			// Return error to user
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"success":      false,
				"blocked":      true,
				"message":      "Message blocked due to security policy",
				"threat_level": result.ThreatLevel.String(),
			})
		}
	}

	// Auto-select provider and model from pool
	provider, _, model, err := h.getDefaultProvider()
	if err != nil {
		// Check if this is a trial quota exhausted error
		if errors.Is(err, providerpool.ErrTrialQuotaExhausted) {
			return c.JSON(http.StatusPaymentRequired, map[string]interface{}{
				"success":           false,
				"trial_exhausted":   true,
				"message":           "trial_quota_exhausted",
				"message_localized": "Trial quota has been exhausted. Please configure your own AI provider to continue.",
			})
		}
		return echo.NewHTTPError(http.StatusServiceUnavailable, "no available providers: "+err.Error())
	}

	// Use model from request if specified
	if req.Model != "" {
		model = req.Model
	}

	// Store user message
	_, err = h.store.AddMessage(c.Request().Context(), convID, memory.Message{
		Role:    "user",
		Content: req.Message,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store message")
	}

	// Emit message event to companion
	if h.companionManager != nil {
		sessionID := h.getCompanionSessionID(convID)
		if sessionID != "" {
			h.emitMessageEvent(c.Request().Context(), sessionID, req.Message, "inbound")
		}
	}

	// Get conversation history
	messages, err := h.store.GetMessages(c.Request().Context(), convID, 50, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get messages")
	}

	// Convert to LLM messages
	llmMessages := make([]llm.Message, len(messages))
	for i, msg := range messages {
		llmMessages[i] = llm.Message{
			Role:    llm.Role(msg.Role),
			Content: msg.Content,
		}
	}

	// Build chat request
	chatReq := llm.ChatRequest{
		Model:       model,
		Messages:    llmMessages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	// Get tool definitions
	toolDefs := h.toolRegistry.Definitions()
	if len(toolDefs) > 0 {
		chatReq.Tools = make([]llm.Tool, len(toolDefs))
		for i, def := range toolDefs {
			chatReq.Tools[i] = llm.Tool{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			}
		}
	}

	// Call LLM and record metrics
	startTime := time.Now()
	resp, err := provider.Chat(c.Request().Context(), chatReq)
	latencyMs := float64(time.Since(startTime).Milliseconds())

	// Estimate tokens if API didn't return usage data
	if resp != nil && resp.Usage.TotalTokens == 0 {
		resp.Usage.PromptTokens = estimateInputTokens(llmMessages)
		resp.Usage.CompletionTokens = estimateTokens(resp.Message.Content)
		resp.Usage.TotalTokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
	}

	// Emit LLM request event to companion
	if h.companionManager != nil {
		sessionID := h.getCompanionSessionID(convID)
		if sessionID != "" {
			h.emitLLMRequestEvent(c.Request().Context(), sessionID, req.Provider, model, resp, err, time.Duration(latencyMs)*time.Millisecond)
		}
	}

	// Record metrics
	if h.metricsRecorder != nil {
		var inputTokens, outputTokens int64
		var errorType string
		success := err == nil

		if resp != nil {
			inputTokens = int64(resp.Usage.PromptTokens)
			outputTokens = int64(resp.Usage.CompletionTokens)
		}
		if err != nil {
			errorType = "api_error"
		}

		h.metricsRecorder.RecordAPICall(model, success, latencyMs, inputTokens, outputTokens, 0, 0, errorType)

		// Record trial usage if this is a trial provider
		if h.providerPool != nil && h.providerPool.TrialQuotaManager != nil && providerpool.IsTrialProvider(req.Provider) {
			h.providerPool.TrialQuotaManager.RecordUsage(inputTokens, outputTokens, convID)
		}
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get response from LLM")
	}

	// Get provider name for display
	providerName := req.Provider
	if h.providerPool != nil {
		if poolProvider, err := h.providerPool.Registry.Get(req.Provider); err == nil {
			providerName = poolProvider.Name
		}
	}

	// Store assistant message with stats
	assistantMsg, err := h.store.AddMessage(c.Request().Context(), convID, memory.Message{
		Role:     "assistant",
		Content:  resp.Message.Content,
		Provider: providerName,
		Model:    model,
		Stats: &memory.MessageStats{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
			LatencyMs:    int64(latencyMs),
		},
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store response")
	}

	return c.JSON(http.StatusOK, SendMessageResponse{
		ID:      assistantMsg.ID,
		Role:    "assistant",
		Content: resp.Message.Content,
	})
}

// ProviderInfo represents information about an LLM provider.
type ProviderInfo struct {
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

// ListProviders lists available LLM providers.
func (h *ChatHandler) ListProviders(c echo.Context) error {
	names := h.providers.List()
	providers := make([]ProviderInfo, 0, len(names))

	for _, name := range names {
		provider := h.providers.Get(name)
		providers = append(providers, ProviderInfo{
			Name:   name,
			Models: provider.Models(),
		})
	}

	return c.JSON(http.StatusOK, providers)
}

// RefreshProviderModels refreshes the model list for providers that support it.
func (h *ChatHandler) RefreshProviderModels(c echo.Context) error {
	providerName := c.Param("provider")

	provider := h.providers.Get(providerName)
	if provider == nil {
		return echo.NewHTTPError(http.StatusNotFound, "provider not found: "+providerName)
	}

	// Check if provider supports model refresh
	if refresher, ok := provider.(llm.ModelRefresher); ok {
		models := refresher.RefreshModels()
		return c.JSON(http.StatusOK, ProviderInfo{
			Name:   providerName,
			Models: models,
		})
	}

	// Provider doesn't support refresh, just return current models
	return c.JSON(http.StatusOK, ProviderInfo{
		Name:   providerName,
		Models: provider.Models(),
	})
}

// ListTools lists available tools.
func (h *ChatHandler) ListTools(c echo.Context) error {
	defs := h.toolRegistry.Definitions()
	return c.JSON(http.StatusOK, defs)
}

// DeleteMessagesRequest represents a request to delete messages.
type DeleteMessagesRequest struct {
	MessageIDs []string `json:"message_ids"`
}

// DeleteMessages deletes multiple messages from a conversation.
func (h *ChatHandler) DeleteMessages(c echo.Context) error {
	convID := c.Param("id")

	var req DeleteMessagesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if len(req.MessageIDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "message_ids is required")
	}

	if err := h.store.DeleteMessages(c.Request().Context(), convID, req.MessageIDs); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"deleted": len(req.MessageIDs),
	})
}

// RegisterChatRoutes registers chat-related routes.
func (h *ChatHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/conversations", h.CreateConversation)
	g.GET("/conversations", h.ListConversations)
	g.GET("/conversations/:id", h.GetConversation)
	g.DELETE("/conversations/:id", h.DeleteConversation)
	g.GET("/conversations/:id/messages", h.GetMessages)
	g.POST("/conversations/:id/messages", h.SendMessage)
	g.DELETE("/conversations/:id/messages", h.DeleteMessages)
	g.POST("/conversations/:id/messages/stream", h.StreamMessage)
	g.POST("/conversations/:id/messages/cancel", h.CancelStream)
	g.GET("/providers", h.ListProviders)
	g.POST("/providers/:provider/refresh", h.RefreshProviderModels)
	g.GET("/tools", h.ListTools)
	g.GET("/streams/active", h.ListActiveStreams)
	g.POST("/streams/cancel-all", h.CancelAllStreams)
}

// StreamMessage sends a message and streams the response.
func (h *ChatHandler) StreamMessage(c echo.Context) error {
	// Record cache bypass for streaming request
	if h.cache != nil {
		h.cache.RecordBypass()
	}

	convID := c.Param("id")

	var req SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Allow empty message if attachments are provided
	if req.Message == "" && len(req.Attachments) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "message or attachments required")
	}

	// Check for prompt injection
	if h.promptGuard != nil {
		result := h.promptGuard.Detect(req.Message)
		if result.IsThreat {
			// Record security event to companion
			if h.companionManager != nil {
				sessionID := h.getCompanionSessionID(convID)
				if sessionID != "" {
					h.emitSecurityEvent(c.Request().Context(), sessionID, result)
				}
			}
			// Return error to user
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"success":      false,
				"blocked":      true,
				"message":      "Message blocked due to security policy",
				"threat_level": result.ThreatLevel.String(),
			})
		}
	}

	// Auto-select provider and model from pool
	provider, providerID, model, err := h.getDefaultProvider()
	if err != nil {
		// Check if this is a trial quota exhausted error
		if errors.Is(err, providerpool.ErrTrialQuotaExhausted) {
			return c.JSON(http.StatusPaymentRequired, map[string]interface{}{
				"success":           false,
				"trial_exhausted":   true,
				"message":           "trial_quota_exhausted",
				"message_localized": "Trial quota has been exhausted. Please configure your own AI provider to continue.",
			})
		}
		return echo.NewHTTPError(http.StatusServiceUnavailable, "no available providers: "+err.Error())
	}

	// Use model from request if specified
	if req.Model != "" {
		model = req.Model
	}

	// Get provider name for display
	providerName := providerID
	if h.providerPool != nil {
		if poolProvider, err := h.providerPool.Registry.Get(providerID); err == nil {
			providerName = poolProvider.Name
		}
	}

	// Store user message with attachments
	var memoryAttachments []memory.MessageAttachment
	for _, att := range req.Attachments {
		memoryAttachments = append(memoryAttachments, memory.MessageAttachment{
			Type:     att.Type,
			Name:     att.Name,
			MimeType: att.MimeType,
			Data:     att.Data,
		})
	}
	_, err = h.store.AddMessage(c.Request().Context(), convID, memory.Message{
		Role:        "user",
		Content:     req.Message,
		Attachments: memoryAttachments,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store message: "+err.Error())
	}

	// Emit message event to companion
	if h.companionManager != nil {
		sessionID := h.getCompanionSessionID(convID)
		if sessionID != "" {
			h.emitMessageEvent(c.Request().Context(), sessionID, req.Message, "inbound")
		}
	}

	// Get conversation history
	messages, err := h.store.GetMessages(c.Request().Context(), convID, 50, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get messages: "+err.Error())
	}

	// Convert to LLM messages
	llmMessages := make([]llm.Message, len(messages))
	for i, msg := range messages {
		llmMessages[i] = llm.Message{
			Role:    llm.Role(msg.Role),
			Content: msg.Content,
		}
	}

	// Add attachments to the last user message (current request) as ContentParts
	if len(req.Attachments) > 0 && len(llmMessages) > 0 {
		lastIdx := len(llmMessages) - 1
		if llmMessages[lastIdx].Role == llm.RoleUser {
			// Build content parts: text first, then attachments
			contentParts := []llm.ContentPart{}
			if llmMessages[lastIdx].Content != "" {
				contentParts = append(contentParts, llm.ContentPart{
					Type: "text",
					Text: llmMessages[lastIdx].Content,
				})
			}
			for _, att := range req.Attachments {
				if att.Type == "image" {
					contentParts = append(contentParts, llm.ContentPart{
						Type:      "image",
						MediaType: att.MimeType,
						Data:      att.Data,
					})
				} else {
					// For files, add as text content with filename prefix
					contentParts = append(contentParts, llm.ContentPart{
						Type: "text",
						Text: fmt.Sprintf("\n\n[File: %s]\n%s", att.Name, decodeBase64Content(att.Data)),
					})
				}
			}
			llmMessages[lastIdx].ContentParts = contentParts
			llmMessages[lastIdx].Content = "" // Clear content when using ContentParts
		}
	}

	// Apply context compaction if needed
	compactedMessages, summary, _ := h.compactMessages(c.Request().Context(), llmMessages, provider)
	if summary != "" {
		// Prepend summary as system context
		summaryMsg := llm.Message{
			Role:    llm.RoleSystem,
			Content: "Previous conversation summary: " + summary,
		}
		compactedMessages = append([]llm.Message{summaryMsg}, compactedMessages...)
	}

	// Build chat request
	chatReq := llm.ChatRequest{
		Model:       model,
		Messages:    compactedMessages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	// Create cancellable context
	streamID := uuid.New().String()
	ctx, cancel := context.WithCancel(c.Request().Context())
	h.streamController.Register(streamID, cancel)
	defer h.streamController.Unregister(streamID)

	// Set SSE headers before starting stream
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Stream-ID", streamID)
	// Disable buffering for nginx and other proxies
	c.Response().Header().Set("X-Accel-Buffering", "no")
	// Disable compression which can cause buffering
	c.Response().Header().Set("Content-Encoding", "identity")

	// Get the underlying http.Flusher for immediate flush
	flusher, ok := c.Response().Writer.(http.Flusher)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}

	// Flush headers immediately
	c.Response().WriteHeader(http.StatusOK)
	flusher.Flush()

	// Track metrics
	startTime := time.Now()
	var fullContent string
	var totalInputTokens, totalOutputTokens int
	var firstChunkTime time.Time

	// Use callback-based streaming to avoid channel issues
	err = provider.ChatStreamCallback(ctx, chatReq, func(chunk llm.StreamChunk) error {
		// Track first chunk time for TTFT calculation
		if firstChunkTime.IsZero() && chunk.Delta != "" {
			firstChunkTime = time.Now()
		}

		// Check for error in chunk
		if chunk.Error != "" {
			// Record error metrics
			if h.metricsRecorder != nil {
				latencyMs := float64(time.Since(startTime).Milliseconds())
				h.metricsRecorder.RecordAPICall(model, false, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "stream_error")
			}
			// Send error to client
			data := map[string]interface{}{
				"error":     chunk.Error,
				"done":      true,
				"stream_id": streamID,
			}
			jsonData, _ := json.Marshal(data)
			c.Response().Write([]byte("data: " + string(jsonData) + "\n\n"))
			flusher.Flush()
			// Store error message
			h.store.AddMessage(context.Background(), convID, memory.Message{
				Role:    "assistant",
				Content: chunk.Error,
			})
			return fmt.Errorf("stream error: %s", chunk.Error)
		}

		fullContent += chunk.Delta

		// Track token usage from chunks
		if chunk.Usage != nil {
			totalInputTokens = chunk.Usage.PromptTokens
			totalOutputTokens = chunk.Usage.CompletionTokens
		}

		// Write SSE data - for non-final chunks only
		if !chunk.Done {
			data := map[string]interface{}{
				"delta":     chunk.Delta,
				"done":      false,
				"stream_id": streamID,
			}
			if chunk.Usage != nil {
				data["usage"] = chunk.Usage
			}

			jsonData, _ := json.Marshal(data)
			c.Response().Write([]byte("data: " + string(jsonData) + "\n\n"))
			flusher.Flush()
		}

		if chunk.Done {
			// Fallback: estimate tokens if provider didn't return usage
			if totalInputTokens == 0 {
				totalInputTokens = estimateInputTokens(compactedMessages)
			}
			if totalOutputTokens == 0 {
				totalOutputTokens = estimateTokens(fullContent)
			}

			// Record successful completion metrics
			latencyMs := float64(time.Since(startTime).Milliseconds())
			if h.metricsRecorder != nil {
				h.metricsRecorder.RecordAPICall(model, true, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "")
				// Record speed metrics
				if !firstChunkTime.IsZero() && totalOutputTokens > 0 {
					ttftMs := float64(firstChunkTime.Sub(startTime).Milliseconds())
					totalDuration := time.Since(startTime).Seconds()
					if totalDuration > 0 {
						tokensPerSecond := float64(totalOutputTokens) / totalDuration
						h.metricsRecorder.RecordSpeed(req.Model, tokensPerSecond, ttftMs, tokensPerSecond)
					}
				}

				// Record trial usage if this is a trial provider
				if h.providerPool != nil && h.providerPool.TrialQuotaManager != nil && providerpool.IsTrialProvider(providerID) {
					h.providerPool.TrialQuotaManager.RecordUsage(int64(totalInputTokens), int64(totalOutputTokens), convID)
				}
			}

			// Calculate speed metrics for response
			var tokensPerSecond float64
			var ttftMs float64
			if !firstChunkTime.IsZero() {
				ttftMs = float64(firstChunkTime.Sub(startTime).Milliseconds())
				totalDuration := time.Since(startTime).Seconds()
				if totalDuration > 0 && totalOutputTokens > 0 {
					tokensPerSecond = float64(totalOutputTokens) / totalDuration
				}
			}

			// Send final chunk with provider/model info and stats
			finalData := map[string]interface{}{
				"delta":     "",
				"done":      true,
				"stream_id": streamID,
				"provider":  providerName,
				"model":     model,
				"stats": map[string]interface{}{
					"input_tokens":      totalInputTokens,
					"output_tokens":     totalOutputTokens,
					"total_tokens":      totalInputTokens + totalOutputTokens,
					"latency_ms":        latencyMs,
					"ttft_ms":           ttftMs,
					"tokens_per_second": tokensPerSecond,
				},
			}
			if chunk.Usage != nil {
				finalData["usage"] = chunk.Usage
			}
			finalJSON, _ := json.Marshal(finalData)
			c.Response().Write([]byte("data: " + string(finalJSON) + "\n\n"))
			flusher.Flush()

			// Store the complete message with stats
			h.store.AddMessage(context.Background(), convID, memory.Message{
				Role:     "assistant",
				Content:  fullContent,
				Provider: providerName,
				Model:    model,
				Stats: &memory.MessageStats{
					InputTokens:     totalInputTokens,
					OutputTokens:    totalOutputTokens,
					TotalTokens:     totalInputTokens + totalOutputTokens,
					LatencyMs:       int64(latencyMs),
					TTFTMs:          int64(ttftMs),
					TokensPerSecond: tokensPerSecond,
				},
			})
			// Generate title for new conversations
			// Pass AI response to check for markdown heading as title
			go h.generateConversationTitle(convID, req.Message, fullContent, "en")

			// Emit LLM request event to companion for streaming
			if h.companionManager != nil {
				sessionID := h.getCompanionSessionID(convID)
				if sessionID != "" {
					llmResp := &llm.ChatResponse{
						Usage: llm.Usage{
							PromptTokens:     totalInputTokens,
							CompletionTokens: totalOutputTokens,
							TotalTokens:      totalInputTokens + totalOutputTokens,
						},
					}
					h.emitLLMRequestEvent(context.Background(), sessionID, providerName, model, llmResp, nil, time.Duration(latencyMs)*time.Millisecond)
				}
			}
		}

		return nil
	})

	// Handle stream completion or error
	if err != nil {
		if ctx.Err() != nil {
			// Stream was cancelled
			if h.metricsRecorder != nil {
				latencyMs := float64(time.Since(startTime).Milliseconds())
				h.metricsRecorder.RecordAPICall(model, false, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "cancelled")
			}
			if fullContent != "" {
				h.store.AddMessage(context.Background(), convID, memory.Message{
					Role:    "assistant",
					Content: fullContent + "\n\n[Response interrupted]",
				})
			}
			data := map[string]interface{}{
				"cancelled": true,
				"done":      true,
			}
			jsonData, _ := json.Marshal(data)
			c.Response().Write([]byte("data: " + string(jsonData) + "\n\n"))
			flusher.Flush()
			return nil
		}
		// Other error - already handled in callback
		return nil
	}

	// Send final DONE marker
	c.Response().Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
	return nil
}

// generateConversationTitle generates a title using LLM summarization.
// Falls back to truncating the user message if LLM is unavailable.
// If aiResponse starts with a markdown heading (#), use that as the title.
// targetLang specifies the language for the generated title (e.g., "en", "zh", "ja").
// This function is safe to call in a goroutine - it recovers from panics.
func (h *ChatHandler) generateConversationTitle(convID, userMessage, aiResponse, targetLang string) {
	// Recover from any panics to prevent crashing the server
	defer func() {
		if r := recover(); r != nil {
			// Log the panic but don't crash - title generation is not critical
			fmt.Printf("panic in generateConversationTitle: %v\n", r)
		}
	}()

	// Check if AI response starts with a markdown heading
	if title := extractMarkdownHeading(aiResponse); title != "" {
		h.store.UpdateConversationTitle(context.Background(), convID, title)
		return
	}

	// If the message is short enough (<=10 runes), use it directly as the title
	if len([]rune(userMessage)) <= 10 {
		title := sanitizeTitle(userMessage)
		h.store.UpdateConversationTitle(context.Background(), convID, title)
		return
	}

	// Try to use LLM to generate a concise title
	// TODO: Re-enable LLM title generation after fixing CC CLI issues
	// title := h.generateTitleWithLLM(userMessage, targetLang)
	// if title == "" {
	// Fallback: use truncated user message
	title := userMessage
	maxLen := 50
	runes := []rune(title)
	if len(runes) > maxLen {
		if idx := findRuneBoundary(title, maxLen); idx > 0 {
			title = title[:idx] + "..."
		} else {
			title = string(runes[:maxLen]) + "..."
		}
	}
	// }
	// Remove newlines and sanitize
	title = sanitizeTitle(title)

	// Update the conversation title
	h.store.UpdateConversationTitle(context.Background(), convID, title)
}

// generateTitleWithLLM uses an LLM to generate a concise conversation title.
// targetLang specifies the language for the generated title (e.g., "en", "zh", "ja").
// It iterates through available providers and uses the first suitable one.
func (h *ChatHandler) generateTitleWithLLM(userMessage, targetLang string) string {
	// Find a suitable provider for title generation
	// Prefer direct API providers over CLI-based ones for speed
	var provider llm.Provider

	// Try providers in order of preference
	providerNames := []string{"claude", "openai", "ollama"}
	for _, name := range providerNames {
		p := h.providers.Get(name)
		if p == nil {
			continue
		}

		// Use this provider
		provider = p
		break
	}

	if provider == nil {
		return ""
	}

	// Get available models
	models := provider.Models()
	if len(models) == 0 {
		return ""
	}

	// Truncate user message if too long (to save tokens)
	content := userMessage
	contentRunes := []rune(content)
	if len(contentRunes) > 500 {
		content = string(contentRunes[:500]) + "..."
	}

	// Build language instruction
	langInstruction := getLanguageInstruction(targetLang)

	// Create a simple prompt for title generation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := llm.ChatRequest{
		Model: models[0], // Use first available model
		Messages: []llm.Message{
			{
				Role:    llm.RoleSystem,
				Content: fmt.Sprintf("Generate a very short title (max 30 characters) for this conversation. Output ONLY the title, no quotes, no explanation. %s", langInstruction),
			},
			{
				Role:    llm.RoleUser,
				Content: content,
			},
		},
		Temperature: 0.3,
		MaxTokens:   50,
	}

	resp, err := provider.Chat(ctx, req)
	if err != nil {
		return ""
	}

	// Clean up the response
	title := sanitizeTitle(resp.Message.Content)
	// Ensure it's not too long
	titleRunes := []rune(title)
	if len(titleRunes) > 50 {
		if idx := findRuneBoundary(title, 50); idx > 0 {
			title = title[:idx]
		} else {
			title = string(titleRunes[:50])
		}
	}
	return title
}

// findRuneBoundary finds the last space before maxLen runes, returning byte position.
// For CJK text without spaces, returns -1 to use rune-based truncation.
func findRuneBoundary(s string, maxLen int) int {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return -1
	}
	// Search backwards from maxLen for a space
	for i := maxLen - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			// Return byte position of this space
			return len(string(runes[:i]))
		}
	}
	return -1
}

// extractMarkdownHeading extracts a title from the first line if it's a markdown heading.
// Returns empty string if the first line is not a heading.
func extractMarkdownHeading(content string) string {
	if content == "" {
		return ""
	}

	// Get the first line
	firstLine := content
	if idx := strings.Index(content, "\n"); idx != -1 {
		firstLine = content[:idx]
	}
	firstLine = strings.TrimSpace(firstLine)

	// Check if it starts with # (markdown heading)
	if !strings.HasPrefix(firstLine, "#") {
		return ""
	}

	// Remove leading # characters and spaces
	title := strings.TrimLeft(firstLine, "#")
	title = strings.TrimSpace(title)

	// Validate: title should not be empty and not too long
	if title == "" {
		return ""
	}

	// Truncate if too long (max 50 characters)
	titleRunes := []rune(title)
	if len(titleRunes) > 50 {
		title = string(titleRunes[:50]) + "..."
	}

	return sanitizeTitle(title)
}

// decodeBase64Content decodes base64 content to string for text files.
func decodeBase64Content(data string) string {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "[Unable to decode file content]"
	}
	return string(decoded)
}

// sanitizeTitle removes newlines and extra whitespace from a title.
func sanitizeTitle(s string) string {
	var result []rune
	lastWasSpace := false
	for _, r := range s {
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
		}
		if r == ' ' {
			if !lastWasSpace {
				result = append(result, r)
				lastWasSpace = true
			}
		} else {
			result = append(result, r)
			lastWasSpace = false
		}
	}
	return string(result)
}

// isDefaultTitle checks if the title is a default/placeholder title that should be auto-generated.
// Supports multiple languages.
func isDefaultTitle(title string) bool {
	defaultTitles := []string{
		"New Conversation",      // English
		"新对话",                   // Chinese Simplified
		"新對話",                   // Chinese Traditional
		"Nueva conversación",    // Spanish
		"Nouvelle conversation", // French
		"Neue Unterhaltung",     // German
		"新しい会話",                 // Japanese
		"새 대화",                  // Korean
	}
	for _, dt := range defaultTitles {
		if title == dt {
			return true
		}
	}
	return false
}

// parseAcceptLanguage parses the Accept-Language header and returns the primary language code.
// Returns "en" as default if parsing fails or header is empty.
func parseAcceptLanguage(header string) string {
	if header == "" {
		return "en"
	}

	// Parse the first language preference (e.g., "zh-CN,zh;q=0.9,en;q=0.8" -> "zh")
	parts := strings.Split(header, ",")
	if len(parts) == 0 {
		return "en"
	}

	// Get the first language and extract the primary code
	lang := strings.TrimSpace(parts[0])
	// Remove quality value if present (e.g., "en;q=0.8" -> "en")
	if idx := strings.Index(lang, ";"); idx > 0 {
		lang = lang[:idx]
	}
	// Extract primary language code (e.g., "zh-CN" -> "zh")
	if idx := strings.Index(lang, "-"); idx > 0 {
		lang = lang[:idx]
	}

	return strings.ToLower(lang)
}

// getLanguageInstruction returns the language instruction for the LLM prompt.
func getLanguageInstruction(langCode string) string {
	languageNames := map[string]string{
		"zh": "Chinese (中文)",
		"en": "English",
		"ja": "Japanese (日本語)",
		"ko": "Korean (한국어)",
		"es": "Spanish (Español)",
		"fr": "French (Français)",
		"de": "German (Deutsch)",
		"pt": "Portuguese (Português)",
		"ru": "Russian (Русский)",
		"ar": "Arabic (العربية)",
		"it": "Italian (Italiano)",
	}

	if name, ok := languageNames[langCode]; ok {
		return fmt.Sprintf("The title MUST be in %s.", name)
	}
	// Default to English if language not recognized
	return "The title should be in English."
}

// CancelStreamRequest represents a request to cancel a stream.
type CancelStreamRequest struct {
	StreamID string `json:"stream_id"`
}

// CancelStream cancels an active streaming response.
func (h *ChatHandler) CancelStream(c echo.Context) error {
	var req CancelStreamRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.StreamID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "stream_id is required")
	}

	if h.streamController.Cancel(req.StreamID) {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"success":   true,
			"stream_id": req.StreamID,
			"message":   "Stream cancelled successfully",
		})
	}

	return c.JSON(http.StatusNotFound, map[string]interface{}{
		"success":   false,
		"stream_id": req.StreamID,
		"message":   "Stream not found or already completed",
	})
}

// ListActiveStreams returns a list of active streaming sessions.
func (h *ChatHandler) ListActiveStreams(c echo.Context) error {
	sessions := h.streamController.ListActiveSessions()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"active_streams": sessions,
		"count":          len(sessions),
	})
}

// CancelAllStreams cancels all active streaming sessions.
func (h *ChatHandler) CancelAllStreams(c echo.Context) error {
	count := h.streamController.CancelAll()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":         true,
		"cancelled_count": count,
		"message":         "All streams cancelled",
	})
}

// compactMessages applies context compaction to messages if needed.
func (h *ChatHandler) compactMessages(ctx context.Context, messages []llm.Message, provider llm.Provider) ([]llm.Message, string, error) {
	compactor := claudecode.NewCompactor(h.compactionConfig, provider)
	return compactor.CompactMessages(ctx, messages)
}

// emitMessageEvent emits a message event to the companion system.
func (h *ChatHandler) emitMessageEvent(ctx context.Context, sessionID, content, direction string) {
	if h.companionManager == nil {
		return
	}
	event := &companion.SessionEvent{
		SessionID: sessionID,
		EventType: companion.EventMessageReceived,
		Platform:  companion.PlatformWeb,
		Message: &companion.MessageEvent{
			Direction: direction,
			Content:   content,
		},
	}
	_ = h.companionManager.EmitEvent(ctx, event)
}

// emitLLMRequestEvent emits an LLM request event to the companion system.
func (h *ChatHandler) emitLLMRequestEvent(ctx context.Context, sessionID, providerName, model string, resp *llm.ChatResponse, err error, duration time.Duration) {
	if h.companionManager == nil {
		return
	}

	llmEvent := &companion.LLMRequestEvent{
		Provider: providerName,
		Model:    model,
		Duration: companion.FromDuration(duration),
	}

	if resp != nil {
		llmEvent.PromptTokens = resp.Usage.PromptTokens
		llmEvent.CompletionTokens = resp.Usage.CompletionTokens
		llmEvent.TotalTokens = resp.Usage.TotalTokens
		llmEvent.Status = "success"
	}

	if err != nil {
		llmEvent.Status = "error"
		llmEvent.Error = err.Error()
	}

	event := &companion.SessionEvent{
		SessionID:  sessionID,
		EventType:  companion.EventLLMRequest,
		Platform:   companion.PlatformWeb,
		LLMRequest: llmEvent,
		Duration:   companion.FromDuration(duration),
	}
	_ = h.companionManager.EmitEvent(ctx, event)
}

// emitSecurityEvent emits a security threat event to the companion system.
func (h *ChatHandler) emitSecurityEvent(ctx context.Context, sessionID string, result *promptguard.DetectionResult) {
	if h.companionManager == nil {
		return
	}

	threatTypes := make([]string, 0, len(result.Detections))
	patterns := make([]string, 0, len(result.Detections))
	for _, d := range result.Detections {
		threatTypes = append(threatTypes, d.Type)
		patterns = append(patterns, d.Pattern)
	}

	event := &companion.SessionEvent{
		SessionID: sessionID,
		EventType: companion.EventSecurityThreat,
		Platform:  companion.PlatformWeb,
		Security: &companion.SecurityEvent{
			ThreatLevel:      mapPromptGuardThreatLevel(result.ThreatLevel),
			ThreatScore:      result.Score,
			ThreatTypes:      threatTypes,
			DetectedPatterns: patterns,
			Action:           "blocked",
			Source:           "prompt_guard",
		},
	}
	_ = h.companionManager.EmitEvent(ctx, event)
}

// mapPromptGuardThreatLevel maps promptguard.ThreatLevel to companion.ThreatLevel.
func mapPromptGuardThreatLevel(level promptguard.ThreatLevel) companion.ThreatLevel {
	switch level {
	case promptguard.ThreatLow:
		return companion.ThreatLevelLow
	case promptguard.ThreatMedium:
		return companion.ThreatLevelMedium
	case promptguard.ThreatHigh:
		return companion.ThreatLevelHigh
	case promptguard.ThreatCritical:
		return companion.ThreatLevelCritical
	default:
		return companion.ThreatLevelNone
	}
}
