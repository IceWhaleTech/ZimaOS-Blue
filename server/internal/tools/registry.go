// Package tools provides a framework for defining and executing tools/functions.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Common errors
var (
	ErrToolNotFound = errors.New("tool not found")
)

// ForwardedResult wraps a tool result that was auto-forwarded from exec.
// The ActualTool field indicates which tool actually executed the request,
// allowing the UI to display the correct tool name (e.g. "web_search" instead of "exec").
type ForwardedResult struct {
	ActualTool string
	Result     interface{}
}

// ToolDefinition describes a tool that can be called by an LLM.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Icon        string                 `json:"icon,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// Tool is the interface that all tools must implement.
type Tool interface {
	// Definition returns the tool's definition.
	Definition() ToolDefinition

	// Execute runs the tool with the given arguments.
	Execute(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// Registry manages registered tools.
type Registry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	disabled map[string]Tool // disabled tools (still registered, but hidden from Definitions/List)
	version  uint64          // incremented on every mutation (Register/Disable/Enable)
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]Tool),
		disabled: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := tool.Definition().Name
	delete(r.disabled, name)
	r.tools[name] = tool
	r.version++
}

// Version returns the current mutation version of the registry.
// It increments on every Register, Disable, or Enable call.
func (r *Registry) Version() uint64 {
	r.mu.RLock()
	v := r.version
	r.mu.RUnlock()
	return v
}

// Disable moves a tool from active to disabled. Disabled tools are hidden from
// Definitions() and List() (not sent to LLM) but still accessible via Get().
func (r *Registry) Disable(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	tool, ok := r.tools[name]
	if !ok {
		return false
	}
	delete(r.tools, name)
	r.disabled[name] = tool
	r.version++
	return true
}

// Enable moves a tool from disabled back to active.
func (r *Registry) Enable(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	tool, ok := r.disabled[name]
	if !ok {
		return false
	}
	delete(r.disabled, name)
	r.tools[name] = tool
	r.version++
	return true
}

// IsDisabled returns true if the tool exists but is disabled.
func (r *Registry) IsDisabled(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.disabled[name]
	return ok
}

// Get retrieves a tool by name. Returns both active and disabled tools.
func (r *Registry) Get(name string) Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if t := r.tools[name]; t != nil {
		return t
	}
	return r.disabled[name]
}

