package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
)

const (
	advisorActionRun    = "run"
	advisorActionStatus = "status"

	advisorGroundingAuto = "auto"
	advisorGroundingNone = "none"
	advisorGroundingWeb  = "web"

	advisorDepthQuick    = "quick"
	advisorDepthStandard = "standard"
	advisorDepthDeep     = "deep"

	advisorOutputDecisionMemo = "decision_memo"
	advisorOutputScorecard    = "scorecard"
	advisorOutputDecisionPack = "decision_pack"

	advisorScorecardPackAuto                = "auto"
	advisorScorecardPackSolutionSelectionV1 = "solution_selection_v1"
	advisorScorecardPackMigrationV1         = "migration_v1"
	advisorScorecardPackArchitectureV1      = "architecture_v1"
	advisorScorecardPackProcessV1           = "process_v1"

	advisorPurposePrimary    = "primary"
	advisorPurposeChallenger = "challenger"

	advisorMaxPromptTokens = 2400
)

var advisorContextKeys = []string{
	"stack",
	"team_size",
	"data_scale",
	"traffic_qps",
	"latency_slo",
	"deployment",
	"budget_level",
	"compliance",
	"current_solution",
	"must_have",
	"must_avoid",
}

var advisorRecencyTerms = []string{
	"latest", "current", "sota", "benchmark", "benchmarks", "maintained", "recommended",
	"最新", "当前", "现状", "基准", "维护", "推荐",
}

var advisorComparisonTerms = []string{
	" vs ", " versus ", "compare", "comparison", "replace", "replacement", "migration", "migrate", "tradeoff",
	"对比", "比较", "替代", "替换", "迁移", "取舍", "权衡",
}

// AdvisorBridge adds advisor-specific model-routing options on top of text chat.
type AdvisorBridge interface {
	Chat(ctx context.Context, prompt string, maxTokens int, opts AdvisorBridgeOptions) (AdvisorBridgeResponse, error)
}

// AdvisorBridgeOptions controls advisor-specific routing intent.
type AdvisorBridgeOptions struct {
	ProviderID   string
	ProviderName string
	Model        string
	Purpose      string
}

// AdvisorBridgeResponse carries response content plus locally resolved route metadata.
type AdvisorBridgeResponse struct {
	Content    string
	Provider   string
	ProviderID string
	Model      string
}

type advisorEvidenceItem struct {
	ID          string `json:"id,omitempty"`
	Label       string `json:"label"`
	URL         string `json:"url"`
	Source      string `json:"source"`
	Kind        string `json:"kind"`
	Note        string `json:"note"`
	Quality     string `json:"quality,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Domain      string `json:"domain,omitempty"`
	Candidate   string `json:"candidate,omitempty"`
	Criterion   string `json:"criterion,omitempty"`
	Stance      string `json:"stance,omitempty"`
}

type advisorSecondOpinion struct {
	Used       bool   `json:"used"`
	Reason     string `json:"reason"`
	Summary    string `json:"summary"`
	Agreement  string `json:"agreement"`
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id"`
	Model      string `json:"model"`
}

type advisorDecisionMemo struct {
	Recommendation string                `json:"recommendation"`
	Why            []string              `json:"why"`
	BestFitFor     []string              `json:"best_fit_for"`
	NotFitFor      []string              `json:"not_fit_for"`
	Alternatives   []string              `json:"alternatives"`
	Tradeoffs      []string              `json:"tradeoffs"`
	Risks          []string              `json:"risks"`
	BestPractices  []string              `json:"best_practices"`
	MissingContext []string              `json:"missing_context"`
	Confidence     float64               `json:"confidence"`
	Evidence       []advisorEvidenceItem `json:"evidence"`
	SecondOpinion  advisorSecondOpinion  `json:"second_opinion"`
}

type advisorWeight struct {
	Criterion string  `json:"criterion"`
	Label     string  `json:"label"`
	Weight    float64 `json:"weight"`
	Source    string  `json:"source"`
}

