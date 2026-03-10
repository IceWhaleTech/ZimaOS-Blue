package proxy

import (
	"testing"
	"time"
)

func TestProviderMemory_Format(t *testing.T) {
	pm := NewProviderMemory()

	// Miss
	if _, ok := pm.RecallFormat("p1", "https://api.example.com"); ok {
		t.Fatal("expected miss on empty cache")
	}

	// Remember + recall
	pm.RememberFormat("p1", "https://api.example.com", string(ProviderTypeAnthropic))
	pt, ok := pm.RecallFormat("p1", "https://api.example.com")
	if !ok || pt != string(ProviderTypeAnthropic) {
		t.Fatalf("expected anthropic, got %v (ok=%v)", pt, ok)
	}

	// Different provider = miss
	if _, ok := pm.RecallFormat("p2", "https://api.example.com"); ok {
		t.Fatal("expected miss for different provider")
	}

	// Forget
	pm.ForgetFormat("p1", "https://api.example.com")
	if _, ok := pm.RecallFormat("p1", "https://api.example.com"); ok {
		t.Fatal("expected miss after forget")
	}
}

func TestProviderMemory_ModelFormat(t *testing.T) {
	pm := NewProviderMemory()
	pid := "p1"
	baseURL := "https://relay.example.com/v1/responses"
	model := "claude-sonnet-4-6"

	if _, ok := pm.RecallModelFormat(pid, baseURL, model); ok {
		t.Fatal("expected miss on empty model-format cache")
	}

	pm.RememberModelFormat(pid, baseURL, model, "anthropic")
	format, ok := pm.RecallModelFormat(pid, baseURL, model)
	if !ok || format != "anthropic" {
		t.Fatalf("expected anthropic, got %q (ok=%v)", format, ok)
	}

	if _, ok := pm.RecallModelFormat(pid, "https://relay.example.com", model); ok {
		t.Fatal("expected different base URL to use a different model-format key")
	}
	if _, ok := pm.RecallModelFormat(pid, baseURL, "claude-haiku-4-5"); ok {
		t.Fatal("expected different model to use a different model-format key")
	}

	pm.ForgetModelFormat(pid, baseURL, model)
	if _, ok := pm.RecallModelFormat(pid, baseURL, model); ok {
		t.Fatal("expected miss after forgetting model-format memory")
	}
}

func TestProviderMemory_ModelAlias(t *testing.T) {
	pm := NewProviderMemory()

	// Miss
	if _, ok := pm.RecallModelAlias("p1", "https://relay.example.com", "claude-3-5-haiku-20241022"); ok {
		t.Fatal("expected miss")
	}

	// Remember + recall
	pm.RememberModelAlias("p1", "https://relay.example.com", "claude-3-5-haiku-20241022", "claude-haiku-4-5")
	actual, ok := pm.RecallModelAlias("p1", "https://relay.example.com", "claude-3-5-haiku-20241022")
	if !ok || actual != "claude-haiku-4-5" {
		t.Fatalf("expected claude-haiku-4-5, got %q (ok=%v)", actual, ok)
	}

	// Different model = miss
	if _, ok := pm.RecallModelAlias("p1", "https://relay.example.com", "claude-3-opus-20240229"); ok {
		t.Fatal("expected miss for different model")
	}

	// Forget
	pm.ForgetModelAlias("p1", "https://relay.example.com", "claude-3-5-haiku-20241022")
	if _, ok := pm.RecallModelAlias("p1", "https://relay.example.com", "claude-3-5-haiku-20241022"); ok {
		t.Fatal("expected miss after forget")
	}
}

func TestProviderMemory_ToolCap(t *testing.T) {
	pm := NewProviderMemory()

	// Miss returns unknown
	level, ok := pm.RecallToolCap("p1", "https://relay.example.com")
	if ok || level != ToolCapUnknown {
		t.Fatalf("expected unknown, got %d (ok=%v)", level, ok)
	}

	// Remember + recall
	pm.RememberToolCap("p1", "https://relay.example.com", ToolCapPrompt)
	level, ok = pm.RecallToolCap("p1", "https://relay.example.com")
	if !ok || level != ToolCapPrompt {
		t.Fatalf("expected ToolCapPrompt, got %d (ok=%v)", level, ok)
	}

	// Overwrite
	pm.RememberToolCap("p1", "https://relay.example.com", ToolCapNone)
	level, ok = pm.RecallToolCap("p1", "https://relay.example.com")
	if !ok || level != ToolCapNone {
		t.Fatalf("expected ToolCapNone, got %d (ok=%v)", level, ok)
	}
}

func TestProviderMemory_Throttle(t *testing.T) {
	pm := NewProviderMemory()

	// Not throttled initially
	if pm.IsThrottled("p1", "https://api.example.com") {
		t.Fatal("expected not throttled")
	}

	// Throttle with short duration
	pm.RememberThrottle("p1", "https://api.example.com", 100*time.Millisecond)
	if !pm.IsThrottled("p1", "https://api.example.com") {
		t.Fatal("expected throttled")
	}

	// Wait for expiry
	time.Sleep(150 * time.Millisecond)
	if pm.IsThrottled("p1", "https://api.example.com") {
		t.Fatal("expected not throttled after expiry")
	}

	// Default duration on zero
	pm.RememberThrottle("p1", "https://api.example.com", 0)
	if !pm.IsThrottled("p1", "https://api.example.com") {
		t.Fatal("expected throttled with default duration")
	}

	// Forget clears throttle
	pm.ForgetThrottle("p1", "https://api.example.com")
	if pm.IsThrottled("p1", "https://api.example.com") {
		t.Fatal("expected not throttled after forget")
	}
}

func TestProviderMemory_MemKeyScoping(t *testing.T) {
	pm := NewProviderMemory()

	// Same provider, different hosts = different keys
	pm.RememberFormat("p1", "https://api.openai.com", string(ProviderTypeOpenAI))
	pm.RememberFormat("p1", "https://relay.example.com", string(ProviderTypeAnthropic))

	pt1, _ := pm.RecallFormat("p1", "https://api.openai.com")
	pt2, _ := pm.RecallFormat("p1", "https://relay.example.com")
	if pt1 == pt2 {
		t.Fatal("expected different formats for different hosts")
	}
}

func TestModelAliases(t *testing.T) {
	aliases, ok := ModelAliases["claude-3-5-haiku-20241022"]
	if !ok || len(aliases) == 0 {
		t.Fatal("expected aliases for claude-3-5-haiku-20241022")
	}
	found := false
	for _, a := range aliases {
		if a == "claude-haiku-4-5" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected claude-haiku-4-5 in aliases")
	}
}
