// Package tools provides a framework for defining and executing tools/functions.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Common errors
var (
	ErrToolNotFound = errors.New("tool not found")
)

// ForwardedResult wraps a tool result that was auto-forwarded from exec.
// The ActualTool field indicates which tool actually executed the request,
// allowing the UI to display the correct tool name (e.g. "web_query" instead of "exec").
type ForwardedResult struct {
	ActualTool string
	Result     interface{}
}

// ToolDefinition describes a tool that can be called by an LLM.
type ToolDefinition struct {
	Name                string                 `json:"name"`
	Description         string                 `json:"description"`
	Icon                string                 `json:"icon,omitempty"`
	Parameters          map[string]interface{} `json:"parameters,omitempty"`
	RiskLevel           string                 `json:"risk_level,omitempty"`
	Aliases             []string               `json:"aliases,omitempty"`
	SearchHints         []string               `json:"search_hints,omitempty"`
	ShouldDefer         bool                   `json:"should_defer,omitempty"`
	AlwaysLoad          bool                   `json:"always_load,omitempty"`
	VisibilityAllowlist []string               `json:"visibility_allowlist,omitempty"`
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
	exposed  map[string]ToolDefinition
	version  uint64 // incremented on every mutation (Register/ExposeDefinition/Disable/Enable)
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools:    make(map[string]Tool),
		disabled: make(map[string]Tool),
		exposed:  make(map[string]ToolDefinition),
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

// ExposeDefinition registers a tool definition that is visible to the model/UI
// but is not backed by a native Tool implementation in this registry.
func (r *Registry) ExposeDefinition(def ToolDefinition) {
	name := strings.TrimSpace(def.Name)
	if name == "" {
		return
	}
	def.Name = name
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exposed[name] = def
	r.version++
}

// Version returns the current mutation version of the registry.
// It increments on every Register, ExposeDefinition, Disable, or Enable call.
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
	defs := make([]ToolDefinition, 0, len(r.tools)+len(r.exposed))
	seen := make(map[string]struct{}, len(r.tools)+len(r.exposed))
	for _, tool := range r.tools {
		def := tool.Definition()
		defs = append(defs, def)
		seen[def.Name] = struct{}{}
	}
	for name, def := range r.exposed {
		if _, ok := seen[name]; ok {
			continue
		}
		defs = append(defs, def)
	}
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name < defs[j].Name
	})
	return defs
}

// DefinitionsForLocale returns tool definitions with lang/locale parameter
// examples adjusted to the user's current locale.
func (r *Registry) DefinitionsForLocale(locale string) []ToolDefinition {
	return localizeToolDefinitions(r.Definitions(), locale)
}

// DefinitionsForRoute returns visible tool definitions for the given runtime route.
func (r *Registry) DefinitionsForRoute(kind ToolRouteKind) []ToolDefinition {
	return filterToolDefinitionsForRoute(r.Definitions(), kind)
}

// DefinitionsForRouteAndLocale returns visible tool definitions for the given
// runtime route with locale-aware schema examples.
func (r *Registry) DefinitionsForRouteAndLocale(kind ToolRouteKind, locale string) []ToolDefinition {
	return localizeToolDefinitions(r.DefinitionsForRoute(kind), locale)
}

// LookupDefinition returns the visible tool definition for a tool name.
func (r *Registry) LookupDefinition(name string) (ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ToolDefinition{}, false
	}
	if tool := r.tools[trimmed]; tool != nil {
		return tool.Definition(), true
	}
	if def, ok := r.exposed[trimmed]; ok {
		return def, true
	}
	if _, disabled := r.disabled[trimmed]; disabled {
		return ToolDefinition{}, false
	}
	return ToolDefinition{}, false
}

// LookupDefinitionForRoute returns the visible tool definition for a tool name
// scoped to a runtime route.
func (r *Registry) LookupDefinitionForRoute(name string, kind ToolRouteKind) (ToolDefinition, bool) {
	def, ok := r.LookupDefinition(name)
	if !ok {
		return ToolDefinition{}, false
	}
	if !toolDefinitionVisibleForRoute(def, kind) {
		return ToolDefinition{}, false
	}
	return def, true
}

func filterToolDefinitionsForRoute(defs []ToolDefinition, kind ToolRouteKind) []ToolDefinition {
	if len(defs) == 0 {
		return defs
	}
	if kind == ToolRouteKindUnknown {
		return defs
	}
	out := make([]ToolDefinition, 0, len(defs))
	for _, def := range defs {
		if toolDefinitionVisibleForRoute(def, kind) {
			out = append(out, def)
		}
	}
	return out
}

func toolDefinitionVisibleForRoute(def ToolDefinition, kind ToolRouteKind) bool {
	if kind == ToolRouteKindUnknown {
		return true
	}
	if len(def.VisibilityAllowlist) == 0 {
		return true
	}
	want := strings.TrimSpace(strings.ToLower(string(kind)))
	if want == "" {
		return true
	}
	for _, raw := range def.VisibilityAllowlist {
		if strings.EqualFold(strings.TrimSpace(raw), want) {
			return true
		}
	}
	return false
}

