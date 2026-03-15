package chatcmd

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const modelsPageSize = 20

// CommandSpec defines a supported chat command.
type CommandSpec struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
}

// ParsedCommand is a normalized slash command.
type ParsedCommand struct {
	Raw     string
	Name    string
	Args    string
	Unknown bool
}

// CommandResult is a deterministic local command response.
type CommandResult struct {
	Content string
}

// CommandState is the persisted deterministic command state.
// For authenticated users, it is shared across their conversations.
type CommandState struct {
	ConversationID      string
	SelectedProviderID  string
	SelectedModelID     string
	Offline             bool
	WebSearchEnabled    bool
	DeepResearchEnabled bool
}

// LatestAssistantMessage captures the latest actual runtime route visible to users.
type LatestAssistantMessage struct {
	Provider string
	Model    string
}

// ProviderInfo describes a provider for command outputs.
type ProviderInfo struct {
	ID         string
	Name       string
	Enabled    bool
	Status     string
	Location   string
	BaseURL    string
	APIFormat  string
	ModelCount int
}

// ModelInfo describes a provider/model pair for command outputs.
type ModelInfo struct {
	ID          string
	ProviderID  string
	DisplayName string
	Enabled     bool
}

// Deps adapts the command core to Blue runtime state.
type Deps struct {
	GetConversationTitle    func(ctx context.Context, conversationID string) (string, error)
	UpdateConversationTitle func(ctx context.Context, conversationID, title string) error
	CountMessages           func(ctx context.Context, conversationID string) (int, error)
	GetLatestAssistant      func(ctx context.Context, conversationID string) (*LatestAssistantMessage, error)
	GetCommandState         func(ctx context.Context, conversationID string) (CommandState, error)
	SaveCommandState        func(ctx context.Context, state CommandState) error
	ResetConversation       func(ctx context.Context, conversationID string) (deleted int, err error)
	HasActiveStream         func(conversationID string) bool
	StopActiveStream        func(conversationID string) bool
	ListProviders           func(ctx context.Context) ([]ProviderInfo, error)
	ListModels              func(ctx context.Context) ([]ModelInfo, error)
}

// Executor executes deterministic chat commands.
type Executor struct {
	deps     Deps
	specs    []CommandSpec
	aliasMap map[string]string
}

// NewExecutor creates a new command executor.
func NewExecutor(deps Deps) *Executor {
	specs := DefaultSpecs()
	aliasMap := make(map[string]string, len(specs)*2)
	for _, spec := range specs {
		aliasMap[strings.ToLower(spec.Name)] = spec.Name
		for _, alias := range spec.Aliases {
			trimmed := strings.TrimSpace(strings.TrimPrefix(alias, "/"))
			if trimmed == "" {
				continue
			}
			aliasMap[strings.ToLower(trimmed)] = spec.Name
		}
	}
	return &Executor{deps: deps, specs: specs, aliasMap: aliasMap}
}

// DefaultSpecs returns the transport-neutral slash command catalog.
func DefaultSpecs() []CommandSpec {
	return []CommandSpec{
		{Name: "help", Aliases: []string{"/help", "/commands"}, Description: "Show available commands.", Usage: "/help"},
		{Name: "ping", Aliases: []string{"/ping"}, Description: "Health check.", Usage: "/ping"},
		{Name: "time", Aliases: []string{"/time"}, Description: "Show server time.", Usage: "/time"},
		{Name: "status", Aliases: []string{"/status"}, Description: "Show current conversation command state.", Usage: "/status"},
		{Name: "model", Aliases: []string{"/model"}, Description: "Inspect or select a model.", Usage: "/model [status|list|reset|auto|<model>|<provider/model>]"},
		{Name: "models", Aliases: []string{"/models"}, Description: "Browse models by provider.", Usage: "/models [provider] [page|all]"},
		{Name: "provider", Aliases: []string{"/provider"}, Description: "Inspect or pin a provider.", Usage: "/provider [status|reset|auto|<provider>]"},
		{Name: "providers", Aliases: []string{"/providers"}, Description: "List enabled providers.", Usage: "/providers"},
		{Name: "offline", Aliases: []string{"/offline"}, Description: "Toggle offline mode.", Usage: "/offline on|off|status"},
		{Name: "web", Aliases: []string{"/web"}, Description: "Toggle web search preference.", Usage: "/web on|off|status"},
		{Name: "deep", Aliases: []string{"/deep"}, Description: "Toggle deep research preference.", Usage: "/deep on|off|status"},
		{Name: "stop", Aliases: []string{"/stop"}, Description: "Stop the active stream for this conversation.", Usage: "/stop"},
		{Name: "clear", Aliases: []string{"/clear", "/reset", "/new"}, Description: "Clear conversation messages and command state.", Usage: "/clear"},
		{Name: "title", Aliases: []string{"/title", "/rename"}, Description: "Rename the conversation.", Usage: "/title <text>"},
	}
}

