package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

type contextCompressionLLMJudgeCandidate struct {
	Name                  string  `json:"name"`
	LatestIntentAlignment float64 `json:"latest_intent_alignment"`
	FactPreservation      float64 `json:"fact_preservation"`
	IdentifierFidelity    float64 `json:"identifier_fidelity"`
	StaleIntentSafety     float64 `json:"stale_intent_safety"`
	Conciseness           float64 `json:"conciseness"`
	Overall               float64 `json:"overall"`
	Reason                string  `json:"reason"`
}

type contextCompressionLLMJudgeReport struct {
	Fixture        string                                `json:"fixture"`
	JudgeModel     string                                `json:"judge_model"`
	BestCandidate  string                                `json:"best_candidate"`
	WorstCandidate string                                `json:"worst_candidate"`
	Summary        string                                `json:"summary"`
	Candidates     []contextCompressionLLMJudgeCandidate `json:"candidates"`
	Notes          []string                              `json:"notes,omitempty"`
}

func TestContextCompressionBenchmark_LLMJudgeComparisonOptional(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ZIMA_RUN_CONTEXT_COMPRESSION_LLM_JUDGE")) != "1" {
		t.Skip("set ZIMA_RUN_CONTEXT_COMPRESSION_LLM_JUDGE=1 to run the optional live LLM judge benchmark")
	}

	baseURL := strings.TrimSpace(os.Getenv("ZIMA_CONTEXT_COMPRESSION_JUDGE_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("ZIMA_CONTEXT_COMPRESSION_JUDGE_API_KEY"))
	modelID := strings.TrimSpace(os.Getenv("ZIMA_CONTEXT_COMPRESSION_JUDGE_MODEL"))
	if baseURL == "" || apiKey == "" || modelID == "" {
		t.Skip("missing ZIMA_CONTEXT_COMPRESSION_JUDGE_BASE_URL or ZIMA_CONTEXT_COMPRESSION_JUDGE_API_KEY or ZIMA_CONTEXT_COMPRESSION_JUDGE_MODEL")
	}

	fixture := middleNeedleFixture()
	candidates := buildComparisonCandidates(t, fixture)
	report, err := runContextCompressionLLMJudge(context.Background(), baseURL, apiKey, modelID, fixture, candidates)
	if err != nil {
		t.Fatalf("runContextCompressionLLMJudge: %v", err)
	}
	if len(report.Candidates) != len(candidates) {
		t.Fatalf("judge candidate count = %d, want %d", len(report.Candidates), len(candidates))
	}
	if strings.TrimSpace(report.BestCandidate) == "" || strings.TrimSpace(report.WorstCandidate) == "" {
		t.Fatalf("judge best/worst candidate missing: %+v", report)
	}

	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(report): %v", err)
	}
	t.Log(string(payload))
}

func runContextCompressionLLMJudge(
	ctx context.Context,
	baseURL, apiKey, modelID string,
	fixture contextCompressionEvalFixture,
	candidates []contextCompressionCandidateOutput,
) (contextCompressionLLMJudgeReport, error) {
	provider := llm.NewOpenAIProvider(apiKey, baseURL)
	judgePrompt, err := buildContextCompressionJudgePrompt(fixture, candidates)
	if err != nil {
		return contextCompressionLLMJudgeReport{}, err
	}

	judgeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	resp, err := provider.Chat(judgeCtx, llm.ChatRequest{
		Model: modelID,
		Messages: []llm.Message{
			{
				Role: llm.RoleSystem,
				Content: strings.TrimSpace(`You are a strict evaluator for context-compression outputs.

Return only one JSON object and nothing else.
Do not use markdown fences.
Do not omit any candidate.
The latest user message always has priority over historical context.`),
			},
			{
				Role:    llm.RoleUser,
				Content: judgePrompt,
			},
		},
		Temperature: 0,
		MaxTokens:   1600,
	})
	if err != nil {
		return contextCompressionLLMJudgeReport{}, err
	}

	var report contextCompressionLLMJudgeReport
	payload := extractJSONObject(resp.Message.Content)
	if err := json.Unmarshal([]byte(payload), &report); err != nil {
		return contextCompressionLLMJudgeReport{}, fmt.Errorf("parse judge response: %w raw=%q", err, truncateRunes(resp.Message.Content, 600))
	}
	report.Fixture = fixture.Name
	report.JudgeModel = modelID
	report.Notes = []string{
		"Live LLM judge is advisory only and is not part of the deterministic CI gate.",
		"Deterministic stale-intent safety still comes from latest_intent_compliance and stale_intent_execution_rate tests.",
	}
	return report, nil
}

func buildContextCompressionJudgePrompt(
	fixture contextCompressionEvalFixture,
	candidates []contextCompressionCandidateOutput,
) (string, error) {
	type candidatePayload struct {
		Name   string `json:"name"`
		Output string `json:"output"`
	}
	payload := struct {
		Fixture struct {
			Name           string        `json:"name"`
			LatestUser     string        `json:"latest_user"`
			ExpectedFacts  []string      `json:"expected_facts"`
			ExpectedIDs    []string      `json:"expected_ids"`
			OlderMessages  []llm.Message `json:"older_messages"`
			RecentMessages []llm.Message `json:"recent_messages"`
		} `json:"fixture"`
		Candidates []candidatePayload `json:"candidates"`
	}{}
	payload.Fixture.Name = fixture.Name
	payload.Fixture.LatestUser = fixture.LatestUser
	payload.Fixture.ExpectedFacts = append([]string(nil), fixture.ExpectedFacts...)
	payload.Fixture.ExpectedIDs = append([]string(nil), fixture.ExpectedIDs...)
	payload.Fixture.OlderMessages = cloneLLMMessages(fixture.OlderMessages)
	payload.Fixture.RecentMessages = cloneLLMMessages(fixture.RecentMessages)
	payload.Candidates = make([]candidatePayload, 0, len(candidates))
	for _, candidate := range candidates {
		payload.Candidates = append(payload.Candidates, candidatePayload{
			Name:   candidate.Name,
			Output: candidate.Output,
		})
	}

	blob, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal judge payload: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(`Evaluate the candidate outputs for a context-compression benchmark.

Scoring rules:
- latest_intent_alignment: does the candidate directly support the latest user request instead of replaying history.
- fact_preservation: does it preserve the important middle-history facts.
- identifier_fidelity: does it preserve exact identifiers like dates, paths, and request IDs.
- stale_intent_safety: does it keep historical pending work as background instead of sounding like an active current-turn command.
- conciseness: does it avoid replaying unnecessary process chatter.
- overall: holistic quality for downstream use.

Important:
- The latest user request is the only current-turn instruction.
- Historical summaries are background only.
- Do not reward transcript dumps just because they contain more tokens.
- A candidate can score high only if it preserves the key facts and identifiers while staying concise.

Return only JSON in exactly this shape:
{
  "best_candidate": "candidate_name",
  "worst_candidate": "candidate_name",
  "summary": "one short paragraph",
  "candidates": [
    {
      "name": "candidate_name",
      "latest_intent_alignment": 0.0,
      "fact_preservation": 0.0,
      "identifier_fidelity": 0.0,
      "stale_intent_safety": 0.0,
      "conciseness": 0.0,
      "overall": 0.0,
      "reason": "short explanation"
    }
  ]
}

Benchmark payload:`))
	sb.WriteString("\n")
	sb.Write(blob)
	return sb.String(), nil
}

func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return trimmed[start : end+1]
	}
	return trimmed
}
