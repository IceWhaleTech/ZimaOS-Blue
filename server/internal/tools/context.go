package tools

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type toolContextKey string

const (
	langKey         toolContextKey = "tool_lang"
	channelKey      toolContextKey = "tool_channel"
	deviceKey       toolContextKey = "tool_device"
	imageInputsKey  toolContextKey = "tool_image_inputs"
	cardEmitKey     toolContextKey = "tool_card_emit"
	sessionIDKey    toolContextKey = "tool_session_id"
	providerKey     toolContextKey = "tool_provider"
	providerIDKey   toolContextKey = "tool_provider_id"
	modelKey        toolContextKey = "tool_model"
	agentIDKey      toolContextKey = "tool_agent_id"
	routeKindKey    toolContextKey = "tool_route_kind"
	runIDKey        toolContextKey = "tool_run_id"
	runStepKey      toolContextKey = "tool_run_step"
	autoConfirmKey  toolContextKey = "tool_auto_confirm"
	checkpointKey   toolContextKey = "tool_browser_checkpoint"
	browserModeKey  toolContextKey = "tool_browser_launch_mode"
	browserHintKey  toolContextKey = "tool_browser_route_hint"
	fsScopeKey      toolContextKey = "tool_fs_scope"
	artifactEmitKey toolContextKey = "tool_artifact_emit"
	eventEmitKey    toolContextKey = "tool_event_emit"
	artifactRootKey toolContextKey = "tool_artifact_root"
	turnIDKey       toolContextKey = "tool_turn_id"
	turnToolsKey    toolContextKey = "tool_turn_tools"
)

type BrowserLaunchMode string
type ToolRouteKind string
type ArtifactEmitFunc func(ctx context.Context, artifact ToolArtifact) error
type EventEmitFunc func(ctx context.Context, event ToolEvent) error

type ToolArtifact struct {
	Kind      string
	Label     string
	PathOrURL string
	MimeType  string
	SizeBytes int64
	Metadata  map[string]interface{}
}

type ToolEvent struct {
	Type           string
	Message        string
	StepIndex      int
	ToolName       string
	CapabilityKind string
	Payload        map[string]interface{}
}

// BrowserRouteHint carries high-level browser intent so the backend can choose
// the right runtime before a request turns into engine-specific calls.
type BrowserRouteHint struct {
	Action           string
	FollowupAction   string
	Vision           bool
	RequiresImage    bool
	RequiresInteract bool
	RequiresRecipe   bool
}

// ToolImageInput carries an inline image attachment through tool execution
// context so review-style tools can recover the original image input even when
// the model omits it from structured arguments.
type ToolImageInput struct {
	Name     string
	MimeType string
	Data     string
}

type fsScopeContext struct {
	roots        []string
	aliases      map[string]string
	replaceRoots bool
}

const (
	BrowserLaunchModeDefault BrowserLaunchMode = ""
	BrowserLaunchModeVisible BrowserLaunchMode = "visible"

	ToolRouteKindUnknown  ToolRouteKind = ""
	ToolRouteKindChat     ToolRouteKind = "chat"
	ToolRouteKindAgent    ToolRouteKind = "agent"
	ToolRouteKindWorkflow ToolRouteKind = "workflow"
)

// CardEmitFunc is a callback that tools can use to emit streaming typeless
// cards during execution. The card map is JSON-serialised and pushed to the
// client's SSE stream immediately.
type CardEmitFunc func(card map[string]interface{})

// BrowserCheckpointFunc asks host application to resolve a browser checkpoint.
type BrowserCheckpointFunc func(ctx context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, error)

// BrowserCheckpointResult is the host resolution for a checkpoint request.
type BrowserCheckpointResult struct {
	Decision     BrowserCheckpointDecision
	CheckpointID string
	Pending      bool
	Message      string
}

// WithCardEmitter returns a context carrying a card emitter callback.
func WithCardEmitter(ctx context.Context, fn CardEmitFunc) context.Context {
	return context.WithValue(ctx, cardEmitKey, fn)
}

