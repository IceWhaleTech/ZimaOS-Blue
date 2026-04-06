package proxy

import (
	"testing"
)

func TestAnthropicToolResultStringSupportsRawJSONMessage(t *testing.T) {
	got := anthropicToolResultString(rawJSONMessage(`{"ok":true,"count":2}`))
	if got != `{"ok":true,"count":2}` {
		t.Fatalf("anthropicToolResultString(raw json) = %q, want %q", got, `{"ok":true,"count":2}`)
	}
}

func TestDetectProviderType(t *testing.T) {
	fc := NewFormatConverter()

	tests := []struct {
		endpoint string
		expected ProviderType
	}{
		{"https://api.anthropic.com", ProviderTypeAnthropic},
		{"https://api.openai.com", ProviderTypeOpenAI},
		{"https://generativelanguage.googleapis.com", ProviderTypeGemini},
		{"https://aiplatform.googleapis.com", ProviderTypeGemini},
		{"https://api.deepseek.com", ProviderTypeOpenAI},
		{"https://localhost:11434", ProviderTypeOpenAI},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			got := fc.DetectProviderType(tt.endpoint)
			if got != tt.expected {
				t.Errorf("DetectProviderType(%s) = %v, want %v", tt.endpoint, got, tt.expected)
			}
		})
	}
}

func TestConvertRequestToAnthropic(t *testing.T) {
	fc := NewFormatConverter()

	openaiReq := `{
		"model": "claude-3-5-sonnet",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello"}
		],
		"max_tokens": 1024,
		"stream": false
	}`

	converted, path, err := fc.ConvertRequest([]byte(openaiReq), ProviderTypeAnthropic)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	if path != "/v1/messages" {
		t.Errorf("path = %s, want /v1/messages", path)
	}

	var anthropicReq AnthropicRequest
	if err := json.Unmarshal(converted, &anthropicReq); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	switch s := anthropicReq.System.(type) {
	case string:
		if s != "You are helpful." {
			t.Errorf("System = %q, want 'You are helpful.'", s)
		}
	case []interface{}:
		if len(s) != 1 {
			t.Fatalf("System blocks = %d, want 1", len(s))
		}
		block, _ := s[0].(map[string]interface{})
		if block == nil || block["text"] != "You are helpful." {
			t.Errorf("System block text = %v, want 'You are helpful.'", block["text"])
		}
	default:
		t.Fatalf("unexpected System type: %T", anthropicReq.System)
	}
	if len(anthropicReq.Messages) != 1 {
		t.Errorf("Messages count = %d, want 1", len(anthropicReq.Messages))
	}
	if anthropicReq.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d, want 1024", anthropicReq.MaxTokens)
	}
}

func TestConvertRequestToGemini(t *testing.T) {
	fc := NewFormatConverter()

	openaiReq := `{
		"model": "gemini-1.5-pro",
		"messages": [
			{"role": "system", "content": "Be concise."},
			{"role": "user", "content": "Hi"}
		],
		"max_tokens": 512,
		"stream": true
	}`

	converted, path, err := fc.ConvertRequest([]byte(openaiReq), ProviderTypeGemini)
	if err != nil {
		t.Fatalf("ConvertRequest failed: %v", err)
	}

	if path != "/v1beta/models/gemini-1.5-pro:streamGenerateContent?alt=sse" {
		t.Errorf("path = %s, want streaming path", path)
	}

	var geminiReq GeminiRequest
	if err := json.Unmarshal(converted, &geminiReq); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if geminiReq.SystemInstruction == nil {
		t.Error("SystemInstruction is nil")
	}
	if len(geminiReq.Contents) != 1 {
		t.Errorf("Contents count = %d, want 1", len(geminiReq.Contents))
	}
}

func TestApplyPromptCaching_SortsAnthropicToolsBeforeAnnotating(t *testing.T) {
	req := &AnthropicRequest{
		Tools: []AnthropicTool{
			{Name: "write"},
			{Name: "ask"},
			{Name: "read"},
		},
	}

	ApplyPromptCaching(req)

	if got := req.Tools[0].Name; got != "ask" {
		t.Fatalf("tool[0] = %q, want %q", got, "ask")
	}
	if got := req.Tools[1].Name; got != "read" {
		t.Fatalf("tool[1] = %q, want %q", got, "read")
	}
	if got := req.Tools[2].Name; got != "write" {
		t.Fatalf("tool[2] = %q, want %q", got, "write")
	}
	if req.Tools[2].CacheControl == nil || req.Tools[2].CacheControl.Type != "ephemeral" {
		t.Fatalf("expected last sorted tool to receive cache_control, got=%+v", req.Tools[2].CacheControl)
	}
}

