package browser

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestStartRelayServerRejectsNonLoopbackHost(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RelayEnabled = true
	cfg.RelayHost = "0.0.0.0"
	cfg.RelayPort = freeRelayPort(t)
	cfg.RelayToken = "test-token"

	relay, err := StartRelayServer(cfg, "")
	if err == nil {
		_ = relay.Close()
		t.Fatal("StartRelayServer() error = nil, want non-loopback rejection")
	}
}

func TestRelayServerJSONVersionRequiresToken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RelayEnabled = true
	cfg.RelayPort = freeRelayPort(t)
	cfg.RelayToken = "test-token"

	relay, err := StartRelayServer(cfg, "")
	if err != nil {
		t.Fatalf("StartRelayServer() error = %v", err)
	}
	defer func() { _ = relay.Close() }()

	info := relay.Info()

	resp, err := http.Get(info.BaseURL + "/json/version")
	if err != nil {
		t.Fatalf("GET /json/version without token error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("GET /json/version without token status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}

	req, err := http.NewRequest(http.MethodGet, info.BaseURL+"/json/version", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set(RelayAuthHeader, info.Token)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /json/version with token error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /json/version with token status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode /json/version response error = %v", err)
	}
	if got := payload["webSocketDebuggerUrl"]; got != "ws://"+net.JoinHostPort(info.Host, itoa(info.Port))+"/cdp" {
		t.Fatalf("webSocketDebuggerUrl = %v, want ws://%s/cdp", got, net.JoinHostPort(info.Host, itoa(info.Port)))
	}
}

func TestRelayServerBridgesExtensionAndCDP(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RelayEnabled = true
	cfg.RelayPort = freeRelayPort(t)
	cfg.RelayToken = "test-token"

	relay, err := StartRelayServer(cfg, "")
	if err != nil {
		t.Fatalf("StartRelayServer() error = %v", err)
	}
	defer func() { _ = relay.Close() }()

	info := relay.Info()
	extensionConn := mustDialRelayWS(t, "ws://"+net.JoinHostPort(info.Host, itoa(info.Port))+relayExtensionWebSocketPath+"?token="+info.Token)
	defer func() { _ = extensionConn.Close() }()

	cdpConn := mustDialRelayWS(t, "ws://"+net.JoinHostPort(info.Host, itoa(info.Port))+relayCDPWebSocketPath+"?token="+info.Token)
	defer func() { _ = cdpConn.Close() }()

	if err := extensionConn.WriteJSON(map[string]interface{}{
		"method": "forwardCDPEvent",
		"params": map[string]interface{}{
			"method": "Target.attachedToTarget",
			"params": map[string]interface{}{
				"sessionId": "blue-tab-1",
				"targetInfo": map[string]interface{}{
					"targetId": "target-1",
					"type":     "page",
					"title":    "Cart",
					"url":      "https://shop.example/cart",
					"attached": true,
				},
				"waitingForDebugger": false,
			},
		},
	}); err != nil {
		t.Fatalf("extension WriteJSON(attach event) error = %v", err)
	}

	event := readJSONMessage(t, cdpConn, func(msg map[string]interface{}) bool {
		return msg["method"] == "Target.attachedToTarget"
	})
	params, _ := event["params"].(map[string]interface{})
	if got := params["sessionId"]; got != "blue-tab-1" {
		t.Fatalf("attached event sessionId = %v, want %v", got, "blue-tab-1")
	}

	if err := cdpConn.WriteJSON(map[string]interface{}{
		"id":     1,
		"method": "Target.getTargets",
	}); err != nil {
		t.Fatalf("cdp WriteJSON(Target.getTargets) error = %v", err)
	}
	resp := readJSONMessage(t, cdpConn, func(msg map[string]interface{}) bool {
		id, ok := msg["id"].(float64)
		return ok && int(id) == 1
	})
	result, _ := resp["result"].(map[string]interface{})
	targetInfos, _ := result["targetInfos"].([]interface{})
	if len(targetInfos) != 1 {
		t.Fatalf("len(targetInfos) = %d, want 1", len(targetInfos))
	}

	if err := cdpConn.WriteJSON(map[string]interface{}{
		"id":     2,
		"method": "Blue.echo",
		"params": map[string]interface{}{"hello": "world"},
	}); err != nil {
		t.Fatalf("cdp WriteJSON(Blue.echo) error = %v", err)
	}

	forwarded := readJSONMessage(t, extensionConn, func(msg map[string]interface{}) bool {
		return msg["method"] == "forwardCDPCommand"
	})
	if got := int(forwarded["id"].(float64)); got == 0 {
		t.Fatalf("forwarded command id = %d, want non-zero", got)
	}
	forwardedParams, _ := forwarded["params"].(map[string]interface{})
	if got := forwardedParams["method"]; got != "Blue.echo" {
		t.Fatalf("forwarded method = %v, want %v", got, "Blue.echo")
	}

	if err := extensionConn.WriteJSON(map[string]interface{}{
		"id":     forwarded["id"],
		"result": map[string]interface{}{"ok": true},
	}); err != nil {
		t.Fatalf("extension WriteJSON(command result) error = %v", err)
	}

	resp = readJSONMessage(t, cdpConn, func(msg map[string]interface{}) bool {
		id, ok := msg["id"].(float64)
		return ok && int(id) == 2
	})
	result, _ = resp["result"].(map[string]interface{})
	if got := result["ok"]; got != true {
		t.Fatalf("forwarded result ok = %v, want true", got)
	}
}

func TestEnsureRelayExtensionDirWritesAssets(t *testing.T) {
	root := t.TempDir()
	dir, err := EnsureRelayExtensionDir(root)
	if err != nil {
		t.Fatalf("EnsureRelayExtensionDir() error = %v", err)
	}

	for _, name := range []string{"manifest.json", "background.js", "background-utils.js", "options.html", "options.js", "options-validation.js"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected extracted asset %q: %v", name, err)
		}
	}
}

func freeRelayPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen(127.0.0.1:0) error = %v", err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

func mustDialRelayWS(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(rawURL, nil)
	if err != nil {
		t.Fatalf("Dial(%q) error = %v", rawURL, err)
	}
	return conn
}

func readJSONMessage(t *testing.T, conn *websocket.Conn, match func(map[string]interface{}) bool) map[string]interface{} {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for {
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatalf("SetReadDeadline() error = %v", err)
		}
		_, payload, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if match(msg) {
			return msg
		}
	}
}

func itoa(v int) string {
	return strconv.Itoa(v)
}