// WithBrowserCheckpointRequester returns a context carrying checkpoint callback.
func WithBrowserCheckpointRequester(ctx context.Context, fn BrowserCheckpointFunc) context.Context {
	return context.WithValue(ctx, checkpointKey, fn)
}

// WithBrowserRouteHint returns a context carrying browser routing hints.
func WithBrowserRouteHint(ctx context.Context, hint BrowserRouteHint) context.Context {
	return context.WithValue(ctx, browserHintKey, hint)
}

// GetBrowserRouteHint extracts browser routing hints from context.
func GetBrowserRouteHint(ctx context.Context) BrowserRouteHint {
	if ctx == nil {
		return BrowserRouteHint{}
	}
	hint, _ := ctx.Value(browserHintKey).(BrowserRouteHint)
	return hint
}

// EmitCard sends a typeless card to the client if an emitter is set.
// Safe to call even when no emitter is present (no-op).
// Automatically adds "typeless": true to the card.
func EmitCard(ctx context.Context, card map[string]interface{}) {
	// Add typeless tag to identify this as a typeless card
	card["typeless"] = true
	if fn, ok := ctx.Value(cardEmitKey).(CardEmitFunc); ok && fn != nil {
		fn(card)
	}
}

// RequestBrowserCheckpoint asks the host to resolve a browser checkpoint.
// Returns ok=false when no requester exists in context.
func RequestBrowserCheckpoint(ctx context.Context, req BrowserCheckpointRequest) (BrowserCheckpointResult, bool, error) {
	fn, ok := ctx.Value(checkpointKey).(BrowserCheckpointFunc)
	if !ok || fn == nil {
		return BrowserCheckpointResult{}, false, nil
	}
	result, err := fn(ctx, req)
	return result, true, err
}

func WithArtifactEmitter(ctx context.Context, fn ArtifactEmitFunc) context.Context {
	return context.WithValue(ctx, artifactEmitKey, fn)
}

func EmitArtifact(ctx context.Context, artifact ToolArtifact) error {
	fn, ok := ctx.Value(artifactEmitKey).(ArtifactEmitFunc)
	if !ok || fn == nil {
		return nil
	}
	return fn(ctx, artifact)
}

func WithEventEmitter(ctx context.Context, fn EventEmitFunc) context.Context {
	return context.WithValue(ctx, eventEmitKey, fn)
}

func EmitEvent(ctx context.Context, event ToolEvent) error {
	fn, ok := ctx.Value(eventEmitKey).(EventEmitFunc)
	if !ok || fn == nil {
		return nil
	}
	return fn(ctx, event)
}

func WithRunArtifactRoot(ctx context.Context, root string) context.Context {
	root = strings.TrimSpace(root)
	if root == "" {
		return ctx
	}
	return context.WithValue(ctx, artifactRootKey, root)
}

