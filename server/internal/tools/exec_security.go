package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ExecSecurity defines the security mode for command execution.
type ExecSecurity string

const (
	ExecSecurityDeny      ExecSecurity = "deny"
	ExecSecurityAllowlist ExecSecurity = "allowlist"
	ExecSecurityFull      ExecSecurity = "full"
)

// AllowlistEntry represents a single allowlist pattern entry.
type AllowlistEntry struct {
	ID               string    `json:"id"`
	Pattern          string    `json:"pattern"`
	LastUsedAt       time.Time `json:"last_used_at,omitempty"`
	LastUsedCommand  string    `json:"last_used_command,omitempty"`
	LastResolvedPath string    `json:"last_resolved_path,omitempty"`
}

// ExecSecurityDefaults holds default security settings.
type ExecSecurityDefaults struct {
	Security ExecSecurity `json:"security,omitempty"`
}

// AllowlistConfig is the persistent allowlist configuration.
type AllowlistConfig struct {
	Version  int                  `json:"version"`
	Defaults ExecSecurityDefaults `json:"defaults,omitempty"`
	Entries  []AllowlistEntry     `json:"entries,omitempty"`
}

// DefaultSafeBins is the set of binaries that are always allowed in allowlist mode.
var DefaultSafeBins = []string{
	"jq", "grep", "cut", "sort", "uniq", "head", "tail", "tr", "wc",
	"cat", "ls", "pwd", "echo", "date", "which", "env", "whoami",
	"dirname", "basename", "realpath", "readlink", "stat", "file",
	"true", "false", "test", "printf", "seq", "tee", "xargs",
}

// CommandSegment represents a parsed segment of a shell command.
type CommandSegment struct {
	Raw            string
	Argv           []string
	ExecutableName string
	ResolvedPath   string
}

// CommandAnalysis is the result of parsing a shell command.
type CommandAnalysis struct {
	OK       bool
	Reason   string
	Segments []CommandSegment
}

// LoadAllowlist reads the allowlist config from a JSON file.
func LoadAllowlist(path string) (*AllowlistConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AllowlistConfig{Version: 1}, nil
		}
		return nil, fmt.Errorf("read allowlist: %w", err)
	}
	var cfg AllowlistConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &AllowlistConfig{Version: 1}, nil
	}
	if cfg.Version != 1 {
		return &AllowlistConfig{Version: 1}, nil
	}
	// Ensure all entries have IDs.
	for i := range cfg.Entries {
		if cfg.Entries[i].ID == "" {
			cfg.Entries[i].ID = uuid.New().String()
		}
	}
	return &cfg, nil
}

// SaveAllowlist writes the allowlist config to a JSON file.
func SaveAllowlist(path string, cfg *AllowlistConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create allowlist dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal allowlist: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write allowlist: %w", err)
	}
	return nil
}

// AnalyzeCommand parses a shell command string into segments and resolves
// executable paths. It rejects commands with dangerous shell tokens.
func AnalyzeCommand(command, cwd string) *CommandAnalysis {
	command = strings.TrimSpace(command)
	if command == "" {
		return &CommandAnalysis{OK: false, Reason: "empty command"}
	}

	// Split by chain operators (&&, ||, ;) then by pipe (|).
	chains := splitChainOperators(command)
	if chains == nil {
		return &CommandAnalysis{OK: false, Reason: "malformed command chain"}
	}

	var segments []CommandSegment
	for _, chain := range chains {
		parts := splitPipeline(chain)
		if parts == nil {
			return &CommandAnalysis{OK: false, Reason: "malformed pipeline"}
		}
		for _, part := range parts {
			seg := parseSegment(part, cwd)
			if seg == nil {
				return &CommandAnalysis{OK: false, Reason: fmt.Sprintf("unable to parse segment: %s", part)}
			}
			segments = append(segments, *seg)
		}
	}

	return &CommandAnalysis{OK: true, Segments: segments}
}

