package selfreflect

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

type LLMCaller interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type MemoryWriter interface {
	Write(ctx context.Context, content string, tags []string) error
}

type LessonKind string

const (
	LessonKindHeuristic   LessonKind = "heuristic"
	LessonKindAntiPattern LessonKind = "anti_pattern"
	LessonKindGuardrail   LessonKind = "guardrail"
)

type Step struct {
	Description string `json:"description"`
	Status      string `json:"status,omitempty"`
	Output      string `json:"output,omitempty"`
}

type Input struct {
	TaskID             string                 `json:"task_id,omitempty"`
	Goal               string                 `json:"goal"`
	Plan               []Step                 `json:"plan,omitempty"`
	VerificationOutput string                 `json:"verification_output,omitempty"`
	FinalStatus        string                 `json:"final_status,omitempty"`
	ResultSummary      string                 `json:"result_summary,omitempty"`
	FailureReason      string                 `json:"failure_reason,omitempty"`
	OwnerUserID        string                 `json:"owner_user_id,omitempty"`
	SourceKind         string                 `json:"source_kind,omitempty"`
	SourceID           string                 `json:"source_id,omitempty"`
	EvaluationSummary  map[string]interface{} `json:"evaluation_summary,omitempty"`
	ProposalCandidates []ProposalCandidate    `json:"proposal_candidates,omitempty"`
	ProposalMode       ProposalMode           `json:"proposal_mode,omitempty"`
}

type Lesson struct {
	Kind        LessonKind `json:"kind"`
	Lesson      string     `json:"lesson"`
	WhenToApply string     `json:"when_to_apply"`
	Evidence    string     `json:"evidence"`
}

type Result struct {
	Summary              string   `json:"summary"`
	Lessons              []Lesson `json:"lessons,omitempty"`
	MemoryWritten        int      `json:"memory_written"`
	SkippedReason        string   `json:"skipped_reason,omitempty"`
	ProposalCount        int      `json:"proposal_count,omitempty"`
	ProposalIDs          []string `json:"proposal_ids,omitempty"`
	ProposalSkippedReason string  `json:"proposal_skipped_reason,omitempty"`
}

type Service struct {
	llm           LLMCaller
	mu            sync.RWMutex
	writer        MemoryWriter
	proposalStore ProposalStore
	workspaceMgr  *workspace.Manager
	proposalGate  func() bool
	maxLessons    int
}

func NewService(llmCaller LLMCaller, writer MemoryWriter) *Service {
	return &Service{llm: llmCaller, writer: writer, maxLessons: 3}
}

func (s *Service) SetLLMCaller(llmCaller LLMCaller) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.llm = llmCaller
}

func (s *Service) SetMemoryWriter(writer MemoryWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writer = writer
}

func (s *Service) SetProposalStore(store ProposalStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proposalStore = store
}

func (s *Service) SetWorkspaceManager(mgr *workspace.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspaceMgr = mgr
}

func (s *Service) SetProposalGateFunc(fn func() bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.proposalGate = fn
}

func (s *Service) Reflect(ctx context.Context, input Input) (*Result, error) {
	input = normalizeInput(input)
	hasReflectionRecord := input.Goal != "" || input.ResultSummary != "" || input.FailureReason != ""
	if !hasReflectionRecord && len(input.ProposalCandidates) == 0 {
		return &Result{SkippedReason: "reflection requires goal, result_summary, or failure_reason"}, nil
	}

	result := &Result{}
	if hasReflectionRecord {
		parsed, err := s.generateReflection(ctx, input)
		if err != nil {
			return nil, err
		}
		result = parsed
		result.Summary = strings.TrimSpace(result.Summary)
		result.Lessons = filterLessons(result.Lessons, input, s.maxLessons)
		if len(result.Lessons) == 0 {
			if result.Summary == "" {
				result.Summary = fallbackSummary(input)
			}
			result.SkippedReason = "no grounded lessons passed quality filters"
		} else {
			if result.Summary == "" {
				result.Summary = fallbackSummary(input)
			}
			writer := s.memoryWriter()
			if writer != nil {
				for _, lesson := range result.Lessons {
					if err := writer.Write(ctx, formatMemoryEntry(lesson), buildMemoryTags(input, lesson.Kind)); err == nil {
						result.MemoryWritten++
					}
				}
			}
		}
	}

	proposalCount, proposalIDs, proposalSkippedReason, err := s.intakeProposals(ctx, input)
	if err != nil {
		return nil, err
	}
	result.ProposalCount = proposalCount
	result.ProposalIDs = proposalIDs
	result.ProposalSkippedReason = proposalSkippedReason

	if !hasReflectionRecord && result.ProposalCount == 0 && result.ProposalSkippedReason == "" {
		result.SkippedReason = "reflection requires goal, result_summary, or failure_reason"
	}
	return result, nil
}

