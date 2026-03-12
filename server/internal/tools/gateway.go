package tools

import (
	"context"
	"errors"
	"sort"
	"time"

	gatewayruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
)

// GatewayConfigInfo is the normalized gateway runtime configuration exposed to tools.
type GatewayConfigInfo struct {
	Enabled               bool  `json:"enabled"`
	ReadBufferSize        int   `json:"read_buffer_size"`
	WriteBufferSize       int   `json:"write_buffer_size"`
	MaxMessageSize        int64 `json:"max_message_size"`
	PingIntervalSeconds   int   `json:"ping_interval_seconds"`
	PongTimeoutSeconds    int   `json:"pong_timeout_seconds"`
	WriteTimeoutSeconds   int   `json:"write_timeout_seconds"`
	RequestTimeoutSeconds int   `json:"request_timeout_seconds"`
	MaxConnections        int   `json:"max_connections"`
}

// GatewayConnectionInfo is the normalized gateway connection view used by tools.
type GatewayConnectionInfo struct {
	ID               string                 `json:"id"`
	UserID           string                 `json:"user_id,omitempty"`
	ConnectedAt      time.Time              `json:"connected_at"`
	MessagesSent     int64                  `json:"messages_sent"`
	MessagesReceived int64                  `json:"messages_received"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// GatewayService provides stable access to gateway runtime info and controls.
type GatewayService interface {
	Stats(ctx context.Context) map[string]interface{}
	Config(ctx context.Context) GatewayConfigInfo
	Methods(ctx context.Context) []string
	ListConnections(ctx context.Context) ([]GatewayConnectionInfo, error)
	GetConnection(ctx context.Context, id string) (*GatewayConnectionInfo, error)
	CloseConnection(ctx context.Context, id string) error
}

// GatewayTool inspects and manages the local gateway runtime.
type GatewayTool struct {
	service  GatewayService
	registry *Registry
}

// NewGatewayTool creates a native gateway tool.
func NewGatewayTool(service GatewayService) *GatewayTool {
	return &GatewayTool{service: service}
}

// SetRegistry enables exec fallback for unsupported lifecycle actions.
func (t *GatewayTool) SetRegistry(registry *Registry) {
	t.registry = registry
}

// Definition returns the tool schema.
func (t *GatewayTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "gateway",
		Description: "Inspect gateway runtime status, registered methods, and active connections. Can also close a stuck connection.",
		Icon:        "gateway",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"status", "list", "get", "close", "start", "stop", "restart", "run", "install", "uninstall"},
					"description": "Gateway action. Defaults to status, or get when an id is provided. Lifecycle actions fall back to exec.",
				},
				"id":              map[string]interface{}{"type": "string", "description": "Gateway connection ID for get/close."},
				"user_id":         map[string]interface{}{"type": "string", "description": "Optional user ID filter for connections."},
				"limit":           map[string]interface{}{"type": "integer", "description": "Maximum connections to return (default 20, max 200)."},
				"offset":          map[string]interface{}{"type": "integer", "description": "Pagination offset (default 0)."},
				"include_methods": map[string]interface{}{"type": "boolean", "description": "Include registered gateway methods in status output (default true)."},
			},
		},
	}
}

// Execute dispatches the requested gateway action.
func (t *GatewayTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t == nil || t.service == nil {
		return nil, errors.New("gateway service not available")
	}
	action := gatewayAction(args)
	switch action {
	case "status":
		return t.executeStatus(ctx, args)
	case "list":
		return t.executeList(ctx, args)
	case "get":
		return t.executeGet(ctx, args)
	case "close":
		return t.executeClose(ctx, args)
	case "start", "stop", "restart", "run", "install", "uninstall":
		return t.executeLifecycleFallback(ctx, action)
	default:
		return nil, errors.New("unsupported gateway action")
	}
}

func (t *GatewayTool) executeStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	connections, count, err := t.filteredConnections(ctx, args)
	if err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"stats":             t.service.Stats(ctx),
		"config":            t.service.Config(ctx),
		"connections":       connections,
		"connection_count":  count,
		"supported_actions": []string{"status", "list", "get", "close"},
	}
	includeMethods, ok := compatBoolArg(args, "include_methods", "includeMethods")
	if !ok || includeMethods {
		methods := t.service.Methods(ctx)
		result["methods"] = methods
		result["method_count"] = len(methods)
	}
	return result, nil
}

func (t *GatewayTool) executeList(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	connections, count, err := t.filteredConnections(ctx, args)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"connections": connections,
		"count":       count,
		"user_id":     firstCompatString(args, "user_id", "user"),
	}, nil
}

func (t *GatewayTool) executeGet(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "connection_id")
	if id == "" {
		return nil, errors.New("id is required")
	}
	connection, err := t.service.GetConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, errors.New("connection not found")
	}
	return map[string]interface{}{
		"connection": connection,
	}, nil
}

func (t *GatewayTool) executeClose(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	id := firstCompatString(args, "id", "connection_id")
	if id == "" {
		return nil, errors.New("id is required")
	}
	connection, err := t.service.GetConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, errors.New("connection not found")
	}
	if err := t.service.CloseConnection(ctx, id); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"closed":     true,
		"id":         id,
		"connection": connection,
	}, nil
}

func (t *GatewayTool) executeLifecycleFallback(ctx context.Context, action string) (interface{}, error) {
	if t.registry == nil {
		return nil, errors.New("gateway lifecycle actions require exec tool")
	}
	execTool := t.registry.Get("exec")
	if execTool == nil {
		return nil, errors.New("gateway lifecycle actions require exec tool")
	}
	return execTool.Execute(ctx, map[string]interface{}{"command": "blue gateway " + action})
}

func (t *GatewayTool) filteredConnections(ctx context.Context, args map[string]interface{}) ([]GatewayConnectionInfo, int, error) {
	connections, err := t.service.ListConnections(ctx)
	if err != nil {
		return nil, 0, err
	}
	userID := firstCompatString(args, "user_id", "user")
	filtered := make([]GatewayConnectionInfo, 0, len(connections))
	for _, connection := range connections {
		if userID != "" && connection.UserID != userID {
			continue
		}
		filtered = append(filtered, connection)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].ConnectedAt.After(filtered[j].ConnectedAt)
	})
	count := len(filtered)
	limit, err := fsAsInt(args, "limit", 20)
	if err != nil {
		return nil, 0, err
	}
	offset, err := fsAsInt(args, "offset", 0)
	if err != nil {
		return nil, 0, err
	}
	limit = fsClamp(limit, 1, 200)
	if offset < 0 {
		offset = 0
	}
	if offset >= len(filtered) {
		return []GatewayConnectionInfo{}, count, nil
	}
	end := len(filtered)
	if offset+limit < end {
		end = offset + limit
	}
	return filtered[offset:end], count, nil
}

func gatewayAction(args map[string]interface{}) string {
	action := firstCompatString(args, "action", "op", "operation", "command")
	if action != "" {
		return action
	}
	if firstCompatString(args, "id", "connection_id") != "" {
		return "get"
	}
	return "status"
}

type gatewayServiceAdapter struct {
	runtime *gatewayruntime.Gateway
}

func (a gatewayServiceAdapter) Stats(_ context.Context) map[string]interface{} {
	if a.runtime == nil {
		return map[string]interface{}{}
	}
	return a.runtime.Stats()
}

func (a gatewayServiceAdapter) Config(_ context.Context) GatewayConfigInfo {
	if a.runtime == nil {
		return GatewayConfigInfo{}
	}
	cfg := a.runtime.Config()
	return GatewayConfigInfo{
		Enabled:               cfg.Enabled,
		ReadBufferSize:        cfg.ReadBufferSize,
		WriteBufferSize:       cfg.WriteBufferSize,
		MaxMessageSize:        cfg.MaxMessageSize,
		PingIntervalSeconds:   cfg.PingIntervalSeconds,
		PongTimeoutSeconds:    cfg.PongTimeoutSeconds,
		WriteTimeoutSeconds:   cfg.WriteTimeoutSeconds,
		RequestTimeoutSeconds: cfg.RequestTimeoutSeconds,
		MaxConnections:        cfg.MaxConnections,
	}
}

func (a gatewayServiceAdapter) Methods(_ context.Context) []string {
	if a.runtime == nil {
		return nil
	}
	return a.runtime.Methods()
}

func (a gatewayServiceAdapter) ListConnections(_ context.Context) ([]GatewayConnectionInfo, error) {
	if a.runtime == nil {
		return nil, nil
	}
	connections := a.runtime.GetConnections()
	result := make([]GatewayConnectionInfo, 0, len(connections))
	for _, connection := range connections {
		if connection == nil {
			continue
		}
		result = append(result, gatewayConnectionInfo(connection))
	}
	return result, nil
}

func (a gatewayServiceAdapter) GetConnection(_ context.Context, id string) (*GatewayConnectionInfo, error) {
	if a.runtime == nil {
		return nil, nil
	}
	connection, ok := a.runtime.GetConnection(id)
	if !ok || connection == nil {
		return nil, nil
	}
	info := gatewayConnectionInfo(connection)
	return &info, nil
}

func (a gatewayServiceAdapter) CloseConnection(_ context.Context, id string) error {
	if a.runtime == nil {
		return errors.New("gateway service not available")
	}
	connection, ok := a.runtime.GetConnection(id)
	if !ok || connection == nil {
		return errors.New("connection not found")
	}
	connection.Close()
	return nil
}

func gatewayConnectionInfo(connection *gatewayruntime.Connection) GatewayConnectionInfo {
	metadata := connection.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	return GatewayConnectionInfo{
		ID:               connection.ID,
		UserID:           connection.UserID,
		ConnectedAt:      connection.ConnectedAt,
		MessagesSent:     connection.MessagesSent.Load(),
		MessagesReceived: connection.MessagesReceived.Load(),
		Metadata:         metadata,
	}
}

// RegisterGatewayTool registers the native gateway tool when a runtime exists.
func RegisterGatewayTool(registry *Registry, runtime *gatewayruntime.Gateway) {
	if registry == nil || runtime == nil {
		return
	}
	tool := NewGatewayTool(gatewayServiceAdapter{runtime: runtime})
	tool.SetRegistry(registry)
	registry.Register(tool)
}