// Executor handles tool execution.
type Executor struct {
	registry   *Registry
	traceStore *ToolTraceStore
}

// NewExecutor creates a new tool executor.
func NewExecutor(registry *Registry) *Executor {
	return &Executor{
		registry: registry,
	}
}

// SetTraceStore wires an optional in-memory trace sink for tool executions.
func (e *Executor) SetTraceStore(store *ToolTraceStore) {
	if e == nil {
		return
	}
	e.traceStore = store
}

// Execute runs a tool by name with the given arguments.
func (e *Executor) Execute(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	// Check context first
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	startedAt := time.Now().UTC()
	record := func(actual string, result interface{}, err error) (interface{}, error) {
		if e != nil && e.traceStore != nil {
			e.traceStore.Record(ctx, startedAt, name, actual, args, result, err)
		}
		return result, err
	}

	normalizedName := normalizeCompatToolName(name)
	resolvedName := name
	tool := e.registry.Get(name)
	if shouldPreferCompatNormalizedTool(name, normalizedName, e.registry) {
		resolvedName = normalizedName
		tool = e.registry.Get(normalizedName)
	}
	if tool == nil {
		resolvedName = normalizedName
		tool = e.registry.Get(normalizedName)
	}
	args = normalizeCompatArgs(name, resolvedName, args)

	if tool == nil {
		if cmd, handled, immediate := resolveFactoryAliasCommand(name, args); handled {
			if immediate != "" {
				return record(normalizeFactoryToolName(name), immediate, nil)
			}
			if execTool := e.registry.Get("exec"); execTool != nil {
				result, err := executeToolWithRecovery(ctx, "exec", execTool, map[string]interface{}{"command": cmd})
				return record("exec", result, err)
			}
			return record(resolvedName, nil, ErrToolNotFound)
		}

		if result, ok := unsupportedFactoryToolResult(name); ok {
			return record(normalizeFactoryToolName(name), result, nil)
		}

		fallbackName, fallbackArgs := normalizeCompatFallbackTarget(name, normalizedName, args)
		// Skill fallback: if the LLM calls a tool that does not exist in the
		// tool registry, try forwarding to `blue <name> key=value` via exec.
		if execTool := e.registry.Get("exec"); execTool != nil {
			cmd := buildSkillCommand(fallbackName, fallbackArgs)
			result, err := executeToolWithRecovery(ctx, "exec", execTool, map[string]interface{}{"command": cmd})
			return record("exec", result, err)
		}
		return record(resolvedName, nil, ErrToolNotFound)
	}

	result, err := executeToolWithRecovery(ctx, resolvedName, tool, args)
	return record(resolvedName, result, err)
}

func executeToolWithRecovery(ctx context.Context, name string, tool Tool, args map[string]interface{}) (result interface{}, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			panicErr := fmt.Errorf("panic: %v", recovered)
			err = newToolRuntimeError(
				"tool_execution_panic",
				fmt.Sprintf("tool %q panicked during execution", strings.TrimSpace(name)),
				panicErr,
				map[string]interface{}{
					"tool":  strings.TrimSpace(name),
					"panic": fmt.Sprintf("%v", recovered),
				},
			)
			result = nil
		}
	}()
	return tool.Execute(ctx, args)
}

func normalizeCompatToolName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "read":
		return "file_read"
	case "write":
		return "file_write"
	case "bash":
		return "exec"
	case "delete", "remove", "rm", "unlink":
		return "file_delete"
	case "sessions_list", "sessions_history", "session_status",
		"sessions_spawn", "sessions_send":
		return "sessions"
	case "memory_search", "memory_get", "memory_read",
		"memory_write", "memory_remember", "memory_store",
		"memory_forget", "memory_delete":
		return "memory"
	case "deep_research", "deep-research", "research_run", "research_status":
		return "research"
	case "web_query", "web_search", "web_fetch", "web_read", "web_extract", "web_crawl":
		return "web_query"
	default:
		return name
	}
}

func normalizeCompatArgs(rawName, normalizedName string, args map[string]interface{}) map[string]interface{} {
	switch strings.ToLower(strings.TrimSpace(normalizedName)) {
	case "bash":
		return normalizeExecCompatArgs(args)
	case "exec":
		return normalizeExecCompatArgs(args)
	case "file_read":
		return normalizeFileReadCompatArgs(args)
	case "file_write":
		return normalizeFileWriteCompatArgs(args)
	case "file_delete":
		return normalizeFileDeleteCompatArgs(args)
	case "edit":
		return normalizeEditCompatArgs(args)
	case "grep", "rg":
		return normalizeGrepCompatArgs(args)
	case "find":
		return normalizeFindCompatArgs(args)
	case "ls":
		return normalizeLsCompatArgs(args)
	case "sessions":
		return normalizeSessionsCompatArgs(rawName, args)
	case "memory":
		return normalizeMemoryCompatArgs(rawName, args)
	case "web_query", "web_search", "web_fetch", "web_read", "web_extract", "web_crawl":
		return normalizeWebCompatArgs(rawName, args)
	case "browser":
		return normalizeBrowserCompatArgs(rawName, args)
	case "research", "deep_research", "research_run", "research_status", "deep-research":
		return normalizeDeepResearchCompatArgs(rawName, args)
	default:
		return args
	}
}