// EvaluateAllowlist checks whether all segments of a command analysis are
// covered by the allowlist or safe bins.
func EvaluateAllowlist(analysis *CommandAnalysis, entries []AllowlistEntry, safeBins map[string]struct{}) (satisfied bool, matches []AllowlistEntry) {
	if !analysis.OK || len(analysis.Segments) == 0 {
		return false, nil
	}

	for _, seg := range analysis.Segments {
		// Check safe bins first.
		if IsSafeBin(seg.ExecutableName, safeBins) {
			continue
		}
		// Check allowlist.
		match := MatchAllowlist(entries, seg.ResolvedPath)
		if match != nil {
			matches = append(matches, *match)
			continue
		}
		return false, matches
	}
	return true, matches
}

// MatchAllowlist checks if a resolved path matches any allowlist entry.
func MatchAllowlist(entries []AllowlistEntry, resolvedPath string) *AllowlistEntry {
	if resolvedPath == "" {
		return nil
	}
	for i := range entries {
		pattern := strings.TrimSpace(entries[i].Pattern)
		if pattern == "" {
			continue
		}
		if matchGlob(pattern, resolvedPath) {
			return &entries[i]
		}
	}
	return nil
}

// IsSafeBin checks if an executable name is in the safe bins set.
func IsSafeBin(execName string, safeBins map[string]struct{}) bool {
	if len(safeBins) == 0 || execName == "" {
		return false
	}
	name := strings.ToLower(execName)
	if runtime.GOOS == "windows" {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	_, ok := safeBins[name]
	return ok
}

// BuildSafeBinsSet creates a set from a slice of safe bin names.
func BuildSafeBinsSet(bins []string) map[string]struct{} {
	m := make(map[string]struct{}, len(bins))
	for _, b := range bins {
		name := strings.TrimSpace(strings.ToLower(b))
		if name != "" {
			m[name] = struct{}{}
		}
	}
	return m
}

// RecordAllowlistUse updates the last-used metadata for a matched entry.
func RecordAllowlistUse(cfg *AllowlistConfig, entry *AllowlistEntry, command, resolvedPath string) {
	for i := range cfg.Entries {
		if cfg.Entries[i].Pattern == entry.Pattern {
			cfg.Entries[i].LastUsedAt = time.Now()
			cfg.Entries[i].LastUsedCommand = command
			cfg.Entries[i].LastResolvedPath = resolvedPath
			return
		}
	}
}

// AddAllowlistEntry adds a new pattern to the allowlist if not already present.
func AddAllowlistEntry(cfg *AllowlistConfig, pattern string) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return
	}
	for _, e := range cfg.Entries {
		if e.Pattern == pattern {
			return
		}
	}
	cfg.Entries = append(cfg.Entries, AllowlistEntry{
		ID:         uuid.New().String(),
		Pattern:    pattern,
		LastUsedAt: time.Now(),
	})
}

// --- internal helpers ---

func splitChainOperators(command string) []string {
	var parts []string
	var buf strings.Builder
	inSingle, inDouble, escaped := false, false, false

	flush := func() bool {
		s := strings.TrimSpace(buf.String())
		buf.Reset()
		if s == "" {
			return false
		}
		parts = append(parts, s)
		return true
	}

	runes := []rune(command)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if escaped {
			buf.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			buf.WriteRune(ch)
			continue
		}
		if inSingle {
			buf.WriteRune(ch)
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			buf.WriteRune(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			buf.WriteRune(ch)
			continue
		}
		if ch == '"' {
			inDouble = true
			buf.WriteRune(ch)
			continue
		}
		// Chain operators.
		if ch == '&' && i+1 < len(runes) && runes[i+1] == '&' {
			flush()
			i++ // skip second &
			continue
		}
		if ch == '|' && i+1 < len(runes) && runes[i+1] == '|' {
			flush()
			i++ // skip second |
			continue
		}
		if ch == ';' {
			flush()
			continue
		}
		buf.WriteRune(ch)
	}

	if escaped || inSingle || inDouble {
		return nil
	}
	flush()
	if len(parts) == 0 {
		return nil
	}
	return parts
}

