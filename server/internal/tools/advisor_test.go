package tools

import (
	"context"
	"encoding/json"
	"testing"
)

type stubAdvisorBridge struct {
	calls []advisorBridgeCall
}

type advisorBridgeCall struct {
	prompt string
	opts   AdvisorBridgeOptions
}

func (s *stubAdvisorBridge) Chat(_ context.Context, prompt string, maxTokens int, opts AdvisorBridgeOptions) (AdvisorBridgeResponse, error) {
	s.calls = append(s.calls, advisorBridgeCall{prompt: prompt, opts: opts})
	_ = maxTokens
	payload := map[string]interface{}{
		"recommendation": "Prefer Go for high-concurrency services; keep Python for data-heavy workflows.",
		"why": []string{
			"Go has better resource efficiency for the stated traffic profile.",
			"Python remains strong for fast iteration and data-heavy tasks.",
		},
		"best_fit_for": []string{"high-concurrency APIs"},
		"not_fit_for":  []string{"rapid experiment-heavy services"},
		"alternatives": []string{"Keep Python and migrate only hot paths"},
		"tradeoffs":    []string{"Higher migration cost"},
		"risks":        []string{"Training cost"},
		"best_practices": []string{
			"Benchmark before migrating",
		},
		"missing_context": []string{},
		"confidence":      0.82,
		"evidence": []map[string]interface{}{
			{"label": "Official docs", "url": "https://go.dev/", "source": "official", "kind": "documentation", "note": "language reference"},
		},
		"second_opinion": map[string]interface{}{
			"used":      opts.Purpose == advisorPurposeChallenger,
			"reason":    "",
			"summary":   "Challenger broadly agrees.",
			"agreement": "agree",
		},
	}
	encoded, _ := json.Marshal(payload)
	return AdvisorBridgeResponse{
		Content:    string(encoded),
		Provider:   "test",
		ProviderID: "test-primary",
		Model:      "claude-sonnet",
	}, nil
}

func TestNormalizeAdvisorArgs_DefaultsAndAliases(t *testing.T) {
	args := map[string]interface{}{
		"query": "Go vs Python for backend services",
	}

	normalizeAdvisorArgs(args)

	if got := firstCompatString(args, "question"); got != "Go vs Python for backend services" {
		t.Fatalf("question = %q, want promoted query", got)
	}
	if got := firstCompatString(args, "grounding"); got != advisorGroundingAuto {
		t.Fatalf("grounding = %q, want %q", got, advisorGroundingAuto)
	}
	if got := firstCompatString(args, "depth"); got != advisorDepthStandard {
		t.Fatalf("depth = %q, want %q", got, advisorDepthStandard)
	}
	if got := firstCompatString(args, "output"); got != advisorOutputDecisionMemo {
		t.Fatalf("output = %q, want %q", got, advisorOutputDecisionMemo)
	}
	if got := firstCompatString(args, "scorecard_pack"); got != advisorScorecardPackAuto {
		t.Fatalf("scorecard_pack = %q, want %q", got, advisorScorecardPackAuto)
	}
}

func TestShouldAskAdvisorContext_LanguageReplaceMissingTwoSignalGroups(t *testing.T) {
	args := map[string]interface{}{
		"category":      "language",
		"decision_mode": "replace",
	}

	missing, ask := advisorMissingContext(args)
	if !ask {
		t.Fatal("expected ask=true when critical advisor context is missing")
	}
	if len(missing) < 2 {
		t.Fatalf("missing=%v, want at least two missing context cues", missing)
	}
}

func TestShouldGroundAdvisorRequest_AutoForCandidateComparison(t *testing.T) {
	args := map[string]interface{}{
		"question":   "zorm vs gorm vs xorm",
		"grounding":  advisorGroundingAuto,
		"category":   "library",
		"candidates": []interface{}{"zorm", "gorm", "xorm"},
	}

	if !shouldGroundAdvisor(args) {
		t.Fatal("expected grounding for named library comparison")
	}
}

func TestShouldUseAdvisorSecondOpinion_CompareMode(t *testing.T) {
	args := map[string]interface{}{
		"decision_mode": "compare",
	}
	if !shouldRunAdvisorSecondOpinion(args, 0.9, false) {
		t.Fatal("expected second opinion for compare mode")
	}
}

