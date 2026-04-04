package skillmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	sec "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
)

type Classifier interface {
	Classify(ctx context.Context, content string) (string, string, error)
}

type VulnProvider interface {
	Analyze(ctx context.Context, surface InstallSurface) (string, []string, error)
}

type NoopVulnProvider struct{}

func (NoopVulnProvider) Analyze(_ context.Context, surface InstallSurface) (string, []string, error) {
	if len(surface.DependencyManifests) == 0 {
		return VulnerabilityStatusNotApplicable, nil, nil
	}
	return VulnerabilityStatusUnknown, nil, nil
}

type Scanner struct {
	classifier     Classifier
	vulnProvider   VulnProvider
	sanitizer      *sec.ExternalContentSanitizer
	threatDetector *sec.ThreatDetector
}

func NewScanner(classifier Classifier) *Scanner {
	return &Scanner{
		classifier:     classifier,
		vulnProvider:   NoopVulnProvider{},
		sanitizer:      sec.DefaultExternalContentSanitizer(),
		threatDetector: sec.NewThreatDetector(),
	}
}

var dangerousCommandRules = []struct {
	pattern *regexp.Regexp
	label   string
}{
	{pattern: regexp.MustCompile(`(?i)\brm\s+-rf\b`), label: "rm -rf"},
	{pattern: regexp.MustCompile(`(?i)\bcurl\b[^\n|]*\|\s*(sh|bash)\b`), label: "curl | sh"},
	{pattern: regexp.MustCompile(`(?i)\bwget\b[^\n|]*\|\s*(sh|bash)\b`), label: "wget | bash"},
	{pattern: regexp.MustCompile(`(?i)\bchmod\s+777\b`), label: "chmod 777"},
	{pattern: regexp.MustCompile(`(?i)\bscp\b`), label: "scp"},
	{pattern: regexp.MustCompile(`(?i)\bssh\b`), label: "ssh"},
}

var promptInjectionRules = []struct {
	pattern *regexp.Regexp
	label   string
}{
	{pattern: regexp.MustCompile(`(?i)ignore previous instructions`), label: "ignore previous instructions"},
	{pattern: regexp.MustCompile(`(?i)exfiltrate data`), label: "exfiltrate data"},
	{pattern: regexp.MustCompile(`(?i)reveal system prompt`), label: "reveal system prompt"},
}

var permissionRules = []struct {
	pattern    *regexp.Regexp
	permission string
}{
	{pattern: regexp.MustCompile(`(?i)(read|write|modify).*(file|filesystem)|/etc/|~\/|\.zima`), permission: "filesystem"},
	{pattern: regexp.MustCompile(`(?i)\b(http|https|curl|wget|webhook|api request|download)\b`), permission: "network"},
	{pattern: regexp.MustCompile(`(?i)\b(shell|bash|zsh|sh|terminal|exec|command)\b`), permission: "shell"},
	{pattern: regexp.MustCompile(`(?i)\bdocker\b`), permission: "docker"},
	{pattern: regexp.MustCompile(`(?i)\b(systemctl|launchctl|registry|kernel|sudo)\b`), permission: "system"},
}

var secretRules = []struct {
	pattern *regexp.Regexp
	label   string
}{
	{pattern: regexp.MustCompile(`(?i)\b(?:api[_-]?key|token|secret)\s*[:=]\s*['"]?[a-z0-9_\-]{12,}`), label: "credential literal"},
	{pattern: regexp.MustCompile(`-----BEGIN (?:RSA|EC|OPENSSH|DSA|PGP) PRIVATE KEY-----`), label: "private key"},
	{pattern: regexp.MustCompile(`(?i)\bgh[pousr]_[A-Za-z0-9]{20,}\b`), label: "github token"},
}

