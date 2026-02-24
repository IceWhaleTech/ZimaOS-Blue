package sockipc

import (
	"context"
	"net"
	"sync"
	"testing"

	"go.uber.org/zap"
)

// TestE2EGenerateAndPoll exercises the full client flow:
// connect → media.generate → media.status (poll until terminal).
func TestE2EGenerateAndPoll(t *testing.T) {
	var mu sync.Mutex
	tasks := map[string]string{} // taskID → status

	gen := func(_ context.Context, category, model, prompt string, params map[string]string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		tasks["task-e2e"] = "processing"
		return "task-e2e", nil
	}

	query := func(_ context.Context, taskID string) (map[string]string, error) {
		mu.Lock()
		defer mu.Unlock()
		st, ok := tasks[taskID]
		if !ok {
			return nil, nil
		}
		return map[string]string{
			"task_id":     taskID,
			"task_status": st,
			"progress":    "0.50",
		}, nil
	}

	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	RegisterMediaHandlers(srv, gen, query, zap.NewNop())
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Step 1: generate
	resp := sendRecv(t, conn, &Request{
		Cmd:    "media.generate",
		Params: map[string]string{"prompt": "a sunset over mountains", "category": "t2i"},
	})
	if resp.Status != "ok" {
		t.Fatalf("generate status = %q, error = %q", resp.Status, resp.Error)
	}
	taskID := resp.Data["task_id"]
	if taskID != "task-e2e" {
		t.Fatalf("task_id = %q", taskID)
	}

	// Step 2: poll — first poll returns "processing"
	resp = sendRecv(t, conn, &Request{
		Cmd:    "media.status",
		Params: map[string]string{"task_id": taskID},
	})
	if resp.Data["task_status"] != "processing" {
		t.Fatalf("poll1 task_status = %q", resp.Data["task_status"])
	}

	// Simulate task completion
	mu.Lock()
	tasks["task-e2e"] = "succeeded"
	mu.Unlock()

	// Step 3: poll again — now "succeeded"
	resp = sendRecv(t, conn, &Request{
		Cmd:    "media.status",
		Params: map[string]string{"task_id": taskID},
	})
	if resp.Data["task_status"] != "succeeded" {
		t.Fatalf("poll2 task_status = %q", resp.Data["task_status"])
	}
}

// TestE2EParamPassthrough tests that extra params (size, message_id, session_id)
// are passed through to the generator.
func TestE2EParamPassthrough(t *testing.T) {
	var capturedParams map[string]string
	gen := func(_ context.Context, category, model, prompt string, params map[string]string) (string, error) {
		capturedParams = params
		return "task-params", nil
	}

	query := func(_ context.Context, taskID string) (map[string]string, error) {
		return map[string]string{
			"task_id":     taskID,
			"task_status": "succeeded",
			"progress":    "1.00",
		}, nil
	}

	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	RegisterMediaHandlers(srv, gen, query, zap.NewNop())
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Generate with extra params
	resp := sendRecv(t, conn, &Request{
		Cmd: "media.generate",
		Params: map[string]string{
			"prompt":     "a dragon",
			"category":   "t2v",
			"model":      "wan-2.1",
			"size":       "1280x720",
			"message_id": "msg-99",
			"session_id": "sess-1",
		},
	})
	if resp.Status != "ok" {
		t.Fatalf("status = %q, error = %q", resp.Status, resp.Error)
	}
	if resp.Data["task_id"] != "task-params" {
		t.Fatalf("task_id = %q", resp.Data["task_id"])
	}

	// Verify params were passed through
	if capturedParams["size"] != "1280x720" {
		t.Errorf("size = %q", capturedParams["size"])
	}
	if capturedParams["message_id"] != "msg-99" {
		t.Errorf("message_id = %q", capturedParams["message_id"])
	}

	// Status check
	resp = sendRecv(t, conn, &Request{
		Cmd:    "media.status",
		Params: map[string]string{"task_id": "task-params"},
	})
	if resp.Data["task_status"] != "succeeded" {
		t.Fatalf("task_status = %q", resp.Data["task_status"])
	}
}