func TestSelectAdvisorPrimaryAndChallenger_PrefersPremiumAndDifferentProvider(t *testing.T) {
	candidates := []advisorModelCandidate{
		{ProviderID: "fast-1", ProviderName: "fast", ModelID: "claude-haiku"},
		{ProviderID: "balanced-1", ProviderName: "balanced", ModelID: "gpt-4.1"},
		{ProviderID: "premium-1", ProviderName: "premium", ModelID: "claude-sonnet-4"},
		{ProviderID: "premium-2", ProviderName: "premium-2", ModelID: "gpt-5"},
	}

	primary, ok := selectAdvisorPrimaryCandidate(candidates)
	if !ok {
		t.Fatal("expected primary candidate")
	}
	if primary.ModelID != "claude-sonnet-4" && primary.ModelID != "gpt-5" {
		t.Fatalf("primary=%+v, want premium tier selection", primary)
	}

	challenger, ok := selectAdvisorChallengerCandidate(candidates, primary)
	if !ok {
		t.Fatal("expected challenger candidate")
	}
	if challenger.ProviderID == primary.ProviderID {
		t.Fatalf("challenger provider=%q, want different from primary=%q", challenger.ProviderID, primary.ProviderID)
	}
}

func TestRegisterAdvisorToolAndGetAdvisorTool(t *testing.T) {
	registry := NewRegistry()

	tool := RegisterAdvisorTool(registry)
	if tool == nil {
		t.Fatal("expected advisor tool")
	}

	got := GetAdvisorTool(registry)
	if got == nil {
		t.Fatal("expected advisor tool from registry")
	}
	if got.Definition().Name != "advisor" {
		t.Fatalf("tool name = %q, want advisor", got.Definition().Name)
	}
}

func TestAdvisorExecuteReturnsDecisionMemoContract(t *testing.T) {
	tool := NewAdvisorTool()
	tool.SetBridge(&stubAdvisorBridge{})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"question":      "Go vs Python for backend services",
		"category":      "language",
		"decision_mode": "recommend",
		"context": map[string]interface{}{
			"current_solution": "python",
			"team_size":        "6",
			"data_scale":       "medium",
			"deployment":       "k8s",
		},
		"grounding": "none",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want JSON string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	for _, key := range []string{
		"recommendation",
		"why",
		"best_fit_for",
		"not_fit_for",
		"alternatives",
		"tradeoffs",
		"risks",
		"best_practices",
		"missing_context",
		"confidence",
		"evidence",
		"second_opinion",
	} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %q in advisor result: %v", key, payload)
		}
	}
}

func TestAdvisorExecuteReturnsScorecardContract(t *testing.T) {
	tool := NewAdvisorTool()
	tool.SetBridge(&stubAdvisorBridge{})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"question":      "Go vs Python vs Node for backend services",
		"category":      "language",
		"decision_mode": "compare",
		"candidates":    []interface{}{"Go", "Python", "Node"},
		"context": map[string]interface{}{
			"current_solution": "python",
			"team_size":        "6",
			"data_scale":       "medium",
			"deployment":       "k8s",
		},
		"grounding": "none",
		"output":    "scorecard",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want JSON string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	for _, key := range []string{
		"pack_id",
		"winner",
		"summary",
		"weights",
		"candidates",
		"sensitivity",
		"missing_context",
		"confidence",
		"evidence",
		"second_opinion",
	} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %q in scorecard result: %v", key, payload)
		}
	}
	if got := payload["pack_id"]; got != advisorScorecardPackSolutionSelectionV1 {
		t.Fatalf("pack_id = %v, want %q", got, advisorScorecardPackSolutionSelectionV1)
	}
}

func TestAdvisorExecuteReturnsDecisionPackContract(t *testing.T) {
	tool := NewAdvisorTool()
	tool.SetBridge(&stubAdvisorBridge{})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"question":      "Should we replace Python with Go?",
		"category":      "migration",
		"decision_mode": "replace",
		"candidates":    []interface{}{"Go", "Python"},
		"context": map[string]interface{}{
			"current_solution": "python",
			"team_size":        "6",
			"data_scale":       "medium",
			"deployment":       "k8s",
		},
		"grounding": "none",
		"output":    "decision_pack",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want JSON string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	for _, key := range []string{"memo", "scorecard", "decision_meta"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing key %q in decision pack: %v", key, payload)
		}
	}
}