func (s *Service) memoryWriter() MemoryWriter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.writer
}

func (s *Service) proposalDependencies() (ProposalStore, *workspace.Manager, func() bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.proposalStore, s.workspaceMgr, s.proposalGate
}

func (s *Service) generateReflection(ctx context.Context, input Input) (*Result, error) {
	if s == nil || s.llm == nil {
		return &Result{Summary: fallbackSummary(input), SkippedReason: "reflection backend not available"}, nil
	}
	resp, err := s.llm.Chat(ctx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: buildReflectionSystemPrompt()},
			{Role: llm.RoleUser, Content: buildReflectionUserPrompt(input)},
		},
		MaxTokens:   700,
		Temperature: 0.2,
	})
	if err != nil {
		return nil, err
	}

	parsed, err := parseReflectionResult(resp.Message.Content)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func (s *Service) ListProposals(ctx context.Context, filter ProposalFilter) ([]Proposal, error) {
	store, _, _ := s.proposalDependencies()
	if store == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	return store.ListProposals(ctx, filter)
}

func (s *Service) GetProposal(ctx context.Context, id string) (*Proposal, error) {
	store, _, _ := s.proposalDependencies()
	if store == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	return store.GetProposal(ctx, strings.TrimSpace(id))
}

func (s *Service) ReviewProposal(ctx context.Context, id string, status ProposalStatus, reviewNote string) (*Proposal, error) {
	if status != ProposalStatusApproved && status != ProposalStatusRejected {
		return nil, fmt.Errorf("invalid proposal status %q", status)
	}
	store, _, _ := s.proposalDependencies()
	if store == nil {
		return nil, fmt.Errorf("proposal store is not configured")
	}
	proposal, err := store.GetProposal(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	now := timeutil.NowTime()
	proposal.Status = status
	proposal.ReviewNote = strings.TrimSpace(reviewNote)
	proposal.ReviewedAt = &now
	if err := store.UpdateProposal(ctx, proposal); err != nil {
		return nil, err
	}
	return store.GetProposal(ctx, proposal.ID)
}

func (s *Service) intakeProposals(ctx context.Context, input Input) (int, []string, string, error) {
	if len(input.ProposalCandidates) == 0 {
		return 0, nil, "", nil
	}
	store, mgr, gate := s.proposalDependencies()
	if gate != nil && !gate() {
		return 0, nil, "agent_auto_reflect is disabled", nil
	}
	if store == nil {
		return 0, nil, "proposal store is not configured", nil
	}
	if mgr == nil {
		return 0, nil, "workspace manager is not configured", nil
	}
	if mode := normalizeProposalMode(input.ProposalMode); mode != ProposalModeReviewOnly {
		return 0, nil, "unsupported proposal_mode", nil
	}

	createdIDs := make([]string, 0, len(input.ProposalCandidates))
	skippedReasons := make([]string, 0, len(input.ProposalCandidates))
	for _, candidate := range input.ProposalCandidates {
		proposal, skipReason, err := s.buildProposal(ctx, store, mgr, input, candidate)
		if err != nil {
			return len(createdIDs), createdIDs, strings.Join(dedupeStrings(skippedReasons), "; "), err
		}
		if proposal == nil {
			if strings.TrimSpace(skipReason) != "" {
				skippedReasons = append(skippedReasons, skipReason)
			}
			continue
		}
		if err := store.CreateProposal(ctx, proposal); err != nil {
			return len(createdIDs), createdIDs, strings.Join(dedupeStrings(skippedReasons), "; "), err
		}
		createdIDs = append(createdIDs, proposal.ID)
	}
	if len(createdIDs) == 0 && len(skippedReasons) == 0 {
		skippedReasons = append(skippedReasons, "no eligible proposal candidates")
	}
	return len(createdIDs), createdIDs, strings.Join(dedupeStrings(skippedReasons), "; "), nil
}

func (s *Service) buildProposal(ctx context.Context, store ProposalStore, mgr *workspace.Manager, input Input, candidate ProposalCandidate) (*Proposal, string, error) {
	candidate = normalizeProposalCandidate(candidate)
	if candidate.Lesson == "" || candidate.Evidence == "" {
		return nil, "candidate missing lesson or evidence", nil
	}
	targetFile := strings.TrimSpace(candidate.TargetFile)
	if targetFile == "" {
		targetFile = proposalTargetFile
	}
	if targetFile != proposalTargetFile {
		return nil, "proposal target is restricted to AGENTS.md", nil
	}
	if len(candidate.EvidenceIDs) == 0 {
		return nil, "candidate is not evidence-traceable", nil
	}
	dedupKey := normalizeDedupKey(candidate.Lesson)
	if dedupKey == "" {
		return nil, "candidate lesson did not produce a stable dedup key", nil
	}
	existing, err := store.FindProposalByDedup(ctx, dedupKey, targetFile)
	switch {
	case err == nil && existing != nil:
		return nil, "duplicate proposal candidate", nil
	case err != nil && err != sql.ErrNoRows:
		return nil, "", err
	}

	proposal := &Proposal{
		OwnerUserID:        strings.TrimSpace(input.OwnerUserID),
		SourceKind:         strings.TrimSpace(input.SourceKind),
		SourceID:           strings.TrimSpace(input.SourceID),
		ProposalMode:       ProposalModeReviewOnly,
		TargetFile:         targetFile,
		TargetSection:      proposalTargetSection,
		Status:             ProposalStatusPending,
		DedupKey:           dedupKey,
		Lesson:             candidate.Lesson,
		WhenToApply:        candidate.WhenToApply,
		Evidence:           candidate.Evidence,
		EvidenceIDs:        append([]string(nil), candidate.EvidenceIDs...),
		EvaluationSummary:  cloneInterfaceMap(input.EvaluationSummary),
		CalibrationSummary: extractCalibrationSummary(input.EvaluationSummary),
	}
	proposal.PatchPreview = renderProposalPatchPreview(mgr, proposal)
	if proposal.PatchPreview == "" {
		return nil, "failed to render patch preview", nil
	}
	return proposal, "", nil
}

func buildReflectionSystemPrompt() string {
	return strings.TrimSpace(`You are a post-task reflection engine for ZimaOS Blue.

## Output Contract
Return ONLY one JSON object in this exact shape:
{
  "summary": "<1-2 concise sentences>",
  "lessons": [
    {
      "kind": "heuristic|anti_pattern|guardrail",
      "lesson": "<short reusable lesson>",
      "when_to_apply": "<when this lesson applies>",
      "evidence": "<specific evidence from the task record>"
    }
  ]
}

## Reflection Rules
- Extract 0-5 grounded, reusable lessons from the task record.
- Prefer concrete heuristics over generic advice.
- No self-praise, self-blame, motivational language, or vague best-practice slogans.
- Each lesson must be supported by evidence from the task record.
- Keep each field concise and operational.
- Use "heuristic" for tactics that worked, "anti_pattern" for approaches that failed, and "guardrail" for checks that should be added next time.
- If the record is too weak, return an empty lessons array.

## Output Rules
- No markdown, no code fences, and no prose outside the JSON object.`)
}

func buildReflectionUserPrompt(input Input) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Goal: %s\n", input.Goal))
	if input.FinalStatus != "" {
		sb.WriteString(fmt.Sprintf("Final status: %s\n", input.FinalStatus))
	}
	if input.ResultSummary != "" {
		sb.WriteString("Result summary:\n")
		sb.WriteString(truncate(input.ResultSummary, 1200))
		sb.WriteString("\n")
	}
	if input.FailureReason != "" {
		sb.WriteString("Failure reason:\n")
		sb.WriteString(truncate(input.FailureReason, 800))
		sb.WriteString("\n")
	}
	if len(input.Plan) > 0 {
		sb.WriteString("\nTask record:\n")
		for i, step := range input.Plan {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, strings.ToUpper(strings.TrimSpace(step.Status)), step.Description))
			if step.Output != "" {
				sb.WriteString(": ")
				sb.WriteString(truncate(step.Output, 280))
			}
			sb.WriteString("\n")
		}
	}
	if input.VerificationOutput != "" {
		sb.WriteString("\nVerification output:\n")
		sb.WriteString(truncate(input.VerificationOutput, 1200))
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func parseReflectionResult(content string) (*Result, error) {
	trimmed := strings.TrimSpace(content)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	var out Result
	if err := json.Unmarshal([]byte(trimmed), &out); err != nil {
		return nil, fmt.Errorf("failed to parse reflection result: %w", err)
	}
	return &out, nil
}

func normalizeInput(input Input) Input {
	input.Goal = strings.TrimSpace(input.Goal)
	input.VerificationOutput = strings.TrimSpace(input.VerificationOutput)
	input.ResultSummary = strings.TrimSpace(input.ResultSummary)
	input.FailureReason = strings.TrimSpace(input.FailureReason)
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.OwnerUserID = strings.TrimSpace(input.OwnerUserID)
	input.SourceKind = strings.TrimSpace(input.SourceKind)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.FinalStatus = normalizeStatus(input.FinalStatus)
	input.ProposalMode = normalizeProposalMode(input.ProposalMode)
	for i := range input.Plan {
		input.Plan[i].Description = strings.TrimSpace(input.Plan[i].Description)
		input.Plan[i].Status = normalizeStatus(input.Plan[i].Status)
		input.Plan[i].Output = strings.TrimSpace(input.Plan[i].Output)
	}
	for i := range input.ProposalCandidates {
		input.ProposalCandidates[i] = normalizeProposalCandidate(input.ProposalCandidates[i])
	}
	return input
}

func normalizeProposalMode(mode ProposalMode) ProposalMode {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case "", string(ProposalModeReviewOnly):
		return ProposalModeReviewOnly
	default:
		return ProposalMode(strings.ToLower(strings.TrimSpace(string(mode))))
	}
}