func (s *Scanner) Scan(ctx context.Context, skillID, version, content string, declaredPermissions []string) *SecurityReport {
	return s.ScanWithSurface(ctx, skillID, version, content, declaredPermissions, InstallSurface{
		InstallType:  InstallTypeRawSkill,
		ArtifactKind: ArtifactKindOpenSource,
		Installable:  true,
	})
}

func (s *Scanner) ScanWithSurface(ctx context.Context, skillID, version, content string, declaredPermissions []string, surface InstallSurface) *SecurityReport {
	surface = normalizeInstallSurface(surface)
	report := &SecurityReport{
		SkillID:             skillID,
		Version:             version,
		Score:               100,
		ScannerVersion:      ScannerVersion,
		LLMStatus:           "skipped",
		SecurityBadge:       BadgeYellow,
		VulnerabilityStatus: VulnerabilityStatusUnknown,
		InstallSurface:      surface,
		HasBinary:           surface.HasBinary,
	}

	normalized := strings.ToLower(content)
	commandHits := make(map[string]struct{})
	promptHits := make(map[string]struct{})
	permissionHits := make(map[string]struct{})
	declared := make(map[string]struct{})
	for _, permission := range declaredPermissions {
		declared[strings.ToLower(strings.TrimSpace(permission))] = struct{}{}
	}

	for _, rule := range dangerousCommandRules {
		if rule.pattern.MatchString(content) {
			commandHits[rule.label] = struct{}{}
			report.Risks = append(report.Risks, SecurityFinding{
				Type:     "dangerous_command",
				Severity: "high",
				Command:  rule.label,
				Message:  fmt.Sprintf("Detected dangerous command pattern: %s", rule.label),
			})
			report.Evidence = append(report.Evidence, SecurityEvidence{
				Type:        "dangerous_command",
				Severity:    "high",
				Title:       rule.label,
				Description: "Potentially destructive or remote-execution shell sequence",
				Value:       rule.label,
			})
		}
	}

	for _, rule := range promptInjectionRules {
		if rule.pattern.MatchString(normalized) {
			promptHits[rule.label] = struct{}{}
			report.HasPromptInjection = true
			if rule.label == "exfiltrate data" {
				report.HasDataExfiltration = true
			}
			report.Risks = append(report.Risks, SecurityFinding{
				Type:     "prompt_injection",
				Severity: "high",
				Pattern:  rule.label,
				Message:  fmt.Sprintf("Detected prompt injection pattern: %s", rule.label),
			})
			report.Evidence = append(report.Evidence, SecurityEvidence{
				Type:        "prompt_injection",
				Severity:    "high",
				Title:       rule.label,
				Description: "Prompt text that tries to override or subvert the agent",
				Value:       rule.label,
			})
		}
	}

	if s.sanitizer != nil {
		detection := s.sanitizer.DetectSuspiciousPatterns(content)
		for _, match := range detection.Matches {
			switch match.Pattern.Name {
			case "ignore_instructions", "new_instructions", "role_override", "system_prompt", "jailbreak", "role_separator":
				report.HasPromptInjection = true
			case "data_exfiltration":
				report.HasDataExfiltration = true
			}
			if match.Pattern.Name == "data_exfiltration" || match.Pattern.Name == "jailbreak" || match.Pattern.Name == "system_prompt" {
				report.Evidence = append(report.Evidence, SecurityEvidence{
					Type:        match.Pattern.Name,
					Severity:    match.Pattern.Severity,
					Title:       match.Pattern.Description,
					Description: "Suspicious external content pattern",
					Value:       match.MatchedText,
				})
			}
		}
	}

	if s.threatDetector != nil {
		for _, threat := range s.threatDetector.DetectThreats(content, "skillmarket", "", "") {
			if threat.Type == sec.ThreatTypeCommandInject {
				report.HasShellInjection = true
				report.Evidence = append(report.Evidence, SecurityEvidence{
					Type:        string(threat.Type),
					Severity:    string(threat.Severity),
					Title:       threat.Description,
					Description: "Shell or command injection pattern from shared threat detector",
					Value:       threat.Details,
				})
			}
		}
	}

	for _, rule := range permissionRules {
		if rule.pattern.MatchString(content) {
			permissionHits[rule.permission] = struct{}{}
		}
	}

	for _, rule := range secretRules {
		if rule.pattern.MatchString(content) {
			report.Secrets = append(report.Secrets, rule.label)
			report.Risks = append(report.Risks, SecurityFinding{
				Type:     "secret",
				Severity: "critical",
				Pattern:  rule.label,
				Message:  fmt.Sprintf("Detected embedded secret: %s", rule.label),
			})
			report.Evidence = append(report.Evidence, SecurityEvidence{
				Type:        "secret",
				Severity:    "critical",
				Title:       rule.label,
				Description: "Embedded credential or private key material",
				Value:       rule.label,
			})
		}
	}

	for permission := range permissionHits {
		report.Permissions = append(report.Permissions, permission)
		if _, ok := declared[permission]; !ok {
			report.Risks = append(report.Risks, SecurityFinding{
				Type:       "undeclared_permission",
				Severity:   "medium",
				Permission: permission,
				Message:    fmt.Sprintf("Detected privileged capability not declared in manifest: %s", permission),
			})
			report.Evidence = append(report.Evidence, SecurityEvidence{
				Type:        "permission",
				Severity:    "medium",
				Title:       permission,
				Description: "Privileged capability inferred from skill content but not declared",
				Value:       permission,
			})
			report.Score -= 10
		}
	}

	if surface.HasBinary {
		report.Evidence = append(report.Evidence, SecurityEvidence{
			Type:        "binary_artifact",
			Severity:    "medium",
			Title:       "Binary artifact detected",
			Description: "Skill package contains an executable or opaque binary payload",
		})
	}
	if len(surface.DependencyManifests) > 0 {
		report.Evidence = append(report.Evidence, SecurityEvidence{
			Type:        "dependency_manifest",
			Severity:    "medium",
			Title:       "Dependency manifests detected",
			Description: strings.Join(surface.DependencyManifests, ", "),
			Value:       strings.Join(surface.DependencyManifests, ", "),
		})
	}

	report.Score -= min(len(commandHits)*25, 40)
	report.Score -= min(len(promptHits)*15, 30)
	if len(report.Secrets) > 0 {
		report.Score -= 50
	}
	if report.HasShellInjection {
		report.Score -= 20
	}
	if report.HasDataExfiltration {
		report.Score -= 15
	}
	if hasRiskyPermissionCombo(permissionHits, "network", "shell") || hasRiskyPermissionCombo(permissionHits, "docker", "system") {
		report.Score -= 10
		report.Risks = append(report.Risks, SecurityFinding{
			Type:     "risky_permission_combo",
			Severity: "high",
			Message:  "Detected risky permission combination",
		})
	}

	if report.Score < 0 {
		report.Score = 0
	}

	report.VulnerabilityStatus, report.Vulnerabilities = s.scanVulnerabilities(ctx, surface)
	report.HasVulnerabilities = report.VulnerabilityStatus == VulnerabilityStatusDetected || report.VulnerabilityStatus == VulnerabilityStatusSuspected
	if report.VulnerabilityStatus == VulnerabilityStatusDetected {
		report.Score -= 35
	} else if report.VulnerabilityStatus == VulnerabilityStatusSuspected {
		report.Score -= 10
	}
	if report.Score < 0 {
		report.Score = 0
	}

	sort.Strings(report.Permissions)
	sort.Strings(report.Secrets)
	sort.Strings(report.Vulnerabilities)
	report.RiskLevel = riskLevelForScore(report.Score, len(report.Secrets) > 0)
	report.SecurityBadge = securityBadgeForReport(report)

	if s.classifier != nil {
		raw, status, err := s.classifier.Classify(ctx, content)
		if err == nil {
			report.LLMVerdictJSON = raw
			report.LLMStatus = status
		} else {
			report.LLMStatus = "error"
			report.LLMVerdictJSON = fmt.Sprintf(`{"error":%q}`, err.Error())
		}
	}

	return report
}

