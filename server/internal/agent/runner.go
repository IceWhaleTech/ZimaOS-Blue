package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// MaxConcurrentTasks is the default limit of concurrent agent tasks per user.
const MaxConcurrentTasks = 3

// MaxToolRoundsPerStep is the max tool rounds within a single plan step.
const MaxToolRoundsPerStep = 50

// LLMCaller abstracts LLM calls so the runner doesn't depend on proxybridge directly.
type LLMCaller interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

// MemoryRecaller abstracts memory recall so the runner doesn't depend on memory package directly.
type MemoryRecaller interface {
	Recall(ctx context.Context, query string, limit int) ([]MemoryResult, error)
}

// MemoryResult is a simplified memory search result for the agent runner.
type MemoryResult struct {
	Content string
	Score   float32
}

// RunnerConfig configures the agent runner.
type RunnerConfig struct {
	MaxConcurrent int
	TaskTimeout   time.Duration
}

// Runner executes agent tasks in the background.
type Runner struct {
	store    *Store
	llm      LLMCaller
	executor *tools.Executor
	registry *tools.Registry
	broker   *sse.Broker
	memory   MemoryRecaller
	config   RunnerConfig

	mu       sync.Mutex
	running  map[string]context.CancelFunc // task ID → cancel
	wg       sync.WaitGroup               // tracks background goroutines

	msgMu     sync.Mutex
	msgQueues map[string][]string // task ID → queued user messages

	askMu     sync.Mutex
	askQueues map[string]chan []QuestionAnswer // task ID → pending answer channel
}

// NewRunner creates a new agent runner.
func NewRunner(store *Store, llmCaller LLMCaller, registry *tools.Registry, executor *tools.Executor, broker *sse.Broker, config RunnerConfig) *Runner {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = MaxConcurrentTasks
	}
	if config.TaskTimeout <= 0 {
		config.TaskTimeout = 30 * time.Minute
	}
	return &Runner{
		store:     store,
		llm:       llmCaller,
		registry:  registry,
		executor:  executor,
		broker:    broker,
		config:    config,
		running:   make(map[string]context.CancelFunc),
		msgQueues: make(map[string][]string),
		askQueues: make(map[string]chan []QuestionAnswer),
	}
}

// SetMemory sets the memory recaller for context injection.
func (r *Runner) SetMemory(m MemoryRecaller) {
	r.memory = m
}

// Submit creates and starts a new agent task. Returns the task immediately.
func (r *Runner) Submit(ctx context.Context, userID, goal, conversationID, conversationCtx string) (*Task, error) {
	// Check concurrency limit
	running, err := r.store.CountRunning(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check running tasks: %w", err)
	}
	if running >= r.config.MaxConcurrent {
		return nil, fmt.Errorf("max concurrent tasks reached (%d)", r.config.MaxConcurrent)
	}

	task := &Task{
		ID:             uuid.New().String(),
		UserID:         userID,
		ConversationID: conversationID,
		Goal:           goal,
		Status:         TaskStatusPending,
	}

	if err := r.store.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Publish creation event
	r.publishEvent(userID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_created",
		Message:   goal,
	})

	// Start background execution
	taskCtx, cancel := context.WithTimeout(context.Background(), r.config.TaskTimeout)
	r.mu.Lock()
	r.running[task.ID] = cancel
	r.mu.Unlock()

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		r.execute(taskCtx, task, conversationCtx)
	}()

	return task, nil
}

// Cancel cancels a running task.
func (r *Runner) Cancel(taskID string) bool {
	r.mu.Lock()
	cancel, ok := r.running[taskID]
	r.mu.Unlock()
	if ok {
		cancel()
		return true
	}
	return false
}

// Shutdown cancels all running tasks and waits for goroutines to exit.
func (r *Runner) Shutdown() {
	r.mu.Lock()
	for _, cancel := range r.running {
		cancel()
	}
	r.mu.Unlock()
	r.wg.Wait()
}

