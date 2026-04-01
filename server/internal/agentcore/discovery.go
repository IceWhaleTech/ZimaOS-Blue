package agentcore

import (
	"strings"
)

// CanonicalSkillID is the stable identifier for a discoverable capability.
type CanonicalSkillID string

const (
	CanonicalWebQuery     CanonicalSkillID = "web_query"
	CanonicalBrowser      CanonicalSkillID = "browser"
	CanonicalAnalyze      CanonicalSkillID = "analyze"
	CanonicalDeepResearch CanonicalSkillID = "deep_research"
	CanonicalExec         CanonicalSkillID = "exec"
	CanonicalUnknown      CanonicalSkillID = "unknown"
)

// ExecutionProfile determines how a discovered skill should be executed.
type ExecutionProfile string

const (
	ExecutionProfileInline      ExecutionProfile = "inline"
	ExecutionProfilePreferFork  ExecutionProfile = "prefer_fork"
	ExecutionProfileRequireFork ExecutionProfile = "require_fork"
)

// NativeSurfaceMode determines the tool surface exposed to the LLM.
type NativeSurfaceMode string

const (
	NativeSurfaceModeLegacy      NativeSurfaceMode = "legacy"
	NativeSurfaceModeClarifyNone NativeSurfaceMode = "clarify_none"
	NativeSurfaceModeSkillExec   NativeSurfaceMode = "skill_exec"
)

// CapabilityDiscoveryEntry describes a discoverable capability.
type CapabilityDiscoveryEntry struct {
	CanonicalID       CanonicalSkillID  `json:"canonical_id"`
	Kind              string            `json:"kind"`
	Aliases           []string          `json:"aliases,omitempty"`
	SearchHints       []string          `json:"search_hints,omitempty"`
	CapabilityTags    []string          `json:"capability_tags,omitempty"`
	ExecutionProfile  ExecutionProfile  `json:"execution_profile"`
	NativeSurfaceMode NativeSurfaceMode `json:"native_surface_mode"`
	CutoverEligible   bool              `json:"cutover_eligible"`
	Description       string            `json:"description,omitempty"`
}

// CapabilityDiscoveryDecision is the internal discover-first decision.
type CapabilityDiscoveryDecision struct {
	CanonicalTarget   CanonicalSkillID  `json:"canonical_target"`
	AliasResolved     string            `json:"alias_resolved,omitempty"`
	NeedClarify       bool              `json:"need_clarify"`
	ClarifyReason     string            `json:"clarify_reason,omitempty"`
	ExecutionProfile  ExecutionProfile  `json:"execution_profile"`
	NativeSurfaceMode NativeSurfaceMode `json:"native_surface_mode"`
	FallbackReason    string            `json:"fallback_reason,omitempty"`
	OriginalDecision  *Decision         `json:"original_decision,omitempty"`
}

// SkillCutoverObservation captures observability for gate alignment.
type SkillCutoverObservation struct {
	SelectedCanonicalSkill string            `json:"selected_canonical_skill"`
	SelectedAlias          string            `json:"selected_alias,omitempty"`
	NativeSurfaceMode      NativeSurfaceMode `json:"native_surface_mode"`
	ExecutionProfile       ExecutionProfile  `json:"execution_profile"`
	SkillExecCutover       bool              `json:"skill_exec_cutover"`
	ClarifyOutcome         string            `json:"clarify_outcome,omitempty"`
	ForkedSkillExecution   bool              `json:"forked_skill_execution"`
	FallbackReason         string            `json:"fallback_reason,omitempty"`
}

var canonicalRegistry map[CanonicalSkillID]CapabilityDiscoveryEntry
var aliasToCanonical map[string]CanonicalSkillID

func init() {
	canonicalRegistry = map[CanonicalSkillID]CapabilityDiscoveryEntry{
		CanonicalWebQuery: {
			CanonicalID:       CanonicalWebQuery,
			Kind:              "skill",
			Aliases:           []string{"web_search", "search", "web_fetch", "web_read", "web_extract", "web_crawl"},
			SearchHints:       []string{"search", "web", "docs", "latest", "news", "find", "lookup"},
			CapabilityTags:    []string{"search", "web", "public"},
			ExecutionProfile:  ExecutionProfilePreferFork,
			NativeSurfaceMode: NativeSurfaceModeSkillExec,
			CutoverEligible:   true,
			Description:       "Unified public web discovery and reading",
		},
		CanonicalBrowser: {
			CanonicalID:       CanonicalBrowser,
			Kind:              "tool",
			Aliases:           []string{},
			SearchHints:       []string{"browse", "open", "click", "login", "screenshot", "interaction"},
			CapabilityTags:    []string{"browser", "interaction", "ui"},
			ExecutionProfile:  ExecutionProfileInline,
			NativeSurfaceMode: NativeSurfaceModeSkillExec,
			CutoverEligible:   true,
			Description:       "Open and interact with web pages",
		},
		CanonicalAnalyze: {
			CanonicalID:       CanonicalAnalyze,
			Kind:              "skill",
			Aliases:           []string{},
			SearchHints:       []string{"analyze", "report", "synthesize", "summary", "aggregate"},
			CapabilityTags:    []string{"analysis", "synthesis", "report"},
			ExecutionProfile:  ExecutionProfilePreferFork,
			NativeSurfaceMode: NativeSurfaceModeSkillExec,
			CutoverEligible:   true,
			Description:       "Synthesized analysis on topics",
		},
		CanonicalDeepResearch: {
			CanonicalID:       CanonicalDeepResearch,
			Kind:              "skill",
			Aliases:           []string{"research", "deep_research"},
			SearchHints:       []string{"research", "citations", "evidence", "timeline", "compare"},
			CapabilityTags:    []string{"research", "citations", "evidence"},
			ExecutionProfile:  ExecutionProfileRequireFork,
			NativeSurfaceMode: NativeSurfaceModeSkillExec,
			CutoverEligible:   true,
			Description:       "Multi-step cited research",
		},
		CanonicalExec: {
			CanonicalID:       CanonicalExec,
			Kind:              "tool",
			ExecutionProfile:  ExecutionProfileInline,
			NativeSurfaceMode: NativeSurfaceModeSkillExec,
			CutoverEligible:   false,
			Description:       "Execute skill and shell commands",
		},
	}
	aliasToCanonical = buildAliasToCanonical()
}

