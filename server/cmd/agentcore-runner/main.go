package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

func main() {
	if len(os.Args) < 2 {
		usageAndExit()
	}
	switch os.Args[1] {
	case "acp":
		if err := runACP(os.Args[2:], os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
	case "a2a":
		if err := runA2A(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
	default:
		usageAndExit()
	}
}

func usageAndExit() {
	fmt.Fprintln(os.Stderr, "usage: agentcore-runner <acp|a2a> [flags]")
	os.Exit(2)
}

type runnerState struct {
	mu       sync.RWMutex
	sessions map[string]*runnerSession
}

type runnerSession struct {
	ID           string
	RemoteID     string
	LastStreamID string
	LastPrompt   string
	Cancelled    bool
	LastTaskID   string
	LastContext  string
	LastResponse string
}

func newRunnerState() *runnerState {
	return &runnerState{sessions: make(map[string]*runnerSession)}
}

func (s *runnerState) mutateSession(id string, fn func(*runnerSession)) runnerSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current := s.sessions[id]; current != nil {
		if fn != nil {
			fn(current)
		}
		return *current
	}
	session := &runnerSession{
		ID: id,
	}
	if fn != nil {
		fn(session)
	}
	s.sessions[id] = session
	return *session
}

func (s *runnerState) session(id string) (runnerSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current := s.sessions[id]
	if current == nil {
		return runnerSession{}, false
	}
	return *current, true
}

func (s *runnerState) sessionByTaskID(taskID string) (runnerSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, session := range s.sessions {
		if session != nil && strings.TrimSpace(session.LastTaskID) == strings.TrimSpace(taskID) {
			return *session, true
		}
	}
	return runnerSession{}, false
}

type runnerConfig struct {
	BlueURL     string
	BlueHeaders http.Header
}

func parseRunnerConfig(args []string) (runnerConfig, error) {
	cfg := runnerConfig{BlueHeaders: make(http.Header)}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--blue-url":
			if i+1 >= len(args) {
				return runnerConfig{}, fmt.Errorf("--blue-url requires a value")
			}
			cfg.BlueURL = strings.TrimSpace(args[i+1])
			i++
		case "--blue-header":
			if i+1 >= len(args) {
				return runnerConfig{}, fmt.Errorf("--blue-header requires a value")
			}
			name, value, err := parseBlueHeader(strings.TrimSpace(args[i+1]))
			if err != nil {
				return runnerConfig{}, err
			}
			cfg.BlueHeaders.Add(name, value)
			i++
		default:
			return runnerConfig{}, fmt.Errorf("unsupported flag %q", args[i])
		}
	}
	return cfg, nil
}

func parseBlueHeader(raw string) (string, string, error) {
	name, value, ok := strings.Cut(raw, "=")
	name = http.CanonicalHeaderKey(strings.TrimSpace(name))
	value = strings.TrimSpace(value)
	if !ok || name == "" || value == "" {
		return "", "", fmt.Errorf("--blue-header expects Name=Value")
	}
	return name, value, nil
}

type blueBridge struct {
	baseURL string
	client  *http.Client
	headers http.Header
}

func newBlueBridge(cfg runnerConfig) *blueBridge {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BlueURL), "/")
	if baseURL == "" {
		return nil
	}
	return &blueBridge{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 2 * time.Minute},
		headers: cloneHeader(cfg.BlueHeaders),
	}
}

func cloneHeader(src http.Header) http.Header {
	if len(src) == 0 {
		return nil
	}
	out := make(http.Header, len(src))
	for key, values := range src {
		out[key] = append([]string(nil), values...)
	}
	return out
}

func (b *blueBridge) endpoint(path string) string {
	if b == nil {
		return ""
	}
	base := b.baseURL
	if strings.HasSuffix(base, "/api/v1") {
		return base + strings.TrimPrefix(path, "/api/v1")
	}
	return base + path
}

func (b *blueBridge) applyHeaders(req *http.Request) {
	if b == nil || req == nil {
		return
	}
	for key, values := range b.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}

