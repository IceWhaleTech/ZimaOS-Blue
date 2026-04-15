package harness

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const (
	runtimeReflectionArtifactKind    = "runtime_reflection"
	runtimeReflectionRecentEventScan = 4096
	runtimeReflectionRecentEventTail = 25
	runtimeReflectionInteractionGate = 5
	runtimeReflectionTerminalFlush   = 3
	runtimeReflectionRepeatThreshold = 3
)

type RuntimeReflectionCoordinator struct {
	controller *Controller
	config     config.RuntimeReflectionConfig

	mu     sync.Mutex
	states map[string]*runtimeReflectionState

	now    func() time.Time
	launch func(func())
}

type runtimeReflectionState struct {
	toolFinishesSinceReview int
	consecutiveFailures     int
	lastFailureFamily       string
	recentFingerprints      []string
	lastReviewAt            time.Time
	reviewInFlight          bool
	reviewSeq               int
	lastStepIndex           int
}

type runtimeReflectionRequest struct {
	RunID              string
	StepIndex          int
	TriggerKind        string
	ReviewSeq          int
	ToolCountWindow    int
	DisableMemoryWrite bool
	Terminal           bool
}

type runtimeObserverMux struct {
	observers []tools.RuntimeEventObserver
}

func NewRuntimeEventObserverMux(observers ...tools.RuntimeEventObserver) tools.RuntimeEventObserver {
	filtered := make([]tools.RuntimeEventObserver, 0, len(observers))
	for _, observer := range observers {
		if observer != nil {
			filtered = append(filtered, observer)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return &runtimeObserverMux{observers: filtered}
}

func (m *runtimeObserverMux) OnToolRequested(event tools.ToolRuntimeEvent) {
	for _, observer := range m.observers {
		observer.OnToolRequested(event)
	}
}

func (m *runtimeObserverMux) OnToolFinished(event tools.ToolRuntimeEvent) {
	for _, observer := range m.observers {
		observer.OnToolFinished(event)
	}
}

func (m *runtimeObserverMux) OnApprovalRequested(event tools.ApprovalRuntimeEvent) {
	for _, observer := range m.observers {
		observer.OnApprovalRequested(event)
	}
}

func (m *runtimeObserverMux) OnApprovalResolved(event tools.ApprovalRuntimeEvent) {
	for _, observer := range m.observers {
		observer.OnApprovalResolved(event)
	}
}

func (m *runtimeObserverMux) OnQuestionRequested(event tools.QuestionRuntimeEvent) {
	for _, observer := range m.observers {
		observer.OnQuestionRequested(event)
	}
}

func (m *runtimeObserverMux) OnQuestionResolved(event tools.QuestionRuntimeEvent) {
	for _, observer := range m.observers {
		observer.OnQuestionResolved(event)
	}
}

func NewRuntimeReflectionCoordinator(controller *Controller, cfg config.RuntimeReflectionConfig) *RuntimeReflectionCoordinator {
	defaults := config.DefaultHarnessConfig().RuntimeReflection
	if cfg.IntervalToolFinishes <= 0 {
		cfg.IntervalToolFinishes = defaults.IntervalToolFinishes
	}
	if cfg.MinReviewGap < 0 {
		cfg.MinReviewGap = 0
	}
	if cfg.RepeatWindow <= 0 {
		cfg.RepeatWindow = defaults.RepeatWindow
	}
	if cfg.MaxEvidenceEvents <= 0 {
		cfg.MaxEvidenceEvents = defaults.MaxEvidenceEvents
	}
	return &RuntimeReflectionCoordinator{
		controller: controller,
		config:     cfg,
		states:     make(map[string]*runtimeReflectionState),
		now:        timeutil.NowTime,
		launch: func(fn func()) {
			go fn()
		},
	}
}

func (c *RuntimeReflectionCoordinator) OnToolRequested(event tools.ToolRuntimeEvent) {}

func (c *RuntimeReflectionCoordinator) OnToolFinished(event tools.ToolRuntimeEvent) {
	if c == nil || !c.config.Enabled || strings.TrimSpace(event.RunID) == "" {
		return
	}
	request := c.noteToolFinished(event)
	if request != nil {
		c.launch(func() {
			c.performReview(context.Background(), *request)
		})
	}
}

func (c *RuntimeReflectionCoordinator) OnApprovalRequested(event tools.ApprovalRuntimeEvent) {
	c.maybeReviewAfterInteraction(strings.TrimSpace(event.RunID), event.StepIndex, "approval_requested")
}

func (c *RuntimeReflectionCoordinator) OnApprovalResolved(event tools.ApprovalRuntimeEvent) {
	c.maybeReviewAfterInteraction(strings.TrimSpace(event.RunID), event.StepIndex, "approval_resolved")
}

func (c *RuntimeReflectionCoordinator) OnQuestionRequested(event tools.QuestionRuntimeEvent) {
	c.maybeReviewAfterInteraction(strings.TrimSpace(event.RunID), event.StepIndex, "question_requested")
}

func (c *RuntimeReflectionCoordinator) OnQuestionResolved(event tools.QuestionRuntimeEvent) {
	c.maybeReviewAfterInteraction(strings.TrimSpace(event.RunID), event.StepIndex, "question_resolved")
}

func (c *RuntimeReflectionCoordinator) OnRunTerminal(ctx context.Context, run *Run) {
	if c == nil || !c.config.Enabled || run == nil || strings.TrimSpace(run.ID) == "" {
		return
	}
	request := c.prepareTerminalFlush(run.ID, run.CurrentStep)
	if request == nil {
		return
	}
	c.performReview(ctx, *request)
}

func (c *RuntimeReflectionCoordinator) noteToolFinished(event tools.ToolRuntimeEvent) *runtimeReflectionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := c.stateForRun(strings.TrimSpace(event.RunID))
	state.lastStepIndex = event.StepIndex
	state.toolFinishesSinceReview++

	fingerprint := runtimeReflectionFingerprint(event)
	if fingerprint != "" {
		state.recentFingerprints = append(state.recentFingerprints, fingerprint)
		if len(state.recentFingerprints) > c.config.RepeatWindow {
			state.recentFingerprints = state.recentFingerprints[len(state.recentFingerprints)-c.config.RepeatWindow:]
		}
	}

	if errText := strings.TrimSpace(event.Error); errText != "" {
		family := runtimeReflectionFailureFamily(event)
		if family != "" && family == state.lastFailureFamily {
			state.consecutiveFailures++
		} else {
			state.consecutiveFailures = 1
			state.lastFailureFamily = family
		}
	} else {
		state.consecutiveFailures = 0
		state.lastFailureFamily = ""
	}

	triggerKind := c.nextToolTriggerKind(state)
	return c.scheduleLocked(state, runtimeReflectionRequest{
		RunID:              strings.TrimSpace(event.RunID),
		StepIndex:          event.StepIndex,
		TriggerKind:        triggerKind,
		ToolCountWindow:    state.toolFinishesSinceReview,
		DisableMemoryWrite: true,
	})
}

func (c *RuntimeReflectionCoordinator) maybeReviewAfterInteraction(runID string, stepIndex int, triggerKind string) {
	if c == nil || !c.config.Enabled || runID == "" {
		return
	}
	c.mu.Lock()
	state := c.stateForRun(runID)
	if stepIndex > 0 {
		state.lastStepIndex = stepIndex
	}
	request := &runtimeReflectionRequest{
		RunID:              runID,
		StepIndex:          state.lastStepIndex,
		TriggerKind:        triggerKind,
		ToolCountWindow:    state.toolFinishesSinceReview,
		DisableMemoryWrite: true,
	}
	if state.toolFinishesSinceReview < runtimeReflectionInteractionGate {
		request = nil
	} else {
		request = c.scheduleLocked(state, *request)
	}
	c.mu.Unlock()

	if request != nil {
		c.launch(func() {
			c.performReview(context.Background(), *request)
		})
	}
}

func (c *RuntimeReflectionCoordinator) prepareTerminalFlush(runID string, stepIndex int) *runtimeReflectionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := c.stateForRun(strings.TrimSpace(runID))
	if stepIndex > 0 {
		state.lastStepIndex = stepIndex
	}
	if state.reviewInFlight || state.toolFinishesSinceReview < runtimeReflectionTerminalFlush {
		return nil
	}
	return c.scheduleLocked(state, runtimeReflectionRequest{
		RunID:              strings.TrimSpace(runID),
		StepIndex:          state.lastStepIndex,
		TriggerKind:        "terminal_flush",
		ToolCountWindow:    state.toolFinishesSinceReview,
		DisableMemoryWrite: false,
		Terminal:           true,
	})
}

