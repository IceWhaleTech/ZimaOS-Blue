package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

type ProposalReflector interface {
	Reflect(ctx context.Context, input selfreflect.Input) (*selfreflect.Result, error)
}

type JudgeEvaluator interface {
	Evaluate(ctx context.Context, req JudgeEvaluationRequest) (*JudgeEvaluationResult, error)
}

type JudgeEvaluationRequest struct {
	Model        string
	Group        *RunGroup
	Item         *RunGroupItem
	Run          *Run
	Calibration  map[string]interface{}
	Verification *HarnessVerificationResult
}

type JudgeEvaluationResult struct {
	Backend string
	Model   string
	Verdict ScoreVerdict
	Score   float64
	Reason  string
	Trace   map[string]interface{}
}

type judgeLLMCaller interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type LLMJudgeEvaluator struct {
	llm judgeLLMCaller
}

func NewLLMJudgeEvaluator(llmCaller judgeLLMCaller) *LLMJudgeEvaluator {
	return &LLMJudgeEvaluator{llm: llmCaller}
}

func (e *LLMJudgeEvaluator) Evaluate(ctx context.Context, req JudgeEvaluationRequest) (*JudgeEvaluationResult, error) {
	if e == nil || e.llm == nil {
		return nil, fmt.Errorf("judge evaluator is not configured")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		return nil, fmt.Errorf("judge model is required")
	}
	resp, err := e.llm.Chat(ctx, llm.ChatRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: buildJudgeSystemPrompt()},
			{Role: llm.RoleUser, Content: buildJudgeUserPrompt(req)},
		},
		MaxTokens:   400,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Score   float64                `json:"score"`
		Verdict ScoreVerdict           `json:"verdict"`
		Reason  string                 `json:"reason"`
		Trace   map[string]interface{} `json:"trace"`
	}
	content := strings.TrimSpace(resp.Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("parse judge response: %w", err)
	}
	return &JudgeEvaluationResult{
		Backend: "evaluator",
		Model:   model,
		Verdict: parsed.Verdict,
		Score:   normalizeScore(parsed.Score),
		Reason:  strings.TrimSpace(parsed.Reason),
		Trace:   parsed.Trace,
	}, nil
}

func (c *Controller) SetJudgeEvaluator(evaluator JudgeEvaluator) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.judgeEvaluator = evaluator
}

func (c *Controller) SetReflector(reflector ProposalReflector) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reflector = reflector
}

func (c *Controller) integrations() (JudgeEvaluator, ProposalReflector) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.judgeEvaluator, c.reflector
}

func buildJudgeSystemPrompt() string {
	return strings.TrimSpace(`You are an evaluation judge for ZimaOS Blue harness runs.

Return only one JSON object:
{
  "score": <number between 0 and 1>,
  "verdict": "pass|partial|fail|error",
  "reason": "<brief grounded rationale>",
  "trace": {
    "strengths": ["..."],
    "risks": ["..."]
  }
}

Rules:
- Judge the run output against the expected result and profile.
- Penalize unsupported claims, empty results, and safety failures.
- Use "fail" when the answer materially misses or contradicts expectations.
- Use "partial" when the result is somewhat useful but incomplete.
- Use "error" only when the run itself failed or produced no evaluable output.
- Keep the rationale concise and evidence-based.`)
}

func buildJudgeUserPrompt(req JudgeEvaluationRequest) string {
	payload := map[string]interface{}{
		"group_subject": req.groupSubject(),
		"profile":       req.itemProfile(),
		"input":         req.itemInput(),
		"expected":      req.itemExpected(),
		"item_metadata": req.itemMetadata(),
		"run": map[string]interface{}{
			"status": req.runStatus(),
			"result": req.runResult(),
			"error":  req.runError(),
		},
	}
	if workspaceSummary := buildJudgeWorkspaceSummary(req.runWorkspaceRoot()); len(workspaceSummary) > 0 {
		payload["workspace_summary"] = workspaceSummary
	}
	if len(req.Calibration) > 0 {
		payload["calibration"] = req.Calibration
	}
	if req.Verification != nil {
		payload["verification"] = verificationPayload(req.Verification)
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}

func (r JudgeEvaluationRequest) groupSubject() string {
	if r.Group == nil {
		return ""
	}
	return strings.TrimSpace(r.Group.Subject)
}

func (r JudgeEvaluationRequest) itemProfile() string {
	if r.Item == nil {
		return ""
	}
	return strings.TrimSpace(r.Item.Profile)
}

func (r JudgeEvaluationRequest) itemInput() map[string]interface{} {
	if r.Item == nil {
		return nil
	}
	return r.Item.Input
}

func (r JudgeEvaluationRequest) itemExpected() map[string]interface{} {
	if r.Item == nil {
		return nil
	}
	return r.Item.Expected
}

func (r JudgeEvaluationRequest) itemMetadata() map[string]interface{} {
	if r.Item == nil {
		return nil
	}
	return cloneMap(r.Item.Metadata)
}

func (r JudgeEvaluationRequest) runStatus() string {
	if r.Run == nil {
		return ""
	}
	return string(r.Run.Status)
}

func (r JudgeEvaluationRequest) runResult() string {
	if r.Run == nil {
		return ""
	}
	return strings.TrimSpace(r.Run.Result)
}

func (r JudgeEvaluationRequest) runError() string {
	if r.Run == nil {
		return ""
	}
	return strings.TrimSpace(r.Run.Error)
}

func (r JudgeEvaluationRequest) runWorkspaceRoot() string {
	if r.Run == nil {
		return ""
	}
	return strings.TrimSpace(r.Run.WorkspaceRoot)
}

func buildJudgeWorkspaceSummary(workspaceRoot string) map[string]interface{} {
	workspaceRoot = strings.TrimSpace(workspaceRoot)
	if workspaceRoot == "" {
		return nil
	}
	entries := make([]map[string]interface{}, 0, 8)
	_ = filepath.Walk(workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if len(entries) >= 8 {
			return filepath.SkipAll
		}
		rel, relErr := filepath.Rel(workspaceRoot, path)
		if relErr != nil {
			return nil
		}
		entry := map[string]interface{}{
			"path": rel,
			"size": info.Size(),
		}
		if blob, readErr := os.ReadFile(path); readErr == nil {
			if text, ok := judgeWorkspaceSnippet(blob); ok {
				entry["snippet"] = text
			} else {
				entry["binary"] = true
			}
		}
		entries = append(entries, entry)
		return nil
	})
	if len(entries) == 0 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		return fmt.Sprint(entries[i]["path"]) < fmt.Sprint(entries[j]["path"])
	})
	return map[string]interface{}{
		"root":  workspaceRoot,
		"files": entries,
	}
}

func judgeWorkspaceSnippet(blob []byte) (string, bool) {
	if len(blob) == 0 {
		return "", true
	}
	for _, b := range blob {
		if b == 0 {
			return "", false
		}
	}
	text := strings.TrimSpace(string(blob))
	if text == "" {
		return "", true
	}
	if len(text) > 400 {
		text = text[:400]
	}
	return text, true
}
