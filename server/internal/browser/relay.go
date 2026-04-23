package browser

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// RelayAuthHeader authenticates Blue's loopback relay HTTP and WebSocket requests.
	RelayAuthHeader = "X-Blue-Relay-Token"

	relayExtensionReconnectGrace  = 20 * time.Second
	relayCommandReconnectWait     = 3 * time.Second
	relayExtensionRequestTimeout  = 30 * time.Second
	relayExtensionPingInterval    = 5 * time.Second
	relayGeneratedTokenBytes      = 32
	relayExtensionStatusPath      = "/extension/status"
	relayExtensionWebSocketPath   = "/extension"
	relayCDPWebSocketPath         = "/cdp"
	relayJSONVersionPath          = "/json/version"
	relayJSONVersionPathWithSlash = "/json/version/"
	relayJSONListPath             = "/json/list"
	relayJSONListPathWithSlash    = "/json/list/"
	relayJSONRootPath             = "/json"
	relayJSONRootPathWithSlash    = "/json/"
	relayTargetActivatePathPrefix = "/json/activate/"
	relayTargetClosePathPrefix    = "/json/close/"
)

// RelayInfo describes Blue's built-in browser relay runtime.
type RelayInfo struct {
	Enabled            bool   `json:"enabled"`
	Host               string `json:"host,omitempty"`
	Port               int    `json:"port,omitempty"`
	BaseURL            string `json:"base_url,omitempty"`
	CDPURL             string `json:"cdp_url,omitempty"`
	Token              string `json:"token,omitempty"`
	AuthHeader         string `json:"auth_header,omitempty"`
	ExtensionDir       string `json:"extension_dir,omitempty"`
	ExtensionConnected bool   `json:"extension_connected"`
}

type relaySocket struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (s *relaySocket) WriteJSON(v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteJSON(v)
}

func (s *relaySocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.Close()
}

type relayTargetInfo struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	URL      string `json:"url,omitempty"`
	Attached bool   `json:"attached,omitempty"`
}

type relayConnectedTarget struct {
	SessionID  string
	TargetID   string
	TargetInfo relayTargetInfo
}

