package agent

import (
	"strings"
	"testing"
)

type recursiveStatePayload struct {
	Name string                 `json:"name"`
	Self *recursiveStatePayload `json:"self,omitempty"`
}

func TestAsStringGuardsRecursivePayloads(t *testing.T) {
	payload := &recursiveStatePayload{Name: "root"}
	payload.Self = payload

	got := asString(payload)
	if !strings.Contains(got, `"name":"root"`) {
		t.Fatalf("asString() = %q, want name field", got)
	}
	if !strings.Contains(got, `[circular payload omitted]`) {
		t.Fatalf("asString() = %q, want circular marker", got)
	}
}