func GetRunArtifactRoot(ctx context.Context) string {
	if v, ok := ctx.Value(artifactRootKey).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// WithUserID returns a context carrying the user ID for tool/skill execution.
func WithUserID(ctx context.Context, userID string) context.Context {
	return skill.WithUserID(ctx, userID)
}

// GetUserID extracts the user ID from the context.
func GetUserID(ctx context.Context) string {
	return skill.GetUserID(ctx)
}

// WithLang returns a context carrying the language/locale (e.g. "en-US", "zh-CN").
func WithLang(ctx context.Context, lang string) context.Context {
	ctx = skill.WithLang(ctx, lang)
	return context.WithValue(ctx, langKey, lang)
}

// GetLang extracts the language from the context. Returns "en-US" if not set.
func GetLang(ctx context.Context) string {
	if v, ok := ctx.Value(langKey).(string); ok && v != "" {
		return v
	}
	return skill.GetLang(ctx)
}

// WithChannel returns a context carrying the channel name (e.g. "telegram", "web").
func WithChannel(ctx context.Context, ch string) context.Context {
	return context.WithValue(ctx, channelKey, ch)
}

// GetChannel extracts the channel name from the context.
func GetChannel(ctx context.Context) string {
	if v, ok := ctx.Value(channelKey).(string); ok {
		return v
	}
	return ""
}

// WithDevice returns a context carrying the device type ("desktop" or "mobile").
func WithDevice(ctx context.Context, device string) context.Context {
	return context.WithValue(ctx, deviceKey, device)
}

// GetDevice extracts the device type from the context.
func GetDevice(ctx context.Context) string {
	if v, ok := ctx.Value(deviceKey).(string); ok {
		return v
	}
	return ""
}

// WithImageInputs returns a context carrying inline image inputs for tools.
func WithImageInputs(ctx context.Context, inputs []ToolImageInput) context.Context {
	if len(inputs) == 0 {
		return ctx
	}
	normalized := make([]ToolImageInput, 0, len(inputs))
	for _, input := range inputs {
		data := strings.TrimSpace(input.Data)
		if data == "" {
			continue
		}
		normalized = append(normalized, ToolImageInput{
			Name:     strings.TrimSpace(input.Name),
			MimeType: strings.TrimSpace(input.MimeType),
			Data:     data,
		})
	}
	if len(normalized) == 0 {
		return ctx
	}
	return context.WithValue(ctx, imageInputsKey, normalized)
}

// GetImageInputs extracts inline image inputs from tool execution context.
func GetImageInputs(ctx context.Context) []ToolImageInput {
	if ctx == nil {
		return nil
	}
	inputs, ok := ctx.Value(imageInputsKey).([]ToolImageInput)
	if !ok || len(inputs) == 0 {
		return nil
	}
	out := make([]ToolImageInput, len(inputs))
	copy(out, inputs)
	return out
}

// WithSessionID returns a context carrying the conversation/session ID.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// GetSessionID extracts the conversation/session ID from the context.
func GetSessionID(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	return ""
}

// WithProvider returns a context carrying the provider name used for tool execution.
func WithProvider(ctx context.Context, provider string) context.Context {
	return context.WithValue(ctx, providerKey, strings.TrimSpace(provider))
}

// GetProvider extracts the provider name from the context.
func GetProvider(ctx context.Context) string {
	if v, ok := ctx.Value(providerKey).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// WithProviderID returns a context carrying the sticky/internal provider ID.
func WithProviderID(ctx context.Context, providerID string) context.Context {
	return context.WithValue(ctx, providerIDKey, strings.TrimSpace(providerID))
}

// GetProviderID extracts the provider ID from the context.
func GetProviderID(ctx context.Context) string {
	if v, ok := ctx.Value(providerIDKey).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// WithModel returns a context carrying the model used for tool execution.
func WithModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, modelKey, strings.TrimSpace(model))
}

// GetModel extracts the model from the context.
func GetModel(ctx context.Context) string {
	if v, ok := ctx.Value(modelKey).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// WithAgentID returns a context carrying the agent identifier.
func WithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDKey, strings.TrimSpace(agentID))
}

// GetAgentID extracts the agent identifier from the context.
func GetAgentID(ctx context.Context) string {
	if v, ok := ctx.Value(agentIDKey).(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// WithRouteKind returns a context carrying the current execution route kind.
func WithRouteKind(ctx context.Context, kind ToolRouteKind) context.Context {
	if strings.TrimSpace(string(kind)) == "" {
		return ctx
	}
	return context.WithValue(ctx, routeKindKey, kind)
}

// GetRouteKind extracts the execution route kind from the context.
func GetRouteKind(ctx context.Context) ToolRouteKind {
	if v, ok := ctx.Value(routeKindKey).(ToolRouteKind); ok {
		return ToolRouteKind(strings.TrimSpace(string(v)))
	}
	if v, ok := ctx.Value(routeKindKey).(string); ok {
		return ToolRouteKind(strings.TrimSpace(v))
	}
	return ToolRouteKindUnknown
}

// WithRunID returns a context carrying the harness run ID.
func WithRunID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, runIDKey, id)
}

