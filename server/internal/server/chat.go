package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tools"
)

// ChatHandler handles chat-related API endpoints.
type ChatHandler struct {
	store        *memory.Store
	providers    *llm.ProviderRegistry
	toolRegistry *tools.Registry
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(store *memory.Store, providers *llm.ProviderRegistry, toolRegistry *tools.Registry) *ChatHandler {
	return &ChatHandler{
		store:        store,
		providers:    providers,
		toolRegistry: toolRegistry,
	}
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

	// Call LLM
	resp, err := provider.Chat(c.Request().Context(), chatReq)
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
	providers := make([]ProviderInfo, len(names))

	for i, name := range names {
		provider := h.providers.Get(name)
		providers[i] = ProviderInfo{
			Name:   name,
			Models: provider.Models(),
		}
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
	g.GET("/providers", h.ListProviders)
	g.POST("/providers/:provider/refresh", h.RefreshProviderModels)
	g.GET("/tools", h.ListTools)
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

	// Build chat request
	chatReq := llm.ChatRequest{
		Model:       req.Model,
		Messages:    llmMessages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	// Start streaming
	chunks, err := provider.ChatStream(c.Request().Context(), chatReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to start streaming: "+err.Error())
	}

	// Set SSE headers
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	ctx := c.Request().Context()
	var fullContent string

	for {
		select {
		case <-ctx.Done():
			return nil
		case chunk, ok := <-chunks:
			if !ok {
				// Channel closed, store the complete message
				if fullContent != "" {
					h.store.AddMessage(context.Background(), convID, memory.Message{
						Role:    "assistant",
						Content: fullContent,
					})
				}
				c.Response().Write([]byte("data: [DONE]\n\n"))
				c.Response().Flush()
				return nil
			}

			fullContent += chunk.Delta

			// Write SSE data
			data := map[string]interface{}{
				"delta": chunk.Delta,
				"done":  chunk.Done,
			}
			if chunk.Usage != nil {
				data["usage"] = chunk.Usage
			}

			jsonData, _ := json.Marshal(data)
			c.Response().Write([]byte("data: " + string(jsonData) + "\n\n"))
			c.Response().Flush()

			if chunk.Done {
				// Store the complete message
				h.store.AddMessage(context.Background(), convID, memory.Message{
					Role:    "assistant",
					Content: fullContent,
				})
				c.Response().Write([]byte("data: [DONE]\n\n"))
				c.Response().Flush()
				return nil
			}
		}
	}
}