func normalizeDeepResearchCompatArgs(rawName string, args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+2)
	for k, v := range args {
		normalized[k] = v
	}
	if strings.TrimSpace(asString(normalized["query"])) == "" {
		if query := firstDeepResearchQuery(normalized); query != "" {
			normalized["query"] = query
		}
	}
	if action := strings.TrimSpace(asString(normalized["action"])); action != "" {
		return normalized
	}
	switch strings.ToLower(strings.TrimSpace(rawName)) {
	case "research_status":
		normalized["action"] = DeepResearchActionStatus
	case "research_run":
		normalized["action"] = DeepResearchActionRun
	default:
		if strings.TrimSpace(firstCompatStringDeep(normalized, "job_id", "jobId", "id")) != "" &&
			firstDeepResearchQuery(normalized) == "" {
			normalized["action"] = DeepResearchActionStatus
		}
	}
	return normalized
}

func normalizeSessionsCompatArgs(rawName string, args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+1)
	for k, v := range args {
		normalized[k] = v
	}
	if strings.TrimSpace(asString(normalized["action"])) != "" {
		return normalized
	}
	switch strings.ToLower(strings.TrimSpace(rawName)) {
	case "sessions_list":
		normalized["action"] = "list"
	case "sessions_history":
		normalized["action"] = "history"
	case "session_status":
		normalized["action"] = "status"
	case "sessions_spawn":
		normalized["action"] = "spawn"
	case "sessions_send":
		normalized["action"] = "send"
	}
	return normalized
}

func normalizeWebCompatArgs(rawName string, args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+3)
	for k, v := range args {
		normalized[k] = v
	}
	if _, ok := normalized["input"]; !ok {
		if input := resolveWebQueryInput(normalized); input != "" {
			normalized["input"] = input
		}
	} else if current, ok := normalized["input"].(string); ok && strings.TrimSpace(current) == "" {
		if input := resolveWebQueryInput(normalized); input != "" {
			normalized["input"] = input
		}
	}
	switch strings.ToLower(strings.TrimSpace(rawName)) {
	case "web_query":
		return normalized
	case "web_search":
		normalized["action"] = "search"
		applyLegacyWebInputAlias(normalized, firstCompatStringDeep(normalized, "query", "q"))
		return normalizeWebSearchCompatArgs(rawName, normalized)
	case "web_fetch":
		normalized["action"] = "fetch"
		applyLegacyWebInputAlias(normalized, firstCompatStringDeep(normalized, "url", "href", "target"))
		return normalizeWebFetchCompatArgs(rawName, normalized)
	case "web_read":
		normalized["action"] = "read"
		applyLegacyWebInputAlias(normalized, firstCompatStringDeep(normalized, "url", "href", "target"))
		return normalizeWebReadCompatArgs(normalized)
	case "web_extract":
		normalized["action"] = "extract"
		return normalizeWebExtractCompatArgs(normalized)
	case "web_crawl":
		normalized["action"] = "crawl"
		return normalizeWebCrawlCompatArgs(normalized)
	default:
		return normalized
	}
}

func applyLegacyWebInputAlias(args map[string]interface{}, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if current, ok := args["input"]; !ok {
		args["input"] = value
	} else if currentString, ok := current.(string); ok && strings.TrimSpace(currentString) == "" {
		args["input"] = value
	}
}

func normalizeCompatFallbackTarget(rawName, normalizedName string, args map[string]interface{}) (string, map[string]interface{}) {
	rawKey := strings.ToLower(strings.TrimSpace(rawName))
	switch rawKey {
	case "web_query", "web_search", "web_fetch", "web_read", "web_extract", "web_crawl":
		return "web_query", normalizeWebCompatArgs(rawName, args)
	case "browser":
		return "browser", normalizeBrowserCompatArgs(rawName, args)
	default:
		if isFactoryToolName(rawName) {
			return rawName, args
		}
		return normalizedName, args
	}
}

func shouldPreferCompatNormalizedTool(rawName, normalizedName string, registry *Registry) bool {
	if registry == nil {
		return false
	}
	rawName = strings.ToLower(strings.TrimSpace(rawName))
	switch rawName {
	case "web_search", "web_fetch", "web_read", "web_extract", "web_crawl":
		if normalizedName == "" || normalizedName == rawName {
			return false
		}
		return registry.Get(normalizedName) != nil
	default:
		return false
	}
}

func normalizeExecCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}
	if command := firstCompatStringDeep(args, "command"); command != "" {
		if topLevel, ok := args["command"].(string); ok && strings.TrimSpace(topLevel) != "" {
			return args
		}
		normalized := make(map[string]interface{}, len(args))
		for k, v := range args {
			normalized[k] = v
		}
		normalized["command"] = command
		return normalized
	}

	candidate := firstCompatStringDeep(args, "cmd")
	if candidate == "" {
		for _, key := range []string{"arguments", "input", "params", "payload"} {
			if nestedCmd := extractExecCommandFromCompatValue(args[key]); nestedCmd != "" {
				candidate = nestedCmd
				break
			}
		}
	}
	if candidate == "" {
		toolVal := firstCompatStringDeep(args, "tool")
		trimmed := strings.TrimSpace(toolVal)
		if trimmed != "" && !strings.EqualFold(trimmed, "exec") {
			candidate = trimmed
		}
	}
	if strings.TrimSpace(candidate) == "" {
		return args
	}

	normalized := make(map[string]interface{}, len(args))
	for k, v := range args {
		normalized[k] = v
	}
	normalized["command"] = candidate
	delete(normalized, "cmd")
	return normalized
}

func normalizeFileReadCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}

	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["path"])) == "" {
		if path := firstCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["start_line"]; !ok {
		if startLine, ok := firstCompatValue(normalized, "start_line", "startLine"); ok {
			normalized["start_line"] = startLine
		}
	}
	if _, ok := normalized["end_line"]; !ok {
		if endLine, ok := firstCompatValue(normalized, "end_line", "endLine"); ok {
			normalized["end_line"] = endLine
		}
	}
	if _, ok := normalized["max_bytes"]; !ok {
		if maxBytes, ok := firstCompatValue(normalized, "max_bytes", "maxBytes"); ok {
			normalized["max_bytes"] = maxBytes
		}
	}

	for _, key := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(normalized[key])
		if !ok {
			continue
		}

		if strings.TrimSpace(asString(normalized["path"])) == "" {
			if path := firstCompatPathString(nested); path != "" {
				normalized["path"] = path
			}
		}
		if _, hasStartLine := normalized["start_line"]; !hasStartLine {
			if startLine, ok := firstCompatValue(nested, "start_line", "startLine"); ok {
				normalized["start_line"] = startLine
			}
		}
		if _, hasEndLine := normalized["end_line"]; !hasEndLine {
			if endLine, ok := firstCompatValue(nested, "end_line", "endLine"); ok {
				normalized["end_line"] = endLine
			}
		}
		if _, hasMaxBytes := normalized["max_bytes"]; !hasMaxBytes {
			if maxBytes, ok := firstCompatValue(nested, "max_bytes", "maxBytes"); ok {
				normalized["max_bytes"] = maxBytes
			}
		}
	}

	return normalized
}

func normalizeFileWriteCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}

	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["path"])) == "" {
		if path := firstCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["content"]; !ok {
		if content, ok := firstCompatValue(normalized, "content", "text", "body", "value"); ok {
			normalized["content"] = content
		}
	}
	if _, ok := normalized["create_dirs"]; !ok {
		if createDirs, ok := firstCompatValue(normalized, "create_dirs", "createDirs"); ok {
			normalized["create_dirs"] = createDirs
		}
	}

	for _, key := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(normalized[key])
		if !ok {
			continue
		}

		if strings.TrimSpace(asString(normalized["path"])) == "" {
			if path := firstCompatPathString(nested); path != "" {
				normalized["path"] = path
			}
		}
		if _, hasContent := normalized["content"]; !hasContent {
			if content, ok := firstCompatValue(nested, "content", "text", "body", "value"); ok {
				normalized["content"] = content
			}
		}
		if _, hasAppend := normalized["append"]; !hasAppend {
			if appendMode, ok := nested["append"]; ok {
				normalized["append"] = appendMode
			}
		}
		if _, hasCreateDirs := normalized["create_dirs"]; !hasCreateDirs {
			if createDirs, ok := firstCompatValue(nested, "create_dirs", "createDirs"); ok {
				normalized["create_dirs"] = createDirs
			}
		}
		if _, hasLine := normalized["line"]; !hasLine {
			if line, ok := nested["line"]; ok {
				normalized["line"] = line
			}
		}
	}

	return normalized
}

func normalizeFileDeleteCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}

	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["path"])) == "" {
		if path := firstCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["recursive"]; !ok {
		if recursive, ok := firstCompatValue(normalized, "recursive", "recursive_delete", "recursiveDelete"); ok {
			normalized["recursive"] = recursive
		}
	}
	if _, ok := normalized["missing_ok"]; !ok {
		if missingOK, ok := firstCompatValue(normalized, "missing_ok", "missingOk", "ignore_missing", "ignoreMissing"); ok {
			normalized["missing_ok"] = missingOK
		}
	}

	for _, key := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(normalized[key])
		if !ok {
			continue
		}

		if strings.TrimSpace(asString(normalized["path"])) == "" {
			if path := firstCompatPathString(nested); path != "" {
				normalized["path"] = path
			}
		}
		if _, hasRecursive := normalized["recursive"]; !hasRecursive {
			if recursive, ok := firstCompatValue(nested, "recursive", "recursive_delete", "recursiveDelete"); ok {
				normalized["recursive"] = recursive
			}
		}
		if _, hasMissingOK := normalized["missing_ok"]; !hasMissingOK {
			if missingOK, ok := firstCompatValue(nested, "missing_ok", "missingOk", "ignore_missing", "ignoreMissing"); ok {
				normalized["missing_ok"] = missingOK
			}
		}
	}

	return normalized
}