// Specs returns the supported command specs.
func (e *Executor) Specs() []CommandSpec {
	return append([]CommandSpec(nil), e.specs...)
}

// Parse parses a slash command using OpenClaw-style /cmd arg and /cmd: arg syntax.
func (e *Executor) Parse(message string) (*ParsedCommand, bool) {
	trimmed := strings.TrimSpace(message)
	if !strings.HasPrefix(trimmed, "/") {
		return nil, false
	}
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "/"))
	if body == "" {
		return nil, false
	}

	commandEnd := 0
	for commandEnd < len(body) {
		ch := body[commandEnd]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			commandEnd++
			continue
		}
		break
	}
	if commandEnd == 0 {
		return nil, false
	}

	rawName := strings.ToLower(strings.TrimSpace(body[:commandEnd]))
	remainder := strings.TrimSpace(body[commandEnd:])
	if strings.HasPrefix(remainder, ":") {
		remainder = strings.TrimSpace(strings.TrimPrefix(remainder, ":"))
	}

	canonical, ok := e.aliasMap[rawName]
	if !ok {
		canonical = rawName
	}
	return &ParsedCommand{Raw: trimmed, Name: canonical, Args: remainder, Unknown: !ok}, true
}

// Execute executes a deterministic slash command.
func (e *Executor) Execute(ctx context.Context, conversationID, message string) (CommandResult, bool) {
	parsed, ok := e.Parse(message)
	if !ok {
		return CommandResult{}, false
	}
	if parsed.Unknown {
		return CommandResult{Content: fmt.Sprintf("Unknown command: `/%s`. Use `/help`.", parsed.Name)}, true
	}

	switch parsed.Name {
	case "help":
		return CommandResult{Content: e.renderHelp()}, true
	case "ping":
		return CommandResult{Content: "pong"}, true
	case "time":
		return CommandResult{Content: "Server time: " + time.Now().Format(time.RFC3339)}, true
	case "status":
		return CommandResult{Content: e.renderStatus(ctx, conversationID)}, true
	case "model":
		return CommandResult{Content: e.executeModel(ctx, conversationID, parsed.Args)}, true
	case "models":
		return CommandResult{Content: e.executeModels(ctx, conversationID, parsed.Args)}, true
	case "provider":
		return CommandResult{Content: e.executeProvider(ctx, conversationID, parsed.Args)}, true
	case "providers":
		return CommandResult{Content: e.executeProviders(ctx)}, true
	case "offline":
		return CommandResult{Content: e.executeToggle(ctx, conversationID, "offline", parsed.Args)}, true
	case "web":
		return CommandResult{Content: e.executeToggle(ctx, conversationID, "web", parsed.Args)}, true
	case "deep":
		return CommandResult{Content: e.executeToggle(ctx, conversationID, "deep", parsed.Args)}, true
	case "stop":
		return CommandResult{Content: e.executeStop(conversationID)}, true
	case "clear":
		return CommandResult{Content: e.executeClear(ctx, conversationID)}, true
	case "title":
		return CommandResult{Content: e.executeTitle(ctx, conversationID, parsed.Args)}, true
	default:
		return CommandResult{Content: fmt.Sprintf("Unknown command: `/%s`. Use `/help`.", parsed.Name)}, true
	}
}

