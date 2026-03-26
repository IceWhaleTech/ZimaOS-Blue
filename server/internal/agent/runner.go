package agent

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// MaxConcurrentTasks is the default limit of concurrent agent tasks per user.
const MaxConcurrentTasks = 3

// MaxToolRoundsPerStep is the max tool rounds within a single plan step.
const MaxToolRoundsPerStep = 50

// defaultAskTimeout is used when Ask timeout is not configured.
const defaultAskTimeout = 10 * time.Minute

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
	AskTimeout    time.Duration
	AutoReflect   bool
	// AskTimeoutAction controls timeout behavior: "error" (default) | "default".
	AskTimeoutAction string
	// MaxToolRoundsPerStep controls tool loop budget per plan step.
	MaxToolRoundsPerStep int
}

// Runner executes agent tasks in the background.
type Runner struct {
	store       *Store
	llm         LLMCaller
	executor    *tools.Executor
	toolGateway *tools.ToolGateway
	registry    *tools.Registry
	broker      *sse.Broker
	memory      MemoryRecaller
	reflector   SelfReflector
	config      RunnerConfig

	mu      sync.Mutex
	running map[string]context.CancelFunc // task ID → cancel
	wg      sync.WaitGroup                // tracks background goroutines

	msgMu     sync.Mutex
	msgQueues map[string][]string // task ID → queued user messages

	askMu     sync.Mutex
	askQueues map[string]chan []QuestionAnswer // task ID → pending answer channel

	askTimeoutFunc       func() time.Duration
	askTimeoutActionFunc func() string // "error" | "default"
	maxToolRoundsFunc    func() int
	autoReflectFunc      func() bool

	groundedRuntime *GroundedRuntime
	groundedSecret  []byte
	eventObserver   TaskEventObserver
	subagents       tools.SubagentExecutor
	writeGuard      tools.WritePathGuard
	execGuard       tools.ExecPathGuard
}

type SelfReflector interface {
	Reflect(ctx context.Context, input selfreflect.Input) (*selfreflect.Result, error)
}

type TaskEventObserver interface {
	HandleTaskEvent(event TaskEvent)
}

// NewRunner creates a new agent runner.
func NewRunner(store *Store, llmCaller LLMCaller, registry *tools.Registry, executor *tools.Executor, broker *sse.Broker, config RunnerConfig) *Runner {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = MaxConcurrentTasks
	}
	if config.TaskTimeout <= 0 {
		config.TaskTimeout = 30 * time.Minute
	}
	if config.AskTimeout <= 0 {
		config.AskTimeout = defaultAskTimeout
	}
	if config.MaxToolRoundsPerStep <= 0 {
		config.MaxToolRoundsPerStep = MaxToolRoundsPerStep
	}
	switch strings.ToLower(strings.TrimSpace(config.AskTimeoutAction)) {
	case "default":
		config.AskTimeoutAction = "default"
	default:
		config.AskTimeoutAction = "error"
	}
	var toolGateway *tools.ToolGateway
	if registry != nil && executor != nil {
		toolGateway = tools.NewToolGateway(registry, executor)
	}
	runner := &Runner{
		store:       store,
		llm:         llmCaller,
		registry:    registry,
		executor:    executor,
		toolGateway: toolGateway,
		broker:      broker,
		config:      config,
		running:     make(map[string]context.CancelFunc),
		msgQueues:   make(map[string][]string),
		askQueues:   make(map[string]chan []QuestionAnswer),
	}
	runner.groundedSecret = newGroundedSecret()
	runner.groundedRuntime = NewGroundedRuntime(GroundedRuntimeConfig{
		PlannerLLM:       llmCaller,
		ResponderLLM:     llmCaller,
		Registry:         registry,
		Executor:         executor,
		Store:            store,
		AskHandler:       runner.handleAskUser,
		ConfirmToolCall:  runner.confirmGroundedToolCall,
		Secret:           runner.groundedSecret,
		MaxPlannerRounds: config.MaxToolRoundsPerStep,
	})
	if runner.toolGateway != nil && runner.groundedRuntime != nil && runner.groundedRuntime.executor != nil {
		runner.groundedRuntime.executor.SetToolGateway(runner.toolGateway)
	}
	return runner
}

// SetToolApprover wires runtime tool approval enforcement into the agent's shared tool gateway.
func (r *Runner) SetToolApprover(approver tools.ToolApprover) {
	if r == nil {
		return
	}
	if r.toolGateway == nil && r.registry != nil && r.executor != nil {
		r.toolGateway = tools.NewToolGateway(r.registry, r.executor)
	}
	if r.toolGateway != nil {
		r.toolGateway.SetApprover(approver)
	}
	if r.groundedRuntime != nil && r.groundedRuntime.executor != nil {
		r.groundedRuntime.executor.SetToolGateway(r.toolGateway)
	}
}

// SetToolEventObserver wires runtime tool lifecycle observation into the shared tool gateway.
func (r *Runner) SetToolEventObserver(observer tools.RuntimeEventObserver) {
	if r == nil {
		return
	}
	if r.toolGateway == nil && r.registry != nil && r.executor != nil {
		r.toolGateway = tools.NewToolGateway(r.registry, r.executor)
	}
	if r.toolGateway != nil {
		r.toolGateway.SetEventObserver(observer)
	}
	if r.groundedRuntime != nil && r.groundedRuntime.executor != nil {
		r.groundedRuntime.executor.SetToolGateway(r.toolGateway)
	}
}

// SetToolMetricsRecorder wires runtime counter recording into the shared tool gateway.
func (r *Runner) SetToolMetricsRecorder(recorder interface {
	RecordCounter(name string, value int64, tags map[string]string)
}) {
	if r == nil || recorder == nil {
		return
	}
	if r.toolGateway == nil && r.registry != nil && r.executor != nil {
		r.toolGateway = tools.NewToolGateway(r.registry, r.executor)
	}
	if r.toolGateway != nil {
		r.toolGateway.SetMetricsRecorder(recorder)
	}
	if r.groundedRuntime != nil && r.groundedRuntime.executor != nil {
		r.groundedRuntime.executor.SetToolGateway(r.toolGateway)
	}
}

func newGroundedSecret() []byte {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return []byte("zimaos-grounded-runtime-fallback-secret")
	}
	return secret
}

func (r *Runner) confirmGroundedToolCall(ctx context.Context, task *Task, step PlanStep, nextTool PlannerToolCall) error {
	argsJSON, _ := json.Marshal(nextTool.Args)
	capability := classifyCapability(nextTool.Tool, string(argsJSON))
	if !shouldRequireConfirm(capability, step.Description) {
		return nil
	}
	if err := r.transitionState(ctx, task, RuntimeStateConfirmGate, "high-risk capability requires confirmation", &capability, TaskStatusWaitingInput); err != nil {
		return err
	}
	answers, askErr := r.AskUser(ctx, task.ID, buildHighRiskConfirmationQuestions(nextTool.Tool, capability, step.Description), step.Index)
	if askErr != nil {
		return askErr
	}
	decision := "skip"
	if len(answers) > 0 && len(answers[0].Values) > 0 {
		decision = answers[0].Values[0]
	}
	switch decision {
	case "abort":
		if err := r.transitionState(ctx, task, RuntimeStateAborted, "user aborted on high-risk tool call", &capability, TaskStatusAborted); err != nil {
			return err
		}
		return fmt.Errorf("user aborted task during high-risk confirmation")
	case "skip":
		if err := r.transitionState(ctx, task, RuntimeStateExecute, "user skipped high-risk tool call", &capability, TaskStatusExecuting); err != nil {
			return err
		}
		return fmt.Errorf("user skipped high-risk tool call")
	default:
		if err := r.transitionState(ctx, task, RuntimeStateExecute, "high-risk tool call approved", &capability, TaskStatusExecuting); err != nil {
			return err
		}
		return nil
	}
}

// SetAskTimeoutFunc sets a dynamic timeout getter (takes precedence over static timeout when >0).
func (r *Runner) SetAskTimeoutFunc(fn func() time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.askTimeoutFunc = fn
}

// SetAskTimeoutActionFunc sets dynamic timeout action getter ("error" | "default").
func (r *Runner) SetAskTimeoutActionFunc(fn func() string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.askTimeoutActionFunc = fn
}

// SetMaxToolRoundsPerStepFunc sets a dynamic max-tool-round getter.
func (r *Runner) SetMaxToolRoundsPerStepFunc(fn func() int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxToolRoundsFunc = fn
}

// SetAutoReflectFunc sets a dynamic auto-reflect toggle getter.
func (r *Runner) SetAutoReflectFunc(fn func() bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.autoReflectFunc = fn
}

func (r *Runner) resolveAskTimeout() time.Duration {
	r.mu.Lock()
	fn := r.askTimeoutFunc
	base := r.config.AskTimeout
	r.mu.Unlock()

	if fn != nil {
		if d := fn(); d > 0 {
			return d
		}
	}
	if base <= 0 {
		return defaultAskTimeout
	}
	return base
}

func (r *Runner) resolveAskTimeoutAction() string {
	r.mu.Lock()
	fn := r.askTimeoutActionFunc
	base := r.config.AskTimeoutAction
	r.mu.Unlock()

	if fn != nil {
		switch strings.ToLower(strings.TrimSpace(fn())) {
		case "default":
			return "default"
		default:
			return "error"
		}
	}
	switch strings.ToLower(strings.TrimSpace(base)) {
	case "default":
		return "default"
	default:
		return "error"
	}
}

func (r *Runner) resolveMaxToolRoundsPerStep() int {
	r.mu.Lock()
	fn := r.maxToolRoundsFunc
	base := r.config.MaxToolRoundsPerStep
	r.mu.Unlock()

	if fn != nil {
		if v := fn(); v > 0 {
			if v > 200 {
				return 200
			}
			return v
		}
	}
	if base <= 0 {
		return MaxToolRoundsPerStep
	}
	if base > 200 {
		return 200
	}
	return base
}

func (r *Runner) resolveAutoReflect() bool {
	r.mu.Lock()
	fn := r.autoReflectFunc
	base := r.config.AutoReflect
	r.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return base
}

// SetMemory sets the memory recaller for context injection.
func (r *Runner) SetMemory(m MemoryRecaller) {
	r.memory = m
}

// SetReflector sets the post-task reflection engine.
func (r *Runner) SetReflector(reflector SelfReflector) {
	r.reflector = reflector
}

// SetSubagentExecutor wires harness-backed child-run execution into the agent runtime.
func (r *Runner) SetSubagentExecutor(executor tools.SubagentExecutor) {
	if r == nil {
		return
	}
	r.subagents = executor
}

// SetWritePathGuard wires runtime-specific direct-write protection into file mutation tools.
func (r *Runner) SetWritePathGuard(guard tools.WritePathGuard) {
	if r == nil {
		return
	}
	r.writeGuard = guard
}

// SetExecPathGuard wires runtime-specific exec path protection into exec-style tools.
func (r *Runner) SetExecPathGuard(guard tools.ExecPathGuard) {
	if r == nil {
		return
	}
	r.execGuard = guard
}

// SetEventObserver wires an optional observer for task SSE events.
func (r *Runner) SetEventObserver(observer TaskEventObserver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventObserver = observer
}

