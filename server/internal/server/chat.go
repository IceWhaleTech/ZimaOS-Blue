package server

import (
	"bytes"
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

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/promptguard"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// messageSlicePool is a sync.Pool for reusing message slices to reduce GC pressure.
var messageSlicePool = sync.Pool{
	New: func() interface{} {
		// Pre-allocate for typical conversation size
		slice := make([]llm.Message, 0, 64)
		return &slice
	},
}

// getMessageSlice gets a message slice from the pool.
func getMessageSlice() *[]llm.Message {
	return messageSlicePool.Get().(*[]llm.Message)
}

// putMessageSlice returns a message slice to the pool.
func putMessageSlice(slice *[]llm.Message) {
	*slice = (*slice)[:0] // Reset length but keep capacity
	messageSlicePool.Put(slice)
}

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
// Provider instances are created fresh each time to ensure API key changes take effect immediately.
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

	logger.Debug().Str("provider_id", providerID).Str("api_format", string(poolProvider.APIFormat)).Str("base_url", poolProvider.BaseURL).Bool("has_key", apiKey != "").Msg("[chat] getProviderFromPool")

	// Create LLM provider based on API format
	var provider llm.Provider
	switch poolProvider.APIFormat {
	case providerpool.APIFormatAnthropic:
		provider = llm.NewClaudeProvider(apiKey, poolProvider.BaseURL)
	case providerpool.APIFormatOllama:
		provider = llm.NewOllamaProvider(poolProvider.BaseURL)
	default:
		// Default to OpenAI-compatible format (covers OpenAI, Google, and custom providers)
		provider = llm.NewCustomProvider(apiKey, poolProvider.BaseURL)
	}

	return provider, nil
}

// tryProviderWithKeyFallback tries a provider with key fallback on auth errors (stream)
func (h *ChatHandler) tryProviderWithKeyFallback(ctx context.Context, providerID string, chatReq llm.ChatRequest, streamCb func(chunk llm.StreamChunk) error) error {
	if h.providerPool == nil {
		provider, err := h.getProviderFromPool(providerID)
		if err != nil {
			return err
		}
		return provider.ChatStreamCallback(ctx, chatReq, streamCb)
	}

	poolProvider, err := h.providerPool.Registry.Get(providerID)
	if err != nil {
		return err
	}

	// Try each enabled API key
	var lastErr error
	for _, key := range poolProvider.APIKeys {
		if !key.Enabled || key.Key == "" {
			continue
		}

		var provider llm.Provider
		switch poolProvider.APIFormat {
		case providerpool.APIFormatAnthropic:
			provider = llm.NewClaudeProvider(key.Key, poolProvider.BaseURL)
		case providerpool.APIFormatOllama:
			provider = llm.NewOllamaProvider(poolProvider.BaseURL)
		default:
			provider = llm.NewCustomProvider(key.Key, poolProvider.BaseURL)
		}

		err := provider.ChatStreamCallback(ctx, chatReq, streamCb)
		if err == nil {
			return nil
		}

		logger.Debug().Str("provider_id", providerID).Str("key_id", key.ID).Err(err).Msg("[chat] key fallback attempt failed")
		lastErr = err

		// If it's not an auth error, don't try other keys
		if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "403") && !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "unauthorized") {
			return err
		}
	}

	return lastErr
}

// tryProviderChatWithKeyFallback tries Chat (non-streaming) with key fallback
func (h *ChatHandler) tryProviderChatWithKeyFallback(ctx context.Context, providerID string, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if h.providerPool == nil {
		provider, err := h.getProviderFromPool(providerID)
		if err != nil {
			return nil, err
		}
		return provider.Chat(ctx, req)
	}

	poolProvider, err := h.providerPool.Registry.Get(providerID)
	if err != nil {
		return nil, err
	}

	// Try each enabled API key
	var lastErr error
	for _, key := range poolProvider.APIKeys {
		if !key.Enabled || key.Key == "" {
			continue
		}

		var provider llm.Provider
		switch poolProvider.APIFormat {
		case providerpool.APIFormatAnthropic:
			provider = llm.NewClaudeProvider(key.Key, poolProvider.BaseURL)
		case providerpool.APIFormatOllama:
			provider = llm.NewOllamaProvider(poolProvider.BaseURL)
		default:
			provider = llm.NewCustomProvider(key.Key, poolProvider.BaseURL)
		}

		resp, err := provider.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}

		logger.Debug().Str("provider_id", providerID).Str("key_id", key.ID).Err(err).Msg("[chat] key fallback attempt failed")
		lastErr = err

		// If it's not an auth error, don't try other keys
		if !strings.Contains(err.Error(), "401") && !strings.Contains(err.Error(), "403") && !strings.Contains(err.Error(), "invalid") && !strings.Contains(err.Error(), "unauthorized") {
			return nil, err
		}
	}

	return nil, lastErr
}

// getDefaultProvider returns an available provider from the pool.
// It checks if any provider is available; the actual routing is handled by the proxy.
// Returns error if no providers are configured, prompting user to configure one.
func (h *ChatHandler) getDefaultProvider() (llm.Provider, string, string, error) {
	// Priority 1: Try provider pool
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
								defaultModel := ""
								if len(nextProvider.AllowedModels) > 0 {
									defaultModel = nextProvider.AllowedModels[0]
								} else if h.providerPool.Discovery != nil {
									if models, err := h.providerPool.Discovery.GetModels(nextProvider.ID); err == nil && len(models) > 0 {
										defaultModel = models[0].ID
									}
								}
								if defaultModel == "" {
									defaultModel = "claude-opus-4-6"
								}
								return provider, nextProvider.ID, defaultModel, nil
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
				// Get default model from provider's allowed models list
				defaultModel := ""
				if len(poolProvider.AllowedModels) > 0 {
					defaultModel = poolProvider.AllowedModels[0]
				} else if h.providerPool.Discovery != nil {
					// Try to fetch models from discovery if not configured
					if models, err := h.providerPool.Discovery.GetModels(poolProvider.ID); err == nil && len(models) > 0 {
						defaultModel = models[0].ID
					}
				}
				// Fallback to common default model if still empty
				if defaultModel == "" {
					defaultModel = "claude-opus-4-6"
				}
				fmt.Printf("[getDefaultProvider] using provider=%s, defaultModel=%s\n", poolProvider.ID, defaultModel)
				return provider, poolProvider.ID, defaultModel, nil
			}
		}
	}

	// Priority 3: Fallback to legacy provider registry
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
	toolExecutor      *tools.Executor
	streamController  *claudecode.StreamController
	compactionConfig  claudecode.CompactionConfig
	claudeCodeHandler *claudecode.Handler
	metricsRecorder   MetricsRecorder
	companionManager  *companion.Manager
	promptGuard       *promptguard.Detector
	cache             *proxy.CCCache
	convToSession     map[string]string
	convMu            sync.RWMutex

	// System prompt builder for channel messages
	systemPromptBuilder *claudecode.SystemPromptBuilder

	// STT service for audio transcription
	sttService stt.Service

	// Memory service for auto-extraction after conversations
	layeredMemory *memory.LayeredMemoryService

	// Performance optimization: async event queue
	eventQueue chan func()
	eventStop  chan struct{}

	// Performance optimization: Conversation message cache
	conversationCache *ConversationCache

	// Performance optimization: Object pools
	requestPool  *RequestPool
	responsePool *ResponsePool

	// Performance optimization: Concurrency optimizer
	concurrencyOpt *ConcurrencyOptimizer
	fastPathCache  *FastPathCache

	// Proxy bridge: routes LLM calls through proxy pipeline (cache/pruner/routing)
	proxyBridge *proxybridge.Bridge
}