func (e *Executor) renderHelp() string {
	lines := []string{"Available commands:"}
	for _, spec := range e.specs {
		lines = append(lines, fmt.Sprintf("- `%s` — %s", spec.Usage, spec.Description))
	}
	return strings.Join(lines, "\n")
}

func (e *Executor) renderStatus(ctx context.Context, conversationID string) string {
	state, _ := e.deps.GetCommandState(ctx, conversationID)
	title, _ := e.deps.GetConversationTitle(ctx, conversationID)
	messageCount, _ := e.deps.CountMessages(ctx, conversationID)
	runtime, _ := e.deps.GetLatestAssistant(ctx, conversationID)
	activeStream := "OFF"
	if e.deps.HasActiveStream != nil && e.deps.HasActiveStream(conversationID) {
		activeStream = "ON"
	}
	selectedProvider := normalizeAuto(state.SelectedProviderID)
	selectedModel := normalizeAuto(state.SelectedModelID)
	runtimeLine := "auto"
	if runtime != nil && (runtime.Provider != "" || runtime.Model != "") {
		runtimeLine = strings.Trim(strings.TrimSpace(runtime.Provider+"/"+runtime.Model), "/")
	}
	lines := []string{
		"Conversation status:",
		fmt.Sprintf("- title: `%s`", title),
		fmt.Sprintf("- provider: `%s`", selectedProvider),
		fmt.Sprintf("- model: `%s`", selectedModel),
		fmt.Sprintf("- runtime: `%s`", runtimeLine),
		fmt.Sprintf("- offline: `%s`", onOff(state.Offline)),
		fmt.Sprintf("- web: `%s`", onOff(state.WebSearchEnabled)),
		fmt.Sprintf("- deep: `%s`", onOff(state.DeepResearchEnabled)),
		fmt.Sprintf("- active_stream: `%s`", activeStream),
		fmt.Sprintf("- messages: `%d`", messageCount),
	}
	return strings.Join(lines, "\n")
}

func (e *Executor) executeModel(ctx context.Context, conversationID, rawArgs string) string {
	state, err := e.deps.GetCommandState(ctx, conversationID)
	if err != nil {
		return "Failed to read conversation command state."
	}
	trimmed := strings.TrimSpace(rawArgs)
	if trimmed == "" {
		return e.renderModelSummary(ctx, conversationID, state)
	}
	lower := strings.ToLower(trimmed)
	if lower == "status" {
		return e.renderModelStatus(ctx, conversationID, state)
	}
	if lower == "list" {
		return e.executeModels(ctx, conversationID, "")
	}
	if lower == "auto" || lower == "reset" {
		state.SelectedModelID = ""
		if err := e.deps.SaveCommandState(ctx, state); err != nil {
			return "Failed to reset model selection."
		}
		return "Model selection reset to `auto`."
	}

	models, _ := e.deps.ListModels(ctx)
	providers, _ := e.deps.ListProviders(ctx)
	providerMap := make(map[string]ProviderInfo, len(providers))
	for _, provider := range providers {
		providerMap[strings.ToLower(provider.ID)] = provider
	}

	if strings.Contains(trimmed, "/") {
		providerID, modelID, ok := splitModelRef(trimmed)
		if !ok {
			return "Usage: `/model <provider/model>`."
		}
		if len(models) > 0 && !hasExactModel(models, providerID, modelID) {
			return fmt.Sprintf("Unknown model: `%s/%s`. Use `/models %s` to browse available models.", providerID, modelID, providerID)
		}
		if len(providerMap) > 0 {
			if _, ok := providerMap[strings.ToLower(providerID)]; !ok {
				return fmt.Sprintf("Unknown provider: `%s`. Use `/providers`.", providerID)
			}
		}
		state.SelectedProviderID = providerID
		state.SelectedModelID = modelID
		if err := e.deps.SaveCommandState(ctx, state); err != nil {
			return "Failed to update model selection."
		}
		return fmt.Sprintf("Model selection set to `%s/%s`.", providerID, modelID)
	}

	if state.SelectedProviderID != "" {
		matches := matchingModelsForProvider(models, state.SelectedProviderID, trimmed)
		if len(matches) == 1 {
			state.SelectedModelID = matches[0].ID
			if err := e.deps.SaveCommandState(ctx, state); err != nil {
				return "Failed to update model selection."
			}
			return fmt.Sprintf("Model selection set to `%s/%s`.", state.SelectedProviderID, matches[0].ID)
		}
		if len(matches) > 1 {
			return fmt.Sprintf("Ambiguous model `%s` within provider `%s`. Use `/model <provider/model>`.", trimmed, state.SelectedProviderID)
		}
	}

	global := matchingModels(models, trimmed)
	if len(global) == 1 {
		state.SelectedProviderID = global[0].ProviderID
		state.SelectedModelID = global[0].ID
		if err := e.deps.SaveCommandState(ctx, state); err != nil {
			return "Failed to update model selection."
		}
		return fmt.Sprintf("Model selection set to `%s/%s`.", global[0].ProviderID, global[0].ID)
	}
	if len(global) > 1 {
		refs := make([]string, 0, len(global))
		for _, match := range global {
			refs = append(refs, fmt.Sprintf("`%s/%s`", match.ProviderID, match.ID))
		}
		sort.Strings(refs)
		if len(refs) > 8 {
			refs = refs[:8]
		}
		return strings.Join([]string{
			fmt.Sprintf("Model `%s` is ambiguous.", trimmed),
			"Use an explicit provider/model reference:",
			strings.Join(refs, ", "),
		}, "\n")
	}
	if len(models) == 0 {
		state.SelectedModelID = trimmed
		if err := e.deps.SaveCommandState(ctx, state); err != nil {
			return "Failed to update model selection."
		}
		return fmt.Sprintf("Model selection set to `%s`.", trimmed)
	}
	return fmt.Sprintf("Unknown model `%s`. Use `/models` to browse available models.", trimmed)
}

