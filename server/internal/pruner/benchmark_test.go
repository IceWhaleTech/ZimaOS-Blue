package pruner

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// --- 8.1 Benchmark Harness: Latency on 1k/5k/10k LOC files ---

func generateGoCode(lines int) string {
	var b strings.Builder
	b.WriteString("package main\n\nimport \"fmt\"\n\n")
	funcNum := 0
	for i := 4; i < lines; {
		funcNum++
		fmt.Fprintf(&b, "func handler%d(req string) error {\n", funcNum)
		i++
		bodyLines := 10
		if i+bodyLines > lines {
			bodyLines = lines - i - 1
		}
		for j := 0; j < bodyLines; j++ {
			fmt.Fprintf(&b, "\tx%d := fmt.Sprintf(\"val%%d\", %d)\n", j, j)
			i++
		}
		b.WriteString("\treturn nil\n}\n\n")
		i += 2
	}
	return b.String()
}

func benchmarkBackend(b *testing.B, backend Backend, code, query string) {
	req := PruneRequest{Code: code, Query: query, Threshold: 0.5}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := backend.Prune(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Local backend benchmarks
func BenchmarkLocalBackend_1kLOC(b *testing.B) {
	code := generateGoCode(1000)
	backend := NewLocalBackend(Config{Threshold: 0.5})
	benchmarkBackend(b, backend, code, "handler request error")
}

func BenchmarkLocalBackend_5kLOC(b *testing.B) {
	code := generateGoCode(5000)
	backend := NewLocalBackend(Config{Threshold: 0.5})
	benchmarkBackend(b, backend, code, "handler request error")
}

func BenchmarkLocalBackend_10kLOC(b *testing.B) {
	code := generateGoCode(10000)
	backend := NewLocalBackend(Config{Threshold: 0.5})
	benchmarkBackend(b, backend, code, "handler request error")
}

// BM25 backend benchmarks
func BenchmarkBM25Backend_1kLOC(b *testing.B) {
	code := generateGoCode(1000)
	backend := NewBM25Backend(Config{Threshold: 0.5})
	benchmarkBackend(b, backend, code, "handler request error")
}

func BenchmarkBM25Backend_5kLOC(b *testing.B) {
	code := generateGoCode(5000)
	backend := NewBM25Backend(Config{Threshold: 0.5})
	benchmarkBackend(b, backend, code, "handler request error")
}

func BenchmarkBM25Backend_10kLOC(b *testing.B) {
	code := generateGoCode(10000)
	backend := NewBM25Backend(Config{Threshold: 0.5})
	benchmarkBackend(b, backend, code, "handler request error")
}

// --- 8.2 Recall@K: measures how many relevant lines are retained ---

// computeRecallAtK calculates the fraction of "relevant" lines retained after pruning.
// A line is considered relevant if it contains any of the query terms.
func computeRecallAtK(original, pruned string, queryTerms []string) float64 {
	origLines := strings.Split(original, "\n")
	prunedSet := make(map[string]bool)
	for _, line := range strings.Split(pruned, "\n") {
		prunedSet[strings.TrimSpace(line)] = true
	}

	relevant := 0
	retained := 0
	for _, line := range origLines {
		trimmed := strings.TrimSpace(line)
		isRelevant := false
		for _, qt := range queryTerms {
			if strings.Contains(strings.ToLower(trimmed), qt) {
				isRelevant = true
				break
			}
		}
		if isRelevant {
			relevant++
			if prunedSet[trimmed] {
				retained++
			}
		}
	}
	if relevant == 0 {
		return 1.0
	}
	return float64(retained) / float64(relevant)
}

func TestRecallAtK_LocalBackend(t *testing.T) {
	code := generateGoCode(1000)
	backend := NewLocalBackend(Config{Threshold: 0.5})
	resp, err := backend.Prune(context.Background(), PruneRequest{
		Code:      code,
		Query:     "handler request error",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}

	recall := computeRecallAtK(code, resp.PrunedCode, []string{"handler", "request", "error"})
	t.Logf("Local Recall@K: %.2f (1k LOC)", recall)
	if recall < 0.5 {
		t.Errorf("recall too low: %.2f (expected >= 0.5)", recall)
	}
}

func TestRecallAtK_BM25Backend(t *testing.T) {
	code := generateGoCode(1000)
	backend := NewBM25Backend(Config{Threshold: 0.5})
	resp, err := backend.Prune(context.Background(), PruneRequest{
		Code:      code,
		Query:     "handler request error",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}

	recall := computeRecallAtK(code, resp.PrunedCode, []string{"handler", "request", "error"})
	t.Logf("BM25 Recall@K: %.2f (1k LOC)", recall)
	if recall < 0.5 {
		t.Errorf("recall too low: %.2f (expected >= 0.5)", recall)
	}
}

// --- 8.3 Token Reduction Calculation ---

func TestTokenReduction_LocalBackend(t *testing.T) {
	sizes := []int{1000, 5000, 10000}
	backend := NewLocalBackend(Config{Threshold: 0.5})

	for _, sz := range sizes {
		code := generateGoCode(sz)
		resp, err := backend.Prune(context.Background(), PruneRequest{
			Code:      code,
			Query:     "handler request error",
			Threshold: 0.5,
		})
		if err != nil {
			t.Fatal(err)
		}
		reduction := 1.0 - resp.CompressionRate
		t.Logf("Local %dk LOC: %d→%d tokens (%.1f%% reduction, %.2fms)",
			sz/1000, resp.OriginalTokens, resp.PrunedTokens, reduction*100, resp.LatencyMs)
	}
}

func TestTokenReduction_BM25Backend(t *testing.T) {
	sizes := []int{1000, 5000, 10000}
	backend := NewBM25Backend(Config{Threshold: 0.5})

	for _, sz := range sizes {
		code := generateGoCode(sz)
		resp, err := backend.Prune(context.Background(), PruneRequest{
			Code:      code,
			Query:     "handler request error",
			Threshold: 0.5,
		})
		if err != nil {
			t.Fatal(err)
		}
		reduction := 1.0 - resp.CompressionRate
		t.Logf("BM25 %dk LOC: %d→%d tokens (%.1f%% reduction, %.2fms)",
			sz/1000, resp.OriginalTokens, resp.PrunedTokens, reduction*100, resp.LatencyMs)
	}
}

// --- 8.4 Memory Profiling ---

func BenchmarkMemory_LocalBackend_10kLOC(b *testing.B) {
	code := generateGoCode(10000)
	backend := NewLocalBackend(Config{Threshold: 0.5})
	req := PruneRequest{Code: code, Query: "handler request", Threshold: 0.5}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Prune(ctx, req)
	}
}

func BenchmarkMemory_BM25Backend_10kLOC(b *testing.B) {
	code := generateGoCode(10000)
	backend := NewBM25Backend(Config{Threshold: 0.5})
	req := PruneRequest{Code: code, Query: "handler request", Threshold: 0.5}
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Prune(ctx, req)
	}
}

// --- IRPruner Benchmarks (Unified: Code + Non-Code) ---

func generateMarkdown(lines int) string {
	var b strings.Builder
	sectionNum := 0
	for i := 0; i < lines; {
		sectionNum++
		fmt.Fprintf(&b, "## Section %d: Topic About Something\n\n", sectionNum)
		i += 2
		paraLines := 8
		if i+paraLines > lines {
			paraLines = lines - i
		}
		for j := 0; j < paraLines; j++ {
			fmt.Fprintf(&b, "This is paragraph %d line %d discussing various topics related to the section above.\n", sectionNum, j)
			i++
		}
		b.WriteString("\n")
		i++
	}
	return b.String()
}

func generateLogOutput(lines int) string {
	var b strings.Builder
	levels := []string{"INFO", "DEBUG", "WARN", "ERROR"}
	msgs := []string{
		"Processing request from client",
		"Database query completed in 45ms",
		"Cache miss for key user_session",
		"Connection pool exhausted, waiting",
		"Handler returned status 200",
	}
	for i := 0; i < lines; i++ {
		level := levels[i%len(levels)]
		msg := msgs[i%len(msgs)]
		fmt.Fprintf(&b, "2026-02-15T10:%02d:%02dZ %s  %s_%d\n", i/60%60, i%60, level, msg, i)
		if i%20 == 19 {
			b.WriteString("\n") // group separator
		}
	}
	return b.String()
}

func benchmarkIRPruner(b *testing.B, content, query string) {
	backend := NewIRPruner(Config{Threshold: 0.5, CacheCapacity: 256, MinLines: 5})
	req := PruneRequest{Content: content, Query: query, Threshold: 0.5}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := backend.Prune(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Code benchmarks
func BenchmarkIRPruner_Code_1k(b *testing.B)  { benchmarkIRPruner(b, generateGoCode(1000), "handler request error") }
func BenchmarkIRPruner_Code_5k(b *testing.B)  { benchmarkIRPruner(b, generateGoCode(5000), "handler request error") }
func BenchmarkIRPruner_Code_10k(b *testing.B) { benchmarkIRPruner(b, generateGoCode(10000), "handler request error") }

// Doc benchmarks
func BenchmarkIRPruner_Doc_1k(b *testing.B)  { benchmarkIRPruner(b, generateMarkdown(1000), "section topic") }
func BenchmarkIRPruner_Doc_5k(b *testing.B)  { benchmarkIRPruner(b, generateMarkdown(5000), "section topic") }
func BenchmarkIRPruner_Doc_10k(b *testing.B) { benchmarkIRPruner(b, generateMarkdown(10000), "section topic") }

// Log benchmarks
func BenchmarkIRPruner_Log_10k(b *testing.B) { benchmarkIRPruner(b, generateLogOutput(10000), "error database") }

// Memory profiling
func BenchmarkMemory_IRPruner_Code_10k(b *testing.B) {
	code := generateGoCode(10000)
	backend := NewIRPruner(Config{Threshold: 0.5, CacheCapacity: 256, MinLines: 5})
	req := PruneRequest{Content: code, Query: "handler request", Threshold: 0.5}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Prune(ctx, req)
	}
}

func BenchmarkMemory_IRPruner_Doc_10k(b *testing.B) {
	doc := generateMarkdown(10000)
	backend := NewIRPruner(Config{Threshold: 0.5, CacheCapacity: 256, MinLines: 5})
	req := PruneRequest{Content: doc, Query: "section topic", Threshold: 0.5}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.Prune(ctx, req)
	}
}

// --- IRPruner Recall@K and Token Reduction ---

func TestRecallAtK_IRPruner_Code(t *testing.T) {
	code := generateGoCode(1000)
	backend := NewIRPruner(Config{Threshold: 0.5, CacheCapacity: 64, MinLines: 5})
	resp, err := backend.Prune(context.Background(), PruneRequest{
		Content:   code,
		Query:     "handler request error",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	recall := computeRecallAtK(code, resp.PrunedContent, []string{"handler", "request", "error"})
	t.Logf("IRPruner Code Recall@K: %.2f (1k LOC)", recall)
	if recall < 0.5 {
		t.Errorf("recall too low: %.2f", recall)
	}
}

func TestRecallAtK_IRPruner_Doc(t *testing.T) {
	doc := generateMarkdown(1000)
	backend := NewIRPruner(Config{Threshold: 0.5, CacheCapacity: 64, MinLines: 5})
	resp, err := backend.Prune(context.Background(), PruneRequest{
		Content:   doc,
		Query:     "section topic",
		Threshold: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	recall := computeRecallAtK(doc, resp.PrunedContent, []string{"section", "topic"})
	t.Logf("IRPruner Doc Recall@K: %.2f (1k lines)", recall)
	if recall < 0.3 {
		t.Errorf("recall too low: %.2f", recall)
	}
}

func TestTokenReduction_IRPruner(t *testing.T) {
	sizes := []int{1000, 5000, 10000}
	backend := NewIRPruner(Config{Threshold: 0.5, CacheCapacity: 64, MinLines: 5})

	for _, sz := range sizes {
		// Code
		code := generateGoCode(sz)
		resp, err := backend.Prune(context.Background(), PruneRequest{
			Content: code, Query: "handler request error", Threshold: 0.5,
		})
		if err != nil {
			t.Fatal(err)
		}
		reduction := 1.0 - resp.CompressionRate
		t.Logf("IRPruner Code %dk: %d→%d tokens (%.1f%% reduction, %.2fms)",
			sz/1000, resp.OriginalTokens, resp.PrunedTokens, reduction*100, resp.LatencyMs)

		// Doc
		doc := generateMarkdown(sz)
		resp, err = backend.Prune(context.Background(), PruneRequest{
			Content: doc, Query: "section topic", Threshold: 0.5,
		})
		if err != nil {
			t.Fatal(err)
		}
		reduction = 1.0 - resp.CompressionRate
		t.Logf("IRPruner Doc  %dk: %d→%d tokens (%.1f%% reduction, %.2fms)",
			sz/1000, resp.OriginalTokens, resp.PrunedTokens, reduction*100, resp.LatencyMs)
	}
}
