package agentsessions

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type A2ARuntime struct {
	client *http.Client
	logger *zap.Logger

	mu       sync.Mutex
	sessions map[string]*a2aSessionState
}

type a2aSessionState struct {
	endpoint         string
	card             map[string]interface{}
	currentRunID     string
	currentRunStream *runtimeStreamImpl
	cancel           context.CancelFunc
	remoteContextID  string
	remoteTaskID     string
}

func NewA2ARuntime(logger *zap.Logger) *A2ARuntime {
	return &A2ARuntime{
		client: &http.Client{Timeout: 60 * time.Second},
		logger: logger,
		sessions: make(map[string]*a2aSessionState),
	}
}

func (r *A2ARuntime) VerifyProfile(ctx context.Context, profile AgentProfile) (*ProfileVerifyResult, error) {
	card, endpoint, err := r.resolveCard(ctx, profile)
	if err != nil {
		return nil, err
	}
	return &ProfileVerifyResult{
		OK:           true,
		MessageCode:  ProfileVerifyMessageCodeProfileVerified,
		Message:      "A2A profile verified",
		Capabilities: []string{"message/send", "message/stream", "tasks/cancel"},
		Details: map[string]interface{}{
			"card":     card,
			"endpoint": endpoint,
		},
	}, nil
}

func (r *A2ARuntime) EnsureSession(ctx context.Context, req EnsureSessionRequest) (*EnsureSessionResult, error) {
	card, endpoint, err := r.resolveCard(ctx, req.Profile)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.sessions[req.Session.ID] = &a2aSessionState{
		endpoint:        endpoint,
		card:            card,
		remoteContextID: req.Session.RemoteSessionID,
	}
	r.mu.Unlock()
	return &EnsureSessionResult{
		RemoteSessionID: req.Session.RemoteSessionID,
		Metadata: map[string]interface{}{
			"endpoint": endpoint,
			"card":     card,
		},
	}, nil
}

func (r *A2ARuntime) SubmitRun(ctx context.Context, req SubmitRunRequest) (*SubmitRunResult, error) {
	r.mu.Lock()
	state := r.sessions[req.Session.ID]
	r.mu.Unlock()
	if state == nil {
		return nil, fmt.Errorf("A2A session is not initialized")
	}

	stream := newRuntimeStream(64)
	runCtx, cancel := context.WithCancel(ctx)
	r.mu.Lock()
	state.currentRunID = req.Run.ID
	state.currentRunStream = stream
	state.cancel = cancel
	r.mu.Unlock()

	go func() {
		defer func() {
			r.mu.Lock()
			current := r.sessions[req.Session.ID]
			if current != nil {
				current.currentRunID = ""
				current.currentRunStream = nil
				current.cancel = nil
			}
			r.mu.Unlock()
		}()

		payload := buildA2ARequestPayload(req.Prompt, state.remoteContextID)
		taskID, contextID, err := r.streamMessage(runCtx, req.Profile, state, payload, stream)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "unsupported") {
				taskID, contextID, err = r.sendMessage(runCtx, req.Profile, state, payload, stream)
			}
		}
		if contextID != "" {
			r.mu.Lock()
			if current := r.sessions[req.Session.ID]; current != nil {
				current.remoteContextID = contextID
			}
			r.mu.Unlock()
		}
		if taskID != "" {
			r.mu.Lock()
			if current := r.sessions[req.Session.ID]; current != nil {
				current.remoteTaskID = taskID
			}
			r.mu.Unlock()
		}
		if err != nil {
			stream.push(RuntimeEvent{Type: "error", Error: err.Error()})
			stream.finish(err)
			return
		}
		stream.push(RuntimeEvent{Type: "done"})
		stream.finish(nil)
	}()

	return &SubmitRunResult{RemoteRunID: req.Run.ID}, nil
}

func (r *A2ARuntime) StreamRun(_ context.Context, req StreamRunRequest) (RuntimeStream, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state := r.sessions[req.Session.ID]
	if state == nil || state.currentRunID != req.Run.ID || state.currentRunStream == nil {
		return nil, fmt.Errorf("A2A run stream is not available")
	}
	return state.currentRunStream, nil
}

func (r *A2ARuntime) CancelRun(ctx context.Context, session ExternalSession, run ExternalRun) error {
	r.mu.Lock()
	state := r.sessions[session.ID]
	r.mu.Unlock()
	if state == nil {
		return nil
	}
	if state.cancel != nil {
		state.cancel()
	}
	taskID := strings.TrimSpace(run.RemoteRunID)
	if taskID == "" {
		taskID = state.remoteTaskID
	}
	if taskID == "" {
		return nil
	}
	_, err := r.callRPC(ctx, state.endpoint, sessionHeaders(nil), []string{"CancelTask", "tasks/cancel"}, map[string]interface{}{
		"id": taskID,
	})
	return err
}

