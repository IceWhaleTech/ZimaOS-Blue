package harness

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeObserverPreservesLongStructuredSearchEvidence(t *testing.T) {
	controller, run := newRuntimeReflectionControllerAndRun(t)
	observer := NewRuntimeObserver(controller)
	record := func(id string, result interface{}) {
		event := tools.ToolRuntimeEvent{RunID: run.ID, ToolName: "bash", ToolCallID: id,
			Arguments: map[string]interface{}{"command": `blue web_query query="OpenAI Responses API docs"`}, Result: result}
		observer.OnToolRequested(event)
		observer.OnToolFinished(event)
	}
	record("empty", `{"stdout":"sources: []\nnext_action: refine_query\nstatus: partial\nfinal_url: \n", "data":{"status":"partial","sources":[],"next_action":"refine_query","content":""}}`)
	events, err := controller.ListEvents(context.Background(), run.ID, 200)
	if err != nil {
		t.Fatal(err)
	}
	if !observationSeen(deriveVerificationObservations(&Run{Status: RunStatusCompleted}, events, nil, []string{"bash"}), "evidence_collection_empty") {
		t.Fatal("empty search must remain rejected")
	}
	source := "https://platform.openai.com/docs/api-reference/responses"
	content := strings.Repeat("Responses API documentation. ", 600)
	raw, err := json.Marshal(map[string]interface{}{
		"stdout": "page: " + content + "\nfinal_url: " + source + "\nstatus: ok",
		"data":   map[string]interface{}{"content": content, "final_url": source, "status": "ok", "sources": []interface{}{map[string]interface{}{"url": source, "selected": true}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	record("recovered", string(raw))
	events, err = controller.ListEvents(context.Background(), run.ID, 200)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.Type != "tool_finished" || eventToolCallID(event) != "recovered" {
			continue
		}
		found = true
		result := decodeVerifierEventPayload(event.PayloadJSON)["result"]
		if encoded, ok := result.(string); ok {
			if err := json.Unmarshal([]byte(encoded), &result); err != nil {
				t.Fatalf("observer corrupted structured result: %v", err)
			}
		}
		data := result.(map[string]interface{})["data"].(map[string]interface{})
		if data["final_url"] != source || data["status"] != "ok" {
			t.Fatal("observer discarded source metadata")
		}
		if len(data["content"].(string)) > maxRuntimeObserverPayloadStringBytes+len("\n[truncated]") {
			t.Fatal("individual content limit lost")
		}
	}
	if !found {
		t.Fatal("missing recovered tool event")
	}
	observations := deriveVerificationObservations(&Run{Status: RunStatusCompleted}, events, nil, []string{"bash"})
	if observationSeen(observations, "evidence_collection_empty") {
		t.Fatal("successful recovery misclassified as empty")
	}
}

func TestRuntimeObserverStillSanitizesNonJSONResults(t *testing.T) {
	cycle := map[string]interface{}{}
	cycle["self"] = cycle
	for name, result := range map[string]interface{}{
		"plain":        strings.Repeat("text", 3000),
		"invalid_json": "{broken: " + strings.Repeat("text", 3000),
		"binary":       []byte{0, 255, 1},
		"cycle":        cycle,
	} {
		t.Run(name, func(t *testing.T) {
			controller, run := newRuntimeReflectionControllerAndRun(t)
			NewRuntimeObserver(controller).OnToolFinished(tools.ToolRuntimeEvent{RunID: run.ID, ToolName: "bash", Result: result})
			events, err := controller.ListEvents(context.Background(), run.ID, 200)
			if err != nil {
				t.Fatal(err)
			}
			for _, event := range events {
				if event.Type != "tool_finished" {
					continue
				}
				if !json.Valid([]byte(event.PayloadJSON)) {
					t.Fatal("unsafe observer JSON")
				}
				if len(event.PayloadJSON) > 2*maxRuntimeObserverPayloadStringBytes {
					t.Fatal("unbounded fallback result")
				}
				return
			}
			t.Fatal("missing tool result")
		})
	}
}
