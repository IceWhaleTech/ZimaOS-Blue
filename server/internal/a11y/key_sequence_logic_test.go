package a11y

import (
	"errors"
	"testing"
)

func TestNormalizeKeySequence(t *testing.T) {
	cleaned, err := normalizeKeySequence([]string{"  ctrl ", "", "  v  "})
	if err != nil {
		t.Fatalf("normalizeKeySequence() error = %v", err)
	}
	if len(cleaned) != 2 {
		t.Fatalf("cleaned len = %d, want 2", len(cleaned))
	}
	if cleaned[0] != "ctrl" {
		t.Fatalf("cleaned[0] = %q, want ctrl", cleaned[0])
	}
	if cleaned[1] != "v" {
		t.Fatalf("cleaned[1] = %q, want v", cleaned[1])
	}
}

func TestNormalizeKeySequence_RejectsEmptyInput(t *testing.T) {
	_, err := normalizeKeySequence([]string{" ", "\n"})
	if err == nil {
		t.Fatal("normalizeKeySequence() error = nil, want error")
	}
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("normalizeKeySequence() error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "unsupported_action" {
		t.Fatalf("error code = %q, want unsupported_action", runtimeErr.Code)
	}
}

func TestHandleLiteralTextKeySequence_UsesTextSenderForSinglePlainText(t *testing.T) {
	sendCalls := 0
	cleaned, handled, err := handleLiteralTextKeySequence(
		[]string{"  hello world  "},
		func(string) bool { return false },
		func(string) bool { return false },
		func(value string) error {
			sendCalls++
			if value != "hello world" {
				t.Fatalf("send value = %q, want hello world", value)
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("handleLiteralTextKeySequence() error = %v", err)
	}
	if !handled {
		t.Fatal("handled = false, want true")
	}
	if len(cleaned) != 1 || cleaned[0] != "hello world" {
		t.Fatalf("cleaned = %#v, want [\"hello world\"]", cleaned)
	}
	if sendCalls != 1 {
		t.Fatalf("sendCalls = %d, want 1", sendCalls)
	}
}

func TestHandleLiteralTextKeySequence_DoesNotTreatShortcutAsText(t *testing.T) {
	sendCalls := 0
	cleaned, handled, err := handleLiteralTextKeySequence(
		[]string{" ctrl ", " v "},
		func(value string) bool { return value == "ctrl" },
		func(value string) bool { return value == "v" },
		func(string) error {
			sendCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("handleLiteralTextKeySequence() error = %v", err)
	}
	if handled {
		t.Fatal("handled = true, want false")
	}
	if len(cleaned) != 2 {
		t.Fatalf("cleaned len = %d, want 2", len(cleaned))
	}
	if sendCalls != 0 {
		t.Fatalf("sendCalls = %d, want 0", sendCalls)
	}
}

func TestHandleLiteralTextKeySequence_DoesNotTreatNamedSingleKeyAsText(t *testing.T) {
	sendCalls := 0
	cleaned, handled, err := handleLiteralTextKeySequence(
		[]string{"enter"},
		func(string) bool { return false },
		func(value string) bool { return value == "enter" },
		func(string) error {
			sendCalls++
			return nil
		},
	)
	if err != nil {
		t.Fatalf("handleLiteralTextKeySequence() error = %v", err)
	}
	if handled {
		t.Fatal("handled = true, want false")
	}
	if len(cleaned) != 1 || cleaned[0] != "enter" {
		t.Fatalf("cleaned = %#v, want [\"enter\"]", cleaned)
	}
	if sendCalls != 0 {
		t.Fatalf("sendCalls = %d, want 0", sendCalls)
	}
}
