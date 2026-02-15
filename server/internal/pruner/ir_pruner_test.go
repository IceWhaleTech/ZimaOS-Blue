package pruner

import (
	"context"
	"strings"
	"testing"
)

func TestIRPruner_PruneCode(t *testing.T) {
	cfg := Config{Threshold: 0.5, CacheCapacity: 64}
	pruner := NewIRPruner(cfg)

	code := strings.Repeat("package main\nimport \"fmt\"\nfunc handler(w http.ResponseWriter, r *http.Request) {\n"+
		"\tfmt.Println(\"handling request\")\n}\n\n", 10) +
		strings.Repeat("func unused() {\n\tx := 1\n\ty := 2\n\tz := x + y\n\t_ = z\n}\n\n", 10)

	resp, err := pruner.Prune(context.Background(), PruneRequest{
		Content:   code,
		Query:     "handler request",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PrunedTokens >= resp.OriginalTokens {
		t.Errorf("expected token reduction, got %d >= %d", resp.PrunedTokens, resp.OriginalTokens)
	}
	if resp.ContentType != ContentCode {
		t.Errorf("expected ContentCode, got %v", resp.ContentType)
	}
	if resp.LatencyMs <= 0 {
		t.Error("expected positive latency")
	}
}

func TestIRPruner_PruneDoc(t *testing.T) {
	cfg := Config{Threshold: 0.5, CacheCapacity: 64, MinLines: 5}
	pruner := NewIRPruner(cfg)

	doc := "## Authentication\n\nThe authentication module handles user login and session management.\n\n" +
		"## Database\n\nThe database layer provides connection pooling and query optimization.\n\n" +
		strings.Repeat("## Unrelated Section\n\nThis section discusses something completely unrelated to authentication.\n\n", 10)

	resp, err := pruner.Prune(context.Background(), PruneRequest{
		Content:   doc,
		Query:     "authentication login",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PrunedTokens >= resp.OriginalTokens {
		t.Errorf("expected token reduction for doc, got %d >= %d", resp.PrunedTokens, resp.OriginalTokens)
	}
	if resp.ContentType != ContentDoc {
		t.Errorf("expected ContentDoc, got %v", resp.ContentType)
	}
}

func TestIRPruner_PruneLogs(t *testing.T) {
	cfg := Config{Threshold: 0.5, CacheCapacity: 64}
	pruner := NewIRPruner(cfg)

	log := strings.Repeat("2026-02-15T10:00:00Z INFO  Starting server on port 8080\n", 5) +
		"\n" +
		strings.Repeat("2026-02-15T10:00:01Z ERROR Connection refused to database\n", 5) +
		"\n" +
		strings.Repeat("2026-02-15T10:00:02Z DEBUG Processing background task\n", 20)

	resp, err := pruner.Prune(context.Background(), PruneRequest{
		Content:   log,
		Query:     "error database",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PrunedTokens >= resp.OriginalTokens {
		t.Errorf("expected token reduction for logs, got %d >= %d", resp.PrunedTokens, resp.OriginalTokens)
	}
}

func TestIRPruner_EmptyContent(t *testing.T) {
	cfg := Config{Threshold: 0.5, CacheCapacity: 64}
	pruner := NewIRPruner(cfg)

	resp, err := pruner.Prune(context.Background(), PruneRequest{
		Content: "",
		Query:   "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PrunedContent != "" {
		t.Error("expected empty pruned content for empty input")
	}
}

func TestIRPruner_ContentTypeHint(t *testing.T) {
	cfg := Config{Threshold: 0.5, CacheCapacity: 64}
	pruner := NewIRPruner(cfg)

	// Force content type via hint even though content looks like code
	code := strings.Repeat("func main() {\n\tfmt.Println(\"hello\")\n}\n", 10)
	resp, err := pruner.Prune(context.Background(), PruneRequest{
		Content:     code,
		Query:       "main",
		Threshold:   0.5,
		ContentType: ContentDoc, // Force doc treatment
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ContentType != ContentDoc {
		t.Errorf("expected ContentDoc from hint, got %v", resp.ContentType)
	}
}

func TestIRPruner_Health(t *testing.T) {
	pruner := NewIRPruner(Config{})
	if err := pruner.Health(context.Background()); err != nil {
		t.Errorf("expected nil health, got %v", err)
	}
}

func TestIRPruner_Close(t *testing.T) {
	pruner := NewIRPruner(Config{})
	if err := pruner.Close(); err != nil {
		t.Errorf("expected nil close, got %v", err)
	}
}

func TestIRPruner_CacheHit(t *testing.T) {
	cfg := Config{Threshold: 0.5, CacheCapacity: 64}
	pruner := NewIRPruner(cfg)

	content := "## Auth\n\nLogin system.\n\n" + strings.Repeat("## Other\n\nUnrelated content.\n\n", 10)
	req := PruneRequest{Content: content, Query: "auth login", Threshold: 0.5}

	// First call
	resp1, err := pruner.Prune(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	// Second call should hit cache and be faster
	resp2, err := pruner.Prune(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp1.PrunedTokens != resp2.PrunedTokens {
		t.Errorf("cache should produce identical results: %d vs %d", resp1.PrunedTokens, resp2.PrunedTokens)
	}
}