func (e *Executor) renderModelSummary(ctx context.Context, conversationID string, state CommandState) string {
	runtime, _ := e.deps.GetLatestAssistant(ctx, conversationID)
	current := describeModelSelection(state)
	lines := []string{fmt.Sprintf("Current model selection: %s", current)}
	if runtime != nil && (runtime.Provider != "" || runtime.Model != "") {
		lines = append(lines, fmt.Sprintf("Runtime: %s", strings.Trim(strings.TrimSpace(runtime.Provider+"/"+runtime.Model), "/")))
	}
	lines = append(lines, "", "Switch: /model <provider/model>", "Browse: /models", "More: /model status")
	return strings.Join(lines, "\n")
}

func (e *Executor) renderModelStatus(ctx context.Context, conversationID string, state CommandState) string {
	providers, _ := e.deps.ListProviders(ctx)
	providerMap := make(map[string]ProviderInfo, len(providers))
	for _, provider := range providers {
		providerMap[strings.ToLower(provider.ID)] = provider
	}
	runtime, _ := e.deps.GetLatestAssistant(ctx, conversationID)
	selectedProvider := strings.TrimSpace(state.SelectedProviderID)
	selectedModel := strings.TrimSpace(state.SelectedModelID)
	targetProviderID := selectedProvider
	if targetProviderID == "" && runtime != nil {
		targetProviderID = strings.TrimSpace(runtime.Provider)
	}
	lines := []string{
		fmt.Sprintf("Selected provider: `%s`", normalizeAuto(selectedProvider)),
		fmt.Sprintf("Selected model: `%s`", normalizeAuto(selectedModel)),
		fmt.Sprintf("Offline: `%s`", onOff(state.Offline)),
		fmt.Sprintf("Web search: `%s`", onOff(state.WebSearchEnabled)),
		fmt.Sprintf("Deep research: `%s`", onOff(state.DeepResearchEnabled)),
	}
	if runtime != nil && (runtime.Provider != "" || runtime.Model != "") {
		lines = append(lines, fmt.Sprintf("Recent runtime: `%s`", strings.Trim(strings.TrimSpace(runtime.Provider+"/"+runtime.Model), "/")))
	}
	if targetProviderID != "" {
		if provider, ok := providerMap[strings.ToLower(targetProviderID)]; ok {
			lines = append(lines,
				fmt.Sprintf("Provider status: `%s`", provider.Status),
				fmt.Sprintf("Provider location: `%s`", provider.Location),
				fmt.Sprintf("Provider base_url: `%s`", blankToDefault(provider.BaseURL, "default")),
				fmt.Sprintf("Provider api_format: `%s`", blankToDefault(provider.APIFormat, "default")),
			)
		} else {
			lines = append(lines, fmt.Sprintf("Provider details unavailable for `%s`.", targetProviderID))
		}
	}
	return strings.Join(lines, "\n")
}

