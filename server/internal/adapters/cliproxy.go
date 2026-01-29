// Package adapters provides adapters for LLM providers that don't natively support tool calling.
package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Tool represents a tool definition.
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolCall represents a tool call request.
type ToolCall struct {
	Name   string                 `json:"name"`
	Input  map[string]interface{} `json:"input"`
}

// ToolResult represents a tool call result.
type ToolResult struct {
	Name    string      `json:"name"`
	Output  interface{} `json:"output"`
	Error   string      `json:"error,omitempty"`
	Success bool        `json:"success"`
}

// LLMProvider is an interface for LLM providers.
type LLMProvider interface {
	Chat(ctx context.Context, messages []Message, options *ChatOptions) (*ChatResponse, error)
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatOptions represents chat options.
type ChatOptions struct {
	Model       string  `json:"model,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

// ChatResponse represents a chat response.
type ChatResponse struct {
	Content string `json:"content"`
	Model   string `json:"model,omitempty"`
}

// CLIProxyAdapter converts tool calls to prompt-based format for providers
// that don't support native tool calling.
type CLIProxyAdapter struct {
	BaseProvider   LLMProvider
	PromptTemplate string
	Tools          map[string]Tool
}

// NewCLIProxyAdapter creates a new CLIProxy adapter.
func NewCLIProxyAdapter(provider LLMProvider) *CLIProxyAdapter {
	return &CLIProxyAdapter{
		BaseProvider:   provider,
		PromptTemplate: defaultPromptTemplate,
		Tools:          make(map[string]Tool),
	}
}

const defaultPromptTemplate = `You have access to the following tools:

{{TOOLS}}

To use a tool, respond with a JSON object in the following format:
{
  "tool": "tool_name",
  "input": {
    "param1": "value1",
    "param2": "value2"
  }
}

If you don't need to use a tool, respond normally.

User request: {{PROMPT}}`

// RegisterTool registers a tool with the adapter.
func (a *CLIProxyAdapter) RegisterTool(tool Tool) {
	a.Tools[tool.Name] = tool
}

// RegisterTools registers multiple tools with the adapter.
func (a *CLIProxyAdapter) RegisterTools(tools []Tool) {
	for _, tool := range tools {
		a.RegisterTool(tool)
	}
}

// BuildToolPrompt builds a prompt that includes tool definitions.
func (a *CLIProxyAdapter) BuildToolPrompt(userPrompt string) string {
	toolsDesc := a.buildToolsDescription()

	prompt := strings.Replace(a.PromptTemplate, "{{TOOLS}}", toolsDesc, 1)
	prompt = strings.Replace(prompt, "{{PROMPT}}", userPrompt, 1)

	return prompt
}

// buildToolsDescription builds a description of all registered tools.
func (a *CLIProxyAdapter) buildToolsDescription() string {
	var sb strings.Builder

	for _, tool := range a.Tools {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
		if len(tool.Parameters) > 0 {
			sb.WriteString("  Parameters:\n")
			for name, schema := range tool.Parameters {
				sb.WriteString(fmt.Sprintf("    - %s: %v\n", name, schema))
			}
		}
	}

	return sb.String()
}

// Chat sends a chat request with tool support.
func (a *CLIProxyAdapter) Chat(ctx context.Context, messages []Message, options *ChatOptions) (*ChatResponse, error) {
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages provided")
	}

	// Get the last user message
	lastMessage := messages[len(messages)-1]
	if lastMessage.Role != "user" {
		return nil, fmt.Errorf("last message must be from user")
	}

	// Build prompt with tool definitions
	prompt := a.BuildToolPrompt(lastMessage.Content)

	// Replace the last message with the tool-enhanced prompt
	enhancedMessages := make([]Message, len(messages))
	copy(enhancedMessages, messages)
	enhancedMessages[len(enhancedMessages)-1] = Message{
		Role:    "user",
		Content: prompt,
	}

	// Call the base provider
	return a.BaseProvider.Chat(ctx, enhancedMessages, options)
}

// ParseToolCall parses a tool call from the response.
func (a *CLIProxyAdapter) ParseToolCall(response string) (*ToolCall, error) {
	// Try to find JSON in the response
	jsonPattern := regexp.MustCompile(`\{[\s\S]*?"tool"[\s\S]*?\}`)
	match := jsonPattern.FindString(response)

	if match == "" {
		return nil, nil // No tool call found
	}

	var toolCall struct {
		Tool  string                 `json:"tool"`
		Input map[string]interface{} `json:"input"`
	}

	if err := json.Unmarshal([]byte(match), &toolCall); err != nil {
		return nil, fmt.Errorf("failed to parse tool call: %w", err)
	}

	if toolCall.Tool == "" {
		return nil, nil // Not a valid tool call
	}

	return &ToolCall{
		Name:  toolCall.Tool,
		Input: toolCall.Input,
	}, nil
}

// ExecuteToolCall executes a tool call and returns the result.
// This is a placeholder - actual tool execution should be implemented by the caller.
func (a *CLIProxyAdapter) ExecuteToolCall(ctx context.Context, call *ToolCall, executor func(context.Context, *ToolCall) (interface{}, error)) (*ToolResult, error) {
	if _, ok := a.Tools[call.Name]; !ok {
		return &ToolResult{
			Name:    call.Name,
			Error:   fmt.Sprintf("unknown tool: %s", call.Name),
			Success: false,
		}, nil
	}

	output, err := executor(ctx, call)
	if err != nil {
		return &ToolResult{
			Name:    call.Name,
			Error:   err.Error(),
			Success: false,
		}, nil
	}

	return &ToolResult{
		Name:    call.Name,
		Output:  output,
		Success: true,
	}, nil
}

// BuildToolResultPrompt builds a prompt that includes the tool result.
func (a *CLIProxyAdapter) BuildToolResultPrompt(result *ToolResult) string {
	if result.Success {
		return fmt.Sprintf("Tool '%s' returned:\n%v\n\nPlease continue with your response.", result.Name, result.Output)
	}
	return fmt.Sprintf("Tool '%s' failed with error: %s\n\nPlease try a different approach.", result.Name, result.Error)
}
