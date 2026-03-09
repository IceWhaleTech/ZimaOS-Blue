package tools

import (
	"context"
	"testing"
)

func TestMessageToolExecuteAddDefaultAction(t *testing.T) {
	svc := &mockPushService{}
	tool := NewMessageTool(svc)
	ctx := WithUserID(context.Background(), "user-1")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"content": "drink water",
		"when":    "1h",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.addCalled || svc.addMessage != "drink water" {
		t.Fatalf("expected add call, got %#v", svc)
	}
	payload := result.(map[string]any)
	if payload["reminder"] == nil {
		t.Fatalf("expected reminder payload, got %#v", payload)
	}
}

func TestMessageToolExecuteDeleteDefaultAction(t *testing.T) {
	svc := &mockPushService{}
	tool := NewMessageTool(svc)
	_, err := tool.Execute(context.Background(), map[string]interface{}{"id": "push-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.deletedID != "push-1" {
		t.Fatalf("deletedID = %q, want push-1", svc.deletedID)
	}
}

func TestRegisterMessageTool(t *testing.T) {
	registry := NewRegistry()
	RegisterMessageTool(registry, &mockPushService{})
	if registry.Get("message") == nil {
		t.Fatal("expected message tool to be registered")
	}
}
