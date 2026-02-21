package claudecode

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

// Provider implements the llm.Provider interface using Claude Code CLI.
type Provider struct {
	config          *ClaudeCodeConfig
	runner          *Runner
	sandboxedRunner *SandboxedRunner
	sessionManager  *SessionManager
	promptBuilder   *SystemPromptBuilder
	bootstrap       *BootstrapHandler

	mu               sync.RWMutex
	sessionContext   map[string]*sessionContext
	companionManager *companion.Manager
	companionSession string // session ID for companion events
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

	// Create sandboxed runner (falls back to regular runner if sandbox not supported)
	sandboxedRunner, _ := NewSandboxedRunner(&configWithDefaults)

	return &Provider{
		config:          &configWithDefaults,
		runner:          NewRunner(&configWithDefaults),
		sandboxedRunner: sandboxedRunner,
		sessionManager:  sessionManager,
		promptBuilder:   NewSystemPromptBuilder(&configWithDefaults),
		bootstrap:       NewBootstrapHandler(&configWithDefaults),
		sessionContext:  make(map[string]*sessionContext),
	}
}

// SetToolRegistry sets the tool registry for including tool descriptions in system prompts.
func (p *Provider) SetToolRegistry(registry *tools.Registry) {
	p.promptBuilder.SetToolRegistry(registry)
}

// SetWorkspace sets the workspace manager for injecting workspace files into system prompts.
func (p *Provider) SetWorkspace(mgr *workspace.Manager) {
	p.promptBuilder.SetWorkspace(mgr)
}

// SetCompanionManager sets the companion manager for event tracking.
func (p *Provider) SetCompanionManager(manager *companion.Manager, sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.companionManager = manager
	p.companionSession = sessionID
}

// emitSandboxExecEvent emits a sandbox execution event to the companion system.
func (p *Provider) emitSandboxExecEvent(ctx context.Context, command string, args []string, duration time.Duration, status string, exitCode int, err error) {
	p.mu.RLock()
	manager := p.companionManager
	sessionID := p.companionSession
	p.mu.RUnlock()

	if manager == nil || sessionID == "" {
		return
	}

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
		Platform:  companion.PlatformAPI,
		ToolCall:  toolEvent,
		Duration:  companion.FromDuration(duration),
		Status:    status,
	}
	if err != nil {
		event.Error = err.Error()
	}
	_ = manager.EmitEvent(ctx, event)
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

	// Execute CLI (use sandboxed runner if available)
	var result *RunResult
	startTime := time.Now()
	useSandbox := p.sandboxedRunner != nil && p.sandboxedRunner.IsSandboxEnabled()
	if useSandbox {
		result, err = p.sandboxedRunner.Run(ctx, params)
	} else {
		result, err = p.runner.Run(ctx, params)
	}
	duration := time.Since(startTime)

	// Emit sandbox execution event if sandbox was used
	if useSandbox {
		status := "completed"
		exitCode := 0
		if err != nil {
			status = "failed"
		}
		if result != nil {
			exitCode = result.ExitCode
		}
		p.emitSandboxExecEvent(ctx, params.Backend.Command, []string{params.Model, params.Prompt}, duration, status, exitCode, err)
	}

	if err != nil {
		return nil, err
	}

	// Update session context
	p.updateSessionContext(params, result)

	// Convert to LLM response
	return p.convertToResponse(result, params.Model), nil
}

