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
)

const (
	// maxSSELineSize is the max size of a single SSE data line (1MB).
	maxSSELineSize = 1 << 20
	// maxResponseSize caps non-streaming response body to prevent OOM (10MB).
	maxResponseSize = 10 << 20
	// defaultTimeout for bridge calls when context has no deadline.
	defaultTimeout = 120 * time.Second
)

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

	rec := httptest.NewRecorder()
	b.handler.ServeHTTP(rec, httpReq)

	if rec.Code >= 400 {
		// Truncate error body to avoid huge error strings (#9)
		errBody := rec.Body.String()
		if len(errBody) > 512 {
			errBody = errBody[:512] + "...(truncated)"
		}
		return nil, fmt.Errorf("proxy returned %d: %s", rec.Code, errBody)
	}

	// Guard against unbounded response size (#4)
	if rec.Body.Len() > maxResponseSize {
		return nil, fmt.Errorf("proxy response too large: %d bytes", rec.Body.Len())
	}

	return ParseChatResponse(rec.Body.Bytes())
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

	req.Stream = true
	body, err := MarshalChatRequest(req)
	if err != nil {
		return fmt.Errorf("bridge marshal: %w", err)
	}
	// Debug: log marshaled request body (truncated)
	if len(body) > 500 {
		slog.Info("[bridge] request body (truncated)", "model", req.Model, "body", string(body[:500]))
	} else {
		slog.Info("[bridge] request body", "model", req.Model, "body", string(body))
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("bridge request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	pr, pw := io.Pipe()
	rw := newPipeResponseWriter(pw)

	// Handler goroutine — writes SSE to pipe
	doneCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		b.handler.ServeHTTP(rw, httpReq)
		if rw.code >= 400 {
			slog.Error("[bridge] proxy handler returned error", "code", rw.code, "model", req.Model)
			doneCh <- fmt.Errorf("proxy returned %d", rw.code)
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
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			// Log non-SSE lines — these may contain error messages from proxy handler
			if line != "" {
				slog.Warn("[bridge] non-SSE line from proxy", "line", line, "model", req.Model)
			}
			continue
		}
		chunkCount++
		payload := strings.TrimPrefix(line, "data: ")
		chunk, done, parseErr := ParseSSEChunk(payload)
		if parseErr != nil {
			// Fix #7: Log malformed SSE instead of silently dropping
			slog.Warn("bridge: malformed SSE chunk", "error", parseErr, "payload_len", len(payload))
			continue
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
	if chunkCount == 0 {
		slog.Error("[bridge] stream ended with zero chunks", "model", req.Model, "scan_err", scanErr)
	}

	// Fix #2: Close pipe reader to unblock handler goroutine, then wait for it
	pr.Close()

	// Fix #3: Blocking wait for handler goroutine to finish — no race on error channel
	handlerErr := <-doneCh
	if handlerErr != nil {
		slog.Error("[bridge] handler error after stream", "error", handlerErr, "chunks", chunkCount, "model", req.Model)
	}

	// Prefer handler-level errors (HTTP 4xx/5xx) over scan errors
	if handlerErr != nil {
		return handlerErr
	}
	return scanErr
}
