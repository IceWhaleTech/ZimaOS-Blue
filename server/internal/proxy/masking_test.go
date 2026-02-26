package proxy

import (
	"strings"
	"testing"
)

func TestDataMasker_Disabled(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{Enabled: false, Rules: GetDefaultRules()})
	input := "My API key is sk-abcdefghijklmnopqrstuvwxyz1234"
	if got := dm.MaskResponse(input); got != input {
		t.Errorf("disabled masker should not modify content, got %q", got)
	}
}

func TestDataMasker_ResponseOnly(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		r.Enabled = true
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})

	input := "My email is test@example.com"

	// Request side should NOT mask (all default rules are response-only)
	if got := dm.MaskRequest(input); got != input {
		t.Errorf("request masking should not apply to response-only rules, got %q", got)
	}

	// Response side SHOULD mask
	got := dm.MaskResponse(input)
	if got == input {
		t.Error("response masking should apply to response-only rules")
	}
	if !strings.Contains(got, "[EMAIL]") {
		t.Errorf("expected [EMAIL] tag in masked output, got %q", got)
	}
}

func TestDataMasker_MaskLabel_DefaultEnglish(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		r.Enabled = true
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})

	got := dm.MaskResponse("Contact: test@example.com")
	if !strings.Contains(got, "🛡️Data Masked") {
		t.Errorf("expected default English label, got %q", got)
	}
}

func TestDataMasker_MaskLabel_Chinese(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		r.Enabled = true
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})
	dm.SetLocaleFunc(func() string { return "zh-CN" })

	got := dm.MaskResponse("Contact: test@example.com")
	if !strings.Contains(got, "🛡️数据脱敏") {
		t.Errorf("expected Chinese label, got %q", got)
	}
}

func TestDataMasker_MaskLabel_Japanese(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		r.Enabled = true
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})
	dm.SetLocaleFunc(func() string { return "ja-JP" })

	got := dm.MaskResponse("Contact: test@example.com")
	if !strings.Contains(got, "🛡️データマスク") {
		t.Errorf("expected Japanese label, got %q", got)
	}
}

func TestDataMasker_MaskLabel_LanguageFallback(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		r.Enabled = true
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})
	// "de-DE" should fall back to "de" prefix
	dm.SetLocaleFunc(func() string { return "de-DE" })

	got := dm.MaskResponse("Contact: test@example.com")
	if !strings.Contains(got, "🛡️Daten maskiert") {
		t.Errorf("expected German label via fallback, got %q", got)
	}
}

func TestDataMasker_MaskLabel_UnknownLocale(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		r.Enabled = true
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})
	dm.SetLocaleFunc(func() string { return "xx-YY" })

	got := dm.MaskResponse("Contact: test@example.com")
	if !strings.Contains(got, "🛡️Data Masked") {
		t.Errorf("expected English fallback for unknown locale, got %q", got)
	}
}

func TestDataMasker_PlaceholderResolution(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{
		Enabled: true,
		Rules: []*MaskingRule{{
			ID:          "test_placeholder",
			Name:        "Test",
			Category:    MaskingPII,
			Pattern:     `secret123`,
			Replacement: "【{MASKED}】[SECRET]",
			Direction:   MaskingResponse,
			Enabled:     true,
		}},
	})
	dm.SetLocaleFunc(func() string { return "ko-KR" })

	got := dm.MaskResponse("The code is secret123")
	if !strings.Contains(got, "🛡️데이터 마스킹") {
		t.Errorf("expected Korean label in placeholder, got %q", got)
	}
	if !strings.Contains(got, "[SECRET]") {
		t.Errorf("expected [SECRET] tag preserved, got %q", got)
	}
	if strings.Contains(got, "secret123") {
		t.Error("original secret should be masked")
	}
}

func TestDataMasker_EmailPattern(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		if r.ID == "email" {
			r.Enabled = true
		}
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})

	tests := []struct {
		name  string
		input string
		want  bool // should be masked
	}{
		{"simple email", "Contact me at user@example.com please", true},
		{"no email", "Hello world", false},
		{"email with subdomain", "Send to admin@mail.example.co.uk", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dm.MaskResponse(tt.input)
			masked := got != tt.input
			if masked != tt.want {
				t.Errorf("masked=%v, want=%v, got=%q", masked, tt.want, got)
			}
		})
	}
}

func TestDataMasker_APIKeyPattern(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		if r.ID == "api_key_format" {
			r.Enabled = true
		}
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})

	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"sk- prefix key", "My key: sk-abcdefghijklmnopqrstuvwxyz", true},
		{"key- prefix key", "Token: key-1234567890abcdefghij", true},
		{"short key (no match)", "sk-short", false},
		{"no key", "Hello world", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dm.MaskResponse(tt.input)
			masked := got != tt.input
			if masked != tt.want {
				t.Errorf("masked=%v, want=%v, got=%q", masked, tt.want, got)
			}
		})
	}
}

func TestDataMasker_CreditCardPattern(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		if r.ID == "credit_card" {
			r.Enabled = true
		}
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})

	got := dm.MaskResponse("Card: 4111-1111-1111-1111")
	if !strings.Contains(got, "[CARD]") {
		t.Errorf("expected credit card to be masked, got %q", got)
	}
}

func TestDataMasker_MaskBytes_ResponseOnly(t *testing.T) {
	rules := GetDefaultRules()
	for _, r := range rules {
		if r.ID == "email" {
			r.Enabled = true
		}
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})

	input := []byte("Email: test@example.com")

	// Request bytes should not be masked
	gotReq := dm.MaskRequestBytes(input)
	if string(gotReq) != string(input) {
		t.Error("request bytes should not be masked for response-only rules")
	}

	// Response bytes should be masked
	gotResp := dm.MaskResponseBytes(input)
	if string(gotResp) == string(input) {
		t.Error("response bytes should be masked")
	}
}

