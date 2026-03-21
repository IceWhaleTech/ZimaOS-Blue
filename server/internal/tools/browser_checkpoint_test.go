package tools

import (
	"context"
	"testing"
	"time"
)

func TestIsBrowserActionHighRisk(t *testing.T) {
	cases := []struct {
		action   string
		actType  string
		recipe   string
		expected bool
	}{
		{action: "act", actType: "click", expected: true},
		{action: "act", actType: "type", expected: true},
		{action: "act", actType: "select", expected: true},
		{action: "act", actType: "hover", expected: false},
		{action: "recipe", recipe: "login", expected: true},
		{action: "recipe", recipe: "fill_form", expected: true},
		{action: "recipe", recipe: "extract", expected: false},
		{action: "navigate", expected: false},
	}
	for _, tc := range cases {
		if got := IsBrowserActionHighRisk(tc.action, tc.actType, tc.recipe); got != tc.expected {
			t.Fatalf("IsBrowserActionHighRisk(%q,%q,%q)=%v want %v", tc.action, tc.actType, tc.recipe, got, tc.expected)
		}
	}
}

func TestBrowserCheckpointManager_DefaultTimeoutIsLonger(t *testing.T) {
	mgr := NewBrowserCheckpointManager(0)
	if got := mgr.DefaultTimeout(); got != 5*time.Minute {
		t.Fatalf("DefaultTimeout() = %v, want %v", got, 5*time.Minute)
	}
}

func TestBrowserCheckpointManager_CreateResolve(t *testing.T) {
	mgr := NewBrowserCheckpointManager(2 * time.Minute)
	rec := mgr.Create(BrowserCheckpointRequest{
		SessionID: "session-1",
		UserID:    "user-1",
		Required:  true,
		RiskLevel: "high",
		Step:      "act",
		Action:    "click",
	})
	if rec.ID == "" {
		t.Fatal("expected checkpoint id")
	}
	if pending := mgr.GetPendingBySession("session-1"); pending == nil || pending.ID != rec.ID {
		t.Fatalf("unexpected pending checkpoint: %+v", pending)
	}
	if !mgr.Resolve(rec.ID, BrowserCheckpointApprove) {
		t.Fatal("expected resolve to succeed")
	}
	if pending := mgr.GetPendingBySession("session-1"); pending != nil {
		t.Fatalf("expected no pending checkpoint, got %+v", pending)
	}
}

func TestBrowserCheckpointManager_WaitTimeout(t *testing.T) {
	mgr := NewBrowserCheckpointManager(25 * time.Millisecond)
	rec := mgr.Create(BrowserCheckpointRequest{
		SessionID: "session-timeout",
		Required:  true,
	})
	decision, err := mgr.Wait(context.Background(), rec.ID)
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if decision != BrowserCheckpointTimeout {
		t.Fatalf("expected timeout decision, got %s", decision)
	}
}

func TestBrowserCheckpointManager_OnePendingPerSession(t *testing.T) {
	mgr := NewBrowserCheckpointManager(2 * time.Minute)
	first := mgr.Create(BrowserCheckpointRequest{SessionID: "same-session", Required: true})
	second := mgr.Create(BrowserCheckpointRequest{SessionID: "same-session", Required: true})
	if first.ID == second.ID {
		t.Fatal("expected new checkpoint id for second request")
	}
	if got := mgr.Get(first.ID); got != nil {
		t.Fatalf("expected first checkpoint to be replaced, got %+v", got)
	}
	if got := mgr.GetPendingBySession("same-session"); got == nil || got.ID != second.ID {
		t.Fatalf("expected second checkpoint pending, got %+v", got)
	}
}

