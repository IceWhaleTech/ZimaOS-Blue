package adapters

import (
	"encoding/json"
)

// ccNexusAdapter converts tool schemas between different provider formats.
type ccNexusAdapter struct {
	SourceFormat string // "openai", "anthropic", "ollama", "gemini"
	TargetFormat string
}

// NewCCNexusAdapter creates a new ccNexus adapter.
func NewCCNexusAdapter(sourceFormat, targetFormat string) *ccNexusAdapter {
	return &ccNexusAdapter{
		SourceFormat: sourceFormat,
		TargetFormat: targetFormat,
	}
}

// OpenAITool represents an OpenAI tool definition.
type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

// OpenAIFunction represents an OpenAI function definition.
type OpenAIFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// AnthropicTool represents an Anthropic tool definition.
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// OllamaTool represents an Ollama tool definition.
type OllamaTool struct {
	Type     string         `json:"type"`
	Function OllamaFunction `json:"function"`
}

// OllamaFunction represents an Ollama function definition.
type OllamaFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  OllamaParameters       `json:"parameters"`
}

// OllamaParameters represents Ollama function parameters.
type OllamaParameters struct {
	Type       string                            `json:"type"`
	Properties map[string]OllamaParameterProperty `json:"properties"`
	Required   []string                          `json:"required,omitempty"`
}

// OllamaParameterProperty represents an Ollama parameter property.
type OllamaParameterProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// GeminiTool represents a Gemini tool definition.
type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"function_declarations"`
}

// GeminiFunctionDeclaration represents a Gemini function declaration.
type GeminiFunctionDeclaration struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ConvertTool converts a tool from one format to another.
func (a *ccNexusAdapter) ConvertTool(tool Tool) (interface{}, error) {
	switch a.TargetFormat {
	case "openai":
		return a.ToOpenAIFormat(tool), nil
	case "anthropic":
		return a.ToAnthropicFormat(tool), nil
	case "ollama":
		return a.ToOllamaFormat(tool), nil
	case "gemini":
		return a.ToGeminiFormat(tool), nil
	default:
		return a.ToOpenAIFormat(tool), nil
	}
}

// ToOpenAIFormat converts a tool to OpenAI format.
func (a *ccNexusAdapter) ToOpenAIFormat(tool Tool) *OpenAITool {
	return &OpenAITool{
		Type: "function",
		Function: OpenAIFunction{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		},
	}
}

// ToAnthropicFormat converts a tool to Anthropic format.
func (a *ccNexusAdapter) ToAnthropicFormat(tool Tool) *AnthropicTool {
	return &AnthropicTool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: tool.Parameters,
	}
}

// ToOllamaFormat converts a tool to Ollama format.
func (a *ccNexusAdapter) ToOllamaFormat(tool Tool) *OllamaTool {
	params := OllamaParameters{
		Type:       "object",
		Properties: make(map[string]OllamaParameterProperty),
	}

	// Convert parameters to Ollama format
	if props, ok := tool.Parameters["properties"].(map[string]interface{}); ok {
		for name, prop := range props {
			if propMap, ok := prop.(map[string]interface{}); ok {
				ollamaProp := OllamaParameterProperty{
					Type: "string", // Default type
				}
				if t, ok := propMap["type"].(string); ok {
					ollamaProp.Type = t
				}
				if d, ok := propMap["description"].(string); ok {
					ollamaProp.Description = d
				}
				if e, ok := propMap["enum"].([]interface{}); ok {
					for _, v := range e {
						if s, ok := v.(string); ok {
							ollamaProp.Enum = append(ollamaProp.Enum, s)
						}
					}
				}
				params.Properties[name] = ollamaProp
			}
		}
	}

	if required, ok := tool.Parameters["required"].([]interface{}); ok {
		for _, r := range required {
			if s, ok := r.(string); ok {
				params.Required = append(params.Required, s)
			}
		}
	}

	return &OllamaTool{
		Type: "function",
		Function: OllamaFunction{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
		},
	}
}

// ToGeminiFormat converts a tool to Gemini format.
func (a *ccNexusAdapter) ToGeminiFormat(tool Tool) *GeminiTool {
	return &GeminiTool{
		FunctionDeclarations: []GeminiFunctionDeclaration{
			{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		},
	}
}

// ConvertTools converts multiple tools to the target format.
func (a *ccNexusAdapter) ConvertTools(tools []Tool) ([]interface{}, error) {
	result := make([]interface{}, len(tools))
	for i, tool := range tools {
		converted, err := a.ConvertTool(tool)
		if err != nil {
			return nil, err
		}
		result[i] = converted
	}
	return result, nil
}

// ConvertToolCall converts a tool call response from one format to another.
func (a *ccNexusAdapter) ConvertToolCall(response []byte, sourceFormat string) (*ToolCall, error) {
	switch sourceFormat {
	case "openai":
		return a.parseOpenAIToolCall(response)
	case "anthropic":
		return a.parseAnthropicToolCall(response)
	case "ollama":
		return a.parseOllamaToolCall(response)
	default:
		return a.parseOpenAIToolCall(response)
	}
}

func (a *ccNexusAdapter) parseOpenAIToolCall(response []byte) (*ToolCall, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 || len(resp.Choices[0].Message.ToolCalls) == 0 {
		return nil, nil
	}

	toolCall := resp.Choices[0].Message.ToolCalls[0]
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
		return nil, err
	}

	return &ToolCall{
		Name:  toolCall.Function.Name,
		Input: input,
	}, nil
}

func (a *ccNexusAdapter) parseAnthropicToolCall(response []byte) (*ToolCall, error) {
	var resp struct {
		Content []struct {
			Type  string                 `json:"type"`
			Name  string                 `json:"name"`
			Input map[string]interface{} `json:"input"`
		} `json:"content"`
	}

	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, err
	}

	for _, content := range resp.Content {
		if content.Type == "tool_use" {
			return &ToolCall{
				Name:  content.Name,
				Input: content.Input,
			}, nil
		}
	}

	return nil, nil
}

func (a *ccNexusAdapter) parseOllamaToolCall(response []byte) (*ToolCall, error) {
	var resp struct {
		Message struct {
			ToolCalls []struct {
				Function struct {
					Name      string                 `json:"name"`
					Arguments map[string]interface{} `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}

	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, err
	}

	if len(resp.Message.ToolCalls) == 0 {
		return nil, nil
	}

	toolCall := resp.Message.ToolCalls[0]
	return &ToolCall{
		Name:  toolCall.Function.Name,
		Input: toolCall.Function.Arguments,
	}, nil
}

