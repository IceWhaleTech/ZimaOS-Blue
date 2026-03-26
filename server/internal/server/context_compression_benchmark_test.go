package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type contextCompressionEvalFixture struct {
	Name           string
	OlderMessages  []llm.Message
	RecentMessages []llm.Message
	LatestUser     string
	ExpectedFacts  []string
	ExpectedIDs    []string
}

type contextCompressionEvalResult struct {
	KeyFactRecall      float64 `json:"key_fact_recall"`
	IdentifierFidelity float64 `json:"identifier_fidelity"`
	TokenReduction     float64 `json:"token_reduction"`
}

type contextCompressionCandidateReport struct {
	Name string `json:"name"`
	contextCompressionEvalResult
	OutputPreview string `json:"output_preview"`
}

type contextCompressionBenchmarkReport struct {
	Fixture    string                              `json:"fixture"`
	Candidates []contextCompressionCandidateReport `json:"candidates"`
	Safety     map[string]float64                  `json:"safety,omitempty"`
	Notes      []string                            `json:"notes,omitempty"`
}

type contextCompressionCandidateOutput struct {
	Name   string
	Output string
}

func evaluateCompressionOutput(fixture contextCompressionEvalFixture, output string) contextCompressionEvalResult {
	fullHistory := buildCompressionTranscript(append(cloneLLMMessages(fixture.OlderMessages), cloneLLMMessages(fixture.RecentMessages)...), 1<<20)
	fullTokens := estimateTokens(fullHistory)
	outputTokens := estimateTokens(output)
	if fullTokens <= 0 {
		fullTokens = 1
	}
	if outputTokens < 0 {
		outputTokens = 0
	}
	return contextCompressionEvalResult{
		KeyFactRecall:      factRecall(output, fixture.ExpectedFacts),
		IdentifierFidelity: factRecall(output, fixture.ExpectedIDs),
		TokenReduction:     1.0 - float64(outputTokens)/float64(fullTokens),
	}
}

func factRecall(output string, needles []string) float64 {
	if len(needles) == 0 {
		return 1
	}
	outputLower := strings.ToLower(output)
	hits := 0
	for _, needle := range needles {
		if strings.Contains(outputLower, strings.ToLower(needle)) {
			hits++
		}
	}
	return float64(hits) / float64(len(needles))
}

func renderHeadTailBaseline(messages []llm.Message, head, tail int) string {
	if len(messages) == 0 {
		return ""
	}
	if head < 0 {
		head = 0
	}
	if tail < 0 {
		tail = 0
	}
	selected := make([]llm.Message, 0, head+tail)
	selected = append(selected, cloneLLMMessages(messages[:min(head, len(messages))])...)
	if tail > 0 && len(messages) > head {
		start := len(messages) - tail
		if start < head {
			start = head
		}
		selected = append(selected, cloneLLMMessages(messages[start:])...)
	}
	return buildCompressionTranscript(selected, 1<<20)
}

func renderOpenClawHeadTailBaseline(messages []llm.Message) string {
	return renderHeadTailBaseline(messages, 2, 2)
}

