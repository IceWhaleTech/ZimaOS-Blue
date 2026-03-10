package proxy

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestLogTokenChurn_IncludesPromptCacheKeyAndEvent(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(prev)
	})

	LogTokenChurn("openai", "gpt-5", "pcache-key-1", 120, 90, 15)

	out := buf.String()
	if !strings.Contains(out, `"prompt_cache_key":"pcache-key-1"`) {
		t.Fatalf("expected prompt_cache_key in log output, got %q", out)
	}
	if !strings.Contains(out, `"cache_event":"hit+create"`) {
		t.Fatalf("expected cache_event in log output, got %q", out)
	}
}