func normalizeProposalCandidate(candidate ProposalCandidate) ProposalCandidate {
	candidate.Lesson = cleanSentence(candidate.Lesson)
	candidate.WhenToApply = cleanSentence(candidate.WhenToApply)
	candidate.Evidence = cleanSentence(candidate.Evidence)
	candidate.TargetFile = strings.TrimSpace(candidate.TargetFile)
	filteredEvidenceIDs := make([]string, 0, len(candidate.EvidenceIDs))
	for _, evidenceID := range candidate.EvidenceIDs {
		if id := strings.TrimSpace(evidenceID); id != "" {
			filteredEvidenceIDs = append(filteredEvidenceIDs, id)
		}
	}
	candidate.EvidenceIDs = filteredEvidenceIDs
	return candidate
}

func normalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "completed", "failed", "partial", "running", "pending", "skipped":
		return status
	default:
		return status
	}
}

func fallbackSummary(input Input) string {
	if input.FinalStatus == "failed" {
		return "The task ended in failure and the reflection engine extracted only grounded lessons from the recorded steps and verification output."
	}
	return "The task finished and the reflection engine extracted reusable lessons from the recorded steps and verification output."
}

var splitTokenRE = regexp.MustCompile(`[a-z0-9]{4,}`)

func filterLessons(in []Lesson, input Input, maxLessons int) []Lesson {
	if maxLessons <= 0 {
		maxLessons = 3
	}
	corpus := buildCorpus(input)
	corpusTokens := tokenSet(corpus)
	seen := map[string]struct{}{}
	filtered := make([]Lesson, 0, len(in))
	for _, lesson := range in {
		lesson.Kind = normalizeKind(lesson.Kind, input.FinalStatus)
		lesson.Lesson = cleanSentence(lesson.Lesson)
		lesson.WhenToApply = cleanSentence(lesson.WhenToApply)
		lesson.Evidence = cleanSentence(lesson.Evidence)
		if lesson.Lesson == "" || lesson.WhenToApply == "" || lesson.Evidence == "" {
			continue
		}
		if isGenericLesson(lesson) {
			continue
		}
		key := normalizeDedupKey(lesson.Lesson)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		if !isEvidenceGrounded(lesson.Evidence, corpus, corpusTokens) {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, lesson)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return lessonPriority(input.FinalStatus, filtered[i].Kind) < lessonPriority(input.FinalStatus, filtered[j].Kind)
	})
	if len(filtered) > maxLessons {
		filtered = filtered[:maxLessons]
	}
	return filtered
}