func firstCompatPathString(args map[string]interface{}) string {
	return firstCompatString(
		args,
		"path",
		"file_path",
		"filePath",
		"filepath",
		"path_name",
		"pathName",
		"pathname",
		"filename",
		"fileName",
		"target_path",
		"targetPath",
		"target_file",
		"targetFile",
		"output_path",
		"outputPath",
		"file",
	)
}

func normalizeEditCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}

	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["path"])) == "" {
		if path := firstCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["old_text"]; !ok {
		if oldText, ok := firstCompatValue(normalized, "old_text", "oldText"); ok {
			normalized["old_text"] = oldText
		}
	}
	if _, ok := normalized["new_text"]; !ok {
		if newText, ok := firstCompatValue(normalized, "new_text", "newText"); ok {
			normalized["new_text"] = newText
		}
	}
	if _, ok := normalized["replace_all"]; !ok {
		if replaceAll, ok := firstCompatValue(normalized, "replace_all", "replaceAll"); ok {
			normalized["replace_all"] = replaceAll
		}
	}

	for _, key := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(normalized[key])
		if !ok {
			continue
		}

		if strings.TrimSpace(asString(normalized["path"])) == "" {
			if path := firstCompatPathString(nested); path != "" {
				normalized["path"] = path
			}
		}
		if _, hasOldText := normalized["old_text"]; !hasOldText {
			if oldText, ok := firstCompatValue(nested, "old_text", "oldText"); ok {
				normalized["old_text"] = oldText
			}
		}
		if _, hasNewText := normalized["new_text"]; !hasNewText {
			if newText, ok := firstCompatValue(nested, "new_text", "newText"); ok {
				normalized["new_text"] = newText
			}
		}
		if _, hasReplaceAll := normalized["replace_all"]; !hasReplaceAll {
			if replaceAll, ok := firstCompatValue(nested, "replace_all", "replaceAll"); ok {
				normalized["replace_all"] = replaceAll
			}
		}
	}

	return normalized
}

func normalizeGrepCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}

	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["pattern"])) == "" {
		if pattern := firstCompatString(normalized, "pattern", "regex", "query", "search", "text", "input", "content"); pattern != "" {
			normalized["pattern"] = pattern
		}
	}
	if strings.TrimSpace(asString(normalized["path"])) == "" {
		if path := firstCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["max_results"]; !ok {
		if maxResults, ok := firstCompatValue(normalized, "max_results", "maxResults", "limit"); ok {
			normalized["max_results"] = maxResults
		}
	}
	if _, ok := normalized["case_sensitive"]; !ok {
		if caseSensitive, ok := firstCompatValue(normalized, "case_sensitive", "caseSensitive"); ok {
			normalized["case_sensitive"] = caseSensitive
		}
	}
	if _, ok := normalized["include_hidden"]; !ok {
		if includeHidden, ok := firstCompatValue(normalized, "include_hidden", "includeHidden"); ok {
			normalized["include_hidden"] = includeHidden
		}
	}

	for _, key := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(normalized[key])
		if !ok {
			continue
		}
		if strings.TrimSpace(asString(normalized["pattern"])) == "" {
			if pattern := firstCompatString(nested, "pattern", "regex", "query", "search", "text", "input", "content"); pattern != "" {
				normalized["pattern"] = pattern
			}
		}
		if strings.TrimSpace(asString(normalized["path"])) == "" {
			if path := firstCompatPathString(nested); path != "" {
				normalized["path"] = path
			}
		}
		if _, hasMaxResults := normalized["max_results"]; !hasMaxResults {
			if maxResults, ok := firstCompatValue(nested, "max_results", "maxResults", "limit"); ok {
				normalized["max_results"] = maxResults
			}
		}
		if _, hasCaseSensitive := normalized["case_sensitive"]; !hasCaseSensitive {
			if caseSensitive, ok := firstCompatValue(nested, "case_sensitive", "caseSensitive"); ok {
				normalized["case_sensitive"] = caseSensitive
			}
		}
		if _, hasIncludeHidden := normalized["include_hidden"]; !hasIncludeHidden {
			if includeHidden, ok := firstCompatValue(nested, "include_hidden", "includeHidden"); ok {
				normalized["include_hidden"] = includeHidden
			}
		}
	}

	return normalized
}

func normalizeFindCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}

	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["path"])) == "" {
		if path := firstCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if strings.TrimSpace(asString(normalized["pattern"])) == "" {
		if pattern := firstCompatString(normalized, "pattern", "glob", "name"); pattern != "" {
			normalized["pattern"] = pattern
		}
	}
	if _, ok := normalized["max_depth"]; !ok {
		if maxDepth, ok := firstCompatValue(normalized, "max_depth", "maxDepth"); ok {
			normalized["max_depth"] = maxDepth
		}
	}
	if _, ok := normalized["include_hidden"]; !ok {
		if includeHidden, ok := firstCompatValue(normalized, "include_hidden", "includeHidden"); ok {
			normalized["include_hidden"] = includeHidden
		}
	}
	if _, ok := normalized["type"]; !ok {
		if typeFilter, ok := firstCompatValue(normalized, "type", "fileType", "entryType"); ok {
			normalized["type"] = typeFilter
		}
	}

	for _, key := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(normalized[key])
		if !ok {
			continue
		}
		if strings.TrimSpace(asString(normalized["path"])) == "" {
			if path := firstCompatPathString(nested); path != "" {
				normalized["path"] = path
			}
		}
		if strings.TrimSpace(asString(normalized["pattern"])) == "" {
			if pattern := firstCompatString(nested, "pattern", "glob", "name"); pattern != "" {
				normalized["pattern"] = pattern
			}
		}
		if _, hasMaxDepth := normalized["max_depth"]; !hasMaxDepth {
			if maxDepth, ok := firstCompatValue(nested, "max_depth", "maxDepth"); ok {
				normalized["max_depth"] = maxDepth
			}
		}
		if _, hasIncludeHidden := normalized["include_hidden"]; !hasIncludeHidden {
			if includeHidden, ok := firstCompatValue(nested, "include_hidden", "includeHidden"); ok {
				normalized["include_hidden"] = includeHidden
			}
		}
		if _, hasType := normalized["type"]; !hasType {
			if typeFilter, ok := firstCompatValue(nested, "type", "fileType", "entryType"); ok {
				normalized["type"] = typeFilter
			}
		}
	}

	return normalized
}

func normalizeLsCompatArgs(args map[string]interface{}) map[string]interface{} {
	if len(args) == 0 {
		return args
	}

	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["path"])) == "" {
		if path := firstCompatPathString(normalized); path != "" {
			normalized["path"] = path
		}
	}
	if _, ok := normalized["max_depth"]; !ok {
		if maxDepth, ok := firstCompatValue(normalized, "max_depth", "maxDepth"); ok {
			normalized["max_depth"] = maxDepth
		}
	}
	if _, ok := normalized["include_hidden"]; !ok {
		if includeHidden, ok := firstCompatValue(normalized, "include_hidden", "includeHidden"); ok {
			normalized["include_hidden"] = includeHidden
		}
	}

	for _, key := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(normalized[key])
		if !ok {
			continue
		}
		if strings.TrimSpace(asString(normalized["path"])) == "" {
			if path := firstCompatPathString(nested); path != "" {
				normalized["path"] = path
			}
		}
		if _, hasMaxDepth := normalized["max_depth"]; !hasMaxDepth {
			if maxDepth, ok := firstCompatValue(nested, "max_depth", "maxDepth"); ok {
				normalized["max_depth"] = maxDepth
			}
		}
		if _, hasIncludeHidden := normalized["include_hidden"]; !hasIncludeHidden {
			if includeHidden, ok := firstCompatValue(nested, "include_hidden", "includeHidden"); ok {
				normalized["include_hidden"] = includeHidden
			}
		}
	}

	return normalized
}

func normalizeMemoryCompatArgs(rawName string, args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+2)
	for k, v := range args {
		normalized[k] = v
	}

	action, _ := normalized["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		if inferred := inferMemoryActionFromAlias(rawName); inferred != "" {
			action = inferred
			normalized["action"] = inferred
		}
	}

	switch action {
	case "search":
		if strings.TrimSpace(asString(normalized["query"])) == "" {
			if q := firstCompatString(normalized, "query", "q", "search", "vsearch", "text", "input", "content"); q != "" {
				normalized["query"] = q
			}
		}
	case "get", "forget":
		if strings.TrimSpace(asString(normalized["id"])) == "" {
			if id := firstCompatString(normalized, "id", "path", "file", "memory_id", "memoryId", "key"); id != "" {
				normalized["id"] = id
			}
		}
	case "remember":
		if strings.TrimSpace(asString(normalized["content"])) == "" {
			if content := firstCompatString(normalized, "content", "text", "input", "memory", "note", "message"); content != "" {
				normalized["content"] = content
			}
		}
		if tags, ok := coerceCompatStringList(normalized["tags"]); ok {
			normalized["tags"] = tags
		} else if category := firstCompatString(normalized, "category", "tag"); category != "" {
			normalized["tags"] = []interface{}{category}
		}
	}

	return normalized
}

func normalizeWebSearchCompatArgs(rawName string, args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+4)
	for k, v := range args {
		normalized[k] = v
	}

	if strings.TrimSpace(asString(normalized["query"])) == "" {
		query := firstCompatStringDeep(normalized, "query", "q", "search", "keyword", "text", "input", "url")
		if query == "" && strings.EqualFold(strings.TrimSpace(rawName), "web_fetch") {
			query = firstCompatStringDeep(normalized, "href", "target")
		}
		if query != "" {
			normalized["query"] = query
		}
	}

	if _, hasMaxResults := normalized["max_results"]; !hasMaxResults {
		if limit, ok := firstCompatIntDeep(normalized, "max_results", "maxResults", "limit"); ok && limit > 0 {
			normalized["max_results"] = limit
		}
	}
	if strings.TrimSpace(asString(normalized["region"])) == "" {
		if region := firstCompatStringDeep(normalized, "region"); region != "" {
			normalized["region"] = region
		}
	}
	if strings.TrimSpace(asString(normalized["provider"])) == "" {
		if provider := firstCompatStringDeep(normalized, "provider"); provider != "" {
			normalized["provider"] = provider
		}
	}
	if strings.TrimSpace(asString(normalized["format"])) == "" {
		if format := firstCompatStringDeep(normalized, "format"); format != "" {
			normalized["format"] = format
		}
	}

	return normalized
}