func splitPipeline(command string) []string {
	var parts []string
	var buf strings.Builder
	inSingle, inDouble, escaped := false, false, false

	flush := func() {
		s := strings.TrimSpace(buf.String())
		buf.Reset()
		if s != "" {
			parts = append(parts, s)
		}
	}

	for _, ch := range command {
		if escaped {
			buf.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			buf.WriteRune(ch)
			continue
		}
		if inSingle {
			buf.WriteRune(ch)
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			buf.WriteRune(ch)
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			buf.WriteRune(ch)
			continue
		}
		if ch == '"' {
			inDouble = true
			buf.WriteRune(ch)
			continue
		}
		// Reject dangerous tokens.
		if ch == '>' || ch == '<' || ch == '`' || ch == '(' || ch == ')' {
			return nil
		}
		if ch == '|' {
			flush()
			continue
		}
		buf.WriteRune(ch)
	}

	if escaped || inSingle || inDouble {
		return nil
	}
	flush()
	return parts
}

func parseSegment(raw, cwd string) *CommandSegment {
	argv := tokenizeShell(raw)
	if len(argv) == 0 {
		return nil
	}
	execName := filepath.Base(argv[0])
	resolved := resolveExecPath(argv[0], cwd)
	if resolved != "" {
		execName = filepath.Base(resolved)
	}
	return &CommandSegment{
		Raw:            raw,
		Argv:           argv,
		ExecutableName: execName,
		ResolvedPath:   resolved,
	}
}

func tokenizeShell(s string) []string {
	var tokens []string
	var buf strings.Builder
	inSingle, inDouble, escaped := false, false, false

	flush := func() {
		if buf.Len() > 0 {
			tokens = append(tokens, buf.String())
			buf.Reset()
		}
	}

	for _, ch := range s {
		if escaped {
			buf.WriteRune(ch)
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		if inSingle {
			if ch == '\'' {
				inSingle = false
			} else {
				buf.WriteRune(ch)
			}
			continue
		}
		if inDouble {
			if ch == '"' {
				inDouble = false
			} else {
				buf.WriteRune(ch)
			}
			continue
		}
		if ch == '\'' {
			inSingle = true
			continue
		}
		if ch == '"' {
			inDouble = true
			continue
		}
		if ch == ' ' || ch == '\t' {
			flush()
			continue
		}
		buf.WriteRune(ch)
	}
	flush()
	return tokens
}

func resolveExecPath(name, cwd string) string {
	// Expand ~ prefix.
	if strings.HasPrefix(name, "~/") {
		home, _ := os.UserHomeDir()
		if home != "" {
			name = filepath.Join(home, name[2:])
		}
	}

	// Absolute or relative path.
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		if filepath.IsAbs(name) {
			if isExecutable(name) {
				return name
			}
			return ""
		}
		base := cwd
		if base == "" {
			base, _ = os.Getwd()
		}
		candidate := filepath.Join(base, name)
		if isExecutable(candidate) {
			return candidate
		}
		return ""
	}

	// Search PATH.
	resolved, err := exec.LookPath(name)
	if err == nil {
		return resolved
	}
	return ""
}

func matchGlob(pattern, target string) bool {
	// Normalize for case-insensitive matching.
	pattern = strings.ToLower(pattern)
	target = strings.ToLower(target)

	// Expand ~ in pattern.
	if strings.HasPrefix(pattern, "~/") {
		home, _ := os.UserHomeDir()
		if home != "" {
			pattern = filepath.Join(home, pattern[2:])
		}
	}

	matched, err := filepath.Match(pattern, target)
	if err != nil {
		return false
	}
	return matched
}