// GetRunID extracts the harness run ID from the context.
func GetRunID(ctx context.Context) string {
	if v, ok := ctx.Value(runIDKey).(string); ok {
		return v
	}
	return ""
}

// WithRunStep returns a context carrying the current harness step index.
func WithRunStep(ctx context.Context, step int) context.Context {
	return context.WithValue(ctx, runStepKey, step)
}

// GetRunStep extracts the harness step index from the context.
func GetRunStep(ctx context.Context) int {
	if v, ok := ctx.Value(runStepKey).(int); ok {
		return v
	}
	return 0
}

// WithAutoConfirm returns a context that auto-confirms approval-style flows.
func WithAutoConfirm(ctx context.Context, enabled bool) context.Context {
	if !enabled {
		return ctx
	}
	return context.WithValue(ctx, autoConfirmKey, true)
}

// GetAutoConfirm reports whether the current context should skip confirmations.
func GetAutoConfirm(ctx context.Context) bool {
	if v, ok := ctx.Value(autoConfirmKey).(bool); ok {
		return v
	}
	return false
}

// WithTurnID returns a context carrying the turn trace ID for metrics and replay.
func WithTurnID(ctx context.Context, turnID string) context.Context {
	return context.WithValue(ctx, turnIDKey, turnID)
}

// GetTurnID extracts the turn trace ID from the context.
func GetTurnID(ctx context.Context) string {
	if v, ok := ctx.Value(turnIDKey).(string); ok {
		return v
	}
	return ""
}

// TurnToolSummary captures per-tool-call data for turn-level metrics.
type TurnToolSummary struct {
	Name       string  `json:"name"`
	ToolCallID string  `json:"tool_call_id"`
	LatencyMs  float64 `json:"latency_ms"`
	Success    bool    `json:"success"`
}

// WithTurnToolCollector returns a context carrying a tool call collector for turn metrics.
func WithTurnToolCollector(ctx context.Context) context.Context {
	collector := make([]TurnToolSummary, 0, 8)
	return context.WithValue(ctx, turnToolsKey, &collector)
}

// AppendTurnTool records a tool call summary into the turn collector.
func AppendTurnTool(ctx context.Context, summary TurnToolSummary) {
	if collector, ok := ctx.Value(turnToolsKey).(*[]TurnToolSummary); ok && collector != nil {
		*collector = append(*collector, summary)
	}
}

// GetTurnToolSummaries returns the collected tool summaries for the current turn.
func GetTurnToolSummaries(ctx context.Context) []TurnToolSummary {
	if collector, ok := ctx.Value(turnToolsKey).(*[]TurnToolSummary); ok && collector != nil {
		out := make([]TurnToolSummary, len(*collector))
		copy(out, *collector)
		return out
	}
	return nil
}

// WithFSScope returns a context carrying additional filesystem roots and aliases
// for file tools (read/write/edit/grep/find/ls).
func WithFSScope(ctx context.Context, roots []string, aliases map[string]string) context.Context {
	return withFSScope(ctx, roots, aliases, false)
}

// WithFSRootOverride returns a context carrying a filesystem root override for
// file tools. Relative file paths resolve against these roots instead of the
// tool's default configured roots.
func WithFSRootOverride(ctx context.Context, roots []string, aliases map[string]string) context.Context {
	return withFSScope(ctx, roots, aliases, true)
}

