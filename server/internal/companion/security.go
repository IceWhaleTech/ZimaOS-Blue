package companion

import (
	"context"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/google/uuid"
)

// SecurityIntegration provides integration between companion monitoring
// and the security subsystem (threat detector, audit logs, sandbox).
type SecurityIntegration struct {
	manager        *Manager
	threatDetector *security.ThreatDetector
	config         *SecurityIntegrationConfig
}

// NewSecurityIntegration creates a new security integration.
func NewSecurityIntegration(manager *Manager, threatDetector *security.ThreatDetector, config *SecurityIntegrationConfig) *SecurityIntegration {
	si := &SecurityIntegration{
		manager:        manager,
		threatDetector: threatDetector,
		config:         config,
	}

	// Set up threat detector callback if enabled
	if config.PromptGuardIntegration && threatDetector != nil {
		threatDetector.SetOnThreat(si.onThreatDetected)
	}

	return si
}

// onThreatDetected is called when the threat detector detects a threat.
// It converts the threat event to a companion session event.
func (si *SecurityIntegration) onThreatDetected(threat security.ThreatEvent) {
	// Find active session for this user/IP
	sessions := si.manager.GetActiveSessions()
	var targetSession *Session

	for _, session := range sessions {
		// Match by user ID or client IP
		if (threat.UserID != "" && session.UserID == threat.UserID) ||
			(threat.IPAddress != "" && session.Metadata.ClientIP == threat.IPAddress) {
			targetSession = session
			break
		}
	}

	// If no matching session found, skip
	if targetSession == nil {
		return
	}

	// Convert threat to companion event
	event := si.convertThreatToEvent(targetSession, threat)

	// Emit the event
	ctx := context.Background()
	_ = si.manager.EmitEvent(ctx, event)
}

// convertThreatToEvent converts a security threat event to a companion session event.
func (si *SecurityIntegration) convertThreatToEvent(session *Session, threat security.ThreatEvent) *SessionEvent {
	return &SessionEvent{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		Timestamp: threat.Timestamp,
		EventType: EventSecurityThreat,
		Platform:  session.Platform,
		UserID:    session.UserID,
		TenantID:  session.TenantID,
		Status:    si.getActionFromThreat(threat),
		Security: &SecurityEvent{
			ThreatLevel:      si.mapThreatSeverity(threat.Severity),
			ThreatScore:      si.calculateThreatScore(threat),
			ThreatTypes:      []string{string(threat.Type)},
			DetectedPatterns: []string{threat.Details},
			Action:           si.getActionFromThreat(threat),
			Source:           "prompt_guard",
		},
	}
}

// mapThreatSeverity maps security.ThreatSeverity to companion.ThreatLevel.
func (si *SecurityIntegration) mapThreatSeverity(severity security.ThreatSeverity) ThreatLevel {
	switch severity {
	case security.SeverityLow:
		return ThreatLevelLow
	case security.SeverityMedium:
		return ThreatLevelMedium
	case security.SeverityHigh:
		return ThreatLevelHigh
	case security.SeverityCritical:
		return ThreatLevelCritical
	default:
		return ThreatLevelNone
	}
}

// calculateThreatScore calculates a numeric threat score from a threat event.
func (si *SecurityIntegration) calculateThreatScore(threat security.ThreatEvent) int {
	baseScore := 0
	switch threat.Severity {
	case security.SeverityLow:
		baseScore = 25
	case security.SeverityMedium:
		baseScore = 50
	case security.SeverityHigh:
		baseScore = 75
	case security.SeverityCritical:
		baseScore = 100
	}

	// Adjust based on whether it was blocked
	if threat.Blocked {
		baseScore -= 10 // Slightly lower if blocked
	}

	if baseScore < 0 {
		baseScore = 0
	}
	if baseScore > 100 {
		baseScore = 100
	}

	return baseScore
}

// getActionFromThreat determines the action taken based on the threat.
func (si *SecurityIntegration) getActionFromThreat(threat security.ThreatEvent) string {
	if threat.Blocked {
		return "blocked"
	}
	return "allowed"
}