func (c *RuntimeReflectionCoordinator) scheduleLocked(state *runtimeReflectionState, request runtimeReflectionRequest) *runtimeReflectionRequest {
	if c == nil || state == nil || request.TriggerKind == "" || request.RunID == "" {
		return nil
	}
	if state.reviewInFlight {
		return nil
	}
	if !request.Terminal && c.config.MinReviewGap > 0 && !state.lastReviewAt.IsZero() && c.now().Sub(state.lastReviewAt) < c.config.MinReviewGap {
		return nil
	}
	state.reviewInFlight = true
	state.reviewSeq++
	state.lastReviewAt = c.now()
	request.ReviewSeq = state.reviewSeq
	if request.StepIndex <= 0 {
		request.StepIndex = state.lastStepIndex
	}
	if request.ToolCountWindow <= 0 {
		request.ToolCountWindow = state.toolFinishesSinceReview
	}
	state.toolFinishesSinceReview = 0
	state.consecutiveFailures = 0
	state.lastFailureFamily = ""
	state.recentFingerprints = nil
	return &request
}

func (c *RuntimeReflectionCoordinator) finishReview(runID string) {
	if c == nil || runID == "" {
		return
	}
	var next *runtimeReflectionRequest

	c.mu.Lock()
	state := c.stateForRun(runID)
	state.reviewInFlight = false
	triggerKind := c.nextToolTriggerKind(state)
	if triggerKind != "" {
		next = c.scheduleLocked(state, runtimeReflectionRequest{
			RunID:              runID,
			StepIndex:          state.lastStepIndex,
			TriggerKind:        triggerKind,
			ToolCountWindow:    state.toolFinishesSinceReview,
			DisableMemoryWrite: true,
		})
	}
	c.mu.Unlock()

	if next != nil {
		c.launch(func() {
			c.performReview(context.Background(), *next)
		})
	}
}