// Submit creates and starts a new agent task. Returns the task immediately.
func (r *Runner) Submit(ctx context.Context, userID, goal, conversationID, conversationCtx string) (*Task, error) {
	task := &Task{
		ID:             uuid.New().String(),
		UserID:         userID,
		ConversationID: conversationID,
		Goal:           goal,
		Status:         TaskStatusPending,
	}
	if err := r.startTask(ctx, task, conversationCtx); err != nil {
		return nil, err
	}
	return task, nil
}

// SubmitTask starts a caller-provided task ID through the normal runner flow.
func (r *Runner) SubmitTask(ctx context.Context, task *Task, conversationCtx string) (*Task, error) {
	if task == nil {
		return nil, fmt.Errorf("task is required")
	}
	if strings.TrimSpace(task.ID) == "" {
		task.ID = uuid.New().String()
	}
	if task.Status == "" {
		task.Status = TaskStatusPending
	}
	if err := r.startTask(ctx, task, conversationCtx); err != nil {
		return nil, err
	}
	return task, nil
}

func (r *Runner) startTask(ctx context.Context, task *Task, conversationCtx string) error {
	// Check concurrency limit
	running, err := r.store.CountRunning(ctx, task.UserID)
	if err != nil {
		return fmt.Errorf("failed to check running tasks: %w", err)
	}
	if running >= r.config.MaxConcurrent {
		return fmt.Errorf("max concurrent tasks reached (%d)", r.config.MaxConcurrent)
	}

	if err := r.store.Create(ctx, task); err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	// Publish creation event
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_created",
		Message:   taskCreatedMessage(task.Goal),
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

	return nil
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
// Sets task status to waiting_input while blocked, then restores the prior running status.
func (r *Runner) AskUser(ctx context.Context, taskID string, questions []AgentQuestion, stepIndex int) ([]QuestionAnswer, error) {
	task, err := r.store.Get(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	resumeStatus := task.Status
	if resumeStatus == "" || resumeStatus == TaskStatusWaitingInput {
		resumeStatus = resumeStatusForRuntimeState(task.RuntimeState)
	}

	ch := make(chan []QuestionAnswer, 1)
	r.askMu.Lock()
	r.askQueues[taskID] = ch
	r.askMu.Unlock()

	defer func() {
		r.askMu.Lock()
		delete(r.askQueues, taskID)
		r.askMu.Unlock()
		if errors.Is(ctx.Err(), context.Canceled) {
			return
		}
		_ = r.store.SetStatus(context.Background(), taskID, resumeStatus, "")
	}()

	// Set status to waiting_input
	_ = r.store.SetStatus(ctx, taskID, TaskStatusWaitingInput, "")

	// Publish question event via SSE
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    taskID,
		EventType: "task_question",
		StepIndex: stepIndex,
		Message:   taskQuestionWaitingMessage(),
		Questions: questions,
	})

	// Block until response/timeout/cancellation.
	askTimeout := r.resolveAskTimeout()
	timer := time.NewTimer(askTimeout)
	defer timer.Stop()

	select {
	case answers := <-ch:
		return answers, nil
	case <-timer.C:
		if r.resolveAskTimeoutAction() == "default" {
			r.publishEvent(task.UserID, TaskEvent{
				TaskID:    taskID,
				EventType: "task_question_timeout",
				StepIndex: stepIndex,
				Message:   taskQuestionTimeoutMessage(askTimeout),
			})
			return defaultQuestionAnswers(questions), nil
		}
		return nil, fmt.Errorf("ask_user timed out after %v — no user response", askTimeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func resumeStatusForRuntimeState(state RuntimeState) TaskStatus {
	switch state {
	case RuntimeStateClarify, RuntimeStatePlan, RuntimeStateConfirmGate:
		return TaskStatusPlanning
	case RuntimeStateExecute, RuntimeStateVerify, RuntimeStateReflect, RuntimeStateRecover, RuntimeStateReport:
		return TaskStatusExecuting
	default:
		return TaskStatusExecuting
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
				Message:   taskQuestionAnsweredMessage(len(answers)),
			})
		}
		return true
	default:
		return false
	}
}