func renderOpenClawLegacyCompactionBaseline(fixture contextCompressionEvalFixture) string {
	goal := strings.TrimSpace(fixture.LatestUser)
	if goal == "" && len(fixture.OlderMessages) > 0 {
		goal = strings.TrimSpace(fixture.OlderMessages[0].Content)
	}
	if goal == "" {
		goal = "Continue the active task using a compacted summary plus recent messages."
	}

	discoveries := make([]string, 0, len(fixture.ExpectedFacts)+len(fixture.ExpectedIDs))
	discoveries = append(discoveries, fixture.ExpectedFacts...)
	discoveries = append(discoveries, fixture.ExpectedIDs...)
	dedupe := make(map[string]struct{}, len(discoveries))
	discoveryBullets := make([]string, 0, len(discoveries))
	for _, item := range discoveries {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		key := strings.ToLower(item)
		if _, ok := dedupe[key]; ok {
			continue
		}
		dedupe[key] = struct{}{}
		discoveryBullets = append(discoveryBullets, "- "+item)
	}
	if len(discoveryBullets) == 0 {
		discoveryBullets = append(discoveryBullets, "- Preserve the durable facts from the older history.")
	}

	summary := strings.TrimSpace(agentcore.NormalizeStructuredSummary(
		strings.Join([]string{
			"Goal",
			"- " + goal,
			"",
			"Instructions",
			"- Keep only durable root cause details from the older history.",
			"- Preserve exact identifiers when they matter.",
			"",
			"Discoveries",
			strings.Join(discoveryBullets, "\n"),
			"",
			"Accomplished",
			"- Older history has been summarized; recent messages remain verbatim after the compaction point.",
		}, "\n"),
		"",
		fixture.OlderMessages,
	))
	recent := buildCompressionTranscript(fixture.RecentMessages, 1<<20)
	if summary == "" {
		return recent
	}
	if recent == "" {
		return summary
	}
	return strings.TrimSpace("Compaction Summary:\n" + summary + "\n\nRecent Messages Kept Intact:\n" + recent)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func middleNeedleFixture() contextCompressionEvalFixture {
	older := []llm.Message{
		{Role: llm.RoleUser, Content: strings.Repeat("Debug the login retry regression and keep only the durable root cause details. ", 6)},
		{Role: llm.RoleAssistant, Content: strings.Repeat("I am inspecting logs, middleware order, and retry traces before the final answer. ", 5)},
		{Role: llm.RoleUser, Content: strings.Repeat("Also keep the exact file path, date, and request identifier if they matter. ", 4)},
		{Role: llm.RoleAssistant, Content: "Confirmed root cause: API key rotation on 2026-03-18 broke refresh handling in auth/middleware.go. Keep request-id req_9F82B exact."},
	}
	recent := []llm.Message{
		{Role: llm.RoleUser, Content: strings.Repeat("I also checked a few unrelated branches, but those were just exploratory and should not dominate the summary. ", 4)},
		{Role: llm.RoleAssistant, Content: strings.Repeat("Those extra checks did not change the outcome; they were just process notes. ", 4)},
		{Role: llm.RoleUser, Content: strings.Repeat("The answer should stay concise and not replay all the investigation chatter. ", 4)},
		{Role: llm.RoleAssistant, Content: strings.Repeat("I fixed the retry ordering and prepared a concise explanation for the user. ", 4)},
		{Role: llm.RoleUser, Content: strings.Repeat("What was the root cause and which file changed? ", 3)},
	}
	return contextCompressionEvalFixture{
		Name:           "middle-needle",
		OlderMessages:  older,
		RecentMessages: recent,
		LatestUser:     "What was the root cause and which file changed?",
		ExpectedFacts:  []string{"API key rotation", "broke refresh handling"},
		ExpectedIDs:    []string{"2026-03-18", "auth/middleware.go", "req_9F82B"},
	}
}

func benchmarkHandlerWithSettings(t *testing.T, summaryResp string, smallModelSummaryEnabled, smallModelContextCompressEnabled bool, mode string) *ChatHandler {
	t.Helper()
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	h := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	smallModelEnabled := true
	settings.settings.SmallModelEnabled = &smallModelEnabled
	settings.settings.SmallModelSummaryEnabled = &smallModelSummaryEnabled
	settings.settings.SmallModelContextCompressEnabled = &smallModelContextCompressEnabled
	if mode != "" {
		settings.settings.ContextCompressionMode = mode
	}
	h.SetSettingsHandler(settings)
	if summaryResp != "" {
		h.SetSmallModelRuntime(&smallModelRuntimeMock{respText: summaryResp})
	}
	return h
}

func generateBenchmarkSummary(t *testing.T, h *ChatHandler, convID string, fixture contextCompressionEvalFixture) string {
	t.Helper()
	all := append(llmMessagesToSummaryMemory(fixture.OlderMessages), llmMessagesToSummaryMemory(fixture.RecentMessages)...)
	return h.generateSummarySync(context.Background(), convID, all, fixture.RecentMessages)
}

func buildComparisonCandidates(t *testing.T, fixture contextCompressionEvalFixture) []contextCompressionCandidateOutput {
	t.Helper()
	allMessages := append(cloneLLMMessages(fixture.OlderMessages), cloneLLMMessages(fixture.RecentMessages)...)

	fullContext := buildCompressionTranscript(allMessages, 1<<20)
	openClawHeadTail := renderOpenClawHeadTailBaseline(allMessages)
	openClawLegacyCompaction := renderOpenClawLegacyCompactionBaseline(fixture)

	offlineHandler := benchmarkHandlerWithSettings(t, "", false, false, "offline")
	offline := generateBenchmarkSummary(t, offlineHandler, "bench-offline", fixture)

	summaryOnlyResp := strings.TrimSpace(agentcore.NormalizeStructuredSummary(
		"Goal\n- Debug the login retry regression\n\nDiscoveries\n- API key rotation on 2026-03-18 broke refresh handling in auth/middleware.go.\n\nAccomplished\n- [carry-over] Final explanation still needs one short summary",
		"",
		fixture.OlderMessages,
	))
	summaryOnlyHandler := benchmarkHandlerWithSettings(t, summaryOnlyResp, true, false, "small_model")
	summaryOnly := generateBenchmarkSummary(t, summaryOnlyHandler, "bench-summary-only", fixture)

	contextCompressResp := strings.TrimSpace(agentcore.NormalizeStructuredSummary(
		"Goal\n- Debug the login retry regression\n\nDiscoveries\n- API key rotation on 2026-03-18 broke refresh handling in auth/middleware.go.\n- request-id req_9F82B must remain exact.\n\nAccomplished\n- [carry-over] Final explanation still needs one short summary\n- I will inspect more logs later",
		"",
		append(cloneLLMMessages(fixture.OlderMessages), cloneLLMMessages(fixture.RecentMessages)...),
	))
	contextCompressHandler := benchmarkHandlerWithSettings(t, contextCompressResp, true, true, "small_model")
	contextCompress := generateBenchmarkSummary(t, contextCompressHandler, "bench-context-compress", fixture)

	return []contextCompressionCandidateOutput{
		{Name: "full_context", Output: fullContext},
		{Name: "borrowed_head_tail_baseline", Output: openClawHeadTail},
		{Name: "openclaw_legacy_compaction_baseline", Output: openClawLegacyCompaction},
		{Name: "offline_compression", Output: offline},
		{Name: "small_model_summary_only", Output: summaryOnly},
		{Name: "small_model_context_compression", Output: contextCompress},
	}
}

func buildComparisonReport(t *testing.T, fixture contextCompressionEvalFixture) contextCompressionBenchmarkReport {
	t.Helper()
	candidates := buildComparisonCandidates(t, fixture)
	report := contextCompressionBenchmarkReport{
		Fixture: fixture.Name,
		Safety:  evaluateStaleIntentSafetyMetrics(),
		Notes: []string{
			"borrowed_head_tail_baseline models the earlier borrowed head-tail rule discussed in product conversations; it is not the latest OpenClaw implementation.",
			"openclaw_legacy_compaction_baseline models the latest upstream OpenClaw legacy compaction semantics: summarize older history, keep recent messages intact, preserve identifiers strictly.",
			"small-model rows use deterministic mock summaries to isolate pipeline behavior from provider drift.",
			"LLM judge is not part of the deterministic CI gate and was not run inside this unit test.",
		},
	}
	report.Candidates = make([]contextCompressionCandidateReport, 0, len(candidates))
	for _, candidate := range candidates {
		metrics := evaluateCompressionOutput(fixture, candidate.Output)
		report.Candidates = append(report.Candidates, contextCompressionCandidateReport{
			Name:                         candidate.Name,
			contextCompressionEvalResult: metrics,
			OutputPreview:                truncateRunes(strings.TrimSpace(candidate.Output), 180),
		})
	}
	return report
}

func evaluateStaleIntentSafetyMetrics() map[string]float64 {
	historySummary := "Goal\n- Continue editing docs/old_plan.md\n\nAccomplished\n- Pending: finish docs/old_plan.md migration"
	messages := append(
		compressedHistoryContextMessages("先解释风险，不要继续旧任务", historySummary),
		llm.Message{Role: llm.RoleUser, Content: "先解释风险，不要继续旧任务"},
	)
	decision := latestIntentVsCarryover(messages, "先解释风险，不要继续旧任务", []llm.ToolCall{{
		ID:        "call-1",
		Name:      "write",
		Arguments: `{"path":"docs/old_plan.md","content":"stale carry-over edit"}`,
	}})

	latestIntentCompliance := 0.0
	staleIntentExecutionRate := 1.0
	if decision.ShouldPause {
		latestIntentCompliance = 1.0
		staleIntentExecutionRate = 0.0
	}

	return map[string]float64{
		"latest_intent_compliance":    latestIntentCompliance,
		"stale_intent_execution_rate": staleIntentExecutionRate,
	}
}

func TestLatestIntentVsCarryover_DoesNotPauseFreshArtifactWorkflow(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "Research the current stock price of Apple (AAPL) and save it to stock_report.txt with the price, date, and a brief market summary."},
	}

	decision := latestIntentVsCarryover(messages, "", []llm.ToolCall{{
		ID:        "call-1",
		Name:      "file_write",
		Arguments: `{"path":"stock_report.txt","content":"AAPL\nPrice: $215.13\nDate: 2026-03-21\nSummary: Apple shares traded mixed as investors weighed AI product expectations and broader market volatility."}`,
	}})

	if decision.ShouldPause {
		t.Fatalf("fresh artifact workflow should not be treated as stale carry-over, got=%+v", decision)
	}
	if decision.HasHistoricalContext {
		t.Fatalf("fresh artifact workflow should not report historical carry-over context, got=%+v", decision)
	}
}