// List returns all active (enabled) tool names sorted alphabetically.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListDisabled returns all disabled tool names sorted alphabetically.
func (r *Registry) ListDisabled() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.disabled))
	for name := range r.disabled {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Definitions returns all tool definitions sorted by name.
func (r *Registry) Definitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		defs = append(defs, tool.Definition())
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

// Executor handles tool execution.
type Executor struct {
	registry *Registry
}

// NewExecutor creates a new tool executor.
func NewExecutor(registry *Registry) *Executor {
	return &Executor{
		registry: registry,
	}
}

// Execute runs a tool by name with the given arguments.
func (e *Executor) Execute(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	// Check context first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	tool := e.registry.Get(name)
	if tool == nil {
		// Skill fallback: if the LLM calls a tool that doesn't exist in the
		// tool registry, try forwarding to `blue <name> key=value` via exec.
		// This handles skills (analyze, web_search, etc.) that the LLM may
		// call as native tools despite the system prompt saying to use exec.
		if execTool := e.registry.Get("exec"); execTool != nil {
			cmd := buildSkillCommand(name, args)
			return execTool.Execute(ctx, map[string]interface{}{"command": cmd})
		}
		return nil, ErrToolNotFound
	}

	return tool.Execute(ctx, args)
}

// ExecuteJSON runs a tool by name with JSON-encoded arguments.
func (e *Executor) ExecuteJSON(ctx context.Context, name string, argsJSON string) (interface{}, error) {
	// Handle empty or whitespace-only argsJSON
	trimmed := strings.TrimSpace(argsJSON)
	if trimmed == "" {
		return e.Execute(ctx, name, nil)
	}

	candidates := buildArgsCandidates(trimmed)
	for _, candidate := range candidates {
		if args, ok := parseJSONObjectArgs(candidate); ok {
			return e.Execute(ctx, name, args)
		}
	}
	for _, candidate := range candidates {
		if args, ok := e.parseLooseArgs(name, candidate); ok {
			return e.Execute(ctx, name, args)
		}
	}

	return nil, fmt.Errorf("invalid tool arguments: expected JSON object or parseable loose args (raw: %q)", argsJSON)
}

// parseJSONObjectArgs tries to parse a JSON object from raw arguments.
// It handles regular objects, nested JSON-string wrapping, and truncated
// string-wrapped JSON emitted by some providers.
func parseJSONObjectArgs(raw string) (map[string]interface{}, bool) {
	var args map[string]interface{}

	if json.Unmarshal([]byte(raw), &args) == nil {
		return args, true
	}
	if normalized := normalizeLooseJSON(raw); normalized != raw {
		if json.Unmarshal([]byte(normalized), &args) == nil {
			return args, true
		}
		if closed := closeIncompleteJSON(normalized); closed != normalized {
			if json.Unmarshal([]byte(closed), &args) == nil {
				return args, true
			}
		}
	}
	if closed := closeIncompleteJSON(raw); closed != raw {
		if json.Unmarshal([]byte(closed), &args) == nil {
			return args, true
		}
	}

	current := raw
	for i := 0; i < 3; i++ {
		var inner string
		if json.Unmarshal([]byte(current), &inner) != nil {
			break
		}
		inner = strings.TrimSpace(inner)
		if inner == "" {
			break
		}
		if json.Unmarshal([]byte(inner), &args) == nil {
			return args, true
		}
		if normalized := normalizeLooseJSON(inner); normalized != inner {
			if json.Unmarshal([]byte(normalized), &args) == nil {
				return args, true
			}
			if closed := closeIncompleteJSON(normalized); closed != normalized {
				if json.Unmarshal([]byte(closed), &args) == nil {
					return args, true
				}
			}
		}
		if closed := closeIncompleteJSON(inner); closed != inner {
			if json.Unmarshal([]byte(closed), &args) == nil {
				return args, true
			}
		}
		current = inner
	}

	if repaired := tryRepairTruncatedJSON(current); repaired != "" {
		if json.Unmarshal([]byte(repaired), &args) == nil {
			return args, true
		}
	}

	if current != raw {
		if repaired := tryRepairTruncatedJSON(raw); repaired != "" {
			if json.Unmarshal([]byte(repaired), &args) == nil {
				return args, true
			}
		}
	}

	return nil, false
}

// parseLooseArgs tries to recover non-JSON arguments commonly emitted by LLMs:
// - key=value key2="value with spaces"
// - raw string mapped into an inferred single string parameter.
func (e *Executor) parseLooseArgs(name, raw string) (map[string]interface{}, bool) {
	if raw == "" {
		return nil, false
	}

	kv := make(map[string]interface{})
	parseKeyValuePairs(raw, kv)
	if len(kv) > 0 {
		return kv, true
	}
	if parseColonValuePairs(raw, kv) {
		return kv, true
	}

	if unquoted, ok := parseJSONString(raw); ok {
		raw = strings.TrimSpace(unquoted)
	}
	if raw == "" {
		return nil, false
	}

	key := e.inferSingleStringArgKey(name)
	if key == "" {
		return nil, false
	}
	return map[string]interface{}{key: raw}, true
}

func parseJSONString(raw string) (string, bool) {
	var s string
	if json.Unmarshal([]byte(raw), &s) != nil {
		return "", false
	}
	return s, true
}

func (e *Executor) inferSingleStringArgKey(name string) string {
	tool := e.registry.Get(name)
	if tool == nil {
		return ""
	}
	params := tool.Definition().Parameters
	if len(params) == 0 {
		return ""
	}

	propsRaw, ok := params["properties"]
	if !ok {
		return ""
	}
	props, ok := propsRaw.(map[string]interface{})
	if !ok || len(props) == 0 {
		return ""
	}

	required := make(map[string]struct{})
	switch req := params["required"].(type) {
	case []string:
		for _, k := range req {
			required[k] = struct{}{}
		}
	case []interface{}:
		for _, v := range req {
			if s, ok := v.(string); ok {
				required[s] = struct{}{}
			}
		}
	}

	preferred := []string{"command", "query", "input", "text", "message", "prompt", "path", "url"}
	for _, key := range preferred {
		if _, ok := required[key]; ok && isStringProperty(props[key]) {
			return key
		}
	}

	if len(required) > 0 {
		keys := make([]string, 0, len(required))
		for k := range required {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if isStringProperty(props[key]) {
				return key
			}
		}
	}

	for _, key := range preferred {
		if isStringProperty(props[key]) {
			return key
		}
	}

	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if isStringProperty(props[key]) {
			return key
		}
	}
	return ""
}

