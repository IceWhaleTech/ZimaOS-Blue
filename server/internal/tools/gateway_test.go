package tools

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubGatewayService struct {
	stats       map[string]interface{}
	config      GatewayConfigInfo
	methods     []string
	connections []GatewayConnectionInfo
	closedIDs   []string
}

func (s *stubGatewayService) Stats(ctx context.Context) map[string]interface{} {
	_ = ctx
	return s.stats
}

func (s *stubGatewayService) Config(ctx context.Context) GatewayConfigInfo {
	_ = ctx
	return s.config
}

func (s *stubGatewayService) Methods(ctx context.Context) []string {
	_ = ctx
	return append([]string(nil), s.methods...)
}

func (s *stubGatewayService) ListConnections(ctx context.Context) ([]GatewayConnectionInfo, error) {
	_ = ctx
	return append([]GatewayConnectionInfo(nil), s.connections...), nil
}

func (s *stubGatewayService) GetConnection(ctx context.Context, id string) (*GatewayConnectionInfo, error) {
	_ = ctx
	for i := range s.connections {
		if s.connections[i].ID == id {
			conn := s.connections[i]
			return &conn, nil
		}
	}
	return nil, nil
}

func (s *stubGatewayService) CloseConnection(ctx context.Context, id string) error {
	_ = ctx
	for _, conn := range s.connections {
		if conn.ID == id {
			s.closedIDs = append(s.closedIDs, id)
			return nil
		}
	}
	return errors.New("connection not found")
}

func TestGatewayToolStatusExecute(t *testing.T) {
	now := time.Now()
	svc := &stubGatewayService{
		stats:   map[string]interface{}{"active_connections": 2},
		config:  GatewayConfigInfo{Enabled: true, MaxConnections: 10},
		methods: []string{"chat.send", "hooks.wake"},
		connections: []GatewayConnectionInfo{
			{ID: "conn_1", UserID: "user_a", ConnectedAt: now.Add(-time.Minute)},
			{ID: "conn_2", UserID: "user_b", ConnectedAt: now},
		},
	}
	tool := NewGatewayTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "status", "limit": 1})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["method_count"].(int) != 2 {
		t.Fatalf("method_count = %v, want 2", payload["method_count"])
	}
	connections := payload["connections"].([]GatewayConnectionInfo)
	if len(connections) != 1 || connections[0].ID != "conn_2" {
		t.Fatalf("unexpected connections: %#v", connections)
	}
	if payload["connection_count"].(int) != 2 {
		t.Fatalf("connection_count = %v, want 2", payload["connection_count"])
	}
}

func TestGatewayToolStatusExecuteSupportsNestedCamelCaseArgs(t *testing.T) {
	svc := &stubGatewayService{
		stats:       map[string]interface{}{"active_connections": 1},
		config:      GatewayConfigInfo{Enabled: true, MaxConnections: 10},
		methods:     []string{"chat.send"},
		connections: []GatewayConnectionInfo{{ID: "conn_1", UserID: "user_a"}},
	}
	tool := NewGatewayTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"action":         "status",
			"includeMethods": false,
			"limit":          1,
			"offset":         0,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if _, ok := payload["methods"]; ok {
		t.Fatalf("expected methods to be omitted, got %#v", payload["methods"])
	}
	if payload["connection_count"].(int) != 1 {
		t.Fatalf("connection_count = %v, want 1", payload["connection_count"])
	}
}

func TestGatewayToolListExecuteFiltersByUser(t *testing.T) {
	svc := &stubGatewayService{
		connections: []GatewayConnectionInfo{{ID: "conn_1", UserID: "user_a"}, {ID: "conn_2", UserID: "user_b"}},
	}
	tool := NewGatewayTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "list", "user_id": "user_b"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	connections := payload["connections"].([]GatewayConnectionInfo)
	if len(connections) != 1 || connections[0].ID != "conn_2" {
		t.Fatalf("unexpected filtered connections: %#v", connections)
	}
}

func TestGatewayToolCloseExecute(t *testing.T) {
	svc := &stubGatewayService{
		connections: []GatewayConnectionInfo{{ID: "conn_1", UserID: "user_a"}},
	}
	tool := NewGatewayTool(svc)
	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "close", "id": "conn_1"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if closed, _ := payload["closed"].(bool); !closed {
		t.Fatalf("expected closed=true, got %#v", payload)
	}
	if len(svc.closedIDs) != 1 || svc.closedIDs[0] != "conn_1" {
		t.Fatalf("unexpected closed IDs: %#v", svc.closedIDs)
	}
}

func TestGatewayToolRestartFallsBackToExecTool(t *testing.T) {
	registry := NewRegistry()
	execTool := &captureArgsTool{
		def: ToolDefinition{
			Name:        "exec",
			Description: "Execute command",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{"type": "string"},
				},
				"required": []string{"command"},
			},
		},
	}
	registry.Register(execTool)
	tool := NewGatewayTool(&stubGatewayService{})
	tool.SetRegistry(registry)
	if _, err := tool.Execute(context.Background(), map[string]interface{}{"action": "restart"}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got, _ := execTool.args["command"].(string); got != "blue gateway restart" {
		t.Fatalf("command = %q, want %q", got, "blue gateway restart")
	}
}