func (c *RuntimeReflectionCoordinator) nextToolTriggerKind(state *runtimeReflectionState) string {
	if state == nil {
		return ""
	}
	switch {
	case state.consecutiveFailures >= 2:
		return "consecutive_failures"
	case repeatedRuntimeFingerprint(state.recentFingerprints):
		return "repeated_fingerprint"
	case state.toolFinishesSinceReview >= c.config.IntervalToolFinishes:
		return "interval_tool_finishes"
	default:
		return ""
	}
}

func (c *RuntimeReflectionCoordinator) performReview(ctx context.Context, request runtimeReflectionRequest) {
	defer c.finishReview(request.RunID)
	record := c.buildRecord(ctx, request)
	record.RecordID = strings.TrimSpace(record.RecordID)
	if record.RecordID == "" {
		record.RecordID = uuid.NewString()
	}
	payload := map[string]interface{}{
		"record_id":            record.RecordID,
		"run_id":               request.RunID,
		"trigger_kind":         record.TriggerKind,
		"review_seq":           record.ReviewSeq,
		"step_index":           record.StepIndex,
		"tool_count_window":    record.ToolCountWindow,
		"evidence_event_ids":   append([]string(nil), record.EvidenceEventIDs...),
		"signals":              cloneMetadataMap(record.Signals),
		"summary":              strings.TrimSpace(record.Summary),
		"lessons":              record.Lessons,
		"mutation_suggestions": record.MutationSuggestions,
		"reflection_signature": strings.TrimSpace(record.ReflectionSignature),
		"status":               strings.TrimSpace(record.Status),
	}
	payloadJSON := marshalMetadata(payload)
	eventType := "runtime_reflection_skipped"
	message := strings.TrimSpace(record.Summary)
	if record.Status == "recorded" {
		eventType = "runtime_reflection_recorded"
	}
	if message == "" {
		message = strings.TrimSpace(record.Status)
	}
	createdAt := timeutil.NowTime()
	if c.controller != nil {
		_ = c.controller.AppendEvent(ctx, RunEvent{
			RunID:       request.RunID,
			Type:        eventType,
			StepIndex:   request.StepIndex,
			Message:     message,
			PayloadJSON: payloadJSON,
			CreatedAt:   createdAt,
		})
		_ = c.controller.AttachArtifact(ctx, ArtifactRef{
			ID:           uuid.NewString(),
			RunID:        request.RunID,
			Kind:         runtimeReflectionArtifactKind,
			Label:        request.TriggerKind,
			MetadataJSON: payloadJSON,
		})
	}
}