func (e *Executor) executeModels(ctx context.Context, _ string, rawArgs string) string {
	models, _ := e.deps.ListModels(ctx)
	providers, _ := e.deps.ListProviders(ctx)
	providerMeta := make(map[string]ProviderInfo, len(providers))
	for _, provider := range providers {
		providerMeta[strings.ToLower(provider.ID)] = provider
	}
	counts := make(map[string]int)
	for _, model := range models {
		counts[strings.ToLower(model.ProviderID)]++
	}
	providerArg, page, all := parseProviderListArgs(rawArgs)
	if providerArg == "" {
		providerIDs := make([]string, 0, len(counts)+len(providerMeta))
		seen := map[string]struct{}{}
		for providerID := range counts {
			providerIDs = append(providerIDs, providerID)
			seen[providerID] = struct{}{}
		}
		for providerID := range providerMeta {
			if _, ok := seen[providerID]; ok {
				continue
			}
			providerIDs = append(providerIDs, providerID)
		}
		sort.Strings(providerIDs)
		if len(providerIDs) == 0 {
			return "No models available."
		}
		lines := []string{"Providers:"}
		for _, providerID := range providerIDs {
			count := counts[providerID]
			if meta, ok := providerMeta[providerID]; ok {
				lines = append(lines, fmt.Sprintf("- %s (%d models)", meta.ID, count))
			} else {
				lines = append(lines, fmt.Sprintf("- %s (%d models)", providerID, count))
			}
		}
		lines = append(lines, "", "Use: /models <provider>", "Switch: /model <provider/model>")
		return strings.Join(lines, "\n")
	}

	providerID := strings.ToLower(providerArg)
	providerModels := make([]ModelInfo, 0)
	for _, model := range models {
		if strings.EqualFold(model.ProviderID, providerID) {
			providerModels = append(providerModels, model)
		}
	}
	if len(providerModels) == 0 {
		lines := []string{fmt.Sprintf("Unknown provider: %s", providerArg)}
		if len(providerMeta) > 0 {
			keys := make([]string, 0, len(providerMeta))
			for _, provider := range providers {
				keys = append(keys, provider.ID)
			}
			sort.Strings(keys)
			lines = append(lines, "", "Available providers:", strings.Join(keys, ", "))
		}
		return strings.Join(lines, "\n")
	}
	return renderProviderModels(providerArg, providerModels, page, all)
}

