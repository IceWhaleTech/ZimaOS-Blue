package server

import "testing"

func TestParseAcceptLanguage(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{header: "", want: "en"},
		{header: "zh-CN,zh;q=0.9,en;q=0.8", want: "zh"},
		{header: "ja-JP", want: "ja"},
		{header: "EN-US,en;q=0.9", want: "en"},
	}

	for _, tt := range tests {
		if got := parseAcceptLanguage(tt.header); got != tt.want {
			t.Errorf("parseAcceptLanguage(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestGetLanguageInstruction(t *testing.T) {
	if got := getLanguageInstruction("zh"); got != "The title MUST be in Chinese (中文)." {
		t.Errorf("getLanguageInstruction(zh) = %q", got)
	}
	if got := getLanguageInstruction("xx"); got != "The title should be in English." {
		t.Errorf("getLanguageInstruction(xx) = %q", got)
	}
}