func normalizeBrowserCompatArgs(rawName string, args map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{}, len(args)+6)
	for k, v := range args {
		normalized[k] = v
	}

	action := strings.ToLower(strings.TrimSpace(asString(normalized["action"])))
	if action == "" {
		action = strings.ToLower(strings.TrimSpace(firstCompatStringDeep(normalized, "action")))
	}
	if action == "" {
		switch strings.ToLower(strings.TrimSpace(rawName)) {
		case "web_fetch":
			action = "navigate"
		default:
			if strings.TrimSpace(asString(normalized["url"])) != "" || firstCompatStringDeep(normalized, "url", "href", "target", "query", "q") != "" {
				action = "navigate"
			}
		}
	}
	if action != "" {
		normalized["action"] = action
	}

	if strings.TrimSpace(asString(normalized["url"])) == "" {
		if url := firstCompatStringDeep(normalized, "url", "href", "target", "input", "query", "q"); url != "" {
			normalized["url"] = url
		}
	}
	if _, ok := normalized["ref"]; !ok {
		if ref, ok := firstCompatValueDeep(normalized, "ref"); ok {
			normalized["ref"] = ref
		}
	}
	if strings.TrimSpace(asString(normalized["act_type"])) == "" {
		if actType := firstCompatStringDeep(normalized, "act_type", "actType"); actType != "" {
			normalized["act_type"] = actType
		}
	}
	action, actType := CanonicalizeBrowserAction(
		asString(normalized["action"]),
		asString(normalized["act_type"]),
	)
	if action != "" {
		normalized["action"] = action
	}
	if actType != "" {
		normalized["act_type"] = actType
	}
	if strings.TrimSpace(asString(normalized["value"])) == "" {
		if value := firstCompatStringDeep(normalized, "value"); value != "" {
			normalized["value"] = value
		}
	}
	if strings.TrimSpace(asString(normalized["target_id"])) == "" {
		if targetID := firstCompatStringDeep(normalized, "target_id", "targetId"); targetID != "" {
			normalized["target_id"] = targetID
		}
	}
	if _, ok := normalized["vision"]; !ok {
		if vision, ok := firstCompatValueDeep(normalized, "vision"); ok {
			normalized["vision"] = vision
		}
	}
	if strings.TrimSpace(asString(normalized["recipe"])) == "" {
		if recipe := firstCompatStringDeep(normalized, "recipe"); recipe != "" {
			normalized["recipe"] = recipe
		}
	}
	if _, ok := normalized["params"]; !ok {
		if params, ok := firstCompatValueDeep(normalized, "params"); ok {
			normalized["params"] = params
		}
	}

	return normalized
}

func inferMemoryActionFromAlias(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "memory_search":
		return "search"
	case "memory_get", "memory_read":
		return "get"
	case "memory_write", "memory_remember", "memory_store":
		return "remember"
	case "memory_forget", "memory_delete":
		return "forget"
	default:
		return ""
	}
}

func firstCompatValueDeep(args map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		value, ok := args[key]
		if !ok || value == nil {
			continue
		}
		if key == "arguments" || key == "input" || key == "params" || key == "payload" {
			if _, nested := coerceCompatMap(value); nested {
				continue
			}
		}
		return value, true
	}
	for _, key := range []string{"arguments", "input", "params", "payload"} {
		nested, ok := coerceCompatMap(args[key])
		if !ok {
			continue
		}
		if value, ok := firstCompatValue(nested, keys...); ok {
			return value, true
		}
	}
	return nil, false
}

func firstCompatStringDeep(args map[string]interface{}, keys ...string) string {
	if value, ok := firstCompatValueDeep(args, keys...); ok {
		if s := strings.TrimSpace(asString(value)); s != "" {
			return s
		}
	}
	return ""
}

func firstCompatIntDeep(args map[string]interface{}, keys ...string) (int, bool) {
	value, ok := firstCompatValueDeep(args, keys...)
	if !ok {
		return 0, false
	}
	return coerceCompatInt(value)
}

func firstCompatString(args map[string]interface{}, keys ...string) string {
	return firstCompatStringDeep(args, keys...)
}