func (b *blueBridge) ensureConversation(ctx context.Context, state *runnerState, sessionID, prompt string) (string, error) {
	if b == nil {
		return "", nil
	}
	if session, ok := state.session(sessionID); ok && strings.TrimSpace(session.RemoteID) != "" {
		return strings.TrimSpace(session.RemoteID), nil
	}
	payload, _ := json.Marshal(map[string]string{
		"title": deriveConversationTitle(prompt),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint("/api/v1/conversations"), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	b.applyHeaders(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("blue create conversation failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var detail map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return "", err
	}
	conversationID := strings.TrimSpace(stringField(detail, "id"))
	if conversationID == "" {
		return "", fmt.Errorf("blue create conversation did not return id")
	}
	state.mutateSession(sessionID, func(session *runnerSession) {
		session.RemoteID = conversationID
	})
	return conversationID, nil
}

func (b *blueBridge) streamPrompt(ctx context.Context, conversationID, prompt string, onDelta func(string) error) (string, string, error) {
	if b == nil {
		return "", "", nil
	}
	payload, _ := json.Marshal(map[string]string{"message": prompt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint("/api/v1/conversations/"+url.PathEscape(conversationID)+"/messages/stream"), bytes.NewReader(payload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	b.applyHeaders(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("blue stream failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	var builder strings.Builder
	var streamID string
	doneSeen := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(strings.ToLower(line), "data:") {
			continue
		}
		data := strings.TrimSpace(line[5:])
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			doneSeen = true
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		if current := strings.TrimSpace(stringField(event, "stream_id")); current != "" {
			streamID = current
		}
		if delta, ok := event["delta"].(string); ok && delta != "" {
			builder.WriteString(delta)
			if onDelta != nil {
				if err := onDelta(delta); err != nil {
					return "", streamID, err
				}
			}
		}
		if done, ok := event["done"].(bool); ok && done {
			doneSeen = true
		}
	}
	if err := scanner.Err(); err != nil {
		return "", streamID, err
	}
	text := strings.TrimSpace(builder.String())
	if text == "" && !doneSeen {
		return "", streamID, fmt.Errorf("blue stream returned no response data")
	}
	return text, streamID, nil
}

func (b *blueBridge) runPrompt(ctx context.Context, state *runnerState, sessionID, prompt string, onDelta func(string) error) (string, string, error) {
	if b == nil {
		return "", "", nil
	}
	conversationID, err := b.ensureConversation(ctx, state, sessionID, prompt)
	if err != nil {
		return "", "", err
	}
	return b.streamPrompt(ctx, conversationID, prompt, onDelta)
}

func (b *blueBridge) cancel(ctx context.Context, session runnerSession) error {
	if b == nil || strings.TrimSpace(session.RemoteID) == "" || strings.TrimSpace(session.LastStreamID) == "" {
		return nil
	}
	payload, _ := json.Marshal(map[string]string{
		"stream_id": strings.TrimSpace(session.LastStreamID),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.endpoint("/api/v1/conversations/"+url.PathEscape(session.RemoteID)+"/messages/cancel"), bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	b.applyHeaders(req)
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("blue cancel failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func deriveConversationTitle(prompt string) string {
	title := strings.TrimSpace(prompt)
	if title == "" {
		return "Agentcore Runner Session"
	}
	runes := []rune(title)
	if len(runes) > 48 {
		title = string(runes[:48])
	}
	return title
}

func runACP(args []string, stdin io.Reader, stdout io.Writer) error {
	cfg, err := parseRunnerConfig(args)
	if err != nil {
		return err
	}
	state := newRunnerState()
	bridge := newBlueBridge(cfg)
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
	writer := &acpWriter{writer: stdout}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			return err
		}
		if err := handleACPMessage(state, writer, bridge, payload); err != nil {
			return err
		}
	}
	return scanner.Err()
}

type acpWriter struct {
	mu     sync.Mutex
	writer io.Writer
}

func (w *acpWriter) write(payload map[string]interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.writer.Write(data)
	return err
}

func handleACPMessage(state *runnerState, writer *acpWriter, bridge *blueBridge, payload map[string]interface{}) error {
	method := strings.TrimSpace(stringField(payload, "method"))
	id, hasID := payload["id"]
	params, _ := payload["params"].(map[string]interface{})

	writeResult := func(result map[string]interface{}) error {
		if !hasID {
			return nil
		}
		return writer.write(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"result":  result,
		})
	}
	writeError := func(err error) error {
		if !hasID {
			return err
		}
		return writer.write(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      id,
			"error": map[string]interface{}{
				"code":    -32601,
				"message": err.Error(),
			},
		})
	}

	switch method {
	case "initialize":
		return writeResult(map[string]interface{}{
			"protocolVersion": 1,
			"agentCapabilities": map[string]interface{}{
				"loadSession": true,
			},
			"agentInfo": map[string]interface{}{
				"name":    "agentcore-runner",
				"title":   "Agentcore Runner",
				"version": "0.1.0",
			},
		})
	case "authenticate":
		return writeResult(map[string]interface{}{
			"ok": true,
		})
	case "session/new":
		sessionID := uuid.NewString()
		state.mutateSession(sessionID, nil)
		return writeResult(map[string]interface{}{
			"sessionId": sessionID,
		})
	case "session/load":
		sessionID := strings.TrimSpace(stringField(params, "sessionId"))
		if sessionID == "" {
			return writeError(errors.New("sessionId is required"))
		}
		state.mutateSession(sessionID, nil)
		return writeResult(map[string]interface{}{
			"sessionId": sessionID,
		})
	case "session/prompt":
		sessionID := strings.TrimSpace(stringField(params, "sessionId"))
		if sessionID == "" {
			return writeError(errors.New("sessionId is required"))
		}
		promptText := extractACPPrompt(params["prompt"])
		responseText := "agentcore-runner received: " + promptText
		streamID := ""
		onDelta := func(delta string) error {
			return writer.write(map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "session/update",
				"params": map[string]interface{}{
					"sessionId": sessionID,
					"update": map[string]interface{}{
						"sessionUpdate": "agent_message_chunk",
						"content": map[string]interface{}{
							"text": delta,
						},
					},
				},
			})
		}
		if bridge != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			var err error
			responseText, streamID, err = bridge.runPrompt(ctx, state, sessionID, promptText, onDelta)
			if err != nil {
				return writeError(err)
			}
		} else if err := onDelta(responseText); err != nil {
			return err
		}
		state.mutateSession(sessionID, func(session *runnerSession) {
			session.LastPrompt = promptText
			session.LastResponse = responseText
			session.LastStreamID = streamID
			session.Cancelled = false
		})
		return writeResult(map[string]interface{}{
			"stopReason": "completed",
		})
	case "session/cancel":
		sessionID := strings.TrimSpace(stringField(params, "sessionId"))
		if sessionID == "" {
			return nil
		}
		if bridge != nil {
			if session, ok := state.session(sessionID); ok {
				_ = bridge.cancel(context.Background(), session)
			}
		}
		state.mutateSession(sessionID, func(session *runnerSession) {
			session.Cancelled = true
		})
		if hasID {
			return writeResult(map[string]interface{}{"cancelled": true})
		}
		return nil
	default:
		return writeError(fmt.Errorf("unsupported method: %s", method))
	}
}

func extractACPPrompt(raw interface{}) string {
	items, _ := raw.([]interface{})
	parts := make([]string, 0, len(items))
	for _, item := range items {
		record, _ := item.(map[string]interface{})
		text := strings.TrimSpace(stringField(record, "text"))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func runA2A(args []string) error {
	listenAddr := "127.0.0.1:7788"
	cfg := runnerConfig{BlueHeaders: make(http.Header)}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			if i+1 >= len(args) {
				return fmt.Errorf("--listen requires an address")
			}
			listenAddr = strings.TrimSpace(args[i+1])
			i++
		case "--blue-url":
			if i+1 >= len(args) {
				return fmt.Errorf("--blue-url requires a value")
			}
			cfg.BlueURL = strings.TrimSpace(args[i+1])
			i++
		case "--blue-header":
			if i+1 >= len(args) {
				return fmt.Errorf("--blue-header requires a value")
			}
			name, value, err := parseBlueHeader(strings.TrimSpace(args[i+1]))
			if err != nil {
				return err
			}
			cfg.BlueHeaders.Add(name, value)
			i++
		default:
			return fmt.Errorf("unsupported flag %q", args[i])
		}
	}
	state := newRunnerState()
	handler := newA2AHandler(listenAddr, state, newBlueBridge(cfg))
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	return srv.ListenAndServe()
}

func newA2AHandler(listenAddr string, state *runnerState, bridge *blueBridge) http.Handler {
	mux := http.NewServeMux()
	baseURL := "http://" + listenAddr
	card := map[string]interface{}{
		"name": "agentcore-runner",
		"url":  baseURL,
		"capabilities": map[string]interface{}{
			"streaming": true,
			"cancel":    true,
		},
	}
	cardHandler := func(w http.ResponseWriter, r *http.Request) {
		writeJSONResponse(w, http.StatusOK, card)
	}
	mux.HandleFunc("/v1/card", cardHandler)
	mux.HandleFunc("/.well-known/agent-card.json", cardHandler)
	mux.HandleFunc("/.well-known/agent.json", cardHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONResponse(w, http.StatusOK, card)
			return
		}
		var envelope map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		method := strings.TrimSpace(stringField(envelope, "method"))
		params, _ := envelope["params"].(map[string]interface{})
		switch method {
		case "SendMessage", "message/send":
			taskID, contextID := allocateA2ATaskIdentity(params)
			text, err := runA2ATask(context.Background(), state, bridge, taskID, contextID, params, nil)
			if err != nil {
				writeJSONResponse(w, http.StatusBadGateway, map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      envelope["id"],
					"error": map[string]interface{}{
						"message": err.Error(),
					},
				})
				return
			}
			writeJSONResponse(w, http.StatusOK, map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      envelope["id"],
				"result":  buildA2AResult(taskID, contextID, text, "completed"),
			})
		case "SendStreamingMessage", "message/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}
			taskID, contextID := allocateA2ATaskIdentity(params)
			onDelta := func(delta string) error {
				payload := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      envelope["id"],
					"result":  buildA2AResult(taskID, contextID, delta, "working"),
				}
				data, _ := json.Marshal(payload)
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					return err
				}
				flusher.Flush()
				return nil
			}
			text, err := runA2ATask(r.Context(), state, bridge, taskID, contextID, params, onDelta)
			if err != nil {
				writeJSONResponse(w, http.StatusBadGateway, map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      envelope["id"],
					"error": map[string]interface{}{
						"message": err.Error(),
					},
				})
				return
			}
			payload := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      envelope["id"],
				"result":  buildA2AResult(taskID, contextID, text, "completed"),
			}
			data, _ := json.Marshal(payload)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case "CancelTask", "tasks/cancel":
			taskID := strings.TrimSpace(stringField(params, "id"))
			if bridge != nil {
				if session, ok := state.sessionByTaskID(taskID); ok {
					_ = bridge.cancel(context.Background(), session)
				}
			}
			writeJSONResponse(w, http.StatusOK, map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      envelope["id"],
				"result": map[string]interface{}{
					"id": taskID,
					"status": map[string]interface{}{
						"state": "cancelled",
					},
				},
			})
		default:
			writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      envelope["id"],
				"error": map[string]interface{}{
					"message": "unsupported method: " + method,
				},
			})
		}
	})
	return mux
}

