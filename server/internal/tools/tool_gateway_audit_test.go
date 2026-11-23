package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestToolGatewayPreservesSerializedJSONAuditMetadata(t *testing.T) {
	content := strings.Repeat("source documentation ", 8000)
	source := "https://platform.openai.com/docs/api-reference/responses"
	raw, err := json.Marshal(map[string]interface{}{
		"stdout": "page: " + content,
		"data":   map[string]interface{}{"content": content, "final_url": source, "status": "ok", "sources": []interface{}{map[string]interface{}{"url": source}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	registry.Register(&gatewayResultTool{def: ToolDefinition{Name: "bash", Parameters: map[string]interface{}{"type": "object"}}, result: string(raw)})
	gateway := NewToolGateway(registry, NewExecutor(registry))
	observer := &runtimeObserverStub{}
	gateway.SetEventObserver(observer)
	result, err := gateway.Execute(context.Background(), ToolGatewayRequest{ToolCallID: "long-audit", ToolName: "bash", Arguments: `{}`, RouteKind: ToolRouteKindAgent})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExecutionResult != string(raw) {
		t.Fatal("execution result contract changed")
	}
	if len(observer.toolFinished) != 1 {
		t.Fatal("missing tool-finished event")
	}
	for label, value := range map[string]interface{}{"audit": result.AuditPayload, "observer": observer.toolFinished[0].Result} {
		if encoded, ok := value.(string); ok {
			if err := json.Unmarshal([]byte(encoded), &value); err != nil {
				t.Fatalf("%s JSON corrupted before observer: %v", label, err)
			}
		}
		payload, ok := value.(map[string]interface{})
		if !ok {
			t.Fatalf("%s lost structured envelope", label)
		}
		data := payload["data"].(map[string]interface{})
		if data["final_url"] != source || data["status"] != "ok" {
			t.Fatalf("%s lost source metadata", label)
		}
		if len(data["content"].(string)) > maxToolGatewayAuditStringBytes+len("\n[truncated]") {
			t.Fatalf("%s audit string limit lost", label)
		}
	}
}
