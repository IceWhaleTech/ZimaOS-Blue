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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	initResp := readACPMessage(t, reader)
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
	sessionResp := readACPMessage(t, reader)
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
	updateMsg := readACPMessage(t, reader)
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

	promptResp := readACPMessage(t, reader)
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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

	waitForHTTPReady(t, "http://"+listenAddr+"/v1/card")
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
	deadline := time.After(5 * time.Second)
	for firstData == "" {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for SSE data")
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

func readACPMessage(t *testing.T, r *bufio.Reader) map[string]any {
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
		t.Fatalf("read acp message: %v", err)
	case <-time.After(5 * time.Second):
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

func waitForHTTPReady(t *testing.T, target string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(target)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", target)
}
