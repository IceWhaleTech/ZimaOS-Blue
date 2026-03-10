package selfreflect

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
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
	TaskID             string `json:"task_id,omitempty"`
	Goal               string `json:"goal"`
	Plan               []Step `json:"plan,omitempty"`
	VerificationOutput string `json:"verification_output,omitempty"`
	FinalStatus        string `json:"final_status,omitempty"`
	ResultSummary      string `json:"result_summary,omitempty"`
	FailureReason      string `json:"failure_reason,omitempty"`
}

type Lesson struct {
	Kind        LessonKind `json:"kind"`
	Lesson      string     `json:"lesson"`
	WhenToApply string     `json:"when_to_apply"`
	Evidence    string     `json:"evidence"`
}

type Result struct {
	Summary       string   `json:"summary"`
	Lessons       []Lesson `json:"lessons,omitempty"`
	MemoryWritten int      `json:"memory_written"`
	SkippedReason string   `json:"skipped_reason,omitempty"`
}

type Service struct {
	llm        LLMCaller
	mu         sync.RWMutex
	writer     MemoryWriter
	maxLessons int
}

func NewService(llmCaller LLMCaller, writer MemoryWriter) *Service {
	return &Service{llm: llmCaller, writer: writer, maxLessons: 3}
}

func (s *Service) SetMemoryWriter(writer MemoryWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writer = writer
}

func (s *Service) Reflect(ctx context.Context, input Input) (*Result, error) {
	input = normalizeInput(input)
	if input.Goal == "" && input.ResultSummary == "" && input.FailureReason == "" {
		return &Result{SkippedReason: "reflection requires goal, result_summary, or failure_reason"}, nil
	}

	result, err := s.generateReflection(ctx, input)
	if err != nil {
		return nil, err
	}

	result.Summary = strings.TrimSpace(result.Summary)
	result.Lessons = filterLessons(result.Lessons, input, s.maxLessons)
	if len(result.Lessons) == 0 {
		if result.Summary == "" {
			result.Summary = fallbackSummary(input)
		}
		result.SkippedReason = "no grounded lessons passed quality filters"
		return result, nil
	}

	if result.Summary == "" {
		result.Summary = fallbackSummary(input)
	}

	writer := s.memoryWriter()
	if writer == nil {
		return result, nil
	}
	for _, lesson := range result.Lessons {
		if err := writer.Write(ctx, formatMemoryEntry(lesson), buildMemoryTags(input, lesson.Kind)); err == nil {
			result.MemoryWritten++
		}
	}
	return result, nil
}

func (s *Service) memoryWriter() MemoryWriter {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.writer
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
	input.FinalStatus = normalizeStatus(input.FinalStatus)
	for i := range input.Plan {
		input.Plan[i].Description = strings.TrimSpace(input.Plan[i].Description)
		input.Plan[i].Status = normalizeStatus(input.Plan[i].Status)
		input.Plan[i].Output = strings.TrimSpace(input.Plan[i].Output)
	}
	return input
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