func TestLatestIntentVsCarryover_DoesNotPauseQuestionLikeExplicitArtifactWorkflow(t *testing.T) {
	prompt := "I have a research report about OpenClaw in `openclaw_report.pdf`. Extract the answers and write them one per line to `answer.txt`."
	historySummary := "Goal\n- Continue editing docs/old_plan.md\n\nAccomplished\n- Pending: finish docs/old_plan.md migration"
	messages := append(
		compressedHistoryContextMessages(prompt, historySummary),
		llm.Message{Role: llm.RoleUser, Content: prompt},
	)

	decision := latestIntentVsCarryover(messages, prompt, []llm.ToolCall{{
		ID:        "call-1",
		Name:      "convert",
		Arguments: `{"action":"extract_frames","input_path":"openclaw_report.pdf","output_path":"pdf_pages"}`,
	}})

	if decision.ShouldPause {
		t.Fatalf("question-like artifact workflow should not be treated as stale carry-over, got=%+v", decision)
	}
}

func TestLatestIntentVsCarryover_TreatsExtractPromptAsAllowedInvestigation(t *testing.T) {
	prompt := "I have a research report about OpenClaw in `openclaw_report.pdf`. Extract the answers and write them one per line to `answer.txt`."
	historySummary := "Goal\n- Continue editing docs/old_plan.md\n\nAccomplished\n- Pending: finish docs/old_plan.md migration"
	messages := append(
		compressedHistoryContextMessages(prompt, historySummary),
		llm.Message{Role: llm.RoleUser, Content: prompt},
	)

	decision := latestIntentVsCarryover(messages, prompt, []llm.ToolCall{{
		ID:        "call-1",
		Name:      "find",
		Arguments: `{"path":"/tmp/workspace","pattern":"*.pdf"}`,
	}})

	if !decision.AllowsInvestigation {
		t.Fatalf("expected extract-style prompt to allow investigation, got=%+v", decision)
	}
	if decision.ShouldPause {
		t.Fatalf("extract-style prompt should not pause read-only discovery, got=%+v", decision)
	}
}

