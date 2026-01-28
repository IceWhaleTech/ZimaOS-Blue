package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
)

// ChatHandler handles chat-related API endpoints.
type ChatHandler struct {
	store            *memory.Store
	providers        *llm.ProviderRegistry
	toolRegistry     *tools.Registry
	streamController *claudecode.StreamController
	compactionConfig claudecode.CompactionConfig
	claudeCodeHandler *claudecode.Handler
	metricsRecorder  MetricsRecorder
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

	return c.JSON(http.StatusCreated, conv)
}

// ListConversations lists all conversations.
func (h *ChatHandler) ListConversations(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 50
	}

	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	convs, err := h.store.ListConversations(c.Request().Context(), limit, offset)
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

// SendMessageRequest represents a request to send a message.
type SendMessageRequest struct {
	Message     string `json:"message"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int    `json:"max_tokens,omitempty"`
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

	if req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "message is required")
	}

	if req.Provider == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	// Get provider
	provider := h.providers.Get(req.Provider)
	if provider == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "provider not found")
	}

	// Store user message
	_, err := h.store.AddMessage(c.Request().Context(), convID, memory.Message{
		Role:    "user",
		Content: req.Message,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store message")
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
		Model:       req.Model,
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

		h.metricsRecorder.RecordAPICall(req.Model, success, latencyMs, inputTokens, outputTokens, 0, 0, errorType)
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get response from LLM")
	}

	// Store assistant message
	assistantMsg, err := h.store.AddMessage(c.Request().Context(), convID, memory.Message{
		Role:    "assistant",
		Content: resp.Message.Content,
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
		// Filter out claude-code provider if it's disabled
		if name == "claude-code" && h.claudeCodeHandler != nil && !h.claudeCodeHandler.IsEnabled() {
			continue
		}
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

// RegisterChatRoutes registers chat-related routes.
func (h *ChatHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/conversations", h.CreateConversation)
	g.GET("/conversations", h.ListConversations)
	g.GET("/conversations/:id", h.GetConversation)
	g.DELETE("/conversations/:id", h.DeleteConversation)
	g.GET("/conversations/:id/messages", h.GetMessages)
	g.POST("/conversations/:id/messages", h.SendMessage)
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
	convID := c.Param("id")

	var req SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Message == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "message is required")
	}

	if req.Provider == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "provider is required")
	}

	// Get provider
	provider := h.providers.Get(req.Provider)
	if provider == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "provider not found: "+req.Provider)
	}

	// Get user's preferred language from Accept-Language header (for future use)
	// acceptLang := c.Request().Header.Get("Accept-Language")
	// targetLang := parseAcceptLanguage(acceptLang)

	// Store user message
	_, err := h.store.AddMessage(c.Request().Context(), convID, memory.Message{
		Role:    "user",
		Content: req.Message,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to store message: "+err.Error())
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
		Model:       req.Model,
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
				h.metricsRecorder.RecordAPICall(req.Model, false, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "stream_error")
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

		// Write SSE data
		data := map[string]interface{}{
			"delta":     chunk.Delta,
			"done":      chunk.Done,
			"stream_id": streamID,
		}
		if chunk.Usage != nil {
			data["usage"] = chunk.Usage
		}

		jsonData, _ := json.Marshal(data)
		c.Response().Write([]byte("data: " + string(jsonData) + "\n\n"))
		flusher.Flush()

		if chunk.Done {
			// Record successful completion metrics
			if h.metricsRecorder != nil {
				latencyMs := float64(time.Since(startTime).Milliseconds())
				h.metricsRecorder.RecordAPICall(req.Model, true, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "")
				// Record speed metrics
				if !firstChunkTime.IsZero() && totalOutputTokens > 0 {
					ttftMs := float64(firstChunkTime.Sub(startTime).Milliseconds())
					totalDuration := time.Since(startTime).Seconds()
					if totalDuration > 0 {
						tokensPerSecond := float64(totalOutputTokens) / totalDuration
						h.metricsRecorder.RecordSpeed(req.Model, tokensPerSecond, ttftMs, tokensPerSecond)
					}
				}
			}
			// Store the complete message
			h.store.AddMessage(context.Background(), convID, memory.Message{
				Role:    "assistant",
				Content: fullContent,
			})
			// Generate title for new conversations
			go h.generateConversationTitle(convID, req.Message, "en")
		}

		return nil
	})

	// Handle stream completion or error
	if err != nil {
		if ctx.Err() != nil {
			// Stream was cancelled
			if h.metricsRecorder != nil {
				latencyMs := float64(time.Since(startTime).Milliseconds())
				h.metricsRecorder.RecordAPICall(req.Model, false, latencyMs, int64(totalInputTokens), int64(totalOutputTokens), 0, 0, "cancelled")
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
// targetLang specifies the language for the generated title (e.g., "en", "zh", "ja").
// This function is safe to call in a goroutine - it recovers from panics.
func (h *ChatHandler) generateConversationTitle(convID, userMessage, targetLang string) {
	// Recover from any panics to prevent crashing the server
	defer func() {
		if r := recover(); r != nil {
			// Log the panic but don't crash - title generation is not critical
			fmt.Printf("panic in generateConversationTitle: %v\n", r)
		}
	}()

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
	if len(title) > maxLen {
		if idx := findWordBoundary(title, maxLen); idx > 0 {
			title = title[:idx] + "..."
		} else {
			title = title[:maxLen] + "..."
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
// For claude-code provider, it extracts API credentials and uses Claude API directly.
func (h *ChatHandler) generateTitleWithLLM(userMessage, targetLang string) string {
	// Find a suitable provider for title generation
	// Prefer direct API providers over CLI-based ones for speed
	var provider llm.Provider

	// Try providers in order of preference
	providerNames := []string{"claude", "openai", "ollama", "claude-code"}
	for _, name := range providerNames {
		p := h.providers.Get(name)
		if p == nil {
			continue
		}

		// For claude-code, try to extract API credentials and use Claude API directly
		if name == "claude-code" {
			if ccProvider, ok := p.(*claudecode.Provider); ok {
				apiKey, baseURL := ccProvider.GetAPICredentials()
				if apiKey != "" {
					// Create a temporary Claude provider with the same credentials
					provider = llm.NewClaudeProvider(apiKey, baseURL)
					break
				}
			}
			// No API key configured for claude-code, skip it
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
	if len(content) > 500 {
		content = content[:500] + "..."
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
	if len(title) > 50 {
		if idx := findWordBoundary(title, 50); idx > 0 {
			title = title[:idx]
		} else {
			title = title[:50]
		}
	}
	return title
}

// findWordBoundary finds the last space before maxLen.
func findWordBoundary(s string, maxLen int) int {
	for i := maxLen - 1; i >= 0; i-- {
		if s[i] == ' ' {
			return i
		}
	}
	return -1
}

// sanitizeTitle removes newlines and extra whitespace from a title.
func sanitizeTitle(s string) string {
	result := make([]byte, 0, len(s))
	lastWasSpace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\n' || c == '\r' || c == '\t' {
			c = ' '
		}
		if c == ' ' {
			if !lastWasSpace {
				result = append(result, c)
				lastWasSpace = true
			}
		} else {
			result = append(result, c)
			lastWasSpace = false
		}
	}
	return string(result)
}

// isDefaultTitle checks if the title is a default/placeholder title that should be auto-generated.
// Supports multiple languages.
func isDefaultTitle(title string) bool {
	defaultTitles := []string{
		"New Conversation",  // English
		"新对话",            // Chinese Simplified
		"新對話",            // Chinese Traditional
		"Nueva conversación", // Spanish
		"Nouvelle conversation", // French
		"Neue Unterhaltung", // German
		"新しい会話",        // Japanese
		"새 대화",           // Korean
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
