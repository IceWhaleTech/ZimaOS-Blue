package proxy

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
)

// UpstreamRequestBridgeContext carries mutable request-building state.
// Bridges can rewrite path/body according to endpoint or API format requirements.
type UpstreamRequestBridgeContext struct {
	Route              *providerpool.RouteResult
	Provider           *providerpool.Provider
	EffectiveFormat    providerpool.APIFormat
	TargetURL          *url.URL
	RequestPath        string
	Body               []byte
	PromptCacheEnabled bool
	AudioTranscriber   chatAudioTranscriber
}

// UpstreamRequestBridge adapts an OpenAI-edge request into a provider-specific upstream request.
type UpstreamRequestBridge interface {
	Name() string
	Match(*UpstreamRequestBridgeContext) bool
	Build(*UpstreamRequestBridgeContext) error
}

var defaultUpstreamRequestBridges = []UpstreamRequestBridge{
	responsesEndpointBridge{},
	codexModelResponsesBridge{},
	anthropicEndpointBridge{},
	cloudCodeFormatBridge{},
	anthropicFormatBridge{},
}

func (ph *ProxyHandler) applyUpstreamRequestBridges(ctx *UpstreamRequestBridgeContext) {
	for _, bridge := range defaultUpstreamRequestBridges {
		if !bridge.Match(ctx) {
			continue
		}
		if err := bridge.Build(ctx); err != nil {
			slog.Warn("[proxy] upstream bridge failed, sending as-is",
				"provider", ctx.Provider.ID,
				"bridge", bridge.Name(),
				"error", err)
			continue
		}
		slog.Debug("[proxy] upstream bridge applied",
			"provider", ctx.Provider.ID,
			"bridge", bridge.Name(),
			"path", ctx.RequestPath)
	}
}

type responsesEndpointBridge struct{}

func (responsesEndpointBridge) Name() string { return "responses_endpoint" }

func (responsesEndpointBridge) Match(ctx *UpstreamRequestBridgeContext) bool {
	if !isChatCompletionsPath(ctx.RequestPath) {
		return false
	}
	basePath := strings.TrimSuffix(ctx.TargetURL.Path, "/")
	return basePath != "" && strings.HasSuffix(basePath, "/responses")
}

func (responsesEndpointBridge) Build(ctx *UpstreamRequestBridgeContext) error {
	basePath := strings.TrimSuffix(ctx.TargetURL.Path, "/")
	ctx.RequestPath = basePath

	converted, err := convertOpenAIChatCompletionsToResponsesWithAudioTranscriber(ctx.Body, ctx.AudioTranscriber)
	if err != nil {
		return fmt.Errorf("convert openai->responses: %w", err)
	}
	ctx.Body = converted
	ctx.EffectiveFormat = providerpool.APIFormatResponses
	return nil
}

type codexModelResponsesBridge struct{}

func (codexModelResponsesBridge) Name() string { return "codex_model_responses" }

func (codexModelResponsesBridge) Match(ctx *UpstreamRequestBridgeContext) bool {
	if ctx.EffectiveFormat != providerpool.APIFormatOpenAI && ctx.EffectiveFormat != providerpool.APIFormatResponses {
		return false
	}
	if !isChatCompletionsPath(ctx.RequestPath) {
		return false
	}
	if ctx.EffectiveFormat == providerpool.APIFormatResponses {
		return true
	}
	model := strings.ToLower(strings.TrimSpace(gjson.GetBytes(ctx.Body, "model").String()))
	return strings.Contains(model, "codex")
}

func (codexModelResponsesBridge) Build(ctx *UpstreamRequestBridgeContext) error {
	ctx.RequestPath = "/v1/responses"
	converted, err := convertOpenAIChatCompletionsToResponsesWithAudioTranscriber(ctx.Body, ctx.AudioTranscriber)
	if err != nil {
		return fmt.Errorf("convert codex openai->responses: %w", err)
	}
	ctx.Body = converted
	ctx.EffectiveFormat = providerpool.APIFormatResponses
	return nil
}

type anthropicEndpointBridge struct{}

func (anthropicEndpointBridge) Name() string { return "anthropic_endpoint" }

func (anthropicEndpointBridge) Match(ctx *UpstreamRequestBridgeContext) bool {
	if !isChatCompletionsPath(ctx.RequestPath) {
		return false
	}
	basePath := strings.TrimSuffix(ctx.TargetURL.Path, "/")
	return basePath != "" && strings.HasSuffix(basePath, "/anthropic")
}

func (anthropicEndpointBridge) Build(ctx *UpstreamRequestBridgeContext) error {
	converted, newPath, err := sharedConverter.ConvertRequestWithCaching(
		ctx.Body,
		ProviderTypeAnthropic,
		ctx.PromptCacheEnabled,
	)
	if err != nil {
		return fmt.Errorf("convert openai->anthropic for endpoint: %w", err)
	}
	ctx.Body = converted
	ctx.RequestPath = newPath // "/v1/messages"
	// Keep downstream logic in sync so URI fixup does not reconvert Anthropic bodies.
	ctx.EffectiveFormat = providerpool.APIFormatAnthropic
	return nil
}

type cloudCodeFormatBridge struct{}

func (cloudCodeFormatBridge) Name() string { return "cloudcode_format" }

func (cloudCodeFormatBridge) Match(ctx *UpstreamRequestBridgeContext) bool {
	return ctx.EffectiveFormat == providerpool.APIFormatCloudCode
}

func (cloudCodeFormatBridge) Build(ctx *UpstreamRequestBridgeContext) error {
	projectID := "default"
	if ctx.Route != nil && ctx.Route.OAuth != nil && ctx.Route.OAuth.ProjectID != "" {
		projectID = ctx.Route.OAuth.ProjectID
	}

	converted, err := oauth.WrapCloudCodeRequest(ctx.Body, projectID, "blue")
	if err != nil {
		return fmt.Errorf("convert openai->cloudcode: %w", err)
	}
	ctx.Body = converted
	ctx.RequestPath = "/v1internal:generate"
	return nil
}

type anthropicFormatBridge struct{}

func (anthropicFormatBridge) Name() string { return "anthropic_format" }

func (anthropicFormatBridge) Match(ctx *UpstreamRequestBridgeContext) bool {
	return ctx.EffectiveFormat == providerpool.APIFormatAnthropic && isChatCompletionsPath(ctx.RequestPath)
}

func (anthropicFormatBridge) Build(ctx *UpstreamRequestBridgeContext) error {
	converted, newPath, err := sharedConverter.ConvertRequestWithCaching(
		ctx.Body,
		ProviderTypeAnthropic,
		ctx.PromptCacheEnabled,
	)
	if err != nil {
		return fmt.Errorf("convert openai->anthropic by format: %w", err)
	}
	ctx.Body = converted
	ctx.RequestPath = newPath // "/v1/messages"
	return nil
}

func isChatCompletionsPath(path string) bool {
	return strings.HasSuffix(path, "/chat/completions")
}