// BuildToolResult builds a tool result in the target format.
func (a *ccNexusAdapter) BuildToolResult(result *ToolResult) (interface{}, error) {
	switch a.TargetFormat {
	case "openai":
		return a.buildOpenAIToolResult(result), nil
	case "anthropic":
		return a.buildAnthropicToolResult(result), nil
	case "ollama":
		return a.buildOllamaToolResult(result), nil
	default:
		return a.buildOpenAIToolResult(result), nil
	}
}

func (a *ccNexusAdapter) buildOpenAIToolResult(result *ToolResult) map[string]interface{} {
	content := ""
	if result.Success {
		if b, err := json.Marshal(result.Output); err == nil {
			content = string(b)
		}
	} else {
		content = result.Error
	}

	return map[string]interface{}{
		"role":         "tool",
		"tool_call_id": result.Name,
		"content":      content,
	}
}

func (a *ccNexusAdapter) buildAnthropicToolResult(result *ToolResult) map[string]interface{} {
	content := result.Output
	if !result.Success {
		content = map[string]interface{}{
			"error": result.Error,
		}
	}

	return map[string]interface{}{
		"type":       "tool_result",
		"tool_use_id": result.Name,
		"content":    content,
	}
}

func (a *ccNexusAdapter) buildOllamaToolResult(result *ToolResult) map[string]interface{} {
	content := ""
	if result.Success {
		if b, err := json.Marshal(result.Output); err == nil {
			content = string(b)
		}
	} else {
		content = result.Error
	}

	return map[string]interface{}{
		"role":    "tool",
		"content": content,
	}
}
