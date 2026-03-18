package server

import (
	"errors"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

func TestShouldSkipPreContentRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "non proxy error keeps retry",
			err:  errors.New("temporary network hiccup"),
			want: false,
		},
		{
			name: "client error skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 400,
				Body:       "bad request",
			},
			want: true,
		},
		{
			name: "overloaded skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 429,
				Body:       "rate limited",
			},
			want: true,
		},
		{
			name: "no provider skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 503,
				Body:       "no available provider",
			},
			want: true,
		},
		{
			name: "tool unsupported skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 400,
				Body:       "provider does not support tool calls",
			},
			want: true,
		},
		{
			name: "upstream 502 keeps retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       `upstream 502: {"error":{"message":"Upstream request failed","type":"upstream_error"}}`,
			},
			want: false,
		},
		{
			name: "generic 502 keeps retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "bad gateway",
			},
			want: false,
		},
		{
			name: "wrapped overloaded build failure 502 keeps retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       `upstream 500: {"error":{"type":"overloaded_error","message":"构建请求失败"},"type":"error"}`,
			},
			want: false,
		},
		{
			name: "provider no response keeps retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "provider prov_x returned no response",
			},
			want: false,
		},
		{
			name: "relay wrapped context window full skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       `{"error":{"message":"Context window is full. Reduce conversation history, system prompt, or tools."}}`,
			},
			want: true,
		},
		{
			name: "empty streaming response keeps retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "provider prov_x returned empty streaming response",
			},
			want: false,
		},
		{
			name: "context canceled skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       `Post "[server]": context canceled`,
			},
			want: true,
		},
		{
			name: "deadline exceeded skips retry",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "upstream request failed: context deadline exceeded",
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldSkipPreContentRetry(tc.err)
			if got != tc.want {
				t.Fatalf("shouldSkipPreContentRetry() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldSkipToolRoundPreContentRetry(t *testing.T) {
	tests := []struct {
		name        string
		chatReq     llm.ChatRequest
		toolRound   int
		fullContent string
		err         error
		want        bool
	}{
		{
			name:      "not a tool round",
			chatReq:   llm.ChatRequest{},
			toolRound: 0,
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "upstream error",
			},
			want: false,
		},
		{
			name: "tool round with previous_response_id and 5xx skips retry",
			chatReq: llm.ChatRequest{
				PreviousResponseID: "resp_prev_1",
				Messages: []llm.Message{
					{Role: llm.RoleTool, Content: `{"ok":true}`},
				},
			},
			toolRound: 1,
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "upstream error",
			},
			want: true,
		},
		{
			name: "tool round without tool context keeps retry path",
			chatReq: llm.ChatRequest{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "continue"},
				},
			},
			toolRound: 1,
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "upstream error",
			},
			want: false,
		},
		{
			name: "tool round with tool result skips retry",
			chatReq: llm.ChatRequest{
				Messages: []llm.Message{
					{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "call_1", Name: "noop_tool", Arguments: "{}"}}},
					{Role: llm.RoleTool, ToolCallID: "call_1", Content: `{"ok":true}`},
				},
			},
			toolRound: 1,
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "upstream error",
			},
			want: true,
		},
		{
			name: "tool round with non-5xx keeps retry path",
			chatReq: llm.ChatRequest{
				Messages: []llm.Message{
					{Role: llm.RoleTool, Content: `{"ok":true}`},
				},
			},
			toolRound: 1,
			err: &proxybridge.ProxyError{
				StatusCode: 400,
				Body:       "bad request",
			},
			want: false,
		},
		{
			name: "tool round with full content already streamed keeps retry path",
			chatReq: llm.ChatRequest{
				Messages: []llm.Message{
					{Role: llm.RoleTool, Content: `{"ok":true}`},
				},
			},
			toolRound:   1,
			fullContent: "already have content",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "upstream error",
			},
			want: false,
		},
		{
			name: "tool round with context canceled keeps retry path",
			chatReq: llm.ChatRequest{
				Messages: []llm.Message{
					{Role: llm.RoleTool, Content: `{"ok":true}`},
				},
			},
			toolRound: 1,
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "context deadline exceeded",
			},
			want: false,
		},
		{
			name: "tool round with non-proxy error keeps retry path",
			chatReq: llm.ChatRequest{
				Messages: []llm.Message{
					{Role: llm.RoleTool, Content: `{"ok":true}`},
				},
			},
			toolRound: 1,
			err:       errors.New("temporary network hiccup"),
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldSkipToolRoundPreContentRetry(tc.chatReq, tc.toolRound, tc.fullContent, tc.err)
			if got != tc.want {
				t.Fatalf("shouldSkipToolRoundPreContentRetry() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldDisableResponsesContinuationForPreContentRetry(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "non proxy error",
			err:  errors.New("temporary issue"),
			want: false,
		},
		{
			name: "client 4xx does not disable continuation",
			err: &proxybridge.ProxyError{
				StatusCode: 400,
				Body:       "bad request",
			},
			want: false,
		},
		{
			name: "zero chunks disables continuation",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "stream ended with zero chunks",
			},
			want: true,
		},
		{
			name: "empty streaming response disables continuation",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "provider x returned empty streaming response",
			},
			want: true,
		},
		{
			name: "continuation marker disables continuation",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "previous_response_id rejected by upstream",
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldDisableResponsesContinuationForPreContentRetry(tc.err)
			if got != tc.want {
				t.Fatalf("shouldDisableResponsesContinuationForPreContentRetry() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMapStreamErrorCode(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		providerID string
		want       string
	}{
		{
			name: "proxy no provider",
			err: &proxybridge.ProxyError{
				StatusCode: 503,
				Body:       "no available provider",
			},
			want: "provider_unavailable",
		},
		{
			name: "proxy no response",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       "stream ended with zero chunks",
			},
			want: "PROVIDER_NO_RESPONSE",
		},
		{
			name: "plain no response",
			err:  errors.New("provider prov_x returned no response"),
			want: "PROVIDER_NO_RESPONSE",
		},
		{
			name: "plain no provider",
			err:  errors.New("no available provider"),
			want: "provider_unavailable",
		},
		{
			name: "proxy wrapped build failure overload",
			err: &proxybridge.ProxyError{
				StatusCode: 502,
				Body:       `upstream 500: {"error":{"type":"overloaded_error","message":"构建请求失败"},"type":"error"}`,
			},
			want: "STREAM_ERROR",
		},
		{
			name: "plain auth",
			err:  errors.New("upstream status 401 unauthorized"),
			want: "provider_auth_error",
		},
		{
			name: "plain rate limit",
			err:  errors.New("HTTP 429 too many requests"),
			want: "provider_rate_limited",
		},
		{
			name:       "trial provider generic",
			err:        errors.New("temporary upstream failure"),
			providerID: providerpool.TrialProviderID,
			want:       "trial_service_busy",
		},
		{
			name: "unknown fallback",
			err:  errors.New("socket closed unexpectedly"),
			want: "STREAM_ERROR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapStreamErrorCode(tc.err, tc.providerID)
			if got != tc.want {
				t.Fatalf("mapStreamErrorCode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsOpenRouterFreeModelPublicationError(t *testing.T) {
	tests := []struct {
		name string
		err  *proxybridge.ProxyError
		want bool
	}{
		{
			name: "matches openrouter privacy policy 404",
			err: &proxybridge.ProxyError{
				StatusCode: 404,
				Body:       `provider returned 404: {"error":{"message":"No endpoints found matching your data policy (Free model publication). Configure: [server]","code":404}}`,
			},
			want: true,
		},
		{
			name: "does not match generic 404",
			err: &proxybridge.ProxyError{
				StatusCode: 404,
				Body:       `provider returned 404: {"error":{"message":"model not found","code":404}}`,
			},
			want: false,
		},
		{
			name: "does not match nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isOpenRouterFreeModelPublicationError(tc.err)
			if got != tc.want {
				t.Fatalf("isOpenRouterFreeModelPublicationError() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMapStreamErrorCode_OpenRouterFreeModelPublication(t *testing.T) {
	err := &proxybridge.ProxyError{
		StatusCode: 404,
		Body:       `provider returned 404: {"error":{"message":"No endpoints found matching your data policy (Free model publication). Configure: [server]","code":404}}`,
	}
	if got := mapStreamErrorCode(err, ""); got != "provider_openrouter_privacy_policy" {
		t.Fatalf("mapStreamErrorCode() = %q, want %q", got, "provider_openrouter_privacy_policy")
	}
}
