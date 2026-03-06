package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const defaultQuestionTimeout = 2 * time.Minute

// QuestionItem describes a single question (one tab in the UI).
type QuestionItem struct {
	ID          string           `json:"id"`
	Question    string           `json:"question"`
	Detail      string           `json:"detail,omitempty"` // optional extra detail shown with ❕ marker
	Header      string           `json:"header"`           // short tab label (max 12 chars)
	Options     []QuestionOption `json:"options,omitempty"`
	MultiSelect bool             `json:"multi_select,omitempty"`
}

// QuestionOption is a selectable option.
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"` // defaults to label if empty
}

// QuestionAnswerResult is the user's answer to one question.
type QuestionAnswerResult struct {
	QuestionID string   `json:"question_id"`
	Selected   []string `json:"selected"`             // selected option values
	OtherText  string   `json:"other_text,omitempty"` // free-text "Other" input
}

// QuestionRequest is the SSE payload sent to the frontend.
type QuestionRequest struct {
	ID        string                 `json:"id"`
	Questions []QuestionItem         `json:"questions"`
	UserID    string                 `json:"user_id"`
	SessionID string                 `json:"session_id,omitempty"`
	ExpiresAt int64                  `json:"expires_at"` // Unix ms
	Context   map[string]interface{} `json:"context,omitempty"`
}

type pendingQuestion struct {
	ch      chan []QuestionAnswerResult
	request QuestionRequest
	created time.Time
}

// QuestionManager handles the ask-user-question flow via SSE.
type QuestionManager struct {
	broker  *sse.Broker
	mu      sync.Mutex
	pending map[string]*pendingQuestion
	timeout time.Duration
	silent  func() bool // returns true when unattended mode is active
	// Optional dynamic overrides (wired from settings at runtime).
	timeoutFunc       func() time.Duration
	timeoutActionFunc func() string // "default" | "error"
}

// NewQuestionManager creates a new question manager.
func NewQuestionManager(broker *sse.Broker, silentFunc func() bool, timeout time.Duration) *QuestionManager {
	if timeout <= 0 {
		timeout = defaultQuestionTimeout
	}
	if silentFunc == nil {
		silentFunc = func() bool { return false }
	}
	return &QuestionManager{
		broker:  broker,
		pending: make(map[string]*pendingQuestion),
		timeout: timeout,
		silent:  silentFunc,
	}
}

// IsSilent returns true when unattended/silent mode is active.
func (m *QuestionManager) IsSilent() bool {
	return m.silent()
}

// SetSilentFunc updates the silent mode function (for deferred wiring).
func (m *QuestionManager) SetSilentFunc(fn func() bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if fn != nil {
		m.silent = fn
	}
}

// SetTimeoutFunc sets a dynamic timeout getter (takes precedence over static timeout when >0).
func (m *QuestionManager) SetTimeoutFunc(fn func() time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeoutFunc = fn
}

// SetTimeoutActionFunc sets the timeout action getter.
// Supported values:
// - "default": return default answers on timeout (existing behavior)
// - "error": return timeout error
func (m *QuestionManager) SetTimeoutActionFunc(fn func() string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeoutActionFunc = fn
}

func (m *QuestionManager) resolveTimeout() time.Duration {
	m.mu.Lock()
	fn := m.timeoutFunc
	base := m.timeout
	m.mu.Unlock()
	if fn != nil {
		if d := fn(); d > 0 {
			return d
		}
	}
	if base <= 0 {
		return defaultQuestionTimeout
	}
	return base
}

func (m *QuestionManager) resolveTimeoutAction() string {
	m.mu.Lock()
	fn := m.timeoutActionFunc
	m.mu.Unlock()
	if fn != nil {
		switch fn() {
		case "error":
			return "error"
		default:
			return "default"
		}
	}
	return "default"
}

// AskQuestions sends questions to the user via SSE and blocks until answers arrive.
// In silent mode, returns default answers immediately (first option per question).
func (m *QuestionManager) AskQuestions(ctx context.Context, userID, sessionID string, questions []QuestionItem) ([]QuestionAnswerResult, bool, error) {
	return m.AskQuestionsWithContext(ctx, userID, sessionID, questions, nil)
}

// AskQuestionsWithContext sends questions to the user via SSE with optional UI context payload.
// In silent mode, returns default answers immediately (first option per question).
func (m *QuestionManager) AskQuestionsWithContext(ctx context.Context, userID, sessionID string, questions []QuestionItem, questionContext map[string]interface{}) ([]QuestionAnswerResult, bool, error) {
	// Assign IDs if missing, normalize values
	for i := range questions {
		if questions[i].ID == "" {
			questions[i].ID = fmt.Sprintf("q%d", i)
		}
		for j := range questions[i].Options {
			if questions[i].Options[j].Value == "" {
				questions[i].Options[j].Value = questions[i].Options[j].Label
			}
		}
	}

	// Silent mode: auto-answer with first option
	if m.IsSilent() {
		return m.defaultAnswers(questions), true, nil
	}

	// Non-web channels without UI: also auto-answer
	ch := GetChannel(ctx)
	if ch != "" && ch != "web" {
		return m.defaultAnswers(questions), true, nil
	}

	// Normalize userID early so the stored request and SSE publish target match.
	if userID == "" {
		userID = "default"
	}
	// If no active SSE consumer exists for this user, do not block on timeout.
	// Treat it as unattended mode and return deterministic defaults immediately.
	if m.broker == nil || m.broker.ClientCount(userID) == 0 {
		if m.resolveTimeoutAction() == "error" {
			return nil, false, fmt.Errorf("question cannot be delivered: no active SSE client for user %q", userID)
		}
		return m.defaultAnswers(questions), true, nil
	}

	reqID := uuid.New().String()
	answerCh := make(chan []QuestionAnswerResult, 1)

	timeout := m.resolveTimeout()
	req := QuestionRequest{
		ID:        reqID,
		Questions: questions,
		UserID:    userID,
		SessionID: sessionID,
		ExpiresAt: timeutil.NowMilli() + timeout.Milliseconds(),
		Context:   questionContext,
	}

	m.mu.Lock()
	m.pending[reqID] = &pendingQuestion{ch: answerCh, request: req, created: timeutil.NowTime()}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.pending, reqID)
		m.mu.Unlock()
	}()

	// Publish SSE event
	m.broker.Publish(userID, "ask", req)

	// Wait for response or timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case answers := <-answerCh:
		return answers, false, nil
	case <-timer.C:
		if m.resolveTimeoutAction() == "error" {
			return nil, false, fmt.Errorf("question timed out after %s", timeout)
		}
		// Timeout: return defaults
		return m.defaultAnswers(questions), true, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