// EnqueueMessage adds a user message to a running task's queue.
// Returns false if the task is not running.
func (r *Runner) EnqueueMessage(taskID, message string) bool {
	r.mu.Lock()
	_, running := r.running[taskID]
	r.mu.Unlock()
	if !running {
		return false
	}
	r.msgMu.Lock()
	r.msgQueues[taskID] = append(r.msgQueues[taskID], message)
	r.msgMu.Unlock()
	return true
}

// drainMessages returns and clears all queued messages for a task.
func (r *Runner) drainMessages(taskID string) []string {
	r.msgMu.Lock()
	defer r.msgMu.Unlock()
	msgs := r.msgQueues[taskID]
	if len(msgs) > 0 {
		r.msgQueues[taskID] = nil
	}
	return msgs
}

// AskUser sends questions to the user and blocks until answers are received or ctx is cancelled.
// Sets task status to waiting_input while blocked, restores to executing after.
func (r *Runner) AskUser(ctx context.Context, taskID string, questions []AgentQuestion, stepIndex int) ([]QuestionAnswer, error) {
	ch := make(chan []QuestionAnswer, 1)
	r.askMu.Lock()
	r.askQueues[taskID] = ch
	r.askMu.Unlock()

	defer func() {
		r.askMu.Lock()
		delete(r.askQueues, taskID)
		r.askMu.Unlock()
		// Restore status to executing
		_ = r.store.SetStatus(context.Background(), taskID, TaskStatusExecuting, "")
	}()

	// Look up userID from running task
	task, err := r.store.Get(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}

	// Set status to waiting_input
	_ = r.store.SetStatus(ctx, taskID, TaskStatusWaitingInput, "")

	// Publish question event via SSE
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    taskID,
		EventType: "task_question",
		StepIndex: stepIndex,
		Message:   "Waiting for user input...",
		Questions: questions,
	})

	// Block with a 10-minute timeout for user response
	askTimeout := 10 * time.Minute
	timer := time.NewTimer(askTimeout)
	defer timer.Stop()

	select {
	case answers := <-ch:
		return answers, nil
	case <-timer.C:
		return nil, fmt.Errorf("ask_user timed out after %v — no user response", askTimeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SubmitAnswers delivers user answers to a pending AskUser call.
// Returns false if no question is pending for this task.
func (r *Runner) SubmitAnswers(taskID string, answers []QuestionAnswer) bool {
	r.askMu.Lock()
	ch, ok := r.askQueues[taskID]
	r.askMu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- answers:
		// Publish answered event
		if task, err := r.store.Get(context.Background(), taskID); err == nil {
			r.publishEvent(task.UserID, TaskEvent{
				TaskID:    taskID,
				EventType: "task_question_answered",
				Message:   fmt.Sprintf("User answered %d question(s)", len(answers)),
			})
		}
		return true
	default:
		return false
	}
}

// handleAskUser parses the ask_user tool arguments, blocks for user answers, and returns the result as JSON.
// Supports both the new sq/mq/a format and the legacy questions array format.
func (r *Runner) handleAskUser(ctx context.Context, task *Task, argsJSON string) string {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		return `{"error":"invalid ask_user arguments: ` + err.Error() + `"}`
	}

	var questions []AgentQuestion

	// Try new sq/mq/a format first
	sq, _ := raw["sq"].(string)
	mq, _ := raw["mq"].(string)
	qText := sq
	multiSelect := false
	if mq != "" {
		qText = mq
		multiSelect = true
	}
	if qText != "" {
		q := AgentQuestion{
			ID:          "q0",
			Question:    qText,
			Header:      truncateRunes(qText, 12),
			MultiSelect: multiSelect,
		}
		if aRaw, ok := raw["a"]; ok {
			if arr, ok := aRaw.([]interface{}); ok {
				for _, item := range arr {
					if s, ok := item.(string); ok && s != "" {
						q.Options = append(q.Options, QuestionOption{Label: s, Value: s})
					}
				}
			}
		}
		questions = []AgentQuestion{q}
	} else {
		// Legacy: questions array
		var req struct {
			Questions []AgentQuestion `json:"questions"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &req); err != nil || len(req.Questions) == 0 {
			return `{"error":"invalid ask_user arguments: sq/mq or questions array required"}`
		}
		questions = req.Questions
	}

	// Assign IDs if missing
	for i := range questions {
		if questions[i].ID == "" {
			questions[i].ID = fmt.Sprintf("q%d", i)
		}
	}
	answers, err := r.AskUser(ctx, task.ID, questions, task.CurrentStep)
	if err != nil {
		errObj, _ := json.Marshal(map[string]string{"error": "user did not answer: " + err.Error()})
		return string(errObj)
	}
	b, _ := json.Marshal(map[string]interface{}{"answers": answers})
	return string(b)
}

// truncateRunes returns the first n runes of s.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// execute runs the full agent loop for a task.
func (r *Runner) execute(ctx context.Context, task *Task, conversationCtx string) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Error().Str("task_id", task.ID).Interface("panic", rec).Msg("[agent] runner panic")
			task.Status = TaskStatusFailed
			task.Error = fmt.Sprintf("internal error: %v", rec)
			_ = r.store.Update(context.Background(), task)
		}
		r.mu.Lock()
		delete(r.running, task.ID)
		r.mu.Unlock()
		// Clean up message queue
		r.msgMu.Lock()
		delete(r.msgQueues, task.ID)
		r.msgMu.Unlock()
		// Clean up pending ask channel
		r.askMu.Lock()
		delete(r.askQueues, task.ID)
		r.askMu.Unlock()
	}()

	// Phase 1: Planning
	task.Status = TaskStatusPlanning
	_ = r.store.Update(ctx, task)
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_planning",
		Message:   "Creating execution plan...",
	})

	// Start timeout warning goroutine (warn at 80% of timeout)
	go r.watchTimeout(ctx, task)

	plan, err := r.generatePlan(ctx, task.Goal, conversationCtx)
	if err != nil {
		r.failTask(ctx, task, fmt.Sprintf("planning failed: %v", err))
		return
	}

	task.Plan = plan
	task.Status = TaskStatusExecuting
	_ = r.store.Update(ctx, task)

	// Phase 2: Execute each step
	var consecutiveFailures int
	const maxConsecutiveFailures = 3 // abort if too many steps fail in a row
	for i := range task.Plan {
		if ctx.Err() != nil {
			task.Status = TaskStatusCancelled
			task.Error = "task cancelled"
			_ = r.store.Update(context.Background(), task)
			r.publishEvent(task.UserID, TaskEvent{
				TaskID:    task.ID,
				EventType: "task_failed",
				Message:   "task cancelled",
			})
			return
		}

		// Drain queued user messages and inject as goal context
		if injected := r.drainMessages(task.ID); len(injected) > 0 {
			combined := strings.Join(injected, "\n")
			task.Goal += "\n\n[User update]: " + combined
			_ = r.store.Update(ctx, task)
			for _, msg := range injected {
				r.publishEvent(task.UserID, TaskEvent{
					TaskID:    task.ID,
					EventType: "task_user_message",
					Message:   msg,
				})
			}
			logger.Info().Str("task_id", task.ID).Int("count", len(injected)).Msg("[agent] injected user messages between steps")
		}

		step := &task.Plan[i]
		task.CurrentStep = i
		step.Status = StepStatusRunning
		now := timeutil.NowTime()
		step.StartedAt = &now
		task.Progress = (i * 100) / len(task.Plan)
		_ = r.store.Update(ctx, task)

		r.publishEvent(task.UserID, TaskEvent{
			TaskID:    task.ID,
			EventType: "task_progress",
			StepIndex: i,
			Progress:  task.Progress,
			Message:   step.Description,
		})

		output, err := r.executeStep(ctx, task, step)
		if err != nil {
			// Retry once before marking as failed
			logger.Warn().Err(err).Int("step", i).Str("task_id", task.ID).Msg("[agent] step failed, retrying once")
			r.publishEvent(task.UserID, TaskEvent{
				TaskID:    task.ID,
				EventType: "task_progress",
				StepIndex: i,
				Message:   fmt.Sprintf("Step %d failed, retrying...", i+1),
			})
			output, err = r.executeStep(ctx, task, step)
		}
		if err != nil {
			step.Status = StepStatusFailed
			step.Output = fmt.Sprintf("Error: %v", err)
			completedAt := timeutil.NowTime()
			step.CompletedAt = &completedAt
			_ = r.store.Update(ctx, task)
			consecutiveFailures++
			if consecutiveFailures >= maxConsecutiveFailures {
				// Too many consecutive failures — skip remaining steps
				logger.Warn().Int("step", i).Str("task_id", task.ID).Msg("[agent] too many consecutive failures, skipping remaining steps")
				for j := i + 1; j < len(task.Plan); j++ {
					task.Plan[j].Status = StepStatusSkipped
				}
				_ = r.store.Update(ctx, task)
				break
			}
			logger.Warn().Err(err).Int("step", i).Str("task_id", task.ID).Msg("[agent] step failed after retry, continuing")
			continue
		}

		consecutiveFailures = 0 // reset on success
		step.Status = StepStatusCompleted
		step.Output = output
		completedAt := timeutil.NowTime()
		step.CompletedAt = &completedAt
		_ = r.store.Update(ctx, task)

		r.publishEvent(task.UserID, TaskEvent{
			TaskID:    task.ID,
			EventType: "task_step_completed",
			StepIndex: i,
			Output:    truncate(output, 500),
			DurationMs: func() int64 {
				if step.StartedAt != nil && step.CompletedAt != nil {
					return step.CompletedAt.Sub(*step.StartedAt).Milliseconds()
				}
				return 0
			}(),
		})
	}

	// Phase 3: Auto-verification
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_progress",
		Progress:  95,
		Message:   "Verifying results...",
	})

	verifyStep := &PlanStep{
		Index:       len(task.Plan),
		Description: "Verify the completed work — run build and tests to confirm correctness",
		Status:      StepStatusRunning,
	}
	now := timeutil.NowTime()
	verifyStep.StartedAt = &now

	verifyOutput, verifyErr := r.executeStep(ctx, task, verifyStep)
	if verifyErr != nil {
		verifyStep.Status = StepStatusFailed
		verifyStep.Output = fmt.Sprintf("Verification error: %v", verifyErr)
	} else {
		verifyStep.Status = StepStatusCompleted
		verifyStep.Output = verifyOutput
	}
	completedAt := timeutil.NowTime()
	verifyStep.CompletedAt = &completedAt
	task.Plan = append(task.Plan, *verifyStep)

	// Phase 4: Summarize and suggest next steps
	task.Status = TaskStatusCompleted
	task.Progress = 100
	task.Result = r.generateSummary(ctx, task)
	_ = r.store.Update(ctx, task)

	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_completed",
		Progress:  100,
		Message:   task.Result,
	})
}

// generatePlan asks the LLM to create a structured plan.
func (r *Runner) generatePlan(ctx context.Context, goal, conversationCtx string) ([]PlanStep, error) {
	// Recall relevant memories for context
	memoryCtx := r.recallMemories(ctx, goal)

	systemPrompt := `You are a task planner. Given a goal, create a step-by-step execution plan.
Return ONLY a JSON array of steps, each with "description" field. Keep steps concrete and actionable.
Example: [{"description":"Install dependencies with pip"},{"description":"Create main.py with FastAPI app"},{"description":"Add authentication middleware"}]
Return 3-10 steps. No markdown, no explanation — just the JSON array.`

	if memoryCtx != "" {
		systemPrompt += "\n\n# Relevant Context\n" + memoryCtx
	}

	userMsg := goal
	if conversationCtx != "" {
		userMsg = "Recent conversation context:\n" + truncate(conversationCtx, 2000) + "\n\nGoal: " + goal
	}

	resp, err := r.llm.Chat(ctx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: systemPrompt},
			{Role: llm.RoleUser, Content: userMsg},
		},
		MaxTokens:   1000,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, err
	}

	// Parse the plan from LLM response
	content := strings.TrimSpace(resp.Message.Content)
	// Strip markdown code fences if present
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var rawSteps []struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(content), &rawSteps); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w (content: %s)", err, truncate(content, 200))
	}

	steps := make([]PlanStep, len(rawSteps))
	for i, rs := range rawSteps {
		steps[i] = PlanStep{
			Index:       i,
			Description: rs.Description,
			Status:      StepStatusPending,
		}
	}
	return steps, nil
}

// executeStep runs a single plan step using the LLM + tools.
func (r *Runner) executeStep(ctx context.Context, task *Task, step *PlanStep) (string, error) {
	// Build context: goal + plan + current step
	var contextMsg strings.Builder
	contextMsg.WriteString(fmt.Sprintf("Goal: %s\n\n", task.Goal))
	contextMsg.WriteString("Plan:\n")
	for _, s := range task.Plan {
		status := "[ ]"
		if s.Status == StepStatusCompleted {
			status = "[x]"
		} else if s.Status == StepStatusRunning {
			status = "[>]"
		} else if s.Status == StepStatusFailed {
			status = "[!]"
		}
		contextMsg.WriteString(fmt.Sprintf("%s %d. %s\n", status, s.Index+1, s.Description))
		if s.Output != "" && s.Status == StepStatusCompleted {
			contextMsg.WriteString(fmt.Sprintf("   Result: %s\n", truncate(s.Output, 200)))
		}
	}
	contextMsg.WriteString(fmt.Sprintf("\nNow execute step %d: %s", step.Index+1, step.Description))

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: `You are an autonomous agent executing a plan step. Use available tools to complete the step. Be concise in your response — just do the work and report the result.

When you encounter ambiguity, need user preferences, or face a decision with multiple valid options, use the ask tool to ask the user.
Use "sq" for single-select or "mq" for multi-select, with "a" as the options array (2-4 strings).
Example: {"sq": "Which approach?", "a": ["Option A", "Option B"]}

Group related questions into a single ask call. Keep questions clear and provide good option labels.`},
		{Role: llm.RoleUser, Content: contextMsg.String()},
	}

	// Tool loop for this step
	var lastContent string
	var lastToolSig string // detect repeated identical tool calls
	var repeatCount int
	const maxRepeats = 3
	cachedTools := r.llmTools() // cache once per step — tool list doesn't change mid-execution

	for round := 0; round < MaxToolRoundsPerStep; round++ {
		if ctx.Err() != nil {
			return lastContent, ctx.Err()
		}

		// Drain queued user messages and inject into the LLM conversation
		if injected := r.drainMessages(task.ID); len(injected) > 0 {
			combined := strings.Join(injected, "\n")
			messages = append(messages, llm.Message{
				Role:    llm.RoleUser,
				Content: "[User message during execution]: " + combined,
			})
			r.publishEvent(task.UserID, TaskEvent{
				TaskID:    task.ID,
				EventType: "task_user_message",
				StepIndex: step.Index,
				Message:   combined,
			})
			logger.Info().Str("task_id", task.ID).Int("step", step.Index).Int("round", round).Msg("[agent] injected user messages between tool rounds")
		}

		resp, err := r.llm.Chat(ctx, llm.ChatRequest{
			Model:       "auto",
			Messages:    messages,
			Tools:       cachedTools,
			MaxTokens:   2000,
			Temperature: 0.2,
		})
		if err != nil {
			return lastContent, err
		}

		lastContent = resp.Message.Content

		// No tool calls — step is done
		if len(resp.Message.ToolCalls) == 0 {
			break
		}

		// Detect repeated identical tool calls to prevent infinite loops
		sig := ""
		for _, tc := range resp.Message.ToolCalls {
			sig += tc.Name + ":" + tc.Arguments + ";"
		}
		if sig == lastToolSig {
			repeatCount++
			if repeatCount >= maxRepeats {
				lastContent += "\n[Agent stopped: repeated identical tool calls detected]"
				break
			}
		} else {
			lastToolSig = sig
			repeatCount = 0
		}

		// Execute tool calls
		messages = append(messages, resp.Message)
		for _, tc := range resp.Message.ToolCalls {
			var content string

			// Intercept ask tool — use agent-specific SSE flow
			// so the frontend knows which task is asking.
			if tc.Name == "ask" {
				content = r.handleAskUser(ctx, task, tc.Arguments)
			} else {
				result, execErr := r.executor.ExecuteJSON(ctx, tc.Name, tc.Arguments)
				if execErr != nil {
					errObj, _ := json.Marshal(map[string]string{"error": execErr.Error()})
					content = string(errObj)
				} else {
					switch v := result.(type) {
					case string:
						content = v
					default:
						b, _ := json.Marshal(v)
						content = string(b)
					}
				}
			}
			// Truncate large tool results to prevent context window overflow
			content = truncate(content, 8000)
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    content,
				ToolCallID: tc.ID,
			})
		}
	}

	return lastContent, nil
}

// generateSummary uses the LLM to create a summary with next-step suggestions.
// Falls back to a simple count-based summary if the LLM call fails.
func (r *Runner) generateSummary(ctx context.Context, task *Task) string {
	// Build step results for context
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Goal: %s\n\nCompleted steps:\n", task.Goal))
	completed, failed := 0, 0
	for _, step := range task.Plan {
		status := "?"
		switch step.Status {
		case StepStatusCompleted:
			status = "OK"
			completed++
		case StepStatusFailed:
			status = "FAILED"
			failed++
		case StepStatusSkipped:
			status = "SKIPPED"
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s", status, step.Description))
		if step.Output != "" {
			sb.WriteString(fmt.Sprintf(": %s", truncate(step.Output, 150)))
		}
		sb.WriteString("\n")
	}

	resp, err := r.llm.Chat(ctx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: `Summarize the completed agent task in 2-3 sentences. Then suggest 1-3 concrete next steps the user might want to take. Format:

Summary: <what was accomplished>

Suggested next steps:
1. <suggestion>
2. <suggestion>`},
			{Role: llm.RoleUser, Content: sb.String()},
		},
		MaxTokens:   500,
		Temperature: 0.3,
	})
	if err != nil {
		// Fallback to simple summary
		result := fmt.Sprintf("Completed %d/%d steps", completed, len(task.Plan))
		if failed > 0 {
			result += fmt.Sprintf(" (%d failed)", failed)
		}
		return result + "."
	}
	return strings.TrimSpace(resp.Message.Content)
}

// failTask marks a task as failed and publishes the event.
func (r *Runner) failTask(ctx context.Context, task *Task, errMsg string) {
	task.Status = TaskStatusFailed
	task.Error = errMsg
	_ = r.store.Update(ctx, task)
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_failed",
		Message:   errMsg,
	})
}

// publishEvent sends an SSE event to the user.
func (r *Runner) publishEvent(userID string, event TaskEvent) {
	if r.broker != nil {
		r.broker.Publish(userID, event.EventType, event)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// llmTools converts the registry's tool definitions to llm.Tool format.
// The ask tool is already registered in the registry as a first-class tool.
func (r *Runner) llmTools() []llm.Tool {
	if r.registry == nil {
		return nil
	}
	defs := r.registry.Definitions()
	out := make([]llm.Tool, len(defs))
	for i, d := range defs {
		out[i] = llm.Tool{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  d.Parameters,
		}
	}
	return out
}

// recallMemories searches for relevant memories and returns context string.
func (r *Runner) recallMemories(ctx context.Context, query string) string {
	if r.memory == nil || query == "" {
		return ""
	}
	results, err := r.memory.Recall(ctx, query, 5)
	if err != nil || len(results) == 0 {
		return ""
	}
	var kept []string
	for _, res := range results {
		if res.Score < 0.5 || res.Content == "" {
			continue
		}
		kept = append(kept, res.Content)
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n---\n")
}

// watchTimeout publishes a warning SSE event when the task is near its timeout.
func (r *Runner) watchTimeout(ctx context.Context, task *Task) {
	deadline, ok := ctx.Deadline()
	if !ok {
		return
	}
	total := time.Until(deadline)
	warnAt := time.Duration(float64(total) * 0.8)
	timer := time.NewTimer(warnAt)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
		remaining := time.Until(deadline).Round(time.Second)
		r.publishEvent(task.UserID, TaskEvent{
			TaskID:    task.ID,
			EventType: "task_progress",
			Message:   fmt.Sprintf("Warning: task approaching timeout (%s remaining)", remaining),
		})
	}
}