func normalizeKind(kind LessonKind, finalStatus string) LessonKind {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case string(LessonKindHeuristic):
		return LessonKindHeuristic
	case string(LessonKindAntiPattern):
		return LessonKindAntiPattern
	case string(LessonKindGuardrail):
		return LessonKindGuardrail
	default:
		if finalStatus == "failed" {
			return LessonKindGuardrail
		}
		return LessonKindHeuristic
	}
}

func lessonPriority(finalStatus string, kind LessonKind) int {
	if finalStatus == "failed" {
		switch kind {
		case LessonKindAntiPattern:
			return 0
		case LessonKindGuardrail:
			return 1
		default:
			return 2
		}
	}
	switch kind {
	case LessonKindHeuristic:
		return 0
	case LessonKindGuardrail:
		return 1
	default:
		return 2
	}
}

func buildCorpus(input Input) string {
	parts := []string{input.Goal, input.ResultSummary, input.FailureReason, input.VerificationOutput}
	for _, step := range input.Plan {
		parts = append(parts, step.Description, step.Output)
	}
	return strings.ToLower(strings.Join(parts, "\n"))
}

func tokenSet(text string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, token := range splitTokenRE.FindAllString(strings.ToLower(text), -1) {
		set[token] = struct{}{}
	}
	return set
}

