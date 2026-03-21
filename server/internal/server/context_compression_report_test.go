package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type contextCompressionExportReport struct {
	GeneratedAtRFC3339 string                            `json:"generated_at_rfc3339"`
	Fixture            string                            `json:"fixture"`
	Deterministic      contextCompressionBenchmarkReport `json:"deterministic"`
	Judge              *contextCompressionLLMJudgeReport `json:"judge,omitempty"`
}

func TestContextCompressionBenchmark_WriteReportOptional(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ZIMA_WRITE_CONTEXT_COMPRESSION_REPORT")) != "1" {
		t.Skip("set ZIMA_WRITE_CONTEXT_COMPRESSION_REPORT=1 to write the context compression benchmark report")
	}

	fixture := middleNeedleFixture()
	deterministic := buildComparisonReport(t, fixture)
	exported := contextCompressionExportReport{
		GeneratedAtRFC3339: time.Now().Format(time.RFC3339),
		Fixture:            fixture.Name,
		Deterministic:      deterministic,
	}

	baseURL := strings.TrimSpace(os.Getenv("ZIMA_CONTEXT_COMPRESSION_JUDGE_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("ZIMA_CONTEXT_COMPRESSION_JUDGE_API_KEY"))
	modelID := strings.TrimSpace(os.Getenv("ZIMA_CONTEXT_COMPRESSION_JUDGE_MODEL"))
	if baseURL != "" && apiKey != "" && modelID != "" {
		judgeReport, err := runContextCompressionLLMJudge(
			context.Background(),
			baseURL,
			apiKey,
			modelID,
			fixture,
			buildComparisonCandidates(t, fixture),
		)
		if err != nil {
			t.Fatalf("runContextCompressionLLMJudge: %v", err)
		}
		exported.Judge = &judgeReport
	}

	reportDir, err := contextCompressionReportDir()
	if err != nil {
		t.Fatalf("contextCompressionReportDir: %v", err)
	}
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", reportDir, err)
	}

	jsonPath := filepath.Join(reportDir, "context_compression_report.json")
	mdPath := filepath.Join(reportDir, "context_compression_report.md")

	payload, err := json.MarshalIndent(exported, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(exported): %v", err)
	}
	if err := os.WriteFile(jsonPath, append(payload, '\n'), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", jsonPath, err)
	}

	markdown := renderContextCompressionReportMarkdown(exported)
	if err := os.WriteFile(mdPath, []byte(markdown), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", mdPath, err)
	}

	t.Logf("wrote context compression report: %s", jsonPath)
	t.Logf("wrote context compression report: %s", mdPath)
}

func contextCompressionReportDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("ZIMA_CONTEXT_COMPRESSION_REPORT_DIR")); dir != "" {
		return dir, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for current := wd; current != "" && current != string(filepath.Separator); current = filepath.Dir(current) {
		candidate := filepath.Join(current, "docs", "reports")
		if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}
	return "", fmt.Errorf("could not locate docs/reports from working directory %q", wd)
}