func allocateA2ATaskIdentity(params map[string]interface{}) (string, string) {
	taskID := uuid.NewString()
	contextID := extractA2AContextID(params)
	if contextID == "" {
		contextID = uuid.NewString()
	}
	return taskID, contextID
}

func runA2ATask(ctx context.Context, state *runnerState, bridge *blueBridge, taskID, contextID string, params map[string]interface{}, onDelta func(string) error) (string, error) {
	promptText := extractA2APrompt(params)
	text := "agentcore-runner received: " + promptText
	streamID := ""
	if bridge != nil {
		var err error
		text, streamID, err = bridge.runPrompt(ctx, state, contextID, promptText, onDelta)
		if err != nil {
			return "", err
		}
	} else if onDelta != nil {
		if err := onDelta(text); err != nil {
			return "", err
		}
	}
	state.mutateSession(contextID, func(session *runnerSession) {
		session.LastTaskID = taskID
		session.LastContext = contextID
		session.LastResponse = text
		session.LastPrompt = promptText
		session.LastStreamID = streamID
		session.Cancelled = false
	})
	return text, nil
}

func extractA2APrompt(params map[string]interface{}) string {
	message, _ := params["message"].(map[string]interface{})
	parts, _ := message["parts"].([]interface{})
	items := make([]string, 0, len(parts))
	for _, item := range parts {
		record, _ := item.(map[string]interface{})
		text := strings.TrimSpace(stringField(record, "text"))
		if text != "" {
			items = append(items, text)
		}
	}
	return strings.TrimSpace(strings.Join(items, "\n"))
}

func extractA2AContextID(params map[string]interface{}) string {
	message, _ := params["message"].(map[string]interface{})
	return strings.TrimSpace(stringField(message, "contextId"))
}

func buildA2AResult(taskID, contextID, text, state string) map[string]interface{} {
	return map[string]interface{}{
		"id":        taskID,
		"contextId": contextID,
		"status": map[string]interface{}{
			"state": state,
		},
		"message": map[string]interface{}{
			"taskId":    taskID,
			"contextId": contextID,
			"role":      "assistant",
			"parts": []map[string]interface{}{
				{
					"text": text,
				},
			},
		},
	}
}

func writeJSONResponse(w http.ResponseWriter, status int, payload map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func stringField(record map[string]interface{}, key string) string {
	if record == nil {
		return ""
	}
	if value, ok := record[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}