func buildAliasToCanonical() map[string]CanonicalSkillID {
	m := make(map[string]CanonicalSkillID)
	for canonical, entry := range canonicalRegistry {
		m[string(canonical)] = canonical
		for _, alias := range entry.Aliases {
			m[normalizeAlias(alias)] = canonical
		}
	}
	return m
}

func normalizeAlias(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// ResolveCanonicalSkill resolves a raw skill name/alias to its canonical ID.
func ResolveCanonicalSkill(raw string) (CanonicalSkillID, bool) {
	if c, ok := aliasToCanonical[normalizeAlias(raw)]; ok {
		return c, true
	}
	return CanonicalUnknown, false
}

// GetDiscoveryEntry returns the discovery entry for a canonical skill.
func GetDiscoveryEntry(id CanonicalSkillID) (CapabilityDiscoveryEntry, bool) {
	entry, ok := canonicalRegistry[id]
	return entry, ok
}

// IsCutoverEligibleCanonical checks if a canonical skill is eligible for cutover.
func IsCutoverEligibleCanonical(id CanonicalSkillID) bool {
	entry, ok := canonicalRegistry[id]
	return ok && entry.CutoverEligible
}

// ExecutionProfileForSkill returns the execution profile for a canonical skill.
func ExecutionProfileForSkill(id CanonicalSkillID) ExecutionProfile {
	if entry, ok := canonicalRegistry[id]; ok {
		return entry.ExecutionProfile
	}
	return ExecutionProfileInline
}

// NativeSurfaceModeForSkill returns the native surface mode for a canonical skill.
func NativeSurfaceModeForSkill(id CanonicalSkillID) NativeSurfaceMode {
	if entry, ok := canonicalRegistry[id]; ok {
		return entry.NativeSurfaceMode
	}
	return NativeSurfaceModeLegacy
}

// BuildDiscoveryDecision creates a CapabilityDiscoveryDecision from an agentcore Decision.
func BuildDiscoveryDecision(d Decision, skillDynamicExposure bool) CapabilityDiscoveryDecision {
	selectedSkill := strings.TrimSpace(d.SelectedSkill)
	canonical, ok := ResolveCanonicalSkill(d.SelectedSkill)
	if !ok {
		canonical = CanonicalUnknown
	}

	nativeMode := NativeSurfaceModeLegacy
	execProfile := ExecutionProfileInline

	if d.NeedClarify {
		nativeMode = NativeSurfaceModeClarifyNone
	} else if selectedSkill != "" {
		if entry, ok := GetDiscoveryEntry(canonical); ok {
			execProfile = entry.ExecutionProfile
		}
		switch {
		case skillDynamicExposure && IsCutoverEligibleCanonical(canonical):
			nativeMode = NativeSurfaceModeForSkill(canonical)
		case !skillDynamicExposure:
			// Preserve the legacy selector behavior until discover-first cutover is enabled.
			nativeMode = NativeSurfaceModeSkillExec
		}
	}

	return CapabilityDiscoveryDecision{
		CanonicalTarget:   canonical,
		AliasResolved:     d.SelectedSkill,
		NeedClarify:       d.NeedClarify,
		ClarifyReason:     d.Reason,
		ExecutionProfile:  execProfile,
		NativeSurfaceMode: nativeMode,
		FallbackReason:    d.Reason,
		OriginalDecision:  &d,
	}
}

// ToObservation converts a discovery decision to an observation for gate/logging.
func (d CapabilityDiscoveryDecision) ToObservation() SkillCutoverObservation {
	return SkillCutoverObservation{
		SelectedCanonicalSkill: string(d.CanonicalTarget),
		SelectedAlias:          d.AliasResolved,
		NativeSurfaceMode:      d.NativeSurfaceMode,
		ExecutionProfile:       d.ExecutionProfile,
		SkillExecCutover:       d.NativeSurfaceMode == NativeSurfaceModeSkillExec,
		ClarifyOutcome:         d.ClarifyReason,
		ForkedSkillExecution:   d.ExecutionProfile == ExecutionProfilePreferFork || d.ExecutionProfile == ExecutionProfileRequireFork,
		FallbackReason:         d.FallbackReason,
	}
}

// IsForkedExecution returns true if the skill should run in isolated worker.
func (d CapabilityDiscoveryDecision) IsForkedExecution() bool {
	return d.ExecutionProfile == ExecutionProfilePreferFork || d.ExecutionProfile == ExecutionProfileRequireFork
}