func TestAdvisorExecuteRejectsUnknownScorecardWeight(t *testing.T) {
	tool := NewAdvisorTool()
	tool.SetBridge(&stubAdvisorBridge{})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"question":          "Go vs Python",
		"category":          "language",
		"decision_mode":     "compare",
		"output":            "scorecard",
		"scorecard_pack":    advisorScorecardPackSolutionSelectionV1,
		"scorecard_weights": map[string]interface{}{"made_up_criterion": 0.5},
	})
	if err == nil {
		t.Fatal("expected invalid scorecard weight error")
	}
}

func TestAdvisorExecuteDeepRunUsesResearchService(t *testing.T) {
	service := &mockResearchService{
		create: func(ctx context.Context, req ResearchCreateJobRequest) (*ResearchJob, error) {
			if req.Mode != "advisor" {
				t.Fatalf("mode = %q, want advisor", req.Mode)
			}
			if req.Depth != advisorDepthDeep {
				t.Fatalf("depth = %q, want deep", req.Depth)
			}
			if req.Output != advisorOutputDecisionPack {
				t.Fatalf("output = %q, want %q", req.Output, advisorOutputDecisionPack)
			}
			if req.ScorecardPack != advisorScorecardPackSolutionSelectionV1 {
				t.Fatalf("scorecard pack = %q, want %q", req.ScorecardPack, advisorScorecardPackSolutionSelectionV1)
			}
			return &ResearchJob{ID: "advisor-job-1", Status: "pending", Query: req.Query, Mode: req.Mode, ResearchDepth: req.Depth}, nil
		},
	}
	tool := NewAdvisorTool()
	tool.SetBridge(&stubAdvisorBridge{})
	tool.SetResearchService(service)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"question":       "Go vs Python vs Node",
		"category":       "language",
		"decision_mode":  "compare",
		"candidates":     []interface{}{"Go", "Python", "Node"},
		"depth":          "deep",
		"output":         "decision_pack",
		"scorecard_pack": advisorScorecardPackSolutionSelectionV1,
		"wait":           false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T, want map envelope", result)
	}
	if got := payload["accepted"]; got != true {
		t.Fatalf("accepted = %v, want true", got)
	}
	if got := payload["terminal"]; got != false {
		t.Fatalf("terminal = %v, want false", got)
	}
	if got := payload["mode"]; got != "advisor" {
		t.Fatalf("mode = %v, want advisor", got)
	}
}

func TestAdvisorExecuteStatusReturnsCompletedDecisionPack(t *testing.T) {
	service := &mockResearchService{
		get: func(id, userID string) (*ResearchJob, error) {
			return &ResearchJob{
				ID:            id,
				Status:        "completed",
				Query:         "Go vs Python",
				Mode:          "advisor",
				ResearchDepth: "deep",
				Answer:        `{"memo":{"recommendation":"Prefer Go"},"scorecard":{"pack_id":"solution_selection_v1","winner":"Go","summary":"Go wins","weights":[],"candidates":[],"sensitivity":{"close_call":false,"margin":12,"top_driver_criteria":["fitness"],"flip_risk":"low"},"missing_context":[],"confidence":0.84,"evidence":[],"second_opinion":{"used":false}},"decision_meta":{"version":"advisor.v2","execution_mode":"async","grounding_used":false,"pack_id":"solution_selection_v1","evidence_conflict":false,"calibration":{"groundedness":0.7,"freshness":0.6,"conflict_risk":"low"}}}`,
			}, nil
		},
	}
	tool := NewAdvisorTool()
	tool.SetBridge(&stubAdvisorBridge{})
	tool.SetResearchService(service)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "status",
		"job_id": "advisor-job-2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want JSON string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if _, ok := payload["job"]; !ok {
		t.Fatalf("expected job metadata in async completion payload: %v", payload)
	}
	if _, ok := payload["decision_meta"]; !ok {
		t.Fatalf("expected decision_meta in async completion payload: %v", payload)
	}
}

func TestAdvisorExecuteRequiresQuestion(t *testing.T) {
	tool := NewAdvisorTool()
	tool.SetBridge(&stubAdvisorBridge{})

	if _, err := tool.Execute(context.Background(), map[string]interface{}{}); err == nil {
		t.Fatal("expected missing question error")
	}
}