// EmitSandboxEvent emits a sandbox execution event to the companion system.
func (si *SecurityIntegration) EmitSandboxEvent(ctx context.Context, sessionID string, toolName string, duration time.Duration, status string, resources *ResourceUsage) error {
	if !si.config.SandboxMonitor {
		return nil
	}

	session, err := si.manager.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	event := &SessionEvent{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		EventType: EventSandboxExec,
		Platform:  session.Platform,
		UserID:    session.UserID,
		TenantID:  session.TenantID,
		Duration:  FromDuration(duration),
		Status:    status,
		ToolCall: &ToolCallEvent{
			ToolName:    toolName,
			ToolID:      uuid.New().String(),
			Duration:    FromDuration(duration),
			Status:      status,
			SandboxUsed: true,
			Resources:   resources,
		},
	}

	return si.manager.EmitEvent(ctx, event)
}

// EmitAuditEvent converts an audit log entry to a companion event.
// This is called when audit log integration is enabled.
func (si *SecurityIntegration) EmitAuditEvent(ctx context.Context, sessionID string, action string, resource string, details map[string]interface{}) error {
	if !si.config.AuditLogIntegration {
		return nil
	}

	session, err := si.manager.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Determine event type based on action
	eventType := SessionEventType("custom")
	var securityEvent *SecurityEvent

	// Check if this is a security-related audit event
	if isSecurityAction(action) {
		eventType = EventSecurityThreat
		securityEvent = &SecurityEvent{
			ThreatLevel: ThreatLevelLow,
			ThreatScore: 10,
			ThreatTypes: []string{action},
			Action:      "logged",
			Source:      "audit",
		}
	}

	event := &SessionEvent{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		EventType: eventType,
		Platform:  session.Platform,
		UserID:    session.UserID,
		TenantID:  session.TenantID,
		Status:    "success",
		Security:  securityEvent,
	}

	return si.manager.EmitEvent(ctx, event)
}

// isSecurityAction checks if an audit action is security-related.
func isSecurityAction(action string) bool {
	securityActions := map[string]bool{
		"login_failed":       true,
		"permission_denied":  true,
		"rate_limited":       true,
		"token_revoked":      true,
		"suspicious_activity": true,
		"config_changed":     true,
	}
	return securityActions[action]
}

// OnPromptGuardResult is called when Prompt Guard completes analysis.
// This provides detailed prompt injection detection results.
func (si *SecurityIntegration) OnPromptGuardResult(ctx context.Context, sessionID string, result *PromptGuardResult) error {
	if !si.config.PromptGuardIntegration {
		return nil
	}

	session, err := si.manager.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	// Only emit event if threats were detected
	if result.ThreatLevel == ThreatLevelNone && len(result.DetectedPatterns) == 0 {
		return nil
	}

	event := &SessionEvent{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Timestamp: time.Now(),
		EventType: EventSecurityThreat,
		Platform:  session.Platform,
		UserID:    session.UserID,
		TenantID:  session.TenantID,
		Status:    result.Action,
		Security: &SecurityEvent{
			ThreatLevel:      result.ThreatLevel,
			ThreatScore:      result.ThreatScore,
			ThreatTypes:      result.ThreatTypes,
			DetectedPatterns: result.DetectedPatterns,
			Action:           result.Action,
			FilteredContent:  result.FilteredContent,
			Source:           "prompt_guard",
		},
	}

	return si.manager.EmitEvent(ctx, event)
}

// PromptGuardResult represents the result of Prompt Guard analysis.
type PromptGuardResult struct {
	ThreatLevel      ThreatLevel `json:"threat_level"`
	ThreatScore      int         `json:"threat_score"`
	ThreatTypes      []string    `json:"threat_types"`
	DetectedPatterns []string    `json:"detected_patterns"`
	Action           string      `json:"action"` // allowed, blocked, filtered
	FilteredContent  string      `json:"filtered_content,omitempty"`
}