func (s *Scanner) scanVulnerabilities(ctx context.Context, surface InstallSurface) (string, []string) {
	if s.vulnProvider == nil {
		s.vulnProvider = NoopVulnProvider{}
	}
	status, vulns, err := s.vulnProvider.Analyze(ctx, surface)
	if err != nil {
		if len(surface.DependencyManifests) == 0 {
			return VulnerabilityStatusNotApplicable, nil
		}
		return VulnerabilityStatusUnknown, nil
	}
	if status == "" {
		if len(surface.DependencyManifests) == 0 {
			return VulnerabilityStatusNotApplicable, nil
		}
		return VulnerabilityStatusUnknown, nil
	}
	return status, vulns
}

func mergeExternalSecuritySignals(report *SecurityReport, external *SourceSecuritySignals) *SecurityReport {
	if report == nil || external == nil {
		return report
	}

	if external.Score != nil && *external.Score >= 0 && *external.Score < report.Score {
		report.Score = *external.Score
	}

	if external.InstallType != "" {
		report.InstallSurface.InstallType = normalizeInstallType(external.InstallType)
	}
	if external.ArtifactKind != "" {
		report.InstallSurface.ArtifactKind = normalizeArtifactKind(external.ArtifactKind)
	}
	if external.Installable != nil {
		report.InstallSurface.Installable = *external.Installable
	}
	if external.HasBinary {
		report.HasBinary = true
		report.InstallSurface.HasBinary = true
	}
	if external.HasScripts {
		report.InstallSurface.HasScripts = true
	}

	if status := mergeVulnerabilityStatus(report.VulnerabilityStatus, external.VulnerabilityStatus); status != report.VulnerabilityStatus {
		switch status {
		case VulnerabilityStatusDetected:
			if report.VulnerabilityStatus != VulnerabilityStatusDetected {
				report.Score -= 35
			}
		case VulnerabilityStatusSuspected:
			if report.VulnerabilityStatus != VulnerabilityStatusDetected && report.VulnerabilityStatus != VulnerabilityStatusSuspected {
				report.Score -= 10
			}
		}
		report.VulnerabilityStatus = status
	}
	if len(external.Vulnerabilities) > 0 {
		report.Vulnerabilities = append(report.Vulnerabilities, external.Vulnerabilities...)
	}
	if external.HasVulnerabilities {
		report.HasVulnerabilities = true
	}
	if report.VulnerabilityStatus == VulnerabilityStatusDetected || report.VulnerabilityStatus == VulnerabilityStatusSuspected {
		report.HasVulnerabilities = true
	}

	if external.HasPromptInjection && !report.HasPromptInjection {
		report.HasPromptInjection = true
		report.Score -= 15
	}
	if external.HasShellInjection && !report.HasShellInjection {
		report.HasShellInjection = true
		report.Score -= 20
	}
	if external.HasDataExfiltration && !report.HasDataExfiltration {
		report.HasDataExfiltration = true
		report.Score -= 15
	}

	report.Permissions = append(report.Permissions, external.Permissions...)
	report.Risks = append(report.Risks, external.Findings...)
	report.Evidence = append(report.Evidence, external.Evidence...)
	if strings.TrimSpace(external.ScannerVersion) != "" {
		if strings.TrimSpace(report.ScannerVersion) == "" || report.ScannerVersion == external.ScannerVersion {
			report.ScannerVersion = external.ScannerVersion
		} else if !strings.Contains(report.ScannerVersion, external.ScannerVersion) {
			report.ScannerVersion = report.ScannerVersion + "+" + external.ScannerVersion
		}
	}

	if report.Score < 0 {
		report.Score = 0
	}
	sort.Strings(report.Permissions)
	sort.Strings(report.Vulnerabilities)
	report.Permissions = dedupeStrings(report.Permissions)
	report.Vulnerabilities = dedupeStrings(report.Vulnerabilities)
	report.RiskLevel = riskLevelForScore(report.Score, len(report.Secrets) > 0)
	report.RiskLevel = moreSevereRiskLevel(report.RiskLevel, external.RiskLevel)
	report.SecurityBadge = securityBadgeForReport(report)
	report.SecurityBadge = moreSevereBadge(report.SecurityBadge, external.SecurityBadge)
	return report
}

