package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	externalRunnerContextTimeout = 30 * time.Second
	acpMessageTimeout            = 15 * time.Second
	httpReadyTimeout             = 20 * time.Second
	ssePayloadTimeout            = 10 * time.Second
)

func TestAgentcoreRunnerBinaryBuildsIndependently(t *testing.T) {
	serverDir := moduleRootDir(t)
	cmd := exec.Command("go", "build", "./cmd/agentcore-runner")
	cmd.Dir = serverDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build ./cmd/agentcore-runner failed: %v\n%s", err, output)
	}
}

func TestAgentcoreRunnerACPModeSessionLifecycle(t *testing.T) {
	bin := buildAgentcoreRunnerBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), externalRunnerContextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "acp")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start runner: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)
	initialize := map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": 1,
		},
	}
	writeACPMessage(t, stdin, initialize)
	initResp := readACPMessage(t, reader, &stderr)
	result := asMap(t, initResp["result"])
	if got := numberAsInt(result["protocolVersion"]); got != 1 {
		t.Fatalf("protocolVersion = %d, want 1", got)
	}

	writeACPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      "2",
		"method":  "session/new",
		"params":  map[string]any{"cwd": t.TempDir()},
	})
	sessionResp := readACPMessage(t, reader, &stderr)
	sessionResult := asMap(t, sessionResp["result"])
	sessionID := strings.TrimSpace(fmt.Sprint(sessionResult["sessionId"]))
	if sessionID == "" {
		t.Fatal("session/new returned empty sessionId")
	}

	writeACPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      "3",
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": sessionID,
			"prompt": []map[string]any{
				{"type": "text", "text": "hello runner"},
			},
		},
	})
	updateMsg := readACPMessage(t, reader, &stderr)
	if method := strings.TrimSpace(fmt.Sprint(updateMsg["method"])); method != "session/update" {
		t.Fatalf("unexpected update method %q", method)
	}
	updateParams := asMap(t, updateMsg["params"])
	update := asMap(t, updateParams["update"])
	if got := strings.TrimSpace(fmt.Sprint(update["sessionUpdate"])); got != "agent_message_chunk" {
		t.Fatalf("sessionUpdate = %q, want agent_message_chunk", got)
	}
	content := asMap(t, update["content"])
	if text := strings.TrimSpace(fmt.Sprint(content["text"])); !strings.Contains(text, "hello runner") {
		t.Fatalf("agent chunk text = %q, want substring hello runner", text)
	}

	promptResp := readACPMessage(t, reader, &stderr)
	promptResult := asMap(t, promptResp["result"])
	if stopReason := strings.TrimSpace(fmt.Sprint(promptResult["stopReason"])); stopReason != "completed" {
		t.Fatalf("stopReason = %q, want completed", stopReason)
	}

	writeACPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"method":  "session/cancel",
		"params": map[string]any{
			"sessionId": sessionID,
		},
	})
}

func TestAgentcoreRunnerA2AModeCardAndStreamingFlow(t *testing.T) {
	bin := buildAgentcoreRunnerBinary(t)
	listenAddr := reserveLoopbackAddr(t)
	ctx, cancel := context.WithTimeout(context.Background(), externalRunnerContextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "a2a", "--listen", listenAddr)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start runner: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	waitForHTTPReady(t, "http://"+listenAddr+"/v1/card", &stderr)
	cardResp, err := http.Get("http://" + listenAddr + "/v1/card")
	if err != nil {
		t.Fatalf("get card: %v", err)
	}
	defer cardResp.Body.Close()
	if cardResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cardResp.Body)
		t.Fatalf("card status = %d body=%s", cardResp.StatusCode, body)
	}
	var card map[string]any
	if err := json.NewDecoder(cardResp.Body).Decode(&card); err != nil {
		t.Fatalf("decode card: %v", err)
	}
	if got := strings.TrimSpace(fmt.Sprint(card["url"])); got != "http://"+listenAddr {
		t.Fatalf("card url = %q, want %q", got, "http://"+listenAddr)
	}

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "message/stream",
		"params": map[string]any{
			"message": map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{"kind": "text", "text": "A2A hello"},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+listenAddr, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); !strings.Contains(ct, "text/event-stream") {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected content-type %q body=%s", ct, body)
	}

	var firstData string
	scanner := bufio.NewScanner(resp.Body)
	deadline := time.After(ssePayloadTimeout)
	for firstData == "" {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for SSE data; stderr=%s", strings.TrimSpace(stderr.String()))
		default:
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan sse: %v", err)
			}
			continue
		}
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "data:") {
			firstData = strings.TrimSpace(line[5:])
		}
	}
	if !strings.Contains(firstData, "A2A hello") {
		t.Fatalf("unexpected first SSE payload %q", firstData)
	}
}

