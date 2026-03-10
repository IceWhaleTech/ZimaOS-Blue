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