func isEvidenceGrounded(evidence, corpus string, corpusTokens map[string]struct{}) bool {
	evidence = strings.ToLower(strings.TrimSpace(evidence))
	if len(evidence) < 12 {
		return false
	}
	if strings.Contains(corpus, evidence) {
		return true
	}
	for _, token := range splitTokenRE.FindAllString(evidence, -1) {
		if _, ok := corpusTokens[token]; ok {
			return true
		}
	}
	return false
}

func isGenericLesson(lesson Lesson) bool {
	joined := strings.ToLower(strings.Join([]string{lesson.Lesson, lesson.WhenToApply, lesson.Evidence}, " "))
	for _, bad := range []string{
		"be more careful",
		"communicate clearly",
		"follow best practices",
		"pay attention",
		"do better next time",
		"be systematic",
		"test thoroughly",
		"avoid mistakes",
		"be cautious",
		"stay focused",
		"be proactive",
	} {
		if strings.Contains(joined, bad) {
			return true
		}
	}
	if len(splitTokenRE.FindAllString(strings.ToLower(lesson.Lesson), -1)) < 3 {
		return true
	}
	return false
}

func cleanSentence(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, " \n\t-•")
	return value
}

func normalizeDedupKey(value string) string {
	value = strings.ToLower(cleanSentence(value))
	replacer := strings.NewReplacer(",", "", ".", "", ":", "", ";", "", "!", "", "?", "", "-", " ")
	value = replacer.Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func formatMemoryEntry(lesson Lesson) string {
	return fmt.Sprintf("[%s] %s\nWhen to apply: %s\nEvidence: %s", lesson.Kind, lesson.Lesson, lesson.WhenToApply, lesson.Evidence)
}

func buildMemoryTags(input Input, kind LessonKind) []string {
	taskID := input.TaskID
	if taskID == "" {
		taskID = "manual"
	}
	status := input.FinalStatus
	if status == "" {
		status = "completed"
	}
	return []string{
		"agent_reflection",
		"self_reflect",
		"task:" + taskID,
		"status:" + status,
		"kind:" + string(kind),
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func cloneInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func extractCalibrationSummary(summary map[string]interface{}) map[string]interface{} {
	if len(summary) == 0 {
		return nil
	}
	if raw, ok := summary["calibration"].(map[string]interface{}); ok && len(raw) > 0 {
		return cloneInterfaceMap(raw)
	}
	out := map[string]interface{}{}
	for _, key := range []string{
		"coverage",
		"groundedness",
		"freshness",
		"conflict_risk",
		"confidence",
		"recommended_action",
		"calibration_ref",
	} {
		if value, ok := summary[key]; ok {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, item := range in {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
