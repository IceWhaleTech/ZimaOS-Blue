package sockipc

import (
	"context"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestServerPingPong(t *testing.T) {
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	srv.Handle("ping", func(_ context.Context, _ *Request) *Response {
		return OkResponse(map[string]string{"pong": "1"})
	})

	startServerOrSkip(t, srv)
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	resp := sendRecv(t, conn, &Request{Cmd: "ping"})
	if resp.Status != "ok" {
		t.Errorf("status = %q, want ok", resp.Status)
	}
	if resp.Data["pong"] != "1" {
		t.Errorf("pong = %q, want 1", resp.Data["pong"])
	}
}

func TestServerUnknownCmd(t *testing.T) {
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	startServerOrSkip(t, srv)
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	resp := sendRecv(t, conn, &Request{Cmd: "nonexistent"})
	if resp.Status != "error" {
		t.Errorf("status = %q, want error", resp.Status)
	}
}

func TestServerMissingCmd(t *testing.T) {
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	startServerOrSkip(t, srv)
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	resp := sendRecv(t, conn, &Request{})
	if resp.Error != "missing cmd" {
		t.Errorf("error = %q, want 'missing cmd'", resp.Error)
	}
}

func TestServerMultipleMessages(t *testing.T) {
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())

	callCount := 0
	srv.Handle("inc", func(_ context.Context, _ *Request) *Response {
		callCount++
		return OkResponse(nil)
	})

	startServerOrSkip(t, srv)
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	for i := 0; i < 3; i++ {
		resp := sendRecv(t, conn, &Request{Cmd: "inc"})
		if resp.Status != "ok" {
			t.Errorf("iteration %d: status = %q", i, resp.Status)
		}
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestServerStaleSocketCleanup(t *testing.T) {
	sock := shortSock(t)

	// Create a stale socket file
	os.WriteFile(sock, []byte("stale"), 0644)

	srv := NewServer(sock, zap.NewNop())
	srv.Handle("ping", func(_ context.Context, _ *Request) *Response {
		return OkResponse(nil)
	})

	startServerOrSkip(t, srv)
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

func TestServerCloseDisconnectsIdleClients(t *testing.T) {
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	startServerOrSkip(t, srv)

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	done := make(chan error, 1)
	go func() {
		done <- srv.Close()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close() blocked with an idle client connection")
	}
}

func TestServerStreamHandlerWritesMultipleResponses(t *testing.T) {
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	srv.HandleStream("watch", func(ctx context.Context, req *Request, conn net.Conn) error {
		if got := req.Params["conversation_id"]; got != "conv-stream" {
			t.Fatalf("conversation_id = %q, want conv-stream", got)
		}
		if err := WriteJSON(conn, OkResponse(map[string]string{"mode": "snapshot", "seq": "1"})); err != nil {
			return err
		}
		return WriteJSON(conn, OkResponse(map[string]string{"mode": "event", "seq": "2"}))
	})

	startServerOrSkip(t, srv)
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := WriteJSON(conn, &Request{
		Cmd:    "watch",
		Params: map[string]string{"conversation_id": "conv-stream"},
	}); err != nil {
		t.Fatalf("WriteJSON(request) error = %v", err)
	}

	resp1, err := ReadJSON[Response](conn)
	if err != nil {
		t.Fatalf("ReadJSON(resp1) error = %v", err)
	}
	if got := resp1.Data["seq"]; got != "1" {
		t.Fatalf("first response seq = %q, want 1", got)
	}

	resp2, err := ReadJSON[Response](conn)
	if err != nil {
		t.Fatalf("ReadJSON(resp2) error = %v", err)
	}
	if got := resp2.Data["seq"]; got != "2" {
		t.Fatalf("second response seq = %q, want 2", got)
	}

	if _, err := ReadJSON[Response](conn); err != io.EOF {
		t.Fatalf("final read error = %v, want io.EOF", err)
	}
}