func TestAgentcoreRunnerACPModeBridgesBlueConversationStream(t *testing.T) {
	blue := newStubBlueConversationServer(t, []string{"Hello Blue", "Again Blue"})
	defer blue.Close()

	bin := buildAgentcoreRunnerBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), externalRunnerContextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "acp", "--blue-url", blue.URL())
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start runner: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)
	writeACPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": 1},
	})
	_ = readACPMessage(t, reader, &stderr)

	writeACPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      "2",
		"method":  "session/new",
		"params":  map[string]any{},
	})
	sessionResp := readACPMessage(t, reader, &stderr)
	sessionID := strings.TrimSpace(fmt.Sprint(asMap(t, sessionResp["result"])["sessionId"]))
	if sessionID == "" {
		t.Fatal("session/new returned empty sessionId")
	}

	for i, want := range []string{"Hello Blue", "Again Blue"} {
		writeACPMessage(t, stdin, map[string]any{
			"jsonrpc": "2.0",
			"id":      strconv.Itoa(3 + i),
			"method":  "session/prompt",
			"params": map[string]any{
				"sessionId": sessionID,
				"prompt": []map[string]any{
					{"type": "text", "text": fmt.Sprintf("prompt-%d", i+1)},
				},
			},
		})
		var deltas []string
		for {
			msg := readACPMessage(t, reader, &stderr)
			if result, ok := msg["result"].(map[string]any); ok {
				if stopReason := strings.TrimSpace(fmt.Sprint(result["stopReason"])); stopReason != "completed" {
					t.Fatalf("stopReason = %q, want completed", stopReason)
				}
				break
			}
			if method := strings.TrimSpace(fmt.Sprint(msg["method"])); method != "session/update" {
				t.Fatalf("unexpected update method %q", method)
			}
			update := asMap(t, asMap(t, asMap(t, msg["params"])["update"])["content"])
			deltas = append(deltas, fmt.Sprint(update["text"]))
		}
		if len(deltas) < 2 {
			t.Fatalf("delta count = %d, want at least 2", len(deltas))
		}
		if got := strings.TrimSpace(strings.Join(deltas, "")); got != want {
			t.Fatalf("joined deltas = %q, want %q", got, want)
		}
		if strings.TrimSpace(deltas[0]) == want {
			t.Fatalf("expected streamed delta chunks, first delta already equals full response %q", deltas[0])
		}
	}

	if got := blue.CreateConversationCount(); got != 1 {
		t.Fatalf("create conversation count = %d, want 1", got)
	}
	if got := blue.StreamMessageCount(); got != 2 {
		t.Fatalf("stream message count = %d, want 2", got)
	}
}

func TestAgentcoreRunnerA2AModeBridgesBlueConversationStream(t *testing.T) {
	blue := newStubBlueConversationServer(t, []string{"Hello from Blue bridge"})
	defer blue.Close()

	bin := buildAgentcoreRunnerBinary(t)
	listenAddr := reserveLoopbackAddr(t)
	ctx, cancel := context.WithTimeout(context.Background(), externalRunnerContextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "a2a", "--listen", listenAddr, "--blue-url", blue.URL())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start runner: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	waitForHTTPReady(t, "http://"+listenAddr+"/v1/card", &stderr)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "message/stream",
		"params": map[string]any{
			"message": map[string]any{
				"role": "user",
				"parts": []map[string]any{
					{"kind": "text", "text": "bridge this"},
				},
			},
		},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+listenAddr, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if ct := strings.ToLower(resp.Header.Get("Content-Type")); !strings.Contains(ct, "text/event-stream") {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unexpected content-type %q body=%s", ct, body)
	}

	var firstPayload string
	var finalPayload string
	scanner := bufio.NewScanner(resp.Body)
	deadline := time.After(ssePayloadTimeout)
	for finalPayload == "" {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for final SSE payload; first=%q final=%q stderr=%s", firstPayload, finalPayload, strings.TrimSpace(stderr.String()))
		default:
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan sse: %v", err)
			}
			continue
		}
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(strings.ToLower(line), "data:") {
			payload := strings.TrimSpace(line[5:])
			if firstPayload == "" {
				firstPayload = payload
			}
			if strings.Contains(payload, `"completed"`) {
				finalPayload = payload
			}
		}
	}
	if strings.Contains(firstPayload, "Hello from Blue bridge") {
		t.Fatalf("expected first payload to be a partial delta, got %q", firstPayload)
	}
	if !strings.Contains(finalPayload, "Hello from Blue bridge") {
		t.Fatalf("expected final payload to contain full response, got %q", finalPayload)
	}
	if got := blue.CreateConversationCount(); got != 1 {
		t.Fatalf("create conversation count = %d, want 1", got)
	}
	if got := blue.StreamMessageCount(); got != 1 {
		t.Fatalf("stream message count = %d, want 1", got)
	}
}

