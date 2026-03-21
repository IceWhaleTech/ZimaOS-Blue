package tools

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// BrowserCheckpointDecision indicates how a checkpoint request is resolved.
type BrowserCheckpointDecision string

const (
	BrowserCheckpointPending BrowserCheckpointDecision = "pending"
	BrowserCheckpointApprove BrowserCheckpointDecision = "approve"
	BrowserCheckpointDeny    BrowserCheckpointDecision = "deny"
	BrowserCheckpointTimeout BrowserCheckpointDecision = "timeout"
)

// BrowserCheckpointScreenshot carries optional screenshot context.
type BrowserCheckpointScreenshot struct {
	MimeType string `json:"mime_type,omitempty"`
	Data     string `json:"data,omitempty"` // base64 data without prefix
	URL      string `json:"url,omitempty"`
}

// BrowserCheckpointContext is attached to ask-question payload for UI rendering.
type BrowserCheckpointContext struct {
	Kind         string                       `json:"kind"`
	CheckpointID string                       `json:"checkpoint_id"`
	Required     bool                         `json:"required"`
	RiskLevel    string                       `json:"risk_level"`
	Step         string                       `json:"step,omitempty"`
	Action       string                       `json:"action,omitempty"`
	URL          string                       `json:"url,omitempty"`
	Screenshot   *BrowserCheckpointScreenshot `json:"screenshot,omitempty"`
}

// BrowserCheckpointRequest represents a checkpoint request emitted by browser tool.
type BrowserCheckpointRequest struct {
	ID         string
	SessionID  string
	UserID     string
	Channel    string
	Required   bool
	RiskLevel  string
	Step       string
	Action     string
	URL        string
	Screenshot *BrowserCheckpointScreenshot
	Timeout    time.Duration
}

// BrowserCheckpointRecord represents a pending checkpoint.
type BrowserCheckpointRecord struct {
	ID         string                       `json:"id"`
	SessionID  string                       `json:"session_id,omitempty"`
	UserID     string                       `json:"user_id,omitempty"`
	Channel    string                       `json:"channel,omitempty"`
	Required   bool                         `json:"required"`
	RiskLevel  string                       `json:"risk_level"`
	Step       string                       `json:"step,omitempty"`
	Action     string                       `json:"action,omitempty"`
	URL        string                       `json:"url,omitempty"`
	Screenshot *BrowserCheckpointScreenshot `json:"screenshot,omitempty"`
	CreatedAt  time.Time                    `json:"created_at"`
	ExpiresAt  time.Time                    `json:"expires_at"`
	Decision   BrowserCheckpointDecision    `json:"decision"`
	DecisionAt *time.Time                   `json:"decision_at,omitempty"`
}

type pendingBrowserCheckpoint struct {
	record BrowserCheckpointRecord
	ch     chan BrowserCheckpointDecision
}

// BrowserCheckpointManager stores pending browser confirmations in memory.
// It enforces one pending checkpoint per session to prevent ambiguous replies.
type BrowserCheckpointManager struct {
	mu        sync.Mutex
	pending   map[string]*pendingBrowserCheckpoint
	bySession map[string]string
	timeout   time.Duration
}

// NewBrowserCheckpointManager creates a manager with the given default timeout.
func NewBrowserCheckpointManager(timeout time.Duration) *BrowserCheckpointManager {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &BrowserCheckpointManager{
		pending:   make(map[string]*pendingBrowserCheckpoint),
		bySession: make(map[string]string),
		timeout:   timeout,
	}
}

// DefaultTimeout returns the manager default timeout.
func (m *BrowserCheckpointManager) DefaultTimeout() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.timeout
}

// SetDefaultTimeout updates the manager default timeout.
func (m *BrowserCheckpointManager) SetDefaultTimeout(timeout time.Duration) {
	if timeout <= 0 {
		return
	}
	m.mu.Lock()
	m.timeout = timeout
	m.mu.Unlock()
}

