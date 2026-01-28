package claudecode

import (
	"context"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
)

// Provider implements the llm.Provider interface using Claude Code CLI.
type Provider struct {
	config         *ClaudeCodeConfig
	runner         *Runner
	sessionManager *SessionManager
	promptBuilder  *SystemPromptBuilder
	bootstrap      *BootstrapHandler

	mu             sync.RWMutex
	sessionContext map[string]*sessionContext
}

// sessionContext tracks context for a session.
type sessionContext struct {
	SessionId      string
	IsFirstMessage bool
	SystemPrompt   string
}

// NewProvider creates a new Claude Code CLI provider.
func NewProvider(config *ClaudeCodeConfig) *Provider {
	configWithDefaults := config.WithDefaults()

	sessionStore := NewInMemorySessionStore()
	sessionManager := NewSessionManager(sessionStore, configWithDefaults.SessionTTL, 0)

	return &Provider{
		config:         &configWithDefaults,
		runner:         NewRunner(&configWithDefaults),
		sessionManager: sessionManager,
		promptBuilder:  NewSystemPromptBuilder(&configWithDefaults),
		bootstrap:      NewBootstrapHandler(&configWithDefaults),
		sessionContext: make(map[string]*sessionContext),
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "claude-code"
}

// Models returns the list of available models.
func (p *Provider) Models() []string {
	return []string{
		"opus",
		"sonnet",
		"haiku",
		"claude-opus-4-5-20251101",
		"claude-sonnet-4-20250514",
		"claude-3-5-haiku-20241022",
	}
}

// Chat sends a chat completion request and returns the response.
func (p *Provider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	// Build run parameters
	params, err := p.buildRunParams(ctx, req)
	if err != nil {
		return nil, err
	}

	// Execute CLI
	result, err := p.runner.Run(ctx, params)
	if err != nil {
		return nil, err
	}

	// Update session context
	p.updateSessionContext(params, result)

	// Convert to LLM response
	return p.convertToResponse(result, req.Model), nil
}

// ChatStream sends a chat completion request and returns a channel of stream chunks.
func (p *Provider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	// Build run parameters
	params, err := p.buildRunParams(ctx, req)
	if err != nil {
		return nil, err
	}

	// Execute CLI with streaming
	streamCh, err := p.runner.RunStream(ctx, params)
	if err != nil {
		return nil, err
	}

	// Convert stream chunks
	resultCh := make(chan llm.StreamChunk, 10)

	go func() {
		defer close(resultCh)

		var sessionId string
		for chunk := range streamCh {
			if chunk.Error != nil {
				// Send error as final chunk
				resultCh <- llm.StreamChunk{
					Done: true,
				}
				return
			}

			// Track session ID
			if chunk.SessionId != "" {
				sessionId = chunk.SessionId
			}

			// Convert to LLM stream chunk
			llmChunk := llm.StreamChunk{
				ID:    sessionId,
				Model: req.Model,
				Delta: chunk.Text,
				Done:  chunk.Done,
			}

			if chunk.Usage != nil {
				llmChunk.Usage = &llm.Usage{
					PromptTokens:     chunk.Usage.InputTokens,
					CompletionTokens: chunk.Usage.OutputTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
			}

			select {
			case resultCh <- llmChunk:
			case <-ctx.Done():
				return
			}
		}

		// Update session context after stream completes
		if sessionId != "" {
			p.mu.Lock()
			if sc, ok := p.sessionContext[sessionId]; ok {
				sc.IsFirstMessage = false
			}
			p.mu.Unlock()
		}
	}()

	return resultCh, nil
}

// buildRunParams builds CLI run parameters from an LLM request.
func (p *Provider) buildRunParams(ctx context.Context, req llm.ChatRequest) (*RunParams, error) {
	// Extract user prompt from messages
	prompt := p.extractUserPrompt(req.Messages)

	// Get or create session context
	sessionCtx := p.getOrCreateSessionContext(req)

	// Build system prompt
	systemPrompt := p.buildSystemPrompt(ctx, req, sessionCtx)

	// Determine model
	model := req.Model
	if model == "" {
		model = p.config.DefaultModel
	}

	return &RunParams{
		Prompt:         prompt,
		Model:          model,
		SessionId:      sessionCtx.SessionId,
		IsResume:       !sessionCtx.IsFirstMessage,
		SystemPrompt:   systemPrompt,
		IsFirstMessage: sessionCtx.IsFirstMessage,
		WorkspaceDir:   p.config.WorkspaceDir,
		Timeout:        p.config.Timeout,
	}, nil
}

// extractUserPrompt extracts the user prompt from messages.
func (p *Provider) extractUserPrompt(messages []llm.Message) string {
	// Find the last user message
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleUser {
			return messages[i].Content
		}
	}

	// If no user message, concatenate all non-system messages
	var parts []string
	for _, msg := range messages {
		if msg.Role != llm.RoleSystem {
			parts = append(parts, msg.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}

// getOrCreateSessionContext gets or creates session context for a request.
func (p *Provider) getOrCreateSessionContext(req llm.ChatRequest) *sessionContext {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Use model as part of session key to separate sessions by model
	sessionKey := req.Model
	if sessionKey == "" {
		sessionKey = p.config.DefaultModel
	}

	if sc, ok := p.sessionContext[sessionKey]; ok {
		return sc
	}

	// Create new session context
	sc := &sessionContext{
		SessionId:      "", // Will be generated by builder
		IsFirstMessage: true,
	}
	p.sessionContext[sessionKey] = sc

	return sc
}

// buildSystemPrompt builds the system prompt for a request.
func (p *Provider) buildSystemPrompt(ctx context.Context, req llm.ChatRequest, sessionCtx *sessionContext) string {
	// Only send system prompt on first message (based on config)
	if !sessionCtx.IsFirstMessage && p.config.Backend.SystemPromptWhen == "first" {
		return ""
	}

	// Extract system message from request
	var systemMessage string
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleSystem {
			systemMessage = msg.Content
			break
		}
	}

	// Build complete system prompt
	return p.promptBuilder.Build(ctx, systemMessage)
}

// updateSessionContext updates session context after a run.
func (p *Provider) updateSessionContext(params *RunParams, result *RunResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	sessionKey := params.Model
	if sessionKey == "" {
		sessionKey = p.config.DefaultModel
	}

	if sc, ok := p.sessionContext[sessionKey]; ok {
		if result.SessionId != "" {
			sc.SessionId = result.SessionId
		}
		sc.IsFirstMessage = false
	}
}

// convertToResponse converts a CLI result to an LLM response.
func (p *Provider) convertToResponse(result *RunResult, model string) *llm.ChatResponse {
	return &llm.ChatResponse{
		ID:    result.SessionId,
		Model: model,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: result.Output.Text,
		},
		Usage: llm.Usage{
			PromptTokens:     result.Output.Usage.InputTokens,
			CompletionTokens: result.Output.Usage.OutputTokens,
			TotalTokens:      result.Output.Usage.TotalTokens,
		},
	}
}

// Close shuts down the provider and cleans up resources.
func (p *Provider) Close() error {
	p.sessionManager.Stop()
	return p.runner.Close()
}

// Start starts background services.
func (p *Provider) Start() {
	p.sessionManager.Start()
}

// SessionManager returns the session manager.
func (p *Provider) SessionManager() *SessionManager {
	return p.sessionManager
}

// ResetSession resets the session context for a model.
func (p *Provider) ResetSession(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if model == "" {
		model = p.config.DefaultModel
	}

	delete(p.sessionContext, model)
}

// ResetAllSessions resets all session contexts.
func (p *Provider) ResetAllSessions() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.sessionContext = make(map[string]*sessionContext)
}