func TestConvertAnthropicResponse(t *testing.T) {
	fc := NewFormatConverter()

	anthropicResp := `{
		"id": "msg_123",
		"type": "message",
		"role": "assistant",
		"content": [{"type": "text", "text": "Hello!"}],
		"model": "claude-3-5-sonnet-20241022",
		"stop_reason": "end_turn",
		"usage": {"input_tokens": 10, "output_tokens": 5}
	}`

	converted, err := fc.ConvertResponse([]byte(anthropicResp), ProviderTypeAnthropic)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	var openaiResp OpenAIChatResponse
	if err := json.Unmarshal(converted, &openaiResp); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if openaiResp.Object != "chat.completion" {
		t.Errorf("Object = %s, want chat.completion", openaiResp.Object)
	}
	if len(openaiResp.Choices) != 1 {
		t.Fatalf("Choices count = %d, want 1", len(openaiResp.Choices))
	}
	if openaiResp.Choices[0].Message.Content != "Hello!" {
		t.Errorf("Content = %s, want 'Hello!'", openaiResp.Choices[0].Message.Content)
	}
	if openaiResp.Choices[0].FinishReason != "stop" {
		t.Errorf("FinishReason = %s, want stop", openaiResp.Choices[0].FinishReason)
	}
}

func TestConvertGeminiResponse(t *testing.T) {
	fc := NewFormatConverter()

	geminiResp := `{
		"candidates": [{
			"content": {"role": "model", "parts": [{"text": "Hi there!"}]},
			"finishReason": "STOP"
		}],
		"usageMetadata": {"promptTokenCount": 5, "candidatesTokenCount": 3, "totalTokenCount": 8}
	}`

	converted, err := fc.ConvertResponse([]byte(geminiResp), ProviderTypeGemini)
	if err != nil {
		t.Fatalf("ConvertResponse failed: %v", err)
	}

	var openaiResp OpenAIChatResponse
	if err := json.Unmarshal(converted, &openaiResp); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if openaiResp.Choices[0].Message.Content != "Hi there!" {
		t.Errorf("Content = %s, want 'Hi there!'", openaiResp.Choices[0].Message.Content)
	}
	if openaiResp.Usage.TotalTokens != 8 {
		t.Errorf("TotalTokens = %d, want 8", openaiResp.Usage.TotalTokens)
	}
}

func TestConvertGeminiModels(t *testing.T) {
	fc := NewFormatConverter()

	geminiModels := `{
		"models": [
			{"name": "models/gemini-1.5-pro", "displayName": "Gemini 1.5 Pro"},
			{"name": "models/gemini-1.5-flash", "displayName": "Gemini 1.5 Flash"}
		]
	}`

	converted, err := fc.ConvertModelsResponse([]byte(geminiModels), ProviderTypeGemini)
	if err != nil {
		t.Fatalf("ConvertModelsResponse failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(converted, &result); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if result["object"] != "list" {
		t.Errorf("object = %v, want list", result["object"])
	}

	data := result["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("data count = %d, want 2", len(data))
	}

	first := data[0].(map[string]interface{})
	if first["id"] != "gemini-1.5-pro" {
		t.Errorf("id = %v, want gemini-1.5-pro", first["id"])
	}
}

func TestModelMapping(t *testing.T) {
	fc := NewFormatConverter()

	tests := []struct {
		input    string
		expected string
	}{
		{"gpt-4o", "claude-3-5-sonnet-20241022"},
		{"gpt-4o-mini", "claude-3-5-haiku-20241022"},
		{"claude-3-5-sonnet", "claude-3-5-sonnet"}, // Claude names pass through as-is
		{"custom-model", "custom-model"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := fc.convertModel(tt.input)
			if got != tt.expected {
				t.Errorf("convertModel(%s) = %s, want %s", tt.input, got, tt.expected)
			}
		})
	}
}