func (m *BrowserCheckpointManager) resolveNoLock(id string, decision BrowserCheckpointDecision) bool {
	p, ok := m.pending[id]
	if !ok {
		return false
	}
	delete(m.pending, id)
	if p.record.SessionID != "" {
		if current, exists := m.bySession[p.record.SessionID]; exists && current == id {
			delete(m.bySession, p.record.SessionID)
		}
	}
	select {
	case p.ch <- decision:
	default:
	}
	close(p.ch)
	return true
}

// Create registers a new pending checkpoint.
func (m *BrowserCheckpointManager) Create(req BrowserCheckpointRequest) BrowserCheckpointRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = m.timeout
	}
	now := timeutil.NowTime()
	expires := now.Add(timeout)

	if req.RiskLevel == "" {
		req.RiskLevel = "high"
	}

	if req.SessionID != "" {
		if oldID, ok := m.bySession[req.SessionID]; ok {
			m.resolveNoLock(oldID, BrowserCheckpointTimeout)
		}
	}

	record := BrowserCheckpointRecord{
		ID:         req.ID,
		SessionID:  req.SessionID,
		UserID:     req.UserID,
		Channel:    req.Channel,
		Required:   req.Required,
		RiskLevel:  req.RiskLevel,
		Step:       req.Step,
		Action:     req.Action,
		URL:        req.URL,
		Screenshot: req.Screenshot,
		CreatedAt:  now,
		ExpiresAt:  expires,
		Decision:   BrowserCheckpointPending,
	}

	m.pending[record.ID] = &pendingBrowserCheckpoint{
		record: record,
		ch:     make(chan BrowserCheckpointDecision, 1),
	}
	if record.SessionID != "" {
		m.bySession[record.SessionID] = record.ID
	}
	return record
}

// Get returns checkpoint record by ID. Expired checkpoints are removed.
func (m *BrowserCheckpointManager) Get(id string) *BrowserCheckpointRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pending[id]
	if !ok {
		return nil
	}
	if timeutil.NowTime().After(p.record.ExpiresAt) {
		m.resolveNoLock(id, BrowserCheckpointTimeout)
		return nil
	}
	cp := p.record
	return &cp
}

// GetPendingBySession returns the pending checkpoint for a session, if any.
func (m *BrowserCheckpointManager) GetPendingBySession(sessionID string) *BrowserCheckpointRecord {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.bySession[sessionID]
	if !ok {
		return nil
	}
	p, exists := m.pending[id]
	if !exists {
		delete(m.bySession, sessionID)
		return nil
	}
	if timeutil.NowTime().After(p.record.ExpiresAt) {
		m.resolveNoLock(id, BrowserCheckpointTimeout)
		return nil
	}
	cp := p.record
	return &cp
}

// Resolve resolves a pending checkpoint by ID.
func (m *BrowserCheckpointManager) Resolve(id string, decision BrowserCheckpointDecision) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resolveNoLock(id, decision)
}

// ResolveBySession resolves pending checkpoint by session ID.
func (m *BrowserCheckpointManager) ResolveBySession(sessionID string, decision BrowserCheckpointDecision) (string, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.bySession[sessionID]
	if !ok {
		return "", false
	}
	return id, m.resolveNoLock(id, decision)
}

// Wait blocks until checkpoint is resolved, timeout expires, or context cancels.
func (m *BrowserCheckpointManager) Wait(ctx context.Context, id string) (BrowserCheckpointDecision, error) {
	m.mu.Lock()
	p, ok := m.pending[id]
	if !ok {
		m.mu.Unlock()
		return BrowserCheckpointDeny, fmt.Errorf("checkpoint %s not found", id)
	}
	waitFor := timeutil.UntilTime(p.record.ExpiresAt)
	if waitFor <= 0 {
		m.resolveNoLock(id, BrowserCheckpointTimeout)
		m.mu.Unlock()
		return BrowserCheckpointTimeout, nil
	}
	ch := p.ch
	m.mu.Unlock()

	timer := time.NewTimer(waitFor)
	defer timer.Stop()

	select {
	case decision, ok := <-ch:
		if !ok {
			return BrowserCheckpointDeny, nil
		}
		return decision, nil
	case <-timer.C:
		_ = m.Resolve(id, BrowserCheckpointTimeout)
		return BrowserCheckpointTimeout, nil
	case <-ctx.Done():
		return BrowserCheckpointDeny, ctx.Err()
	}
}

