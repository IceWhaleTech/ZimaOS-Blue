package cards

import (
	"strings"
	"sync"
	"testing"
)

func TestCardRegexes_InitializeOnDemand(t *testing.T) {
	originalImageSuffix := genericCardImageURLSuffix
	originalWindowsPath := genericCardWindowsAbsPath
	originalRedactionRules := sensitiveTextRedactionRules

	genericCardImageURLSuffix = nil
	genericCardWindowsAbsPath = nil
	sensitiveTextRedactionRules = nil
	genericCardRegexesOnce = sync.Once{}
	sensitiveTextRedactionRulesOnce = sync.Once{}
	t.Cleanup(func() {
		genericCardImageURLSuffix = originalImageSuffix
		genericCardWindowsAbsPath = originalWindowsPath
		sensitiveTextRedactionRules = originalRedactionRules
		genericCardRegexesOnce = sync.Once{}
		sensitiveTextRedactionRulesOnce = sync.Once{}
	})

	if genericCardImageURLSuffix != nil || genericCardWindowsAbsPath != nil {
		t.Fatal("expected generic card regexes to start nil")
	}
	if sensitiveTextRedactionRules != nil {
		t.Fatal("expected redaction rules to start nil")
	}

	redacted := RedactSensitiveText("Authorization: Bearer secret-token-1234567890")
	if !strings.Contains(redacted, "[REDACTED]") || !strings.Contains(redacted, "[TOKEN_REDACTED]") {
		t.Fatalf("RedactSensitiveText() = %q, want sensitive token markers", redacted)
	}
	if strings.Contains(redacted, "secret-token-1234567890") {
		t.Fatalf("RedactSensitiveText() leaked the raw token: %q", redacted)
	}
	if len(sensitiveTextRedactionRules) == 0 {
		t.Fatal("expected redaction rules to initialize on demand")
	}

	card := ToCard("browser", `{"message":"Screenshot captured","screenshot":"C:\\Users\\orca\\AppData\\Local\\ZimaOS\\browser\\shot.png"}`)
	if card == nil {
		t.Fatal("expected generic card output")
	}
	if genericCardImageURLSuffix == nil || genericCardWindowsAbsPath == nil {
		t.Fatal("expected generic card path regexes to initialize on demand")
	}
}