// MetricsRecorder is an interface for recording API call metrics.
type MetricsRecorder interface {
	RecordAPICall(model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string)
	RecordAPICallForUser(userID, model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string)
	RecordSpeed(model string, tokensPerSecond, ttftMs, decodeSpeed float64)
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(store *memory.Store, providers *llm.ProviderRegistry, toolRegistry *tools.Registry) *ChatHandler {
	h := &ChatHandler{
		store:            store,
		providers:        providers,
		toolRegistry:     toolRegistry,
		toolExecutor:     tools.NewExecutor(toolRegistry),
		streamController: claudecode.NewStreamController(),
		compactionConfig: claudecode.DefaultCompactionConfig(),
		convToSession:    make(map[string]string),
		eventQueue:       make(chan func(), 100), // Buffered channel for async events
		eventStop:        make(chan struct{}),
		conversationCache: NewConversationCache(5*time.Minute, 100), // 5min TTL, max 100 conversations
	}
	// Start async event processor
	go h.processEventQueue()
	return h
}

// processEventQueue processes events asynchronously to avoid blocking request handlers.
func (h *ChatHandler) processEventQueue() {
	for {
		select {
		case <-h.eventStop:
			return
		case fn := <-h.eventQueue:
			if fn != nil {
				fn()
			}
		}
	}
}

// Close stops the async event processor.
func (h *ChatHandler) Close() {
	close(h.eventStop)
}

// Shutdown cancels all active SSE streams and stops the event processor.
// Call this before httpServer.Shutdown() so long-lived connections close promptly.
func (h *ChatHandler) Shutdown() {
	h.streamController.CancelAll()
	h.Close()
}

// queueEvent queues an event for async processing. Falls back to sync if queue is full.
func (h *ChatHandler) queueEvent(fn func()) {
	select {
	case h.eventQueue <- fn:
		// Queued successfully
	default:
		// Queue full, run synchronously
		fn()
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

// SetSystemPromptBuilder sets the system prompt builder for channel messages.
func (h *ChatHandler) SetSystemPromptBuilder(builder *claudecode.SystemPromptBuilder) {
	h.systemPromptBuilder = builder
}

// SetSTTService sets the STT service for audio transcription.
func (h *ChatHandler) SetSTTService(service stt.Service) {
	h.sttService = service
}

// SetLayeredMemory sets the layered memory service for auto-extraction.
func (h *ChatHandler) SetLayeredMemory(svc *memory.LayeredMemoryService) {
	h.layeredMemory = svc
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

// SetProxyBridge sets the proxy bridge for routing LLM calls through the proxy pipeline.
func (h *ChatHandler) SetProxyBridge(bridge *proxybridge.Bridge) {
	h.proxyBridge = bridge
}

// getCompanionSessionID returns the companion session ID for a conversation.
func (h *ChatHandler) getCompanionSessionID(convID string) string {
	h.convMu.RLock()
	defer h.convMu.RUnlock()
	return h.convToSession[convID]
}

// getUserID returns the user ID from the request context, or empty string if not authenticated.
func (h *ChatHandler) getUserID(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		return claims.UserID
	}
	return ""
}

// checkConversationOwnership verifies the caller owns the conversation (or is admin).
// Returns the conversation if authorized, or an HTTP error.
func (h *ChatHandler) checkConversationOwnership(c echo.Context, id string) (*memory.Conversation, error) {
	conv, err := h.store.GetConversation(c.Request().Context(), id)
	if err != nil {
		if err == memory.ErrNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "conversation not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
	}

	claims := auth.GetUserFromContext(c)
	// Allow if: admin, owner, or conversation has no owner (legacy data)
	if claims != nil && claims.Role != "admin" && conv.UserID != "" && conv.UserID != claims.UserID {
		return nil, echo.NewHTTPError(http.StatusNotFound, "conversation not found")
	}

	return conv, nil
}

// GetProviderRegistry returns the provider registry.
func (h *ChatHandler) GetProviderRegistry() *llm.ProviderRegistry {
	return h.providers
}

// isTextFile checks if a file is a text-based file that can be read as plain text.
func isTextFile(filename, mimeType string) bool {
	// Check MIME type first
	textMimeTypes := []string{
		"text/", "application/json", "application/xml", "application/javascript",
		"application/x-yaml", "application/yaml", "application/toml",
	}
	for _, t := range textMimeTypes {
		if strings.HasPrefix(mimeType, t) || mimeType == t {
			return true
		}
	}

	// Check file extension
	textExtensions := []string{
		".txt", ".md", ".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".cfg",
		".log", ".csv", ".html", ".htm", ".css", ".js", ".ts", ".jsx", ".tsx",
		".py", ".go", ".java", ".c", ".cpp", ".h", ".hpp", ".rs", ".rb", ".php",
		".sh", ".bash", ".zsh", ".sql", ".graphql", ".vue", ".svelte",
	}
	lowerName := strings.ToLower(filename)
	for _, ext := range textExtensions {
		if strings.HasSuffix(lowerName, ext) {
			return true
		}
	}
	return false
}

// channelConversationID builds a stable conversation ID for IM channels.
func channelConversationID(channelName, chatID string) string {
	if chatID != "" {
		return "ch:" + channelName + ":" + chatID
	}
	return "ch:" + channelName
}

// ProcessChannelMessage processes a message from a channel (e.g., Feishu) and returns AI response.
func (h *ChatHandler) ProcessChannelMessage(ctx context.Context, msg channel.Message) (string, error) {
	logger.Info().
		Str("channel", msg.ChannelName).
		Str("user_id", msg.UserID).
		Str("content", msg.Content).
		Bool("has_provider_pool", h.providerPool != nil).
		Bool("has_claude_code", h.claudeCodeHandler != nil).
		Msg("ProcessChannelMessage called")

	// Extract language from message metadata
	lang := i18n.DefaultLanguage
	if msg.Metadata != nil {
		if langStr, ok := msg.Metadata["language"].(string); ok {
			lang = i18n.ParseLanguage(langStr)
		}
	}

	// Validate input - allow empty content if there are attachments
	if msg.Content == "" && len(msg.Attachments) == 0 {
		logger.Warn().Str("channel", msg.ChannelName).Msg("empty message content")
		return "", fmt.Errorf("empty message content")
	}

	// Build a stable conversation ID from channel + chat so we can persist history
	convID := channelConversationID(msg.ChannelName, msg.ChatID)

	// Ensure conversation exists in store (create if first message)
	if h.store != nil {
		if _, err := h.store.GetConversation(ctx, convID); err != nil {
			title := fmt.Sprintf("%s chat", msg.ChannelName)
			if msg.Username != "" {
				title = fmt.Sprintf("%s - %s", msg.ChannelName, msg.Username)
			}
			_, _ = h.store.CreateConversationWithID(ctx, convID, title)
		}
	}

	// Persist user message
	if h.store != nil {
		_, err := h.store.AddMessage(ctx, convID, memory.Message{
			Role:    "user",
			Content: msg.Content,
		})
		if err != nil {
			logger.Warn().Err(err).Str("conv_id", convID).Msg("failed to persist IM user message")
		}
	}

	// Use provider pool with system prompt
	var messages []llm.Message

	// Add system prompt if available
	if h.systemPromptBuilder != nil {
		systemPrompt := h.systemPromptBuilder.Build(ctx, "")
		if systemPrompt != "" {
			messages = append(messages, llm.Message{
				Role:    "system",
				Content: systemPrompt,
			})
		}
	}

	// Load conversation history for context continuity
	if h.store != nil {
		history, err := h.store.GetMessages(ctx, convID, 30, 0)
		if err == nil && len(history) > 0 {
			for _, m := range history {
				messages = append(messages, llm.Message{
					Role:    llm.Role(m.Role),
					Content: m.Content,
				})
			}
			logger.Debug().
				Str("conv_id", convID).
				Int("history_count", len(history)).
				Msg("loaded IM conversation history")
		}
	}

	// Build multimodal user message if attachments present (replaces text-only version from history)
	var userMsg *llm.Message

	// Convert channel attachments to LLM content parts for multimodal support
	if len(msg.Attachments) > 0 {
		userMsgBase := llm.Message{
			Role:    "user",
			Content: msg.Content,
		}
		var contentParts []llm.ContentPart
		var transcribedTexts []string

		// Add text content if present
		if msg.Content != "" {
			contentParts = append(contentParts, llm.ContentPart{
				Type: "text",
				Text: msg.Content,
			})
		}

		// Process attachments
		for _, att := range msg.Attachments {
			switch att.Type {
			case channel.MessageTypeImage:
				if len(att.Data) > 0 {
					mediaType := att.MimeType
					if mediaType == "" {
						mediaType = "image/png"
					}
					encoded := base64.StdEncoding.EncodeToString(att.Data)
					contentParts = append(contentParts, llm.ContentPart{
						Type:      "image",
						MediaType: mediaType,
						Data:      encoded,
					})
					logger.Info().
						Str("channel", msg.ChannelName).
						Str("attachment_id", att.ID).
						Int64("size", att.Size).
						Msg("Added image attachment to LLM message")
				}

			case channel.MessageTypeAudio:
				// Try to transcribe audio using STT service
				if h.sttService != nil && len(att.Data) > 0 {
					format := stt.FormatOGG // Default for opus
					if strings.Contains(att.MimeType, "wav") {
						format = stt.FormatWAV
					} else if strings.Contains(att.MimeType, "mp3") {
						format = stt.FormatMP3
					}

					resp, err := h.sttService.Transcribe(ctx, &stt.TranscribeRequest{
						Audio:  bytes.NewReader(att.Data),
						Format: format,
					})
					if err != nil {
						logger.Warn().Err(err).Str("attachment_id", att.ID).Msg("Failed to transcribe audio")
						transcribedTexts = append(transcribedTexts, "[语音消息，转写失败]")
					} else if resp.Text != "" {
						transcribedTexts = append(transcribedTexts, fmt.Sprintf("[语音消息]: %s", resp.Text))
						logger.Info().
							Str("channel", msg.ChannelName).
							Str("transcribed", resp.Text).
							Msg("Transcribed audio attachment")
					}
				} else {
					transcribedTexts = append(transcribedTexts, "[语音消息，暂不支持转写]")
				}

			case channel.MessageTypeFile:
				// Try to extract text from text-based files
				if len(att.Data) > 0 && isTextFile(att.Name, att.MimeType) {
					textContent := string(att.Data)
					// Limit text content to avoid token overflow
					if len(textContent) > 10000 {
						textContent = textContent[:10000] + "\n...[内容过长，已截断]"
					}
					transcribedTexts = append(transcribedTexts, fmt.Sprintf("[文件: %s]\n%s", att.Name, textContent))
					logger.Info().
						Str("channel", msg.ChannelName).
						Str("filename", att.Name).
						Int("size", len(att.Data)).
						Msg("Extracted text from file attachment")
				} else {
					transcribedTexts = append(transcribedTexts, fmt.Sprintf("[文件: %s，暂不支持处理]", att.Name))
				}
			}
		}

		// Add transcribed texts as text content
		if len(transcribedTexts) > 0 {
			contentParts = append(contentParts, llm.ContentPart{
				Type: "text",
				Text: strings.Join(transcribedTexts, "\n"),
			})
		}

		if len(contentParts) > 0 {
			userMsgBase.ContentParts = contentParts
			userMsgBase.Content = "" // Clear content when using content parts
			// Attachments (images etc.) are not in history, so append as extra message
			userMsg = &userMsgBase
		}
	}

	// If history was loaded, the current user message is already the last entry.
	// Only append explicitly if we have multimodal content (attachments) that
	// replaces the text-only version, or if there was no history.
	if userMsg != nil {
		// Replace the last message (text-only from history) with multimodal version
		if len(messages) > 0 && messages[len(messages)-1].Role == "user" {
			messages[len(messages)-1] = *userMsg
		} else {
			messages = append(messages, *userMsg)
		}
	}

	// Apply context compaction if conversation is getting long
	if len(messages) > 10 {
		if compactProvider, _, _, compactErr := h.getDefaultProvider(); compactErr == nil {
			compacted, summary, _ := h.compactMessages(ctx, messages, compactProvider)
			if summary != "" {
				summaryMsg := llm.Message{
					Role:    llm.RoleSystem,
					Content: "Previous conversation summary: " + summary,
				}
				compacted = append([]llm.Message{summaryMsg}, compacted...)
			}
			messages = compacted
			logger.Debug().
				Int("before", len(messages)).
				Int("after", len(compacted)).
				Bool("has_summary", summary != "").
				Msg("compacted IM conversation context")
		}
	}

	// Recall relevant memories for IM context
	if memoryCtx := h.recallMemories(ctx, msg.Content); memoryCtx != "" {
		messages = append([]llm.Message{{Role: llm.RoleSystem, Content: memoryCtx}}, messages...)
	}

	var responseContent string

	// Use provider pool router with failover if available
	if h.providerPool != nil && h.providerPool.Router != nil {
		// Set up failover callback for logging
		h.providerPool.Router.SetFailoverCallback(func(result *providerpool.FailoverResult) {
			if len(result.FailedAttempts) > 0 {
				logger.Warn().
					Str("request_id", result.RequestID).
					Int("total_attempts", result.TotalAttempts).
					Int("failed_count", len(result.FailedAttempts)).
					Str("success_provider", result.SuccessProvider).
					Str("final_error", result.FinalError).
					Dur("duration", result.EndTime.Sub(result.StartTime)).
					Msg("channel message routing with failover")

				for i, attempt := range result.FailedAttempts {
					logger.Debug().
						Str("request_id", result.RequestID).
						Int("attempt", i+1).
						Str("provider_id", attempt.ProviderID).
						Str("reason", string(attempt.Reason)).
						Str("error", attempt.Error).
						Dur("latency", attempt.Latency).
						Msg("failover attempt details")
				}
			}
		})

		// Check available models
		availableModels := h.providerPool.Router.ListAvailableModels()
		if len(availableModels) == 0 {
			logger.Error().Msg("no models available in provider pool")
			return "", fmt.Errorf("no AI models available, please configure a provider")
		}

		modelID := availableModels[0].ID
		logger.Debug().
			Str("model_id", modelID).
			Int("available_models", len(availableModels)).
			Msg("selected model for channel message")

		routeReq := &providerpool.RouteRequest{
			ModelID:  modelID,
			Strategy: providerpool.RoutingStrategyPriority,
		}

		err := h.providerPool.Router.RouteWithFallback(ctx, routeReq, func(result *providerpool.RouteResult) error {
			logger.Debug().
				Str("provider_id", result.Provider.ID).
				Str("provider_name", result.Provider.Name).
				Str("model_id", result.Model.ID).
				Str("api_format", string(result.Provider.APIFormat)).
				Msg("routing to provider")

			// Get the LLM provider from pool (handles custom provider IDs like prov_xxx)
			provider, err := h.getProviderFromPool(result.Provider.ID)
			if err != nil {
				logger.Debug().
					Str("provider_id", result.Provider.ID).
					Str("api_format", string(result.Provider.APIFormat)).
					Err(err).
					Msg("getProviderFromPool failed, trying legacy registry")

				// Fallback to legacy registry lookup using mapped ID
				mappedID := mapProviderID(result.Provider.ID)
				provider = h.providers.Get(mappedID)
				if provider == nil {
					// Last resort: try using the provider's API format to create a new provider
					apiKey := ""
					if result.APIKey != nil {
						apiKey = result.APIKey.Key
					}
					switch result.Provider.APIFormat {
					case providerpool.APIFormatAnthropic:
						provider = llm.NewClaudeProvider(apiKey, result.Provider.BaseURL)
					case providerpool.APIFormatOllama:
						provider = llm.NewOllamaProvider(result.Provider.BaseURL)
					default:
						provider = llm.NewCustomProvider(apiKey, result.Provider.BaseURL)
					}
					logger.Debug().
						Str("provider_id", result.Provider.ID).
						Str("api_format", string(result.Provider.APIFormat)).
						Msg("created provider from RouteResult")
				}
			}

			if provider == nil {
				return fmt.Errorf("failed to get provider for %s", result.Provider.ID)
			}

			req := llm.ChatRequest{
				Model:    result.Model.ID,
				Messages: messages,
			}

			// Add tool definitions
			imToolDefs := h.toolRegistry.Definitions()
			if len(imToolDefs) > 0 {
				req.Tools = make([]llm.Tool, len(imToolDefs))
				for i, def := range imToolDefs {
					req.Tools[i] = llm.Tool{
						Name:        def.Name,
						Description: def.Description,
						Parameters:  def.Parameters,
					}
				}
			}

			logger.Debug().
				Str("model", req.Model).
				Int("messages", len(req.Messages)).
				Msg("sending chat request to LLM")

			// Tool execution loop for IM
			var resp *llm.ChatResponse
			for imRound := 0; imRound < maxToolRounds; imRound++ {
				if h.proxyBridge != nil {
					resp, err = h.proxyBridge.Chat(ctx, req)
					if err != nil {
						logger.Warn().Err(err).Msg("[chat] proxyBridge failed, falling back to direct provider with key fallback")
						resp, err = h.tryProviderChatWithKeyFallback(ctx, result.Provider.ID, req)
					}
				} else {
					resp, err = h.tryProviderChatWithKeyFallback(ctx, result.Provider.ID, req)
				}
				if err != nil {
					logger.Error().
						Err(err).
						Str("provider_id", result.Provider.ID).
						Str("model", req.Model).
						Msg("LLM chat request failed")
					return err
				}
				if len(resp.Message.ToolCalls) == 0 {
					break
				}
				logger.Info().Int("round", imRound).Int("tool_calls", len(resp.Message.ToolCalls)).Msg("[im] executing tool calls")
				toolResults := h.executeToolCalls(ctx, resp.Message.ToolCalls)
				req.Messages = append(req.Messages, resp.Message)
				req.Messages = append(req.Messages, toolResults...)
			}

			if resp.Message.Content == "" {
				logger.Warn().
					Str("provider_id", result.Provider.ID).
					Str("model", req.Model).
					Msg("LLM returned empty response")
				return fmt.Errorf("LLM returned empty response")
			}

			responseContent = resp.Message.Content
			logger.Debug().
				Str("provider_id", result.Provider.ID).
				Int("response_len", len(responseContent)).
				Msg("LLM chat request succeeded")
			return nil
		})

		if err != nil {
			// Log cooldown info and provide detailed error message
			cooldowns := h.providerPool.Router.ListCooldowns()
			if len(cooldowns) > 0 {
				logger.Warn().Int("providers_in_cooldown", len(cooldowns)).Msg("providers in cooldown")
			}
			logger.Error().Err(err).Msg("LLM chat failed with all providers")

			// Provide user-friendly error message based on error type
			if errors.Is(err, providerpool.ErrNoAvailableProvider) {
				if len(cooldowns) > 0 {
					return "", fmt.Errorf("%s", i18n.T(lang, i18n.MsgProvidersInCooldown, len(cooldowns)))
				}
				return "", fmt.Errorf("%s", i18n.T(lang, i18n.MsgNoProviderAvailable))
			}
			return "", fmt.Errorf("%s", i18n.T(lang, i18n.MsgServiceUnavailable))
		}

		if responseContent == "" {
			logger.Error().Msg("LLM returned empty response after successful routing")
			return "", fmt.Errorf("AI returned empty response")
		}

		h.persistChannelResponse(ctx, convID, responseContent)
		return responseContent, nil
	}

	// Fallback: use getDefaultProvider without failover
	logger.Debug().Msg("using fallback provider (no provider pool)")
	fbProvider, providerName, model, err := h.getDefaultProvider()
	if err != nil {
		logger.Error().Err(err).Msg("failed to get default provider")
		return "", fmt.Errorf("no AI provider available: %w", err)
	}

	// If no model from provider, try to get from provider's default
	if model == "" {
		// Try to get first available model from provider
		if h.providerPool != nil {
			if models := h.providerPool.Router.ListAvailableModels(); len(models) > 0 {
				model = models[0].ID
			}
		}
		if model == "" {
			logger.Error().Str("provider", providerName).Msg("no model available")
			return "", fmt.Errorf("no model available for provider %s", providerName)
		}
	}

	req := llm.ChatRequest{
		Model:    model,
		Messages: messages,
	}

	// Add tool definitions to fallback path
	fbToolDefs := h.toolRegistry.Definitions()
	if len(fbToolDefs) > 0 {
		req.Tools = make([]llm.Tool, len(fbToolDefs))
		for i, def := range fbToolDefs {
			req.Tools[i] = llm.Tool{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			}
		}
	}

	logger.Debug().
		Str("provider", providerName).
		Str("model", model).
		Msg("sending chat request via fallback provider")

	// Tool execution loop for fallback path
	var resp *llm.ChatResponse
	for fbRound := 0; fbRound < maxToolRounds; fbRound++ {
		resp, err = fbProvider.Chat(ctx, req)
		if err != nil {
			logger.Error().Err(err).Str("provider", providerName).Msg("fallback LLM chat failed")
			return "", fmt.Errorf("AI chat failed (%s): %w", providerName, err)
		}
		if len(resp.Message.ToolCalls) == 0 {
			break
		}
		logger.Info().Int("round", fbRound).Int("tool_calls", len(resp.Message.ToolCalls)).Msg("[im-fallback] executing tool calls")
		toolResults := h.executeToolCalls(ctx, resp.Message.ToolCalls)
		req.Messages = append(req.Messages, resp.Message)
		req.Messages = append(req.Messages, toolResults...)
	}

	if resp.Message.Content == "" {
		logger.Warn().Str("provider", providerName).Msg("fallback LLM returned empty response")
		return "", fmt.Errorf("AI returned empty response")
	}

	h.persistChannelResponse(ctx, convID, resp.Message.Content)
	return resp.Message.Content, nil
}

// persistChannelResponse saves the assistant response and invalidates cache for IM conversations.
func (h *ChatHandler) persistChannelResponse(ctx context.Context, convID, content string) {
	if h.store == nil {
		return
	}
	_, err := h.store.AddMessage(ctx, convID, memory.Message{
		Role:    "assistant",
		Content: content,
	})
	if err != nil {
		logger.Warn().Err(err).Str("conv_id", convID).Msg("failed to persist IM assistant message")
	}
	h.conversationCache.Invalidate(convID)

	// Async memory extraction from IM conversations
	if h.layeredMemory != nil {
		h.queueEvent(func() {
			h.extractMemory(convID, "im")
		})
	}
}

// recallMemories searches for relevant memories and returns a system message to prepend.
// Returns empty string if no relevant memories found.
func (h *ChatHandler) recallMemories(ctx context.Context, userMessage string) string {
	if h.layeredMemory == nil || userMessage == "" {
		return ""
	}
	results, err := h.layeredMemory.Recall(ctx, userMessage, 5)
	if err != nil || len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Relevant memories about the user:\n")
	for _, r := range results {
		if r.CombinedScore < 0.3 || r.Chunk.Content == "" {
			continue
		}
		sb.WriteString("- ")
		sb.WriteString(r.Chunk.Content)
		sb.WriteString("\n")
	}
	if sb.Len() < 40 { // Only header, no useful memories
		return ""
	}
	return sb.String()
}

// maxToolRounds limits the number of tool call round-trips to prevent infinite loops.
const maxToolRounds = 5

// executeToolCalls executes tool calls and returns tool result messages.
// Each result is returned as an llm.Message with Role=tool and the JSON result as content.
// For known tool types (e.g. Web Search), the result is also wrapped as a typeless card.
func (h *ChatHandler) executeToolCalls(ctx context.Context, toolCalls []llm.ToolCall) []llm.Message {
	var results []llm.Message
	for _, tc := range toolCalls {
		result, err := h.toolExecutor.ExecuteJSON(ctx, tc.Name, tc.Arguments)
		var content string
		if err != nil {
			content = fmt.Sprintf(`{"error":"%s"}`, err.Error())
		} else {
			switch v := result.(type) {
			case string:
				content = v
			default:
				b, _ := json.Marshal(v)
				content = string(b)
			}
		}
		results = append(results, llm.Message{
			Role:       llm.RoleTool,
			Content:    content,
			ToolCallID: tc.ID,
		})
	}
	return results
}

// formatToolResultsAsTypeless converts tool results into typeless card blocks
// that can be appended to the assistant's text content for frontend rendering.
func formatToolResultsAsTypeless(toolCalls []llm.ToolCall, toolResults []llm.Message) string {
	var sb strings.Builder
	for i, tc := range toolCalls {
		if i >= len(toolResults) {
			break
		}
		content := toolResults[i].Content
		card := toolResultToCard(tc.Name, content)
		if card != nil {
			cardJSON, _ := json.Marshal(card)
			sb.WriteString("\n\n```typeless\n")
			sb.Write(cardJSON)
			sb.WriteString("\n```")
		}
	}
	return sb.String()
}

// toolResultToCard converts a single tool result into a typeless card map.
// Returns nil if no card should be rendered.
func toolResultToCard(toolName, content string) map[string]interface{} {
	switch toolName {
	case "web_search":
		return webSearchCard(content)
	case "calculator":
		return calculatorCard(content)
	case "current_time":
		return currentTimeCard(content)
	case "file_read":
		return fileReadCard(content)
	case "file_write":
		return fileWriteCard(content)
	case "system_info":
		return systemInfoCard(content)
	case "memory_search":
		return memorySearchCard(content)
	default:
		return genericToolCard(toolName, content)
	}
}

func webSearchCard(content string) map[string]interface{} {
	var resp struct {
		Query      string            `json:"query"`
		Results    []json.RawMessage `json:"results"`
		TotalCount int               `json:"total_count"`
	}
	if json.Unmarshal([]byte(content), &resp) != nil || len(resp.Results) == 0 {
		return nil
	}
	var results []interface{}
	for _, r := range resp.Results {
		var item interface{}
		json.Unmarshal(r, &item)
		results = append(results, item)
	}
	return map[string]interface{}{
		"type":        "search",
		"query":       resp.Query,
		"total_count": resp.TotalCount,
		"results":     results,
	}
}

func calculatorCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "Calculator",
			"status":  "error",
			"message": errMsg,
		}
	}
	expr, _ := data["expression"].(string)
	result := fmt.Sprintf("%v", data["result"])
	details := []map[string]interface{}{}
	if expr != "" {
		details = append(details, map[string]interface{}{"label": "Expression", "value": expr})
	}
	details = append(details, map[string]interface{}{"label": "Result", "value": result, "copyable": true})
	return map[string]interface{}{
		"type":    "result",
		"title":   "Calculator",
		"status":  "success",
		"message": result,
		"details": details,
	}
}

func currentTimeCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	details := []map[string]interface{}{}
	for _, key := range []string{"datetime", "timezone", "unix"} {
		if v, ok := data[key]; ok {
			details = append(details, map[string]interface{}{"label": key, "value": fmt.Sprintf("%v", v)})
		}
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   "Current Time",
		"status":  "info",
		"details": details,
	}
}

func fileReadCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "File Read",
			"status":  "error",
			"message": errMsg,
		}
	}
	fileContent, _ := data["content"].(string)
	filePath, _ := data["path"].(string)
	if fileContent == "" {
		return nil
	}
	return map[string]interface{}{
		"type":     "collapsible-code",
		"title":    "File Read",
		"filename": filePath,
		"code":     fileContent,
	}
}

func fileWriteCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	status := "success"
	msg := "File written successfully"
	if errMsg, ok := data["error"].(string); ok {
		status = "error"
		msg = errMsg
	} else if m, ok := data["message"].(string); ok {
		msg = m
	}
	details := []map[string]interface{}{}
	if p, ok := data["path"].(string); ok {
		details = append(details, map[string]interface{}{"label": "Path", "value": p})
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   "File Write",
		"status":  status,
		"message": msg,
		"details": details,
	}
}

func systemInfoCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	details := []map[string]interface{}{}
	for _, key := range []string{"os", "arch", "hostname", "cpu_cores", "memory_total", "go_version"} {
		if v, ok := data[key]; ok {
			details = append(details, map[string]interface{}{"label": key, "value": fmt.Sprintf("%v", v)})
		}
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   "System Info",
		"status":  "info",
		"details": details,
	}
}