func TestParseBrowserCheckpointDecision(t *testing.T) {
	approveInputs := []string{
		"1", "yes", "continue", "继续", "确认",
		"oui", "ja", "sí", "sim", "да", "はい", "네", "ano", "tak", "ναι", "igen", "da", "fortsett", "fortsätt", "അതെ",
		"go ahead", "嗯", "收到", "  go   ahead!! ",
		"carry on", "endavant", "pokračuj", "fortsæt", "fortfahren", "επιβεβαίωσε", "adelante", "continuez", "ceadaigh", "nastavi",
		"folytasd", "procedi", "進めて", "계속해", "ശരി", "doorgaan", "dalej", "prosseguir", "continuă", "продолжай",
		"potvrdiť", "kör på", "同意", "允許",
	}
	for _, input := range approveInputs {
		decision, ok := ParseBrowserCheckpointDecision(input)
		if !ok || decision != BrowserCheckpointApprove {
			t.Fatalf("expected approve for input %q, got decision=%s ok=%v", input, decision, ok)
		}
	}

	denyInputs := []string{
		"2", "no", "cancel", "取消", "拒绝",
		"non", "nein", "cancelar", "annulla", "não", "нет", "いいえ", "아니요", "nie", "όχι", "nem", "otkaži", "nu", "nei", "nej", "ഇല്ല",
		"nope", "算了", "  cancel   it  ",
		"never mind", "atura", "odmítnout", "annuller", "stoppen", "σταμάτησε", "rechaza", "refuse", "stad", "prekini",
		"állj", "fermati", "やめて", "멈춰", "വേണ്ട", "annuleren", "przerwij", "cancele", "oprește", "отмени",
		"odmietni", "stoppa", "不同意", "不要繼續",
	}
	for _, input := range denyInputs {
		decision, ok := ParseBrowserCheckpointDecision(input)
		if !ok || decision != BrowserCheckpointDeny {
			t.Fatalf("expected deny for input %q, got decision=%s ok=%v", input, decision, ok)
		}
	}

	if decision, ok := ParseBrowserCheckpointDecision("maybe later"); ok || decision != BrowserCheckpointPending {
		t.Fatalf("expected pending/false for unrecognized input, got decision=%s ok=%v", decision, ok)
	}
}

func TestNormalizeBrowserSiteOrigin(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{input: "https://Example.com/foo?bar=baz", want: "https://example.com"},
		{input: "http://example.com:80/path", want: "http://example.com"},
		{input: "https://example.com:443/path", want: "https://example.com"},
		{input: "https://example.com:8443/path", want: "https://example.com:8443"},
		{input: "example.com", want: "https://example.com"},
		{input: "file:///tmp/test.html", want: ""},
		{input: "", want: ""},
	}
	for _, tc := range cases {
		if got := NormalizeBrowserSiteOrigin(tc.input); got != tc.want {
			t.Fatalf("NormalizeBrowserSiteOrigin(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestBrowserSiteAllowlistStore(t *testing.T) {
	db := newTestDB(t)
	store, err := NewBrowserSiteAllowlistStore(db)
	if err != nil {
		t.Fatal(err)
	}

	if got := store.Match("https://example.com/path", "user-1"); got != nil {
		t.Fatalf("expected no match in empty store, got %+v", got)
	}

	if err := store.Add("https://Example.com/account", "user-1"); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if err := store.Add("https://example.com/another-path", "user-1"); err != nil {
		t.Fatalf("duplicate Add returned error: %v", err)
	}

	if got := store.Match("https://example.com/settings", "user-1"); got == nil {
		t.Fatal("expected normalized origin match for same user")
	} else if got.Origin != "https://example.com" {
		t.Fatalf("match origin = %q, want %q", got.Origin, "https://example.com")
	}

	if got := store.Match("https://example.com/settings", "user-2"); got != nil {
		t.Fatalf("expected no cross-user match, got %+v", got)
	}

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("List len = %d, want 1", len(entries))
	}
	if entries[0].Origin != "https://example.com" {
		t.Fatalf("List origin = %q, want %q", entries[0].Origin, "https://example.com")
	}

	if err := store.Delete(entries[0].ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if got := store.Match("https://example.com/settings", "user-1"); got != nil {
		t.Fatalf("expected match to be deleted, got %+v", got)
	}

	if err := store.Add("file:///tmp/test.html", "user-1"); err == nil {
		t.Fatal("expected invalid origin add to fail")
	}
}
