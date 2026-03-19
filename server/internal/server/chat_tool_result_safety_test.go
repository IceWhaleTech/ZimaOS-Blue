package server

import (
	"strings"
	"testing"
)

type recursiveChatPayload struct {
	Name string                `json:"name"`
	Self *recursiveChatPayload `json:"self,omitempty"`
}

func TestFormatValueGuardsRecursivePayloads(t *testing.T) {
	payload := &recursiveChatPayload{Name: "root"}
	payload.Self = payload

	got := formatValue(payload)
	if !strings.Contains(got, `"name":"root"`) {
		t.Fatalf("formatValue() = %q, want name field", got)
	}
	if !strings.Contains(got, `[circular payload omitted]`) {
		t.Fatalf("formatValue() = %q, want circular marker", got)
	}
}

func TestAnyToStringForLLMGuardsRecursivePayloads(t *testing.T) {
	payload := &recursiveChatPayload{Name: "root"}
	payload.Self = payload

	got := anyToStringForLLM(payload)
	if !strings.Contains(got, `"name":"root"`) {
		t.Fatalf("anyToStringForLLM() = %q, want name field", got)
	}
	if !strings.Contains(got, `[circular payload omitted]`) {
		t.Fatalf("anyToStringForLLM() = %q, want circular marker", got)
	}
}

func TestCardActionFormValueGuardsRecursivePayloads(t *testing.T) {
	payload := &recursiveChatPayload{Name: "root"}
	payload.Self = payload

	got := cardActionFormValue(map[string]interface{}{"payload": payload}, "payload")
	if !strings.Contains(got, `"name":"root"`) {
		t.Fatalf("cardActionFormValue() = %q, want name field", got)
	}
	if !strings.Contains(got, `[circular payload omitted]`) {
		t.Fatalf("cardActionFormValue() = %q, want circular marker", got)
	}
}
