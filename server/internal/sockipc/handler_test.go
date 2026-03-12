package sockipc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"go.uber.org/zap"
)

// shortSock returns a short socket path to avoid macOS 104-byte limit.
func shortSock(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "sipc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "s.sock")
}

// helper: start server with media handlers, return client conn + cleanup
func setupMediaServer(t *testing.T, generate MediaGenerator, query MediaStatusQuerier) (net.Conn, func()) {
	t.Helper()
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	RegisterMediaHandlers(srv, generate, query, zap.NewNop())
	startServerOrSkip(t, srv)
	conn, err := net.Dial("unix", sock)
	if err != nil {
		srv.Close()
		t.Fatal(err)
	}
	return conn, func() { conn.Close(); srv.Close() }
}

func sendRecv(t *testing.T, conn net.Conn, req *Request) *Response {
	t.Helper()
	if err := WriteJSON(conn, req); err != nil {
		t.Fatal(err)
	}
	resp, err := ReadJSON[Response](conn)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestHandlerPing(t *testing.T) {
	conn, cleanup := setupMediaServer(t, nil, nil)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "ping"})
	if resp.Status != "ok" {
		t.Errorf("status = %q", resp.Status)
	}
}

func TestHandlerMediaGenerateSuccess(t *testing.T) {
	gen := func(_ context.Context, category, model, prompt string, params map[string]string) (string, error) {
		if prompt != "a cat" {
			t.Errorf("prompt = %q", prompt)
		}
		if category != "t2i" {
			t.Errorf("category = %q", category)
		}
		return "task-123", nil
	}
	conn, cleanup := setupMediaServer(t, gen, nil)
	defer cleanup()

	req := &Request{
		Cmd:    "media.generate",
		Params: map[string]string{"prompt": "a cat", "category": "t2i", "model": "nano-banana-pro"},
	}
	resp := sendRecv(t, conn, req)

	if resp.Status != "ok" {
		t.Errorf("status = %q, error = %q", resp.Status, resp.Error)
	}
	if resp.Data["task_id"] != "task-123" {
		t.Errorf("task_id = %q", resp.Data["task_id"])
	}
}

func TestHandlerMediaGenerateMissingPrompt(t *testing.T) {
	conn, cleanup := setupMediaServer(t, nil, nil)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "media.generate"})
	if resp.Status != "error" {
		t.Errorf("status = %q", resp.Status)
	}
	if resp.Error != "missing prompt" {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestHandlerMediaStatus(t *testing.T) {
	query := func(_ context.Context, taskID string) (map[string]string, error) {
		if taskID == "task-123" {
			return map[string]string{
				"task_id":     "task-123",
				"task_status": "succeeded",
				"progress":    "1.0",
			}, nil
		}
		return nil, nil
	}
	conn, cleanup := setupMediaServer(t, nil, query)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "media.status", Params: map[string]string{"task_id": "task-123"}})
	if resp.Status != "ok" {
		t.Errorf("status = %q", resp.Status)
	}
	if resp.Data["task_status"] != "succeeded" {
		t.Errorf("task_status = %q", resp.Data["task_status"])
	}
}

func TestHandlerMediaStatusNotFound(t *testing.T) {
	query := func(_ context.Context, taskID string) (map[string]string, error) {
		return nil, nil
	}
	conn, cleanup := setupMediaServer(t, nil, query)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "media.status", Params: map[string]string{"task_id": "nope"}})
	if resp.Error != "task not found" {
		t.Errorf("error = %q", resp.Error)
	}
}

func TestHandlerDefaultCategory(t *testing.T) {
	var gotCategory string
	gen := func(_ context.Context, category, model, prompt string, params map[string]string) (string, error) {
		gotCategory = category
		return "task-1", nil
	}
	conn, cleanup := setupMediaServer(t, gen, nil)
	defer cleanup()

	sendRecv(t, conn, &Request{Cmd: "media.generate", Params: map[string]string{"prompt": "test"}})
	if gotCategory != "t2i" {
		t.Errorf("default category = %q, want t2i", gotCategory)
	}
}

func TestHandlerMediaGeneratePropagatesBlueUserID(t *testing.T) {
	var gotUserID string
	gen := func(ctx context.Context, category, model, prompt string, params map[string]string) (string, error) {
		gotUserID = tools.GetUserID(ctx)
		return "task-1", nil
	}
	conn, cleanup := setupMediaServer(t, gen, nil)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "media.generate", Params: map[string]string{"prompt": "test", "__blue_user_id": "user-123"}})
	if resp.Status != "ok" {
		t.Fatalf("status = %q, error = %q", resp.Status, resp.Error)
	}
	if gotUserID != "user-123" {
		t.Fatalf("user_id = %q, want user-123", gotUserID)
	}
}

func TestHandlerMediaStatusPropagatesBlueUserID(t *testing.T) {
	var gotUserID string
	query := func(ctx context.Context, taskID string) (map[string]string, error) {
		gotUserID = tools.GetUserID(ctx)
		return map[string]string{
			"task_id":     taskID,
			"task_status": "succeeded",
			"progress":    "1.0",
		}, nil
	}
	conn, cleanup := setupMediaServer(t, nil, query)
	defer cleanup()

	resp := sendRecv(t, conn, &Request{Cmd: "media.status", Params: map[string]string{"task_id": "task-123", "__blue_user_id": "user-123"}})
	if resp.Status != "ok" {
		t.Fatalf("status = %q, error = %q", resp.Status, resp.Error)
	}
	if gotUserID != "user-123" {
		t.Fatalf("user_id = %q, want user-123", gotUserID)
	}
}
