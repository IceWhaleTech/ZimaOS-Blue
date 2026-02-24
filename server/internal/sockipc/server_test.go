package sockipc

import (
	"context"
	"net"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestServerPingPong(t *testing.T) {
	sock := shortSock(t)
	srv := NewServer(sock, zap.NewNop())
	srv.Handle("ping", func(_ context.Context, _ *Request) *Response {
		return OkResponse(map[string]string{"pong": "1"})
	})

	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
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
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
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
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
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

	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
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

	if err := srv.Start(); err != nil {
		t.Fatalf("Start failed with stale socket: %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}