func coerceCompatMap(v interface{}) (map[string]interface{}, bool) {
	switch typed := v.(type) {
	case map[string]interface{}:
		return typed, true
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return nil, false
		}
		if parsed, ok := parseJSONObjectArgs(raw); ok {
			return parsed, true
		}
	}
	return nil, false
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func coerceCompatStringList(v interface{}) ([]interface{}, bool) {
	switch typed := v.(type) {
	case []interface{}:
		out := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		if len(out) > 0 {
			return out, true
		}
	case []string:
		out := make([]interface{}, 0, len(typed))
		for _, s := range typed {
			if trimmed := strings.TrimSpace(s); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		if len(out) > 0 {
			return out, true
		}
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return nil, false
		}
		parts := strings.Split(raw, ",")
		out := make([]interface{}, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		if len(out) > 0 {
			return out, true
		}
	}
	return nil, false
}

func coerceCompatInt(v interface{}) (int, bool) {
	switch typed := v.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case float32:
		return int(typed), true
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(n), true
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return 0, false
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func extractExecCommandFromCompatValue(v interface{}) string {
	switch typed := v.(type) {
	case map[string]interface{}:
		if command, ok := typed["command"].(string); ok && strings.TrimSpace(command) != "" {
			return command
		}
		if cmd, ok := typed["cmd"].(string); ok && strings.TrimSpace(cmd) != "" {
			return cmd
		}
		if toolVal, ok := typed["tool"].(string); ok {
			trimmed := strings.TrimSpace(toolVal)
			if trimmed != "" && !strings.EqualFold(trimmed, "exec") {
				return trimmed
			}
		}
		// Handle one more nesting level: {"arguments":{"cmd":"..."}}
		if nested := extractExecCommandFromCompatValue(typed["arguments"]); nested != "" {
			return nested
		}
	case string:
		raw := strings.TrimSpace(typed)
		if raw == "" {
			return ""
		}
		if parsed, ok := parseJSONObjectArgs(raw); ok {
			return extractExecCommandFromCompatValue(parsed)
		}
		if strings.EqualFold(raw, "exec") {
			return ""
		}
		return raw
	}
	return ""
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
	if escaped := escapeLiteralJSONStringControlChars(raw); escaped != raw {
		if json.Unmarshal([]byte(escaped), &args) == nil {
			return args, true
		}
	}
	if repaired, ok := extractLastJSONObjectFromConcatenatedPayload(raw); ok {
		if json.Unmarshal([]byte(repaired), &args) == nil {
			return args, true
		}
	}
	if normalized := normalizeLooseJSON(raw); normalized != raw {
		if json.Unmarshal([]byte(normalized), &args) == nil {
			return args, true
		}
		if repaired, ok := extractLastJSONObjectFromConcatenatedPayload(normalized); ok {
			if json.Unmarshal([]byte(repaired), &args) == nil {
				return args, true
			}
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

func escapeLiteralJSONStringControlChars(raw string) string {
	if raw == "" {
		return raw
	}

	var out strings.Builder
	out.Grow(len(raw) + 16)

	inString := false
	escaped := false
	changed := false

	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				out.WriteByte(ch)
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				out.WriteByte(ch)
				escaped = true
				continue
			case '"':
				out.WriteByte(ch)
				inString = false
				continue
			case '\n':
				out.WriteString(`\n`)
				changed = true
				continue
			case '\r':
				out.WriteString(`\r`)
				changed = true
				continue
			case '\t':
				out.WriteString(`\t`)
				changed = true
				continue
			default:
				if ch < 0x20 {
					out.WriteString(fmt.Sprintf(`\u%04x`, ch))
					changed = true
					continue
				}
			}
			out.WriteByte(ch)
			continue
		}

		if ch == '"' {
			inString = true
		}
		out.WriteByte(ch)
	}

	if !changed {
		return raw
	}
	return out.String()
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
	if single, ok := parseSingleStringJSONArray(raw); ok {
		raw = single
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

func parseSingleStringJSONArray(raw string) (string, bool) {
	var values []string
	if json.Unmarshal([]byte(raw), &values) != nil || len(values) != 1 {
		return "", false
	}
	value := strings.TrimSpace(values[0])
	if value == "" {
		return "", false
	}
	return value, true
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

func extractLastJSONObjectFromConcatenatedPayload(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}

	segments := make([]string, 0, 2)
	for i := 0; i < len(trimmed); {
		for i < len(trimmed) && isJSONWhitespace(trimmed[i]) {
			i++
		}
		if i >= len(trimmed) {
			break
		}
		if trimmed[i] != '{' {
			return "", false
		}

		start := i
		depth := 0
		inString := false
		escapeNext := false
		for i < len(trimmed) {
			c := trimmed[i]

			if escapeNext {
				escapeNext = false
				i++
				continue
			}
			if c == '\\' {
				escapeNext = true
				i++
				continue
			}
			if c == '"' {
				inString = !inString
				i++
				continue
			}
			if inString {
				i++
				continue
			}

			switch c {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					i++
					segments = append(segments, trimmed[start:i])
					goto nextSegment
				}
			}
			i++
		}
		return "", false

	nextSegment:
	}

	if len(segments) < 2 {
		return "", false
	}
	for i := len(segments) - 1; i >= 0; i-- {
		var obj map[string]interface{}
		if json.Unmarshal([]byte(segments[i]), &obj) == nil && len(obj) > 0 {
			return segments[i], true
		}
	}
	return segments[len(segments)-1], true
}

func isJSONWhitespace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
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
		Parameters: map[string]interface{}{
			"type":                 "object",
			"properties":           map[string]interface{}{},
			"additionalProperties": true,
		},
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
