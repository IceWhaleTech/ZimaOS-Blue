package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

const (
	defaultEventLimit = 200
)

type Defaults struct {
	ArtifactRoot      string
	StorePath         string
	ApprovalMode      ApprovalMode
	SandboxMode       string
	MaxDuration       time.Duration
	MaxSteps          int
	MaxToolRounds     int
	MaxSubagents      int
	MaxDepth          int
	ProtectedPrefixes []string
}

type PolicyResolver struct {
	defaults Defaults
	agents   *config.AgentsConfig
}

func NewPolicyResolver(cfg config.HarnessConfig, agents *config.AgentsConfig) *PolicyResolver {
	return &PolicyResolver{
		defaults: Defaults{
			ArtifactRoot:  strings.TrimSpace(cfg.ArtifactRoot),
			StorePath:     strings.TrimSpace(cfg.StorePath),
			ApprovalMode:  ApprovalMode(strings.TrimSpace(cfg.DefaultApprovalMode)),
			SandboxMode:   strings.TrimSpace(cfg.DefaultSandboxMode),
			MaxDuration:   cfg.DefaultMaxDuration,
			MaxSteps:      cfg.DefaultMaxSteps,
			MaxToolRounds: cfg.DefaultMaxToolRounds,
			MaxSubagents:  3,
			MaxDepth:      2,
		},
		agents: agents,
	}
}

func (r *PolicyResolver) Resolve(spec RunSpec) RunSpec {
	out := spec
	if strings.TrimSpace(out.WorkspaceRoot) == "" {
		out.WorkspaceRoot = r.defaultWorkspaceRoot()
	}
	if out.ApprovalMode == "" {
		out.ApprovalMode = r.defaults.ApprovalMode
	}
	if strings.TrimSpace(out.SandboxMode) == "" {
		out.SandboxMode = r.defaults.SandboxMode
	}
	if out.MaxDuration <= 0 {
		out.MaxDuration = r.defaults.MaxDuration
	}
	if out.MaxSteps <= 0 {
		out.MaxSteps = r.defaults.MaxSteps
	}
	if out.MaxToolRounds <= 0 {
		out.MaxToolRounds = r.defaults.MaxToolRounds
	}
	if out.MaxSubagents <= 0 {
		out.MaxSubagents = r.defaults.MaxSubagents
	}
	if out.MaxDepth <= 0 {
		out.MaxDepth = r.defaults.MaxDepth
	}
	if r.agents == nil {
		return out
	}
	agentCfg, ok := resolveAgentConfig(r.agents, out.AgentID)
	if !ok {
		return out
	}
	if strings.TrimSpace(out.Model) == "" {
		out.Model = agentCfg.Model
	}
	if strings.TrimSpace(out.SandboxMode) == "" || out.SandboxMode == r.defaults.SandboxMode {
		if mode := strings.TrimSpace(agentCfg.Sandbox.Mode); mode != "" {
			out.SandboxMode = mode
		}
	}
	if out.MaxSubagents <= 0 || out.MaxSubagents == r.defaults.MaxSubagents {
		if agentCfg.Subagents.MaxParallel > 0 {
			out.MaxSubagents = agentCfg.Subagents.MaxParallel
		}
	}
	if out.MaxDepth <= 0 || out.MaxDepth == r.defaults.MaxDepth {
		if agentCfg.Subagents.MaxDepth > 0 {
			out.MaxDepth = agentCfg.Subagents.MaxDepth
		}
	}
	if out.MaxDuration <= 0 || out.MaxDuration == r.defaults.MaxDuration {
		if agentCfg.Subagents.Timeout > 0 {
			out.MaxDuration = agentCfg.Subagents.Timeout
		}
	}
	return out
}