func (e *Executor) executeProvider(ctx context.Context, conversationID, rawArgs string) string {
	state, err := e.deps.GetCommandState(ctx, conversationID)
	if err != nil {
		return "Failed to read conversation command state."
	}
	providers, _ := e.deps.ListProviders(ctx)
	providerMap := make(map[string]ProviderInfo, len(providers))
	for _, provider := range providers {
		providerMap[strings.ToLower(provider.ID)] = provider
	}
	trimmed := strings.TrimSpace(rawArgs)
	if trimmed == "" {
		current := normalizeAuto(state.SelectedProviderID)
		return fmt.Sprintf("Current provider pin: `%s`\n\nUse `/provider <id>` to pin or `/provider status` for details.", current)
	}
	lower := strings.ToLower(trimmed)
	if lower == "status" {
		target := strings.TrimSpace(state.SelectedProviderID)
		if target == "" {
			return "Current provider pin: `auto`"
		}
		provider, ok := providerMap[strings.ToLower(target)]
		if !ok {
			return fmt.Sprintf("Current provider pin: `%s`\nProvider details are unavailable.", target)
		}
		lines := []string{
			fmt.Sprintf("Provider: `%s`", provider.ID),
			fmt.Sprintf("Status: `%s`", provider.Status),
			fmt.Sprintf("Location: `%s`", provider.Location),
			fmt.Sprintf("Base URL: `%s`", blankToDefault(provider.BaseURL, "default")),
			fmt.Sprintf("API format: `%s`", blankToDefault(provider.APIFormat, "default")),
			fmt.Sprintf("Models: `%d`", provider.ModelCount),
		}
		return strings.Join(lines, "\n")
	}
	if lower == "auto" || lower == "reset" {
		state.SelectedProviderID = ""
		if err := e.deps.SaveCommandState(ctx, state); err != nil {
			return "Failed to reset provider pin."
		}
		return "Provider pin reset to `auto`."
	}
	provider, ok := providerMap[strings.ToLower(trimmed)]
	if !ok {
		return fmt.Sprintf("Unknown provider `%s`. Use `/providers`.", trimmed)
	}
	state.SelectedProviderID = provider.ID
	if err := e.deps.SaveCommandState(ctx, state); err != nil {
		return "Failed to update provider pin."
	}
	if state.SelectedModelID != "" {
		return fmt.Sprintf("Provider pin set to `%s` with model `%s` unchanged.", provider.ID, state.SelectedModelID)
	}
	return fmt.Sprintf("Provider pin set to `%s`.", provider.ID)
}

func (e *Executor) executeProviders(ctx context.Context) string {
	providers, _ := e.deps.ListProviders(ctx)
	if len(providers) == 0 {
		return "No enabled providers available."
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].ID < providers[j].ID })
	lines := []string{"Enabled providers:"}
	for _, provider := range providers {
		lines = append(lines, fmt.Sprintf("- %s [%s, %s] (%d models)", provider.ID, blankToDefault(provider.Status, "unknown"), blankToDefault(provider.Location, "unknown"), provider.ModelCount))
	}
	return strings.Join(lines, "\n")
}

func (e *Executor) executeToggle(ctx context.Context, conversationID, kind, rawArgs string) string {
	state, err := e.deps.GetCommandState(ctx, conversationID)
	if err != nil {
		return "Failed to read conversation command state."
	}
	action := strings.ToLower(strings.TrimSpace(rawArgs))
	if action == "" {
		action = "status"
	}
	current := false
	label := ""
	switch kind {
	case "offline":
		current = state.Offline
		label = "Offline mode"
	case "web":
		current = state.WebSearchEnabled
		label = "Web search"
	case "deep":
		current = state.DeepResearchEnabled
		label = "Deep research"
	}
	set := func(value bool) string {
		switch kind {
		case "offline":
			state.Offline = value
		case "web":
			state.WebSearchEnabled = value
		case "deep":
			state.DeepResearchEnabled = value
		}
		if err := e.deps.SaveCommandState(ctx, state); err != nil {
			return fmt.Sprintf("Failed to update %s.", strings.ToLower(label))
		}
		return fmt.Sprintf("%s is now %s.", label, onOff(value))
	}
	if action == "on" {
		return set(true)
	}
	if action == "off" {
		return set(false)
	}
	return fmt.Sprintf("%s status: %s", label, onOff(current))
}

func (e *Executor) executeStop(conversationID string) string {
	if e.deps.HasActiveStream == nil || e.deps.StopActiveStream == nil {
		return "Stop is unavailable in this surface."
	}
	if !e.deps.HasActiveStream(conversationID) {
		return "No active stream for this conversation."
	}
	if !e.deps.StopActiveStream(conversationID) {
		return "Failed to stop the active stream."
	}
	return "Stopped the active stream."
}

func (e *Executor) executeClear(ctx context.Context, conversationID string) string {
	deleted, err := e.deps.ResetConversation(ctx, conversationID)
	if err != nil {
		return "Failed to clear conversation state."
	}
	return fmt.Sprintf("Conversation cleared. Removed %d messages and reset command state.", deleted)
}

func (e *Executor) executeTitle(ctx context.Context, conversationID, rawArgs string) string {
	title := strings.TrimSpace(rawArgs)
	if title == "" {
		return "Usage: /title <text>"
	}
	if err := e.deps.UpdateConversationTitle(ctx, conversationID, title); err != nil {
		return "Failed to update conversation title."
	}
	return fmt.Sprintf("Conversation title updated to `%s`.", title)
}

