package tools

import "testing"

func TestRegisterBrowserTool(t *testing.T) {
	registry := NewRegistry()
	backend := &mockBrowserBackend{}
	RegisterBrowserTool(registry, backend)
	tool := registry.Get("browser")
	if tool == nil {
		t.Fatal("expected browser tool to be registered")
	}
	browserTool, ok := tool.(*BrowserTool)
	if !ok {
		t.Fatalf("tool type = %T, want *BrowserTool", tool)
	}
	if browserTool.Backend() != backend {
		t.Fatal("expected browser backend to be wired")
	}
}