func TestAgentcoreRunnerACPModeBridgeSendsConfiguredBlueHeader(t *testing.T) {
	blue := newStubBlueConversationServer(t, []string{"Header ok"})
	blue.RequireHeader("Authorization", "Bearer secret-token")
	defer blue.Close()

	bin := buildAgentcoreRunnerBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), externalRunnerContextTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "acp", "--blue-url", blue.URL(), "--blue-header", "Authorization=Bearer secret-token")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start runner: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	reader := bufio.NewReader(stdout)
	writeACPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "initialize",
		"params":  map[string]any{"protocolVersion": 1},
	})
	_ = readACPMessage(t, reader, &stderr)

	writeACPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      "2",
		"method":  "session/new",
		"params":  map[string]any{},
	})
	sessionResp := readACPMessage(t, reader, &stderr)
	sessionID := strings.TrimSpace(fmt.Sprint(asMap(t, sessionResp["result"])["sessionId"]))
	if sessionID == "" {
		t.Fatal("session/new returned empty sessionId")
	}

	writeACPMessage(t, stdin, map[string]any{
		"jsonrpc": "2.0",
		"id":      "3",
		"method":  "session/prompt",
		"params": map[string]any{
			"sessionId": sessionID,
			"prompt": []map[string]any{
				{"type": "text", "text": "auth please"},
			},
		},
	})

	var deltas []string
	for {
		msg := readACPMessage(t, reader, &stderr)
		if result, ok := msg["result"].(map[string]any); ok {
			if stopReason := strings.TrimSpace(fmt.Sprint(result["stopReason"])); stopReason != "completed" {
				t.Fatalf("stopReason = %q, want completed", stopReason)
			}
			break
		}
		update := asMap(t, asMap(t, asMap(t, msg["params"])["update"])["content"])
		deltas = append(deltas, fmt.Sprint(update["text"]))
	}
	if got := strings.TrimSpace(strings.Join(deltas, "")); got != "Header ok" {
		t.Fatalf("joined deltas = %q, want Header ok", got)
	}
}

func moduleRootDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func buildAgentcoreRunnerBinary(t *testing.T) string {
	t.Helper()
	serverDir := moduleRootDir(t)
	bin := filepath.Join(t.TempDir(), "agentcore-runner")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/agentcore-runner")
	cmd.Dir = serverDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build runner failed: %v\n%s", err, output)
	}
	return bin
}

func writeACPMessage(t *testing.T, w io.Writer, payload map[string]any) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal acp payload: %v", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		t.Fatalf("write acp payload: %v", err)
	}
}

func readACPMessage(t *testing.T, r *bufio.Reader, stderr *bytes.Buffer) map[string]any {
	t.Helper()
	done := make(chan map[string]any, 1)
	errCh := make(chan error, 1)
	go func() {
		line, err := r.ReadBytes('\n')
		if err != nil {
			errCh <- err
			return
		}
		var payload map[string]any
		if err := json.Unmarshal(bytes.TrimSpace(line), &payload); err != nil {
			errCh <- err
			return
		}
		done <- payload
	}()
	select {
	case payload := <-done:
		return payload
	case err := <-errCh:
		if stderrText := strings.TrimSpace(stderrBufferText(stderr)); stderrText != "" {
			t.Fatalf("read acp message: %v; stderr=%s", err, stderrText)
		}
		t.Fatalf("read acp message: %v", err)
	case <-time.After(acpMessageTimeout):
		if stderrText := strings.TrimSpace(stderrBufferText(stderr)); stderrText != "" {
			t.Fatalf("timed out waiting for acp message; stderr=%s", stderrText)
		}
		t.Fatal("timed out waiting for acp message")
	}
	return nil
}

