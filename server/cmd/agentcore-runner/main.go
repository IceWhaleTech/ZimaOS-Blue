package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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
		if err := runACP(os.Stdin, os.Stdout); err != nil {
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
	LastPrompt   string
	Cancelled    bool
	LastTaskID   string
	LastContext  string
	LastResponse string
}

func newRunnerState() *runnerState {
	return &runnerState{sessions: make(map[string]*runnerSession)}
}

func (s *runnerState) ensureSession(id string) *runnerSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current := s.sessions[id]; current != nil {
		return current
	}
	session := &runnerSession{
		ID:       id,
		RemoteID: id,
	}
	s.sessions[id] = session
	return session
}

func (s *runnerState) session(id string) *runnerSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessions[id]
}

func runACP(stdin io.Reader, stdout io.Writer) error {
	state := newRunnerState()
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
		if err := handleACPMessage(state, writer, payload); err != nil {
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

func handleACPMessage(state *runnerState, writer *acpWriter, payload map[string]interface{}) error {
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
		state.ensureSession(sessionID)
		return writeResult(map[string]interface{}{
			"sessionId": sessionID,
		})
	case "session/load":
		sessionID := strings.TrimSpace(stringField(params, "sessionId"))
		if sessionID == "" {
			return writeError(errors.New("sessionId is required"))
		}
		state.ensureSession(sessionID)
		return writeResult(map[string]interface{}{
			"sessionId": sessionID,
		})
	case "session/prompt":
		sessionID := strings.TrimSpace(stringField(params, "sessionId"))
		if sessionID == "" {
			return writeError(errors.New("sessionId is required"))
		}
		session := state.ensureSession(sessionID)
		promptText := extractACPPrompt(params["prompt"])
		session.LastPrompt = promptText
		session.LastResponse = "agentcore-runner received: " + promptText
		session.Cancelled = false
		if err := writer.write(map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "session/update",
			"params": map[string]interface{}{
				"sessionId": sessionID,
				"update": map[string]interface{}{
					"sessionUpdate": "agent_message_chunk",
					"content": map[string]interface{}{
						"text": session.LastResponse,
					},
				},
			},
		}); err != nil {
			return err
		}
		return writeResult(map[string]interface{}{
			"stopReason": "completed",
		})
	case "session/cancel":
		sessionID := strings.TrimSpace(stringField(params, "sessionId"))
		if sessionID == "" {
			return nil
		}
		session := state.ensureSession(sessionID)
		session.Cancelled = true
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
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			if i+1 >= len(args) {
				return fmt.Errorf("--listen requires an address")
			}
			listenAddr = strings.TrimSpace(args[i+1])
			i++
		default:
			return fmt.Errorf("unsupported flag %q", args[i])
		}
	}
	state := newRunnerState()
	handler := newA2AHandler(listenAddr, state)
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

func newA2AHandler(listenAddr string, state *runnerState) http.Handler {
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
			taskID, contextID, text := runA2ATask(state, params)
			writeJSONResponse(w, http.StatusOK, map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      envelope["id"],
				"result": buildA2AResult(taskID, contextID, text, "completed"),
			})
		case "SendStreamingMessage", "message/stream":
			taskID, contextID, text := runA2ATask(state, params)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			flusher, ok := w.(http.Flusher)
			if !ok {
				http.Error(w, "streaming unsupported", http.StatusInternalServerError)
				return
			}
			payload := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      envelope["id"],
				"result": buildA2AResult(taskID, contextID, text, "completed"),
			}
			data, _ := json.Marshal(payload)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case "CancelTask", "tasks/cancel":
			taskID := strings.TrimSpace(stringField(params, "id"))
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

func runA2ATask(state *runnerState, params map[string]interface{}) (string, string, string) {
	taskID := uuid.NewString()
	contextID := uuid.NewString()
	text := "agentcore-runner received: " + extractA2APrompt(params)
	session := state.ensureSession(contextID)
	session.LastTaskID = taskID
	session.LastContext = contextID
	session.LastResponse = text
	return taskID, contextID, text
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

func buildA2AResult(taskID, contextID, text, state string) map[string]interface{} {
	return map[string]interface{}{
		"id": taskID,
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