// WithMergedFSScope merges additional filesystem roots and aliases into the
// existing FS scope while preserving whether the current scope replaces tool
// default roots or merely appends to them.
func WithMergedFSScope(ctx context.Context, roots []string, aliases map[string]string) context.Context {
	existing, ok := getFSScopeContext(ctx)
	if !ok {
		return WithFSScope(ctx, roots, aliases)
	}

	mergedRoots := make([]string, 0, len(existing.roots)+len(roots))
	seenRoots := make(map[string]struct{}, len(existing.roots)+len(roots))
	appendRoot := func(raw string) {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return
		}
		clean := filepath.Clean(abs)
		if _, exists := seenRoots[clean]; exists {
			return
		}
		seenRoots[clean] = struct{}{}
		mergedRoots = append(mergedRoots, clean)
	}
	for _, root := range existing.roots {
		appendRoot(root)
	}
	for _, root := range roots {
		appendRoot(root)
	}

	mergedAliases := make(map[string]string, len(existing.aliases)+len(aliases))
	appendAlias := func(rawAlias, rawPath string, overwrite bool) {
		alias := normalizeFSAliasKey(rawAlias)
		if alias == "" {
			return
		}
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" {
			return
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return
		}
		if _, exists := mergedAliases[alias]; exists && !overwrite {
			return
		}
		mergedAliases[alias] = filepath.Clean(abs)
	}
	for alias, path := range aliases {
		appendAlias(alias, path, false)
	}
	for alias, path := range existing.aliases {
		appendAlias(alias, path, true)
	}

	if len(mergedRoots) == 0 && len(mergedAliases) == 0 {
		return ctx
	}
	return withFSScope(ctx, mergedRoots, mergedAliases, existing.replaceRoots)
}

func withFSScope(ctx context.Context, roots []string, aliases map[string]string, replaceRoots bool) context.Context {
	normalizedRoots := make([]string, 0, len(roots))
	seenRoots := make(map[string]struct{}, len(roots))
	for _, raw := range roots {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		clean := filepath.Clean(abs)
		if _, ok := seenRoots[clean]; ok {
			continue
		}
		seenRoots[clean] = struct{}{}
		normalizedRoots = append(normalizedRoots, clean)
	}

	normalizedAliases := make(map[string]string, len(aliases))
	for rawAlias, rawPath := range aliases {
		alias := normalizeFSAliasKey(rawAlias)
		if alias == "" {
			continue
		}
		trimmed := strings.TrimSpace(rawPath)
		if trimmed == "" {
			continue
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		normalizedAliases[alias] = filepath.Clean(abs)
	}

	if len(normalizedRoots) == 0 && len(normalizedAliases) == 0 {
		return ctx
	}

	return context.WithValue(ctx, fsScopeKey, fsScopeContext{
		roots:        normalizedRoots,
		aliases:      normalizedAliases,
		replaceRoots: replaceRoots,
	})
}

// GetFSScope returns additional filesystem roots and aliases from the context.
func GetFSScope(ctx context.Context) (roots []string, aliases map[string]string) {
	v, ok := getFSScopeContext(ctx)
	if !ok {
		return nil, nil
	}

	if len(v.roots) > 0 {
		roots = make([]string, len(v.roots))
		copy(roots, v.roots)
	}
	if len(v.aliases) > 0 {
		aliases = make(map[string]string, len(v.aliases))
		for k, path := range v.aliases {
			aliases[k] = path
		}
	}
	return roots, aliases
}

func getFSScopeContext(ctx context.Context) (fsScopeContext, bool) {
	v, ok := ctx.Value(fsScopeKey).(fsScopeContext)
	if !ok {
		return fsScopeContext{}, false
	}
	return v, true
}

func normalizeFSAliasKey(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

// WithBrowserLaunchMode returns a context carrying a browser launch hint.
func WithBrowserLaunchMode(ctx context.Context, mode BrowserLaunchMode) context.Context {
	if mode == BrowserLaunchModeDefault {
		return ctx
	}
	return context.WithValue(ctx, browserModeKey, mode)
}

// GetBrowserLaunchMode extracts the browser launch hint from the context.
func GetBrowserLaunchMode(ctx context.Context) BrowserLaunchMode {
	if v, ok := ctx.Value(browserModeKey).(BrowserLaunchMode); ok {
		return v
	}
	if v, ok := ctx.Value(browserModeKey).(string); ok {
		return BrowserLaunchMode(v)
	}
	return BrowserLaunchModeDefault
}
