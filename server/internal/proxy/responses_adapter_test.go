package proxy

import (
	"context"
	gojson "encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

type fixedResponsesContextCompressor struct {
	output string
}

func (f fixedResponsesContextCompressor) CompressAssistantContext(ResponsesAssistantCompressionInput) (string, error) {
	return f.output, nil
}

func TestConvertOpenAIChatCompletionsToResponses(t *testing.T) {
	body := []byte(`{
		"model":"o3",
		"stream":true,
		"max_tokens":77,
		"temperature":0.3,
		"messages":[
			{"role":"system","content":"system prompt"},
			{"role":"user","content":"hello"},
			{"role":"assistant","content":"","tool_calls":[{"id":"call_1","type":"function","function":{"name":"exec","arguments":"{\"cmd\":\"ls\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":{"ok":true}},
			{"role":"user","content":[{"type":"text","text":"next"}]}
		],
		"tools":[
			{"type":"function","function":{"name":"exec","description":"run command","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
	if got := gjson.GetBytes(converted, "input.0.role").String(); got != "system" {
		t.Fatalf("input.0.role = %q, want %q", got, "system")
	}
	if got := gjson.GetBytes(converted, "input.0.content.0.text").String(); got != "system prompt" {
		t.Fatalf("input.0.content.0.text = %q, want %q", got, "system prompt")
	}
	if got := gjson.GetBytes(converted, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(converted, "input.2.type").String(); got != "function_call" {
		t.Fatalf("input.2.type = %q, want %q", got, "function_call")
	}
	if got := gjson.GetBytes(converted, "input.2.call_id").String(); got != "call_1" {
		t.Fatalf("input.2.call_id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(converted, "input.2.name").String(); got != "exec" {
		t.Fatalf("input.2.name = %q, want %q", got, "exec")
	}
	if got := gjson.GetBytes(converted, "input.2.arguments").String(); got != `{"cmd":"ls"}` {
		t.Fatalf("input.2.arguments = %q, want %q", got, `{"cmd":"ls"}`)
	}
	if got := gjson.GetBytes(converted, "input.3.type").String(); got != "function_call_output" {
		t.Fatalf("input.3.type = %q, want %q", got, "function_call_output")
	}
	if got := gjson.GetBytes(converted, "input.3.call_id").String(); got != "call_1" {
		t.Fatalf("input.3.call_id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(converted, "input.3.output").String(); got != `{"ok":true}` {
		t.Fatalf("input.3.output = %q, want %q", got, `{"ok":true}`)
	}
	if got := gjson.GetBytes(converted, "tools.0.type").String(); got != "function" {
		t.Fatalf("tools.0.type = %q, want %q", got, "function")
	}
	if got := gjson.GetBytes(converted, "tools.0.name").String(); got != "exec" {
		t.Fatalf("tools.0.name = %q, want %q", got, "exec")
	}
	if got := gjson.GetBytes(converted, "max_output_tokens").Int(); got != 77 {
		t.Fatalf("max_output_tokens = %d, want 77", got)
	}
	if got := gjson.GetBytes(converted, "stream").Bool(); !got {
		t.Fatal("stream = false, want true")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_StripsTopLevelCompositeKeywordsFromToolSchema(t *testing.T) {
	body := []byte(`{
		"model":"o3",
		"messages":[{"role":"user","content":"hello"}],
		"tools":[
			{"type":"function","function":{"name":"deep_research","parameters":{"type":"object","anyOf":[{"required":["query"]},{"required":["job_id"]}]}}}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if gjson.GetBytes(converted, "tools.0.parameters.anyOf").Exists() {
		t.Fatalf("top-level anyOf should be stripped for OpenAI-compatible tool schemas: %s", string(converted))
	}
	if got := gjson.GetBytes(converted, "tools.0.parameters.required.#").Int(); got != 0 {
		t.Fatalf("len(tools.0.parameters.required) = %d, want 0", got)
	}
}

func TestConvertOpenAIChatCompletionsToResponses_ContinuationUsesIncrementalMessages(t *testing.T) {
	body := []byte(`{
		"model":"o3",
		"previous_response_id":"resp_prev_123",
		"messages":[
			{"role":"system","content":"system prompt should not be resent"},
			{"role":"user","content":"old user"},
			{"role":"assistant","content":"very long previous assistant answer"},
			{"role":"user","content":"new followup"}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "previous_response_id").String(); got != "resp_prev_123" {
		t.Fatalf("previous_response_id = %q, want %q", got, "resp_prev_123")
	}
	if got := gjson.GetBytes(converted, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
	if gjson.GetBytes(converted, "instructions").Exists() {
		t.Fatalf("instructions should be omitted for continuation payload: %s", string(converted))
	}
	if got := gjson.GetBytes(converted, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2", got)
	}
	if got := gjson.GetBytes(converted, "input.0.role").String(); got != "assistant" {
		t.Fatalf("input.0.role = %q, want %q", got, "assistant")
	}
	if got := gjson.GetBytes(converted, "input.0.content.0.text").String(); got != "very long previous assistant answer" {
		t.Fatalf("input.0.content.0.text = %q, want %q", got, "very long previous assistant answer")
	}
	if got := gjson.GetBytes(converted, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(converted, "input.1.content.0.text").String(); got != "new followup" {
		t.Fatalf("input.1.content.0.text = %q, want %q", got, "new followup")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_ContinuationSkipsAssistantToolCallEcho(t *testing.T) {
	body := []byte(`{
		"model":"o3",
		"previous_response_id":"resp_prev_tool_1",
		"messages":[
			{"role":"assistant","content":"正在执行工具调用","tool_calls":[{"id":"call_1","type":"function","function":{"name":"exec","arguments":"{\"cmd\":\"ls\"}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"{\"ok\":true}"},
			{"role":"user","content":"继续"}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(converted))
	}
	if got := gjson.GetBytes(converted, "input.0.type").String(); got != "function_call_output" {
		t.Fatalf("input.0.type = %q, want %q", got, "function_call_output")
	}
	if got := gjson.GetBytes(converted, "input.0.call_id").String(); got != "call_1" {
		t.Fatalf("input.0.call_id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(converted, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(converted, "input.1.content.0.text").String(); got != "继续" {
		t.Fatalf("input.1.content.0.text = %q, want %q", got, "继续")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_ContinuationKeepsAssistantForShortChoiceReply(t *testing.T) {
	body := []byte(`{
		"model":"o3",
		"previous_response_id":"resp_prev_choice_1",
		"messages":[
			{"role":"user","content":"请根据上面的选项选择一个答案"},
			{"role":"assistant","content":"A) 方案一\nB) 方案二\nC) 方案三"},
			{"role":"user","content":"B"}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(converted))
	}
	if got := gjson.GetBytes(converted, "input.0.role").String(); got != "assistant" {
		t.Fatalf("input.0.role = %q, want %q", got, "assistant")
	}
	if got := gjson.GetBytes(converted, "input.0.content.0.text").String(); got != "A) 方案一\nB) 方案二\nC) 方案三" {
		t.Fatalf("input.0.content.0.text = %q, want assistant choices", got)
	}
	if got := gjson.GetBytes(converted, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(converted, "input.1.content.0.text").String(); got != "B" {
		t.Fatalf("input.1.content.0.text = %q, want %q", got, "B")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_ContinuationKeepsExplicitInstructions(t *testing.T) {
	body := []byte(`{
		"model":"o3",
		"previous_response_id":"resp_prev_123",
		"instructions":"updated instructions",
		"messages":[
			{"role":"system","content":"old system should not be resent as input"},
			{"role":"assistant","content":"old assistant"},
			{"role":"user","content":"new followup"}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	if got := gjson.GetBytes(converted, "previous_response_id").String(); got != "resp_prev_123" {
		t.Fatalf("previous_response_id = %q, want %q", got, "resp_prev_123")
	}
	if got := gjson.GetBytes(converted, "instructions").String(); got != "updated instructions" {
		t.Fatalf("instructions = %q, want %q", got, "updated instructions")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_PreservesMaxOutputTokens(t *testing.T) {
	body := []byte(`{
		"model":"o3",
		"max_tokens":16384,
		"messages":[{"role":"user","content":"hello"}]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "max_output_tokens").Int(); got != 16384 {
		t.Fatalf("max_output_tokens = %d, want %d", got, 16384)
	}
}

func TestConvertOpenAIChatCompletionsToResponses_MapsAdvancedFields(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4.1",
		"max_tokens":999,
		"max_completion_tokens":321,
		"tool_choice":{"type":"function","function":{"name":"exec"}},
		"parallel_tool_calls":false,
		"response_format":{
			"type":"json_schema",
			"json_schema":{
				"name":"task_result",
				"schema":{"type":"object","properties":{"ok":{"type":"boolean"}}},
				"strict":true
			}
		},
		"reasoning_effort":"high",
		"user":"user_123",
		"safety_identifier":"safe_user_1",
		"prompt_cache_key":"pcache-key-1",
		"metadata":{"trace_id":"abc123","source":"blue"},
		"messages":[{"role":"user","content":"hello"}]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "max_output_tokens").Int(); got != 321 {
		t.Fatalf("max_output_tokens = %d, want %d", got, 321)
	}
	if got := gjson.GetBytes(converted, "tool_choice.type").String(); got != "function" {
		t.Fatalf("tool_choice.type = %q, want %q", got, "function")
	}
	if got := gjson.GetBytes(converted, "tool_choice.function.name").String(); got != "exec" {
		t.Fatalf("tool_choice.function.name = %q, want %q", got, "exec")
	}
	if got := gjson.GetBytes(converted, "parallel_tool_calls"); !got.Exists() || got.Bool() {
		t.Fatalf("parallel_tool_calls = %v (exists=%v), want false", got.Bool(), got.Exists())
	}
	if got := gjson.GetBytes(converted, "text.format.type").String(); got != "json_schema" {
		t.Fatalf("text.format.type = %q, want %q", got, "json_schema")
	}
	if got := gjson.GetBytes(converted, "text.format.name").String(); got != "task_result" {
		t.Fatalf("text.format.name = %q, want %q", got, "task_result")
	}
	if got := gjson.GetBytes(converted, "text.format.schema.type").String(); got != "object" {
		t.Fatalf("text.format.schema.type = %q, want %q", got, "object")
	}
	if got := gjson.GetBytes(converted, "text.format.strict").Bool(); !got {
		t.Fatalf("text.format.strict = false, want true")
	}
	if got := gjson.GetBytes(converted, "reasoning.effort").String(); got != "high" {
		t.Fatalf("reasoning.effort = %q, want %q", got, "high")
	}
	if got := gjson.GetBytes(converted, "user").String(); got != "user_123" {
		t.Fatalf("user = %q, want %q", got, "user_123")
	}
	if got := gjson.GetBytes(converted, "safety_identifier").String(); got != "safe_user_1" {
		t.Fatalf("safety_identifier = %q, want %q", got, "safe_user_1")
	}
	if got := gjson.GetBytes(converted, "prompt_cache_key").String(); got != "pcache-key-1" {
		t.Fatalf("prompt_cache_key = %q, want %q", got, "pcache-key-1")
	}
	if got := gjson.GetBytes(converted, "metadata.trace_id").String(); got != "abc123" {
		t.Fatalf("metadata.trace_id = %q, want %q", got, "abc123")
	}
	if got := gjson.GetBytes(converted, "metadata.source").String(); got != "blue" {
		t.Fatalf("metadata.source = %q, want %q", got, "blue")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_MapsJSONResponseFormat(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4.1",
		"response_format":{"type":"json_object"},
		"messages":[{"role":"user","content":"hello"}]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "text.format.type").String(); got != "json_object" {
		t.Fatalf("text.format.type = %q, want %q", got, "json_object")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_MapsReasoningEffortExtraHighAlias(t *testing.T) {
	body := []byte(`{
		"model":"gpt-4.1",
		"reasoning_effort":"extrahigh",
		"messages":[{"role":"user","content":"hello"}]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "reasoning.effort").String(); got != "xhigh" {
		t.Fatalf("reasoning.effort = %q, want %q", got, "xhigh")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_MultimodalAndStructuredInput(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5",
		"messages":[
			{"role":"system","content":"Follow instructions"},
			{"role":"user","content":[
				{"type":"text","text":"describe this"},
				{"type":"image_url","image_url":{"url":"https://example.com/a.png"}},
				{"type":"input_audio","input_audio":{"data":"AAA","format":"wav"}},
				{"type":"text","text":""},
				{"type":"image_url","image_url":""}
			]}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
	if got := gjson.GetBytes(converted, "input.0.role").String(); got != "system" {
		t.Fatalf("input.0.role = %q, want %q", got, "system")
	}
	if got := gjson.GetBytes(converted, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(converted, "input.1.content.#").Int(); got != 3 {
		t.Fatalf("input.1.content length = %d, want 3", got)
	}
	if got := gjson.GetBytes(converted, "input.1.content.0.type").String(); got != "input_text" {
		t.Fatalf("input.1.content.0.type = %q, want %q", got, "input_text")
	}
	if got := gjson.GetBytes(converted, "input.1.content.1.type").String(); got != "input_image" {
		t.Fatalf("input.1.content.1.type = %q, want %q", got, "input_image")
	}
	if got := gjson.GetBytes(converted, "input.1.content.2.type").String(); got != "input_audio" {
		t.Fatalf("input.1.content.2.type = %q, want %q", got, "input_audio")
	}
	if got := gjson.GetBytes(converted, "input.1.content.2.input_audio.format").String(); got != "wav" {
		t.Fatalf("input.1.content.2.input_audio.format = %q, want %q", got, "wav")
	}
}

func TestConvertOpenAIChatCompletionsToResponses_WithAudioTranscriber(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5",
		"messages":[
			{"role":"user","content":[
				{"type":"input_audio","input_audio":{"data":"AAA","format":"wav"}}
			]}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponsesWithAudioTranscriber(body, func(inputAudio any) (string, bool) {
		m, ok := inputAudio.(map[string]interface{})
		if !ok {
			t.Fatalf("inputAudio type = %T, want map[string]interface{}", inputAudio)
		}
		if got := anyToString(m["format"]); got != "wav" {
			t.Fatalf("input_audio.format = %q, want %q", got, "wav")
		}
		return "transcribed text", true
	})
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "input.0.content.0.type").String(); got != "input_text" {
		t.Fatalf("input.0.content.0.type = %q, want %q", got, "input_text")
	}
	if got := gjson.GetBytes(converted, "input.0.content.0.text").String(); got != "transcribed text" {
		t.Fatalf("input.0.content.0.text = %q, want %q", got, "transcribed text")
	}
	if gjson.GetBytes(converted, "input.0.content.0.input_audio").Exists() {
		t.Fatalf("input_audio should be removed after transcription: %s", string(converted))
	}
}

func TestConvertOpenAIChatCompletionsToResponses_DropsInvalidStructuredItems(t *testing.T) {
	body := []byte(`{
		"model":"gpt-5",
		"messages":[
			{"role":"user","content":[{"type":"text","text":""}]},
			{"role":"assistant","content":"","tool_calls":[{"id":"call_bad","type":"function","function":{"name":"","arguments":"{}"}}]},
			{"role":"tool","content":"ignored because missing tool_call_id"}
		]
	}`)

	converted, err := convertOpenAIChatCompletionsToResponses(body)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if gjson.GetBytes(converted, "input").Exists() {
		t.Fatalf("input should be omitted when all items are invalid, got: %s", string(converted))
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesEndpointConvertsBody(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "p1",
			BaseURL:   "https://chatgpt.com/backend-api/codex/responses",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := []byte(`{"model":"o3","messages":[{"role":"user","content":"hi"}],"max_tokens":5}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/backend-api/codex/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/backend-api/codex/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if !gjson.GetBytes(convertedBody, "input").Exists() {
		t.Fatalf("converted body missing input: %s", string(convertedBody))
	}
	if gjson.GetBytes(convertedBody, "messages").Exists() {
		t.Fatalf("converted body still contains messages: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 5 {
		t.Fatalf("max_output_tokens = %d, want 5", got)
	}
	if got := gjson.GetBytes(convertedBody, "store").Bool(); got {
		t.Fatalf("store = true, want false for codex fixed endpoint (no explicit store in request → default is false)")
	}
	if !gjson.GetBytes(convertedBody, "include").Exists() {
		t.Fatalf("store=false without include for stateless request: want include:[\"reasoning.encrypted_content\"]")
	}
}

func TestBuildUpstreamRequestWithFormat_ForcesIdentityAcceptEncoding(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "p1",
			BaseURL:   "https://api.openai.com",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)

	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	if got := upstreamReq.Header.Get("Accept-Encoding"); got != "identity" {
		t.Fatalf("accept-encoding = %q, want %q", got, "identity")
	}
}

func TestBuildUpstreamRequestWithFormat_AnthropicEndpointConvertsBody(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "minimax-like",
			BaseURL:   "https://api.minimaxi.com/anthropic",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := []byte(`{"model":"MiniMax-M1","messages":[{"role":"system","content":"You are helpful."},{"role":"user","content":"hi"}],"max_tokens":16}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/anthropic/v1/messages" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/anthropic/v1/messages")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	var anthropicReq AnthropicRequest
	if err := gojson.Unmarshal(convertedBody, &anthropicReq); err != nil {
		t.Fatalf("converted body is not anthropic request: %v, body=%s", err, string(convertedBody))
	}
	if anthropicReq.System == nil {
		t.Fatalf("expected system prompt in anthropic request, got nil")
	}
	if len(anthropicReq.Messages) != 1 {
		t.Fatalf("anthropic messages len = %d, want 1", len(anthropicReq.Messages))
	}
}

func TestBuildUpstreamRequestWithFormat_CodexModelUsesResponsesOnOpenAICompatProvider(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"hi"}],"max_tokens":9}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/v1/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/v1/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if !gjson.GetBytes(convertedBody, "input").Exists() {
		t.Fatalf("converted body missing input: %s", string(convertedBody))
	}
	if gjson.GetBytes(convertedBody, "messages").Exists() {
		t.Fatalf("converted body unexpectedly contains messages: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 9 {
		t.Fatalf("max_output_tokens = %d, want 9", got)
	}
	if got := gjson.GetBytes(convertedBody, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesFormatUsesResponsesEndpoint(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-responses",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatResponses,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"hi"}],"max_tokens":9}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/v1/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/v1/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if !gjson.GetBytes(convertedBody, "input").Exists() {
		t.Fatalf("converted body missing input: %s", string(convertedBody))
	}
	if gjson.GetBytes(convertedBody, "messages").Exists() {
		t.Fatalf("converted body still contains messages: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 9 {
		t.Fatalf("max_output_tokens = %d, want 9", got)
	}
	if got := gjson.GetBytes(convertedBody, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesPathConvertsMessagesEvenWhenFormatIsResponses(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"hi"}],"max_tokens":9}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/v1/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/v1/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if !gjson.GetBytes(convertedBody, "input").Exists() {
		t.Fatalf("converted body missing input: %s", string(convertedBody))
	}
	if gjson.GetBytes(convertedBody, "messages").Exists() {
		t.Fatalf("converted body still contains messages: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 9 {
		t.Fatalf("max_output_tokens = %d, want 9", got)
	}
	if got := gjson.GetBytes(convertedBody, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
}

func TestBuildUpstreamRequestWithFormat_GenericResponsesPathForcesStoreTrue(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","store":false,"input":[{"role":"user","content":[{"type":"input_text","text":"hello"}]}]}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	// Client explicitly set store:false - we now respect it and inject include per spec
	if got := gjson.GetBytes(convertedBody, "store").Bool(); got {
		t.Fatalf("store = true, want false (client explicit preference should be respected)")
	}
	if !gjson.GetBytes(convertedBody, "include").Exists() {
		t.Fatalf("store=false without include: want include:[\"reasoning.encrypted_content\"]")
	}
}

func TestBuildUpstreamRequestWithFormat_DisableResponsesContinuationDropsPreviousResponseID(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req.Header.Set(DisableResponsesContinuationHeader, "1")
	body := []byte(`{"model":"gpt-5.3-codex-spark","previous_response_id":"resp_old_1","messages":[{"role":"user","content":"continue"}]}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "previous_response_id"); got.Exists() {
		t.Fatalf("previous_response_id should be removed when continuation is disabled, got: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesPathExtractsInstructionsFromMessages(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-instr-1"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"messages":[
			{"role":"system","content":"You are system."},
			{"role":"developer","content":"Follow dev rules."},
			{"role":"user","content":"hello"}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "instructions").String(); got != "You are system.\n\nFollow dev rules." {
		t.Fatalf("instructions = %q, want merged system+developer", got)
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "user" {
		t.Fatalf("input.0.role = %q, want %q", got, "user")
	}
	if got := ph.getCachedResponsesInstructions(req); got != "You are system.\n\nFollow dev rules." {
		t.Fatalf("cached instructions = %q, want merged system+developer", got)
	}
}

func TestBuildUpstreamRequestWithFormat_DisableContinuationKeepsCachedInstructions(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	baseReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	baseReq = baseReq.WithContext(WithSessionID(context.Background(), "sess-instr-2"))
	ph.setCachedResponsesInstructions(baseReq, "cached global instructions")
	ph.setCachedResponsesPreviousID(baseReq, "resp_prev_old")

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-instr-2"))
	req.Header.Set(DisableResponsesContinuationHeader, "1")
	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"retry without continuation"}]}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "previous_response_id"); got.Exists() {
		t.Fatalf("previous_response_id should be removed when continuation is disabled, got: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "instructions").String(); got != "cached global instructions" {
		t.Fatalf("instructions = %q, want cached value", got)
	}
	if got := ph.getCachedResponsesPreviousID(req); got != "" {
		t.Fatalf("cached previous_response_id = %q, want empty", got)
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationUnchangedInstructionsNotResent(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	baseReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	baseReq = baseReq.WithContext(WithSessionID(context.Background(), "sess-instr-3"))
	ph.setCachedResponsesInstructions(baseReq, "stable instructions")
	ph.setCachedResponsesPreviousIDForRoute(baseReq, result, "resp_prev_3")

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-instr-3"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"messages":[
			{"role":"system","content":"stable instructions"},
			{"role":"user","content":"continue"}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "previous_response_id").String(); got != "resp_prev_3" {
		t.Fatalf("previous_response_id = %q, want %q", got, "resp_prev_3")
	}
	if got := gjson.GetBytes(convertedBody, "instructions"); got.Exists() {
		t.Fatalf("instructions should be omitted when unchanged on continuation, got: %s", string(convertedBody))
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationInjectedPrevIDTrimsResponsesInput(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	seedReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	seedReq = seedReq.WithContext(WithSessionID(context.Background(), "sess-prev-trim-1"))
	ph.setCachedResponsesPreviousIDForRoute(seedReq, result, "resp_prev_trim_1")

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-prev-trim-1"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"Reply with ONLY: OK"}]},
			{"role":"assistant","content":[{"type":"input_text","text":"OK"}]},
			{"role":"user","content":[{"type":"input_text","text":"Reply with ONLY: NEXT"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "previous_response_id").String(); got != "resp_prev_trim_1" {
		t.Fatalf("previous_response_id = %q, want %q", got, "resp_prev_trim_1")
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "assistant" {
		t.Fatalf("input.0.role = %q, want %q", got, "assistant")
	}
	if got := gjson.GetBytes(convertedBody, "input.0.content.0.text").String(); got != "OK" {
		t.Fatalf("input.0.content.0.text = %q, want %q", got, "OK")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.content.0.text").String(); got != "Reply with ONLY: NEXT" {
		t.Fatalf("input.1.content.0.text = %q, want %q", got, "Reply with ONLY: NEXT")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationExistingPrevIDTrimsResponsesInput(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_1",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"A"}]},
			{"role":"assistant","content":[{"type":"input_text","text":"B"}]},
			{"role":"user","content":[{"type":"input_text","text":"C"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "previous_response_id").String(); got != "resp_existing_1" {
		t.Fatalf("previous_response_id = %q, want %q", got, "resp_existing_1")
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "assistant" {
		t.Fatalf("input.0.role = %q, want %q", got, "assistant")
	}
	if got := gjson.GetBytes(convertedBody, "input.0.content.0.text").String(); got != "B" {
		t.Fatalf("input.0.content.0.text = %q, want %q", got, "B")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.content.0.text").String(); got != "C" {
		t.Fatalf("input.1.content.0.text = %q, want %q", got, "C")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationExistingPrevIDKeepsAssistantForShortChoice(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_choice_1",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"请从选项里选一个"}]},
			{"role":"assistant","content":[{"type":"input_text","text":"A) 苹果 B) 香蕉 C) 梨"}]},
			{"role":"user","content":[{"type":"input_text","text":"A"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "assistant" {
		t.Fatalf("input.0.role = %q, want %q", got, "assistant")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.content.0.text").String(); got != "A" {
		t.Fatalf("input.1.content.0.text = %q, want %q", got, "A")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationInjectsCachedAssistantWhenInputLacksContext(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	seedReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	seedReq = seedReq.WithContext(WithSessionID(context.Background(), "sess-assist-cache-1"))
	ph.setCachedResponsesAssistantForRoute(seedReq, result, "A) 选项一 B) 选项二")

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-assist-cache-1"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_choice_cache_1",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"1"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "assistant" {
		t.Fatalf("input.0.role = %q, want %q", got, "assistant")
	}
	if got := gjson.GetBytes(convertedBody, "input.0.content.0.text").String(); got != "A) 选项一 B) 选项二" {
		t.Fatalf("input.0.content.0.text = %q, want cached assistant text", got)
	}
	if got := gjson.GetBytes(convertedBody, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.content.0.text").String(); got != "1" {
		t.Fatalf("input.1.content.0.text = %q, want %q", got, "1")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationSkipsAssistantForSubstantiveFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_long_1",
		"input":[
			{"role":"assistant","content":[{"type":"input_text","text":"A) 苹果 B) 香蕉 C) 梨"}]},
			{"role":"user","content":[{"type":"input_text","text":"请给我一个详细比较，重点说明口感、甜度、储存方式和价格区间，再给出最终建议。"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "user" {
		t.Fatalf("input.0.role = %q, want %q", got, "user")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesPathChatPayloadContinuationSkipsAssistantForSubstantiveFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_long_messages_1",
		"messages":[
			{"role":"assistant","content":"A) 苹果 B) 香蕉 C) 梨"},
			{"role":"user","content":"请给我一个完整的独立方案，包含目标、实施步骤、风险和验收标准。"}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "user" {
		t.Fatalf("input.0.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(convertedBody, "messages").Exists(); got {
		t.Fatalf("messages should be converted out for /responses path, body=%s", string(convertedBody))
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationDoesNotInjectCachedAssistantForSubstantiveFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	seedReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	seedReq = seedReq.WithContext(WithSessionID(context.Background(), "sess-assist-cache-2"))
	ph.setCachedResponsesAssistantForRoute(seedReq, result, "A) 选项一 B) 选项二")

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-assist-cache-2"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_choice_cache_2",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"请写一个完整实现方案，包含目录结构、关键函数签名和测试策略。"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "user" {
		t.Fatalf("input.0.role = %q, want %q", got, "user")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationSkipsAssistantForShortStandaloneFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_short_standalone_1",
		"input":[
			{"role":"assistant","content":[{"type":"input_text","text":"A) option one B) option two C) option three"}]},
			{"role":"user","content":[{"type":"input_text","text":"Write a Dockerfile template"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "user" {
		t.Fatalf("input.0.role = %q, want %q", got, "user")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationDoesNotInjectCachedAssistantForShortStandaloneFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	seedReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	seedReq = seedReq.WithContext(WithSessionID(context.Background(), "sess-assist-cache-short-standalone-1"))
	ph.setCachedResponsesAssistantForRoute(seedReq, result, "A) option one B) option two C) option three")

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-assist-cache-short-standalone-1"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_short_standalone_cache_1",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"Write a Dockerfile template"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "user" {
		t.Fatalf("input.0.role = %q, want %q", got, "user")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationKeepsAssistantForOrdinalCueFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_ordinal_cue_1",
		"input":[
			{"role":"assistant","content":[{"type":"input_text","text":"A) option one B) option two C) option three"}]},
			{"role":"user","content":[{"type":"input_text","text":"Please explain the second option with implementation details."}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "assistant" {
		t.Fatalf("input.0.role = %q, want %q", got, "assistant")
	}
	if got := gjson.GetBytes(convertedBody, "input.0.content.0.text").String(); !strings.Contains(got, "A)") || !strings.Contains(got, "B)") {
		t.Fatalf("assistant options should be kept for ordinal cue follow-up, got: %q", got)
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationCompactsAssistantForChoiceFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	assistantText := "A) 方案一\nB) 方案二\nC) 方案三\n" + strings.Repeat("这是很长的解释段落，用来模拟超长assistant上下文。", 120)
	bodyObj := map[string]any{
		"model":                "gpt-5.3-codex-spark",
		"stream":               true,
		"previous_response_id": "resp_existing_compact_1",
		"input": []map[string]any{
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "input_text", "text": assistantText},
				},
			},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "B"},
				},
			},
		},
	}
	body, err := gojson.Marshal(bodyObj)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	gotAssistant := gjson.GetBytes(convertedBody, "input.0.content.0.text").String()
	if len([]rune(gotAssistant)) >= len([]rune(assistantText)) {
		t.Fatalf("assistant context not compacted: got len=%d, original len=%d", len([]rune(gotAssistant)), len([]rune(assistantText)))
	}
	if !strings.Contains(gotAssistant, "A)") || !strings.Contains(gotAssistant, "B)") {
		t.Fatalf("compacted assistant context should preserve options, got: %q", gotAssistant)
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationUsesCustomResponsesCompressor(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	ph.SetResponsesContextCompressor(fixedResponsesContextCompressor{output: "QWEN-0.8B-SUMMARY"})

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	bodyObj := map[string]any{
		"model":                "gpt-5.3-codex-spark",
		"stream":               true,
		"previous_response_id": "resp_existing_compact_custom_1",
		"input": []map[string]any{
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "input_text", "text": "A) option one\nB) option two\nC) option three\n" + strings.Repeat("details ", 400)},
				},
			},
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "B"},
				},
			},
		},
	}
	body, err := gojson.Marshal(bodyObj)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.0.content.0.text").String(); got != "QWEN-0.8B-SUMMARY" {
		t.Fatalf("input.0.content.0.text = %q, want %q", got, "QWEN-0.8B-SUMMARY")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesPathChatPayloadContinuationCompactsAssistantForChoiceFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	assistantText := "A) 方案一\nB) 方案二\nC) 方案三\n" + strings.Repeat("这是很长的解释段落，用来模拟超长assistant上下文。", 120)
	bodyObj := map[string]any{
		"model":                "gpt-5.3-codex-spark",
		"stream":               true,
		"previous_response_id": "resp_existing_compact_messages_1",
		"messages": []map[string]any{
			{"role": "assistant", "content": assistantText},
			{"role": "user", "content": "B"},
		},
	}
	body, err := gojson.Marshal(bodyObj)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	gotAssistant := gjson.GetBytes(convertedBody, "input.0.content.0.text").String()
	if len([]rune(gotAssistant)) >= len([]rune(assistantText)) {
		t.Fatalf("assistant context not compacted: got len=%d, original len=%d", len([]rune(gotAssistant)), len([]rune(assistantText)))
	}
	if !strings.Contains(gotAssistant, "A)") || !strings.Contains(gotAssistant, "B)") {
		t.Fatalf("compacted assistant context should preserve options, got: %q", gotAssistant)
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationCompactsInjectedCachedAssistantForChoiceFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	assistantText := "A) 选项一\nB) 选项二\nC) 选项三\n" + strings.Repeat("超长解释内容。", 220)
	seedReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	seedReq = seedReq.WithContext(WithSessionID(context.Background(), "sess-assist-cache-compact-1"))
	ph.setCachedResponsesAssistantForRoute(seedReq, result, assistantText)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-assist-cache-compact-1"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_choice_cache_compact_1",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"1"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	gotAssistant := gjson.GetBytes(convertedBody, "input.0.content.0.text").String()
	if len([]rune(gotAssistant)) >= len([]rune(assistantText)) {
		t.Fatalf("cached assistant context not compacted: got len=%d, original len=%d", len([]rune(gotAssistant)), len([]rune(assistantText)))
	}
	if !strings.Contains(gotAssistant, "A)") || !strings.Contains(gotAssistant, "B)") {
		t.Fatalf("compacted cached assistant context should preserve options, got: %q", gotAssistant)
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationInjectsAssistantFromResponseIDCache(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	ph.setCachedResponsesAssistantByResponseID("resp_cache_lookup_1", "A) 上下文选项 B) 其他")

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_cache_lookup_1",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"1"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.role").String(); got != "assistant" {
		t.Fatalf("input.0.role = %q, want %q", got, "assistant")
	}
	if got := gjson.GetBytes(convertedBody, "input.0.content.0.text").String(); got != "A) 上下文选项 B) 其他" {
		t.Fatalf("input.0.content.0.text = %q, want cached assistant text", got)
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationKeepsAllToolOutputsWithoutAssistant(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_tool_round",
		"input":[
			{"type":"function_call_output","call_id":"call_1","output":"{\"ok\":true}"},
			{"type":"function_call_output","call_id":"call_2","output":"{\"ok\":true}"}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.call_id").String(); got != "call_1" {
		t.Fatalf("input.0.call_id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.call_id").String(); got != "call_2" {
		t.Fatalf("input.1.call_id = %q, want %q", got, "call_2")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationToolOnlyShapeDropsStaleUserHistory(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	// REGRESSION-GUARD: tool-only continuation payloads must not keep stale
	// historical user turns; only function_call_output + latest follow-up user
	// turn should remain after TrimInput compaction.
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_tool_only_shape_1",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"old context 1"}]},
			{"role":"user","content":[{"type":"input_text","text":"old context 2"}]},
			{"type":"function_call","call_id":"call_1","name":"web_search","arguments":"{\"q\":\"ZimaOS Blue\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"{\"ok\":true}"},
			{"role":"user","content":[{"type":"input_text","text":"latest follow-up"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.type").String(); got != "function_call_output" {
		t.Fatalf("input.0.type = %q, want %q", got, "function_call_output")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.role").String(); got != "user" {
		t.Fatalf("input.1.role = %q, want %q", got, "user")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.content.0.text").String(); got != "latest follow-up" {
		t.Fatalf("input.1.content.0.text = %q, want %q", got, "latest follow-up")
	}
	if got := gjson.GetBytes(convertedBody, "input.0.name"); got.Exists() {
		t.Fatalf("function_call item should be dropped when output exists, body=%s", string(convertedBody))
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationToolPayloadDoesNotCarryAssistantWithoutUserFollowup(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"stream":true,
		"previous_response_id":"resp_existing_tool_only",
		"input":[
			{"role":"assistant","content":[{"type":"input_text","text":"我会调用工具"}]},
			{"type":"function_call_output","call_id":"call_1","output":"{\"ok\":true}"}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 1 {
		t.Fatalf("input length = %d, want 1; body=%s", got, string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "input.0.type").String(); got != "function_call_output" {
		t.Fatalf("input.0.type = %q, want %q", got, "function_call_output")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationToolOutputOverflowIsCompacted(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	oversizedOutput := strings.Repeat("x", responsesContinuationToolOutputMaxRunes+5000)
	bodyObj := map[string]any{
		"model":                "gpt-5.3-codex-spark",
		"stream":               true,
		"previous_response_id": "resp_existing_tool_overflow_1",
		"input": []map[string]any{
			{
				"type":    "function_call_output",
				"call_id": "call_big",
				"output":  oversizedOutput,
			},
			{
				"type":    "function_call_output",
				"call_id": "call_small",
				"output":  "{\"ok\":true}",
			},
		},
	}
	body, err := gojson.Marshal(bodyObj)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "input.#").Int(); got != 2 {
		t.Fatalf("input length = %d, want 2; body=%s", got, string(convertedBody))
	}
	gotBig := gjson.GetBytes(convertedBody, "input.0.output").String()
	if len([]rune(gotBig)) >= len([]rune(oversizedOutput)) {
		t.Fatalf("oversized output not compacted: got len=%d, original=%d", len([]rune(gotBig)), len([]rune(oversizedOutput)))
	}
	if !strings.Contains(gotBig, "[tool content trimmed]") {
		t.Fatalf("oversized output should contain trim marker, got=%q", gotBig)
	}
	if got := gjson.GetBytes(convertedBody, "input.0.call_id").String(); got != "call_big" {
		t.Fatalf("input.0.call_id = %q, want %q", got, "call_big")
	}
	if got := gjson.GetBytes(convertedBody, "input.1.call_id").String(); got != "call_small" {
		t.Fatalf("input.1.call_id = %q, want %q", got, "call_small")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesContinuationToolOutputOverflowIsCompacted(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatResponses,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	oversizedOutput := strings.Repeat("x", responsesContinuationToolOutputMaxRunes+5000)
	bodyObj := map[string]any{
		"model":                "gpt-5.3-codex-spark",
		"previous_response_id": "resp_existing_tool_overflow_chat_1",
		"messages": []map[string]any{
			{
				"role":         "tool",
				"tool_call_id": "call_big",
				"content":      oversizedOutput,
			},
		},
	}
	body, err := gojson.Marshal(bodyObj)
	if err != nil {
		t.Fatalf("marshal body failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/v1/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/v1/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	gotBig := gjson.GetBytes(convertedBody, "input.0.output").String()
	if len([]rune(gotBig)) >= len([]rune(oversizedOutput)) {
		t.Fatalf("openai-compat oversized output not compacted: got len=%d, original=%d", len([]rune(gotBig)), len([]rune(oversizedOutput)))
	}
	if !strings.Contains(gotBig, "[tool content trimmed]") {
		t.Fatalf("openai-compat overflow output should contain trim marker, got=%q", gotBig)
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationPreviousIDScopedByProvider(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	routeA := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-a",
			BaseURL:   "https://relay-a.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-a"},
	}
	routeB := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-b",
			BaseURL:   "https://relay-b.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-b"},
	}

	seedReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	seedReq = seedReq.WithContext(WithSessionID(context.Background(), "sess-prev-scope-1"))
	ph.setCachedResponsesPreviousIDForRoute(seedReq, routeA, "resp_from_a")

	reqB := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqB = reqB.WithContext(WithSessionID(context.Background(), "sess-prev-scope-1"))
	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"continue"}]}`)
	upstreamReqB, err := ph.buildUpstreamRequestWithFormat(reqB, routeB, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat for provider-b failed: %v", err)
	}
	defer upstreamReqB.Body.Close()
	convertedB, err := io.ReadAll(upstreamReqB.Body)
	if err != nil {
		t.Fatalf("read converted body for provider-b failed: %v", err)
	}
	if got := gjson.GetBytes(convertedB, "previous_response_id"); got.Exists() {
		t.Fatalf("provider-b request should not inherit provider-a previous_response_id, got: %s", string(convertedB))
	}

	reqA := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqA = reqA.WithContext(WithSessionID(context.Background(), "sess-prev-scope-1"))
	upstreamReqA, err := ph.buildUpstreamRequestWithFormat(reqA, routeA, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat for provider-a failed: %v", err)
	}
	defer upstreamReqA.Body.Close()
	convertedA, err := io.ReadAll(upstreamReqA.Body)
	if err != nil {
		t.Fatalf("read converted body for provider-a failed: %v", err)
	}
	if got := gjson.GetBytes(convertedA, "previous_response_id").String(); got != "resp_from_a" {
		t.Fatalf("provider-a previous_response_id = %q, want %q", got, "resp_from_a")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationDisabledForProviderStripsPreviousResponseID(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	route := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-disable-prev",
			BaseURL:   "https://relay-disable.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-disable"},
	}

	seedReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	seedReq = seedReq.WithContext(WithSessionID(context.Background(), "sess-prev-disable-1"))
	ph.setCachedResponsesPreviousIDForRoute(seedReq, route, "resp_cached_should_not_be_used")
	ph.markResponsesContinuationDisabledForRoute(seedReq, route)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-prev-disable-1"))
	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"continue"}]}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, route, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	converted, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(converted, "previous_response_id"); got.Exists() {
		t.Fatalf("previous_response_id should be stripped when continuation is disabled: %s", string(converted))
	}
	if got := gjson.GetBytes(converted, "input.0.role").String(); got != "user" {
		t.Fatalf("input.0.role = %q, want %q", got, "user")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationDisabledScopedBySession(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	route := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-disable-prev",
			BaseURL:   "https://relay-disable.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-disable"},
	}

	reqASeed := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqASeed = reqASeed.WithContext(WithSessionID(context.Background(), "sess-disable-a"))
	ph.setCachedResponsesPreviousIDForRoute(reqASeed, route, "resp_prev_a")
	ph.markResponsesContinuationDisabledForRoute(reqASeed, route)

	reqBSeed := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqBSeed = reqBSeed.WithContext(WithSessionID(context.Background(), "sess-disable-b"))
	ph.setCachedResponsesPreviousIDForRoute(reqBSeed, route, "resp_prev_b")

	body := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"continue"}]}`)

	reqA := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqA = reqA.WithContext(WithSessionID(context.Background(), "sess-disable-a"))
	upstreamA, err := ph.buildUpstreamRequestWithFormat(reqA, route, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("build request for session A failed: %v", err)
	}
	defer upstreamA.Body.Close()
	convertedA, err := io.ReadAll(upstreamA.Body)
	if err != nil {
		t.Fatalf("read request A body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedA, "previous_response_id"); got.Exists() {
		t.Fatalf("session A previous_response_id should be stripped after disable: %s", string(convertedA))
	}

	reqB := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqB = reqB.WithContext(WithSessionID(context.Background(), "sess-disable-b"))
	upstreamB, err := ph.buildUpstreamRequestWithFormat(reqB, route, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("build request for session B failed: %v", err)
	}
	defer upstreamB.Body.Close()
	convertedB, err := io.ReadAll(upstreamB.Body)
	if err != nil {
		t.Fatalf("read request B body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedB, "previous_response_id").String(); got != "resp_prev_b" {
		t.Fatalf("session B previous_response_id = %q, want %q", got, "resp_prev_b")
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationDisabledForProvider_SanitizesAssistantAndOrphanToolItems(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	route := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-disable-prev",
			BaseURL:   "https://relay-disable.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-disable"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-prev-disable-sanitize-1"))
	ph.markResponsesContinuationDisabledForRoute(req, route)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"previous_response_id":"resp_should_be_removed",
		"input":[
			{"type":"function_call","call_id":"call_orphan","name":"web_search","arguments":"{\"q\":\"orphan\"}"},
			{"type":"function_call","call_id":"call_pair","name":"web_search","arguments":"{\"q\":\"paired\"}"},
			{"type":"function_call_output","call_id":"call_pair","output":"{\"ok\":true}"},
			{"type":"function_call_output","call_id":"call_orphan_output","output":"{\"ok\":false}"},
			{"role":"assistant","content":[{"type":"input_text","text":"assistant context"}]},
			{"role":"user","content":[{"type":"input_text","text":"real user ask"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, route, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	converted, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "previous_response_id"); got.Exists() {
		t.Fatalf("previous_response_id should be stripped when continuation is disabled: %s", string(converted))
	}

	items := gjson.GetBytes(converted, "input").Array()
	if len(items) == 0 {
		t.Fatalf("input should not be empty after sanitization: %s", string(converted))
	}

	hasPairedCall := false
	hasPairedOutput := false
	hasFallbackCall := false
	hasFallbackOutput := false
	for _, item := range items {
		role := strings.ToLower(strings.TrimSpace(item.Get("role").String()))
		itemType := strings.TrimSpace(item.Get("type").String())
		callID := strings.TrimSpace(item.Get("call_id").String())
		if role == "assistant" {
			t.Fatalf("assistant role should be converted for continuation-disabled route: %s", string(converted))
		}
		switch itemType {
		case "function_call":
			if callID != "call_pair" {
				t.Fatalf("orphan function_call should be removed, got call_id=%q body=%s", callID, string(converted))
			}
			hasPairedCall = true
		case "function_call_output":
			if callID != "call_pair" {
				t.Fatalf("orphan function_call_output should be removed, got call_id=%q body=%s", callID, string(converted))
			}
			hasPairedOutput = true
		}
		if role == "user" {
			text := item.Get("content.0.text").String()
			if strings.Contains(text, "Previous tool call") {
				hasFallbackCall = true
			}
			if strings.Contains(text, "Previous tool output") {
				hasFallbackOutput = true
			}
		}
	}

	if !hasPairedCall {
		t.Fatalf("paired function_call should remain: %s", string(converted))
	}
	if !hasPairedOutput {
		t.Fatalf("paired function_call_output should remain: %s", string(converted))
	}
	if !hasFallbackCall {
		t.Fatalf("orphan function_call should be converted into user text fallback: %s", string(converted))
	}
	if !hasFallbackOutput {
		t.Fatalf("orphan function_call_output should be converted into user text fallback: %s", string(converted))
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationDisabledForProvider_ConvertsAssistantRoleWithoutToolItems(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	route := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-disable-prev",
			BaseURL:   "https://relay-disable.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-disable"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-prev-disable-assist-1"))
	ph.markResponsesContinuationDisabledForRoute(req, route)
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"previous_response_id":"resp_should_be_removed",
		"input":[
			{"role":"assistant","content":[{"type":"input_text","text":"assistant context"}]},
			{"role":"user","content":[{"type":"input_text","text":"real user ask"}]}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, route, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	converted, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "previous_response_id"); got.Exists() {
		t.Fatalf("previous_response_id should be stripped when continuation is disabled: %s", string(converted))
	}
	for _, item := range gjson.GetBytes(converted, "input").Array() {
		if strings.EqualFold(strings.TrimSpace(item.Get("role").String()), "assistant") {
			t.Fatalf("assistant role should be converted for continuation-disabled route: %s", string(converted))
		}
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationUnchangedToolsOmitted(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	route := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-tools",
			BaseURL:   "https://relay-tools.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-tools"},
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	firstReq = firstReq.WithContext(WithSessionID(context.Background(), "sess-tools-1"))
	firstBody := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"messages":[{"role":"user","content":"first turn"}],
		"tools":[{"type":"function","function":{"name":"exec","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}}]
	}`)
	upstreamFirst, err := ph.buildUpstreamRequestWithFormat(firstReq, route, firstBody, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("build first request failed: %v", err)
	}
	defer upstreamFirst.Body.Close()
	firstConverted, err := io.ReadAll(upstreamFirst.Body)
	if err != nil {
		t.Fatalf("read first converted body failed: %v", err)
	}
	if got := gjson.GetBytes(firstConverted, "tools").Exists(); !got {
		t.Fatalf("first request should include tools, body=%s", string(firstConverted))
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	secondReq = secondReq.WithContext(WithSessionID(context.Background(), "sess-tools-1"))
	secondBody := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"previous_response_id":"resp_tools_1",
		"messages":[{"role":"user","content":"second turn"}],
		"tools":[{"type":"function","function":{"name":"exec","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}}]
	}`)
	upstreamSecond, err := ph.buildUpstreamRequestWithFormat(secondReq, route, secondBody, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("build second request failed: %v", err)
	}
	defer upstreamSecond.Body.Close()
	secondConverted, err := io.ReadAll(upstreamSecond.Body)
	if err != nil {
		t.Fatalf("read second converted body failed: %v", err)
	}
	if got := gjson.GetBytes(secondConverted, "tools").Exists(); got {
		t.Fatalf("continuation with unchanged tools should omit tools, body=%s", string(secondConverted))
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationChangedToolsResent(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	route := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-tools",
			BaseURL:   "https://relay-tools.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-tools"},
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	firstReq = firstReq.WithContext(WithSessionID(context.Background(), "sess-tools-2"))
	firstBody := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"messages":[{"role":"user","content":"first turn"}],
		"tools":[{"type":"function","function":{"name":"exec","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}}]
	}`)
	if _, err := ph.buildUpstreamRequestWithFormat(firstReq, route, firstBody, providerpool.APIFormatResponses); err != nil {
		t.Fatalf("build first request failed: %v", err)
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	secondReq = secondReq.WithContext(WithSessionID(context.Background(), "sess-tools-2"))
	secondBody := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"previous_response_id":"resp_tools_2",
		"messages":[{"role":"user","content":"second turn"}],
		"tools":[{"type":"function","function":{"name":"read_file","parameters":{"type":"object","properties":{"path":{"type":"string"}}}}}]
	}`)
	upstreamSecond, err := ph.buildUpstreamRequestWithFormat(secondReq, route, secondBody, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("build second request failed: %v", err)
	}
	defer upstreamSecond.Body.Close()
	secondConverted, err := io.ReadAll(upstreamSecond.Body)
	if err != nil {
		t.Fatalf("read second converted body failed: %v", err)
	}
	if got := gjson.GetBytes(secondConverted, "tools").Exists(); !got {
		t.Fatalf("changed tools should be resent, body=%s", string(secondConverted))
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationToolsScopedByProvider(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	routeA := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-a",
			BaseURL:   "https://relay-a.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-a"},
	}
	routeB := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "provider-b",
			BaseURL:   "https://relay-b.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark"},
		APIKey: &providerpool.APIKey{Key: "sk-b"},
	}
	toolsBody := `[
		{"type":"function","function":{"name":"exec","parameters":{"type":"object","properties":{"cmd":{"type":"string"}}}}}
	]`

	firstReqA := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	firstReqA = firstReqA.WithContext(WithSessionID(context.Background(), "sess-tools-scope-1"))
	firstBodyA := []byte(`{"model":"gpt-5.3-codex-spark","messages":[{"role":"user","content":"first"}],"tools":` + toolsBody + `}`)
	if _, err := ph.buildUpstreamRequestWithFormat(firstReqA, routeA, firstBodyA, providerpool.APIFormatResponses); err != nil {
		t.Fatalf("build provider-a first request failed: %v", err)
	}

	reqB := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqB = reqB.WithContext(WithSessionID(context.Background(), "sess-tools-scope-1"))
	bodyB := []byte(`{"model":"gpt-5.3-codex-spark","previous_response_id":"resp_tools_scope_b","messages":[{"role":"user","content":"continue"}],"tools":` + toolsBody + `}`)
	upstreamReqB, err := ph.buildUpstreamRequestWithFormat(reqB, routeB, bodyB, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("build provider-b request failed: %v", err)
	}
	defer upstreamReqB.Body.Close()
	convertedB, err := io.ReadAll(upstreamReqB.Body)
	if err != nil {
		t.Fatalf("read provider-b converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedB, "tools").Exists(); !got {
		t.Fatalf("provider-b request should not inherit provider-a tools cache, body=%s", string(convertedB))
	}

	reqA := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	reqA = reqA.WithContext(WithSessionID(context.Background(), "sess-tools-scope-1"))
	bodyA := []byte(`{"model":"gpt-5.3-codex-spark","previous_response_id":"resp_tools_scope_a","messages":[{"role":"user","content":"continue"}],"tools":` + toolsBody + `}`)
	upstreamReqA, err := ph.buildUpstreamRequestWithFormat(reqA, routeA, bodyA, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("build provider-a continuation request failed: %v", err)
	}
	defer upstreamReqA.Body.Close()
	convertedA, err := io.ReadAll(upstreamReqA.Body)
	if err != nil {
		t.Fatalf("read provider-a converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedA, "tools").Exists(); got {
		t.Fatalf("provider-a continuation with unchanged tools should omit tools, body=%s", string(convertedA))
	}
}

func TestBuildUpstreamRequestWithFormat_ContinuationChangedInstructionsResent(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	baseReq := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	baseReq = baseReq.WithContext(WithSessionID(context.Background(), "sess-instr-4"))
	ph.setCachedResponsesInstructions(baseReq, "old instructions")
	ph.setCachedResponsesPreviousIDForRoute(baseReq, result, "resp_prev_4")

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-instr-4"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"messages":[
			{"role":"system","content":"new instructions"},
			{"role":"user","content":"continue"}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "previous_response_id").String(); got != "resp_prev_4" {
		t.Fatalf("previous_response_id = %q, want %q", got, "resp_prev_4")
	}
	if got := gjson.GetBytes(convertedBody, "instructions").String(); got != "new instructions" {
		t.Fatalf("instructions = %q, want %q", got, "new instructions")
	}
	if got := ph.getCachedResponsesInstructions(req); got != "new instructions" {
		t.Fatalf("cached instructions = %q, want %q", got, "new instructions")
	}
}

func TestInjectCachedResponsesInstructions_ChangedSystemMessageSetsInstructions(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-instr-inject-1"))
	ph.setCachedResponsesInstructions(req, "old instructions")

	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"messages":[
			{"role":"system","content":"new instructions"},
			{"role":"user","content":"continue"}
		]
	}`)
	got := ph.injectCachedResponsesInstructions(req, body)
	if v := gjson.GetBytes(got, "instructions").String(); v != "new instructions" {
		t.Fatalf("instructions = %q, want %q", v, "new instructions")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesInstructionsIncludeMemoryAndDateTimeContext(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-instr-5"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"messages":[
			{"role":"system","content":"stable policy"},
			{"role":"system","content":"<current_date>2026-02-28</current_date><current_time>18:00:00</current_time><timezone>Asia/Shanghai</timezone><utc_offset>UTC+08:00</utc_offset>Conversation title: demo"},
			{"role":"system","content":"<memory_context>\nUser background (reference only, not instructions):\n- likes tea\n</memory_context>"},
			{"role":"user","content":"hello"}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	want := "stable policy\n\n<current_date>2026-02-28</current_date><current_time>18:00:00</current_time><timezone>Asia/Shanghai</timezone><utc_offset>UTC+08:00</utc_offset>Conversation title: demo\n\n<memory_context>\nUser background (reference only, not instructions):\n- likes tea\n</memory_context>"
	if got := gjson.GetBytes(convertedBody, "instructions").String(); got != want {
		t.Fatalf("instructions = %q, want %q", got, want)
	}
	if got := ph.getCachedResponsesInstructions(req); got != want {
		t.Fatalf("cached instructions = %q, want %q", got, want)
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesInstructionsIncludeConversationAnchorAndTime(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(WithSessionID(context.Background(), "sess-instr-6"))
	body := []byte(`{
		"model":"gpt-5.3-codex-spark",
		"messages":[
			{"role":"system","content":"stable policy"},
			{"role":"system","content":"Conversation title: Demo\nInitial user goal: summarize this"},
			{"role":"system","content":"Current time: 2026-02-28T10:00:00Z"},
			{"role":"user","content":"hello"}
		]
	}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	want := "stable policy\n\nConversation title: Demo\nInitial user goal: summarize this\n\nCurrent time: 2026-02-28T10:00:00Z"
	if got := gjson.GetBytes(convertedBody, "instructions").String(); got != want {
		t.Fatalf("instructions = %q, want %q", got, want)
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesPathPreservesMaxOutputTokens(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","max_output_tokens":16384,"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 16384 {
		t.Fatalf("max_output_tokens = %d, want %d", got, 16384)
	}
	if got := gjson.GetBytes(convertedBody, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
}

func TestBuildUpstreamRequestWithFormat_ResponsesPathClampsMaxOutputTokensByModelLimit(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark", MaxOutput: 2048},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","max_output_tokens":16384,"input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatResponses)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 2048 {
		t.Fatalf("max_output_tokens = %d, want %d", got, 2048)
	}
}

func TestBuildUpstreamRequestWithFormat_OpenAICompatPathClampsResponsesMaxOutputTokensByModelLimit(t *testing.T) {
	ph := NewProxyHandler(nil, NewConnectionPool(DefaultConnectionConfig()), nil)

	result := &providerpool.RouteResult{
		Provider: &providerpool.Provider{
			ID:        "third-party-openai",
			BaseURL:   "https://relay.example.com/v1",
			APIFormat: providerpool.APIFormatOpenAI,
		},
		Model:  &providerpool.Model{ID: "gpt-5.3-codex-spark", MaxOutput: 1024},
		APIKey: &providerpool.APIKey{Key: "sk-test"},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	body := []byte(`{"model":"gpt-5.3-codex-spark","max_tokens":4096,"messages":[{"role":"user","content":"hi"}]}`)
	upstreamReq, err := ph.buildUpstreamRequestWithFormat(req, result, body, providerpool.APIFormatOpenAI)
	if err != nil {
		t.Fatalf("buildUpstreamRequestWithFormat failed: %v", err)
	}
	defer upstreamReq.Body.Close()

	if upstreamReq.URL.Path != "/v1/responses" {
		t.Fatalf("upstream path = %q, want %q", upstreamReq.URL.Path, "/v1/responses")
	}

	convertedBody, err := io.ReadAll(upstreamReq.Body)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}
	if got := gjson.GetBytes(convertedBody, "max_output_tokens").Int(); got != 1024 {
		t.Fatalf("max_output_tokens = %d, want %d", got, 1024)
	}
	if !gjson.GetBytes(convertedBody, "input").Exists() {
		t.Fatalf("responses body missing input: %s", string(convertedBody))
	}
	if gjson.GetBytes(convertedBody, "messages").Exists() {
		t.Fatalf("responses body should not contain messages: %s", string(convertedBody))
	}
	if got := gjson.GetBytes(convertedBody, "store").Bool(); !got {
		t.Fatalf("store = false, want true")
	}
}

func TestConvertResponsesToOpenAIChatCompletions(t *testing.T) {
	body := []byte(`{
		"id":"resp_123",
		"object":"response",
		"created_at":1730000000,
		"model":"gpt-5.3-codex",
		"output":[
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello "} ,{"type":"output_text","text":"world"}]},
			{"type":"function_call","id":"fc_1","call_id":"call_1","name":"web_search","arguments":"{\"query\":\"ZimaOS Blue\"}"}
		],
		"usage":{"input_tokens":12,"output_tokens":5,"total_tokens":17}
	}`)

	converted, err := convertResponsesToOpenAIChatCompletions(body, "gpt-5.3-codex-spark")
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if got := gjson.GetBytes(converted, "object").String(); got != "chat.completion" {
		t.Fatalf("object = %q, want %q", got, "chat.completion")
	}
	if got := gjson.GetBytes(converted, "choices.0.message.content").String(); got != "hello world" {
		t.Fatalf("content = %q, want %q", got, "hello world")
	}
	if got := gjson.GetBytes(converted, "choices.0.message.tool_calls.0.id").String(); got != "call_1" {
		t.Fatalf("tool call id = %q, want %q", got, "call_1")
	}
	if got := gjson.GetBytes(converted, "choices.0.message.tool_calls.0.function.name").String(); got != "web_search" {
		t.Fatalf("tool call function name = %q, want %q", got, "web_search")
	}
	if got := gjson.GetBytes(converted, "choices.0.message.tool_calls.0.function.arguments").String(); got != `{"query":"ZimaOS Blue"}` {
		t.Fatalf("tool call arguments = %q, want %q", got, `{"query":"ZimaOS Blue"}`)
	}
	if got := gjson.GetBytes(converted, "choices.0.finish_reason").String(); got != "tool_calls" {
		t.Fatalf("finish_reason = %q, want %q", got, "tool_calls")
	}
	if got := gjson.GetBytes(converted, "model").String(); got != "gpt-5.3-codex-spark" {
		t.Fatalf("model = %q, want %q", got, "gpt-5.3-codex-spark")
	}
	if got := gjson.GetBytes(converted, "usage.prompt_tokens").Int(); got != 12 {
		t.Fatalf("usage.prompt_tokens = %d, want 12", got)
	}
}
