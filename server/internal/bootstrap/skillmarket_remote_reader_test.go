package bootstrap

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
)

func TestBuildSkillMarketReadArgs_RawHTMLDisablesFallbacksAndPinsHTTPLane(t *testing.T) {
	args := buildSkillMarketReadArgs(skillmarket.RemoteReadRequest{
		URL:         "https://example.com/catalog",
		Format:      "text",
		MaxChars:    4096,
		WantRawHTML: true,
		Headers: map[string]string{
			"X-Test": "value",
		},
	})

	if got := args["url"]; got != "https://example.com/catalog" {
		t.Fatalf("url = %#v, want %q", got, "https://example.com/catalog")
	}
	if got := args["format"]; got != "text" {
		t.Fatalf("format = %#v, want %q", got, "text")
	}
	if got := args["max_chars"]; got != 4096 {
		t.Fatalf("max_chars = %#v, want %d", got, 4096)
	}
	headers, ok := args["headers"].(map[string]string)
	if !ok {
		t.Fatalf("headers type = %T, want map[string]string", args["headers"])
	}
	if got := headers["X-Test"]; got != "value" {
		t.Fatalf("headers[X-Test] = %q, want %q", got, "value")
	}
	if got := args["lane"]; got != "http" {
		t.Fatalf("lane = %#v, want %q", got, "http")
	}
	if got := args["disable_internal_fallbacks"]; got != true {
		t.Fatalf("disable_internal_fallbacks = %#v, want true", got)
	}
}

func TestBuildSkillMarketReadArgs_DefaultReadsPreserveAutoLane(t *testing.T) {
	args := buildSkillMarketReadArgs(skillmarket.RemoteReadRequest{
		URL:      "https://example.com/catalog",
		MaxChars: 2048,
	})

	if got := args["url"]; got != "https://example.com/catalog" {
		t.Fatalf("url = %#v, want %q", got, "https://example.com/catalog")
	}
	if got := args["format"]; got != "text" {
		t.Fatalf("format = %#v, want %q", got, "text")
	}
	if got := args["max_chars"]; got != 2048 {
		t.Fatalf("max_chars = %#v, want %d", got, 2048)
	}
	if _, ok := args["lane"]; ok {
		t.Fatalf("lane unexpectedly set: %#v", args["lane"])
	}
	if _, ok := args["disable_internal_fallbacks"]; ok {
		t.Fatalf("disable_internal_fallbacks unexpectedly set: %#v", args["disable_internal_fallbacks"])
	}
}