func (c *RuntimeReflectionCoordinator) buildRecord(ctx context.Context, request runtimeReflectionRequest) runtimeReflectionRecord {
	record := runtimeReflectionRecord{
		RecordID:        uuid.NewString(),
		RunID:           request.RunID,
		TriggerKind:     request.TriggerKind,
		ReviewSeq:       request.ReviewSeq,
		StepIndex:       request.StepIndex,
		ToolCountWindow: request.ToolCountWindow,
		Status:          "skipped",
	}
	if c == nil || c.controller == nil {
		record.Summary = "runtime reflection coordinator is not configured"
		return record
	}
	run, err := c.controller.GetStored(ctx, request.RunID)
	if err != nil || run == nil {
		record.Summary = "runtime reflection skipped: run not found"
		return record
	}
	_, reflector := c.controller.integrations()
	if reflector == nil {
		record.Summary = "runtime reflection skipped: reflector not configured"
		return record
	}

	events, err := c.controller.ListEvents(ctx, request.RunID, runtimeReflectionRecentEventScan)
	if err != nil {
		events = nil
	}
	evidenceWindow, evidenceIDs := buildRuntimeReflectionEvidenceWindow(events, c.config.MaxEvidenceEvents)
	record.EvidenceEventIDs = evidenceIDs
	record.Signals = buildRuntimeReflectionSignals(run, events, request, evidenceIDs)

	result, reflectErr := reflector.Reflect(ctx, selfreflect.Input{
		TaskID:             request.RunID,
		Goal:               strings.TrimSpace(run.Goal),
		FinalStatus:        runtimeReflectionFinalStatus(run, request.Terminal),
		ResultSummary:      strings.TrimSpace(run.Result),
		FailureReason:      strings.TrimSpace(run.Error),
		OwnerUserID:        strings.TrimSpace(run.UserID),
		SourceKind:         "runtime_checkpoint",
		SourceID:           request.RunID,
		TriggerKind:        request.TriggerKind,
		EvidenceWindow:     evidenceWindow,
		RuntimeSignals:     cloneMetadataMap(record.Signals),
		DisableMemoryWrite: request.DisableMemoryWrite,
	})
	if reflectErr != nil {
		record.Summary = "runtime reflection skipped: " + strings.TrimSpace(reflectErr.Error())
		return record
	}
	if result == nil {
		record.Summary = "runtime reflection skipped: empty result"
		return record
	}

	record.Summary = strings.TrimSpace(result.Summary)
	record.Lessons = append([]selfreflect.Lesson(nil), result.Lessons...)
	record.MutationSuggestions = append([]selfreflect.MutationSuggestion(nil), result.MutationSuggestions...)
	record.ReflectionSignature = strings.TrimSpace(result.ReflectionSignature)
	if len(record.Lessons) == 0 && len(record.MutationSuggestions) == 0 {
		if skipped := strings.TrimSpace(result.SkippedReason); skipped != "" {
			record.Summary = skipped
		}
		if record.Summary == "" {
			record.Summary = "runtime reflection skipped: no grounded lessons passed quality filters"
		}
		return record
	}
	if record.Summary == "" {
		record.Summary = "runtime reflection recorded"
	}
	record.Status = "recorded"
	return record
}

func (c *RuntimeReflectionCoordinator) stateForRun(runID string) *runtimeReflectionState {
	state := c.states[runID]
	if state != nil {
		return state
	}
	state = &runtimeReflectionState{}
	c.states[runID] = state
	return state
}