func memorySearchCard(content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) != nil {
		return nil
	}
	if errMsg, ok := data["error"].(string); ok {
		return map[string]interface{}{
			"type":    "result",
			"title":   "Memory Search",
			"status":  "error",
			"message": errMsg,
		}
	}
	msg := "Search completed"
	if results, ok := data["results"].([]interface{}); ok {
		msg = fmt.Sprintf("Found %d results", len(results))
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   "Memory Search",
		"status":  "success",
		"message": msg,
	}
}

// genericToolCard creates a result card for any unrecognized tool.
func genericToolCard(toolName, content string) map[string]interface{} {
	var data map[string]interface{}
	if json.Unmarshal([]byte(content), &data) == nil {
		if errMsg, ok := data["error"].(string); ok {
			return map[string]interface{}{
				"type":    "result",
				"title":   toolName,
				"status":  "error",
				"message": errMsg,
			}
		}
	}
	display := content
	if len(display) > 500 {
		display = display[:500] + "..."
	}
	return map[string]interface{}{
		"type":    "result",
		"title":   toolName,
		"status":  "info",
		"message": display,
	}
}

// extractMemory extracts important information from conversation messages and saves to daily log.
// source is "im" or "web" for tagging. Returns true if memory was saved.
func (h *ChatHandler) extractMemory(convID, source string) bool {
	if h.store == nil || h.layeredMemory == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Load recent messages — only need 2+ (one user + one assistant)
	messages, err := h.store.GetMessages(ctx, convID, 20, 0)
	if err != nil || len(messages) < 2 {
		return false
	}

	// Build conversation text
	var sb strings.Builder
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n\n", msg.Role, msg.Content))
	}

	// Use LLM to extract structured facts
	provider, _, _, err := h.getDefaultProvider()
	if err != nil {
		return false
	}

	req := llm.ChatRequest{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: `You are a memory extraction assistant. Extract important facts from this conversation as short, structured bullet points.
Focus on: user preferences, personal info, decisions, key facts, action items, technical choices.
Each bullet should be a standalone fact (e.g. "- User prefers Go for backend development").
If nothing worth remembering, respond with exactly "NO_MEMORY_NEEDED".
Respond in the same language as the conversation.`},
			{Role: llm.RoleUser, Content: sb.String()},
		},
		MaxTokens:   300,
		Temperature: 0.3,
	}
	var resp *llm.ChatResponse
	if h.proxyBridge != nil {
		resp, err = h.proxyBridge.Chat(ctx, req)
		if err != nil {
			resp, err = provider.Chat(ctx, req)
		}
	} else {
		resp, err = provider.Chat(ctx, req)
	}
	if err != nil || resp.Message.Content == "" || strings.TrimSpace(resp.Message.Content) == "NO_MEMORY_NEEDED" {
		return false
	}

	tags := []string{"auto-" + source, "conv:" + convID}
	if err := h.layeredMemory.AppendToDaily(ctx, resp.Message.Content, tags); err != nil {
		logger.Warn().Err(err).Str("conv_id", convID).Msg("failed to save memory")
		return false
	}

	logger.Info().Str("conv_id", convID).Str("source", source).Msg("extracted memory from conversation")

	// Emit memory_saved companion event
	if h.companionManager != nil {
		sessionID := h.getCompanionSessionID(convID)
		if sessionID != "" {
			memoryContent := resp.Message.Content // capture before closure
			h.queueEvent(func() {
				h.companionManager.EmitEvent(context.Background(), &companion.SessionEvent{
					SessionID: sessionID,
					Timestamp: time.Now(),
					EventType: companion.EventMemorySaved,
					Message: &companion.MessageEvent{
						Direction:   "system",
						Content:     memoryContent,
						ContentType: "memory",
						Length:      len(memoryContent),
					},
					Status: "success",
				})
			})
		}
	}

	return true
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

	conv, err := h.store.CreateConversation(c.Request().Context(), req.Title, h.getUserID(c))
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
	userID := h.getUserID(c)

	if query != "" {
		// Search conversations by title
		convs, err = h.store.SearchConversations(c.Request().Context(), query, limit, userID)
	} else {
		// List all conversations with pagination
		offset, _ := strconv.Atoi(c.QueryParam("offset"))
		convs, err = h.store.ListConversations(c.Request().Context(), limit, offset, userID)
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

	conv, err := h.checkConversationOwnership(c, id)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, conv)
}