type relayCdpCommand struct {
	ID        int64           `json:"id"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
}

type relayCdpResponse struct {
	ID        int64       `json:"id"`
	Result    interface{} `json:"result,omitempty"`
	Error     interface{} `json:"error,omitempty"`
	SessionID string      `json:"sessionId,omitempty"`
}

type relayCdpError struct {
	Message string `json:"message"`
}

type relayCdpEvent struct {
	Method    string      `json:"method"`
	Params    interface{} `json:"params,omitempty"`
	SessionID string      `json:"sessionId,omitempty"`
}

type relayExtensionCommand struct {
	ID     int64  `json:"id"`
	Method string `json:"method"`
	Params struct {
		Method    string          `json:"method"`
		Params    json.RawMessage `json:"params,omitempty"`
		SessionID string          `json:"sessionId,omitempty"`
	} `json:"params"`
}

type relayExtensionEvent struct {
	Method string `json:"method"`
	Params struct {
		Method    string          `json:"method"`
		Params    json.RawMessage `json:"params,omitempty"`
		SessionID string          `json:"sessionId,omitempty"`
	} `json:"params"`
}

type relayPendingExtensionResponse struct {
	ch    chan relayExtensionResult
	timer *time.Timer
}

type relayExtensionResult struct {
	result json.RawMessage
	err    error
}

// RelayServer bridges Chrome extension debugger sessions into a loopback CDP endpoint.
type RelayServer struct {
	cfg          *Config
	extensionDir string

	server   *http.Server
	listener net.Listener

	extensionUpgrader websocket.Upgrader
	cdpUpgrader       websocket.Upgrader

	mu                     sync.RWMutex
	extension              *relaySocket
	cdpClients             map[*relaySocket]struct{}
	connectedTargets       map[string]relayConnectedTarget
	pendingExtension       map[int64]*relayPendingExtensionResponse
	reconnectWaiters       map[chan bool]struct{}
	disconnectCleanupTimer *time.Timer
	closed                 bool
	nextExtensionID        int64
}

// StartRelayServer starts Blue's built-in loopback browser relay.
func StartRelayServer(cfg *Config, extensionDir string) (*RelayServer, error) {
	if cfg == nil || !cfg.RelayEnabled {
		return nil, nil
	}

	host := cfg.RelayHostOrDefault()
	if !isRelayLoopbackHost(host) {
		return nil, fmt.Errorf("browser relay_host must be loopback, got %q", host)
	}

	if strings.TrimSpace(cfg.RelayToken) == "" {
		token, err := generateRelayToken()
		if err != nil {
			return nil, err
		}
		cfg.RelayToken = token
	}

	srv := &RelayServer{
		cfg:              cfg,
		extensionDir:     extensionDir,
		cdpClients:       make(map[*relaySocket]struct{}),
		connectedTargets: make(map[string]relayConnectedTarget),
		pendingExtension: make(map[int64]*relayPendingExtensionResponse),
		reconnectWaiters: make(map[chan bool]struct{}),
	}
	srv.extensionUpgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			return origin == "" || strings.HasPrefix(origin, "chrome-extension://")
		},
	}
	srv.cdpUpgrader = websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool { return true },
	}

	addr := net.JoinHostPort(host, strconv.Itoa(cfg.RelayPortOrDefault()))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv.listener = listener
	srv.server = &http.Server{Handler: srv}

	go func() {
		_ = srv.server.Serve(listener)
	}()

	return srv, nil
}

// Close stops the relay server and all active WebSocket bridges.
func (s *RelayServer) Close() error {
	if s == nil {
		return nil
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	extension := s.extension
	s.extension = nil
	cdpClients := make([]*relaySocket, 0, len(s.cdpClients))
	for client := range s.cdpClients {
		cdpClients = append(cdpClients, client)
	}
	s.cdpClients = make(map[*relaySocket]struct{})
	if s.disconnectCleanupTimer != nil {
		s.disconnectCleanupTimer.Stop()
		s.disconnectCleanupTimer = nil
	}
	s.connectedTargets = make(map[string]relayConnectedTarget)
	s.flushReconnectWaitersLocked(false)
	for id, pending := range s.pendingExtension {
		delete(s.pendingExtension, id)
		if pending.timer != nil {
			pending.timer.Stop()
		}
		pending.ch <- relayExtensionResult{err: errors.New("relay server stopping")}
		close(pending.ch)
	}
	s.mu.Unlock()

	if extension != nil {
		_ = extension.Close()
	}
	for _, client := range cdpClients {
		_ = client.Close()
	}
	if s.server != nil {
		_ = s.server.Close()
	}
	return nil
}

// Info returns the current relay runtime details.
func (s *RelayServer) Info() RelayInfo {
	if s == nil || s.cfg == nil {
		return RelayInfo{}
	}

	host := s.cfg.RelayHostOrDefault()
	port := s.cfg.RelayPortOrDefault()
	if tcpAddr, ok := s.listener.Addr().(*net.TCPAddr); ok && tcpAddr.Port > 0 {
		port = tcpAddr.Port
	}

	s.mu.RLock()
	connected := s.extension != nil
	s.mu.RUnlock()

	baseURL := fmt.Sprintf("http://%s:%d", host, port)
	token := strings.TrimSpace(s.cfg.RelayToken)
	cdpURL := baseURL
	if token != "" {
		cdpURL = baseURL + "?token=" + url.QueryEscape(token)
	}

	return RelayInfo{
		Enabled:            true,
		Host:               host,
		Port:               port,
		BaseURL:            baseURL,
		CDPURL:             cdpURL,
		Token:              token,
		AuthHeader:         RelayAuthHeader,
		ExtensionDir:       s.extensionDir,
		ExtensionConnected: connected,
	}
}

// ServeHTTP serves the relay's loopback HTTP and WebSocket endpoints.
func (s *RelayServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s == nil {
		http.NotFound(w, r)
		return
	}

	if !isRelayLoopbackRemote(r.RemoteAddr) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	s.maybeApplyExtensionCORS(w, r)

	if r.Method == http.MethodOptions {
		s.handleRelayPreflight(w, r)
		return
	}

	switch r.URL.Path {
	case "/":
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte("Blue browser relay OK"))
			return
		}
	case relayExtensionStatusPath:
		if !s.authenticateRelayRequest(w, r) {
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]bool{"connected": s.extensionConnected()})
		return
	case relayJSONVersionPath, relayJSONVersionPathWithSlash:
		if !s.authenticateRelayRequest(w, r) {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.writeRelayJSONVersion(w, r)
		return
	case relayJSONRootPath, relayJSONRootPathWithSlash, relayJSONListPath, relayJSONListPathWithSlash:
		if !s.authenticateRelayRequest(w, r) {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.writeRelayJSONList(w, r)
		return
	case relayExtensionWebSocketPath:
		if !s.authenticateRelayRequest(w, r) {
			return
		}
		s.handleExtensionWebSocket(w, r)
		return
	case relayCDPWebSocketPath:
		if !s.authenticateRelayRequest(w, r) {
			return
		}
		s.handleCDPWebSocket(w, r)
		return
	}

	if strings.HasPrefix(r.URL.Path, relayTargetActivatePathPrefix) {
		if !s.authenticateRelayRequest(w, r) {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleRelayTargetAction(w, r, relayTargetActivatePathPrefix, "Target.activateTarget")
		return
	}
	if strings.HasPrefix(r.URL.Path, relayTargetClosePathPrefix) {
		if !s.authenticateRelayRequest(w, r) {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleRelayTargetAction(w, r, relayTargetClosePathPrefix, "Target.closeTarget")
		return
	}

	http.NotFound(w, r)
}

func (s *RelayServer) writeRelayJSONVersion(w http.ResponseWriter, r *http.Request) {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		info := s.Info()
		host = net.JoinHostPort(info.Host, strconv.Itoa(info.Port))
	}
	payload := map[string]interface{}{
		"Browser":              "Blue/browser-relay",
		"Protocol-Version":     "1.3",
		"webSocketDebuggerUrl": "ws://" + host + relayCDPWebSocketPath,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *RelayServer) writeRelayJSONList(w http.ResponseWriter, r *http.Request) {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		info := s.Info()
		host = net.JoinHostPort(info.Host, strconv.Itoa(info.Port))
	}
	wsURL := "ws://" + host + relayCDPWebSocketPath

	targets := s.snapshotTargets()
	list := make([]map[string]interface{}, 0, len(targets))
	for _, target := range targets {
		info := target.TargetInfo
		list = append(list, map[string]interface{}{
			"id":                   target.TargetID,
			"type":                 relayTargetType(info.Type),
			"title":                info.Title,
			"description":          info.Title,
			"url":                  info.URL,
			"webSocketDebuggerUrl": wsURL,
			"devtoolsFrontendUrl":  "/devtools/inspector.html?ws=" + strings.TrimPrefix(wsURL, "ws://"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *RelayServer) handleRelayTargetAction(w http.ResponseWriter, r *http.Request, prefix, method string) {
	rawTargetID := strings.TrimPrefix(r.URL.Path, prefix)
	targetID, err := url.PathUnescape(strings.TrimSpace(rawTargetID))
	if err != nil || targetID == "" {
		http.Error(w, "invalid targetId", http.StatusBadRequest)
		return
	}

	go func() {
		_, _ = s.sendToExtension(relayExtensionCommand{
			ID:     atomic.AddInt64(&s.nextExtensionID, 1),
			Method: "forwardCDPCommand",
			Params: struct {
				Method    string          `json:"method"`
				Params    json.RawMessage `json:"params,omitempty"`
				SessionID string          `json:"sessionId,omitempty"`
			}{
				Method: method,
				Params: mustMarshalRaw(map[string]string{"targetId": targetID}),
			},
		})
	}()

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (s *RelayServer) handleExtensionWebSocket(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	existing := s.extension
	s.mu.RUnlock()
	if existing != nil {
		http.Error(w, "Extension already connected", http.StatusConflict)
		return
	}

	conn, err := s.extensionUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	socket := &relaySocket{conn: conn}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = socket.Close()
		return
	}
	if s.extension != nil {
		s.mu.Unlock()
		_ = socket.Close()
		return
	}
	s.extension = socket
	if s.disconnectCleanupTimer != nil {
		s.disconnectCleanupTimer.Stop()
		s.disconnectCleanupTimer = nil
	}
	s.flushReconnectWaitersLocked(true)
	s.mu.Unlock()

	go s.runExtensionSocket(socket)
}

func (s *RelayServer) handleCDPWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.cdpUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &relaySocket{conn: conn}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		_ = client.Close()
		return
	}
	knownTargets := make([]relayConnectedTarget, 0, len(s.connectedTargets))
	for _, target := range s.connectedTargets {
		knownTargets = append(knownTargets, target)
	}
	s.cdpClients[client] = struct{}{}
	s.mu.Unlock()

	s.emitTargetsToClient(client, knownTargets, "autoAttach")
	go s.runCDPSocket(client)
}

func (s *RelayServer) runExtensionSocket(socket *relaySocket) {
	pingTicker := time.NewTicker(relayExtensionPingInterval)
	defer pingTicker.Stop()

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-pingTicker.C:
				if err := socket.WriteJSON(map[string]string{"method": "ping"}); err != nil {
					_ = socket.Close()
					return
				}
			}
		}
	}()

	defer func() {
		close(done)
		_ = socket.Close()
		s.onExtensionSocketClosed(socket)
	}()

	for {
		_, payload, err := socket.conn.ReadMessage()
		if err != nil {
			return
		}
		s.handleExtensionMessage(payload)
	}
}

func (s *RelayServer) runCDPSocket(client *relaySocket) {
	defer func() {
		s.mu.Lock()
		delete(s.cdpClients, client)
		s.mu.Unlock()
		_ = client.Close()
	}()

	for {
		_, payload, err := client.conn.ReadMessage()
		if err != nil {
			return
		}

		var cmd relayCdpCommand
		if err := json.Unmarshal(payload, &cmd); err != nil {
			continue
		}
		if cmd.ID == 0 || strings.TrimSpace(cmd.Method) == "" {
			continue
		}
		go s.handleCDPCommand(client, cmd)
	}
}

func (s *RelayServer) handleCDPCommand(client *relaySocket, cmd relayCdpCommand) {
	if relayCommandNeedsExtension(cmd.Method) && !s.extensionConnected() {
		if !s.waitForExtensionReconnect(relayCommandReconnectWait) {
			s.writeCDPResponse(client, relayCdpResponse{
				ID:        cmd.ID,
				SessionID: cmd.SessionID,
				Error: relayCdpError{
					Message: "Chrome extension not connected",
				},
			})
			return
		}
	}

	result, err := s.routeCDPCommand(cmd)
	if err != nil {
		s.pruneStaleTargets(cmd, err)
		s.writeCDPResponse(client, relayCdpResponse{
			ID:        cmd.ID,
			SessionID: cmd.SessionID,
			Error: relayCdpError{
				Message: err.Error(),
			},
		})
		return
	}

	if cmd.Method == "Target.setAutoAttach" && cmd.SessionID == "" {
		s.emitKnownTargetsToClient(client, "autoAttach")
	}
	if cmd.Method == "Target.setDiscoverTargets" {
		var params struct {
			Discover bool `json:"discover"`
		}
		_ = json.Unmarshal(cmd.Params, &params)
		if params.Discover {
			s.emitKnownTargetsToClient(client, "discover")
		}
	}
	if cmd.Method == "Target.attachToTarget" {
		var params struct {
			TargetID string `json:"targetId"`
		}
		_ = json.Unmarshal(cmd.Params, &params)
		if target := s.findTargetByTargetID(params.TargetID); target != nil {
			_ = client.WriteJSON(relayCdpEvent{
				Method: "Target.attachedToTarget",
				Params: map[string]interface{}{
					"sessionId": target.SessionID,
					"targetInfo": relayTargetInfo{
						TargetID: target.TargetInfo.TargetID,
						Type:     relayTargetType(target.TargetInfo.Type),
						Title:    target.TargetInfo.Title,
						URL:      target.TargetInfo.URL,
						Attached: true,
					},
					"waitingForDebugger": false,
				},
			})
		}
	}

	s.writeCDPResponse(client, relayCdpResponse{
		ID:        cmd.ID,
		SessionID: cmd.SessionID,
		Result:    result,
	})
}

func (s *RelayServer) routeCDPCommand(cmd relayCdpCommand) (interface{}, error) {
	switch cmd.Method {
	case "Browser.getVersion":
		return map[string]interface{}{
			"protocolVersion": "1.3",
			"product":         "Chrome/Blue-Relay",
			"revision":        "0",
			"userAgent":       "Blue-Relay",
			"jsVersion":       "V8",
		}, nil
	case "Browser.setDownloadBehavior", "Browser.close", "Target.setAutoAttach", "Target.setDiscoverTargets":
		return map[string]interface{}{}, nil
	case "Target.getTargets":
		targets := s.snapshotTargets()
		targetInfos := make([]relayTargetInfo, 0, len(targets))
		for _, target := range targets {
			info := target.TargetInfo
			info.Attached = true
			info.Type = relayTargetType(info.Type)
			targetInfos = append(targetInfos, info)
		}
		return map[string]interface{}{"targetInfos": targetInfos}, nil
	case "Target.getTargetInfo":
		var params struct {
			TargetID string `json:"targetId"`
		}
		_ = json.Unmarshal(cmd.Params, &params)
		if params.TargetID != "" {
			if target := s.findTargetByTargetID(params.TargetID); target != nil {
				return map[string]interface{}{"targetInfo": target.TargetInfo}, nil
			}
		}
		if cmd.SessionID != "" {
			if target := s.findTargetBySessionID(cmd.SessionID); target != nil {
				return map[string]interface{}{"targetInfo": target.TargetInfo}, nil
			}
		}
		targets := s.snapshotTargets()
		if len(targets) == 0 {
			return map[string]interface{}{"targetInfo": relayTargetInfo{}}, nil
		}
		return map[string]interface{}{"targetInfo": targets[0].TargetInfo}, nil
	case "Target.attachToTarget":
		var params struct {
			TargetID string `json:"targetId"`
		}
		_ = json.Unmarshal(cmd.Params, &params)
		if params.TargetID == "" {
			return nil, errors.New("targetId required")
		}
		target := s.findTargetByTargetID(params.TargetID)
		if target == nil {
			return nil, errors.New("target not found")
		}
		return map[string]interface{}{"sessionId": target.SessionID}, nil
	default:
		extensionCommand := relayExtensionCommand{
			ID:     atomic.AddInt64(&s.nextExtensionID, 1),
			Method: "forwardCDPCommand",
		}
		extensionCommand.Params.Method = cmd.Method
		extensionCommand.Params.Params = cmd.Params
		extensionCommand.Params.SessionID = cmd.SessionID

		raw, err := s.sendToExtension(extensionCommand)
		if err != nil {
			return nil, err
		}
		if len(raw) == 0 || string(raw) == "null" {
			return map[string]interface{}{}, nil
		}

		var decoded interface{}
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, err
		}
		if decoded == nil {
			return map[string]interface{}{}, nil
		}
		return decoded, nil
	}
}

func (s *RelayServer) sendToExtension(command relayExtensionCommand) (json.RawMessage, error) {
	socket := s.currentExtension()
	if socket == nil {
		return nil, errors.New("Chrome extension not connected")
	}

	respCh := make(chan relayExtensionResult, 1)
	timer := time.AfterFunc(relayExtensionRequestTimeout, func() {
		s.mu.Lock()
		pending, ok := s.pendingExtension[command.ID]
		if ok {
			delete(s.pendingExtension, command.ID)
		}
		s.mu.Unlock()
		if ok {
			pending.ch <- relayExtensionResult{err: fmt.Errorf("extension request timeout: %s", command.Params.Method)}
			close(pending.ch)
		}
	})

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		timer.Stop()
		return nil, errors.New("relay server stopping")
	}
	s.pendingExtension[command.ID] = &relayPendingExtensionResponse{ch: respCh, timer: timer}
	s.mu.Unlock()

	if err := socket.WriteJSON(command); err != nil {
		timer.Stop()
		s.mu.Lock()
		delete(s.pendingExtension, command.ID)
		s.mu.Unlock()
		return nil, err
	}

	result := <-respCh
	return result.result, result.err
}

func (s *RelayServer) handleExtensionMessage(payload []byte) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return
	}

	if rawID, ok := envelope["id"]; ok {
		var id int64
		if err := json.Unmarshal(rawID, &id); err == nil {
			if _, hasResult := envelope["result"]; hasResult || envelope["error"] != nil {
				s.resolvePendingExtension(id, envelope["result"], envelope["error"])
				return
			}
		}
	}

	var method string
	if rawMethod, ok := envelope["method"]; ok {
		_ = json.Unmarshal(rawMethod, &method)
	}
	switch method {
	case "pong":
		return
	case "forwardCDPEvent":
		var event relayExtensionEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return
		}
		s.handleForwardCDPEvent(event)
	}
}

func (s *RelayServer) resolvePendingExtension(id int64, result json.RawMessage, rawErr json.RawMessage) {
	s.mu.Lock()
	pending, ok := s.pendingExtension[id]
	if ok {
		delete(s.pendingExtension, id)
	}
	s.mu.Unlock()
	if !ok {
		return
	}
	if pending.timer != nil {
		pending.timer.Stop()
	}

	reply := relayExtensionResult{result: result}
	if len(rawErr) > 0 && string(rawErr) != "null" {
		var errText string
		if err := json.Unmarshal(rawErr, &errText); err == nil && strings.TrimSpace(errText) != "" {
			reply.err = errors.New(errText)
		} else {
			reply.err = errors.New(string(rawErr))
		}
	}

	pending.ch <- reply
	close(pending.ch)
}

func (s *RelayServer) handleForwardCDPEvent(event relayExtensionEvent) {
	method := strings.TrimSpace(event.Params.Method)
	if method == "" {
		return
	}

	switch method {
	case "Target.attachedToTarget":
		var attached struct {
			SessionID          string          `json:"sessionId"`
			TargetInfo         relayTargetInfo `json:"targetInfo"`
			WaitingForDebugger bool            `json:"waitingForDebugger"`
		}
		if err := json.Unmarshal(event.Params.Params, &attached); err == nil {
			if relayTargetType(attached.TargetInfo.Type) != "page" || attached.SessionID == "" || attached.TargetInfo.TargetID == "" {
				return
			}

			attached.TargetInfo.Type = relayTargetType(attached.TargetInfo.Type)
			prev, changed := s.upsertConnectedTarget(relayConnectedTarget{
				SessionID:  attached.SessionID,
				TargetID:   attached.TargetInfo.TargetID,
				TargetInfo: attached.TargetInfo,
			})
			if changed && prev != nil && prev.TargetID != attached.TargetInfo.TargetID {
				s.broadcastCDPEvent(relayCdpEvent{
					Method: "Target.detachedFromTarget",
					Params: map[string]interface{}{
						"sessionId": attached.SessionID,
						"targetId":  prev.TargetID,
					},
					SessionID: attached.SessionID,
				})
			}
			if changed {
				s.broadcastCDPEvent(relayCdpEvent{
					Method:    method,
					Params:    mustUnmarshalAny(event.Params.Params),
					SessionID: event.Params.SessionID,
				})
			}
			return
		}
	case "Target.detachedFromTarget":
		var detached struct {
			SessionID string `json:"sessionId"`
			TargetID  string `json:"targetId"`
		}
		_ = json.Unmarshal(event.Params.Params, &detached)
		if detached.SessionID != "" {
			s.dropConnectedTargetSession(detached.SessionID)
		} else if detached.TargetID != "" {
			s.dropConnectedTargetsByTargetID(detached.TargetID)
		}
	case "Target.targetDestroyed", "Target.targetCrashed":
		var payload struct {
			TargetID string `json:"targetId"`
		}
		_ = json.Unmarshal(event.Params.Params, &payload)
		if payload.TargetID != "" {
			s.dropConnectedTargetsByTargetID(payload.TargetID)
		}
	case "Target.targetInfoChanged":
		var payload struct {
			TargetInfo relayTargetInfo `json:"targetInfo"`
		}
		_ = json.Unmarshal(event.Params.Params, &payload)
		if payload.TargetInfo.TargetID != "" && relayTargetType(payload.TargetInfo.Type) == "page" {
			s.refreshTargetInfo(payload.TargetInfo.TargetID, payload.TargetInfo)
		}
	}

	s.broadcastCDPEvent(relayCdpEvent{
		Method:    method,
		Params:    mustUnmarshalAny(event.Params.Params),
		SessionID: event.Params.SessionID,
	})
}

func (s *RelayServer) onExtensionSocketClosed(socket *relaySocket) {
	s.mu.Lock()
	if s.extension != socket {
		s.mu.Unlock()
		return
	}
	s.extension = nil
	for id, pending := range s.pendingExtension {
		delete(s.pendingExtension, id)
		if pending.timer != nil {
			pending.timer.Stop()
		}
		pending.ch <- relayExtensionResult{err: errors.New("Chrome extension disconnected")}
		close(pending.ch)
	}
	if s.disconnectCleanupTimer != nil {
		s.disconnectCleanupTimer.Stop()
	}
	s.disconnectCleanupTimer = time.AfterFunc(relayExtensionReconnectGrace, func() {
		s.handleExtensionDisconnectGraceExpired()
	})
	s.mu.Unlock()
}

func (s *RelayServer) handleExtensionDisconnectGraceExpired() {
	s.mu.Lock()
	if s.closed || s.extension != nil {
		s.mu.Unlock()
		return
	}
	cdpClients := make([]*relaySocket, 0, len(s.cdpClients))
	for client := range s.cdpClients {
		cdpClients = append(cdpClients, client)
	}
	s.connectedTargets = make(map[string]relayConnectedTarget)
	s.flushReconnectWaitersLocked(false)
	s.mu.Unlock()

	for _, client := range cdpClients {
		_ = client.Close()
	}
}

func (s *RelayServer) emitKnownTargetsToClient(client *relaySocket, mode string) {
	targets := s.snapshotTargets()
	s.emitTargetsToClient(client, targets, mode)
}

func (s *RelayServer) emitTargetsToClient(client *relaySocket, targets []relayConnectedTarget, mode string) {
	for _, target := range targets {
		info := target.TargetInfo
		info.Attached = true
		if mode == "autoAttach" {
			_ = client.WriteJSON(relayCdpEvent{
				Method: "Target.attachedToTarget",
				Params: map[string]interface{}{
					"sessionId":          target.SessionID,
					"targetInfo":         info,
					"waitingForDebugger": false,
				},
			})
			continue
		}
		_ = client.WriteJSON(relayCdpEvent{
			Method: "Target.targetCreated",
			Params: map[string]interface{}{
				"targetInfo": info,
			},
		})
	}
}

func (s *RelayServer) writeCDPResponse(client *relaySocket, res relayCdpResponse) {
	_ = client.WriteJSON(res)
}

func (s *RelayServer) broadcastCDPEvent(evt relayCdpEvent) {
	s.mu.RLock()
	clients := make([]*relaySocket, 0, len(s.cdpClients))
	for client := range s.cdpClients {
		clients = append(clients, client)
	}
	s.mu.RUnlock()

	for _, client := range clients {
		_ = client.WriteJSON(evt)
	}
}

func (s *RelayServer) waitForExtensionReconnect(timeout time.Duration) bool {
	if s.extensionConnected() {
		return true
	}

	ch := make(chan bool, 1)
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false
	}
	if s.extension != nil {
		s.mu.Unlock()
		return true
	}
	s.reconnectWaiters[ch] = struct{}{}
	s.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case ok := <-ch:
		return ok
	case <-timer.C:
		s.mu.Lock()
		delete(s.reconnectWaiters, ch)
		s.mu.Unlock()
		return false
	}
}

func (s *RelayServer) flushReconnectWaitersLocked(connected bool) {
	for ch := range s.reconnectWaiters {
		ch <- connected
		close(ch)
		delete(s.reconnectWaiters, ch)
	}
}

func (s *RelayServer) extensionConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.extension != nil
}

func (s *RelayServer) currentExtension() *relaySocket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.extension
}

func (s *RelayServer) snapshotTargets() []relayConnectedTarget {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]relayConnectedTarget, 0, len(s.connectedTargets))
	for _, target := range s.connectedTargets {
		out = append(out, target)
	}
	return out
}

func (s *RelayServer) findTargetByTargetID(targetID string) *relayConnectedTarget {
	if strings.TrimSpace(targetID) == "" {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, target := range s.connectedTargets {
		if target.TargetID == targetID {
			copied := target
			return &copied
		}
	}
	return nil
}

func (s *RelayServer) findTargetBySessionID(sessionID string) *relayConnectedTarget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	target, ok := s.connectedTargets[sessionID]
	if !ok {
		return nil
	}
	copied := target
	return &copied
}

func (s *RelayServer) upsertConnectedTarget(target relayConnectedTarget) (*relayConnectedTarget, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev, existed := s.connectedTargets[target.SessionID]
	s.connectedTargets[target.SessionID] = target
	if !existed {
		return nil, true
	}
	if prev.TargetID != target.TargetID || prev.TargetInfo.Title != target.TargetInfo.Title || prev.TargetInfo.URL != target.TargetInfo.URL {
		copied := prev
		return &copied, true
	}
	return &prev, false
}

func (s *RelayServer) dropConnectedTargetSession(sessionID string) *relayConnectedTarget {
	s.mu.Lock()
	defer s.mu.Unlock()
	target, ok := s.connectedTargets[sessionID]
	if !ok {
		return nil
	}
	delete(s.connectedTargets, sessionID)
	copied := target
	return &copied
}

func (s *RelayServer) dropConnectedTargetsByTargetID(targetID string) []relayConnectedTarget {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := make([]relayConnectedTarget, 0)
	for sessionID, target := range s.connectedTargets {
		if target.TargetID != targetID {
			continue
		}
		delete(s.connectedTargets, sessionID)
		removed = append(removed, target)
	}
	return removed
}

func (s *RelayServer) refreshTargetInfo(targetID string, info relayTargetInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for sessionID, target := range s.connectedTargets {
		if target.TargetID != targetID {
			continue
		}
		if info.Title != "" {
			target.TargetInfo.Title = info.Title
		}
		if info.URL != "" {
			target.TargetInfo.URL = info.URL
		}
		if info.Type != "" {
			target.TargetInfo.Type = relayTargetType(info.Type)
		}
		s.connectedTargets[sessionID] = target
	}
}

func (s *RelayServer) pruneStaleTargets(cmd relayCdpCommand, err error) {
	if !isRelayMissingTargetError(err) {
		return
	}
	if cmd.SessionID != "" {
		s.dropConnectedTargetSession(cmd.SessionID)
		return
	}
	var params struct {
		TargetID string `json:"targetId"`
	}
	_ = json.Unmarshal(cmd.Params, &params)
	if params.TargetID != "" {
		s.dropConnectedTargetsByTargetID(params.TargetID)
	}
}

func (s *RelayServer) authenticateRelayRequest(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(s.cfg.RelayToken)
	if expected == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}

	token := strings.TrimSpace(r.Header.Get(RelayAuthHeader))
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("token"))
	}
	if token != expected {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *RelayServer) maybeApplyExtensionCORS(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if strings.HasPrefix(origin, "chrome-extension://") {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
}

func (s *RelayServer) handleRelayPreflight(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" && !strings.HasPrefix(origin, "chrome-extension://") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	requestedHeaders := strings.Split(strings.TrimSpace(r.Header.Get("Access-Control-Request-Headers")), ",")
	allowedHeaders := []string{"content-type", strings.ToLower(RelayAuthHeader)}
	for _, header := range requestedHeaders {
		header = strings.TrimSpace(strings.ToLower(header))
		if header != "" {
			allowedHeaders = append(allowedHeaders, header)
		}
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	if origin == "" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(uniqueStrings(allowedHeaders), ", "))
	w.Header().Set("Access-Control-Max-Age", "86400")
	w.Header().Set("Vary", "Origin, Access-Control-Request-Headers")
	w.WriteHeader(http.StatusNoContent)
}

func generateRelayToken() (string, error) {
	buf := make([]byte, relayGeneratedTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isRelayLoopbackHost(host string) bool {
	trimmed := strings.Trim(strings.TrimSpace(host), "[]")
	switch strings.ToLower(trimmed) {
	case "localhost":
		return true
	}
	ip := net.ParseIP(trimmed)
	return ip != nil && ip.IsLoopback()
}

func isRelayLoopbackRemote(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err != nil {
		host = strings.TrimSpace(remoteAddr)
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func relayTargetType(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "page"
	}
	return trimmed
}

func isRelayMissingTargetError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "target not found") ||
		strings.Contains(msg, "no target with given id") ||
		strings.Contains(msg, "session not found") ||
		strings.Contains(msg, "cannot find session")
}

func mustMarshalRaw(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func mustUnmarshalAny(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func relayCommandNeedsExtension(method string) bool {
	switch strings.TrimSpace(method) {
	case "Browser.getVersion",
		"Browser.setDownloadBehavior",
		"Browser.close",
		"Target.setAutoAttach",
		"Target.setDiscoverTargets",
		"Target.getTargets",
		"Target.getTargetInfo",
		"Target.attachToTarget":
		return false
	default:
		return true
	}
}