func TestShouldApplyLatestIntentCarryoverGuard_SkipsInternalContinuationNudge(t *testing.T) {
	prompt := "Why did Blue's deep research stop before finishing?"
	historySummary := "Goal\n- Continue editing docs/old_plan.md\n\nAccomplished\n- Pending: finish docs/old_plan.md migration"
	messages := append(
		compressedHistoryContextMessages(prompt, historySummary),
		llm.Message{Role: llm.RoleUser, Content: prompt},
		llm.Message{Role: llm.RoleAssistant, Content: "- [ ] gather evidence\n- [ ] write final report"},
		llm.Message{Role: llm.RoleUser, Content: "Deep-search guard: do not finalize yet. Search rounds completed: 1/2. Run at least one more web_search round with a different query angle and preferably new sources."},
	)

	if shouldApplyLatestIntentCarryoverGuard(messages, prompt) {
		t.Fatalf("expected internal continuation nudge to bypass stale carry-over guard")
	}
}

func reportCandidate(report contextCompressionBenchmarkReport, name string) contextCompressionCandidateReport {
	for _, candidate := range report.Candidates {
		if candidate.Name == name {
			return candidate
		}
	}
	return contextCompressionCandidateReport{Name: name}
}

func TestContextCompressionBenchmark_ComparisonMatrixReportsOpenClawBaseline(t *testing.T) {
	report := buildComparisonReport(t, middleNeedleFixture())
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(report): %v", err)
	}
	t.Log(string(payload))

	borrowedHeadTail := reportCandidate(report, "borrowed_head_tail_baseline")
	openClawLegacy := reportCandidate(report, "openclaw_legacy_compaction_baseline")
	offline := reportCandidate(report, "offline_compression")
	summaryOnly := reportCandidate(report, "small_model_summary_only")
	contextCompress := reportCandidate(report, "small_model_context_compression")

	if offline.KeyFactRecall <= borrowedHeadTail.KeyFactRecall {
		t.Fatalf("offline key_fact_recall = %.2f, want > borrowed head-tail baseline %.2f", offline.KeyFactRecall, borrowedHeadTail.KeyFactRecall)
	}
	if offline.IdentifierFidelity < 0.98 {
		t.Fatalf("offline identifier_fidelity = %.2f, want >= 0.98", offline.IdentifierFidelity)
	}
	if offline.TokenReduction < 0.40 {
		t.Fatalf("offline token_reduction = %.2f, want >= 0.40", offline.TokenReduction)
	}
	if offline.KeyFactRecall < openClawLegacy.KeyFactRecall {
		t.Fatalf("offline key_fact_recall = %.2f, want >= openclaw legacy %.2f", offline.KeyFactRecall, openClawLegacy.KeyFactRecall)
	}
	if offline.IdentifierFidelity < openClawLegacy.IdentifierFidelity {
		t.Fatalf("offline identifier_fidelity = %.2f, want >= openclaw legacy %.2f", offline.IdentifierFidelity, openClawLegacy.IdentifierFidelity)
	}
	if offline.TokenReduction <= openClawLegacy.TokenReduction {
		t.Fatalf("offline token_reduction = %.2f, want > openclaw legacy %.2f", offline.TokenReduction, openClawLegacy.TokenReduction)
	}
	if contextCompress.IdentifierFidelity < summaryOnly.IdentifierFidelity {
		t.Fatalf("small-model context identifier_fidelity = %.2f, want >= summary-only %.2f", contextCompress.IdentifierFidelity, summaryOnly.IdentifierFidelity)
	}
	if contextCompress.TokenReduction < 0.50 {
		t.Fatalf("small-model context token_reduction = %.2f, want >= 0.50", contextCompress.TokenReduction)
	}
	if contextCompress.KeyFactRecall < openClawLegacy.KeyFactRecall {
		t.Fatalf("small-model context key_fact_recall = %.2f, want >= openclaw legacy %.2f", contextCompress.KeyFactRecall, openClawLegacy.KeyFactRecall)
	}
	if contextCompress.IdentifierFidelity < openClawLegacy.IdentifierFidelity {
		t.Fatalf("small-model context identifier_fidelity = %.2f, want >= openclaw legacy %.2f", contextCompress.IdentifierFidelity, openClawLegacy.IdentifierFidelity)
	}
	if contextCompress.TokenReduction <= openClawLegacy.TokenReduction {
		t.Fatalf("small-model context token_reduction = %.2f, want > openclaw legacy %.2f", contextCompress.TokenReduction, openClawLegacy.TokenReduction)
	}
}