// DeleteConversation deletes a conversation.
func (h *ChatHandler) DeleteConversation(c echo.Context) error {
	id := c.Param("id")

	if _, err := h.checkConversationOwnership(c, id); err != nil {
		return err
	}

	err := h.store.DeleteConversation(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete conversation")
	}

	h.conversationCache.Invalidate(id)

	return c.NoContent(http.StatusNoContent)
}

// PinConversation pins a conversation.
func (h *ChatHandler) PinConversation(c echo.Context) error {
	id := c.Param("id")

	if _, err := h.checkConversationOwnership(c, id); err != nil {
		return err
	}

	err := h.store.PinConversation(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to pin conversation")
	}

	h.conversationCache.Invalidate(id)

	return c.NoContent(http.StatusNoContent)
}

// UnpinConversation unpins a conversation.
func (h *ChatHandler) UnpinConversation(c echo.Context) error {
	id := c.Param("id")

	if _, err := h.checkConversationOwnership(c, id); err != nil {
		return err
	}

	err := h.store.UnpinConversation(c.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to unpin conversation")
	}

	h.conversationCache.Invalidate(id)

	return c.NoContent(http.StatusNoContent)
}

// GetMessages retrieves messages for a conversation.
func (h *ChatHandler) GetMessages(c echo.Context) error {
	id := c.Param("id")

	if _, err := h.checkConversationOwnership(c, id); err != nil {
		return err
	}

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
	Regenerate  bool                `json:"regenerate,omitempty"`
}