func asMap(t *testing.T, value any) map[string]any {
	t.Helper()
	record, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value %T is not a map", value)
	}
	return record
}

func numberAsInt(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
}

func reserveLoopbackAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback addr: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func waitForHTTPReady(t *testing.T, target string, stderr *bytes.Buffer) {
	t.Helper()
	deadline := time.Now().Add(httpReadyTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(target)
		if err == nil {
			resp.Body.Close()
			return
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	stderrText := strings.TrimSpace(stderrBufferText(stderr))
	if lastErr != nil && stderrText != "" {
		t.Fatalf("timed out waiting for %s; last_err=%v stderr=%s", target, lastErr, stderrText)
	}
	if lastErr != nil {
		t.Fatalf("timed out waiting for %s; last_err=%v", target, lastErr)
	}
	if stderrText != "" {
		t.Fatalf("timed out waiting for %s; stderr=%s", target, stderrText)
	}
	t.Fatalf("timed out waiting for %s", target)
}

func stderrBufferText(stderr *bytes.Buffer) string {
	if stderr == nil {
		return ""
	}
	return stderr.String()
}

type stubBlueConversationServer struct {
	server *httptest.Server

	mu                   sync.Mutex
	createConversation   int
	streamMessage        int
	nextMessageResponses []string
	expectedHeaders      map[string]string
}

func newStubBlueConversationServer(t *testing.T, responses []string) *stubBlueConversationServer {
	t.Helper()
	stub := &stubBlueConversationServer{
		nextMessageResponses: append([]string(nil), responses...),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/conversations", func(w http.ResponseWriter, r *http.Request) {
		if err := stub.checkHeaders(r); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		stub.mu.Lock()
		stub.createConversation++
		stub.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"conv-1","title":"Runner Bridge"}`)
	})
	mux.HandleFunc("/api/v1/conversations/conv-1/messages/stream", func(w http.ResponseWriter, r *http.Request) {
		if err := stub.checkHeaders(r); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		stub.mu.Lock()
		stub.streamMessage++
		responseText := ""
		if len(stub.nextMessageResponses) > 0 {
			responseText = stub.nextMessageResponses[0]
			stub.nextMessageResponses = stub.nextMessageResponses[1:]
		}
		stub.mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		for _, chunk := range splitBridgeChunks(responseText) {
			payload, _ := json.Marshal(map[string]any{
				"delta":     chunk,
				"done":      false,
				"stream_id": "stream-1",
			})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			if flusher != nil {
				flusher.Flush()
			}
		}
		payload, _ := json.Marshal(map[string]any{
			"delta":     "",
			"done":      true,
			"stream_id": "stream-1",
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	})
	stub.server = httptest.NewServer(mux)
	return stub
}

func splitBridgeChunks(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := strings.Fields(text)
	if len(parts) <= 1 {
		return []string{text}
	}
	out := make([]string, 0, len(parts))
	for i, part := range parts {
		if i < len(parts)-1 {
			out = append(out, part+" ")
		} else {
			out = append(out, part)
		}
	}
	return out
}

func (s *stubBlueConversationServer) Close() {
	if s != nil && s.server != nil {
		s.server.Close()
	}
}

func (s *stubBlueConversationServer) URL() string {
	if s == nil || s.server == nil {
		return ""
	}
	return s.server.URL
}

func (s *stubBlueConversationServer) CreateConversationCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createConversation
}

func (s *stubBlueConversationServer) StreamMessageCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streamMessage
}

func (s *stubBlueConversationServer) RequireHeader(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.expectedHeaders == nil {
		s.expectedHeaders = make(map[string]string)
	}
	s.expectedHeaders[http.CanonicalHeaderKey(strings.TrimSpace(name))] = strings.TrimSpace(value)
}

func (s *stubBlueConversationServer) checkHeaders(r *http.Request) error {
	s.mu.Lock()
	expected := make(map[string]string, len(s.expectedHeaders))
	for k, v := range s.expectedHeaders {
		expected[k] = v
	}
	s.mu.Unlock()
	for name, want := range expected {
		if got := strings.TrimSpace(r.Header.Get(name)); got != want {
			return fmt.Errorf("missing required header %s", name)
		}
	}
	return nil
}