func isStringProperty(prop interface{}) bool {
	m, ok := prop.(map[string]interface{})
	if !ok {
		return false
	}
	typ, _ := m["type"].(string)
	return typ == "string"
}

func buildArgsCandidates(raw string) []string {
	seen := make(map[string]struct{}, 4)
	candidates := make([]string, 0, 4)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		candidates = append(candidates, s)
	}

	add(raw)
	if unfenced, ok := stripCodeFence(raw); ok {
		add(unfenced)
		if jsonSnippet, ok := extractFirstJSONObject(unfenced); ok {
			add(jsonSnippet)
		}
	}
	if jsonSnippet, ok := extractFirstJSONObject(raw); ok {
		add(jsonSnippet)
	}
	return candidates
}

func stripCodeFence(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}
	firstNL := strings.IndexByte(trimmed, '\n')
	if firstNL < 0 {
		return "", false
	}
	header := strings.TrimSpace(trimmed[:firstNL])
	if !strings.HasPrefix(header, "```") {
		return "", false
	}
	body := strings.TrimSpace(trimmed[firstNL+1:])
	if !strings.HasSuffix(body, "```") {
		return "", false
	}
	body = strings.TrimSpace(strings.TrimSuffix(body, "```"))
	if body == "" {
		return "", false
	}
	return body, true
}

func extractFirstJSONObject(raw string) (string, bool) {
	start := strings.IndexByte(raw, '{')
	if start < 0 {
		return "", false
	}

	depth := 0
	inString := false
	escapeNext := false
	for i := start; i < len(raw); i++ {
		c := raw[i]

		if escapeNext {
			escapeNext = false
			continue
		}
		if c == '\\' {
			escapeNext = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}

		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1], true
			}
		}
	}
	return "", false
}

func parseColonValuePairs(raw string, out map[string]interface{}) bool {
	lines := strings.Split(raw, "\n")
	found := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:colon])
		if key == "" {
			continue
		}
		if strings.ContainsAny(key, " \t") {
			continue
		}

		value := strings.TrimSpace(line[colon+1:])
		if value == "" {
			continue
		}
		value = strings.Trim(value, `"'`)
		if value == "" {
			continue
		}
		out[key] = value
		found = true
	}
	return found
}

func normalizeLooseJSON(raw string) string {
	var b strings.Builder
	b.Grow(len(raw) + 8)

	lastNonWS := byte(0)
	writeByte := func(c byte) {
		b.WriteByte(c)
		switch c {
		case ' ', '\t', '\n', '\r':
		default:
			lastNonWS = c
		}
	}

	isIdentStart := func(c byte) bool {
		return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' || c == '$'
	}
	isIdentChar := func(c byte) bool {
		return isIdentStart(c) || (c >= '0' && c <= '9') || c == '-'
	}

	for i := 0; i < len(raw); {
		ch := raw[i]

		switch ch {
		case '"':
			writeByte(ch)
			i++
			for i < len(raw) {
				c := raw[i]
				writeByte(c)
				i++
				if c == '\\' && i < len(raw) {
					writeByte(raw[i])
					i++
					continue
				}
				if c == '"' {
					break
				}
			}
		case '\'':
			// Single-quoted string => double-quoted JSON string.
			writeByte('"')
			i++
			for i < len(raw) {
				c := raw[i]
				i++
				if c == '\\' && i < len(raw) {
					writeByte('\\')
					writeByte(raw[i])
					i++
					continue
				}
				if c == '\'' {
					break
				}
				if c == '"' {
					writeByte('\\')
				}
				writeByte(c)
			}
			writeByte('"')
		case ',':
			j := i + 1
			for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t' || raw[j] == '\n' || raw[j] == '\r') {
				j++
			}
			if j >= len(raw) || raw[j] == '}' || raw[j] == ']' {
				i++
				continue
			}
			writeByte(ch)
			i++
		default:
			if isIdentStart(ch) && (lastNonWS == '{' || lastNonWS == ',') {
				start := i
				for i < len(raw) && isIdentChar(raw[i]) {
					i++
				}
				ident := raw[start:i]

				j := i
				for j < len(raw) && (raw[j] == ' ' || raw[j] == '\t') {
					j++
				}
				if j < len(raw) && raw[j] == ':' {
					writeByte('"')
					b.WriteString(ident)
					lastNonWS = ident[len(ident)-1]
					writeByte('"')
					for i < j {
						writeByte(raw[i])
						i++
					}
					continue
				}

				b.WriteString(ident)
				lastNonWS = ident[len(ident)-1]
				continue
			}
			writeByte(ch)
			i++
		}
	}

	return b.String()
}