type advisorCriterionScore struct {
	Criterion   string   `json:"criterion"`
	Score       float64  `json:"score"`
	Reason      string   `json:"reason"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type advisorCandidateScore struct {
	Name            string                  `json:"name"`
	Rank            int                     `json:"rank"`
	TotalScore      float64                 `json:"total_score"`
	Verdict         string                  `json:"verdict"`
	Strengths       []string                `json:"strengths"`
	Concerns        []string                `json:"concerns"`
	BestFitFor      []string                `json:"best_fit_for"`
	CriterionScores []advisorCriterionScore `json:"criterion_scores"`
}

type advisorSensitivity struct {
	CloseCall         bool     `json:"close_call"`
	Margin            float64  `json:"margin"`
	TopDriverCriteria []string `json:"top_driver_criteria"`
	FlipRisk          string   `json:"flip_risk"`
}

type advisorCalibration struct {
	Groundedness float64 `json:"groundedness"`
	Freshness    float64 `json:"freshness"`
	ConflictRisk string  `json:"conflict_risk"`
}

type advisorDecisionMeta struct {
	Version          string             `json:"version"`
	ExecutionMode    string             `json:"execution_mode"`
	GroundingUsed    bool               `json:"grounding_used"`
	PackID           string             `json:"pack_id"`
	EvidenceConflict bool               `json:"evidence_conflict"`
	Calibration      advisorCalibration `json:"calibration"`
}

type advisorScorecard struct {
	PackID         string                  `json:"pack_id"`
	Winner         string                  `json:"winner"`
	Summary        string                  `json:"summary"`
	Weights        []advisorWeight         `json:"weights"`
	Candidates     []advisorCandidateScore `json:"candidates"`
	Sensitivity    advisorSensitivity      `json:"sensitivity"`
	MissingContext []string                `json:"missing_context"`
	Confidence     float64                 `json:"confidence"`
	Evidence       []advisorEvidenceItem   `json:"evidence"`
	SecondOpinion  advisorSecondOpinion    `json:"second_opinion"`
}

type advisorModelCandidate struct {
	ProviderID   string
	ProviderName string
	ModelID      string
}

type advisorPromptEnvelope struct {
	Action           string                 `json:"action,omitempty"`
	Question         string                 `json:"question"`
	Category         string                 `json:"category"`
	DecisionMode     string                 `json:"decision_mode"`
	Candidates       []string               `json:"candidates"`
	Context          map[string]interface{} `json:"context"`
	Constraints      map[string]interface{} `json:"constraints"`
	Grounding        string                 `json:"grounding"`
	Depth            string                 `json:"depth"`
	Output           string                 `json:"output"`
	ScorecardPack    string                 `json:"scorecard_pack,omitempty"`
	ScorecardWeights map[string]float64     `json:"scorecard_weights,omitempty"`
	Lang             string                 `json:"lang"`
	MissingContext   []string               `json:"missing_context"`
	Evidence         []advisorEvidenceItem  `json:"evidence"`
}

type advisorLLMEnvelope struct {
	Recommendation string                   `json:"recommendation"`
	Why            []string                 `json:"why"`
	BestFitFor     []string                 `json:"best_fit_for"`
	NotFitFor      []string                 `json:"not_fit_for"`
	Alternatives   []string                 `json:"alternatives"`
	Tradeoffs      []string                 `json:"tradeoffs"`
	Risks          []string                 `json:"risks"`
	BestPractices  []string                 `json:"best_practices"`
	MissingContext []string                 `json:"missing_context"`
	Confidence     float64                  `json:"confidence"`
	Evidence       []map[string]interface{} `json:"evidence"`
}

// AdvisorTool returns decision-oriented technology and practice recommendations.
type AdvisorTool struct {
	mu       sync.RWMutex
	bridge   AdvisorBridge
	executor *Executor
	research ResearchService
}

// NewAdvisorTool creates a new advisor tool.
func NewAdvisorTool() *AdvisorTool {
	return &AdvisorTool{}
}

// SetBridge injects the advisor bridge.
func (t *AdvisorTool) SetBridge(bridge AdvisorBridge) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.bridge = bridge
}

// SetExecutor injects the tool executor for ask/web grounding.
func (t *AdvisorTool) SetExecutor(executor *Executor) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.executor = executor
}

// SetResearchService injects the async research runtime used for deep advisor jobs.
func (t *AdvisorTool) SetResearchService(service ResearchService) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.research = service
}

// Definition returns the advisor tool schema.
func (t *AdvisorTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "advisor",
		Description: "Decision advisor for technology selection, replacements, migrations, and best practices. Supports decision memos, weighted scorecards, and async deep advisor runs.",
		Icon:        "advisor",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{
					"type":        "string",
					"enum":        []string{advisorActionRun, advisorActionStatus},
					"description": "run to execute advisor, status to poll an existing deep advisor job.",
				},
				"question": map[string]interface{}{
					"type":        "string",
					"description": "The decision question to answer.",
				},
				"job_id": map[string]interface{}{
					"type":        "string",
					"description": "Async advisor job ID used with action=status.",
				},
				"id": map[string]interface{}{
					"type":        "string",
					"description": "Alias for job_id.",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"architecture", "language", "framework", "library", "process", "migration", "ops"},
					"description": "Optional decision category.",
				},
				"decision_mode": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"recommend", "compare", "review", "replace", "best_practice"},
					"description": "Decision style: recommend, compare, review, replace, or best_practice.",
				},
				"candidates": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Optional candidate technologies or approaches.",
				},
				"context": map[string]interface{}{
					"type":        "object",
					"description": "Optional runtime/team context such as stack, team_size, data_scale, deployment, and current_solution.",
				},
				"constraints": map[string]interface{}{
					"type":        "object",
					"description": "Optional hard constraints such as compliance, budget, latency, must_have, and must_avoid.",
				},
				"grounding": map[string]interface{}{
					"type":        "string",
					"enum":        []string{advisorGroundingAuto, advisorGroundingNone, advisorGroundingWeb},
					"description": "Grounding policy: auto (default), none, or web.",
				},
				"depth": map[string]interface{}{
					"type":        "string",
					"enum":        []string{advisorDepthQuick, advisorDepthStandard, advisorDepthDeep},
					"description": "Reasoning depth: quick, standard (default), or deep.",
				},
				"output": map[string]interface{}{
					"type":        "string",
					"enum":        []string{advisorOutputDecisionMemo, advisorOutputScorecard, advisorOutputDecisionPack},
					"description": "Output format: decision_memo, scorecard, or decision_pack.",
				},
				"scorecard_pack": map[string]interface{}{
					"type":        "string",
					"enum":        []string{advisorScorecardPackAuto, advisorScorecardPackSolutionSelectionV1, advisorScorecardPackMigrationV1, advisorScorecardPackArchitectureV1, advisorScorecardPackProcessV1},
					"description": "Optional scorecard criteria pack.",
				},
				"scorecard_weights": map[string]interface{}{
					"type":        "object",
					"description": "Optional criterion weight overrides for the selected scorecard pack.",
				},
				"lang": map[string]interface{}{
					"type":        "string",
					"description": "Preferred output language.",
				},
				"wait": map[string]interface{}{
					"type":        "boolean",
					"description": "When depth=deep, wait for completion before returning. Defaults to true.",
				},
				"wait_timeout_seconds": map[string]interface{}{
					"type":        "integer",
					"description": "Optional max wait time for deep advisor runs.",
				},
				"poll_interval_ms": map[string]interface{}{
					"type":        "integer",
					"description": "Polling interval for deep advisor runs. Defaults to 500ms.",
				},
			},
			"anyOf": []interface{}{
				map[string]interface{}{"required": []string{"question"}},
				map[string]interface{}{"required": []string{"job_id"}},
				map[string]interface{}{"required": []string{"id"}},
			},
		},
	}
}

// Execute runs the advisor decision pipeline.
func (t *AdvisorTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if args == nil {
		args = map[string]interface{}{}
	}
	normalizeAdvisorArgs(args)
	action := normalizeAdvisorAction(firstCompatString(args, "action"))
	if action == advisorActionStatus {
		return t.executeAdvisorStatus(ctx, args)
	}

	question := strings.TrimSpace(firstCompatString(args, "question"))
	if question == "" {
		return nil, errors.New("question is required")
	}
	if lang := strings.TrimSpace(firstCompatString(args, "lang", "language")); lang == "" {
		args["lang"] = GetLang(ctx)
	}
	if shouldRunAdvisorAsync(args) {
		return t.executeAdvisorAsync(ctx, args)
	}
	return t.executeAdvisorSync(ctx, args)
}

func (t *AdvisorTool) executeAdvisorSync(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.RLock()
	bridge := t.bridge
	executor := t.executor
	t.mu.RUnlock()
	if bridge == nil {
		return nil, errors.New("advisor bridge not available")
	}

	missingContext, _ := advisorMissingContext(args)
	if len(missingContext) > 0 {
		updatedArgs, updatedMissing, err := advisorAskMissingContext(ctx, args, executor, missingContext)
		if err == nil {
			args = updatedArgs
			missingContext = updatedMissing
		}
	}

	evidence, groundingNote := collectAdvisorEvidence(ctx, args, executor)
	evidence = ensureAdvisorEvidenceItems(evidence)
	evidenceConflict := false
	if len(evidence) > 1 {
		evidenceConflict = advisorEvidenceConflict(evidence)
	}

	primaryCandidate, _ := selectAdvisorPrimaryCandidate(advisorAvailableCandidates(bridge))
	preferSecondOpinion := shouldRunAdvisorSecondOpinion(args, 1, evidenceConflict)

	primaryMemo, primaryMeta, primaryErr := t.runAdvisorCall(
		ctx,
		bridge,
		args,
		missingContext,
		evidence,
		primaryCandidate,
		advisorPurposePrimary,
	)
	if primaryErr != nil {
		return nil, primaryErr
	}

	finalMemo := primaryMemo
	finalMemo.MissingContext = uniqueStrings(append(finalMemo.MissingContext, missingContext...))
	if groundingNote != "" && len(finalMemo.Evidence) == 0 {
		finalMemo.MissingContext = uniqueStrings(append(finalMemo.MissingContext, groundingNote))
	}

	if preferSecondOpinion || shouldRunAdvisorSecondOpinion(args, finalMemo.Confidence, evidenceConflict) {
		challengerCandidate, ok := selectAdvisorChallengerCandidate(advisorAvailableCandidates(bridge), primaryCandidate)
		if ok {
			challengerMemo, challengerMeta, challengerErr := t.runAdvisorCall(
				ctx,
				bridge,
				args,
				missingContext,
				evidence,
				challengerCandidate,
				advisorPurposeChallenger,
			)
			if challengerErr == nil {
				finalMemo.SecondOpinion = mergeAdvisorSecondOpinion(primaryMeta, challengerMeta, finalMemo, challengerMemo)
			} else {
				finalMemo.SecondOpinion = advisorSecondOpinion{
					Used:   false,
					Reason: challengerErr.Error(),
				}
			}
		} else {
			finalMemo.SecondOpinion = advisorSecondOpinion{
				Used:   false,
				Reason: "no challenger candidate available",
			}
		}
	}

	finalMemo = ensureAdvisorDecisionMemo(finalMemo)
	finalMemo.Evidence = ensureAdvisorEvidenceItems(finalMemo.Evidence)

	packDef, weights, err := advisorResolveScorecardWeights(args)
	if err != nil {
		return nil, err
	}
	scorecard := buildAdvisorScorecard(args, packDef, weights, finalMemo, evidenceConflict)
	meta := buildAdvisorDecisionMeta(args, "sync", len(finalMemo.Evidence) > 0, evidenceConflict, scorecard.PackID, nil, finalMemo)
	return encodeAdvisorResult(args, finalMemo, scorecard, meta, nil)
}

func (t *AdvisorTool) executeAdvisorAsync(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.RLock()
	service := t.research
	t.mu.RUnlock()
	if service == nil {
		return t.executeAdvisorSync(ctx, args)
	}

	req, err := advisorBuildResearchCreateJobRequest(ctx, args)
	if err != nil {
		return nil, err
	}
	job, err := service.CreateJob(ctx, req)
	if err != nil {
		return nil, err
	}

	wait := true
	if parsed := parseResearchBoolArg(args, "wait"); parsed != nil {
		wait = *parsed
	}
	if !wait {
		payload := advisorJobEnvelope(job, true)
		payload["mode"] = "advisor"
		return payload, nil
	}

	finalJob, err := advisorWaitForResearchJob(ctx, service, job.ID, args)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			payload := advisorJobEnvelope(job, true)
			payload["terminal"] = false
			return payload, nil
		}
		return nil, err
	}
	return advisorCompletedPayloadFromJob(finalJob)
}

func (t *AdvisorTool) executeAdvisorStatus(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	t.mu.RLock()
	service := t.research
	t.mu.RUnlock()
	if service == nil {
		return nil, errors.New("advisor research runtime not available")
	}
	jobID := strings.TrimSpace(firstCompatString(args, "job_id", "jobId", "id"))
	if jobID == "" {
		return nil, errors.New("job_id is required")
	}
	job, err := service.GetJobForUser(jobID, GetUserID(ctx))
	if err != nil {
		return nil, err
	}
	if !isResearchTerminalStatus(job.Status) {
		return advisorJobEnvelope(job, false), nil
	}
	return advisorCompletedPayloadFromJob(job)
}

func advisorWaitForResearchJob(ctx context.Context, service ResearchService, jobID string, args map[string]interface{}) (*ResearchJob, error) {
	pollInterval := 500
	if raw, ok := compatArgValue(args, "poll_interval_ms", "pollIntervalMs"); ok {
		if ms := parseResearchIntArg(raw); ms > 0 {
			pollInterval = ms
		}
	}
	waitCtx := ctx
	cancel := func() {}
	if raw, ok := compatArgValue(args, "wait_timeout_seconds", "waitTimeoutSeconds"); ok {
		if seconds := parseResearchIntArg(raw); seconds > 0 {
			waitCtx, cancel = context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
		}
	}
	defer cancel()
	ticker := time.NewTicker(time.Duration(pollInterval) * time.Millisecond)
	defer ticker.Stop()
	for {
		job, err := service.GetJobForUser(jobID, GetUserID(ctx))
		if err != nil {
			return nil, err
		}
		if isResearchTerminalStatus(job.Status) {
			return job, nil
		}
		select {
		case <-waitCtx.Done():
			return nil, waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func advisorBuildResearchCreateJobRequest(ctx context.Context, args map[string]interface{}) (ResearchCreateJobRequest, error) {
	req, err := buildResearchCreateJobRequest(ctx, map[string]interface{}{
		"mode":              "advisor",
		"question":          firstCompatString(args, "question"),
		"category":          firstCompatString(args, "category"),
		"decision_mode":     firstCompatString(args, "decision_mode"),
		"candidates":        args["candidates"],
		"context":           args["context"],
		"constraints":       args["constraints"],
		"grounding":         firstCompatString(args, "grounding"),
		"depth":             firstCompatString(args, "depth"),
		"output":            firstCompatString(args, "output"),
		"scorecard_pack":    firstCompatString(args, "scorecard_pack"),
		"scorecard_weights": args["scorecard_weights"],
		"lang":              firstCompatString(args, "lang", "language"),
	})
	if err != nil {
		return ResearchCreateJobRequest{}, err
	}
	req.Mode = "advisor"
	req.Depth = strings.TrimSpace(firstCompatString(args, "depth"))
	req.Output = strings.TrimSpace(firstCompatString(args, "output"))
	req.ScorecardPack = strings.TrimSpace(firstCompatString(args, "scorecard_pack"))
	req.ScorecardWeights = advisorParseWeightMap(args["scorecard_weights"])
	req.UserID = GetUserID(ctx)
	req.ConversationID = strings.TrimSpace(GetSessionID(ctx))
	return req, nil
}

func advisorCompletedPayloadFromJob(job *ResearchJob) (interface{}, error) {
	if job == nil {
		return nil, errors.New("advisor job not found")
	}
	raw := strings.TrimSpace(job.Answer)
	if raw == "" && len(job.Report) > 0 {
		raw = strings.TrimSpace(formatValue(job.Report["answer"]))
	}
	if raw == "" {
		payload := advisorJobEnvelope(job, false)
		payload["terminal"] = true
		return payload, nil
	}
	decoded := map[string]interface{}{}
	if json.Unmarshal([]byte(raw), &decoded) != nil {
		return raw, nil
	}
	decoded["job"] = advisorJobEnvelope(job, false)
	if pack, ok := decoded["scorecard"].(map[string]interface{}); ok {
		if meta, ok := decoded["decision_meta"].(map[string]interface{}); ok {
			meta["execution_mode"] = "async"
			if meta["pack_id"] == nil || strings.TrimSpace(formatValue(meta["pack_id"])) == "" {
				meta["pack_id"] = strings.TrimSpace(formatValue(pack["pack_id"]))
			}
			if calibration, ok := job.Report["calibration"].(map[string]interface{}); ok {
				meta["calibration"] = calibration
			}
			decoded["decision_meta"] = meta
		}
	}
	encoded, err := json.Marshal(decoded)
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

func advisorJobEnvelope(job *ResearchJob, accepted bool) map[string]interface{} {
	payload := map[string]interface{}{
		"job_id":   "",
		"status":   "",
		"progress": 0,
		"mode":     "advisor",
		"terminal": false,
	}
	if accepted {
		payload["accepted"] = true
	}
	if job == nil {
		return payload
	}
	payload["job_id"] = job.ID
	payload["status"] = job.Status
	payload["progress"] = job.Progress
	if strings.TrimSpace(job.Mode) != "" {
		payload["mode"] = job.Mode
	}
	payload["terminal"] = isResearchTerminalStatus(job.Status)
	if strings.TrimSpace(job.Query) != "" {
		payload["query"] = job.Query
	}
	return payload
}

type advisorCallMeta struct {
	Provider   string
	ProviderID string
	Model      string
	Purpose    string
}

func (t *AdvisorTool) runAdvisorCall(
	ctx context.Context,
	bridge AdvisorBridge,
	args map[string]interface{},
	missingContext []string,
	evidence []advisorEvidenceItem,
	candidate advisorModelCandidate,
	purpose string,
) (advisorDecisionMemo, advisorCallMeta, error) {
	prompt, err := buildAdvisorPrompt(args, missingContext, evidence, purpose)
	if err != nil {
		return advisorDecisionMemo{}, advisorCallMeta{}, err
	}
	resp, err := bridge.Chat(ctx, prompt, advisorMaxPromptTokens, AdvisorBridgeOptions{
		ProviderID:   candidate.ProviderID,
		ProviderName: candidate.ProviderName,
		Model:        candidate.ModelID,
		Purpose:      purpose,
	})
	if err != nil {
		return advisorDecisionMemo{}, advisorCallMeta{}, err
	}
	memo := parseAdvisorDecisionMemo(resp.Content)
	if len(memo.Evidence) == 0 && len(evidence) > 0 {
		memo.Evidence = append([]advisorEvidenceItem(nil), evidence...)
	}
	if memo.Confidence <= 0 {
		memo.Confidence = advisorDefaultConfidence(args, len(memo.Evidence) > 0)
	}
	return ensureAdvisorDecisionMemo(memo), advisorCallMeta{
		Provider:   strings.TrimSpace(resp.Provider),
		ProviderID: strings.TrimSpace(resp.ProviderID),
		Model:      strings.TrimSpace(resp.Model),
		Purpose:    purpose,
	}, nil
}

func normalizeAdvisorArgs(args map[string]interface{}) {
	if args == nil {
		return
	}
	advisorNormalizeStringAlias(args, "action")
	advisorNormalizeStringAlias(args, "question", "query", "topic", "prompt", "message", "input")
	advisorNormalizeStringAlias(args, "category", "kind", "type")
	advisorNormalizeStringAlias(args, "decision_mode", "decisionMode")
	advisorNormalizeStringAlias(args, "grounding")
	advisorNormalizeStringAlias(args, "depth")
	advisorNormalizeStringAlias(args, "output")
	advisorNormalizeStringAlias(args, "scorecard_pack", "scorecardPack")
	advisorNormalizeStringAlias(args, "lang", "language", "locale")
	advisorNormalizeStringAlias(args, "job_id", "jobId", "id")

	action := normalizeAdvisorAction(firstCompatString(args, "action"))
	args["action"] = action

	if question := strings.TrimSpace(firstCompatString(args, "question")); question != "" {
		args["question"] = question
	}

	if category := normalizeAdvisorCategory(firstCompatString(args, "category")); category != "" {
		args["category"] = category
	} else if inferred := inferAdvisorCategory(args); inferred != "" {
		args["category"] = inferred
	}

	if mode := normalizeAdvisorDecisionMode(firstCompatString(args, "decision_mode")); mode != "" {
		args["decision_mode"] = mode
	} else if inferred := inferAdvisorDecisionMode(args); inferred != "" {
		args["decision_mode"] = inferred
	}

	grounding := strings.ToLower(strings.TrimSpace(firstCompatString(args, "grounding")))
	switch grounding {
	case advisorGroundingNone, advisorGroundingWeb:
	default:
		grounding = advisorGroundingAuto
	}
	args["grounding"] = grounding

	depth := strings.ToLower(strings.TrimSpace(firstCompatString(args, "depth")))
	switch depth {
	case advisorDepthQuick, advisorDepthDeep:
	default:
		depth = advisorDepthStandard
	}
	args["depth"] = depth

	output := strings.ToLower(strings.TrimSpace(firstCompatString(args, "output")))
	switch output {
	case advisorOutputScorecard, advisorOutputDecisionPack:
	default:
		output = advisorOutputDecisionMemo
	}
	args["output"] = output

	scorecardPack := normalizeAdvisorScorecardPack(firstCompatString(args, "scorecard_pack"))
	if scorecardPack == "" {
		scorecardPack = advisorScorecardPackAuto
	}
	args["scorecard_pack"] = scorecardPack

	if raw, ok := compatArgValue(args, "candidates", "options", "choices"); ok {
		items := advisorCollectStrings(raw)
		if len(items) > 0 {
			args["candidates"] = items
		}
	}

	contextMap := map[string]interface{}{}
	if raw, ok := compatArgValue(args, "context"); ok {
		if existing, ok := coerceCompatMap(raw); ok {
			for key, value := range existing {
				contextMap[strings.ToLower(strings.TrimSpace(key))] = advisorNormalizeContextValue(value)
			}
		}
	}
	for _, key := range advisorContextKeys {
		if value, ok := compatArgValue(args, key); ok {
			contextMap[key] = advisorNormalizeContextValue(value)
		}
	}
	if len(contextMap) > 0 {
		args["context"] = contextMap
	}

	if raw, ok := compatArgValue(args, "constraints"); ok {
		if existing, ok := coerceCompatMap(raw); ok {
			constraints := make(map[string]interface{}, len(existing))
			for key, value := range existing {
				constraints[strings.ToLower(strings.TrimSpace(key))] = advisorNormalizeContextValue(value)
			}
			args["constraints"] = constraints
		}
	}

	if raw, ok := compatArgValue(args, "scorecard_weights", "scorecardWeights"); ok {
		if existing, ok := coerceCompatMap(raw); ok && len(existing) > 0 {
			weights := make(map[string]float64, len(existing))
			for key, value := range existing {
				if weight := advisorNormalizeConfidence(value); weight > 0 {
					weights[strings.ToLower(strings.TrimSpace(key))] = weight
				}
			}
			args["scorecard_weights"] = weights
		}
	}
}

func advisorMissingContext(args map[string]interface{}) ([]string, bool) {
	normalizeAdvisorArgs(args)
	category := normalizeAdvisorCategory(firstCompatString(args, "category"))
	mode := normalizeAdvisorDecisionMode(firstCompatString(args, "decision_mode"))
	contextMap := advisorContextMap(args)
	constraintsMap := advisorConstraintsMap(args)

	switch {
	case category == "language" || category == "framework" || category == "library" || category == "migration" || mode == "replace":
		missing := make([]string, 0, 3)
		if advisorTextValue(contextMap["current_solution"]) == "" {
			missing = append(missing, "current_solution")
		}
		if advisorTextValue(contextMap["team_size"]) == "" && advisorTextValue(contextMap["data_scale"]) == "" {
			missing = append(missing, "team_size_or_data_scale")
		}
		if !advisorHasDecisionConstraint(contextMap, constraintsMap) {
			missing = append(missing, "decision_constraints")
		}
		return missing, len(missing) >= 2
	case category == "process":
		missing := make([]string, 0, 3)
		if advisorTextValue(contextMap["team_size"]) == "" && advisorTextValue(contextMap["stage"]) == "" {
			missing = append(missing, "team_size_or_stage")
		}
		if advisorTextValue(contextMap["current_solution"]) == "" {
			missing = append(missing, "process_maturity")
		}
		if len(advisorCollectStrings(contextMap["must_have"])) == 0 && len(advisorCollectStrings(contextMap["must_avoid"])) == 0 && len(advisorCollectStrings(constraintsMap["pain_points"])) == 0 {
			missing = append(missing, "pain_points")
		}
		return missing, len(missing) >= 2
	case category == "architecture" || category == "ops":
		missing := make([]string, 0, 3)
		if advisorTextValue(contextMap["deployment"]) == "" {
			missing = append(missing, "deployment")
		}
		if !advisorHasOpsConstraint(contextMap, constraintsMap) {
			missing = append(missing, "availability_latency_or_cost_constraints")
		}
		if advisorTextValue(contextMap["data_scale"]) == "" {
			missing = append(missing, "data_scale")
		}
		return missing, len(missing) >= 2
	default:
		missing := []string{}
		if advisorTextValue(contextMap["team_size"]) == "" && advisorTextValue(contextMap["data_scale"]) == "" {
			missing = append(missing, "team_size_or_data_scale")
		}
		if !advisorHasDecisionConstraint(contextMap, constraintsMap) {
			missing = append(missing, "decision_constraints")
		}
		return missing, len(missing) >= 2
	}
}

func shouldGroundAdvisor(args map[string]interface{}) bool {
	normalizeAdvisorArgs(args)
	switch strings.ToLower(strings.TrimSpace(firstCompatString(args, "grounding"))) {
	case advisorGroundingNone:
		return false
	case advisorGroundingWeb:
		return true
	}

	category := normalizeAdvisorCategory(firstCompatString(args, "category"))
	mode := normalizeAdvisorDecisionMode(firstCompatString(args, "decision_mode"))
	question := strings.ToLower(strings.TrimSpace(firstCompatString(args, "question")))
	candidates := advisorCollectStrings(args["candidates"])

	if len(candidates) > 0 {
		return true
	}
	for _, term := range advisorRecencyTerms {
		if strings.Contains(question, term) {
			return true
		}
	}
	switch category {
	case "language", "framework", "library", "migration":
		return true
	}
	switch mode {
	case "compare", "replace":
		return true
	}
	for _, term := range advisorComparisonTerms {
		if strings.Contains(question, term) {
			return true
		}
	}
	return false
}

func shouldRunAdvisorSecondOpinion(args map[string]interface{}, primaryConfidence float64, evidenceConflict bool) bool {
	normalizeAdvisorArgs(args)
	mode := normalizeAdvisorDecisionMode(firstCompatString(args, "decision_mode"))
	depth := strings.ToLower(strings.TrimSpace(firstCompatString(args, "depth")))
	candidates := advisorCollectStrings(args["candidates"])

	if mode == "compare" || mode == "replace" {
		return true
	}
	if len(candidates) >= 2 {
		return true
	}
	if depth == advisorDepthDeep {
		return true
	}
	if evidenceConflict {
		return true
	}
	return primaryConfidence > 0 && primaryConfidence < 0.75
}

func selectAdvisorPrimaryCandidate(candidates []advisorModelCandidate) (advisorModelCandidate, bool) {
	for _, tier := range []string{"premium", "balanced", "fast"} {
		for _, candidate := range candidates {
			if advisorModelTier(candidate.ModelID) == tier {
				return candidate, true
			}
		}
	}
	return advisorModelCandidate{}, false
}

func selectAdvisorChallengerCandidate(candidates []advisorModelCandidate, primary advisorModelCandidate) (advisorModelCandidate, bool) {
	primaryFamily := advisorModelFamily(primary.ModelID)
	for _, candidate := range candidates {
		if candidate.ModelID == primary.ModelID && candidate.ProviderID == primary.ProviderID {
			continue
		}
		if candidate.ProviderID != "" && primary.ProviderID != "" && candidate.ProviderID != primary.ProviderID {
			return candidate, true
		}
	}
	for _, candidate := range candidates {
		if candidate.ModelID == primary.ModelID && candidate.ProviderID == primary.ProviderID {
			continue
		}
		if advisorModelFamily(candidate.ModelID) != "" && advisorModelFamily(candidate.ModelID) != primaryFamily {
			return candidate, true
		}
	}
	return advisorModelCandidate{}, false
}

type ProxyBridgeAdvisorAdapter struct {
	bridge *proxybridge.Bridge
	pool   *providerpool.Pool
}

func NewProxyBridgeAdvisorAdapter(bridge *proxybridge.Bridge, pool *providerpool.Pool) *ProxyBridgeAdvisorAdapter {
	return &ProxyBridgeAdvisorAdapter{bridge: bridge, pool: pool}
}

func (a *ProxyBridgeAdvisorAdapter) Chat(ctx context.Context, prompt string, maxTokens int, opts AdvisorBridgeOptions) (AdvisorBridgeResponse, error) {
	if a == nil || a.bridge == nil {
		return AdvisorBridgeResponse{}, fmt.Errorf("advisor bridge not available")
	}
	if maxTokens <= 0 {
		maxTokens = advisorMaxPromptTokens
	}
	ctx = a.applyRoute(ctx, opts)
	resp, err := a.bridge.Chat(ctx, llm.ChatRequest{
		Model: firstNonEmptyAdvisorValue(opts.Model, "auto"),
		Messages: []llm.Message{{
			Role:    llm.RoleUser,
			Content: prompt,
		}},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return AdvisorBridgeResponse{}, err
	}
	result := AdvisorBridgeResponse{
		Content: resp.Message.Content,
	}
	if resp != nil {
		result.Provider = strings.TrimSpace(resp.Provider)
		result.ProviderID = strings.TrimSpace(resp.ProviderID)
		result.Model = strings.TrimSpace(resp.Model)
	}
	return result, nil
}

func (a *ProxyBridgeAdvisorAdapter) applyRoute(ctx context.Context, opts AdvisorBridgeOptions) context.Context {
	if strings.TrimSpace(opts.ProviderID) != "" {
		ctx = proxy.WithPinnedProvider(ctx, strings.TrimSpace(opts.ProviderID))
	}
	if proxy.GetResolvedRouteFromContext(ctx) == nil {
		ctx = proxy.WithResolvedRoute(ctx, &proxy.ResolvedRoute{})
	}
	return ctx
}

func (a *ProxyBridgeAdvisorAdapter) AvailableCandidates() []advisorModelCandidate {
	if a == nil || a.pool == nil || a.pool.Router == nil {
		return nil
	}
	models := a.pool.Router.ListAvailableModels()
	if len(models) == 0 {
		return nil
	}
	candidates := make([]advisorModelCandidate, 0, len(models))
	for _, model := range models {
		if model == nil || !model.Enabled {
			continue
		}
		providerID := strings.TrimSpace(model.ProviderID)
		providerName := providerID
		if a.pool.Registry != nil && providerID != "" {
			if provider, err := a.pool.Registry.Get(providerID); err == nil && provider != nil && strings.TrimSpace(provider.Name) != "" {
				providerName = strings.TrimSpace(provider.Name)
			}
		}
		candidates = append(candidates, advisorModelCandidate{
			ProviderID:   providerID,
			ProviderName: providerName,
			ModelID:      strings.TrimSpace(model.ID),
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := advisorTierRank(advisorModelTier(candidates[i].ModelID))
		right := advisorTierRank(advisorModelTier(candidates[j].ModelID))
		if left != right {
			return left < right
		}
		if candidates[i].ProviderID != candidates[j].ProviderID {
			return candidates[i].ProviderID < candidates[j].ProviderID
		}
		return candidates[i].ModelID < candidates[j].ModelID
	})
	return candidates
}

func advisorAvailableCandidates(bridge AdvisorBridge) []advisorModelCandidate {
	type availableCandidates interface {
		AvailableCandidates() []advisorModelCandidate
	}
	if bridge == nil {
		return nil
	}
	if provider, ok := bridge.(availableCandidates); ok {
		return provider.AvailableCandidates()
	}
	return nil
}

func buildAdvisorPrompt(args map[string]interface{}, missingContext []string, evidence []advisorEvidenceItem, purpose string) (string, error) {
	payload := advisorPromptEnvelope{
		Action:           normalizeAdvisorAction(firstCompatString(args, "action")),
		Question:         strings.TrimSpace(firstCompatString(args, "question")),
		Category:         normalizeAdvisorCategory(firstCompatString(args, "category")),
		DecisionMode:     normalizeAdvisorDecisionMode(firstCompatString(args, "decision_mode")),
		Candidates:       advisorCollectStrings(args["candidates"]),
		Context:          advisorContextMap(args),
		Constraints:      advisorConstraintsMap(args),
		Grounding:        strings.TrimSpace(firstCompatString(args, "grounding")),
		Depth:            strings.TrimSpace(firstCompatString(args, "depth")),
		Output:           strings.TrimSpace(firstCompatString(args, "output")),
		ScorecardPack:    strings.TrimSpace(firstCompatString(args, "scorecard_pack")),
		ScorecardWeights: advisorParseWeightMap(args["scorecard_weights"]),
		Lang:             strings.TrimSpace(firstCompatString(args, "lang", "language")),
		MissingContext:   append([]string(nil), missingContext...),
		Evidence:         append([]advisorEvidenceItem(nil), evidence...),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	tone := "Provide the best recommendation."
	if purpose == advisorPurposeChallenger {
		tone = "Act as a challenger. Stress test the recommendation and call out overconfidence."
	}

	return strings.Join([]string{
		"You are an engineering advisor for technology selection, replacements, migrations, and best practices.",
		tone,
		"Return JSON only with these exact top-level keys: recommendation, why, best_fit_for, not_fit_for, alternatives, tradeoffs, risks, best_practices, missing_context, confidence, evidence.",
		"Keep confidence between 0 and 1. If context is missing, make assumptions explicit in missing_context.",
		"Do not wrap the JSON in markdown fences.",
		string(raw),
	}, "\n\n"), nil
}

func parseAdvisorDecisionMemo(raw string) advisorDecisionMemo {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ensureAdvisorDecisionMemo(advisorDecisionMemo{})
	}
	parsed := map[string]interface{}{}
	if json.Unmarshal([]byte(raw), &parsed) != nil {
		if extracted := advisorExtractJSONObject(raw); extracted != "" {
			_ = json.Unmarshal([]byte(extracted), &parsed)
		}
	}
	if len(parsed) == 0 {
		return ensureAdvisorDecisionMemo(advisorDecisionMemo{Recommendation: raw})
	}

	memo := advisorDecisionMemo{
		Recommendation: strings.TrimSpace(formatValue(parsed["recommendation"])),
		Why:            advisorCollectStrings(parsed["why"]),
		BestFitFor:     advisorCollectStrings(parsed["best_fit_for"]),
		NotFitFor:      advisorCollectStrings(parsed["not_fit_for"]),
		Alternatives:   advisorCollectStrings(parsed["alternatives"]),
		Tradeoffs:      advisorCollectStrings(parsed["tradeoffs"]),
		Risks:          advisorCollectStrings(parsed["risks"]),
		BestPractices:  advisorCollectStrings(parsed["best_practices"]),
		MissingContext: advisorCollectStrings(parsed["missing_context"]),
		Confidence:     advisorNormalizeConfidence(parsed["confidence"]),
		Evidence:       advisorParseEvidence(parsed["evidence"]),
	}
	return ensureAdvisorDecisionMemo(memo)
}

func ensureAdvisorDecisionMemo(memo advisorDecisionMemo) advisorDecisionMemo {
	if memo.Why == nil {
		memo.Why = []string{}
	}
	if memo.BestFitFor == nil {
		memo.BestFitFor = []string{}
	}
	if memo.NotFitFor == nil {
		memo.NotFitFor = []string{}
	}
	if memo.Alternatives == nil {
		memo.Alternatives = []string{}
	}
	if memo.Tradeoffs == nil {
		memo.Tradeoffs = []string{}
	}
	if memo.Risks == nil {
		memo.Risks = []string{}
	}
	if memo.BestPractices == nil {
		memo.BestPractices = []string{}
	}
	if memo.MissingContext == nil {
		memo.MissingContext = []string{}
	}
	if memo.Evidence == nil {
		memo.Evidence = []advisorEvidenceItem{}
	}
	if memo.Confidence < 0 {
		memo.Confidence = 0
	}
	if memo.Confidence > 1 {
		memo.Confidence = 1
	}
	return memo
}

func collectAdvisorEvidence(ctx context.Context, args map[string]interface{}, executor *Executor) ([]advisorEvidenceItem, string) {
	if !shouldGroundAdvisor(args) || executor == nil {
		return nil, ""
	}

	question := strings.TrimSpace(firstCompatString(args, "question"))
	candidates := advisorCollectStrings(args["candidates"])
	searchQuery := question
	if len(candidates) > 0 && !strings.Contains(strings.ToLower(searchQuery), strings.ToLower(strings.Join(candidates, " "))) {
		searchQuery = searchQuery + " " + strings.Join(candidates, " ")
	}

	result, err := executor.Execute(ctx, "web_search", map[string]interface{}{
		"query": searchQuery,
		"limit": 4,
	})
	if err != nil {
		return nil, fmt.Sprintf("grounding unavailable: %v", err)
	}

	payload := map[string]interface{}{}
	switch typed := result.(type) {
	case string:
		if json.Unmarshal([]byte(typed), &payload) != nil {
			return nil, "grounding returned unparsable results"
		}
	case map[string]interface{}:
		payload = typed
	default:
		return nil, "grounding returned unsupported payload"
	}

	items := []advisorEvidenceItem{}
	if raw, ok := payload["results"].([]interface{}); ok {
		for _, item := range raw {
			entry, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			url := strings.TrimSpace(formatValue(entry["url"]))
			if url == "" {
				continue
			}
			items = append(items, advisorEvidenceItem{
				Label:   firstNonEmptyAdvisorValue(strings.TrimSpace(formatValue(entry["title"])), url),
				URL:     url,
				Source:  "search",
				Kind:    "web",
				Note:    strings.TrimSpace(formatValue(entry["description"])),
				Quality: advisorEvidenceQuality(url),
				Domain:  advisorEvidenceDomain(url),
				Stance:  "context",
			})
			if len(items) >= 4 {
				break
			}
		}
	}
	if len(items) == 0 {
		return nil, "grounding returned no usable evidence"
	}
	return ensureAdvisorEvidenceItems(items), ""
}

func advisorAskMissingContext(ctx context.Context, args map[string]interface{}, executor *Executor, missingContext []string) (map[string]interface{}, []string, error) {
	if executor == nil || len(missingContext) == 0 {
		return args, missingContext, nil
	}
	questions, bindings := advisorBuildAskQuestions(args, missingContext)
	if len(questions) == 0 {
		return args, missingContext, nil
	}
	result, err := executor.Execute(ctx, "ask", map[string]interface{}{"questions": questions})
	if err != nil {
		return args, missingContext, err
	}
	payload, ok := result.(map[string]interface{})
	if !ok {
		if raw, ok := result.(string); ok {
			parsed := map[string]interface{}{}
			if json.Unmarshal([]byte(raw), &parsed) == nil {
				payload = parsed
				ok = true
			}
		}
	}
	if !ok {
		return args, missingContext, fmt.Errorf("ask returned unsupported payload")
	}
	updated := advisorCloneMap(args)
	if updated == nil {
		updated = map[string]interface{}{}
	}
	contextMap := advisorContextMap(updated)
	constraintsMap := advisorConstraintsMap(updated)
	qa, _ := payload["qa"].([]interface{})
	for idx, binding := range bindings {
		if idx >= len(qa) {
			continue
		}
		answerMap, ok := qa[idx].(map[string]interface{})
		if !ok {
			continue
		}
		answers := advisorCollectStrings(answerMap["a"])
		if len(answers) == 0 {
			continue
		}
		value := strings.TrimSpace(strings.Join(answers, ", "))
		switch binding.target {
		case "context":
			contextMap[binding.key] = value
		case "constraints":
			constraintsMap[binding.key] = value
		}
	}
	if len(contextMap) > 0 {
		updated["context"] = contextMap
	}
	if len(constraintsMap) > 0 {
		updated["constraints"] = constraintsMap
	}
	updatedMissing, _ := advisorMissingContext(updated)
	return updated, updatedMissing, nil
}

type advisorAskBinding struct {
	target string
	key    string
}

func advisorBuildAskQuestions(args map[string]interface{}, missingContext []string) ([]map[string]interface{}, []advisorAskBinding) {
	category := normalizeAdvisorCategory(firstCompatString(args, "category"))
	mode := normalizeAdvisorDecisionMode(firstCompatString(args, "decision_mode"))
	questions := make([]map[string]interface{}, 0, min(3, len(missingContext)))
	bindings := make([]advisorAskBinding, 0, min(3, len(missingContext)))
	addText := func(question string, target string, key string) {
		if len(questions) >= 3 {
			return
		}
		questions = append(questions, map[string]interface{}{
			"question": question,
			"type":     "text",
		})
		bindings = append(bindings, advisorAskBinding{target: target, key: key})
	}
	addRadio := func(question string, target string, key string, options []string) {
		if len(questions) >= 3 {
			return
		}
		questions = append(questions, map[string]interface{}{
			"question": question,
			"type":     "radio",
			"options":  options,
		})
		bindings = append(bindings, advisorAskBinding{target: target, key: key})
	}
	for _, missing := range missingContext {
		switch missing {
		case "current_solution":
			addText("What is the current solution or default choice today?", "context", "current_solution")
		case "team_size_or_data_scale":
			if category == "architecture" || category == "ops" {
				addText("What data scale are you operating at?", "context", "data_scale")
			} else {
				addText("What is the team size or approximate data scale?", "context", "team_size")
			}
		case "decision_constraints":
			addRadio("Which constraint matters most for this decision?", "constraints", "priority_constraint", []string{"Performance", "Cost", "Delivery speed", "Operational complexity"})
		case "team_size_or_stage":
			addText("What is the team size or current project stage?", "context", "team_size")
		case "process_maturity":
			addText("What does the current process look like today?", "context", "current_solution")
		case "pain_points":
			addText("What is the main pain point you want to fix?", "constraints", "pain_points")
		case "deployment":
			addRadio("Which deployment model best matches your environment?", "context", "deployment", []string{"Kubernetes", "VMs", "Single host", "Managed cloud service"})
		case "availability_latency_or_cost_constraints":
			addRadio("Which constraint matters most here?", "constraints", "priority_constraint", []string{"Availability", "Latency", "Cost"})
		case "data_scale":
			addText("What is the approximate data scale or workload size?", "context", "data_scale")
		}
		if len(questions) >= 3 {
			break
		}
	}
	if mode == "replace" && len(questions) < 3 && advisorTextValue(advisorContextMap(args)["current_solution"]) == "" {
		addText("What are you replacing today?", "context", "current_solution")
	}
	return questions, bindings
}

func ensureAdvisorEvidenceItems(items []advisorEvidenceItem) []advisorEvidenceItem {
	if items == nil {
		return []advisorEvidenceItem{}
	}
	out := make([]advisorEvidenceItem, 0, len(items))
	for idx, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			item.ID = fmt.Sprintf("ev-%d", idx+1)
		}
		if strings.TrimSpace(item.Domain) == "" {
			item.Domain = advisorEvidenceDomain(item.URL)
		}
		if strings.TrimSpace(item.Quality) == "" {
			item.Quality = advisorEvidenceQuality(item.URL)
		}
		if strings.TrimSpace(item.Stance) == "" {
			item.Stance = "context"
		}
		out = append(out, item)
	}
	return out
}

func advisorEvidenceDomain(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return strings.ToLower(strings.TrimSpace(trimmed))
}

func advisorEvidenceQuality(rawURL string) string {
	domain := advisorEvidenceDomain(rawURL)
	switch {
	case domain == "":
		return ""
	case strings.Contains(domain, "github.com"):
		if strings.Contains(strings.ToLower(rawURL), "/releases") {
			return "release"
		}
		return "repo"
	case strings.Contains(domain, "docs.") || strings.HasSuffix(domain, ".org") || strings.HasSuffix(domain, ".dev") || strings.HasSuffix(domain, ".io"):
		return "official"
	case strings.Contains(domain, "openoffice") || strings.Contains(domain, "onlyoffice") || strings.Contains(domain, "libreoffice"):
		return "vendor"
	default:
		return "ecosystem"
	}
}

func mergeAdvisorSecondOpinion(primaryMeta, challengerMeta advisorCallMeta, primaryMemo, challengerMemo advisorDecisionMemo) advisorSecondOpinion {
	agreement := "mixed"
	if advisorRecommendationsAgree(primaryMemo.Recommendation, challengerMemo.Recommendation) {
		agreement = "agree"
	} else if strings.TrimSpace(challengerMemo.Recommendation) != "" {
		agreement = "challenge"
	}
	summary := strings.TrimSpace(challengerMemo.Recommendation)
	if summary == "" && len(challengerMemo.Why) > 0 {
		summary = challengerMemo.Why[0]
	}
	return advisorSecondOpinion{
		Used:       true,
		Reason:     "second opinion requested for decision validation",
		Summary:    summary,
		Agreement:  agreement,
		Provider:   challengerMeta.Provider,
		ProviderID: challengerMeta.ProviderID,
		Model:      challengerMeta.Model,
	}
}

func advisorRecommendationsAgree(left, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	switch {
	case left == "" || right == "":
		return false
	case left == right:
		return true
	case strings.Contains(left, right), strings.Contains(right, left):
		return true
	default:
		return false
	}
}

func advisorExtractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}

func advisorParseEvidence(raw interface{}) []advisorEvidenceItem {
	items, ok := raw.([]interface{})
	if !ok {
		if typed, ok := raw.([]map[string]interface{}); ok {
			items = make([]interface{}, 0, len(typed))
			for _, item := range typed {
				items = append(items, item)
			}
		}
	}
	if len(items) == 0 {
		return []advisorEvidenceItem{}
	}
	out := make([]advisorEvidenceItem, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, advisorEvidenceItem{
			ID:          strings.TrimSpace(formatValue(entry["id"])),
			Label:       strings.TrimSpace(formatValue(entry["label"])),
			URL:         strings.TrimSpace(formatValue(entry["url"])),
			Source:      strings.TrimSpace(formatValue(entry["source"])),
			Kind:        strings.TrimSpace(formatValue(entry["kind"])),
			Note:        strings.TrimSpace(formatValue(entry["note"])),
			Quality:     strings.TrimSpace(formatValue(entry["quality"])),
			PublishedAt: strings.TrimSpace(formatValue(entry["published_at"])),
			Domain:      strings.TrimSpace(formatValue(entry["domain"])),
			Candidate:   strings.TrimSpace(formatValue(entry["candidate"])),
			Criterion:   strings.TrimSpace(formatValue(entry["criterion"])),
			Stance:      strings.TrimSpace(formatValue(entry["stance"])),
		})
	}
	return ensureAdvisorEvidenceItems(out)
}

func advisorNormalizeConfidence(raw interface{}) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case json.Number:
		if parsed, err := value.Float64(); err == nil {
			return parsed
		}
	case string:
		if parsed := strings.TrimSpace(value); parsed != "" {
			var out float64
			if _, err := fmt.Sscanf(parsed, "%f", &out); err == nil {
				return out
			}
		}
	}
	return 0
}

func advisorDefaultConfidence(args map[string]interface{}, grounded bool) float64 {
	confidence := 0.68
	if grounded {
		confidence += 0.08
	}
	if _, ask := advisorMissingContext(args); ask {
		confidence -= 0.12
	}
	if strings.EqualFold(firstCompatString(args, "depth"), advisorDepthDeep) {
		confidence += 0.04
	}
	if confidence < 0.1 {
		return 0.1
	}
	if confidence > 0.95 {
		return 0.95
	}
	return confidence
}

type advisorPackCriterion struct {
	ID     string
	Label  string
	Weight float64
}

type advisorPackDefinition struct {
	ID       string
	Criteria []advisorPackCriterion
}

func normalizeAdvisorAction(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", advisorActionRun:
		return advisorActionRun
	case advisorActionStatus, "poll", "resume":
		return advisorActionStatus
	default:
		return advisorActionRun
	}
}

func normalizeAdvisorScorecardPack(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", advisorScorecardPackAuto:
		return advisorScorecardPackAuto
	case advisorScorecardPackSolutionSelectionV1,
		advisorScorecardPackMigrationV1,
		advisorScorecardPackArchitectureV1,
		advisorScorecardPackProcessV1:
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func shouldRunAdvisorAsync(args map[string]interface{}) bool {
	return strings.EqualFold(firstCompatString(args, "depth"), advisorDepthDeep) &&
		!strings.EqualFold(firstCompatString(args, "mode"), "advisor")
}

func advisorPackDefinitions() map[string]advisorPackDefinition {
	return map[string]advisorPackDefinition{
		advisorScorecardPackSolutionSelectionV1: {
			ID: advisorScorecardPackSolutionSelectionV1,
			Criteria: []advisorPackCriterion{
				{ID: "fitness", Label: "Fitness", Weight: 0.25},
				{ID: "team_velocity", Label: "Team Velocity", Weight: 0.15},
				{ID: "performance_efficiency", Label: "Performance Efficiency", Weight: 0.15},
				{ID: "operability", Label: "Operability", Weight: 0.10},
				{ID: "ecosystem_maturity", Label: "Ecosystem Maturity", Weight: 0.15},
				{ID: "cost", Label: "Cost", Weight: 0.10},
				{ID: "risk_lock_in", Label: "Risk / Lock-in", Weight: 0.10},
			},
		},
		advisorScorecardPackMigrationV1: {
			ID: advisorScorecardPackMigrationV1,
			Criteria: []advisorPackCriterion{
				{ID: "target_fitness", Label: "Target Fitness", Weight: 0.20},
				{ID: "migration_effort", Label: "Migration Effort", Weight: 0.20},
				{ID: "operational_risk", Label: "Operational Risk", Weight: 0.20},
				{ID: "team_enablement", Label: "Team Enablement", Weight: 0.10},
				{ID: "performance_cost_delta", Label: "Performance / Cost Delta", Weight: 0.15},
				{ID: "ecosystem_maintainability", Label: "Ecosystem Maintainability", Weight: 0.15},
			},
		},
		advisorScorecardPackArchitectureV1: {
			ID: advisorScorecardPackArchitectureV1,
			Criteria: []advisorPackCriterion{
				{ID: "reliability", Label: "Reliability", Weight: 0.20},
				{ID: "scalability", Label: "Scalability", Weight: 0.15},
				{ID: "latency", Label: "Latency", Weight: 0.15},
				{ID: "operability", Label: "Operability", Weight: 0.15},
				{ID: "security_compliance", Label: "Security / Compliance", Weight: 0.15},
				{ID: "cost_efficiency", Label: "Cost Efficiency", Weight: 0.10},
				{ID: "delivery_complexity", Label: "Delivery Complexity", Weight: 0.10},
			},
		},
		advisorScorecardPackProcessV1: {
			ID: advisorScorecardPackProcessV1,
			Criteria: []advisorPackCriterion{
				{ID: "feedback_speed", Label: "Feedback Speed", Weight: 0.20},
				{ID: "quality_risk_reduction", Label: "Quality / Risk Reduction", Weight: 0.20},
				{ID: "team_cognitive_load", Label: "Team Cognitive Load", Weight: 0.15},
				{ID: "coordination_overhead", Label: "Coordination Overhead", Weight: 0.15},
				{ID: "adoption_cost", Label: "Adoption Cost", Weight: 0.15},
				{ID: "measurability", Label: "Measurability", Weight: 0.15},
			},
		},
	}
}

func advisorResolveScorecardPack(args map[string]interface{}) string {
	explicit := normalizeAdvisorScorecardPack(firstCompatString(args, "scorecard_pack"))
	if explicit != "" && explicit != advisorScorecardPackAuto {
		return explicit
	}
	category := normalizeAdvisorCategory(firstCompatString(args, "category"))
	mode := normalizeAdvisorDecisionMode(firstCompatString(args, "decision_mode"))
	switch {
	case category == "process":
		return advisorScorecardPackProcessV1
	case category == "architecture" || category == "ops":
		return advisorScorecardPackArchitectureV1
	case category == "migration" || mode == "replace":
		return advisorScorecardPackMigrationV1
	default:
		return advisorScorecardPackSolutionSelectionV1
	}
}

func advisorParseWeightMap(raw interface{}) map[string]float64 {
	if existing, ok := raw.(map[string]float64); ok {
		out := make(map[string]float64, len(existing))
		for key, value := range existing {
			out[strings.ToLower(strings.TrimSpace(key))] = value
		}
		return out
	}
	typed, ok := coerceCompatMap(raw)
	if !ok || len(typed) == 0 {
		return nil
	}
	out := make(map[string]float64, len(typed))
	for key, value := range typed {
		if weight := advisorNormalizeConfidence(value); weight > 0 {
			out[strings.ToLower(strings.TrimSpace(key))] = weight
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func advisorResolveScorecardWeights(args map[string]interface{}) (advisorPackDefinition, []advisorWeight, error) {
	packID := advisorResolveScorecardPack(args)
	pack, ok := advisorPackDefinitions()[packID]
	if !ok {
		return advisorPackDefinition{}, nil, fmt.Errorf("unsupported scorecard_pack: %s", packID)
	}

	rawOverrides := advisorParseWeightMap(args["scorecard_weights"])
	weightByCriterion := make(map[string]float64, len(pack.Criteria))
	sourceByCriterion := make(map[string]string, len(pack.Criteria))
	total := 0.0
	for _, criterion := range pack.Criteria {
		weight := criterion.Weight
		source := "default"
		if override, ok := rawOverrides[criterion.ID]; ok {
			if override <= 0 {
				return advisorPackDefinition{}, nil, fmt.Errorf("scorecard weight for %s must be > 0", criterion.ID)
			}
			weight = override
			source = "user"
		}
		weightByCriterion[criterion.ID] = weight
		sourceByCriterion[criterion.ID] = source
		total += weight
	}
	for criterion := range rawOverrides {
		if _, ok := weightByCriterion[criterion]; !ok {
			return advisorPackDefinition{}, nil, fmt.Errorf("unknown scorecard criterion: %s", criterion)
		}
	}
	if total <= 0 {
		return advisorPackDefinition{}, nil, fmt.Errorf("scorecard weights must sum to > 0")
	}

	weights := make([]advisorWeight, 0, len(pack.Criteria))
	for _, criterion := range pack.Criteria {
		weights = append(weights, advisorWeight{
			Criterion: criterion.ID,
			Label:     criterion.Label,
			Weight:    weightByCriterion[criterion.ID] / total,
			Source:    sourceByCriterion[criterion.ID],
		})
	}
	return pack, weights, nil
}

func buildAdvisorScorecard(args map[string]interface{}, pack advisorPackDefinition, weights []advisorWeight, memo advisorDecisionMemo, evidenceConflict bool) advisorScorecard {
	evidence := ensureAdvisorEvidenceItems(memo.Evidence)
	candidates := advisorScorecardCandidates(args, memo)
	winner := advisorPickWinner(candidates, memo)
	ordered := advisorRankCandidates(candidates, winner)
	evidenceIDs := advisorEvidenceIDs(evidence, 2)

	items := make([]advisorCandidateScore, 0, len(ordered))
	for idx, candidate := range ordered {
		criterionScores := make([]advisorCriterionScore, 0, len(weights))
		total := 0.0
		for _, weight := range weights {
			score := advisorCandidateCriterionScore(idx, weight.Criterion)
			reason := advisorCriterionReason(candidate, winner, weight.Label, memo)
			criterionScores = append(criterionScores, advisorCriterionScore{
				Criterion:   weight.Criterion,
				Score:       score,
				Reason:      reason,
				EvidenceIDs: append([]string(nil), evidenceIDs...),
			})
			total += score * weight.Weight
		}
		totalScore := roundAdvisorScore(total * 20)
		concerns := uniqueStrings(append([]string{}, memo.Tradeoffs...))
		concerns = uniqueStrings(append(concerns, memo.Risks...))
		if len(concerns) > 2 {
			concerns = concerns[:2]
		}
		strengths := memo.Why
		if len(strengths) > 2 {
			strengths = strengths[:2]
		}
		bestFitFor := memo.BestFitFor
		if candidate != winner && len(bestFitFor) == 0 {
			bestFitFor = []string{"Teams willing to trade peak fit for continuity or familiarity."}
		}
		if len(bestFitFor) == 0 {
			bestFitFor = []string{}
		}
		items = append(items, advisorCandidateScore{
			Name:            candidate,
			Rank:            idx + 1,
			TotalScore:      totalScore,
			Verdict:         advisorCandidateVerdict(candidate, winner, totalScore),
			Strengths:       append([]string(nil), strengths...),
			Concerns:        append([]string(nil), concerns...),
			BestFitFor:      append([]string(nil), bestFitFor...),
			CriterionScores: criterionScores,
		})
	}

	margin := 100.0
	if len(items) >= 2 {
		margin = items[0].TotalScore - items[1].TotalScore
	}
	topCriteria := make([]string, 0, len(weights))
	for _, weight := range weights {
		topCriteria = append(topCriteria, weight.Criterion)
	}
	if len(topCriteria) > 3 {
		topCriteria = topCriteria[:3]
	}

	return ensureAdvisorScorecard(advisorScorecard{
		PackID:     pack.ID,
		Winner:     winner,
		Summary:    firstNonEmptyAdvisorValue(strings.TrimSpace(memo.Recommendation), winner),
		Weights:    weights,
		Candidates: items,
		Sensitivity: advisorSensitivity{
			CloseCall:         margin < 8,
			Margin:            roundAdvisorScore(margin),
			TopDriverCriteria: topCriteria,
			FlipRisk:          advisorFlipRisk(margin, len(memo.MissingContext), memo.SecondOpinion, evidenceConflict),
		},
		MissingContext: append([]string(nil), memo.MissingContext...),
		Confidence:     memo.Confidence,
		Evidence:       evidence,
		SecondOpinion:  memo.SecondOpinion,
	})
}

func ensureAdvisorScorecard(scorecard advisorScorecard) advisorScorecard {
	if scorecard.Weights == nil {
		scorecard.Weights = []advisorWeight{}
	}
	if scorecard.Candidates == nil {
		scorecard.Candidates = []advisorCandidateScore{}
	}
	if scorecard.MissingContext == nil {
		scorecard.MissingContext = []string{}
	}
	if scorecard.Evidence == nil {
		scorecard.Evidence = []advisorEvidenceItem{}
	}
	if scorecard.Sensitivity.TopDriverCriteria == nil {
		scorecard.Sensitivity.TopDriverCriteria = []string{}
	}
	return scorecard
}

func buildAdvisorDecisionMeta(args map[string]interface{}, executionMode string, grounded bool, evidenceConflict bool, packID string, job *ResearchJob, memo advisorDecisionMemo) advisorDecisionMeta {
	meta := advisorDecisionMeta{
		Version:          "advisor.v2",
		ExecutionMode:    executionMode,
		GroundingUsed:    grounded,
		PackID:           packID,
		EvidenceConflict: evidenceConflict,
		Calibration: advisorCalibration{
			Groundedness: advisorMetaGroundedness(grounded, memo),
			Freshness:    advisorMetaFreshness(args, grounded),
			ConflictRisk: advisorMetaConflictRisk(evidenceConflict, memo.SecondOpinion),
		},
	}
	if job != nil && len(job.Report) > 0 {
		if calibration, ok := job.Report["calibration"].(map[string]interface{}); ok {
			meta.Calibration = advisorCalibration{
				Groundedness: advisorNormalizeConfidence(calibration["groundedness"]),
				Freshness:    advisorNormalizeConfidence(calibration["freshness"]),
				ConflictRisk: firstNonEmptyAdvisorValue(strings.TrimSpace(formatValue(calibration["conflict_risk"])), meta.Calibration.ConflictRisk),
			}
		}
	}
	return meta
}

func encodeAdvisorResult(args map[string]interface{}, memo advisorDecisionMemo, scorecard advisorScorecard, meta advisorDecisionMeta, job map[string]interface{}) (interface{}, error) {
	output := strings.TrimSpace(firstCompatString(args, "output"))
	switch output {
	case advisorOutputScorecard:
		payload := map[string]interface{}{
			"pack_id":         scorecard.PackID,
			"winner":          scorecard.Winner,
			"summary":         scorecard.Summary,
			"weights":         scorecard.Weights,
			"candidates":      scorecard.Candidates,
			"sensitivity":     scorecard.Sensitivity,
			"missing_context": scorecard.MissingContext,
			"confidence":      scorecard.Confidence,
			"evidence":        scorecard.Evidence,
			"second_opinion":  scorecard.SecondOpinion,
			"decision_meta":   meta,
		}
		if job != nil {
			payload["job"] = job
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return string(encoded), nil
	case advisorOutputDecisionPack:
		payload := map[string]interface{}{
			"memo":          memo,
			"scorecard":     scorecard,
			"decision_meta": meta,
		}
		if job != nil {
			payload["job"] = job
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return string(encoded), nil
	default:
		payload := map[string]interface{}{}
		raw, err := json.Marshal(memo)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		payload["decision_meta"] = meta
		if job != nil {
			payload["job"] = job
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		return string(encoded), nil
	}
}

func advisorScorecardCandidates(args map[string]interface{}, memo advisorDecisionMemo) []string {
	candidates := advisorCollectStrings(args["candidates"])
	if len(candidates) == 0 {
		question := strings.TrimSpace(firstCompatString(args, "question"))
		normalized := strings.NewReplacer(" versus ", " vs ", " VS ", " vs ").Replace(question)
		if strings.Contains(strings.ToLower(normalized), " vs ") {
			parts := strings.Split(normalized, " vs ")
			candidates = advisorCollectStrings(parts)
		}
	}
	if len(candidates) == 0 {
		if current := advisorTextValue(advisorContextMap(args)["current_solution"]); current != "" {
			candidates = append(candidates, current)
		}
	}
	winner := advisorPickWinner(candidates, memo)
	if winner != "" {
		candidates = uniqueStrings(append([]string{winner}, candidates...))
	}
	if len(candidates) == 0 && strings.TrimSpace(memo.Recommendation) != "" {
		candidates = []string{strings.TrimSpace(memo.Recommendation)}
	}
	return uniqueStrings(candidates)
}

func advisorPickWinner(candidates []string, memo advisorDecisionMemo) string {
	recommendation := strings.ToLower(strings.TrimSpace(memo.Recommendation))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if strings.Contains(recommendation, strings.ToLower(strings.TrimSpace(candidate))) {
			return candidate
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return strings.TrimSpace(memo.Recommendation)
}

func advisorRankCandidates(candidates []string, winner string) []string {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]string, 0, len(candidates))
	if winner != "" {
		out = append(out, winner)
	}
	for _, candidate := range candidates {
		if candidate == "" || strings.EqualFold(candidate, winner) {
			continue
		}
		out = append(out, candidate)
	}
	return uniqueStrings(out)
}

func advisorCandidateCriterionScore(rank int, criterion string) float64 {
	base := 4.6 - float64(rank)*0.8
	if strings.Contains(criterion, "risk") || strings.Contains(criterion, "cost") || strings.Contains(criterion, "effort") || strings.Contains(criterion, "complexity") {
		base -= 0.2
	}
	if base < 1.5 {
		base = 1.5
	}
	if base > 5 {
		base = 5
	}
	return roundAdvisorScore(base)
}

func advisorCriterionReason(candidate string, winner string, label string, memo advisorDecisionMemo) string {
	if strings.EqualFold(candidate, winner) {
		if len(memo.Why) > 0 {
			return memo.Why[0]
		}
		return fmt.Sprintf("%s scores best on %s for the stated context.", candidate, label)
	}
	return fmt.Sprintf("%s remains viable, but is weaker than %s on %s in the current context.", candidate, winner, label)
}

func advisorEvidenceIDs(items []advisorEvidenceItem, limit int) []string {
	out := make([]string, 0, min(limit, len(items)))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		out = append(out, item.ID)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func advisorCandidateVerdict(candidate string, winner string, totalScore float64) string {
	switch {
	case strings.EqualFold(candidate, winner):
		return "recommended"
	case totalScore >= 70:
		return "conditional"
	default:
		return "not_recommended"
	}
}

func advisorFlipRisk(margin float64, missingContext int, secondOpinion advisorSecondOpinion, evidenceConflict bool) string {
	switch {
	case evidenceConflict || secondOpinion.Agreement == "challenge" || missingContext >= 2:
		return "high"
	case margin < 8 || missingContext > 0:
		return "medium"
	default:
		return "low"
	}
}

func advisorMetaGroundedness(grounded bool, memo advisorDecisionMemo) float64 {
	score := 0.42
	if grounded {
		score += 0.25
	}
	if len(memo.Evidence) >= 3 {
		score += 0.1
	}
	if len(memo.MissingContext) > 0 {
		score -= 0.08
	}
	return clampAdvisorUnitScore(score)
}

func advisorMetaFreshness(args map[string]interface{}, grounded bool) float64 {
	score := 0.35
	if grounded {
		score += 0.2
	}
	question := strings.ToLower(strings.TrimSpace(firstCompatString(args, "question")))
	for _, term := range advisorRecencyTerms {
		if strings.Contains(question, term) {
			score += 0.15
			break
		}
	}
	return clampAdvisorUnitScore(score)
}

func advisorMetaConflictRisk(evidenceConflict bool, secondOpinion advisorSecondOpinion) string {
	switch {
	case evidenceConflict || secondOpinion.Agreement == "challenge":
		return "high"
	case secondOpinion.Used:
		return "medium"
	default:
		return "low"
	}
}

func roundAdvisorScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	return float64(int(v*10+0.5)) / 10
}

func clampAdvisorUnitScore(v float64) float64 {
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	default:
		return roundAdvisorScore(v)
	}
}

func advisorModelTier(modelID string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	for _, token := range []string{"opus", "sonnet", "gpt-5", " o3", "o3", "o4", " pro", " max"} {
		if strings.Contains(normalized, strings.TrimSpace(token)) {
			return "premium"
		}
	}
	for _, token := range []string{"claude", "gpt-4.1", "gemini", "glm", "qwen"} {
		if strings.Contains(normalized, token) {
			return "balanced"
		}
	}
	for _, token := range []string{"haiku", "mini", "flash", "nano"} {
		if strings.Contains(normalized, token) {
			return "fast"
		}
	}
	return ""
}

func advisorTierRank(tier string) int {
	switch tier {
	case "premium":
		return 0
	case "balanced":
		return 1
	case "fast":
		return 2
	default:
		return 3
	}
}

func advisorModelFamily(modelID string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	switch {
	case strings.Contains(normalized, "claude"):
		return "claude"
	case strings.Contains(normalized, "gpt"):
		return "gpt"
	case strings.Contains(normalized, "gemini"):
		return "gemini"
	case strings.Contains(normalized, "qwen"):
		return "qwen"
	case strings.Contains(normalized, "glm"):
		return "glm"
	case strings.Contains(normalized, "o3"), strings.Contains(normalized, "o4"):
		return "openai-reasoning"
	default:
		return normalized
	}
}

func advisorEvidenceConflict(evidence []advisorEvidenceItem) bool {
	seen := map[string]struct{}{}
	for _, item := range evidence {
		hostish := strings.TrimSpace(item.Label + "|" + item.URL)
		if hostish == "" {
			continue
		}
		if _, ok := seen[hostish]; ok {
			return true
		}
		seen[hostish] = struct{}{}
	}
	return false
}

func normalizeAdvisorCategory(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "architecture", "language", "framework", "library", "process", "migration", "ops":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func normalizeAdvisorDecisionMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "recommend", "compare", "review", "replace", "best_practice":
		return strings.ToLower(strings.TrimSpace(raw))
	case "best-practice", "best practice":
		return "best_practice"
	default:
		return ""
	}
}

func inferAdvisorCategory(args map[string]interface{}) string {
	question := strings.ToLower(strings.TrimSpace(firstCompatString(args, "question")))
	switch {
	case strings.Contains(question, "go"), strings.Contains(question, "python"), strings.Contains(question, "node"), strings.Contains(question, "language"):
		return "language"
	case strings.Contains(question, "gorm"), strings.Contains(question, "xorm"), strings.Contains(question, "zorm"), strings.Contains(question, "lib"), strings.Contains(question, "library"):
		return "library"
	case strings.Contains(question, "ddd"), strings.Contains(question, "tdd"), strings.Contains(question, "refactor"), strings.Contains(question, "process"):
		return "process"
	case strings.Contains(question, "migrate"), strings.Contains(question, "migration"), strings.Contains(question, "replace"), strings.Contains(question, "替换"), strings.Contains(question, "迁移"):
		return "migration"
	case strings.Contains(question, "deploy"), strings.Contains(question, "ops"), strings.Contains(question, "availability"):
		return "ops"
	case strings.Contains(question, "architecture"), strings.Contains(question, "架构"):
		return "architecture"
	default:
		return ""
	}
}

func inferAdvisorDecisionMode(args map[string]interface{}) string {
	question := strings.ToLower(strings.TrimSpace(firstCompatString(args, "question")))
	switch {
	case len(advisorCollectStrings(args["candidates"])) >= 2:
		return "compare"
	case strings.Contains(question, "replace"), strings.Contains(question, "替换"), strings.Contains(question, "替代"), strings.Contains(question, "migrate"), strings.Contains(question, "迁移"):
		return "replace"
	case strings.Contains(question, "best practice"), strings.Contains(question, "最佳实践"), strings.Contains(question, "tdd"), strings.Contains(question, "ddd"), strings.Contains(question, "refactor"):
		return "best_practice"
	case strings.Contains(question, "compare"), strings.Contains(question, "vs"), strings.Contains(question, "比较"), strings.Contains(question, "对比"):
		return "compare"
	default:
		return "recommend"
	}
}

func advisorCollectStrings(raw interface{}) []string {
	switch typed := raw.(type) {
	case nil:
		return nil
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return nil
		}
		if strings.Contains(trimmed, ",") {
			parts := strings.Split(trimmed, ",")
			out := make([]string, 0, len(parts))
			for _, part := range parts {
				if normalized := strings.TrimSpace(part); normalized != "" {
					out = append(out, normalized)
				}
			}
			return uniqueStrings(out)
		}
		return []string{trimmed}
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if normalized := strings.TrimSpace(item); normalized != "" {
				out = append(out, normalized)
			}
		}
		return uniqueStrings(out)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if normalized := strings.TrimSpace(fmt.Sprint(item)); normalized != "" && normalized != "<nil>" {
				out = append(out, normalized)
			}
		}
		return uniqueStrings(out)
	default:
		normalized := strings.TrimSpace(fmt.Sprint(raw))
		if normalized == "" || normalized == "<nil>" {
			return nil
		}
		return []string{normalized}
	}
}

func advisorContextMap(args map[string]interface{}) map[string]interface{} {
	if existing, ok := coerceCompatMap(args["context"]); ok && existing != nil {
		return advisorCloneMap(existing)
	}
	return map[string]interface{}{}
}

func advisorConstraintsMap(args map[string]interface{}) map[string]interface{} {
	if existing, ok := coerceCompatMap(args["constraints"]); ok && existing != nil {
		return advisorCloneMap(existing)
	}
	return map[string]interface{}{}
}

func advisorHasDecisionConstraint(contextMap, constraintsMap map[string]interface{}) bool {
	for _, key := range []string{"budget_level", "latency_slo", "traffic_qps", "deployment", "compliance"} {
		if advisorTextValue(contextMap[key]) != "" || advisorTextValue(constraintsMap[key]) != "" {
			return true
		}
	}
	if len(advisorCollectStrings(contextMap["must_have"])) > 0 || len(advisorCollectStrings(contextMap["must_avoid"])) > 0 {
		return true
	}
	return len(constraintsMap) > 0
}

func advisorHasOpsConstraint(contextMap, constraintsMap map[string]interface{}) bool {
	for _, key := range []string{"latency_slo", "budget_level", "traffic_qps", "compliance"} {
		if advisorTextValue(contextMap[key]) != "" || advisorTextValue(constraintsMap[key]) != "" {
			return true
		}
	}
	return len(advisorCollectStrings(contextMap["must_have"])) > 0 || len(advisorCollectStrings(contextMap["must_avoid"])) > 0
}

func advisorNormalizeStringAlias(args map[string]interface{}, canonical string, aliases ...string) {
	if args == nil {
		return
	}
	if value := strings.TrimSpace(firstCompatString(args, canonical)); value != "" {
		args[canonical] = value
		return
	}
	for _, alias := range aliases {
		raw, ok := args[alias]
		if !ok {
			continue
		}
		if value := strings.TrimSpace(fmt.Sprint(raw)); value != "" && value != "<nil>" {
			args[canonical] = value
			return
		}
	}
}

func advisorNormalizeContextValue(raw interface{}) interface{} {
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []string:
		return advisorCollectStrings(typed)
	case []interface{}:
		return advisorCollectStrings(typed)
	default:
		return raw
	}
}

func advisorCloneMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func firstNonEmptyAdvisorValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func advisorTextValue(v interface{}) string {
	value := strings.TrimSpace(fmt.Sprint(v))
	if value == "" || value == "<nil>" {
		return ""
	}
	return value
}

func formatValue(v interface{}) string {
	switch v.(type) {
	case string, float64, bool:
		return fmt.Sprintf("%v", v)
	default:
		if v == nil {
			return ""
		}
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