// handleAskUser parses the ask_user tool arguments, blocks for user answers, and returns the result as JSON.
// Supports single-question shorthand (q/mq + a) and questions array format.
func (r *Runner) handleAskUser(ctx context.Context, task *Task, argsJSON string) string {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		return `{"error":"invalid ask_user arguments: ` + err.Error() + `"}`
	}

	var questions []AgentQuestion

	// Try new q/mq/a format first
	q, _ := raw["q"].(string)
	mq, _ := raw["mq"].(string)
	qText := q
	multiSelect := false
	if mq != "" {
		qText = mq
		multiSelect = true
	}
	if qText != "" {
		q := AgentQuestion{
			ID:          "q0",
			Question:    qText,
			Detail:      extractAgentQuestionDetail(raw),
			Header:      truncateRunes(qText, 12),
			MultiSelect: multiSelect,
		}
		if multiSelect {
			q.Type = "checkbox"
		} else {
			q.Type = "radio"
		}
		if aRaw, ok := raw["a"]; ok {
			q.Options = parseAgentQuestionOptions(aRaw)
		}
		if len(q.Options) == 0 {
			if aRaw, ok := raw["options"]; ok {
				q.Options = parseAgentQuestionOptions(aRaw)
			}
		}
		questions = []AgentQuestion{q}
	} else {
		// Legacy: questions array
		var req struct {
			Questions []AgentQuestion `json:"questions"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &req); err != nil || len(req.Questions) == 0 {
			return `{"error":"invalid ask_user arguments: provide q/mq + a or questions[]"}`
		}
		questions = req.Questions
	}

	normalized, normErr := normalizeAgentQuestions(questions)
	if normErr != nil {
		errObj, _ := json.Marshal(map[string]string{"error": normErr.Error()})
		return string(errObj)
	}
	answers, err := r.AskUser(ctx, task.ID, normalized, task.CurrentStep)
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

func parseAgentQuestionOptions(v interface{}) []QuestionOption {
	switch arr := v.(type) {
	case []interface{}:
		out := make([]QuestionOption, 0, len(arr))
		for _, item := range arr {
			switch val := item.(type) {
			case string:
				s := strings.TrimSpace(val)
				if s == "" {
					continue
				}
				out = append(out, QuestionOption{Label: s, Value: s})
			case map[string]interface{}:
				label := ""
				for _, k := range []string{"label", "text", "name", "title", "value"} {
					if s, ok := val[k].(string); ok && strings.TrimSpace(s) != "" {
						label = strings.TrimSpace(s)
						break
					}
				}
				if label == "" {
					continue
				}
				desc := ""
				for _, k := range []string{"description", "hint"} {
					if s, ok := val[k].(string); ok && strings.TrimSpace(s) != "" {
						desc = strings.TrimSpace(s)
						break
					}
				}
				value := label
				if s, ok := val["value"].(string); ok && strings.TrimSpace(s) != "" {
					value = strings.TrimSpace(s)
				}
				out = append(out, QuestionOption{Label: label, Description: desc, Value: value})
			}
		}
		return out
	case []string:
		out := make([]QuestionOption, 0, len(arr))
		for _, s := range arr {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, QuestionOption{Label: s, Value: s})
		}
		return out
	default:
		return nil
	}
}

func extractAgentQuestionDetail(m map[string]interface{}) string {
	for _, k := range []string{"detail", "details", "extra_detail", "description", "hint"} {
		raw, ok := m[k]
		if !ok {
			continue
		}
		s, ok := raw.(string)
		if !ok || strings.TrimSpace(s) == "" {
			continue
		}
		return strings.TrimSpace(s)
	}
	return ""
}

func normalizeAgentQuestions(questions []AgentQuestion) ([]AgentQuestion, error) {
	if len(questions) == 0 {
		return nil, fmt.Errorf("at least one question is required")
	}
	out := make([]AgentQuestion, 0, len(questions))
	for i := range questions {
		q := questions[i]
		q.Question = strings.TrimSpace(q.Question)
		if q.Question == "" {
			return nil, fmt.Errorf("question text is required")
		}
		q.Detail = strings.TrimSpace(q.Detail)
		if q.ID == "" {
			q.ID = fmt.Sprintf("q%d", i)
		}
		if q.Header == "" {
			q.Header = truncateRunes(q.Question, 12)
		}
		q.Type = strings.ToLower(strings.TrimSpace(q.Type))
		opts := make([]QuestionOption, 0, len(q.Options))
		seen := make(map[string]struct{}, len(q.Options))
		for _, opt := range q.Options {
			label := strings.TrimSpace(opt.Label)
			value := strings.TrimSpace(opt.Value)
			if label == "" && value == "" {
				continue
			}
			if label == "" {
				label = value
			}
			if value == "" {
				value = label
			}
			key := strings.ToLower(value)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			opts = append(opts, QuestionOption{
				Label:       label,
				Description: strings.TrimSpace(opt.Description),
				Value:       value,
			})
		}
		// Keep choice questions deterministic while allowing free-text prompts.
		if len(opts) == 0 && !isAgentTextQuestionType(q.Type) {
			return nil, fmt.Errorf("question %q must include at least 2 options or set type=text", q.Question)
		}
		if len(opts) > 0 && len(opts) < 2 {
			return nil, fmt.Errorf("question %q must include at least 2 options", q.Question)
		}
		if len(opts) > 5 {
			opts = opts[:5]
		}
		q.Options = opts
		out = append(out, q)
	}
	return out, nil
}

func isAgentTextQuestionType(qType string) bool {
	switch strings.ToLower(strings.TrimSpace(qType)) {
	case "text", "input", "textarea", "freeform", "free-form":
		return true
	default:
		return false
	}
}

func buildClarificationQuestions() []AgentQuestion {
	return []AgentQuestion{{
		ID:       "goal_scope",
		Header:   "Scope",
		Question: "你希望这次任务采用哪种执行深度？",
		Detail:   "当前目标缺少关键范围信息；如果你暂时不回复，系统会默认选择平衡方案。",
		Options: []QuestionOption{
			{Label: "平衡方案（推荐）", Value: "balanced", Description: "速度与质量均衡，适合作为默认执行方式"},
			{Label: "快速可运行", Value: "fast", Description: "优先速度，较少工程化"},
			{Label: "工程化完善", Value: "rigorous", Description: "包含测试、文档与健壮性"},
		},
		Required: true,
	}}
}

func buildPlanConfirmationQuestions(plan runtimePlan) []AgentQuestion {
	detailParts := []string{fmt.Sprintf("当前计划共 %d 个步骤", len(plan.Subtasks))}
	if reasons := compactListForQuestion(plan.RequiresConfirmation, 2); len(reasons) > 0 {
		detailParts = append(detailParts, "需要你确认："+strings.Join(reasons, "；"))
	}
	detailParts = append(detailParts, "如果你暂时不回复，系统会先回到规划阶段收紧方案")
	return []AgentQuestion{{
		ID:       "plan_gate",
		Header:   "Plan",
		Question: "计划已生成，下一步怎么做？",
		Detail:   strings.Join(detailParts, "。") + "。",
		Options: []QuestionOption{
			{Label: "先调整计划（推荐）", Value: "revise", Description: "返回规划阶段，降低风险并提高确定性"},
			{Label: "继续执行", Value: "continue", Description: "按当前计划继续"},
			{Label: "中止任务", Value: "abort", Description: "立即停止任务"},
		},
		Required: true,
	}}
}

func buildHighRiskConfirmationQuestions(toolName string, cap CapabilityInfo, stepDescription string) []AgentQuestion {
	detailParts := []string{
		fmt.Sprintf("风险级别：%s", strings.ToUpper(strings.TrimSpace(cap.RiskLevel))),
		fmt.Sprintf("调用类型：%s", cap.Kind),
	}
	if step := strings.TrimSpace(stepDescription); step != "" {
		detailParts = append(detailParts, "当前步骤："+step)
	}
	if !cap.Idempotent {
		detailParts = append(detailParts, "该调用可能产生不可逆副作用")
	}
	detailParts = append(detailParts, "如果你暂时不回复，系统会默认跳过本次调用")
	return []AgentQuestion{{
		ID:       "tool_gate",
		Header:   "Confirm",
		Question: fmt.Sprintf("高风险调用 `%s` 已准备执行，下一步怎么做？", toolName),
		Detail:   strings.Join(detailParts, "。") + "。",
		Options: []QuestionOption{
			{Label: "跳过本次调用（推荐）", Value: "skip", Description: "跳过这次高风险操作，并继续寻找更安全路径"},
			{Label: "继续执行", Value: "continue", Description: "执行本次调用"},
			{Label: "中止任务", Value: "abort", Description: "立即中止任务"},
		},
		Required: true,
	}}
}

func compactListForQuestion(items []string, limit int) []string {
	if limit <= 0 {
		limit = len(items)
	}
	out := make([]string, 0, limit)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func taskQuestionWaitingMessage() string {
	return "Waiting for your input to continue."
}

func taskQuestionTimeoutMessage(askTimeout time.Duration) string {
	return fmt.Sprintf("No reply received in %s. Continuing with the default option.", askTimeout)
}

func taskQuestionAnsweredMessage(answerCount int) string {
	return fmt.Sprintf("Received %d user answer(s). Continuing execution.", answerCount)
}

func taskPlanningStartedMessage() string {
	return "Building the execution plan."
}

func taskStepRetryMessage(stepIndex int) string {
	return fmt.Sprintf("Step %d failed. Retrying once with a safer path.", stepIndex+1)
}

func taskVerifyingMessage() string {
	return "Running verification checks."
}

func taskReflectingMessage() string {
	return "Capturing reusable lessons from this task."
}

func taskTimeoutWarningMessage(remaining time.Duration) string {
	return fmt.Sprintf("Task is nearing timeout (%s remaining).", remaining)
}

func taskCreatedMessage(goal string) string {
	msg := strings.TrimSpace(goal)
	if msg == "" {
		return "Task created."
	}
	return "Task created: " + msg
}

func taskUserUpdateMessage(message string) string {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "Received user update."
	}
	return "Received user update: " + msg
}

func taskFailedMessage(errMsg string) string {
	msg := strings.TrimSpace(errMsg)
	if msg == "" {
		return "Task failed."
	}
	switch {
	case strings.HasPrefix(msg, "clarify failed:"):
		return "Task failed during clarification: " + strings.TrimSpace(strings.TrimPrefix(msg, "clarify failed:"))
	case strings.HasPrefix(msg, "planning failed:"):
		return "Task failed during planning: " + strings.TrimSpace(strings.TrimPrefix(msg, "planning failed:"))
	case strings.HasPrefix(msg, "confirm gate failed:"):
		return "Task failed during confirmation: " + strings.TrimSpace(strings.TrimPrefix(msg, "confirm gate failed:"))
	case strings.HasPrefix(msg, "replanning failed:"):
		return "Task failed while revising the plan: " + strings.TrimSpace(strings.TrimPrefix(msg, "replanning failed:"))
	case strings.HasPrefix(msg, "verification failed and recovery failed:"):
		return "Verification failed and recovery did not succeed: " + strings.TrimSpace(strings.TrimPrefix(msg, "verification failed and recovery failed:"))
	case msg == "verification did not pass after bounded recovery retry":
		return "Verification did not pass after the bounded recovery retry."
	case strings.HasPrefix(msg, "runtime transition failed"):
		return "Task failed while updating runtime state: " + msg
	default:
		return "Task failed: " + msg
	}
}

func taskCancelledMessage(reason string) string {
	msg := strings.TrimSpace(reason)
	if msg == "" || msg == "task cancelled" {
		return "Task cancelled."
	}
	return "Task cancelled: " + msg
}

func taskStateTransitionMessage(reason string, from RuntimeState, to RuntimeState) string {
	switch strings.TrimSpace(reason) {
	case "task accepted":
		return "Task accepted."
	case "goal missing critical details":
		return "Goal needs clarification before planning."
	case "start planning":
		return "Planning started."
	case "plan requires user confirmation":
		return "Plan requires your confirmation."
	case "user requested plan revision":
		return "Revising the plan."
	case "user aborted at confirm gate":
		return "Task aborted at plan confirmation."
	case "start execution":
		return "Execution started."
	case "verifying task result":
		return "Verification started."
	case "reflecting on task outcome":
		return "Capturing reusable lessons."
	case "verification failed":
		return "Verification failed. Starting recovery."
	case "re-verify after recovery":
		return "Re-running verification after recovery."
	case "generating final report":
		return "Generating final report."
	case "task completed":
		return "Task completed."
	case "high-risk capability requires confirmation":
		return "High-risk action requires your confirmation."
	case "user aborted on high-risk tool call":
		return "Task aborted at high-risk confirmation."
	case "user skipped high-risk tool call":
		return "Skipped high-risk action and continued."
	case "high-risk tool call approved":
		return "High-risk action approved. Continuing."
	case "task failed":
		return "Task failed."
	default:
		if from != "" && to != "" {
			return fmt.Sprintf("State changed: %s → %s.", from, to)
		}
		if to != "" {
			return fmt.Sprintf("State changed: %s.", to)
		}
		return "State updated."
	}
}

func defaultQuestionAnswers(questions []AgentQuestion) []QuestionAnswer {
	results := make([]QuestionAnswer, len(questions))
	for i, q := range questions {
		results[i] = QuestionAnswer{QuestionID: q.ID}
		if len(q.Options) == 0 {
			continue
		}
		val := strings.TrimSpace(q.Options[0].Value)
		if val == "" {
			val = strings.TrimSpace(q.Options[0].Label)
		}
		if val != "" {
			results[i].Values = []string{val}
		}
	}
	return results
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

	if err := r.transitionState(ctx, task, RuntimeStateIntake, "task accepted", nil, ""); err != nil {
		r.failTask(ctx, task, fmt.Sprintf("runtime transition failed at intake: %v", err))
		return
	}

	// Phase 0: Clarify (explicit gate for ambiguous goals)
	if requiresClarification(task.Goal) {
		if err := r.transitionState(ctx, task, RuntimeStateClarify, "goal missing critical details", nil, TaskStatusWaitingInput); err != nil {
			r.failTask(ctx, task, fmt.Sprintf("runtime transition failed at clarify: %v", err))
			return
		}
		clarifyQs := buildClarificationQuestions()
		answers, askErr := r.AskUser(ctx, task.ID, clarifyQs, 0)
		if askErr != nil {
			r.failTask(ctx, task, fmt.Sprintf("clarify failed: %v", askErr))
			return
		}
		if len(answers) > 0 && len(answers[0].Values) > 0 {
			task.Goal = strings.TrimSpace(task.Goal + "\nExecution preference: " + answers[0].Values[0])
		}
	}

	if err := r.transitionState(ctx, task, RuntimeStatePlan, "start planning", nil, TaskStatusPlanning); err != nil {
		r.failTask(ctx, task, fmt.Sprintf("runtime transition failed at planning: %v", err))
		return
	}
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_planning",
		Message:   taskPlanningStartedMessage(),
	})

	// Start timeout warning goroutine (warn at 80% of timeout)
	go r.watchTimeout(ctx, task)

	planSpec, err := r.generatePlan(ctx, task.Goal, conversationCtx)
	if err != nil {
		r.failTask(ctx, task, fmt.Sprintf("planning failed: %v", err))
		return
	}

	task.Plan = planSpec.Steps
	task.SuccessCriteria = mergeTaskSuccessCriteria(task, planSpec.SuccessCriteria)
	task.FallbackPlan = mergeTaskFallbackPlan(task, planSpec.FallbackPlan)
	if task.Goal == "" && planSpec.Goal != "" {
		task.Goal = planSpec.Goal
	}

	if planNeedsConfirmation(planSpec.Raw) {
		if err := r.transitionState(ctx, task, RuntimeStateConfirmGate, "plan requires user confirmation", nil, TaskStatusWaitingInput); err != nil {
			r.failTask(ctx, task, fmt.Sprintf("runtime transition failed at confirm gate: %v", err))
			return
		}
		answers, askErr := r.AskUser(ctx, task.ID, buildPlanConfirmationQuestions(planSpec.Raw), 0)
		if askErr != nil {
			r.failTask(ctx, task, fmt.Sprintf("confirm gate failed: %v", askErr))
			return
		}
		decision := "revise"
		if len(answers) > 0 && len(answers[0].Values) > 0 {
			decision = answers[0].Values[0]
		}
		switch decision {
		case "revise":
			if err := r.transitionState(ctx, task, RuntimeStatePlan, "user requested plan revision", nil, TaskStatusPlanning); err != nil {
				r.failTask(ctx, task, fmt.Sprintf("runtime transition failed when revising plan: %v", err))
				return
			}
			planSpec, err = r.generatePlan(ctx, task.Goal+"\nPlease revise plan to reduce risk and improve determinism.", conversationCtx)
			if err != nil {
				r.failTask(ctx, task, fmt.Sprintf("replanning failed: %v", err))
				return
			}
			task.Plan = planSpec.Steps
			task.SuccessCriteria = mergeTaskSuccessCriteria(task, planSpec.SuccessCriteria)
			task.FallbackPlan = mergeTaskFallbackPlan(task, planSpec.FallbackPlan)
		case "abort":
			if err := r.transitionState(ctx, task, RuntimeStateAborted, "user aborted at confirm gate", nil, TaskStatusAborted); err != nil {
				r.failTask(ctx, task, fmt.Sprintf("runtime transition failed when aborting at confirm gate: %v", err))
				return
			}
			task.Result = "Task aborted by user at confirmation gate."
			task.Progress = 100
			_ = r.store.Update(ctx, task)
			return
		}
	}

	if err := r.transitionState(ctx, task, RuntimeStateExecute, "start execution", nil, TaskStatusExecuting); err != nil {
		r.failTask(ctx, task, fmt.Sprintf("runtime transition failed before execution: %v", err))
		return
	}
	task.Status = TaskStatusExecuting
	_ = r.store.Update(ctx, task)

	// Phase 2: Execute each step
	var consecutiveFailures int
	const maxConsecutiveFailures = 3 // abort if too many steps fail in a row
	for i := range task.Plan {
		if ctx.Err() != nil {
			r.cancelTask(task, "task cancelled")
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
					Message:   taskUserUpdateMessage(msg),
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
			if errors.Is(ctx.Err(), context.Canceled) {
				r.cancelTask(task, "task cancelled")
				return
			}
			if task.RuntimeState == RuntimeStateAborted {
				task.Status = TaskStatusAborted
				task.Progress = 100
				_ = r.store.Update(ctx, task)
				return
			}
			// Retry once before marking as failed
			logger.Warn().Err(err).Int("step", i).Str("task_id", task.ID).Msg("[agent] step failed, retrying once")
			r.publishEvent(task.UserID, TaskEvent{
				TaskID:    task.ID,
				EventType: "task_progress",
				StepIndex: i,
				Message:   taskStepRetryMessage(i),
			})
			output, err = r.executeStep(ctx, task, step)
		}
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				r.cancelTask(task, "task cancelled")
				return
			}
			if task.RuntimeState == RuntimeStateAborted {
				task.Status = TaskStatusAborted
				task.Progress = 100
				_ = r.store.Update(ctx, task)
				return
			}
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
	if ctx.Err() != nil {
		r.cancelTask(task, "task cancelled")
		return
	}

	// Phase 3: Auto-verification
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_progress",
		Progress:  95,
		Message:   taskVerifyingMessage(),
	})

	verificationCtx := compileVerificationContext(task)
	verificationPolicy := resolveRuntimeVerificationPolicy(task)

	if err := r.transitionState(ctx, task, RuntimeStateVerify, "verifying task result", nil, TaskStatusExecuting); err != nil {
		r.failTask(ctx, task, fmt.Sprintf("runtime transition failed before verify: %v", err))
		return
	}
	groundedVerifyStep := &PlanStep{
		Index:       len(task.Plan),
		Description: groundedVerificationStepDescription(verificationCtx.TaskKind, false),
		Status:      StepStatusRunning,
	}
	now := timeutil.NowTime()
	groundedVerifyStep.StartedAt = &now
	_, groundedVerifyOutput, groundedVerifyErr := r.groundedRuntime.VerifyTask(task)
	if errors.Is(ctx.Err(), context.Canceled) {
		r.cancelTask(task, "task cancelled")
		return
	}
	if groundedVerifyErr != nil {
		r.appendAudit(task, RuntimeAuditEvent{
			Timestamp: timeutil.NowTime(),
			Reason:    "grounded_verification_failed",
			Error:     groundedVerifyErr.Error(),
		})
		recordVerificationFailure(task, groundedVerifyOutput)
		groundedVerifyStep.Status = StepStatusFailed
		groundedVerifyStep.Output = groundedVerifyOutput
	} else {
		r.appendAudit(task, RuntimeAuditEvent{
			Timestamp: timeutil.NowTime(),
			Reason:    "grounded_verification_passed",
		})
		groundedVerifyStep.Status = StepStatusCompleted
		groundedVerifyStep.Output = groundedVerifyOutput
		task.VerifiedOutput = groundedVerifyOutput
	}
	completedAt := timeutil.NowTime()
	groundedVerifyStep.CompletedAt = &completedAt
	task.Plan = append(task.Plan, *groundedVerifyStep)
	_ = r.store.Update(ctx, task)
	if groundedVerifyStep.Status != StepStatusCompleted {
		r.failTask(ctx, task, "grounded verification failed")
		return
	}
	if ctx.Err() != nil {
		r.cancelTask(task, "task cancelled")
		return
	}

	if verificationPolicy.EnableExternalQA {
		externalVerifyStep := &PlanStep{
			Index:       len(task.Plan),
			Description: externalVerificationStepDescription(verificationCtx.TaskKind, false),
			Status:      StepStatusRunning,
		}
		externalStartedAt := timeutil.NowTime()
		externalVerifyStep.StartedAt = &externalStartedAt
		verificationResult, verificationOutput, verificationErr := r.runVerification(ctx, task, verificationCtx)
		if errors.Is(ctx.Err(), context.Canceled) {
			r.cancelTask(task, "task cancelled")
			return
		}
		externalCompletedAt := timeutil.NowTime()
		externalVerifyStep.CompletedAt = &externalCompletedAt
		externalVerifyStep.Output = verificationOutput
		if verificationErr != nil {
			externalVerifyStep.Status = StepStatusFailed
			recordVerificationFailure(task, verificationOutput)
			r.appendAudit(task, RuntimeAuditEvent{
				Timestamp: timeutil.NowTime(),
				Reason:    "external_verification_failed",
				Error:     verificationErr.Error(),
			})
		} else {
			externalVerifyStep.Status = StepStatusCompleted
			task.VerifiedOutput = verificationOutput
			r.appendAudit(task, RuntimeAuditEvent{
				Timestamp: timeutil.NowTime(),
				Reason:    "external_verification_passed",
			})
		}
		task.Plan = append(task.Plan, *externalVerifyStep)
		_ = r.store.Update(ctx, task)
		if verificationErr != nil {
			if verificationPolicy.MaxRecoveryAttempts <= 0 {
				r.failTask(ctx, task, "verification did not pass after bounded recovery retry")
				return
			}
			if err := r.transitionState(ctx, task, RuntimeStateRecover, "verification failed", nil, TaskStatusExecuting); err != nil {
				r.failTask(ctx, task, fmt.Sprintf("runtime transition failed before recovery: %v", err))
				return
			}
			recoveryStep := &PlanStep{
				Index:       len(task.Plan),
				Description: recoveryStepDescription(verificationCtx.TaskKind),
				Status:      StepStatusRunning,
			}
			recoveryStartedAt := timeutil.NowTime()
			recoveryStep.StartedAt = &recoveryStartedAt
			recoveryOutput, recoveryErr := r.runRecovery(ctx, task, verificationCtx, verificationResult)
			if errors.Is(ctx.Err(), context.Canceled) {
				r.cancelTask(task, "task cancelled")
				return
			}
			recoveryCompletedAt := timeutil.NowTime()
			recoveryStep.CompletedAt = &recoveryCompletedAt
			recoveryStep.Output = recoveryOutput
			if recoveryErr != nil {
				recoveryStep.Status = StepStatusFailed
				recordVerificationFailure(task, recoveryOutput)
				r.appendAudit(task, RuntimeAuditEvent{
					Timestamp: timeutil.NowTime(),
					Reason:    "recovery_failed",
					Error:     recoveryErr.Error(),
				})
			} else {
				recoveryStep.Status = StepStatusCompleted
				r.appendAudit(task, RuntimeAuditEvent{
					Timestamp: timeutil.NowTime(),
					Reason:    "recovery_applied",
				})
			}
			task.Plan = append(task.Plan, *recoveryStep)
			_ = r.store.Update(ctx, task)
			if recoveryErr != nil {
				r.failTask(ctx, task, fmt.Sprintf("verification failed and recovery failed: %v", recoveryErr))
				return
			}
			if err := r.transitionState(ctx, task, RuntimeStateVerify, "re-verify after recovery", nil, TaskStatusExecuting); err != nil {
				r.failTask(ctx, task, fmt.Sprintf("runtime transition failed before re-verify: %v", err))
				return
			}

			retryGroundedVerifyStep := &PlanStep{
				Index:       len(task.Plan),
				Description: groundedVerificationStepDescription(verificationCtx.TaskKind, true),
				Status:      StepStatusRunning,
			}
			retryGroundedStartedAt := timeutil.NowTime()
			retryGroundedVerifyStep.StartedAt = &retryGroundedStartedAt
			_, retryGroundedOutput, retryGroundedErr := r.groundedRuntime.VerifyTask(task)
			if errors.Is(ctx.Err(), context.Canceled) {
				r.cancelTask(task, "task cancelled")
				return
			}
			retryGroundedCompletedAt := timeutil.NowTime()
			retryGroundedVerifyStep.CompletedAt = &retryGroundedCompletedAt
			retryGroundedVerifyStep.Output = retryGroundedOutput
			if retryGroundedErr != nil {
				retryGroundedVerifyStep.Status = StepStatusFailed
				recordVerificationFailure(task, retryGroundedOutput)
				r.appendAudit(task, RuntimeAuditEvent{
					Timestamp: timeutil.NowTime(),
					Reason:    "grounded_verification_failed_after_recovery",
					Error:     retryGroundedErr.Error(),
				})
			} else {
				retryGroundedVerifyStep.Status = StepStatusCompleted
				task.VerifiedOutput = retryGroundedOutput
				r.appendAudit(task, RuntimeAuditEvent{
					Timestamp: timeutil.NowTime(),
					Reason:    "grounded_verification_passed_after_recovery",
				})
			}
			task.Plan = append(task.Plan, *retryGroundedVerifyStep)
			_ = r.store.Update(ctx, task)
			if retryGroundedErr != nil {
				r.failTask(ctx, task, "verification did not pass after bounded recovery retry")
				return
			}

			retryExternalVerifyStep := &PlanStep{
				Index:       len(task.Plan),
				Description: externalVerificationStepDescription(verificationCtx.TaskKind, true),
				Status:      StepStatusRunning,
			}
			retryExternalStartedAt := timeutil.NowTime()
			retryExternalVerifyStep.StartedAt = &retryExternalStartedAt
			_, retryVerificationOutput, retryVerificationErr := r.runVerification(ctx, task, verificationCtx)
			if errors.Is(ctx.Err(), context.Canceled) {
				r.cancelTask(task, "task cancelled")
				return
			}
			retryExternalCompletedAt := timeutil.NowTime()
			retryExternalVerifyStep.CompletedAt = &retryExternalCompletedAt
			retryExternalVerifyStep.Output = retryVerificationOutput
			if retryVerificationErr != nil {
				retryExternalVerifyStep.Status = StepStatusFailed
				recordVerificationFailure(task, retryVerificationOutput)
				r.appendAudit(task, RuntimeAuditEvent{
					Timestamp: timeutil.NowTime(),
					Reason:    "external_verification_failed_after_recovery",
					Error:     retryVerificationErr.Error(),
				})
			} else {
				retryExternalVerifyStep.Status = StepStatusCompleted
				task.VerifiedOutput = retryVerificationOutput
				r.appendAudit(task, RuntimeAuditEvent{
					Timestamp: timeutil.NowTime(),
					Reason:    "external_verification_passed_after_recovery",
				})
			}
			task.Plan = append(task.Plan, *retryExternalVerifyStep)
			_ = r.store.Update(ctx, task)
			if retryVerificationErr != nil {
				r.failTask(ctx, task, "verification did not pass after bounded recovery retry")
				return
			}
		}
	} else {
		r.appendAudit(task, RuntimeAuditEvent{
			Timestamp: timeutil.NowTime(),
			Reason:    "external_verification_skipped",
		})
	}

	task.Result = buildBaseResultSummary(task, TaskStatusCompleted, "")
	reflection := r.runReflection(ctx, task, TaskStatusCompleted, "")
	if ctx.Err() != nil {
		r.cancelTask(task, "task cancelled")
		return
	}

	// Phase 4: Summarize and suggest next steps
	if err := r.transitionState(ctx, task, RuntimeStateReport, "generating final report", nil, TaskStatusExecuting); err != nil {
		r.failTask(ctx, task, fmt.Sprintf("runtime transition failed before report: %v", err))
		return
	}
	task.Status = TaskStatusCompleted
	task.Progress = 100
	task.Result = buildGroundedTaskReport(task)
	if learned := formatLearnedSection(reflection); learned != "" {
		task.Result = task.Result + "\n\n" + learned
	}
	task.VerifiedOutput = task.Result
	if ctx.Err() != nil {
		r.cancelTask(task, "task cancelled")
		return
	}
	_ = r.store.Update(ctx, task)
	if err := r.transitionState(ctx, task, RuntimeStateDone, "task completed", nil, TaskStatusCompleted); err != nil {
		r.failTask(ctx, task, fmt.Sprintf("runtime transition failed before done: %v", err))
		return
	}

	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_completed",
		Progress:  100,
		Message:   task.Result,
	})
}

type planResult struct {
	Goal            string
	Raw             runtimePlan
	Steps           []PlanStep
	SuccessCriteria []string
	FallbackPlan    []string
}

func buildPlanningSystemPrompt(memoryCtx string) string {
	var sb strings.Builder
	sb.WriteString("You are a deterministic task planner for ZimaOS Blue. Plan the work before execution.\n\n")
	sb.WriteString("## Output Contract\n")
	sb.WriteString("Return ONLY one JSON object in this exact shape:\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"goal\": \"<normalized goal>\",\n")
	sb.WriteString("  \"subtasks\": [{\"description\":\"...\"}],\n")
	sb.WriteString("  \"requires_confirmation\": [\"...\"],\n")
	sb.WriteString("  \"success_criteria\": [\"...\"],\n")
	sb.WriteString("  \"fallback_plan\": [\"...\"]\n")
	sb.WriteString("}\n")
	sb.WriteString("Always include all keys. Use [] when a list is empty.\n")

	sb.WriteString("\n## Planning Rules\n")
	sb.WriteString("- Create 3-10 ordered subtasks. Each one must be concrete, actionable, and execution-ready.\n")
	sb.WriteString("- Prefer the smallest plan that can fully satisfy the goal.\n")
	sb.WriteString("- Preserve explicit user constraints, scope limits, and acceptance signals.\n")
	sb.WriteString("- Avoid duplicate, vague, or overlapping subtasks.\n")
	sb.WriteString("- Use requires_confirmation only for real user-choice, destructive, irreversible, privileged, or high-risk gates.\n")
	sb.WriteString("- Make success_criteria observable and testable.\n")
	sb.WriteString("- Make fallback_plan safe, bounded, and finite; no loops, no open-ended retries, and no retrying the same action without a change.\n")
	sb.WriteString("- Normalize the goal, but do not invent missing requirements.\n")
	sb.WriteString("- If the task may write a large file, plan to split content into smaller write chunks and continue with append=true instead of one huge write payload.\n")

	sb.WriteString("\n## Output Rules\n")
	sb.WriteString("- No markdown, no code fences, no commentary, and no prose outside the JSON object.\n")
	sb.WriteString("- Each subtask object must contain a non-empty description string.\n")
	sb.WriteString("- Keep wording concise and execution-oriented.\n")

	if strings.TrimSpace(memoryCtx) != "" {
		sb.WriteString("\n## Relevant Context\n")
		sb.WriteString(memoryCtx)
	}

	return strings.TrimSpace(sb.String())
}

// generatePlan asks the LLM to create a structured plan.
func (r *Runner) generatePlan(ctx context.Context, goal, conversationCtx string) (*planResult, error) {
	// Recall relevant memories for context
	memoryCtx := r.recallMemories(ctx, goal)

	systemPrompt := buildPlanningSystemPrompt(memoryCtx)

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
		Temperature: 0.2,
	})
	if err != nil {
		return nil, err
	}

	parsed, err := parseRuntimePlan(resp.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w (content: %s)", err, truncate(strings.TrimSpace(resp.Message.Content), 200))
	}
	steps := make([]PlanStep, len(parsed.Subtasks))
	for i, rs := range parsed.Subtasks {
		steps[i] = PlanStep{
			Index:       i,
			Description: rs,
			Status:      StepStatusPending,
		}
	}
	return &planResult{
		Goal:            parsed.Goal,
		Raw:             parsed,
		Steps:           steps,
		SuccessCriteria: parsed.SuccessCriteria,
		FallbackPlan:    parsed.FallbackPlan,
	}, nil
}

// executeStep runs a single plan step using the LLM + tools.
func (r *Runner) executeStep(ctx context.Context, task *Task, step *PlanStep) (string, error) {
	ctx = tools.WithRunID(ctx, task.ID)
	ctx = tools.WithRunStep(ctx, step.Index)
	ctx = tools.WithSubagentExecutor(ctx, r.subagents)
	ctx = tools.WithWritePathGuard(ctx, r.writeGuard)
	ctx = tools.WithExecPathGuard(ctx, r.execGuard)
	if workspaceRoot := strings.TrimSpace(task.WorkspaceRoot); workspaceRoot != "" {
		ctx = tools.WithFSRootOverride(ctx, []string{workspaceRoot}, map[string]string{
			"workspace": workspaceRoot,
		})
	}
	if r.groundedRuntime == nil {
		// Backward-compatible fallback used by focused prompt/unit tests that
		// construct a Runner manually without the grounded runtime wiring.
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

		cachedTools := r.llmTools()
		return r.executeLoopWithTools(
			ctx,
			task,
			step.Index,
			step.Description,
			buildStepExecutionSystemPrompt(cachedTools),
			contextMsg.String(),
			cachedTools,
		)
	}
	result, err := r.groundedRuntime.ExecuteStep(ctx, task, *step, task.Plan, r.resolveMaxToolRoundsPerStep())
	if result != nil {
		task.VerifiedOutput = result.VerifiedOutput
		task.VerificationErrors = mergeVerificationErrors(task.VerificationErrors, result.VerificationErrors)
		task.GroundingStatus = combineGroundingStatus(task.GroundingStatus, result.GroundingStatus)
		if strings.TrimSpace(result.Output) != "" {
			return result.Output, err
		}
	}
	if err != nil {
		return "", err
	}
	return "unknown", nil
}

func buildStepExecutionSystemPrompt(tools []llm.Tool) string {
	var sb strings.Builder
	sb.WriteString("You are an autonomous agent executing a single plan step inside ZimaOS Blue. Finish the current step using the available tools, then stop and report the result.\n\n")
	sb.WriteString("## Tooling\n")
	sb.WriteString("Tool names are case-sensitive. Call tools exactly as listed.\n")
	if len(tools) == 0 {
		sb.WriteString("No first-class tools are available in this run; rely on direct reasoning only.\n")
	} else {
		sb.WriteString("Tool availability:\n")
		for _, tool := range tools {
			sb.WriteString("- ")
			sb.WriteString(tool.Name)
			if desc := strings.TrimSpace(tool.Description); desc != "" {
				sb.WriteString(": ")
				sb.WriteString(desc)
			}
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("Prefer first-class tools over shell commands when they directly cover the action.\n")
	sb.WriteString("Do not depend on 'blue' CLI subcommands. If CLI-specific commands are unavailable, fall back to standard shell commands and available file/workspace tools.\n")
	if hasStepTool(tools, "exec") && hasStepTool(tools, "process") {
		sb.WriteString("For long-running commands, use exec with a sensible timeout and inspect the returned session_id with process; avoid tight polling loops or repeating the same command without a change.\n")
	}

	sb.WriteString("\n## Tool Call Style\n")
	sb.WriteString("Default: do not narrate routine, low-risk tool calls; just call the tool.\n")
	sb.WriteString("Narrate only when it helps: multi-step work, complex problems, sensitive actions, or when the user explicitly asks.\n")
	sb.WriteString("Keep narration brief and value-dense; avoid repeating obvious steps.\n")

	sb.WriteString("\n## Step Execution\n")
	sb.WriteString("Stay focused on the current step while using the full plan as context.\n")
	sb.WriteString("If a tool fails, inspect the result, adjust the approach, and avoid repeating identical failing calls.\n")
	sb.WriteString("When the step is complete or blocked, stop calling tools and explain the result or blocker concisely.\n")

	sb.WriteString("\n## Coding Task Defaults\n")
	sb.WriteString("For coding tasks, first check whether mainstream skills are available: superpowers and ui-ux-pro-max-skill.\n")
	sb.WriteString("If missing, use available skill-install workflow before implementation; if blocked or unavailable, note it once and continue with best effort.\n")
	sb.WriteString("If stack preferences are unclear, ask once and remember them for future steps.\n")
	sb.WriteString("Default stack when not specified: backend Go, frontend React, mobile React Native, client Electron.\n")
	sb.WriteString("If the repository/runtime already implies a stack (for example Python or Node.js), follow the existing environment instead of forcing defaults.\n")

	if hasStepTool(tools, "ask") {
		sb.WriteString("\n## Ask Tool\n")
		sb.WriteString("When you encounter ambiguity, need user preferences, or face multiple valid options, use the ask tool instead of guessing.\n")
		sb.WriteString("Preferred format: {\"questions\":[{\"question\":\"Which approach?\",\"type\":\"radio\",\"options\":[\"Option A\",\"Option B\"]}]}\n")
		sb.WriteString("Text-input format: {\"questions\":[{\"question\":\"What details should I use?\",\"type\":\"text\"}]}\n")
		sb.WriteString("Single-question shorthand: {\"q\":\"Which approach?\",\"a\":[\"Option A\",\"Option B\"]} or {\"mq\":\"...\",\"a\":[...]}\n")
		sb.WriteString("Optional extra detail: set \"detail\" and it will be shown with a ❕ marker.\n")
		sb.WriteString("Inside questions items, use only \"question\"/\"detail\"/\"type\"/\"options\".\n")
		sb.WriteString("Group related questions into a single ask call. Keep questions clear and provide good option labels.\n")
	}

	return strings.TrimSpace(sb.String())
}

var errNoProgressAbort = errors.New("agent stopped due to no progress")

func (r *Runner) executeLoopWithTools(ctx context.Context, task *Task, stepIndex int, actionDescription, systemPrompt, userPrompt string, cachedTools []llm.Tool) (string, error) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: userPrompt},
	}

	var lastContent string
	var lastToolSig string
	var repeatCount int
	const maxRepeats = 3

	progressState := ProgressSignatureState{}
	maxRounds := r.resolveMaxToolRoundsPerStep()
	for round := 0; round < maxRounds; round++ {
		if ctx.Err() != nil {
			return lastContent, ctx.Err()
		}

		if injected := r.drainMessages(task.ID); len(injected) > 0 {
			combined := strings.Join(injected, "\n")
			messages = append(messages, llm.Message{
				Role:    llm.RoleUser,
				Content: "[User message during execution]: " + combined,
			})
			r.publishEvent(task.UserID, TaskEvent{
				TaskID:    task.ID,
				EventType: "task_user_message",
				StepIndex: stepIndex,
				Message:   taskUserUpdateMessage(combined),
			})
			logger.Info().Str("task_id", task.ID).Int("step", stepIndex).Int("round", round).Msg("[agent] injected user messages between tool rounds")
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

		lastContent = strings.TrimSpace(resp.Message.Content)
		if len(resp.Message.ToolCalls) == 0 {
			break
		}

		sig := toolCallSignature(resp.Message.ToolCalls)
		if sig == lastToolSig {
			repeatCount++
			if repeatCount >= maxRepeats {
				r.appendAudit(task, RuntimeAuditEvent{
					Timestamp: timeutil.NowTime(),
					Reason:    "no_progress_abort",
					Error:     "repeated identical tool calls",
				})
				lastContent = strings.TrimSpace(lastContent + "\n[Agent stopped: repeated identical tool calls detected]")
				return lastContent, errNoProgressAbort
			}
		} else {
			lastToolSig = sig
			repeatCount = 0
		}

		messages = append(messages, resp.Message)
		toolSummaries := make([]string, 0, len(resp.Message.ToolCalls))
		for _, tc := range resp.Message.ToolCalls {
			capability := classifyCapability(tc.Name, tc.Arguments)
			if shouldRequireConfirm(capability, actionDescription+" "+lastContent) {
				if err := r.transitionState(ctx, task, RuntimeStateConfirmGate, "high-risk capability requires confirmation", &capability, TaskStatusWaitingInput); err != nil {
					return lastContent, err
				}
				ans, askErr := r.AskUser(ctx, task.ID, buildHighRiskConfirmationQuestions(tc.Name, capability, actionDescription), stepIndex)
				if askErr != nil {
					return lastContent, askErr
				}
				decision := "skip"
				if len(ans) > 0 && len(ans[0].Values) > 0 {
					decision = ans[0].Values[0]
				}
				if decision == "abort" {
					if err := r.transitionState(ctx, task, RuntimeStateAborted, "user aborted on high-risk tool call", &capability, TaskStatusAborted); err != nil {
						return lastContent, err
					}
					return lastContent, fmt.Errorf("user aborted task during high-risk confirmation")
				}
				if decision == "skip" {
					if err := r.transitionState(ctx, task, RuntimeStateExecute, "user skipped high-risk tool call", &capability, TaskStatusExecuting); err != nil {
						return lastContent, err
					}
					continue
				}
				if err := r.transitionState(ctx, task, RuntimeStateExecute, "high-risk tool call approved", &capability, TaskStatusExecuting); err != nil {
					return lastContent, err
				}
			}

			var content string
			if tc.Name == "ask" {
				content = r.handleAskUser(ctx, task, tc.Arguments)
			} else {
				var (
					result  interface{}
					execErr error
				)
				if r.toolGateway != nil {
					gatewayResult, gatewayErr := r.toolGateway.Execute(ctx, tools.ToolGatewayRequest{
						ToolCallID: tc.ID,
						ToolName:   tc.Name,
						Arguments:  tc.Arguments,
						SessionID:  strings.TrimSpace(task.ConversationID),
						RouteKind:  tools.ToolRouteKindAgent,
						UserID:     strings.TrimSpace(task.UserID),
					})
					execErr = gatewayErr
					if gatewayResult != nil {
						if gatewayErr == nil && gatewayResult.ExecutionResult != nil {
							result = gatewayResult.ExecutionResult
						} else if gatewayResult.AuditPayload != nil {
							result = gatewayResult.AuditPayload
						}
					}
				} else if r.executor != nil {
					result, execErr = r.executor.ExecuteJSON(ctx, tc.Name, tc.Arguments)
				} else {
					execErr = fmt.Errorf("tool executor is unavailable")
				}
				if execErr != nil {
					errObj, _ := json.Marshal(tools.ToolErrorPayload(execErr))
					content = string(errObj)
				} else {
					switch v := result.(type) {
					case string:
						content = v
					default:
						b, marshalErr := json.Marshal(v)
						if marshalErr != nil {
							content = tools.SafeToolPayloadString(v, 64*1024)
						} else {
							content = string(b)
						}
					}
				}
			}
			r.appendAudit(task, RuntimeAuditEvent{
				Timestamp:  timeutil.NowTime(),
				Reason:     "tool_call_executed",
				Capability: &capability,
			})
			content = truncate(content, 8000)
			toolSummaries = append(toolSummaries, normalizeProgressSummary(content))
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    content,
				ToolCallID: tc.ID,
			})
		}

		if progressState.Observe(sig, lastContent, toolSummaries) {
			r.appendAudit(task, RuntimeAuditEvent{
				Timestamp: timeutil.NowTime(),
				Reason:    "no_progress_abort",
				Error:     strings.Join(toolSummaries, " | "),
			})
			lastContent = strings.TrimSpace(lastContent + "\n[Agent stopped: no progress detected after repeated tool rounds]")
			return lastContent, errNoProgressAbort
		}
	}

	return lastContent, nil
}

func buildVerificationSystemPrompt(kind TaskKind, tools []llm.Tool) string {
	var sb strings.Builder
	sb.WriteString("You are the strict verification engine for ZimaOS Blue.\n\n")
	sb.WriteString("## Output Contract\n")
	sb.WriteString("Return ONLY one JSON object in this exact shape:\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"status\": \"pass|fail\",\n")
	sb.WriteString("  \"summary\": \"<brief verification summary>\",\n")
	sb.WriteString("  \"criteria_results\": [{\"criterion\":\"...\",\"status\":\"pass|fail\",\"evidence\":\"...\"}],\n")
	sb.WriteString("  \"suggested_recovery\": \"<optional short next action>\",\n")
	sb.WriteString("  \"executed_checks\": [\"...\"]\n")
	sb.WriteString("}\n")
	sb.WriteString("Always include status, summary, criteria_results, and executed_checks. If evidence is missing or uncertain, return fail.\n")

	sb.WriteString("\n## Verification Rules\n")
	sb.WriteString("- Evaluate every provided success criterion explicitly.\n")
	sb.WriteString("- Use tools when needed, but stop once you have enough evidence.\n")
	sb.WriteString("- Do not mark a criterion as pass without concrete evidence from the task record or tool output.\n")
	switch kind {
	case TaskKindCode:
		sb.WriteString("- For code tasks, confirm the intended deliverable landed in files or behavior before running relevant build/test/check commands.\n")
	case TaskKindDocs:
		sb.WriteString("- For docs tasks, verify the requested document or copy change exists and covers the requested scope. Do not default to build or tests unless the criteria require them.\n")
	case TaskKindResearch:
		sb.WriteString("- For research tasks, verify coverage, evidence quality, and unsupported claims. Do not default to build or tests.\n")
	case TaskKindOps:
		sb.WriteString("- For ops tasks, prefer service, process, config, and log validation. Only run build or tests when the criteria require them.\n")
	default:
		sb.WriteString("- For generic tasks, verify the requested deliverable and scope coverage first. Do not default to build or tests unless the criteria require them.\n")
	}

	if len(tools) > 0 {
		sb.WriteString("\n## Available Tools\n")
		for _, tool := range tools {
			sb.WriteString("- ")
			sb.WriteString(tool.Name)
			if desc := strings.TrimSpace(tool.Description); desc != "" {
				sb.WriteString(": ")
				sb.WriteString(desc)
			}
			sb.WriteByte('\n')
		}
	}

	sb.WriteString("\n## Constraints\n")
	sb.WriteString("- No markdown, no code fences, and no prose outside the JSON object.\n")
	sb.WriteString("- unsupported statuses like unknown/partial are forbidden.\n")
	sb.WriteString("- executed_checks must describe the real checks you performed.\n")
	return strings.TrimSpace(sb.String())
}

func buildVerificationUserPrompt(task *Task, verificationCtx VerificationContext) string {
	var sb strings.Builder
	sb.WriteString("Goal: ")
	sb.WriteString(strings.TrimSpace(verificationCtx.Goal))
	sb.WriteString("\n")
	sb.WriteString("Task kind: ")
	sb.WriteString(string(verificationCtx.TaskKind))
	sb.WriteString("\n")
	if len(verificationCtx.SuccessCriteria) > 0 {
		sb.WriteString("Success criteria:\n")
		for _, criterion := range verificationCtx.SuccessCriteria {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(criterion))
			sb.WriteString("\n")
		}
	}
	if len(verificationCtx.FallbackPlan) > 0 {
		sb.WriteString("Fallback plan:\n")
		for _, item := range verificationCtx.FallbackPlan {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(item))
			sb.WriteString("\n")
		}
	}
	sb.WriteString("Task record:\n")
	for _, step := range task.Plan {
		status := strings.ToUpper(strings.TrimSpace(string(step.Status)))
		if status == "" {
			status = "PENDING"
		}
		sb.WriteString("- [")
		sb.WriteString(status)
		sb.WriteString("] ")
		sb.WriteString(strings.TrimSpace(step.Description))
		if output := strings.TrimSpace(step.Output); output != "" {
			sb.WriteString(": ")
			sb.WriteString(truncate(output, 240))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("Verify every success criterion explicitly and fail if any criterion lacks concrete evidence.")
	return strings.TrimSpace(sb.String())
}

func (r *Runner) runVerification(ctx context.Context, task *Task, verificationCtx VerificationContext) (*VerificationResult, string, error) {
	cachedTools := r.llmTools()
	rawOutput, loopErr := r.executeLoopWithTools(
		ctx,
		task,
		len(task.Plan),
		"Verify the completed work",
		buildVerificationSystemPrompt(verificationCtx.TaskKind, cachedTools),
		buildVerificationUserPrompt(task, verificationCtx),
		cachedTools,
	)
	if loopErr != nil {
		synthetic := syntheticVerificationFailure(verificationCtx, "Verification execution failed.", loopErr.Error())
		ordered, _, missing := evaluateVerificationResult(synthetic, verificationCtx.SuccessCriteria)
		return synthetic, formatVerificationOutput(verificationCtx, synthetic, ordered, missing), fmt.Errorf("verification loop failed: %w", loopErr)
	}

	parsed, err := parseVerificationResult(rawOutput)
	if err != nil {
		synthetic := syntheticVerificationFailure(verificationCtx, "Verification contract was invalid.", err.Error())
		ordered, failed, missing := evaluateVerificationResult(synthetic, verificationCtx.SuccessCriteria)
		_ = failed
		return synthetic, formatVerificationOutput(verificationCtx, synthetic, ordered, missing), fmt.Errorf("invalid verification result: %w", err)
	}

	ordered, failed, missing := evaluateVerificationResult(parsed, verificationCtx.SuccessCriteria)
	output := formatVerificationOutput(verificationCtx, parsed, ordered, missing)
	if verificationPassed(parsed, failed, missing) {
		return parsed, output, nil
	}
	return parsed, output, fmt.Errorf("verification criteria not satisfied")
}

func buildRecoverySystemPrompt(kind TaskKind, tools []llm.Tool) string {
	var sb strings.Builder
	sb.WriteString("You are the bounded recovery engine for ZimaOS Blue.\n\n")
	sb.WriteString("## Recovery Rules\n")
	sb.WriteString("- Make one focused recovery attempt only.\n")
	sb.WriteString("- Use the fallback plan in the exact order provided.\n")
	sb.WriteString("- The first fallback item is the primary action. Do not skip it unless it is impossible.\n")
	sb.WriteString("- If verifier suggested_recovery differs from the first fallback item, treat it as an extra constraint, not a replacement.\n")
	sb.WriteString("- Focus only on failed verification criteria.\n")
	sb.WriteString("- Stop after the recovery attempt and summarize what changed or why it could not be applied.\n")
	switch kind {
	case TaskKindCode:
		sb.WriteString("- For code tasks, prefer the smallest change that can satisfy the failed criteria.\n")
	case TaskKindDocs:
		sb.WriteString("- For docs tasks, change only the requested document content needed to satisfy the failed criteria.\n")
	case TaskKindResearch:
		sb.WriteString("- For research tasks, improve evidence quality or coverage; do not pad the report with unsupported claims.\n")
	case TaskKindOps:
		sb.WriteString("- For ops tasks, prioritize safe config, process, or runtime checks before broader actions.\n")
	default:
		sb.WriteString("- For generic tasks, prioritize the minimum change needed to satisfy the failed criteria.\n")
	}
	if len(tools) > 0 {
		sb.WriteString("\n## Available Tools\n")
		for _, tool := range tools {
			sb.WriteString("- ")
			sb.WriteString(tool.Name)
			if desc := strings.TrimSpace(tool.Description); desc != "" {
				sb.WriteString(": ")
				sb.WriteString(desc)
			}
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("\n## Constraints\n")
	sb.WriteString("- No markdown headers, no code fences, and no open-ended retry plan.\n")
	return strings.TrimSpace(sb.String())
}

func buildRecoveryUserPrompt(task *Task, verificationCtx VerificationContext, verificationResult *VerificationResult) string {
	var sb strings.Builder
	sb.WriteString("Goal: ")
	sb.WriteString(strings.TrimSpace(verificationCtx.Goal))
	sb.WriteString("\n")
	sb.WriteString("Task kind: ")
	sb.WriteString(string(verificationCtx.TaskKind))
	sb.WriteString("\n")
	if verificationResult != nil {
		sb.WriteString("Verification summary: ")
		sb.WriteString(strings.TrimSpace(verificationResult.Summary))
		sb.WriteString("\n")
	}
	ordered, _, missing := evaluateVerificationResult(verificationResult, verificationCtx.SuccessCriteria)
	if len(ordered) > 0 {
		sb.WriteString("Failed criteria:\n")
		for _, criterion := range ordered {
			if criterion.Status == "pass" {
				continue
			}
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(criterion.Criterion))
			if evidence := strings.TrimSpace(criterion.Evidence); evidence != "" {
				sb.WriteString(": ")
				sb.WriteString(truncate(evidence, 220))
			}
			sb.WriteString("\n")
		}
	}
	if len(missing) > 0 {
		sb.WriteString("Missing criteria coverage:\n")
		for _, criterion := range missing {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(criterion))
			sb.WriteString("\n")
		}
	}
	if len(verificationCtx.FallbackPlan) > 0 {
		sb.WriteString("Ordered fallback plan:\n")
		for i, item := range verificationCtx.FallbackPlan {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(item)))
		}
	}
	if verificationResult != nil && strings.TrimSpace(verificationResult.SuggestedRecovery) != "" {
		sb.WriteString("Verifier suggested recovery (supporting constraint only): ")
		sb.WriteString(strings.TrimSpace(verificationResult.SuggestedRecovery))
		sb.WriteString("\n")
	}
	if summaries := recentSuccessfulStepSummaries(task, 3); len(summaries) > 0 {
		sb.WriteString("Recent successful steps:\n")
		for _, summary := range summaries {
			sb.WriteString("- ")
			sb.WriteString(summary)
			sb.WriteString("\n")
		}
	}
	sb.WriteString("Apply the first fallback item as the primary action and stop after one recovery attempt.")
	return strings.TrimSpace(sb.String())
}

func (r *Runner) runRecovery(ctx context.Context, task *Task, verificationCtx VerificationContext, verificationResult *VerificationResult) (string, error) {
	cachedTools := r.llmTools()
	return r.executeLoopWithTools(
		ctx,
		task,
		len(task.Plan),
		"Recovery: retry failed validation with safer fallback path",
		buildRecoverySystemPrompt(verificationCtx.TaskKind, cachedTools),
		buildRecoveryUserPrompt(task, verificationCtx, verificationResult),
		cachedTools,
	)
}

type runtimeVerificationPolicy struct {
	EnableExternalQA    bool
	MaxRecoveryAttempts int
}

func resolveRuntimeVerificationPolicy(task *Task) runtimeVerificationPolicy {
	policy := runtimeVerificationPolicy{}
	if task == nil {
		return policy
	}
	sources := taskMetadataSources(task)
	if enabled, ok := metadataBoolFromMaps(sources, "enable_external_qa", "external_qa_enabled"); ok {
		policy.EnableExternalQA = enabled
	}
	if attempts, ok := metadataIntFromMaps(sources, "max_recovery_attempts"); ok {
		policy.MaxRecoveryAttempts = attempts
	}
	if policy.MaxRecoveryAttempts < 0 {
		policy.MaxRecoveryAttempts = 0
	}
	if policy.MaxRecoveryAttempts > 1 {
		policy.MaxRecoveryAttempts = 1
	}
	if policy.EnableExternalQA && policy.MaxRecoveryAttempts == 0 {
		policy.MaxRecoveryAttempts = 1
	}
	return policy
}

func mergeTaskSuccessCriteria(task *Task, planned []string) []string {
	values := append([]string(nil), planned...)
	for _, source := range taskMetadataSources(task) {
		values = append(metadataStringSlice(source, "task_success_criteria"), values...)
		values = append(metadataStringSlice(source, "success_criteria"), values...)
		values = append(metadataStringSlice(source, "deliverables"), values...)
	}
	if task != nil {
		values = append(append([]string(nil), task.SuccessCriteria...), values...)
	}
	return effectiveSuccessCriteria(values)
}

func mergeTaskFallbackPlan(task *Task, planned []string) []string {
	values := append([]string(nil), planned...)
	for _, source := range taskMetadataSources(task) {
		values = append(metadataStringSlice(source, "task_fallback_plan"), values...)
		values = append(metadataStringSlice(source, "fallback_order"), values...)
		values = append(metadataStringSlice(source, "fallback_plan"), values...)
	}
	if task != nil {
		values = append(append([]string(nil), task.FallbackPlan...), values...)
	}
	return effectiveFallbackPlan(values)
}

func taskMetadataSources(task *Task) []map[string]interface{} {
	if task == nil || len(task.Metadata) == 0 {
		return nil
	}
	sources := []map[string]interface{}{task.Metadata}
	for _, key := range []string{"runtime_adaptation", "verification_policy", "harness_contract", "resume_checkpoint"} {
		if nested := metadataMapValue(task.Metadata, key); len(nested) > 0 {
			sources = append(sources, nested)
		}
	}
	return sources
}

func metadataMapValue(meta map[string]interface{}, key string) map[string]interface{} {
	if len(meta) == 0 {
		return nil
	}
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	typed, _ := raw.(map[string]interface{})
	if len(typed) == 0 {
		return nil
	}
	return typed
}

func metadataBoolFromMaps(sources []map[string]interface{}, keys ...string) (bool, bool) {
	for _, source := range sources {
		for _, key := range keys {
			raw, ok := source[key]
			if !ok {
				continue
			}
			if typed, ok := raw.(bool); ok {
				return typed, true
			}
		}
	}
	return false, false
}

func metadataIntFromMaps(sources []map[string]interface{}, keys ...string) (int, bool) {
	for _, source := range sources {
		for _, key := range keys {
			raw, ok := source[key]
			if !ok {
				continue
			}
			switch value := raw.(type) {
			case int:
				return value, true
			case int32:
				return int(value), true
			case int64:
				return int(value), true
			case float64:
				return int(value), true
			case float32:
				return int(value), true
			}
		}
	}
	return 0, false
}

func metadataStringSlice(meta map[string]interface{}, key string) []string {
	if len(meta) == 0 {
		return nil
	}
	raw, ok := meta[key]
	if !ok {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return dedupeStrings(values)
	case []interface{}:
		out := make([]string, 0, len(values))
		for _, item := range values {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				out = append(out, text)
			}
		}
		return dedupeStrings(out)
	default:
		if text := strings.TrimSpace(fmt.Sprint(values)); text != "" {
			return []string{text}
		}
		return nil
	}
}

func recordVerificationFailure(task *Task, output string) {
	if task == nil {
		return
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return
	}
	for _, existing := range task.VerificationErrors {
		if strings.TrimSpace(existing) == output {
			return
		}
	}
	task.VerificationErrors = append(task.VerificationErrors, output)
}

func groundedVerificationStepDescription(kind TaskKind, retry bool) string {
	return verificationStepDescription(kind, retry) + " [grounded]"
}

func externalVerificationStepDescription(kind TaskKind, retry bool) string {
	return verificationStepDescription(kind, retry) + " [external QA]"
}

func recentSuccessfulStepSummaries(task *Task, limit int) []string {
	if task == nil || limit <= 0 {
		return nil
	}
	out := make([]string, 0, limit)
	for i := len(task.Plan) - 1; i >= 0 && len(out) < limit; i-- {
		step := task.Plan[i]
		if step.Status != StepStatusCompleted {
			continue
		}
		summary := strings.TrimSpace(step.Description)
		if output := strings.TrimSpace(step.Output); output != "" {
			summary += ": " + truncate(output, 160)
		}
		out = append(out, summary)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func hasStepTool(tools []llm.Tool, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func buildBaseResultSummary(task *Task, finalStatus TaskStatus, failureReason string) string {
	if task == nil {
		return ""
	}
	completed, failed, skipped := 0, 0, 0
	for _, step := range task.Plan {
		switch step.Status {
		case StepStatusCompleted:
			completed++
		case StepStatusFailed:
			failed++
		case StepStatusSkipped:
			skipped++
		}
	}
	switch finalStatus {
	case TaskStatusFailed:
		return fmt.Sprintf(
			"Goal %q failed after %d completed step(s), %d failed step(s), and %d skipped step(s). Failure reason: %s",
			strings.TrimSpace(task.Goal),
			completed,
			failed,
			skipped,
			strings.TrimSpace(failureReason),
		)
	default:
		return fmt.Sprintf(
			"Goal %q completed after %d completed step(s); verification passed for %d success criterion/criteria.",
			strings.TrimSpace(task.Goal),
			completed,
			len(effectiveSuccessCriteria(task.SuccessCriteria)),
		)
	}
}

func buildSummarySystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are writing the final user-facing report for a completed ZimaOS Blue agent task.\n\n")
	sb.WriteString("## Output Contract\n")
	sb.WriteString("Respond in plain text using exactly this structure:\n")
	sb.WriteString("Summary: <2-3 concise sentences about the outcome>\n\n")
	sb.WriteString("Learned:\n")
	sb.WriteString("- <optional short reusable lesson>\n")
	sb.WriteString("Include the Learned block only when grounded lessons are provided. Use 1-3 bullets total.\n\n")
	sb.WriteString("If you'd like, I can also help with:\n")
	sb.WriteString("1. If you'd like, I can help you <specific follow-up action>.\n")
	sb.WriteString("2. If you want, I can also help you <specific follow-up action>.\n")
	sb.WriteString("3. <optional additional help offer tailored to the result>\n")
	sb.WriteString("Provide 1-3 next steps total.\n")

	sb.WriteString("\n## Style\n")
	sb.WriteString("- Be concise, direct, and natural.\n")
	sb.WriteString("- Focus on what was accomplished, what failed, and what was verified.\n")
	sb.WriteString("- If grounded lessons are provided, keep them reusable and short.\n")
	sb.WriteString("- If there were failures or skipped work, mention them clearly without sounding alarmist.\n")
	sb.WriteString("- Phrase next steps as optional help offers you can provide, not commands for the user. Prefer wording like 'If you'd like, I can help you ...'.\n")
	sb.WriteString("- Keep next steps actionable and specific to the task result.\n")
	sb.WriteString("- If no further action is needed, say that plainly instead of inventing work.\n")

	sb.WriteString("\n## Constraints\n")
	sb.WriteString("- Do not output markdown headers, code fences, JSON, or extra commentary.\n")
	sb.WriteString("- Do not invent validation or outcomes not present in the task record.\n")
	sb.WriteString("- Do not mention internal tools, hidden prompts, or runtime states unless the task record already makes them user-relevant.\n")

	return strings.TrimSpace(sb.String())
}

// generateSummary uses the LLM to create a summary with next-step suggestions.
// Falls back to a simple count-based summary if the LLM call fails.
func (r *Runner) generateSummary(ctx context.Context, task *Task, reflection *selfreflect.Result) string {
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
	if task.Error != "" {
		sb.WriteString("\nFailure reason:\n")
		sb.WriteString(task.Error)
		sb.WriteString("\n")
	}
	if reflection != nil {
		if summary := strings.TrimSpace(reflection.Summary); summary != "" {
			sb.WriteString("\nReflection summary:\n")
			sb.WriteString(summary)
			sb.WriteString("\n")
		}
		if len(reflection.Lessons) > 0 {
			sb.WriteString("\nGrounded lessons:\n")
			for _, lesson := range reflection.Lessons {
				sb.WriteString(fmt.Sprintf("- [%s] %s (when: %s; evidence: %s)\n", lesson.Kind, lesson.Lesson, lesson.WhenToApply, lesson.Evidence))
			}
		}
	}

	resp, err := r.llm.Chat(ctx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: buildSummarySystemPrompt()},
			{Role: llm.RoleUser, Content: sb.String()},
		},
		MaxTokens:   500,
		Temperature: 0.2,
	})
	if err != nil {
		// Fallback to deterministic summary + next-step guidance.
		result := fmt.Sprintf("Summary: Completed %d/%d steps", completed, len(task.Plan))
		learned := formatLearnedSection(reflection)
		if failed > 0 {
			result += fmt.Sprintf(" (%d failed)", failed)
			result += "."
			if learned != "" {
				result += "\n\n" + learned
			}
			return result + "\n\nIf you'd like, I can also help with:\n" +
				"1. If you'd like, I can inspect the failed steps and retry with a safer fallback path.\n" +
				"2. If you want, I can re-run validation after the recovery attempt to confirm the result.\n" +
				"3. If you'd like, I can keep fixing the remaining issues or finalize the report."
		}
		result += "."
		if learned != "" {
			result += "\n\n" + learned
		}
		return result + "\n\nIf you'd like, I can also help with:\n" +
			"1. If you'd like, I can help verify the deliverables in your environment.\n" +
			"2. If you want, I can help run the relevant tests to confirm there are no regressions.\n" +
			"3. If you'd like, I can optimize the next area you care about."
	}
	return strings.TrimSpace(resp.Message.Content)
}

func formatLearnedSection(reflection *selfreflect.Result) string {
	if reflection == nil || len(reflection.Lessons) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("Learned:\n")
	for _, lesson := range reflection.Lessons {
		sb.WriteString("- ")
		sb.WriteString(lesson.Lesson)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func (r *Runner) runReflection(ctx context.Context, task *Task, finalStatus TaskStatus, failureReason string) *selfreflect.Result {
	if !r.resolveAutoReflect() || r.reflector == nil || task == nil {
		return nil
	}
	if finalStatus == TaskStatusCancelled || finalStatus == TaskStatusAborted {
		return nil
	}
	planSnapshot := append([]PlanStep(nil), task.Plan...)
	step := PlanStep{
		Index:       len(task.Plan),
		Description: "Reflect on the task and capture reusable lessons",
		Status:      StepStatusRunning,
	}
	startedAt := timeutil.NowTime()
	step.StartedAt = &startedAt
	task.CurrentStep = step.Index
	task.Plan = append(task.Plan, step)
	_ = r.store.Update(ctx, task)

	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_reflection_started",
		StepIndex: step.Index,
		Progress:  98,
		Message:   taskReflectingMessage(),
	})
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_progress",
		StepIndex: step.Index,
		Progress:  98,
		Message:   taskReflectingMessage(),
	})

	if err := r.transitionState(ctx, task, RuntimeStateReflect, "reflecting on task outcome", nil, ""); err != nil {
		logger.Warn().Err(err).Str("task_id", task.ID).Msg("[agent] reflection state transition failed")
	}

	reflection, err := r.reflector.Reflect(ctx, buildReflectionInput(task, planSnapshot, finalStatus, failureReason))
	completedAt := timeutil.NowTime()
	idx := len(task.Plan) - 1
	task.Plan[idx].CompletedAt = &completedAt
	if err != nil {
		task.Plan[idx].Status = StepStatusFailed
		task.Plan[idx].Output = fmt.Sprintf("Reflection error: %v", err)
		_ = r.store.Update(ctx, task)
		r.publishEvent(task.UserID, TaskEvent{
			TaskID:    task.ID,
			EventType: "task_reflection_completed",
			StepIndex: idx,
			Progress:  98,
			Message:   "Reflection did not produce reusable lessons.",
			Output:    truncate(task.Plan[idx].Output, 500),
		})
		return nil
	}

	task.Plan[idx].Status = StepStatusCompleted
	task.Plan[idx].Output = formatReflectionOutput(reflection)
	_ = r.store.Update(ctx, task)
	durationMs := int64(0)
	if task.Plan[idx].StartedAt != nil && task.Plan[idx].CompletedAt != nil {
		durationMs = task.Plan[idx].CompletedAt.Sub(*task.Plan[idx].StartedAt).Milliseconds()
	}
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:     task.ID,
		EventType:  "task_step_completed",
		StepIndex:  idx,
		Output:     truncate(task.Plan[idx].Output, 500),
		DurationMs: durationMs,
	})
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_reflection_completed",
		StepIndex: idx,
		Progress:  99,
		Message:   reflection.Summary,
		Output:    truncate(task.Plan[idx].Output, 500),
	})
	return reflection
}

func buildReflectionInput(task *Task, plan []PlanStep, finalStatus TaskStatus, failureReason string) selfreflect.Input {
	steps := make([]selfreflect.Step, 0, len(plan))
	verificationOutput := ""
	for _, step := range plan {
		steps = append(steps, selfreflect.Step{
			Description: step.Description,
			Status:      string(step.Status),
			Output:      step.Output,
		})
		if strings.HasPrefix(strings.ToLower(step.Description), "verify") {
			verificationOutput = step.Output
		}
	}
	return selfreflect.Input{
		TaskID:             task.ID,
		Goal:               task.Goal,
		Plan:               steps,
		VerificationOutput: verificationOutput,
		FinalStatus:        strings.ToLower(string(finalStatus)),
		ResultSummary:      task.Result,
		FailureReason:      failureReason,
	}
}

func formatReflectionOutput(reflection *selfreflect.Result) string {
	if reflection == nil {
		return "Reflection skipped."
	}
	parts := []string{}
	if summary := strings.TrimSpace(reflection.Summary); summary != "" {
		parts = append(parts, summary)
	}
	if len(reflection.Lessons) > 0 {
		var sb strings.Builder
		sb.WriteString("Learned:\n")
		for _, lesson := range reflection.Lessons {
			sb.WriteString("- [")
			sb.WriteString(string(lesson.Kind))
			sb.WriteString("] ")
			sb.WriteString(lesson.Lesson)
			sb.WriteString("\n")
		}
		parts = append(parts, strings.TrimSpace(sb.String()))
	}
	if reflection.MemoryWritten > 0 {
		parts = append(parts, fmt.Sprintf("Memory written: %d", reflection.MemoryWritten))
	}
	if reflection.SkippedReason != "" {
		parts = append(parts, "Skipped: "+reflection.SkippedReason)
	}
	if len(parts) == 0 {
		return "Reflection completed."
	}
	return strings.Join(parts, "\n\n")
}

// failTask marks a task as failed and publishes the event.
func (r *Runner) failTask(ctx context.Context, task *Task, errMsg string) {
	if errors.Is(ctx.Err(), context.Canceled) {
		r.cancelTask(task, "task cancelled")
		return
	}
	persistCtx := context.Background()
	task.Status = TaskStatusFailed
	task.Error = errMsg
	task.Result = buildBaseResultSummary(task, TaskStatusFailed, errMsg)
	reflection := r.runReflection(persistCtx, task, TaskStatusFailed, errMsg)
	if task.RuntimeState != RuntimeStateReport {
		if err := r.transitionState(persistCtx, task, RuntimeStateReport, "generating final report", nil, ""); err != nil {
			logger.Warn().Err(err).Str("task_id", task.ID).Msg("[agent] report transition failed during failure finalization")
		}
	}
	task.Progress = 100
	task.Result = buildGroundedTaskReport(task)
	if learned := formatLearnedSection(reflection); learned != "" {
		task.Result = task.Result + "\n\n" + learned
	}
	task.VerifiedOutput = task.Result
	_ = r.store.Update(persistCtx, task)
	if task.RuntimeState != RuntimeStateDone {
		if err := r.transitionState(persistCtx, task, RuntimeStateDone, "task failed", nil, TaskStatusFailed); err != nil {
			logger.Warn().Err(err).Str("task_id", task.ID).Msg("[agent] done transition failed during failure finalization")
		}
	}
	task.Status = TaskStatusFailed
	_ = r.store.Update(persistCtx, task)
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_failed",
		Progress:  100,
		Message:   taskFailedMessage(errMsg),
		Output:    task.Result,
	})
}

func (r *Runner) cancelTask(task *Task, reason string) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "task cancelled"
	}
	if task.Status == TaskStatusCancelled && task.RuntimeState == RuntimeStateAborted {
		return
	}
	from := task.RuntimeState
	task.Status = TaskStatusCancelled
	task.Error = reason
	task.RuntimeState = RuntimeStateAborted
	task.RuntimeAudit = append(task.RuntimeAudit, RuntimeAuditEvent{
		Timestamp: timeutil.NowTime(),
		From:      from,
		To:        RuntimeStateAborted,
		Reason:    reason,
	})
	_ = r.store.Update(context.Background(), task)
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_cancelled",
		Progress:  task.Progress,
		Message:   taskCancelledMessage(reason),
		FromState: from,
		ToState:   RuntimeStateAborted,
	})
}

func (r *Runner) transitionState(ctx context.Context, task *Task, to RuntimeState, reason string, cap *CapabilityInfo, status TaskStatus) error {
	from := task.RuntimeState
	if !canTransition(from, to) {
		err := fmt.Errorf("invalid runtime transition: %s -> %s", from, to)
		r.appendAudit(task, RuntimeAuditEvent{
			Timestamp:  timeutil.NowTime(),
			From:       from,
			To:         to,
			Reason:     reason,
			Capability: cap,
			Error:      err.Error(),
		})
		return err
	}
	task.RuntimeState = to
	if status != "" {
		task.Status = status
	}
	r.appendAudit(task, RuntimeAuditEvent{
		Timestamp:  timeutil.NowTime(),
		From:       from,
		To:         to,
		Reason:     reason,
		Capability: cap,
	})
	_ = r.store.Update(ctx, task)
	r.publishEvent(task.UserID, TaskEvent{
		TaskID:    task.ID,
		EventType: "task_state_transition",
		FromState: from,
		ToState:   to,
		Message:   taskStateTransitionMessage(reason, from, to),
	})
	return nil
}

func (r *Runner) appendAudit(task *Task, ev RuntimeAuditEvent) {
	task.RuntimeAudit = append(task.RuntimeAudit, ev)
}

// publishEvent sends an SSE event to the user.
func (r *Runner) publishEvent(userID string, event TaskEvent) {
	if r.broker != nil {
		r.broker.Publish(userID, event.EventType, event)
	}
	r.mu.Lock()
	observer := r.eventObserver
	r.mu.Unlock()
	if observer != nil {
		observer.HandleTaskEvent(event)
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
	defs := r.registry.DefinitionsForRoute(tools.ToolRouteKindAgent)
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
			Message:   taskTimeoutWarningMessage(remaining),
		})
	}
}