func repeatedRuntimeFingerprint(fingerprints []string) bool {
	if len(fingerprints) < runtimeReflectionRepeatThreshold {
		return false
	}
	last := strings.TrimSpace(fingerprints[len(fingerprints)-1])
	if last == "" {
		return false
	}
	count := 0
	for _, fingerprint := range fingerprints {
		if strings.TrimSpace(fingerprint) == last {
			count++
		}
	}
	return count >= runtimeReflectionRepeatThreshold
}

func runtimeReflectionFingerprint(event tools.ToolRuntimeEvent) string {
	base := runtimeReflectionFirstNonEmpty(strings.TrimSpace(event.CapabilityKind), strings.TrimSpace(event.ToolName))
	if base == "" {
		return ""
	}
	if failure := runtimeReflectionFailureFamily(event); failure != "" {
		return base + "|" + failure
	}
	return ""
}

func runtimeReflectionFailureFamily(event tools.ToolRuntimeEvent) string {
	errorText := strings.ToLower(strings.TrimSpace(event.Error))
	if errorText == "" {
		return ""
	}
	switch {
	case strings.Contains(errorText, "missing_title"):
		return "missing_title"
	case strings.Contains(errorText, "timeout"):
		return "timeout"
	default:
		return normalizeRuntimeReflectionKey(errorText)
	}
}

func buildRuntimeReflectionEvidenceWindow(events []RunEvent, limit int) ([]selfreflect.RuntimeEvidenceItem, []string) {
	if limit <= 0 {
		limit = runtimeReflectionRepeatThreshold
	}
	if len(events) == 0 {
		return nil, nil
	}
	start := 0
	if len(events) > runtimeReflectionRecentEventTail {
		start = len(events) - runtimeReflectionRecentEventTail
	}
	window := events[start:]
	filtered := make([]selfreflect.RuntimeEvidenceItem, 0, len(window))
	for _, event := range window {
		if strings.HasPrefix(strings.TrimSpace(event.Type), "runtime_reflection_") || strings.TrimSpace(event.Type) == runtimeSkillEvolutionEventType {
			continue
		}
		filtered = append(filtered, selfreflect.RuntimeEvidenceItem{
			ID:          strings.TrimSpace(event.ID),
			EventType:   strings.TrimSpace(event.Type),
			StepIndex:   event.StepIndex,
			Summary:     strings.TrimSpace(event.Message),
			PayloadJSON: strings.TrimSpace(event.PayloadJSON),
		})
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	ids := make([]string, 0, len(filtered))
	for _, item := range filtered {
		if item.ID != "" {
			ids = append(ids, item.ID)
		}
	}
	return filtered, ids
}

func buildRuntimeReflectionSignals(run *Run, events []RunEvent, request runtimeReflectionRequest, evidenceIDs []string) map[string]interface{} {
	signals := map[string]interface{}{
		"trigger_kind":           request.TriggerKind,
		"review_seq":             request.ReviewSeq,
		"tool_count_window":      request.ToolCountWindow,
		"last_reviewed_event_id": lastString(evidenceIDs),
	}
	if run != nil {
		if skillID := runtimeSkillEvolutionCanonicalSkillID(run); skillID != "" {
			signals["selected_canonical_skill"] = skillID
		}
		if request.Terminal {
			signals["terminal_status"] = strings.TrimSpace(string(run.Status))
		}
	}
	if metrics := runtimeSkillEvolutionMetrics(run, events); len(metrics) > 0 {
		signals["runtime_metrics"] = metrics
	}
	if usage := runtimeSkillEvolutionUsage(run, events); len(usage) > 0 {
		signals["runtime_usage"] = usage
	}
	if quality := runtimeSkillEvolutionQuality(run, events); len(quality) > 0 {
		signals["runtime_quality"] = quality
	}
	return signals
}

func runtimeReflectionFinalStatus(run *Run, terminal bool) string {
	if terminal && run != nil {
		return strings.ToLower(strings.TrimSpace(string(run.Status)))
	}
	return "running"
}

func lastString(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(items[len(items)-1])
}

func runtimeReflectionFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeRuntimeReflectionKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(",", "", ".", "", ":", "", ";", "", "!", "", "?", "", "-", " ", "_", " ")
	value = replacer.Replace(value)
	return strings.Join(strings.Fields(value), " ")
}