func renderContextCompressionReportMarkdown(report contextCompressionExportReport) string {
	var b strings.Builder
	b.WriteString("# Context Compression Benchmark Report\n\n")
	b.WriteString("- Generated at: ")
	b.WriteString(report.GeneratedAtRFC3339)
	b.WriteString("\n")
	b.WriteString("- Fixture: `")
	b.WriteString(report.Fixture)
	b.WriteString("`\n")

	if winner := topCandidateByTokenReduction(report.Deterministic.Candidates); winner != nil {
		b.WriteString("- Best deterministic compression ratio: `")
		b.WriteString(winner.Name)
		b.WriteString("` (`")
		b.WriteString(formatPercent(winner.TokenReduction))
		b.WriteString("` token reduction)\n")
	}
	if report.Judge != nil {
		b.WriteString("- Live LLM judge winner: `")
		b.WriteString(report.Judge.BestCandidate)
		b.WriteString("` using `")
		b.WriteString(report.Judge.JudgeModel)
		b.WriteString("`\n")
	}

	b.WriteString("\n## Deterministic Metrics\n\n")
	b.WriteString("| Candidate | key_fact_recall | identifier_fidelity | token_reduction |\n")
	b.WriteString("|---|---:|---:|---:|\n")
	for _, candidate := range report.Deterministic.Candidates {
		b.WriteString("| `")
		b.WriteString(candidate.Name)
		b.WriteString("` | ")
		b.WriteString(formatScore(candidate.KeyFactRecall))
		b.WriteString(" | ")
		b.WriteString(formatScore(candidate.IdentifierFidelity))
		b.WriteString(" | ")
		b.WriteString(formatPercent(candidate.TokenReduction))
		b.WriteString(" |\n")
	}

	if len(report.Deterministic.Safety) > 0 {
		keys := make([]string, 0, len(report.Deterministic.Safety))
		for key := range report.Deterministic.Safety {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b.WriteString("\n## Safety Metrics\n\n")
		b.WriteString("| Metric | Value |\n")
		b.WriteString("|---|---:|\n")
		for _, key := range keys {
			b.WriteString("| `")
			b.WriteString(key)
			b.WriteString("` | ")
			b.WriteString(formatScore(report.Deterministic.Safety[key]))
			b.WriteString(" |\n")
		}
	}

	if report.Judge != nil {
		b.WriteString("\n## Live LLM Judge\n\n")
		b.WriteString("| Candidate | overall | latest_intent_alignment | fact_preservation | identifier_fidelity | stale_intent_safety | conciseness |\n")
		b.WriteString("|---|---:|---:|---:|---:|---:|---:|\n")
		for _, candidate := range sortedJudgeCandidates(report.Judge.Candidates) {
			b.WriteString("| `")
			b.WriteString(candidate.Name)
			b.WriteString("` | ")
			b.WriteString(formatScore(candidate.Overall))
			b.WriteString(" | ")
			b.WriteString(formatScore(candidate.LatestIntentAlignment))
			b.WriteString(" | ")
			b.WriteString(formatScore(candidate.FactPreservation))
			b.WriteString(" | ")
			b.WriteString(formatScore(candidate.IdentifierFidelity))
			b.WriteString(" | ")
			b.WriteString(formatScore(candidate.StaleIntentSafety))
			b.WriteString(" | ")
			b.WriteString(formatScore(candidate.Conciseness))
			b.WriteString(" |\n")
		}

		b.WriteString("\n## Judge Summary\n\n")
		b.WriteString(report.Judge.Summary)
		b.WriteString("\n")
	}

	if len(report.Deterministic.Notes) > 0 {
		b.WriteString("\n## Notes\n\n")
		for _, note := range report.Deterministic.Notes {
			b.WriteString("- ")
			b.WriteString(note)
			b.WriteString("\n")
		}
	}
	if report.Judge != nil && len(report.Judge.Notes) > 0 {
		for _, note := range report.Judge.Notes {
			b.WriteString("- ")
			b.WriteString(note)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func topCandidateByTokenReduction(candidates []contextCompressionCandidateReport) *contextCompressionCandidateReport {
	if len(candidates) == 0 {
		return nil
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.TokenReduction > best.TokenReduction {
			best = candidate
		}
	}
	return &best
}

func sortedJudgeCandidates(candidates []contextCompressionLLMJudgeCandidate) []contextCompressionLLMJudgeCandidate {
	sorted := append([]contextCompressionLLMJudgeCandidate(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Overall == sorted[j].Overall {
			return sorted[i].Name < sorted[j].Name
		}
		return sorted[i].Overall > sorted[j].Overall
	})
	return sorted
}

func formatScore(v float64) string {
	return fmt.Sprintf("%.2f", v)
}

func formatPercent(v float64) string {
	return fmt.Sprintf("%.2f%%", v*100)
}