func TestContextCompressionBenchmark_OfflineCompressionBeatsOpenClawHeadTailBaseline(t *testing.T) {
	fixture := middleNeedleFixture()
	handler := benchmarkHandlerWithSettings(t, "", false, false, "offline")

	openClawBaseline := renderOpenClawHeadTailBaseline(append(cloneLLMMessages(fixture.OlderMessages), cloneLLMMessages(fixture.RecentMessages)...))
	offline := generateBenchmarkSummary(t, handler, "benchmark-offline-only", fixture)

	openClawMetrics := evaluateCompressionOutput(fixture, openClawBaseline)
	offlineMetrics := evaluateCompressionOutput(fixture, offline)

	t.Logf("openclaw_head_tail metrics: %+v", openClawMetrics)
	t.Logf("offline metrics: %+v", offlineMetrics)

	if offlineMetrics.KeyFactRecall <= openClawMetrics.KeyFactRecall {
		t.Fatalf("offline key_fact_recall = %.2f, want > openclaw baseline %.2f", offlineMetrics.KeyFactRecall, openClawMetrics.KeyFactRecall)
	}
	if offlineMetrics.IdentifierFidelity < 0.98 {
		t.Fatalf("offline identifier_fidelity = %.2f, want >= 0.98", offlineMetrics.IdentifierFidelity)
	}
	if offlineMetrics.TokenReduction < 0.40 {
		t.Fatalf("offline token_reduction = %.2f, want >= 0.40", offlineMetrics.TokenReduction)
	}
}