// CleanupExpired removes expired checkpoints and returns removed IDs.
func (m *BrowserCheckpointManager) CleanupExpired() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := timeutil.NowTime()
	expired := make([]string, 0)
	for id, p := range m.pending {
		if now.After(p.record.ExpiresAt) {
			expired = append(expired, id)
			m.resolveNoLock(id, BrowserCheckpointTimeout)
		}
	}
	return expired
}

// IsBrowserActionHighRisk returns true when browser action should require confirmation.
func IsBrowserActionHighRisk(action, actType, recipe string) bool {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "act":
		switch strings.TrimSpace(strings.ToLower(actType)) {
		case "click", "type", "select", "submit":
			return true
		default:
			return false
		}
	case "recipe":
		switch strings.TrimSpace(strings.ToLower(recipe)) {
		case "login", "fill_form":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

var browserCheckpointApproveTokens = map[string]struct{}{
	"k": {}, "kk": {}, "sure": {}, "go": {}, "go ahead": {}, "lets go": {}, "let's go": {},
	"1": {}, "y": {}, "yes": {}, "ok": {}, "okay": {}, "continue": {}, "proceed": {}, "confirm": {},
	"好": {}, "好的": {}, "行": {}, "可以": {}, "继续": {}, "继续吧": {}, "继续执行": {}, "确认": {}, "繼續": {}, "確認": {}, "嗯": {}, "嗯嗯": {}, "收到": {}, "明白": {},
	"oui": {}, "continuer": {}, "confirmer": {},
	"ja": {}, "weiter": {}, "bestätigen": {}, "bestaetigen": {},
	"sí": {}, "si": {}, "continuar": {}, "confirmar": {}, "continúa": {}, "continua": {},
	"sì": {}, "conferma": {},
	"sim": {},
	"да":  {}, "продолжить": {}, "подтвердить": {},
	"はい": {}, "続行": {}, "確認する": {},
	"네": {}, "예": {}, "계속": {}, "확인": {},
	"ano": {}, "pokračovat": {}, "pokracovat": {}, "potvrdit": {},
	"tak": {}, "kontynuuj": {}, "potwierdz": {},
	"نعم": {},
	"ναι": {}, "συνέχεια": {}, "συνεχίστε": {}, "συνεχισε": {},
	"igen": {}, "folytatás": {}, "folytatas": {}, "megerősít": {}, "megerosit": {},
	"da": {}, "nastavi": {}, "potvrdi": {}, "confirmă": {}, "confirma": {},
	"fortsett": {}, "bekreft": {},
	"fortsätt": {}, "fortsaett": {}, "bekräfta": {}, "bekrafta": {},
	"lean ar aghaidh": {}, "deimhnigh": {},
	"അതെ": {}, "തുടരുക": {}, "സ്ഥിരീകരിക്കുക": {},
}

var browserCheckpointDenyTokens = map[string]struct{}{
	"nah": {}, "nope": {}, "pass": {}, "cancel it": {}, "stop it": {}, "don't": {}, "dont": {},
	"2": {}, "n": {}, "no": {}, "cancel": {}, "stop": {}, "deny": {}, "reject": {}, "abort": {},
	"取消": {}, "拒绝": {}, "不要": {}, "停止": {}, "中止": {}, "取消吧": {}, "拒絕": {}, "算了": {}, "不用了": {}, "别继续": {}, "不继续": {},
	"non": {}, "annuler": {}, "arrêter": {}, "arreter": {}, "refuser": {},
	"nein": {}, "abbrechen": {}, "stopp": {}, "ablehnen": {},
	"cancelar": {}, "detener": {}, "rechazar": {},
	"annulla": {}, "ferma": {}, "rifiuta": {},
	"não": {}, "nao": {}, "parar": {}, "recusar": {},
	"нет": {}, "отмена": {}, "стоп": {}, "отклонить": {},
	"いいえ": {}, "キャンセル": {}, "拒否": {},
	"아니요": {}, "아니오": {}, "취소": {}, "중지": {}, "거부": {},
	"ne": {}, "zrušit": {}, "zrusit": {}, "zamítnout": {}, "zamitnout": {},
	"nie": {}, "anuluj": {}, "odrzuć": {}, "odrzuc": {},
	"όχι": {}, "ακύρωση": {}, "ακυρωση": {}, "σταμάτα": {}, "σταματα": {},
	"nem": {}, "mégse": {}, "megse": {}, "elutasít": {}, "elutasit": {},
	"otkaži": {}, "otkazi": {}, "odbij": {},
	"nu": {}, "anulează": {}, "anuleaza": {}, "respinge": {},
	"stans": {}, "afbryd": {},
	"nei": {}, "avbryt": {},
	"nej": {},
	"ná":  {}, "na": {}, "cealaigh": {}, "diúltaigh": {}, "diultaigh": {},
	"ഇല്ല": {}, "റദ്ദാക്കുക": {}, "നിർത്തുക": {},
}

func addBrowserCheckpointTokens(tokens map[string]struct{}, values ...string) {
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		tokens[normalized] = struct{}{}
	}
}

func init() {
	addBrowserCheckpointTokens(browserCheckpointApproveTokens,
		"alright", "carry on", "please continue", "continue please", "please proceed", "do it",
		"同意", "允许", "允許", "批准",
		"endavant", "segueix",
		"pokračuj", "pokracuj",
		"fortsæt", "fortsaet", "bekræft", "bekraeft",
		"fortfahren", "bestätige", "bestaetige",
		"επιβεβαίωσε", "επιβεβαιωσε", "συνέχισε", "συνεχισε",
		"adelante", "sigue",
		"continuez", "d'accord", "daccord",
		"ceadaigh",
		"folytasd", "rendben",
		"procedi",
		"進めて", "承認",
		"계속해", "승인",
		"ശരി", "തുടരാം",
		"doorgaan", "bevestig", "bevestigen", "akkoord",
		"dalej", "zatwierdź", "zatwierdz",
		"prosseguir", "seguir",
		"continuă", "continua",
		"продолжай", "подтверждаю",
		"potvrdiť",
		"kör på", "kor pa",
	)
	addBrowserCheckpointTokens(browserCheckpointDenyTokens,
		"not now", "never mind", "please cancel", "cancel please",
		"不同意", "取消操作", "不要继续", "不要繼續",
		"atura", "cancel·la",
		"odmítnout", "odmitnout", "zastav", "zastavit",
		"annuller",
		"stoppen",
		"σταμάτησε", "σταματησε", "απόρριψε", "απορριψε",
		"rechaza", "cancela", "para",
		"refuse", "arrête", "arrete",
		"stad",
		"prekini",
		"állj", "allj",
		"fermati",
		"やめて", "中止して",
		"멈춰", "중단",
		"വേണ്ട",
		"annuleren", "weiger",
		"przerwij", "zatrzymaj",
		"cancele", "rejeite", "pare",
		"oprește", "opreste",
		"отмени", "откажи",
		"odmietni",
		"stoppa",
	)
}

// ParseBrowserCheckpointDecision parses a free-text IM/voice reply into a decision.
func ParseBrowserCheckpointDecision(text string) (BrowserCheckpointDecision, bool) {
	normalized := strings.ToLower(strings.TrimSpace(text))
	normalized = strings.Trim(normalized, " \t\r\n.,!?;:，。！？；：、~～`'\"“”‘’()（）[]【】")
	normalized = strings.Join(strings.Fields(normalized), " ")
	if normalized == "" {
		return BrowserCheckpointPending, false
	}
	if _, ok := browserCheckpointApproveTokens[normalized]; ok {
		return BrowserCheckpointApprove, true
	}
	if _, ok := browserCheckpointDenyTokens[normalized]; ok {
		return BrowserCheckpointDeny, true
	}
	return BrowserCheckpointPending, false
}