func (r *PolicyResolver) ResolveChild(parent *Run, spec RunSpec) (RunSpec, error) {
	if parent == nil {
		return RunSpec{}, fmt.Errorf("parent run is required")
	}
	spec.ParentRunID = parent.ID
	if strings.TrimSpace(spec.UserID) == "" {
		spec.UserID = parent.UserID
	}
	if strings.TrimSpace(spec.ConversationID) == "" {
		spec.ConversationID = parent.ConversationID
	}
	if strings.TrimSpace(spec.SessionID) == "" {
		spec.SessionID = parent.SessionID
	}
	if strings.TrimSpace(spec.AgentID) == "" {
		spec.AgentID = parent.AgentID
	}
	if strings.TrimSpace(spec.Model) == "" {
		spec.Model = parent.Model
	}
	if strings.TrimSpace(spec.WorkspaceRoot) == "" {
		spec.WorkspaceRoot = parent.WorkspaceRoot
	}
	if spec.ApprovalMode == "" {
		spec.ApprovalMode = parent.ApprovalMode
	}
	if strings.TrimSpace(spec.SandboxMode) == "" {
		spec.SandboxMode = parent.SandboxMode
	}
	spec = r.Resolve(spec)

	if parent.MaxDepth > 0 && parent.Depth+1 > parent.MaxDepth {
		return RunSpec{}, fmt.Errorf("max depth exceeded")
	}
	if parent.MaxDepth > 0 && (spec.MaxDepth <= 0 || spec.MaxDepth > parent.MaxDepth) {
		spec.MaxDepth = parent.MaxDepth
	}
	if parent.MaxSubagents > 0 && (spec.MaxSubagents <= 0 || spec.MaxSubagents > parent.MaxSubagents) {
		spec.MaxSubagents = parent.MaxSubagents
	}
	if parent.MaxSteps > 0 && (spec.MaxSteps <= 0 || spec.MaxSteps > parent.MaxSteps) {
		spec.MaxSteps = parent.MaxSteps
	}
	if parent.MaxToolRounds > 0 && (spec.MaxToolRounds <= 0 || spec.MaxToolRounds > parent.MaxToolRounds) {
		spec.MaxToolRounds = parent.MaxToolRounds
	}
	if parent.MaxDuration > 0 && (spec.MaxDuration <= 0 || spec.MaxDuration > parent.MaxDuration) {
		spec.MaxDuration = parent.MaxDuration
	}
	if strings.TrimSpace(parent.SandboxMode) != "" && widensSandbox(strings.TrimSpace(parent.SandboxMode), strings.TrimSpace(spec.SandboxMode)) {
		return RunSpec{}, fmt.Errorf("child run cannot widen sandbox scope")
	}
	if parent.ApprovalMode != "" && widensApproval(parent.ApprovalMode, spec.ApprovalMode) {
		return RunSpec{}, fmt.Errorf("child run cannot widen approval mode")
	}
	return spec, nil
}

func (r *PolicyResolver) defaultWorkspaceRoot() string {
	if r == nil {
		return filepath.Join(".", "data", "workspace")
	}
	if storePath := strings.TrimSpace(r.defaults.StorePath); storePath != "" {
		return filepath.Join(filepath.Dir(storePath), "workspace")
	}
	if artifactRoot := strings.TrimSpace(r.defaults.ArtifactRoot); artifactRoot != "" {
		clean := filepath.Clean(artifactRoot)
		if strings.EqualFold(filepath.Base(clean), "artifacts") {
			parent := filepath.Dir(clean)
			if strings.EqualFold(filepath.Base(parent), "harness") {
				return filepath.Join(filepath.Dir(parent), "workspace")
			}
			return filepath.Join(parent, "workspace")
		}
		return filepath.Join(filepath.Dir(clean), "workspace")
	}
	return filepath.Join(".", "data", "workspace")
}

func (r *PolicyResolver) ArtifactRoot(runID string) string {
	root := strings.TrimSpace(r.defaults.ArtifactRoot)
	if root == "" {
		root = "./data/harness/artifacts"
	}
	return filepath.Join(root, strings.TrimSpace(runID))
}

func (r *PolicyResolver) IsProtectedPath(path string, artifactRoot string) bool {
	protected, _ := r.protectedPathReason(path, artifactRoot)
	return protected
}

