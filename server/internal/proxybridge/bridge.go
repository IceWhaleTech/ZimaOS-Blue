package proxybridge

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

const (
	// maxSSELineSize is the max size of a single SSE data line (1MB).
	maxSSELineSize = 1 << 20
	// maxResponseSize caps non-streaming response body to prevent OOM (10MB).
	maxResponseSize = 10 << 20
	// defaultTimeout for bridge calls when context has no deadline.
	defaultTimeout = 30 * time.Second
)

// ProxyError wraps an HTTP status code from the proxy handler so callers
// can distinguish client errors (4xx, non-retryable) from server errors (5xx, retryable).
type ProxyError struct {
	StatusCode int
	Body       string // truncated error body for diagnostics
}

func (e *ProxyError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("proxy returned %d: %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("proxy returned %d", e.StatusCode)
}

// IsClientError returns true for 4xx status codes (request is invalid, retrying won't help).
func (e *ProxyError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// IsOverloaded returns true for 429 (rate limit) or 529 (overloaded) — retrying is pointless.
func (e *ProxyError) IsOverloaded() bool {
	return e.StatusCode == 429 || e.StatusCode == 529
}

// IsNoProvider returns true when no provider is available for the requested model.
// Retrying won't help — the user needs to configure/enable a provider.
func (e *ProxyError) IsNoProvider() bool {
	return e.StatusCode == 503 && strings.Contains(e.Body, "no available provider")
}

// Bridge adapts llm.ChatRequest/ChatResponse to flow through an http.Handler proxy.
type Bridge struct {
	handler http.Handler
}

// NewBridge creates a new bridge that routes LLM calls through the given handler.
func NewBridge(handler http.Handler) *Bridge {
	return &Bridge{handler: handler}
}

// ensureTimeout returns a context with a deadline if one isn't already set.
func ensureTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

// Chat sends a non-streaming request through the proxy pipeline.
func (b *Bridge) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	// Inject ResolvedRoute into context so the proxy handler can populate it
	var resolved proxy.ResolvedRoute
	ctx = proxy.WithResolvedRoute(ctx, &resolved)

	req.Stream = false
	body, err := MarshalChatRequest(req)
	if err != nil {
		return nil, fmt.Errorf("bridge marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bridge request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if proxy.DisableResponsesContinuationFromContext(ctx) {
		httpReq.Header.Set(proxy.DisableResponsesContinuationHeader, "1")
	}

	rec := httptest.NewRecorder()
	b.handler.ServeHTTP(rec, httpReq)

	if rec.Code >= 400 {
		errBody := rec.Body.String()
		if len(errBody) > 512 {
			errBody = errBody[:512] + "...(truncated)"
		}
		return nil, &ProxyError{StatusCode: rec.Code, Body: errBody}
	}

	// Guard against unbounded response size (#4)
	if rec.Body.Len() > maxResponseSize {
		return nil, fmt.Errorf("proxy response too large: %d bytes", rec.Body.Len())
	}

	resp, parseErr := ParseChatResponse(rec.Body.Bytes())
	if parseErr != nil {
		return nil, parseErr
	}
	// Inject resolved provider/model into response.
	// Always prefer resolved.Model — the upstream provider may return its own
	// model name which differs from our routing model ID.
	if resp != nil {
		if resolved.Provider != "" {
			resp.Provider = resolved.Provider
		}
		if resolved.ProviderID != "" {
			resp.ProviderID = resolved.ProviderID
		}
		if resolved.Model != "" {
			resp.Model = resolved.Model
		}
	}
	return resp, nil
}

// pipeResponseWriter implements http.ResponseWriter + http.Flusher over an io.Writer.
type pipeResponseWriter struct {
	w       io.Writer
	header  http.Header
	code    int
	written bool
}

func newPipeResponseWriter(w io.Writer) *pipeResponseWriter {
	return &pipeResponseWriter{w: w, header: make(http.Header)}
}

func (p *pipeResponseWriter) Header() http.Header { return p.header }

func (p *pipeResponseWriter) WriteHeader(code int) {
	p.code = code
	p.written = true
}

func (p *pipeResponseWriter) Write(b []byte) (int, error) {
	if !p.written {
		p.code = http.StatusOK
		p.written = true
	}
	return p.w.Write(b)
}

func (p *pipeResponseWriter) Flush() {
	if f, ok := p.w.(interface{ Flush() }); ok {
		f.Flush()
	}
}

// ChatStream sends a streaming request through the proxy pipeline.
func (b *Bridge) ChatStream(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	ctx, cancel := ensureTimeout(ctx)
	defer cancel()

	// Check if caller provided a ResolvedRoute to populate
	callerRoute := proxy.GetResolvedRouteFromContext(ctx)

	// Inject our own ResolvedRoute into context so the proxy handler can populate it
	var resolved proxy.ResolvedRoute
	ctx = proxy.WithResolvedRoute(ctx, &resolved)

	req.Stream = true
	body, err := MarshalChatRequest(req)
	if err != nil {
		return fmt.Errorf("bridge marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("bridge request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if proxy.DisableResponsesContinuationFromContext(ctx) {
		httpReq.Header.Set(proxy.DisableResponsesContinuationHeader, "1")
	}

	pr, pw := io.Pipe()
	rw := newPipeResponseWriter(pw)

	// Handler goroutine — writes SSE to pipe
	doneCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		b.handler.ServeHTTP(rw, httpReq)
		if rw.code >= 400 {
			slog.Error("[bridge] proxy handler returned error", "code", rw.code, "model", req.Model)
			doneCh <- &ProxyError{StatusCode: rw.code}
		} else {
			slog.Debug("[bridge] proxy handler completed", "code", rw.code, "model", req.Model)
			doneCh <- nil
		}
	}()

	// Fix #1: Increase scanner buffer to handle large SSE lines (tool calls, base64, etc.)
	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSELineSize)

	var scanErr error
	var chunkCount int
	var nonSSELines []string // capture non-SSE lines for error diagnostics
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			// SSE comments/blank lines are keep-alive heartbeats or frame separators.
			continue
		}
		if strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") {
			// Standard SSE control fields can precede data lines (e.g. Responses API emits `event:`).
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			// Log truly non-SSE lines — these may contain error messages from proxy handler.
			slog.Warn("[bridge] non-SSE line from proxy", "line", line, "model", req.Model)
			nonSSELines = append(nonSSELines, line)
			continue
		}
		payload := strings.TrimPrefix(line, "data:")
		if len(payload) > 0 && payload[0] == ' ' {
			payload = payload[1:]
		}
		if payload == "" {
			continue
		}
		chunk, done, parseErr := ParseSSEChunk(payload)
		if parseErr != nil {
			// Fix #7: Log malformed SSE instead of silently dropping
			slog.Warn("bridge: malformed SSE chunk", "error", parseErr, "payload_len", len(payload))
			continue
		}
		// Ignore metadata/no-op events that carry no delta, tool call, usage, or terminal state.
		// Responses API emits many bookkeeping events (created/added/done) that should not
		// trigger downstream callback invocations.
		if !done && chunk.Delta == "" && chunk.Error == "" && len(chunk.ToolCalls) == 0 && chunk.Usage == nil {
			continue
		}
		chunkCount++
		// Inject actual provider/model from the resolved route (set by proxy handler
		// via context before any data is written to the pipe, so it's safe to read here).
		// IMPORTANT: Always prefer resolved.Model over chunk.Model. The upstream
		// provider may return its own model name (e.g. "gpt-4o-2024-08-06") which
		// differs from our routing model ID. Using the upstream name for subsequent
		// tool rounds causes routing failures (model not in snapshot → blind fallback).
		if chunk.Provider == "" && resolved.Provider != "" {
			chunk.Provider = resolved.Provider
		}
		if chunk.ProviderID == "" && resolved.ProviderID != "" {
			chunk.ProviderID = resolved.ProviderID
		}
		if resolved.Model != "" {
			chunk.Model = resolved.Model
		}
		if chunkCount == 1 {
			slog.Info("[bridge] first chunk metadata",
				"chunk.Provider", chunk.Provider,
				"chunk.Model", chunk.Model,
				"resolved.Provider", resolved.Provider,
				"resolved.Model", resolved.Model,
			)
		}
		if cbErr := callback(chunk); cbErr != nil {
			scanErr = cbErr
			break
		}
		if done {
			break
		}
	}
	if scanErr == nil {
		scanErr = scanner.Err()
	}
	zeroChunks := chunkCount == 0
	if zeroChunks {
		slog.Error("[bridge] stream ended with zero chunks", "model", req.Model, "scan_err", scanErr)
	}

	// Fix #2: Close pipe reader to unblock handler goroutine, then wait for it
	pr.Close()

	// Fix #3: Blocking wait for handler goroutine to finish — no race on error channel
	handlerErr := <-doneCh
	if handlerErr != nil {
		slog.Error("[bridge] handler error after stream", "error", handlerErr, "chunks", chunkCount, "model", req.Model)
	}

	// Prefer handler-level errors (HTTP 4xx/5xx) over scan errors.
	// Enrich ProxyError with non-SSE body lines captured during scanning so that
	// callers (e.g. IsNoProvider, IsClientError) can inspect the error body.
	if handlerErr != nil {
		if pe, ok := handlerErr.(*ProxyError); ok && pe.Body == "" && len(nonSSELines) > 0 {
			pe.Body = strings.Join(nonSSELines, "\n")
		}
		return handlerErr
	}

	// A 200 stream that never emitted a parsable SSE chunk is a protocol failure.
	// Surface it as an error so callers can retry/fail instead of silently succeeding.
	if zeroChunks {
		body := strings.Join(nonSSELines, "\n")
		if body == "" {
			body = "stream ended with zero chunks"
		}
		return &ProxyError{StatusCode: http.StatusBadGateway, Body: body}
	}

	// Propagate resolved route back to caller if they provided one
	if callerRoute != nil {
		if callerRoute.Provider == "" {
			callerRoute.Provider = resolved.Provider
		}
		if callerRoute.ProviderID == "" {
			callerRoute.ProviderID = resolved.ProviderID
		}
		if callerRoute.Model == "" {
			callerRoute.Model = resolved.Model
		}
	}

	return scanErr
}