func closeIncompleteJSON(raw string) string {
	var b strings.Builder
	b.Grow(len(raw) + 8)

	braceCount := 0
	bracketCount := 0
	inString := false
	escapeNext := false

	for i := 0; i < len(raw); i++ {
		c := raw[i]
		b.WriteByte(c)

		if escapeNext {
			escapeNext = false
			continue
		}
		if c == '\\' {
			escapeNext = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}

		switch c {
		case '{':
			braceCount++
		case '}':
			braceCount--
		case '[':
			bracketCount++
		case ']':
			bracketCount--
		}
	}

	if inString {
		b.WriteByte('"')
	}
	for bracketCount > 0 {
		b.WriteByte(']')
		bracketCount--
	}
	for braceCount > 0 {
		b.WriteByte('}')
		braceCount--
	}
	return b.String()
}

// tryRepairTruncatedJSON attempts to fix truncated JSON strings.
// Some providers return incomplete JSON strings like "\"{\"command\":" without
// a closing quote. This tries to find the closing quote and extract the inner JSON.
func tryRepairTruncatedJSON(s string) string {
	// Must start with quote to be a candidate for repair
	if len(s) < 2 || s[0] != '"' {
		return ""
	}
	// Find the last quote that could be the closing quote
	// We look for an unescaped quote at the end
	for i := len(s) - 1; i >= 1; i-- {
		if s[i] == '"' {
			// Check if this quote is unescaped (not preceded by backslash)
			if i == 1 || s[i-1] != '\\' {
				// Found potential closing quote
				inner := s[1:i]
				// Unescape the inner string
				var builder strings.Builder
				for j := 0; j < len(inner); j++ {
					if inner[j] == '\\' && j+1 < len(inner) {
						// Skip the backslash and take the next character as-is
						j++
					}
					builder.WriteByte(inner[j])
				}
				repaired := builder.String()
				// Verify it looks like valid JSON (starts with { or [)
				if len(repaired) > 0 && (repaired[0] == '{' || repaired[0] == '[') {
					return repaired
				}
				break
			}
		}
	}
	return ""
}

// buildSkillCommand builds a `blue <name> key=value ...` command string
// from a tool name and its arguments for skill fallback execution.
func buildSkillCommand(name string, args map[string]interface{}) string {
	var sb strings.Builder
	sb.WriteString("blue ")
	sb.WriteString(name)
	for k, v := range args {
		sb.WriteByte(' ')
		sb.WriteString(k)
		sb.WriteByte('=')
		switch val := v.(type) {
		case string:
			// Quote strings that contain spaces
			if strings.ContainsAny(val, " \t\n\"") {
				sb.WriteString(fmt.Sprintf("%q", val))
			} else {
				sb.WriteString(val)
			}
		default:
			b, _ := json.Marshal(val)
			sb.WriteString(fmt.Sprintf("%q", string(b)))
		}
	}
	return sb.String()
}

// MockTool is a mock implementation of Tool for testing.
type MockTool struct {
	name        string
	description string
	result      interface{}
	err         error
	delay       bool
}

// NewMockTool creates a new mock tool.
func NewMockTool(name, description string) *MockTool {
	return &MockTool{
		name:        name,
		description: description,
	}
}

// SetResult sets the result to return from Execute.
func (m *MockTool) SetResult(result interface{}) {
	m.result = result
}

// SetError sets the error to return from Execute.
func (m *MockTool) SetError(err error) {
	m.err = err
}

// SetDelay sets whether Execute should wait for context cancellation.
func (m *MockTool) SetDelay(delay bool) {
	m.delay = delay
}

// Definition returns the tool's definition.
func (m *MockTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        m.name,
		Description: m.description,
	}
}

// Execute runs the mock tool.
func (m *MockTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if m.delay {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.err != nil {
		return nil, m.err
	}

	return m.result, nil
}
