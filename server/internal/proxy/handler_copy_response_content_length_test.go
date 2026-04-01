package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestCopyResponse_AnthropicNonStreamingRewritesContentLengthAfterConversion(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)

	body := `{"id":"msg_123","type":"message","role":"assistant","content":[{"type":"text","text":"OK"}],"model":"claude-haiku-4-5-20251001","stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":4}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Content-Length": []string{"359"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}

	rec := httptest.NewRecorder()
	pr := &parsedRequest{upstreamFormat: ProviderTypeAnthropic}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ph.copyResponse(rec, resp, pr, req)

	gotBody := rec.Body.String()
	wantLength := strconv.Itoa(len(gotBody))
	if got := rec.Header().Get("Content-Length"); got != wantLength {
		t.Fatalf("Content-Length = %q, want %q; body=%s", got, wantLength, gotBody)
	}
	if !strings.Contains(gotBody, `"object":"chat.completion"`) {
		t.Fatalf("expected converted OpenAI response, got: %s", gotBody)
	}
}