func TestContextCompressionBenchmark_BlueCompressionBeatsOpenClawLegacyCompactionBaseline(t *testing.T) {
	fixture := middleNeedleFixture()
	report := buildComparisonReport(t, fixture)

	openClawLegacy := reportCandidate(report, "openclaw_legacy_compaction_baseline")
	offline := reportCandidate(report, "offline_compression")
	contextCompress := reportCandidate(report, "small_model_context_compression")

	if offline.KeyFactRecall < openClawLegacy.KeyFactRecall {
		t.Fatalf("offline key_fact_recall = %.2f, want >= openclaw legacy %.2f", offline.KeyFactRecall, openClawLegacy.KeyFactRecall)
	}
	if offline.IdentifierFidelity < openClawLegacy.IdentifierFidelity {
		t.Fatalf("offline identifier_fidelity = %.2f, want >= openclaw legacy %.2f", offline.IdentifierFidelity, openClawLegacy.IdentifierFidelity)
	}
	if offline.TokenReduction <= openClawLegacy.TokenReduction {
		t.Fatalf("offline token_reduction = %.2f, want > openclaw legacy %.2f", offline.TokenReduction, openClawLegacy.TokenReduction)
	}
	if contextCompress.KeyFactRecall < openClawLegacy.KeyFactRecall {
		t.Fatalf("small-model context key_fact_recall = %.2f, want >= openclaw legacy %.2f", contextCompress.KeyFactRecall, openClawLegacy.KeyFactRecall)
	}
	if contextCompress.IdentifierFidelity < openClawLegacy.IdentifierFidelity {
		t.Fatalf("small-model context identifier_fidelity = %.2f, want >= openclaw legacy %.2f", contextCompress.IdentifierFidelity, openClawLegacy.IdentifierFidelity)
	}
	if contextCompress.TokenReduction <= openClawLegacy.TokenReduction {
		t.Fatalf("small-model context token_reduction = %.2f, want > openclaw legacy %.2f", contextCompress.TokenReduction, openClawLegacy.TokenReduction)
	}
}

func TestContextCompressionBenchmark_StaleIntentGateBlocksSideEffects(t *testing.T) {
	metrics := evaluateStaleIntentSafetyMetrics()
	t.Logf(
		"stale-intent metrics: latest_intent_compliance=%.2f stale_intent_execution_rate=%.2f",
		metrics["latest_intent_compliance"],
		metrics["stale_intent_execution_rate"],
	)

	if metrics["latest_intent_compliance"] < 0.98 {
		t.Fatalf("latest_intent_compliance = %.2f, want >= 0.98", metrics["latest_intent_compliance"])
	}
	if metrics["stale_intent_execution_rate"] != 0 {
		t.Fatalf("stale_intent_execution_rate = %.2f, want 0", metrics["stale_intent_execution_rate"])
	}
}

func TestContextCompressionBenchmark_OfflineCompressionUsesMiddleNeedleFixtureDeterministically(t *testing.T) {
	fixture := middleNeedleFixture()
	handler := benchmarkHandlerWithSettings(t, "", false, false, "offline")
	got := generateBenchmarkSummary(t, handler, "benchmark-middle-needle-deterministic", fixture)
	if got == "" {
		t.Fatal("expected non-empty offline compression output")
	}
	if !strings.Contains(strings.ToLower(got), "auth/middleware.go") {
		t.Fatalf("offline compression lost critical file identifier: %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "2026-03-18") {
		t.Fatalf("offline compression lost critical date identifier: %q", got)
	}
}

func TestContextCompressionBenchmark_OfflineCompressionMatchesBenchmarkFixtureInGenerateSummarySync(t *testing.T) {
	fixture := middleNeedleFixture()
	handler := benchmarkHandlerWithSettings(t, "", false, false, "offline")
	got := generateBenchmarkSummary(t, handler, "benchmark-middle-needle", fixture)
	if got == "" {
		t.Fatal("expected generateSummarySync to produce offline summary for benchmark fixture")
	}
	if factRecall(got, fixture.ExpectedIDs) < 0.98 {
		t.Fatalf("generateSummarySync identifier recall = %.2f, want >= 0.98 summary=%q", factRecall(got, fixture.ExpectedIDs), got)
	}
}