func normalizeInstallSurface(surface InstallSurface) InstallSurface {
	if surface.InstallType == "" {
		surface.InstallType = InstallTypeManualExternal
	}
	if surface.ArtifactKind == "" {
		surface.ArtifactKind = ArtifactKindUnknown
	}
	return surface
}

func securityBadgeForReport(report *SecurityReport) string {
	switch {
	case report == nil:
		return BadgeYellow
	case report.RiskLevel == RiskHigh || report.RiskLevel == RiskCritical:
		return BadgeRed
	case report.VulnerabilityStatus == VulnerabilityStatusDetected:
		return BadgeRed
	case len(report.Secrets) > 0:
		return BadgeRed
	case report.HasShellInjection && report.Score < 80:
		return BadgeRed
	case report.RiskLevel == RiskMedium:
		return BadgeYellow
	case report.VulnerabilityStatus == VulnerabilityStatusSuspected:
		return BadgeYellow
	case report.HasPromptInjection || report.HasDataExfiltration || report.HasBinary:
		return BadgeYellow
	case report.InstallSurface.ArtifactKind == ArtifactKindClosedBinary || report.InstallSurface.ArtifactKind == ArtifactKindMixed:
		return BadgeYellow
	case report.RiskLevel == RiskLow &&
		(report.VulnerabilityStatus == VulnerabilityStatusNone ||
			report.VulnerabilityStatus == VulnerabilityStatusNotApplicable ||
			report.VulnerabilityStatus == VulnerabilityStatusUnknown) &&
		!report.HasPromptInjection &&
		!report.HasShellInjection &&
		!report.HasDataExfiltration &&
		!report.HasBinary &&
		(report.InstallSurface.ArtifactKind == ArtifactKindOpenSource ||
			report.InstallSurface.ArtifactKind == ArtifactKindUnknown):
		return BadgeGreen
	default:
		return BadgeYellow
	}
}

