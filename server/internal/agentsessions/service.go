package agentsessions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const templateOnlyACPProfileMessage = "ACP built-in profiles are setup templates only. Duplicate one and configure a runnable command before using it."

var (
	ErrBuiltinProfileDeleteDenied = errors.New("built-in profiles cannot be deleted")
	ErrProfileInUse               = errors.New("agent profile is still used by one or more sessions")
)

type CreateSessionParams struct {
	ProfileID      string                 `json:"profile_id"`
	UserID         string                 `json:"user_id,omitempty"`
	Name           string                 `json:"name,omitempty"`
	CWD            string                 `json:"cwd,omitempty"`
	InitialMessage string                 `json:"initial_message,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type SendMessageParams struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type SessionDetail struct {
	Session   ExternalSession      `json:"session"`
	Profile   *AgentProfile        `json:"profile,omitempty"`
	ActiveRun *ExternalRun         `json:"active_run,omitempty"`
	History   []SessionHistoryItem `json:"history,omitempty"`
}

type sessionWorker struct {
	queue chan string
}

type Service struct {
	store    *SQLiteStore
	runtimes map[ProtocolKind]ProtocolRuntime
	logger   *zap.Logger

	workersMu sync.Mutex
	workers   map[string]*sessionWorker

	subsMu sync.RWMutex
	subs   map[string]map[chan RunEvent]struct{}
}

func NewService(store *SQLiteStore, logger *zap.Logger, runtimes map[ProtocolKind]ProtocolRuntime) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("agent sessions service requires a store")
	}
	if len(runtimes) == 0 {
		return nil, fmt.Errorf("agent sessions service requires at least one runtime")
	}
	svc := &Service{
		store:    store,
		runtimes: runtimes,
		logger:   logger,
		workers:  make(map[string]*sessionWorker),
		subs:     make(map[string]map[chan RunEvent]struct{}),
	}
	if err := svc.SeedBuiltinProfiles(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) SeedBuiltinProfiles() error {
	existing, err := s.store.ListProfiles()
	if err != nil {
		return err
	}
	existingByID := make(map[string]AgentProfile, len(existing))
	migratedByBuiltinID := make(map[string]AgentProfile)
	for _, profile := range existing {
		existingByID[profile.ID] = profile
		if sourceBuiltinID := migratedFromBuiltinProfileID(profile); sourceBuiltinID != "" && !profile.Builtin {
			migratedByBuiltinID[sourceBuiltinID] = profile
		}
	}
	for _, builtin := range builtinProfiles() {
		existingProfile, ok := existingByID[builtin.ID]
		if ok {
			if needsLegacyBuiltinACPMigration(existingProfile, builtin) {
				migratedProfile, migrated := migratedByBuiltinID[builtin.ID]
				if !migrated {
					migratedProfile = buildMigratedACPProfile(existingProfile)
					if err := s.store.SaveProfile(&migratedProfile); err != nil {
						return err
					}
					migratedByBuiltinID[builtin.ID] = migratedProfile
				}
				if err := s.rebindSessionsToProfile(existingProfile.ID, migratedProfile.ID); err != nil {
					return err
				}
			}
			normalized := normalizeBuiltinProfile(existingProfile, builtin)
			if err := s.store.SaveProfile(&normalized); err != nil {
				return err
			}
			continue
		}
		cp := builtin
		if err := s.store.SaveProfile(&cp); err != nil {
			return err
		}
	}
	return nil
}

func builtinProfiles() []AgentProfile {
	now := timeutil.NowTime().UTC()
	return []AgentProfile{
		{
			ID:                   "claude",
			Protocol:             ProtocolACP,
			Name:                 "claude",
			Title:                "Claude Code ACP",
			Description:          "Setup template for Claude Code ACP runtimes.",
			Builtin:              true,
			TemplateOnly:         true,
			CredentialProviderID: "anthropic",
			Metadata: map[string]interface{}{
				"reference": "openclaw/acpx",
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:                   "codex",
			Protocol:             ProtocolACP,
			Name:                 "codex",
			Title:                "Codex ACP",
			Description:          "Setup template for Codex ACP runtimes.",
			Builtin:              true,
			TemplateOnly:         true,
			CredentialProviderID: "openai-codex",
			Metadata: map[string]interface{}{
				"reference": "openclaw/acpx",
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:                   "gemini",
			Protocol:             ProtocolACP,
			Name:                 "gemini",
			Title:                "Gemini CLI ACP",
			Description:          "Setup template for Gemini CLI ACP runtimes.",
			Builtin:              true,
			TemplateOnly:         true,
			CredentialProviderID: "google-gemini-cli",
			Metadata: map[string]interface{}{
				"reference": "openclaw/acpx",
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
		{
			ID:          "generic-a2a",
			Protocol:    ProtocolA2A,
			Name:        "generic-a2a",
			Title:       "Generic Remote A2A Agent",
			Description: "Remote A2A agent card profile.",
			Builtin:     true,
			Metadata: map[string]interface{}{
				"reference": "openclaw/acpx",
			},
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

func (s *Service) ListProfiles() ([]AgentProfile, error) {
	return s.store.ListProfiles()
}

func (s *Service) GetProfile(id string) (*AgentProfile, error) {
	return s.store.GetProfile(id)
}

func (s *Service) SaveProfile(profile *AgentProfile) error {
	if profile == nil {
		return fmt.Errorf("profile is nil")
	}
	if existing, err := s.store.GetProfile(profile.ID); err == nil && existing.Builtin {
		return fmt.Errorf("built-in profiles are read-only")
	}
	if profile.Protocol == "" {
		return fmt.Errorf("profile protocol is required")
	}
	if strings.TrimSpace(profile.Name) == "" {
		return fmt.Errorf("profile name is required")
	}
	profile.Builtin = false
	profile.TemplateOnly = false
	return s.store.SaveProfile(profile)
}

func (s *Service) DeleteProfile(id string) error {
	profile, err := s.store.GetProfile(id)
	if err != nil {
		return err
	}
	if profile.Builtin {
		return ErrBuiltinProfileDeleteDenied
	}
	sessions, err := s.store.ListSessionsByProfileID(profile.ID)
	if err != nil {
		return err
	}
	if count := len(sessions); count > 0 {
		suffix := "s"
		if count == 1 {
			suffix = ""
		}
		return fmt.Errorf("%w: still used by %d session%s", ErrProfileInUse, count, suffix)
	}
	return s.store.DeleteProfile(profile.ID)
}

func (s *Service) VerifyProfile(ctx context.Context, id string, candidate *AgentProfile) (*ProfileVerifyResult, error) {
	profile, err := s.resolveProfileForCheck(id, candidate)
	if err != nil {
		return nil, err
	}
	if err := validateRunnableACPProfile(profile); err != nil {
		return nil, err
	}
	runtime, err := s.runtimeFor(profile.Protocol)
	if err != nil {
		return nil, err
	}
	result, err := runtime.VerifyProfile(ctx, *profile)
	if err != nil {
		return nil, err
	}
	profile.LastVerifiedAt = timeutil.NowTime().UTC()
	profile.HealthStatus = "verified"
	profile.HealthMessage = result.Message
	_ = s.store.SaveProfile(profile)
	return result, nil
}

func (s *Service) HealthProfile(ctx context.Context, id string) (*ProfileHealthResult, error) {
	profile, err := s.store.GetProfile(id)
	if err != nil {
		return nil, err
	}
	if err := validateRunnableACPProfile(profile); err != nil {
		return nil, err
	}
	runtime, err := s.runtimeFor(profile.Protocol)
	if err != nil {
		return nil, err
	}
	result, err := runtime.Health(ctx, *profile)
	if err != nil {
		return nil, err
	}
	profile.LastHealthAt = timeutil.NowTime().UTC()
	if result.Healthy {
		profile.HealthStatus = "healthy"
	} else {
		profile.HealthStatus = "error"
	}
	profile.HealthMessage = result.Message
	_ = s.store.SaveProfile(profile)
	return result, nil
}

func (s *Service) ListSessions(limit, offset int, userID string, protocol ProtocolKind) ([]ExternalSession, error) {
	return s.store.ListSessions(limit, offset, userID, protocol)
}

func (s *Service) GetSessionDetail(sessionID string, historyLimit int) (*SessionDetail, error) {
	session, err := s.store.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	profile, _ := s.store.GetProfile(session.ProfileID)
	activeRun, _ := s.store.LatestActiveRun(session.ID)
	history, err := s.store.BuildHistory(session.ID, historyLimit)
	if err != nil {
		return nil, err
	}
	return &SessionDetail{
		Session:   *session,
		Profile:   profile,
		ActiveRun: activeRun,
		History:   history,
	}, nil
}

func (s *Service) GetSessionHistory(sessionID string, limit int) ([]SessionHistoryItem, error) {
	return s.store.BuildHistory(sessionID, limit)
}

func (s *Service) CreateSession(ctx context.Context, params CreateSessionParams) (*SessionDetail, error) {
	profile, err := s.store.GetProfile(params.ProfileID)
	if err != nil {
		return nil, err
	}
	if err := validateRunnableACPProfile(profile); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = profile.Title
		if name == "" {
			name = profile.Name
		}
	}
	session := &ExternalSession{
		ProfileID: params.ProfileID,
		Protocol:  profile.Protocol,
		UserID:    strings.TrimSpace(params.UserID),
		Name:      name,
		CWD:       strings.TrimSpace(params.CWD),
		Status:    SessionStatusCreating,
		Metadata:  cloneMap(params.Metadata),
	}
	if err := s.store.SaveSession(session); err != nil {
		return nil, err
	}
	session.Status = SessionStatusIdle
	if err := s.store.SaveSession(session); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.InitialMessage) != "" {
		if _, err := s.SendMessage(ctx, SendMessageParams{
			SessionID: session.ID,
			Message:   params.InitialMessage,
		}); err != nil {
			return nil, err
		}
	}
	return s.GetSessionDetail(session.ID, 50)
}

func (s *Service) SendMessage(ctx context.Context, params SendMessageParams) (*ExternalRun, error) {
	session, err := s.store.GetSession(params.SessionID)
	if err != nil {
		return nil, err
	}
	profile, err := s.store.GetProfile(session.ProfileID)
	if err != nil {
		return nil, err
	}
	if err := validateRunnableACPProfile(profile); err != nil {
		return nil, err
	}
	if session.Status == SessionStatusClosed {
		return nil, fmt.Errorf("session is closed")
	}
	message := strings.TrimSpace(params.Message)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	run := &ExternalRun{
		SessionID: session.ID,
		Status:    RunStatusQueued,
		Prompt:    message,
	}
	if err := s.store.SaveRun(run); err != nil {
		return nil, err
	}
	if _, err := s.persistAndPublishEvent(session.ID, run.ID, "user_message", map[string]interface{}{
		"role":    "user",
		"content": message,
		"text":    message,
	}); err != nil {
		return nil, err
	}
	s.enqueueRun(session.ID, run.ID)
	return run, nil
}

func (s *Service) CancelSession(ctx context.Context, sessionID string) error {
	session, err := s.store.GetSession(sessionID)
	if err != nil {
		return err
	}
	run, err := s.store.LatestActiveRun(sessionID)
	if err != nil {
		return err
	}
	if run == nil {
		return nil
	}
	profile, err := s.store.GetProfile(session.ProfileID)
	if err != nil {
		return err
	}
	session.Status = SessionStatusCancelling
	if err := s.store.SaveSession(session); err != nil {
		return err
	}
	_, _ = s.persistAndPublishEvent(session.ID, run.ID, "cancel_requested", map[string]interface{}{
		"status": "cancelling",
	})
	runtime, err := s.runtimeFor(profile.Protocol)
	if err != nil {
		return err
	}
	return runtime.CancelRun(ctx, *session, *run)
}

func (s *Service) CloseSession(ctx context.Context, sessionID string) error {
	session, err := s.store.GetSession(sessionID)
	if err != nil {
		return err
	}
	profile, err := s.store.GetProfile(session.ProfileID)
	if err != nil {
		return err
	}
	runtime, err := s.runtimeFor(profile.Protocol)
	if err != nil {
		return err
	}
	if err := runtime.CloseSession(ctx, *profile, *session); err != nil {
		return err
	}
	session.Status = SessionStatusClosed
	session.ClosedAt = timeutil.NowTime().UTC()
	if err := s.store.SaveSession(session); err != nil {
		return err
	}
	_, _ = s.persistAndPublishEvent(session.ID, "", "session_closed", map[string]interface{}{
		"status": "closed",
	})
	return nil
}

func (s *Service) ListEvents(sessionID string, limit int, afterID int64) ([]RunEvent, error) {
	return s.store.ListEvents(sessionID, limit, afterID)
}

func (s *Service) SubscribeSession(sessionID string) (<-chan RunEvent, func()) {
	ch := make(chan RunEvent, 32)
	key := strings.TrimSpace(sessionID)
	s.subsMu.Lock()
	defer s.subsMu.Unlock()
	if s.subs[key] == nil {
		s.subs[key] = make(map[chan RunEvent]struct{})
	}
	s.subs[key][ch] = struct{}{}
	cancel := func() {
		s.subsMu.Lock()
		defer s.subsMu.Unlock()
		if subs := s.subs[key]; subs != nil {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(s.subs, key)
			}
		}
		close(ch)
	}
	return ch, cancel
}

func (s *Service) resolveProfileForCheck(id string, candidate *AgentProfile) (*AgentProfile, error) {
	if strings.TrimSpace(id) != "" {
		return s.store.GetProfile(id)
	}
	if candidate == nil {
		return nil, fmt.Errorf("profile id or body is required")
	}
	cp := *candidate
	return &cp, nil
}

func (s *Service) runtimeFor(kind ProtocolKind) (ProtocolRuntime, error) {
	runtime := s.runtimes[kind]
	if runtime == nil {
		return nil, fmt.Errorf("runtime not configured for protocol %s", kind)
	}
	return runtime, nil
}

func (s *Service) enqueueRun(sessionID, runID string) {
	s.workersMu.Lock()
	worker := s.workers[sessionID]
	if worker == nil {
		worker = &sessionWorker{queue: make(chan string, 32)}
		s.workers[sessionID] = worker
		go s.runWorker(strings.TrimSpace(sessionID), worker)
	}
	s.workersMu.Unlock()

	worker.queue <- runID
}

func (s *Service) runWorker(sessionID string, worker *sessionWorker) {
	timer := time.NewTimer(10 * time.Minute)
	defer timer.Stop()
	for {
		select {
		case runID := <-worker.queue:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(10 * time.Minute)
			if err := s.processRun(context.Background(), sessionID, runID); err != nil && s.logger != nil {
				s.logger.Warn("agent session run failed", zap.String("session_id", sessionID), zap.String("run_id", runID), zap.Error(err))
			}
		case <-timer.C:
			s.workersMu.Lock()
			delete(s.workers, sessionID)
			s.workersMu.Unlock()
			return
		}
	}
}

func (s *Service) processRun(ctx context.Context, sessionID, runID string) error {
	session, err := s.store.GetSession(sessionID)
	if err != nil {
		return err
	}
	run, err := s.store.GetRun(runID)
	if err != nil {
		return err
	}
	profile, err := s.store.GetProfile(session.ProfileID)
	if err != nil {
		return err
	}
	runtime, err := s.runtimeFor(profile.Protocol)
	if err != nil {
		return err
	}
	if err := validateRunnableACPProfile(profile); err != nil {
		return s.failRun(session, run, err)
	}

	session.Status = SessionStatusRunning
	run.Status = RunStatusRunning
	run.StartedAt = timeutil.NowTime().UTC()
	if err := s.store.SaveSession(session); err != nil {
		return err
	}
	if err := s.store.SaveRun(run); err != nil {
		return err
	}
	_, _ = s.persistAndPublishEvent(session.ID, run.ID, "run_started", map[string]interface{}{
		"status": string(run.Status),
	})

	ensureResult, err := runtime.EnsureSession(ctx, EnsureSessionRequest{
		Profile: *profile,
		Session: *session,
	})
	if err != nil {
		return s.failRun(session, run, err)
	}
	if ensureResult != nil && strings.TrimSpace(ensureResult.RemoteSessionID) != "" {
		session.RemoteSessionID = strings.TrimSpace(ensureResult.RemoteSessionID)
		_ = s.store.SaveSession(session)
	}
	if ensureResult != nil && len(ensureResult.Metadata) > 0 {
		if session.Metadata == nil {
			session.Metadata = map[string]interface{}{}
		}
		for k, v := range ensureResult.Metadata {
			session.Metadata[k] = v
		}
		_ = s.store.SaveSession(session)
	}

	submitResult, err := runtime.SubmitRun(ctx, SubmitRunRequest{
		Profile: *profile,
		Session: *session,
		Run:     *run,
		Prompt:  run.Prompt,
	})
	if err != nil {
		return s.failRun(session, run, err)
	}
	if submitResult != nil && strings.TrimSpace(submitResult.RemoteRunID) != "" {
		run.RemoteRunID = strings.TrimSpace(submitResult.RemoteRunID)
		_ = s.store.SaveRun(run)
	}

	stream, err := runtime.StreamRun(ctx, StreamRunRequest{
		Profile: *profile,
		Session: *session,
		Run:     *run,
	})
	if err != nil {
		return s.failRun(session, run, err)
	}

	var assistantText strings.Builder
	for event := range stream.Events() {
		if runtimeEventContributesToAssistantTranscript(event) && strings.TrimSpace(event.Text) != "" {
			assistantText.WriteString(event.Text)
		}
		if _, err := s.persistRuntimeEvent(session.ID, run.ID, event); err != nil && s.logger != nil {
			s.logger.Warn("failed to persist runtime event", zap.String("session_id", session.ID), zap.String("run_id", run.ID), zap.Error(err))
		}
	}

	waitErr := stream.Wait()
	if waitErr != nil {
		if isCancelledError(waitErr) || session.Status == SessionStatusCancelling {
			run.Status = RunStatusCancelled
			run.Error = waitErr.Error()
			run.CompletedAt = timeutil.NowTime().UTC()
			session.Status = SessionStatusIdle
			_ = s.store.SaveRun(run)
			_ = s.store.SaveSession(session)
			_, _ = s.persistAndPublishEvent(session.ID, run.ID, "run_cancelled", map[string]interface{}{
				"status": string(run.Status),
				"error":  run.Error,
			})
			return nil
		}
		return s.failRun(session, run, waitErr)
	}

	finalText := strings.TrimSpace(assistantText.String())
	if finalText != "" {
		_, _ = s.persistAndPublishEvent(session.ID, run.ID, "assistant_message", map[string]interface{}{
			"role":    "assistant",
			"content": finalText,
			"text":    finalText,
		})
	}

	run.Status = RunStatusCompleted
	run.CompletedAt = timeutil.NowTime().UTC()
	session.Status = SessionStatusIdle
	if err := s.store.SaveRun(run); err != nil {
		return err
	}
	if err := s.store.SaveSession(session); err != nil {
		return err
	}
	_, _ = s.persistAndPublishEvent(session.ID, run.ID, "run_completed", map[string]interface{}{
		"status":      string(run.Status),
		"stop_reason": run.StopReason,
	})
	return nil
}

func (s *Service) failRun(session *ExternalSession, run *ExternalRun, failure error) error {
	run.Status = RunStatusFailed
	run.Error = failure.Error()
	run.CompletedAt = timeutil.NowTime().UTC()
	session.Status = SessionStatusError
	session.LastError = failure.Error()
	if err := s.store.SaveRun(run); err != nil {
		return err
	}
	if err := s.store.SaveSession(session); err != nil {
		return err
	}
	_, _ = s.persistAndPublishEvent(session.ID, run.ID, "run_failed", map[string]interface{}{
		"status": string(run.Status),
		"error":  run.Error,
	})
	return failure
}

func (s *Service) persistRuntimeEvent(sessionID, runID string, event RuntimeEvent) (*RunEvent, error) {
	payload := map[string]interface{}{
		"type":   event.Type,
		"role":   event.Role,
		"text":   event.Text,
		"error":  event.Error,
		"status": event.Status,
		"remote": event.Remote,
	}
	for k, v := range event.Payload {
		payload[k] = v
	}
	eventType := strings.TrimSpace(event.Type)
	if eventType == "" {
		eventType = "runtime_event"
	}
	return s.persistAndPublishEvent(sessionID, runID, eventType, payload)
}

func (s *Service) persistAndPublishEvent(sessionID, runID, eventType string, payload interface{}) (*RunEvent, error) {
	event, err := s.store.AppendEvent(sessionID, runID, eventType, payload)
	if err != nil {
		return nil, err
	}
	s.publishEvent(*event)
	return event, nil
}

func (s *Service) publishEvent(event RunEvent) {
	s.subsMu.RLock()
	defer s.subsMu.RUnlock()
	for ch := range s.subs[event.SessionID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func isCancelledError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "cancel") || strings.Contains(text, "context canceled")
}

func validateRunnableACPProfile(profile *AgentProfile) error {
	if profile == nil {
		return nil
	}
	if profile.Protocol != ProtocolACP {
		return nil
	}
	if !profile.TemplateOnly {
		return nil
	}
	return fmt.Errorf(templateOnlyACPProfileMessage)
}

func migratedFromBuiltinProfileID(profile AgentProfile) string {
	if len(profile.Metadata) == 0 {
		return ""
	}
	source, _ := profile.Metadata["migrated_from_builtin_profile_id"].(string)
	return strings.TrimSpace(source)
}

func needsLegacyBuiltinACPMigration(existing AgentProfile, builtin AgentProfile) bool {
	if builtin.Protocol != ProtocolACP {
		return false
	}
	if existing.Protocol != ProtocolACP {
		return false
	}
	if existing.TemplateOnly {
		return false
	}
	return len(existing.Command) > 0
}

func normalizeBuiltinProfile(existing AgentProfile, builtin AgentProfile) AgentProfile {
	normalized := existing
	normalized.Protocol = builtin.Protocol
	normalized.Builtin = true
	normalized.TemplateOnly = builtin.TemplateOnly
	normalized.Command = append([]string(nil), builtin.Command...)
	normalized.Env = nil
	normalized.CWD = ""
	normalized.CardURL = builtin.CardURL
	normalized.EndpointURL = builtin.EndpointURL
	normalized.Headers = nil
	normalized.AuthMethodID = ""
	if strings.TrimSpace(normalized.Name) == "" {
		normalized.Name = builtin.Name
	}
	if strings.TrimSpace(normalized.Title) == "" {
		normalized.Title = builtin.Title
	}
	if strings.TrimSpace(normalized.Description) == "" {
		normalized.Description = builtin.Description
	}
	if strings.TrimSpace(normalized.CredentialProviderID) == "" {
		normalized.CredentialProviderID = builtin.CredentialProviderID
	}
	if len(normalized.Metadata) == 0 {
		normalized.Metadata = cloneMap(builtin.Metadata)
	} else if normalized.Metadata["reference"] == nil && builtin.Metadata["reference"] != nil {
		normalized.Metadata["reference"] = builtin.Metadata["reference"]
	}
	return normalized
}

func buildMigratedACPProfile(existing AgentProfile) AgentProfile {
	migrated := existing
	migrated.ID = ""
	migrated.Builtin = false
	migrated.TemplateOnly = false
	migrated.HealthStatus = ""
	migrated.HealthMessage = ""
	migrated.LastVerifiedAt = time.Time{}
	migrated.LastHealthAt = time.Time{}
	if baseName := strings.TrimSpace(migrated.Name); baseName != "" {
		migrated.Name = baseName + "-migrated"
	} else {
		migrated.Name = existing.ID + "-migrated"
	}
	if title := strings.TrimSpace(migrated.Title); title != "" {
		migrated.Title = title + " (Migrated)"
	}
	migrated.Metadata = cloneMap(existing.Metadata)
	if migrated.Metadata == nil {
		migrated.Metadata = map[string]interface{}{}
	}
	migrated.Metadata["migrated_from_builtin_profile_id"] = existing.ID
	return migrated
}

func (s *Service) rebindSessionsToProfile(fromProfileID, toProfileID string) error {
	sessions, err := s.store.ListSessionsByProfileID(fromProfileID)
	if err != nil {
		return err
	}
	for i := range sessions {
		session := sessions[i]
		if session.ProfileID == toProfileID {
			continue
		}
		session.ProfileID = toProfileID
		if err := s.store.SaveSession(&session); err != nil {
			return err
		}
	}
	return nil
}

func runtimeEventContributesToAssistantTranscript(event RuntimeEvent) bool {
	if event.Role != "assistant" {
		return false
	}
	switch event.Type {
	case "assistant_delta", "assistant_message":
		return true
	default:
		return false
	}
}

func payloadForLog(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	return string(raw)
}