func (r *A2ARuntime) CloseSession(_ context.Context, _ AgentProfile, session ExternalSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, session.ID)
	return nil
}

func (r *A2ARuntime) Health(ctx context.Context, profile AgentProfile) (*ProfileHealthResult, error) {
	_, endpoint, err := r.resolveCard(ctx, profile)
	if err != nil {
		return &ProfileHealthResult{Healthy: false, Message: err.Error()}, nil
	}
	return &ProfileHealthResult{
		Healthy:     true,
		MessageCode: ProfileHealthMessageCodeRuntimeHealthy,
		Message:     "A2A runtime is healthy",
		Details:     map[string]interface{}{"endpoint": endpoint},
	}, nil
}

func (r *A2ARuntime) resolveCard(ctx context.Context, profile AgentProfile) (map[string]interface{}, string, error) {
	headers := sessionHeaders(profile.Headers)
	candidates := buildA2ACardCandidates(profile)
	for _, candidate := range candidates {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
		if err != nil {
			continue
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := r.client.Do(req)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			continue
		}
		var card map[string]interface{}
		if err := json.Unmarshal(body, &card); err != nil {
			continue
		}
		endpoint := strings.TrimSpace(stringField(card, "url"))
		if endpoint == "" {
			endpoint = strings.TrimSpace(profile.EndpointURL)
		}
		if endpoint == "" {
			if parsed, err := url.Parse(candidate); err == nil {
				parsed.Path = "/"
				_jsii := parsed.String()
				endpoint = strings.TrimRight(_jsii, "/")
			}
		}
		return card, endpoint, nil
	}
	return nil, "", fmt.Errorf("failed to discover A2A agent card")
}

func buildA2ACardCandidates(profile AgentProfile) []string {
	seen := map[string]struct{}{}
	var candidates []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}
	add(profile.CardURL)
	base := strings.TrimSpace(profile.EndpointURL)
	if base != "" {
		add(strings.TrimRight(base, "/") + "/v1/card")
		add(strings.TrimRight(base, "/") + "/.well-known/agent-card.json")
		add(strings.TrimRight(base, "/") + "/.well-known/agent.json")
	}
	return candidates
}

func buildA2ARequestPayload(prompt, contextID string) map[string]interface{} {
	params := map[string]interface{}{
		"message": map[string]interface{}{
			"role": "user",
			"parts": []map[string]interface{}{
				{
					"kind": "text",
					"text": prompt,
				},
				{
					"text":      prompt,
					"mediaType": "text/plain",
				},
			},
		},
		"metadata": map[string]interface{}{},
	}
	if contextID != "" {
		params["message"].(map[string]interface{})["contextId"] = contextID
	}
	return params
}

func (r *A2ARuntime) streamMessage(ctx context.Context, profile AgentProfile, state *a2aSessionState, params map[string]interface{}, stream *runtimeStreamImpl) (string, string, error) {
	headers := sessionHeaders(profile.Headers)
	methods := []string{"SendStreamingMessage", "message/stream"}
	var lastErr error
	for _, method := range methods {
		payload, _ := buildRPCRequest([]string{method}, params)
		body, _ := json.Marshal(payload)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, state.endpoint, bytes.NewReader(body))
		if err != nil {
			return "", "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 300 {
			payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			lastErr = fmt.Errorf("%s failed with status %d: %s", method, resp.StatusCode, strings.TrimSpace(string(payload)))
			continue
		}
		if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
			payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			resp.Body.Close()
			taskID, contextID := parseA2AJSONPayload(payload, stream)
			if taskID != "" || contextID != "" {
				return taskID, contextID, nil
			}
			lastErr = fmt.Errorf("%s did not return an SSE stream", method)
			continue
		}

		var taskID string
		var contextID string
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 4<<20)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, ":") {
				continue
			}
			if !strings.HasPrefix(strings.ToLower(line), "data:") {
				continue
			}
			chunk := strings.TrimSpace(line[5:])
			if chunk == "" {
				continue
			}
			parsedTaskID, parsedContextID, err := parseA2AChunk([]byte(chunk), stream)
			if err != nil {
				if r.logger != nil {
					r.logger.Debug("A2A stream chunk parse failed", zap.Error(err))
				}
				continue
			}
			if parsedTaskID != "" {
				taskID = parsedTaskID
			}
			if parsedContextID != "" {
				contextID = parsedContextID
			}
		}
		resp.Body.Close()
		if err := scanner.Err(); err != nil {
			lastErr = err
			continue
		}
		return taskID, contextID, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("A2A streaming methods are unavailable")
	}
	return "", "", lastErr
}