// ChatStream sends a chat completion request and returns a channel of stream chunks.
func (p *Provider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	// Build run parameters
	params, err := p.buildRunParams(ctx, req)
	if err != nil {
		return nil, err
	}

	// Execute CLI with streaming (use sandboxed runner if available)
	var streamCh <-chan CliStreamChunk
	if p.sandboxedRunner != nil {
		streamCh, err = p.sandboxedRunner.RunStream(ctx, params)
	} else {
		streamCh, err = p.runner.RunStream(ctx, params)
	}
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
				// Send error as final chunk with error message
				errMsg := chunk.Error.Error()
				// Make error message more user-friendly
				errMsgLower := strings.ToLower(errMsg)
				if strings.Contains(errMsgLower, "invalid api key") ||
					strings.Contains(errMsgLower, "authentication") ||
					strings.Contains(errMsgLower, "please run /login") ||
					strings.Contains(errMsgLower, "api key") ||
					strings.Contains(errMsgLower, "unauthorized") {
					errMsg = "API key is invalid or not configured. Please check your provider settings."
				}
				resultCh <- llm.StreamChunk{
					Done:  true,
					Error: errMsg,
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

// ChatStreamCallback sends a chat completion request and calls the callback for each chunk.
func (p *Provider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	// Build run parameters
	params, err := p.buildRunParams(ctx, req)
	if err != nil {
		return err
	}

	// Execute CLI with streaming (use sandboxed runner if available)
	var streamCh <-chan CliStreamChunk
	startTime := time.Now()
	useSandbox := p.sandboxedRunner != nil && p.sandboxedRunner.IsSandboxEnabled()
	if useSandbox {
		streamCh, err = p.sandboxedRunner.RunStream(ctx, params)
	} else {
		streamCh, err = p.runner.RunStream(ctx, params)
	}
	if err != nil {
		// Emit sandbox event on error
		if useSandbox {
			p.emitSandboxExecEvent(ctx, params.Backend.Command, []string{params.Model}, time.Since(startTime), "failed", 1, err)
		}
		return err
	}

	var sessionId string
	var streamErr error
	for chunk := range streamCh {
		if chunk.Error != nil {
			// Make error message more user-friendly
			errMsg := chunk.Error.Error()
			errMsgLower := strings.ToLower(errMsg)
			if strings.Contains(errMsgLower, "invalid api key") ||
				strings.Contains(errMsgLower, "authentication") ||
				strings.Contains(errMsgLower, "please run /login") ||
				strings.Contains(errMsgLower, "api key") ||
				strings.Contains(errMsgLower, "unauthorized") {
				errMsg = "API key is invalid or not configured. Please check your provider settings."
			}
			streamErr = chunk.Error
			return callback(llm.StreamChunk{
				Done:  true,
				Error: errMsg,
			})
		}

		// Track session ID
		if chunk.SessionId != "" {
			sessionId = chunk.SessionId
		}

		// Convert to LLM stream chunk
		// Use actual provider/model from config if available
		actualModel := params.Model
		if p.config.ActualModel != "" {
			actualModel = p.config.ActualModel
		}
		llmChunk := llm.StreamChunk{
			ID:       sessionId,
			Model:    actualModel,
			Provider: p.config.ActualProvider,
			Delta:    chunk.Text,
			Done:     chunk.Done,
		}

		if chunk.Usage != nil {
			llmChunk.Usage = &llm.Usage{
				PromptTokens:     chunk.Usage.InputTokens,
				CompletionTokens: chunk.Usage.OutputTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		if err := callback(llmChunk); err != nil {
			return err
		}
	}

	// Emit sandbox event after stream completes
	if useSandbox {
		status := "completed"
		if streamErr != nil {
			status = "failed"
		}
		p.emitSandboxExecEvent(ctx, params.Backend.Command, []string{params.Model}, time.Since(startTime), status, 0, streamErr)
	}

	// Update session context after stream completes
	if sessionId != "" {
		p.mu.Lock()
		if sc, ok := p.sessionContext[sessionId]; ok {
			sc.IsFirstMessage = false
		}
		p.mu.Unlock()
	}

	return nil
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
	if p.sandboxedRunner != nil {
		p.sandboxedRunner.Close()
	}
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

// IsSandboxEnabled returns whether sandbox mode is enabled.
func (p *Provider) IsSandboxEnabled() bool {
	return p.sandboxedRunner != nil && p.sandboxedRunner.IsSandboxEnabled()
}

// GetSandboxStatus returns the current sandbox status.
func (p *Provider) GetSandboxStatus() map[string]interface{} {
	if p.sandboxedRunner == nil {
		return map[string]interface{}{
			"enabled":   false,
			"active":    false,
			"supported": false,
		}
	}
	return p.sandboxedRunner.GetSandboxStatus()
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

// UpdateCredentials updates the API key and base URL for the provider.
func (p *Provider) UpdateCredentials(apiKey, baseURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config.APIKey = apiKey
	p.config.BaseURL = baseURL

	// Recreate runner with updated config
	configWithDefaults := p.config.WithDefaults()
	p.runner = NewRunner(&configWithDefaults)
}

// GetAPICredentials returns the API key and base URL configured for this provider.
// This can be used to create a direct Claude API provider for simpler tasks.
func (p *Provider) GetAPICredentials() (apiKey, baseURL string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config.APIKey, p.config.BaseURL
}
