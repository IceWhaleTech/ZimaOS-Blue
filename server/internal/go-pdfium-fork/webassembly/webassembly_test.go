package webassembly

import (
	"strings"
	"testing"
)

func TestInitRequiresWASM(t *testing.T) {
	_, err := Init(Config{})
	if err == nil {
		t.Fatal("expected Init to require external WASM bytes")
	}
	if !strings.Contains(err.Error(), "Config.WASM is required") {
		t.Fatalf("error = %v, want Config.WASM is required", err)
	}
}