func riskLevelForScore(score int, hasSecrets bool) string {
	switch {
	case hasSecrets || score < 40:
		return RiskCritical
	case score < 60:
		return RiskHigh
	case score < 80:
		return RiskMedium
	default:
		return RiskLow
	}
}

func hasRiskyPermissionCombo(values map[string]struct{}, a, b string) bool {
	_, aOK := values[a]
	_, bOK := values[b]
	return aOK && bOK
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mergeVulnerabilityStatus(local, external string) string {
	local = normalizeVulnerabilityStatus(local)
	external = normalizeVulnerabilityStatus(external)
	switch {
	case local == VulnerabilityStatusDetected || external == VulnerabilityStatusDetected:
		return VulnerabilityStatusDetected
	case local == VulnerabilityStatusSuspected || external == VulnerabilityStatusSuspected:
		return VulnerabilityStatusSuspected
	case external == VulnerabilityStatusNone && (local == "" || local == VulnerabilityStatusUnknown || local == VulnerabilityStatusNotApplicable):
		return VulnerabilityStatusNone
	case local == VulnerabilityStatusNone && (external == "" || external == VulnerabilityStatusUnknown || external == VulnerabilityStatusNotApplicable):
		return VulnerabilityStatusNone
	case external == VulnerabilityStatusNotApplicable && local == "":
		return VulnerabilityStatusNotApplicable
	case local == VulnerabilityStatusNotApplicable && external == "":
		return VulnerabilityStatusNotApplicable
	case local == VulnerabilityStatusUnknown || external == VulnerabilityStatusUnknown:
		if local == VulnerabilityStatusNone || external == VulnerabilityStatusNone {
			return VulnerabilityStatusNone
		}
		return VulnerabilityStatusUnknown
	case external != "":
		return external
	default:
		return local
	}
}

func normalizeVulnerabilityStatus(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "none", "clean", "pass", "passed", "ok":
		return VulnerabilityStatusNone
	case "unknown", "pending":
		return VulnerabilityStatusUnknown
	case "suspected", "warn", "warning":
		return VulnerabilityStatusSuspected
	case "detected", "found", "fail", "failed", "critical":
		return VulnerabilityStatusDetected
	case "na", "n/a", "not_applicable", "not-applicable":
		return VulnerabilityStatusNotApplicable
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func normalizeInstallType(value string) string {
	switch normalizeSkillID(value) {
	case InstallTypeBuiltinCommands:
		return InstallTypeBuiltinCommands
	case InstallTypeRawSkill:
		return InstallTypeRawSkill
	case InstallTypeGitRepo:
		return InstallTypeGitRepo
	case InstallTypeSourceArchive:
		return InstallTypeSourceArchive
	case InstallTypeScriptPackage:
		return InstallTypeScriptPackage
	case InstallTypeBinaryPackage:
		return InstallTypeBinaryPackage
	case InstallTypeManualExternal:
		return InstallTypeManualExternal
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func normalizeArtifactKind(value string) string {
	switch normalizeSkillID(value) {
	case ArtifactKindOpenSource:
		return ArtifactKindOpenSource
	case ArtifactKindClosedBinary:
		return ArtifactKindClosedBinary
	case ArtifactKindMixed:
		return ArtifactKindMixed
	case ArtifactKindUnknown:
		return ArtifactKindUnknown
	default:
		return strings.TrimSpace(strings.ToLower(value))
	}
}

func moreSevereRiskLevel(current, candidate string) string {
	if riskLevelRank(candidate) > riskLevelRank(current) {
		return candidate
	}
	return current
}

func riskLevelRank(level string) int {
	switch strings.TrimSpace(strings.ToLower(level)) {
	case RiskCritical:
		return 4
	case RiskHigh:
		return 3
	case RiskMedium:
		return 2
	case RiskLow:
		return 1
	default:
		return 0
	}
}

func moreSevereBadge(current, candidate string) string {
	if badgeRank(candidate) > badgeRank(current) {
		return candidate
	}
	return current
}

func badgeRank(value string) int {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case BadgeRed:
		return 3
	case BadgeYellow:
		return 2
	case BadgeGreen:
		return 1
	default:
		return 0
	}
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

type LLMClassifier struct {
	registry *llm.ProviderRegistry
	provider string
	model    string
}

func NewLLMClassifier(registry *llm.ProviderRegistry, provider, model string) *LLMClassifier {
	if registry == nil || strings.TrimSpace(provider) == "" || strings.TrimSpace(model) == "" {
		return nil
	}
	return &LLMClassifier{registry: registry, provider: provider, model: model}
}

func (c *LLMClassifier) Classify(ctx context.Context, content string) (string, string, error) {
	if c == nil || c.registry == nil {
		return "", "skipped", nil
	}
	provider := c.registry.Get(c.provider)
	if provider == nil {
		return "", "skipped", nil
	}
	resp, err := provider.Chat(ctx, llm.ChatRequest{
		Model: c.model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Classify AI agent skill security risks and return strict JSON with keys risk_summary, suspicious, confidence."},
			{Role: llm.RoleUser, Content: content},
		},
		Temperature: 0,
		MaxTokens:   300,
	})
	if err != nil {
		return "", "error", err
	}
	raw := strings.TrimSpace(resp.Message.Content)
	if !json.Valid([]byte(raw)) {
		raw = fmt.Sprintf(`{"raw":%q}`, raw)
	}
	return raw, "completed", nil
}