func (r *PolicyResolver) protectedPathReason(path string, currentArtifactRoot string) (bool, string) {
	clean, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || clean == "" {
		return false, ""
	}
	for _, suffix := range []string{
		string(filepath.Separator) + ".git",
		string(filepath.Separator) + ".git" + string(filepath.Separator),
	} {
		if strings.Contains(clean, suffix) {
			return true, fmt.Sprintf("%q is inside .git/", clean)
		}
	}
	base := strings.ToLower(filepath.Base(clean))
	if strings.HasPrefix(base, ".env") {
		return true, fmt.Sprintf("%q matches protected .env*", clean)
	}
	if r.isBlueConfigPath(clean) {
		return true, fmt.Sprintf("%q is a protected Blue config file", clean)
	}
	if storePath := strings.TrimSpace(r.defaults.StorePath); storePath != "" {
		if absStore, err := filepath.Abs(storePath); err == nil && clean == absStore {
			return true, fmt.Sprintf("%q is the harness store path", clean)
		}
	}
	if artifactBase := strings.TrimSpace(r.defaults.ArtifactRoot); artifactBase != "" {
		if absBase, err := filepath.Abs(artifactBase); err == nil && pathWithin(absBase, clean) {
			if currentArtifactRoot != "" {
				if absCurrent, currentErr := filepath.Abs(currentArtifactRoot); currentErr == nil && pathWithin(absCurrent, clean) {
					return false, ""
				}
			}
			return true, fmt.Sprintf("%q is inside another run's harness artifact root", clean)
		}
	}
	return false, ""
}

func (r *PolicyResolver) isBlueConfigPath(clean string) bool {
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return false
	}
	candidates := []string{
		"config.yaml",
		"config.yml",
		filepath.Join("config", "config.yaml"),
		filepath.Join("config", "config.yml"),
		"/etc/zimaos-blue/config.yaml",
		"/etc/zimaos-blue/config.yml",
	}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		candidates = append(candidates,
			filepath.Join(cwd, "config.yaml"),
			filepath.Join(cwd, "config.yml"),
			filepath.Join(cwd, "config", "config.yaml"),
			filepath.Join(cwd, "config", "config.yml"),
		)
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		candidates = append(candidates,
			filepath.Join(home, ".zimaos-blue", "config.yaml"),
			filepath.Join(home, ".zimaos-blue", "config.yml"),
		)
	}
	for _, candidate := range candidates {
		absCandidate, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if clean == absCandidate {
			return true
		}
	}
	return false
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func resolveAgentConfig(cfg *config.AgentsConfig, agentID string) (config.AgentConfig, bool) {
	if cfg == nil {
		return config.AgentConfig{}, false
	}
	out := cfg.Defaults
	id := strings.TrimSpace(agentID)
	if id == "" {
		return out, true
	}
	for _, item := range cfg.List {
		if strings.EqualFold(strings.TrimSpace(item.ID), id) {
			if strings.TrimSpace(item.Model) != "" {
				out.Model = item.Model
			}
			if strings.TrimSpace(item.Sandbox.Mode) != "" {
				out.Sandbox = item.Sandbox
			}
			if item.Subagents.MaxParallel > 0 {
				out.Subagents.MaxParallel = item.Subagents.MaxParallel
			}
			if item.Subagents.MaxDepth > 0 {
				out.Subagents.MaxDepth = item.Subagents.MaxDepth
			}
			if item.Subagents.Timeout > 0 {
				out.Subagents.Timeout = item.Subagents.Timeout
			}
			return out, true
		}
	}
	return out, true
}

func widensSandbox(parent, child string) bool {
	order := map[string]int{
		"deny":      0,
		"inherit":   1,
		"session":   1,
		"process":   2,
		"workspace": 2,
		"full":      3,
	}
	return order[strings.ToLower(child)] > order[strings.ToLower(parent)]
}

func widensApproval(parent, child ApprovalMode) bool {
	order := map[ApprovalMode]int{
		ApprovalModeDeny:  0,
		ApprovalModeAsk:   1,
		ApprovalModeAllow: 2,
	}
	return order[child] > order[parent]
}