// ResolveAnswer is called by the REST endpoint when the user responds.
func (m *QuestionManager) ResolveAnswer(id string, answers []QuestionAnswerResult) bool {
	// Deduplicate answers: keep only unique selected values, preserve OtherText
	for i := range answers {
		if len(answers[i].Selected) <= 1 {
			continue
		}
		otherText := answers[i].OtherText
		hasOther := false
		for _, v := range answers[i].Selected {
			if v == "__other__" {
				hasOther = true
				break
			}
		}
		// Deduplicate selected values
		seen := make(map[string]bool)
		uniqueSelected := make([]string, 0, len(answers[i].Selected))
		for _, v := range answers[i].Selected {
			if v == "__other__" {
				// Only keep __other__ once
				if !seen["__other__"] {
					seen["__other__"] = true
					uniqueSelected = append(uniqueSelected, v)
				}
			} else {
				if !seen[v] {
					seen[v] = true
					uniqueSelected = append(uniqueSelected, v)
				}
			}
		}
		answers[i].Selected = uniqueSelected
		// Restore OtherText if __other__ was present
		if hasOther && otherText != "" {
			answers[i].OtherText = otherText
		}
	}

	m.mu.Lock()
	p, ok := m.pending[id]
	if ok {
		delete(m.pending, id)
	}
	m.mu.Unlock()

	if !ok {
		return false
	}

	select {
	case p.ch <- answers:
	default:
	}
	return true
}

// DismissQuestion is called when the user skips/dismisses a question.
// It sends default answers so the blocked AskQuestions call unblocks immediately.
func (m *QuestionManager) DismissQuestion(id string) bool {
	m.mu.Lock()
	p, ok := m.pending[id]
	if ok {
		delete(m.pending, id)
	}
	m.mu.Unlock()

	if !ok {
		return false
	}

	// Send default answers (first option per question) so the tool call completes.
	defaults := m.defaultAnswers(p.request.Questions)
	select {
	case p.ch <- defaults:
	default:
	}
	return true
}

// PendingCount returns the number of pending questions.
func (m *QuestionManager) PendingCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pending)
}

// CleanupExpired removes expired pending questions and returns removed request IDs.
func (m *QuestionManager) CleanupExpired() []string {
	nowMs := timeutil.NowMilli()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pending) == 0 {
		return nil
	}
	removed := make([]string, 0)
	for id, p := range m.pending {
		if p == nil || p.request.ExpiresAt <= nowMs {
			removed = append(removed, id)
			delete(m.pending, id)
		}
	}
	return removed
}

// GetPending returns the first pending QuestionRequest (if any).
// Used by the REST endpoint so the frontend can restore the dialog on page switch.
func (m *QuestionManager) GetPending(userID string) *QuestionRequest {
	if userID == "" {
		userID = "default"
	}
	nowMs := timeutil.NowMilli()
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest *pendingQuestion
	for id, p := range m.pending {
		if p == nil || p.request.ExpiresAt <= nowMs {
			delete(m.pending, id)
			continue
		}
		if p.request.UserID != userID {
			continue
		}
		if latest == nil || p.created.After(latest.created) {
			latest = p
		}
	}
	if latest != nil {
		req := latest.request
		return &req
	}
	return nil
}

// GetPendingBySession returns the most recent pending QuestionRequest for sessionID.
// This is a fallback path when user-scoped matching is unavailable.
func (m *QuestionManager) GetPendingBySession(sessionID string) *QuestionRequest {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	nowMs := timeutil.NowMilli()
	m.mu.Lock()
	defer m.mu.Unlock()
	var latest *pendingQuestion
	for id, p := range m.pending {
		if p == nil || p.request.ExpiresAt <= nowMs {
			delete(m.pending, id)
			continue
		}
		if strings.TrimSpace(p.request.SessionID) != sessionID {
			continue
		}
		if latest == nil || p.created.After(latest.created) {
			latest = p
		}
	}
	if latest != nil {
		req := latest.request
		return &req
	}
	return nil
}

// defaultAnswers returns auto-generated answers selecting the first option per question.
func (m *QuestionManager) defaultAnswers(questions []QuestionItem) []QuestionAnswerResult {
	results := make([]QuestionAnswerResult, len(questions))
	for i, q := range questions {
		results[i] = QuestionAnswerResult{QuestionID: q.ID}
		if len(q.Options) > 0 {
			val := q.Options[0].Value
			if val == "" {
				val = q.Options[0].Label
			}
			results[i].Selected = []string{val}
		}
	}
	return results
}
