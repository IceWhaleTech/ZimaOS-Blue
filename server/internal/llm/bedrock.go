package llm

import (
	"time"
)

const (
	// Default Bedrock endpoint - users should configure their own region-specific endpoint
	defaultBedrockBaseURL = "https://bedrock-runtime.us-east-1.amazonaws.com"
	bedrockTimeout        = 120 * time.Second
)

// BedrockProvider implements the Provider interface for Amazon Bedrock.
// Amazon Bedrock supports OpenAI-compatible API through the Converse API.
type BedrockProvider struct {
	*OpenAIProvider
}

// NewBedrockProvider creates a new Amazon Bedrock provider.
func NewBedrockProvider(apiKey, baseURL string) *BedrockProvider {
	if baseURL == "" {
		baseURL = defaultBedrockBaseURL
	}

	openaiProvider := NewOpenAIProvider(apiKey, baseURL)

	return &BedrockProvider{
		OpenAIProvider: openaiProvider,
	}
}

// Name returns the provider name.
func (p *BedrockProvider) Name() string {
	return "bedrock"
}

// getDefaultModels returns the default model list for Amazon Bedrock.
func (p *BedrockProvider) getDefaultModels() []string {
	return []string{
		"anthropic.claude-3-5-sonnet-20241022-v2:0",
		"anthropic.claude-3-5-haiku-20241022-v1:0",
		"anthropic.claude-3-opus-20240229-v1:0",
		"anthropic.claude-3-sonnet-20240229-v1:0",
		"anthropic.claude-3-haiku-20240307-v1:0",
		"meta.llama3-3-70b-instruct-v1:0",
		"meta.llama3-2-90b-instruct-v1:0",
		"meta.llama3-2-11b-instruct-v1:0",
		"meta.llama3-2-3b-instruct-v1:0",
		"meta.llama3-2-1b-instruct-v1:0",
		"amazon.nova-pro-v1:0",
		"amazon.nova-lite-v1:0",
		"amazon.nova-micro-v1:0",
		"mistral.mistral-large-2407-v1:0",
		"cohere.command-r-plus-v1:0",
	}
}

// Models returns the list of available models.
func (p *BedrockProvider) Models() []string {
	// Try to fetch models from API
	models := p.fetchModels()
	if len(models) > 0 {
		return models
	}
	// Return default fallback list for Amazon Bedrock
	return p.getDefaultModels()
}