func normalizeAuto(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "auto"
	}
	return trimmed
}

func onOff(enabled bool) string {
	if enabled {
		return "ON"
	}
	return "OFF"
}

func blankToDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func describeModelSelection(state CommandState) string {
	provider := strings.TrimSpace(state.SelectedProviderID)
	model := strings.TrimSpace(state.SelectedModelID)
	switch {
	case provider != "" && model != "":
		return fmt.Sprintf("`%s/%s`", provider, model)
	case model != "":
		return fmt.Sprintf("`%s`", model)
	case provider != "":
		return fmt.Sprintf("provider `%s` (model auto)", provider)
	default:
		return "`auto`"
	}
}

func parseProviderListArgs(raw string) (provider string, page int, all bool) {
	page = 1
	tokens := strings.Fields(strings.TrimSpace(raw))
	if len(tokens) == 0 {
		return "", 1, false
	}
	provider = tokens[0]
	for _, token := range tokens[1:] {
		lower := strings.ToLower(token)
		if lower == "all" || lower == "--all" {
			all = true
			continue
		}
		if value, err := strconv.Atoi(lower); err == nil && value > 0 {
			page = value
		}
	}
	return provider, page, all
}

func renderProviderModels(providerID string, models []ModelInfo, page int, all bool) string {
	sort.Slice(models, func(i, j int) bool {
		if models[i].ID == models[j].ID {
			return models[i].DisplayName < models[j].DisplayName
		}
		return models[i].ID < models[j].ID
	})
	total := len(models)
	if all {
		page = 1
	}
	if page <= 0 {
		page = 1
	}
	pageCount := 1
	pageSize := total
	if !all {
		pageSize = modelsPageSize
		pageCount = (total + pageSize - 1) / pageSize
		if page > pageCount {
			page = pageCount
		}
	}
	start := 0
	end := total
	if !all {
		start = (page - 1) * pageSize
		end = start + pageSize
		if end > total {
			end = total
		}
	}
	lines := []string{fmt.Sprintf("Models (%s) — showing %d-%d of %d", providerID, start+1, end, total)}
	for _, model := range models[start:end] {
		lines = append(lines, fmt.Sprintf("- %s/%s", providerID, model.ID))
	}
	lines = append(lines, "", "Switch: /model <provider/model>")
	if !all && page < pageCount {
		lines = append(lines, fmt.Sprintf("More: /models %s %d", providerID, page+1))
		lines = append(lines, fmt.Sprintf("All: /models %s all", providerID))
	}
	return strings.Join(lines, "\n")
}

func splitModelRef(raw string) (providerID, modelID string, ok bool) {
	trimmed := strings.Trim(strings.TrimSpace(raw), "/")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	providerID = strings.TrimSpace(parts[0])
	modelID = strings.TrimSpace(parts[1])
	if providerID == "" || modelID == "" {
		return "", "", false
	}
	return providerID, modelID, true
}

func hasExactModel(models []ModelInfo, providerID, modelID string) bool {
	for _, model := range models {
		if strings.EqualFold(model.ProviderID, providerID) && strings.EqualFold(model.ID, modelID) {
			return true
		}
	}
	return false
}

func matchingModelsForProvider(models []ModelInfo, providerID, rawModel string) []ModelInfo {
	matches := make([]ModelInfo, 0)
	for _, model := range models {
		if !strings.EqualFold(model.ProviderID, providerID) {
			continue
		}
		if strings.EqualFold(model.ID, rawModel) || strings.EqualFold(model.DisplayName, rawModel) {
			matches = append(matches, model)
		}
	}
	return matches
}

func matchingModels(models []ModelInfo, rawModel string) []ModelInfo {
	matches := make([]ModelInfo, 0)
	for _, model := range models {
		if strings.EqualFold(model.ID, rawModel) || strings.EqualFold(model.DisplayName, rawModel) {
			matches = append(matches, model)
		}
	}
	return matches
}