func TestDataMasker_AddRemoveRule(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{Enabled: true})

	err := dm.AddRule(&MaskingRule{
		ID:          "test_rule",
		Name:        "Test",
		Pattern:     `SENSITIVE`,
		Replacement: "[REDACTED]",
		Direction:   MaskingBoth,
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("AddRule error: %v", err)
	}

	got := dm.Mask("Data: SENSITIVE info", MaskingResponse)
	if !strings.Contains(got, "[REDACTED]") {
		t.Errorf("expected rule to mask, got %q", got)
	}

	dm.RemoveRule("test_rule")
	got = dm.Mask("Data: SENSITIVE info", MaskingResponse)
	if strings.Contains(got, "[REDACTED]") {
		t.Error("removed rule should not mask")
	}
}

func TestDataMasker_SetRuleEnabled(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{Enabled: true})
	_ = dm.AddRule(&MaskingRule{
		ID: "toggle", Name: "Toggle", Pattern: `TOGGLE`,
		Replacement: "[OFF]", Direction: MaskingBoth, Enabled: true,
	})

	// Disable
	dm.SetRuleEnabled("toggle", false)
	got := dm.Mask("TOGGLE", MaskingResponse)
	if got != "TOGGLE" {
		t.Error("disabled rule should not mask")
	}

	// Re-enable
	dm.SetRuleEnabled("toggle", true)
	got = dm.Mask("TOGGLE", MaskingResponse)
	if got == "TOGGLE" {
		t.Error("re-enabled rule should mask")
	}
}

func TestDataMasker_Stats(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{Enabled: true})
	_ = dm.AddRule(&MaskingRule{
		ID: "stat_test", Name: "Stat", Pattern: `SECRET`,
		Replacement: "[X]", Direction: MaskingBoth, Enabled: true,
	})

	dm.Mask("SECRET data", MaskingResponse)
	dm.Mask("SECRET again", MaskingResponse)

	stats := dm.Stats()
	if stats["total_masks"] != int64(2) {
		t.Errorf("expected total_masks=2, got %v", stats["total_masks"])
	}

	dm.ResetStats()
	stats = dm.Stats()
	if stats["total_masks"] != int64(0) {
		t.Errorf("expected total_masks=0 after reset, got %v", stats["total_masks"])
	}
}

func TestDataMasker_GetRule(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{Enabled: true})
	_ = dm.AddRule(&MaskingRule{
		ID: "find_me", Name: "FindMe", Pattern: `x`,
		Replacement: "y", Direction: MaskingBoth, Enabled: true,
	})

	rule, ok := dm.GetRule("find_me")
	if !ok {
		t.Fatal("expected to find rule")
	}
	if rule.Name != "FindMe" {
		t.Errorf("expected name 'FindMe', got %q", rule.Name)
	}

	_, ok = dm.GetRule("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent rule")
	}
}

func TestDataMasker_ListRules(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: GetDefaultRules()})
	rules := dm.ListRules()
	if len(rules) != len(GetDefaultRules()) {
		t.Errorf("expected %d rules, got %d", len(GetDefaultRules()), len(rules))
	}
}

func TestDataMasker_SetEnabled(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{Enabled: false})
	if dm.IsEnabled() {
		t.Error("expected disabled")
	}
	dm.SetEnabled(true)
	if !dm.IsEnabled() {
		t.Error("expected enabled after SetEnabled(true)")
	}
}

func TestDataMasker_InvalidPattern(t *testing.T) {
	dm := NewDataMasker(&MaskingConfig{Enabled: true})
	err := dm.AddRule(&MaskingRule{
		ID: "bad", Name: "Bad", Pattern: `[invalid`,
		Replacement: "x", Direction: MaskingBoth, Enabled: true,
	})
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

func TestDataMasker_OnMaskCallback(t *testing.T) {
	var called bool
	dm := NewDataMasker(&MaskingConfig{
		Enabled: true,
		OnMask: func(ruleID, original, masked string) {
			called = true
		},
	})
	_ = dm.AddRule(&MaskingRule{
		ID: "cb", Name: "CB", Pattern: `CALLBACK`,
		Replacement: "[X]", Direction: MaskingBoth, Enabled: true,
	})

	dm.Mask("CALLBACK test", MaskingResponse)
	if !called {
		t.Error("OnMask callback should have been called")
	}
}

func TestMaskLabelMap_AllLocales(t *testing.T) {
	// Verify all supported language prefixes have entries
	expectedLangs := []string{
		"zh", "ja", "ko", "de", "fr", "es", "pt", "it", "nl",
		"ru", "pl", "cs", "sk", "da", "sv", "nb", "hu", "ro",
		"hr", "el", "ca", "ga", "ml",
	}
	for _, lang := range expectedLangs {
		if _, ok := maskLabelMap[lang]; !ok {
			t.Errorf("missing maskLabelMap entry for language %q", lang)
		}
	}
}

func BenchmarkDataMasker_MaskResponse(b *testing.B) {
	rules := GetDefaultRules()
	for _, r := range rules {
		r.Enabled = true
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})
	input := "Contact test@example.com or call 555-123-4567. Key: sk-abcdefghijklmnopqrstuvwxyz"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dm.MaskResponse(input)
	}
}

func BenchmarkDataMasker_NoMatch(b *testing.B) {
	rules := GetDefaultRules()
	for _, r := range rules {
		r.Enabled = true
	}
	dm := NewDataMasker(&MaskingConfig{Enabled: true, Rules: rules})
	input := "This is a normal message with no sensitive data at all."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dm.MaskResponse(input)
	}
}