// SendMessageResponse represents a response from sending a message.
type SendMessageResponse struct {
	ID       string `json:"id"`
	Role     string `json:"role"`
	Content  string `json:"content"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

// SendMessage sends a message and gets a response from the LLM.
func (h *ChatHandler) SendMessage(c echo.Context) error {
	convID := c.Param("id")

	if _, err := h.checkConversationOwnership(c, convID); err != nil {
		return err
	}

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
		return echo.NewHTTPError(http.StatusServiceUnavailable, proxy.SanitizeError(err))
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

	// Invalidate cache after storing user message so history fetch below is fresh
	h.conversationCache.Invalidate(convID)

	// Emit message event to companion (async)
	sessionID := h.getCompanionSessionID(convID)
	h.emitMessageEventAsync(sessionID, req.Message, "inbound", req.Regenerate)

	// Try to get from cache first
	var messages []memory.Message
	cachedMessages, cacheHit := h.conversationCache.Get(convID)
	if cacheHit {
		messages = cachedMessages
	} else {
		// Cache miss - fetch from database
		messages, err = h.store.GetMessages(c.Request().Context(), convID, 50, 0)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get messages")
		}
		// Store in cache for next time
		h.conversationCache.Set(convID, messages)
	}

	// Convert to LLM messages using pooled slice
	llmMessagesPtr := getMessageSlice()
	defer putMessageSlice(llmMessagesPtr)
	llmMessages := *llmMessagesPtr
	for _, msg := range messages {
		llmMessages = append(llmMessages, llm.Message{
			Role:    llm.Role(msg.Role),
			Content: msg.Content,
		})
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

	// Recall relevant memories and inject as system context (SendMessage)
	if memoryCtx := h.recallMemories(c.Request().Context(), req.Message); memoryCtx != "" {
		compactedMessages = append([]llm.Message{{Role: llm.RoleSystem, Content: memoryCtx}}, compactedMessages...)
	}

	// Build chat request
	chatReq := llm.ChatRequest{
		Model:       model,
		Messages:    compactedMessages,
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

	// Call LLM with tool execution loop
	startTime := time.Now()
	var resp *llm.ChatResponse
	for round := 0; round < maxToolRounds; round++ {
		if h.proxyBridge != nil {
			resp, err = h.proxyBridge.Chat(c.Request().Context(), chatReq)
			if err != nil {
				resp, err = provider.Chat(c.Request().Context(), chatReq)
			}
		} else {
			resp, err = provider.Chat(c.Request().Context(), chatReq)
		}
		if err != nil || resp == nil {
			break
		}
		// No tool calls — done
		if len(resp.Message.ToolCalls) == 0 {
			break
		}
		// Execute tool calls and feed results back
		logger.Info().Int("round", round).Int("tool_calls", len(resp.Message.ToolCalls)).Msg("[chat] executing tool calls")
		toolResults := h.executeToolCalls(c.Request().Context(), resp.Message.ToolCalls)
		// Append assistant message (with tool_calls) + tool results to conversation
		chatReq.Messages = append(chatReq.Messages, resp.Message)
		chatReq.Messages = append(chatReq.Messages, toolResults...)
	}
	latencyMs := float64(time.Since(startTime).Milliseconds())

	// Append typeless cards for tool results to content
	if resp != nil && len(resp.Message.ToolCalls) == 0 {
		// Check if previous rounds had tool calls by looking at messages
		for i := len(chatReq.Messages) - 1; i >= 0; i-- {
			msg := chatReq.Messages[i]
			if msg.Role == llm.RoleAssistant && len(msg.ToolCalls) > 0 {
				// Find corresponding tool results
				var toolResults []llm.Message
				for j := i + 1; j < len(chatReq.Messages) && chatReq.Messages[j].Role == llm.RoleTool; j++ {
					toolResults = append(toolResults, chatReq.Messages[j])
				}
				cards := formatToolResultsAsTypeless(msg.ToolCalls, toolResults)
				if cards != "" {
					resp.Message.Content += cards
				}
				break
			}
		}
	}

	// Use actual model from response if available
	if resp != nil && resp.Model != "" {
		model = resp.Model
	}

	// Estimate tokens if API didn't return usage data
	if resp != nil && resp.Usage.TotalTokens == 0 {
		resp.Usage.PromptTokens = estimateInputTokens(llmMessages)
		resp.Usage.CompletionTokens = estimateTokens(resp.Message.Content)
		resp.Usage.TotalTokens = resp.Usage.PromptTokens + resp.Usage.CompletionTokens
	}

	// Emit LLM request event to companion (async)
	h.emitLLMRequestEventAsync(sessionID, req.Provider, model, resp, err, time.Duration(latencyMs)*time.Millisecond)
	if resp != nil {
		h.emitMessageSentEventAsync(sessionID, resp.Message.Content, resp.Usage.CompletionTokens)
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

		userID := h.getUserID(c)
		h.metricsRecorder.RecordAPICallForUser(userID, model, success, latencyMs, inputTokens, outputTokens, 0, 0, errorType)

		// Record trial usage if this is a trial provider
		if h.providerPool != nil && h.providerPool.TrialQuotaManager != nil && providerpool.IsTrialProvider(req.Provider) {
			h.providerPool.TrialQuotaManager.RecordUsage(inputTokens, outputTokens, convID)
		}
	}

	if err != nil {
		// Emit error event to companion (async)
		sanitizedErr := proxy.SanitizeError(err)
		h.emitErrorEventAsync(sessionID, sanitizedErr)
		// For trial provider errors, return a friendly message key so the frontend
		// can show a localized, user-friendly message instead of raw error text.
		if providerpool.IsTrialProvider(providerID) {
			return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
				"success":     false,
				"trial_error": true,
				"message":     "trial_service_busy",
			})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, sanitizedErr)
	}

	// Get provider name for display
	providerName := providerID
	if h.providerPool != nil {
		if poolProvider, err := h.providerPool.Registry.Get(providerID); err == nil {
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

	// Invalidate cache after storing new message
	h.conversationCache.Invalidate(convID)

	// Async memory extraction for web chat
	if h.layeredMemory != nil {
		h.queueEvent(func() {
			h.extractMemory(convID, "web")
		})
	}

	return c.JSON(http.StatusOK, SendMessageResponse{
		ID:       assistantMsg.ID,
		Role:     "assistant",
		Content:  resp.Message.Content,
		Provider: providerName,
		Model:    model,
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

	if _, err := h.checkConversationOwnership(c, convID); err != nil {
		return err
	}

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
	g.POST("/conversations/:id/pin", h.PinConversation)
	g.POST("/conversations/:id/unpin", h.UnpinConversation)
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

	if _, err := h.checkConversationOwnership(c, convID); err != nil {
		return err
	}

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
	logger.Debug().Str("provider_id", providerID).Str("model", model).Bool("provider_ok", provider != nil).Err(err).Msg("[chat] getDefaultProvider")
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
		return echo.NewHTTPError(http.StatusServiceUnavailable, proxy.SanitizeError(err))
	}

	// Use model from request if specified
	if req.Model != "" {
		model = req.Model
	}
	logger.Debug().Str("model", model).Str("provider", providerID).Msg("[chat] using provider")
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

	// Invalidate cache after storing user message so history fetch below is fresh
	h.conversationCache.Invalidate(convID)

	// Emit message event to companion (async)
	sessionID := h.getCompanionSessionID(convID)
	h.emitMessageEventAsync(sessionID, req.Message, "inbound", req.Regenerate)

	// Get conversation history - try cache first
	var messages []memory.Message
	cachedMessages, cacheHit := h.conversationCache.Get(convID)
	if cacheHit {
		messages = cachedMessages
	} else {
		// Cache miss - fetch from database
		messages, err = h.store.GetMessages(c.Request().Context(), convID, 50, 0)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get messages: "+err.Error())
		}
		// Store in cache for next time
		h.conversationCache.Set(convID, messages)
	}

	// Convert to LLM messages using pooled slice
	llmMessagesPtr := getMessageSlice()
	defer putMessageSlice(llmMessagesPtr)
	llmMessages := *llmMessagesPtr
	for _, msg := range messages {
		llmMessages = append(llmMessages, llm.Message{
			Role:    llm.Role(msg.Role),
			Content: msg.Content,
		})
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

	// Recall relevant memories and inject as system context (StreamMessage)
	if memoryCtx := h.recallMemories(c.Request().Context(), req.Message); memoryCtx != "" {
		compactedMessages = append([]llm.Message{{Role: llm.RoleSystem, Content: memoryCtx}}, compactedMessages...)
	}

	// Build chat request
	chatReq := llm.ChatRequest{
		Model:       model,
		Messages:    compactedMessages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	// Add tool definitions to streaming request
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
	logger.Debug().Str("model", chatReq.Model).Int("messages", len(chatReq.Messages)).Bool("stream", chatReq.Stream).Msg("[chat] request")

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
	var actualModel string    // Track actual model from response
	var actualProvider string // Track actual provider from response
	userID := h.getUserID(c)

	// Pre-allocate buffer for SSE writes to reduce allocations
	sseBuffer := bytes.NewBuffer(make([]byte, 0, 512))

	// Tool execution loop for streaming — collect tool calls, execute, re-stream
	var streamToolCalls []llm.ToolCall
	var streamErrorHandled bool // true when chunk.Error already sent done+stored
	var streamCompleted bool    // true when chunk.Done fired for final round (persistence deferred)
	var finalLatencyMs, finalTTFTMs, finalTPS float64

	// Incremental persistence: insert placeholder message before streaming starts.
	// This ensures a page refresh mid-stream still shows partial content.
	var streamingMsgID string
	var lastFlushLen int
	const flushInterval = 512 // flush to DB every N new chars

	for toolRound := 0; toolRound < maxToolRounds; toolRound++ {
	streamToolCalls = streamToolCalls[:0]
	streamErrorHandled = false

	// Use callback-based streaming to avoid channel issues
	logger.Debug().Str("stream_id", streamID).Str("provider_type", fmt.Sprintf("%T", provider)).Int("tool_round", toolRound).Msg("[chat] starting stream")
	streamCb := func(chunk llm.StreamChunk) error {
		// Track first chunk time for TTFT calculation
		if firstChunkTime.IsZero() && chunk.Delta != "" {
			firstChunkTime = time.Now()
		}

		// Capture actual model/provider from response if provided
		if chunk.Model != "" && actualModel == "" {
			actualModel = chunk.Model
		}
		if chunk.Provider != "" && actualProvider == "" {
			actualProvider = chunk.Provider
		}

		// Collect tool calls from stream chunks (merge partial arguments)
		if len(chunk.ToolCalls) > 0 {
			for _, tc := range chunk.ToolCalls {
				if tc.ID != "" && tc.Name != "" {
					// New tool call — strip bogus initial arguments from some providers
					if tc.Arguments == "null" || tc.Arguments == "undefined" {
						tc.Arguments = ""
					}
					streamToolCalls = append(streamToolCalls, tc)
				} else if len(streamToolCalls) > 0 && tc.Arguments != "" {
					// Partial argument delta — append to last tool call
					streamToolCalls[len(streamToolCalls)-1].Arguments += tc.Arguments
				}
			}
		}

		// Check for error in chunk
		if chunk.Error != "" {
			// Record error metrics
			if h.metricsRecorder != nil {
				latencyMs := float64(time.Since(startTime).Milliseconds())
				h.metricsRecorder.RecordAPICallForUser(userID, model, false, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "stream_error")
			}
			// For trial provider, replace raw error with friendly message key
			chunkErr := chunk.Error
			if providerpool.IsTrialProvider(providerID) {
				chunkErr = "trial_service_busy"
			}
			// Send error to client
			data := map[string]interface{}{
				"error":     chunkErr,
				"done":      true,
				"stream_id": streamID,
			}
			jsonData, _ := json.Marshal(data)
			c.Response().Write([]byte("data: " + string(jsonData) + "\n\n"))
			flusher.Flush()
			// Persist partial content if any was streamed before the error
			if fullContent != "" {
				if streamingMsgID != "" {
					h.store.UpdateMessageContent(context.Background(), streamingMsgID, fullContent, nil)
				} else {
					h.store.AddMessage(context.Background(), convID, memory.Message{
						Role:     "assistant",
						Content:  fullContent,
						Provider: providerName,
						Model:    model,
					})
				}
			}
			h.conversationCache.Invalidate(convID)
			streamErrorHandled = true
			return fmt.Errorf("stream error: %s", chunk.Error)
		}

		fullContent += chunk.Delta

		// Incremental persistence: insert or update the message in DB periodically
		// so a page refresh mid-stream still shows partial content.
		if chunk.Delta != "" && len(fullContent)-lastFlushLen >= flushInterval {
			if streamingMsgID == "" {
				// First flush — insert placeholder
				if m, err := h.store.AddMessage(context.Background(), convID, memory.Message{
					Role:     "assistant",
					Content:  fullContent,
					Provider: providerName,
					Model:    model,
				}); err == nil {
					streamingMsgID = m.ID
					h.conversationCache.Invalidate(convID)
				}
			} else {
				h.store.UpdateMessageContent(context.Background(), streamingMsgID, fullContent, nil)
			}
			lastFlushLen = len(fullContent)
		}

		// Track token usage from chunks
		if chunk.Usage != nil {
			totalInputTokens = chunk.Usage.PromptTokens
			totalOutputTokens = chunk.Usage.CompletionTokens
		}

		// Write SSE data - for non-final chunks only
		if !chunk.Done {
			// Use pre-allocated buffer to reduce allocations
			sseBuffer.Reset()
			sseBuffer.WriteString(`data: {"delta":"`)
			// Escape JSON string manually for performance
			for _, r := range chunk.Delta {
				switch r {
				case '"':
					sseBuffer.WriteString(`\"`)
				case '\\':
					sseBuffer.WriteString(`\\`)
				case '\n':
					sseBuffer.WriteString(`\n`)
				case '\r':
					sseBuffer.WriteString(`\r`)
				case '\t':
					sseBuffer.WriteString(`\t`)
				default:
					sseBuffer.WriteRune(r)
				}
			}
			sseBuffer.WriteString(`","done":false,"stream_id":"`)
			sseBuffer.WriteString(streamID)
			sseBuffer.WriteString(`"}`)
			sseBuffer.WriteString("\n\n")
			c.Response().Write(sseBuffer.Bytes())
			flusher.Flush()
		}

		if chunk.Done {
			// If there are pending tool calls, skip persistence and final SSE —
			// the tool loop will reset fullContent and re-stream.
			if len(streamToolCalls) > 0 {
				return nil
			}

			// Use actual model from response if available, otherwise use request model
			if actualModel != "" {
				model = actualModel
			}

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
				h.metricsRecorder.RecordAPICallForUser(userID, model, true, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "")
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
					logger.Debug().Str("provider_id", providerID).Int("input", totalInputTokens).Int("output", totalOutputTokens).Msg("[chat] recording trial usage")
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
			// Use actual provider/model from response if available
			finalProvider := providerName
			if actualProvider != "" {
				finalProvider = actualProvider
			}
			finalModel := model
			if actualModel != "" {
				finalModel = actualModel
			}
			finalData := map[string]interface{}{
				"delta":     "",
				"done":      true,
				"stream_id": streamID,
				"provider":  finalProvider,
				"model":     finalModel,
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

			// Defer persistence until after typeless cards are appended (outside callback)
			streamCompleted = true
			finalLatencyMs = latencyMs
			finalTTFTMs = ttftMs
			finalTPS = tokensPerSecond

			// Log completion metrics
			logger.Info().
				Str("stream_id", streamID).
				Str("conv_id", convID).
				Str("provider", finalProvider).
				Str("model", finalModel).
				Int("input_tokens", totalInputTokens).
				Int("output_tokens", totalOutputTokens).
				Float64("latency_ms", latencyMs).
				Float64("ttft_ms", ttftMs).
				Float64("tokens_per_sec", tokensPerSecond).
				Msg("[chat] stream completed")
		}

		return nil
	}
	if h.proxyBridge != nil {
		err = h.proxyBridge.ChatStream(ctx, chatReq, streamCb)
		if err != nil {
			logger.Warn().Err(err).Msg("[chat] proxyBridge failed, falling back to direct provider with key fallback")
			err = h.tryProviderWithKeyFallback(ctx, providerID, chatReq, streamCb)
		}
	} else {
		err = h.tryProviderWithKeyFallback(ctx, providerID, chatReq, streamCb)
	}

	// If stream had tool calls, execute them and loop back
	if err == nil && len(streamToolCalls) > 0 {
		logger.Info().Int("round", toolRound).Int("tool_calls", len(streamToolCalls)).Msg("[chat] stream: executing tool calls")
		// Send tool execution status to client
		toolStatusData, _ := json.Marshal(map[string]interface{}{
			"tool_executing": true,
			"tool_calls":     len(streamToolCalls),
			"stream_id":      streamID,
		})
		c.Response().Write([]byte("data: " + string(toolStatusData) + "\n\n"))
		flusher.Flush()

		// Execute tools
		toolResults := h.executeToolCalls(ctx, streamToolCalls)

		// Build assistant message with tool calls for context
		assistantMsg := llm.Message{
			Role:      llm.RoleAssistant,
			Content:   fullContent,
			ToolCalls: streamToolCalls,
		}
		chatReq.Messages = append(chatReq.Messages, assistantMsg)
		chatReq.Messages = append(chatReq.Messages, toolResults...)

		// Reset content for next round (LLM will generate new response)
		fullContent = ""
		continue
	}
	break
	} // end tool loop

	// After tool loop: append typeless cards for the last tool round's results
	if len(streamToolCalls) == 0 {
		// Check if previous rounds had tool calls
		for i := len(chatReq.Messages) - 1; i >= 0; i-- {
			msg := chatReq.Messages[i]
			if msg.Role == llm.RoleAssistant && len(msg.ToolCalls) > 0 {
				var toolResults []llm.Message
				for j := i + 1; j < len(chatReq.Messages) && chatReq.Messages[j].Role == llm.RoleTool; j++ {
					toolResults = append(toolResults, chatReq.Messages[j])
				}
				cards := formatToolResultsAsTypeless(msg.ToolCalls, toolResults)
				if cards != "" {
					fullContent += cards
					// Stream the typeless card to client
					cardData, _ := json.Marshal(map[string]interface{}{
						"delta":     cards,
						"done":      false,
						"stream_id": streamID,
					})
					c.Response().Write([]byte("data: " + string(cardData) + "\n\n"))
					flusher.Flush()
				}
				break
			}
		}
	}

	// Handle stream completion or error
	if err != nil {
		if ctx.Err() != nil {
			// Stream was cancelled
			if h.metricsRecorder != nil {
				latencyMs := float64(time.Since(startTime).Milliseconds())
				h.metricsRecorder.RecordAPICallForUser(userID, model, false, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "cancelled")
			}
			if fullContent != "" {
				if streamingMsgID != "" {
					h.store.UpdateMessageContent(context.Background(), streamingMsgID, fullContent+"\n\n[Response interrupted]", nil)
				} else {
					h.store.AddMessage(context.Background(), convID, memory.Message{
						Role:    "assistant",
						Content: fullContent + "\n\n[Response interrupted]",
					})
				}
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
		// Other error — chunk.Error callback may have already sent done+stored
		if streamErrorHandled {
			return nil
		}
		logger.Error().Err(err).Str("conv_id", convID).Str("model", model).Msg("[chat] stream error")
		errMsg := "An error occurred while streaming the response"
		if providerpool.IsTrialProvider(providerID) {
			errMsg = "trial_service_busy"
		}
		if h.metricsRecorder != nil {
			latencyMs := float64(time.Since(startTime).Milliseconds())
			h.metricsRecorder.RecordAPICallForUser(userID, model, false, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "error")
		}
		// Persist partial content so the user doesn't lose what was already streamed
		if fullContent != "" {
			if streamingMsgID != "" {
				h.store.UpdateMessageContent(context.Background(), streamingMsgID, fullContent, nil)
			} else {
				h.store.AddMessage(context.Background(), convID, memory.Message{
					Role:     "assistant",
					Content:  fullContent,
					Provider: providerName,
					Model:    model,
				})
			}
			h.conversationCache.Invalidate(convID)
		}
		errData, _ := json.Marshal(map[string]interface{}{
			"error":   errMsg,
			"done":    true,
			"delta":   "",
		})
		c.Response().Write([]byte("data: " + string(errData) + "\n\n"))
		flusher.Flush()
		return nil
	}

	// Persist assistant message AFTER typeless cards are appended to fullContent.
	// Safety net: also persist if fullContent is non-empty even when streamCompleted
	// wasn't explicitly set (e.g., missing finish_reason from provider, bridge error).
	if fullContent != "" && (streamCompleted || err == nil) {
		if !streamCompleted {
			logger.Warn().Str("conv_id", convID).Str("model", model).Msg("[chat] persisting message without explicit stream completion (safety net)")
		}
		// Safety-net token estimation: if the Done block was never reached
		// (e.g., bridge path without finish_reason), estimate tokens here.
		if totalInputTokens == 0 {
			totalInputTokens = estimateInputTokens(compactedMessages)
		}
		if totalOutputTokens == 0 {
			totalOutputTokens = estimateTokens(fullContent)
		}
		// Compute latency/TTFT if not already set
		if finalLatencyMs == 0 {
			finalLatencyMs = float64(time.Since(startTime).Milliseconds())
		}
		if finalTTFTMs == 0 && !firstChunkTime.IsZero() {
			finalTTFTMs = float64(firstChunkTime.Sub(startTime).Milliseconds())
		}
		if finalTPS == 0 && totalOutputTokens > 0 {
			totalDuration := time.Since(startTime).Seconds()
			if totalDuration > 0 {
				finalTPS = float64(totalOutputTokens) / totalDuration
			}
		}
		finalStats := &memory.MessageStats{
			InputTokens:     totalInputTokens,
			OutputTokens:    totalOutputTokens,
			TotalTokens:     totalInputTokens + totalOutputTokens,
			LatencyMs:       int64(finalLatencyMs),
			TTFTMs:          int64(finalTTFTMs),
			TokensPerSecond: finalTPS,
		}
		if streamingMsgID != "" {
			// Update the incrementally-persisted message with final content + stats
			if updErr := h.store.UpdateMessageContent(context.Background(), streamingMsgID, fullContent, finalStats); updErr != nil {
				logger.Error().Err(updErr).Str("conv_id", convID).Msg("[chat] failed to update streaming message")
			}
		} else {
			// No incremental message was created (short response) — insert now
			if _, addErr := h.store.AddMessage(context.Background(), convID, memory.Message{
				Role:     "assistant",
				Content:  fullContent,
				Provider: providerName,
				Model:    model,
				Stats:    finalStats,
			}); addErr != nil {
				logger.Error().Err(addErr).Str("conv_id", convID).Msg("[chat] failed to persist assistant message")
			}
		}
		h.conversationCache.Invalidate(convID)

		// Async memory extraction for web chat streaming
		if h.layeredMemory != nil {
			capturedConvID := convID
			h.queueEvent(func() {
				h.extractMemory(capturedConvID, "web")
			})
		}

		// Generate title for new conversations
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
				h.emitLLMRequestEventAsync(sessionID, providerName, model, llmResp, nil, time.Duration(finalLatencyMs)*time.Millisecond)
				h.emitMessageSentEventAsync(sessionID, fullContent, totalOutputTokens)
			}
		}
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
// If the conversation already has a custom title (not auto-generated), it will not be updated.
func (h *ChatHandler) generateConversationTitle(convID, userMessage, aiResponse, targetLang string) {
	// Recover from any panics to prevent crashing the server
	defer func() {
		if r := recover(); r != nil {
			// Log the panic but don't crash - title generation is not critical
			fmt.Printf("panic in generateConversationTitle: %v\n", r)
		}
	}()

	// Check if conversation already has a custom title (not the default/auto-generated one)
	conv, err := h.store.GetConversation(context.Background(), convID)
	if err == nil && conv != nil && conv.Title != "" && !isDefaultTitle(conv.Title) {
		// If the title doesn't look like an auto-generated one (truncated user message),
		// skip updating it. Auto-generated titles typically end with "..." or match the start of userMessage
		currentTitle := conv.Title
		// Check if current title is NOT a prefix of the user message (meaning it was manually set or from a previous conversation)
		userMsgPrefix := userMessage
		if len([]rune(userMsgPrefix)) > 50 {
			userMsgPrefix = string([]rune(userMsgPrefix)[:50])
		}
		// If current title doesn't start with the same content as user message prefix, it's a custom title
		if !strings.HasPrefix(userMsgPrefix, strings.TrimSuffix(currentTitle, "...")) &&
			currentTitle != userMessage &&
			len(currentTitle) > 0 {
			// Title appears to be custom, don't overwrite it
			return
		}
	}

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
	title := h.generateTitleWithLLM(userMessage, targetLang)
	if title == "" {
		// Fallback: use truncated user message
		title = userMessage
		maxLen := 50
		runes := []rune(title)
		if len(runes) > maxLen {
			if idx := findRuneBoundary(title, maxLen); idx > 0 {
				title = title[:idx] + "..."
			} else {
				title = string(runes[:maxLen]) + "..."
			}
		}
	}
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

	var (
		resp *llm.ChatResponse
		err  error
	)
	if h.proxyBridge != nil {
		resp, err = h.proxyBridge.Chat(ctx, req)
		if err != nil {
			resp, err = provider.Chat(ctx, req)
		}
	} else {
		resp, err = provider.Chat(ctx, req)
	}
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

// emitMessageEventAsync queues a message event for async processing.
func (h *ChatHandler) emitMessageEventAsync(sessionID, content, direction string, regenerate bool) {
	if h.companionManager == nil || sessionID == "" {
		return
	}
	h.queueEvent(func() {
		h.emitMessageEvent(context.Background(), sessionID, content, direction, regenerate)
	})
}

// emitLLMRequestEventAsync queues an LLM request event for async processing.
func (h *ChatHandler) emitLLMRequestEventAsync(sessionID, providerName, model string, resp *llm.ChatResponse, err error, duration time.Duration) {
	if h.companionManager == nil || sessionID == "" {
		return
	}
	h.queueEvent(func() {
		h.emitLLMRequestEvent(context.Background(), sessionID, providerName, model, resp, err, duration)
	})
}

// emitMessageSentEventAsync queues a message sent event for async processing.
func (h *ChatHandler) emitMessageSentEventAsync(sessionID, content string, tokens int) {
	if h.companionManager == nil || sessionID == "" {
		return
	}
	h.queueEvent(func() {
		h.emitMessageSentEvent(context.Background(), sessionID, content, tokens)
	})
}

// emitErrorEventAsync queues an error event for async processing.
func (h *ChatHandler) emitErrorEventAsync(sessionID string, errMsg string) {
	if h.companionManager == nil || sessionID == "" {
		return
	}
	h.queueEvent(func() {
		h.emitErrorEvent(context.Background(), sessionID, errMsg)
	})
}

// emitMessageEvent emits a message event to the companion system.
func (h *ChatHandler) emitMessageEvent(ctx context.Context, sessionID, content, direction string, regenerate bool) {
	if h.companionManager == nil {
		return
	}
	// Use different event type based on direction and regenerate flag
	var eventType companion.SessionEventType
	if regenerate {
		eventType = companion.EventRegenerate
	} else if direction == "outbound" {
		eventType = companion.EventMessageSent
	} else {
		eventType = companion.EventMessageReceived
	}
	event := &companion.SessionEvent{
		SessionID: sessionID,
		EventType: eventType,
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
		llmEvent.Error = proxy.SanitizeError(err)
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

// emitMessageSentEvent emits a message sent event to the companion system.
func (h *ChatHandler) emitMessageSentEvent(ctx context.Context, sessionID, content string, tokens int) {
	if h.companionManager == nil {
		return
	}
	event := &companion.SessionEvent{
		SessionID: sessionID,
		EventType: companion.EventMessageSent,
		Platform:  companion.PlatformWeb,
		Message: &companion.MessageEvent{
			Direction: "outbound",
			Content:   content,
		},
	}
	_ = h.companionManager.EmitEvent(ctx, event)
}

// emitErrorEvent emits an error event to the companion system.
func (h *ChatHandler) emitErrorEvent(ctx context.Context, sessionID string, errMsg string) {
	if h.companionManager == nil {
		return
	}
	event := &companion.SessionEvent{
		SessionID: sessionID,
		EventType: companion.EventError,
		Platform:  companion.PlatformWeb,
		Status:    "error",
		Error:     errMsg,
	}
	_ = h.companionManager.EmitEvent(ctx, event)
}

// emitToolCallEvent emits a tool call event to the companion system.
func (h *ChatHandler) emitToolCallEvent(ctx context.Context, sessionID, toolName, toolID string, input map[string]interface{}, output interface{}, duration time.Duration, status string, err error) {
	if h.companionManager == nil {
		return
	}
	toolEvent := &companion.ToolCallEvent{
		ToolName: toolName,
		ToolID:   toolID,
		Input:    input,
		Output:   output,
		Duration: companion.FromDuration(duration),
		Status:   status,
	}
	event := &companion.SessionEvent{
		SessionID: sessionID,
		EventType: companion.EventToolCall,
		Platform:  companion.PlatformWeb,
		ToolCall:  toolEvent,
		Duration:  companion.FromDuration(duration),
		Status:    status,
	}
	if err != nil {
		event.Error = proxy.SanitizeError(err)
	}
	_ = h.companionManager.EmitEvent(ctx, event)
}

// emitSandboxExecEvent emits a sandbox execution event to the companion system.
func (h *ChatHandler) emitSandboxExecEvent(ctx context.Context, sessionID, command string, args []string, duration time.Duration, status string, exitCode int, err error) {
	if h.companionManager == nil {
		return
	}
	// Use ToolCallEvent structure for sandbox execution
	input := map[string]interface{}{
		"command": command,
		"args":    args,
	}
	toolEvent := &companion.ToolCallEvent{
		ToolName:    "sandbox_exec",
		Input:       input,
		Duration:    companion.FromDuration(duration),
		Status:      status,
		SandboxUsed: true,
	}
	if exitCode != 0 {
		toolEvent.Output = map[string]interface{}{"exit_code": exitCode}
	}
	event := &companion.SessionEvent{
		SessionID: sessionID,
		EventType: companion.EventSandboxExec,
		Platform:  companion.PlatformWeb,
		ToolCall:  toolEvent,
		Duration:  companion.FromDuration(duration),
		Status:    status,
	}
	if err != nil {
		event.Error = proxy.SanitizeError(err)
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
