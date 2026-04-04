package tools

import "time"

// PolicyMode determines the enforcement level for exec commands.
type PolicyMode string

const (
	// PolicyAudit logs all commands but does not block any (except Critical risk).
	PolicyAudit PolicyMode = "audit"
	// PolicyRestricted blocks high-risk commands, allows medium/low.
	PolicyRestricted PolicyMode = "restricted"
	// PolicyWorkspace only allows commands scoped to the project directory.
	PolicyWorkspace PolicyMode = "workspace"
	// PolicyFull allows all commands but still blocks Critical risk.
	PolicyFull PolicyMode = "full"
)

// ExecPolicy defines the runtime policy for exec commands.
type ExecPolicy struct {
	Mode             PolicyMode    `json:"mode"`
	MaxRiskThreshold int           `json:"max_risk_threshold"` // commands above this score are blocked
	MaxRetries       int           `json:"max_retries"`        // max times the same command can be retried
	RetryWindow      time.Duration `json:"retry_window"`       // window for counting retries
	MaxCommandLen    int           `json:"max_command_len"`    // max command string length (0 = unlimited)
}

// DefaultExecPolicy returns sensible defaults.
func DefaultExecPolicy() ExecPolicy {
	return ExecPolicy{
		Mode:             PolicyFull,
		MaxRiskThreshold: 80, // block Critical (90+), allow High (60-89) in full mode
		MaxRetries:       3,
		RetryWindow:      5 * time.Minute,
		MaxCommandLen:    20_000,
	}
}

// PolicyForMode returns a policy with thresholds appropriate for the given mode.
func PolicyForMode(mode PolicyMode) ExecPolicy {
	p := DefaultExecPolicy()
	p.Mode = mode
	switch mode {
	case PolicyAudit:
		p.MaxRiskThreshold = 100 // only block truly critical (score > 100 = impossible)
	case PolicyRestricted:
		p.MaxRiskThreshold = 50 // block High and Critical
	case PolicyWorkspace:
		p.MaxRiskThreshold = 60 // block High and Critical
	case PolicyFull:
		p.MaxRiskThreshold = 80 // block only Critical
	}
	return p
}