func (r *A2ARuntime) sendMessage(ctx context.Context, profile AgentProfile, state *a2aSessionState, params map[string]interface{}, stream *runtimeStreamImpl) (string, string, error) {
	headers := sessionHeaders(profile.Headers)
	result, err := r.callRPC(ctx, state.endpoint, headers, []string{"SendMessage", "message/send"}, params)
	if err != nil {
		return "", "", err
	}
	taskID, contextID := emitA2AResult(result, stream)
	return taskID, contextID, nil
}

func (r *A2ARuntime) callRPC(ctx context.Context, endpoint string, headers map[string]string, methods []string, params map[string]interface{}) (map[string]interface{}, error) {
	var lastErr error
	for _, method := range methods {
		payload, _ := buildRPCRequest([]string{method}, params)
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 300 {
			payload, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
			resp.Body.Close()
			lastErr = fmt.Errorf("%s failed with status %d: %s", method, resp.StatusCode, strings.TrimSpace(string(payload)))
			continue
		}
		var envelope map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			resp.Body.Close()
			lastErr = err
			continue
		}
		resp.Body.Close()
		if errPayload, ok := envelope["error"].(map[string]interface{}); ok {
			lastErr = fmt.Errorf("%s", stringField(errPayload, "message"))
			continue
		}
		if result, ok := envelope["result"].(map[string]interface{}); ok {
			return result, nil
		}
		return map[string]interface{}{}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("A2A RPC methods are unavailable")
	}
	return nil, lastErr
}

func buildRPCRequest(methods []string, params map[string]interface{}) (map[string]interface{}, string) {
	method := ""
	if len(methods) > 0 {
		method = methods[0]
	}
	return map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  method,
		"params":  params,
	}, method
}

func parseA2AChunk(payload []byte, stream *runtimeStreamImpl) (string, string, error) {
	var envelope map[string]interface{}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return "", "", err
	}
	if result, ok := envelope["result"].(map[string]interface{}); ok {
		taskID, contextID := emitA2AResult(result, stream)
		return taskID, contextID, nil
	}
	return "", "", nil
}

func parseA2AJSONPayload(payload []byte, stream *runtimeStreamImpl) (string, string) {
	var envelope map[string]interface{}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return "", ""
	}
	if result, ok := envelope["result"].(map[string]interface{}); ok {
		taskID, contextID := emitA2AResult(result, stream)
		return taskID, contextID
	}
	return "", ""
}

func emitA2AResult(result map[string]interface{}, stream *runtimeStreamImpl) (string, string) {
	taskID, contextID := extractA2ATaskRefs(result)
	if state := extractA2AState(result); state != "" {
		stream.push(RuntimeEvent{Type: "status", Status: state, Text: state})
	}
	for _, text := range extractA2ATextParts(result) {
		stream.push(RuntimeEvent{Type: "assistant_delta", Role: "assistant", Text: text})
	}
	return taskID, contextID
}

func extractA2ATaskRefs(result map[string]interface{}) (string, string) {
	if task, ok := result["task"].(map[string]interface{}); ok {
		return stringField(task, "id"), stringField(task, "contextId")
	}
	if message, ok := result["message"].(map[string]interface{}); ok {
		return stringField(message, "taskId"), stringField(message, "contextId")
	}
	return stringField(result, "id"), stringField(result, "contextId")
}

func extractA2AState(result map[string]interface{}) string {
	if task, ok := result["task"].(map[string]interface{}); ok {
		if status, ok := task["status"].(map[string]interface{}); ok {
			return stringField(status, "state")
		}
	}
	if status, ok := result["status"].(map[string]interface{}); ok {
		return stringField(status, "state")
	}
	return ""
}

func extractA2ATextParts(value interface{}) []string {
	switch typed := value.(type) {
	case map[string]interface{}:
		if text := stringField(typed, "text"); text != "" {
			return []string{text}
		}
		var parts []string
		for _, key := range []string{"message", "artifact", "artifacts", "history", "parts", "task"} {
			if child, ok := typed[key]; ok {
				parts = append(parts, extractA2ATextParts(child)...)
			}
		}
		return parts
	case []interface{}:
		var parts []string
		for _, item := range typed {
			parts = append(parts, extractA2ATextParts(item)...)
		}
		return parts
	default:
		return nil
	}
}

func sessionHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[key] = value
	}
	return out
}
