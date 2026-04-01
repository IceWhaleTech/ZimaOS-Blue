package proxy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	stdjson "encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

type promptCacheSnapshotContextKey struct{}

const (
	promptCacheBreakMinReadDrop  = 2000
	promptCacheBreakDefaultLimit = 32
)

type PromptCacheStateSnapshot struct {
	TrackingKey      string
	Provider         string
	Model            string
	Path             string
	SystemHash       string
	CacheControlHash string
	ToolsHash        string
	ExtraBodyHash    string
	SystemBlockCount int
	ToolCount        int
	ToolNames        []string
	PerToolHashes    map[string]string
}

type PromptCacheBreakObservation struct {
	Reasons []string
	Summary string
}

type promptCacheTrackedState struct {
	Snapshot        PromptCacheStateSnapshot
	CacheReadTokens int
}

type PromptCacheBreakDetector struct {
	mu         sync.Mutex
	maxTracked int
	order      []string
	states     map[string]promptCacheTrackedState
}

func NewPromptCacheBreakDetector(maxTracked int) *PromptCacheBreakDetector {
	if maxTracked <= 0 {
		maxTracked = promptCacheBreakDefaultLimit
	}
	return &PromptCacheBreakDetector{
		maxTracked: maxTracked,
		states:     make(map[string]promptCacheTrackedState),
	}
}

func (d *PromptCacheBreakDetector) Observe(snapshot PromptCacheStateSnapshot, cacheReadTokens int) *PromptCacheBreakObservation {
	if d == nil || snapshot.TrackingKey == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	prev, ok := d.states[snapshot.TrackingKey]
	d.recordLocked(snapshot, cacheReadTokens)
	if !ok {
		return nil
	}
	if prev.CacheReadTokens <= 0 || cacheReadTokens >= prev.CacheReadTokens {
		return nil
	}
	if prev.CacheReadTokens-cacheReadTokens < promptCacheBreakMinReadDrop {
		return nil
	}

	reasons, summary := diffPromptCacheSnapshots(prev.Snapshot, snapshot)
	if len(reasons) == 0 {
		return nil
	}
	return &PromptCacheBreakObservation{Reasons: reasons, Summary: summary}
}

func (d *PromptCacheBreakDetector) recordLocked(snapshot PromptCacheStateSnapshot, cacheReadTokens int) {
	if _, exists := d.states[snapshot.TrackingKey]; !exists {
		d.order = append(d.order, snapshot.TrackingKey)
	}
	d.states[snapshot.TrackingKey] = promptCacheTrackedState{
		Snapshot:        snapshot,
		CacheReadTokens: cacheReadTokens,
	}
	for len(d.order) > d.maxTracked {
		oldest := d.order[0]
		d.order = d.order[1:]
		delete(d.states, oldest)
	}
}

func BuildPromptCacheStateSnapshot(body []byte, provider, fallbackModel, path string) PromptCacheStateSnapshot {
	provider = strings.TrimSpace(provider)
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	if model == "" {
		model = strings.TrimSpace(fallbackModel)
	}

	systemTexts := promptCacheStableSystemTexts(body)
	toolEntries := promptCacheToolEntries(body)
	snapshot := PromptCacheStateSnapshot{
		Provider:         provider,
		Model:            model,
		Path:             strings.TrimSpace(path),
		SystemHash:       hashStringList(systemTexts),
		CacheControlHash: promptCacheControlHash(body),
		ToolsHash:        promptCacheToolsHash(toolEntries),
		ExtraBodyHash:    promptCacheExtraBodyHash(body),
		SystemBlockCount: len(systemTexts),
		ToolCount:        len(toolEntries),
		ToolNames:        promptCacheToolNames(toolEntries),
		PerToolHashes:    promptCachePerToolHashes(toolEntries),
	}
	snapshot.TrackingKey = buildPromptCacheTrackingKey(snapshot)
	return snapshot
}

type promptCacheToolEntry struct {
	Name       string
	Normalized string
	Hash       string
}

func promptCacheStableSystemTexts(body []byte) []string {
	system := gjson.GetBytes(body, "system")
	if system.Exists() {
		if system.IsArray() {
			cached := make([]string, 0, len(system.Array()))
			all := make([]string, 0, len(system.Array()))
			for _, item := range system.Array() {
				text := strings.TrimSpace(item.Get("text").String())
				if text == "" {
					continue
				}
				all = append(all, text)
				if item.Get("cache_control").Exists() {
					cached = append(cached, text)
				}
			}
			if len(cached) > 0 {
				return cached
			}
			return all
		}
		if text := strings.TrimSpace(system.String()); text != "" {
			return []string{text}
		}
	}

	messages := gjson.GetBytes(body, "messages")
	if messages.Exists() && messages.IsArray() {
		var out []string
		for _, msg := range messages.Array() {
			role := strings.ToLower(strings.TrimSpace(msg.Get("role").String()))
			if role != "system" && role != "developer" {
				continue
			}
			if text := strings.TrimSpace(msg.Get("content").String()); text != "" {
				out = append(out, text)
				continue
			}
			content := msg.Get("content")
			if !content.Exists() || !content.IsArray() {
				continue
			}
			for _, block := range content.Array() {
				text := strings.TrimSpace(block.Get("text").String())
				if text != "" {
					out = append(out, text)
				}
			}
		}
		return out
	}

	return nil
}

func promptCacheControlHash(body []byte) string {
	payload := map[string]any{}
	if system := promptCacheSystemControlShape(body); len(system) > 0 {
		payload["system"] = system
	}
	if tools := promptCacheToolsControlShape(body); len(tools) > 0 {
		payload["tools"] = tools
	}
	if messages := promptCacheMessagesControlShape(body); len(messages) > 0 {
		payload["messages"] = messages
	}
	return hashJSONValue(payload)
}

func promptCacheSystemControlShape(body []byte) []map[string]any {
	system := gjson.GetBytes(body, "system")
	if !system.Exists() || !system.IsArray() {
		return nil
	}
	out := make([]map[string]any, 0, len(system.Array()))
	for _, item := range system.Array() {
		if !item.Get("cache_control").Exists() {
			continue
		}
		entry := map[string]any{
			"type": strings.TrimSpace(item.Get("type").String()),
		}
		if cc, ok := parsePromptCacheRawJSON(item.Get("cache_control").Raw); ok {
			entry["cache_control"] = cc
		}
		out = append(out, entry)
	}
	return out
}

func promptCacheToolsControlShape(body []byte) []map[string]any {
	tools := gjson.GetBytes(body, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}
	out := make([]map[string]any, 0, len(tools.Array()))
	for _, tool := range tools.Array() {
		if !tool.Get("cache_control").Exists() {
			continue
		}
		entry := map[string]any{
			"name": promptCacheToolName(tool, 0),
		}
		if cc, ok := parsePromptCacheRawJSON(tool.Get("cache_control").Raw); ok {
			entry["cache_control"] = cc
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return fmt.Sprint(out[i]["name"]) < fmt.Sprint(out[j]["name"])
	})
	return out
}

func promptCacheMessagesControlShape(body []byte) []map[string]any {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return nil
	}
	out := make([]map[string]any, 0, len(messages.Array()))
	for _, msg := range messages.Array() {
		content := msg.Get("content")
		if !content.Exists() || !content.IsArray() {
			continue
		}
		blocks := make([]map[string]any, 0, len(content.Array()))
		for _, block := range content.Array() {
			if !block.Get("cache_control").Exists() {
				continue
			}
			entry := map[string]any{
				"type": strings.TrimSpace(block.Get("type").String()),
			}
			if cc, ok := parsePromptCacheRawJSON(block.Get("cache_control").Raw); ok {
				entry["cache_control"] = cc
			}
			blocks = append(blocks, entry)
		}
		if len(blocks) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"role":   strings.ToLower(strings.TrimSpace(msg.Get("role").String())),
			"blocks": blocks,
		})
	}
	return out
}

func promptCacheToolEntries(body []byte) []promptCacheToolEntry {
	tools := gjson.GetBytes(body, "tools")
	if !tools.Exists() || !tools.IsArray() {
		return nil
	}
	out := make([]promptCacheToolEntry, 0, len(tools.Array()))
	for idx, tool := range tools.Array() {
		name := promptCacheToolName(tool, idx)
		normalized := normalizePromptCacheToolRaw(tool.Raw)
		out = append(out, promptCacheToolEntry{
			Name:       name,
			Normalized: normalized,
			Hash:       hashString(normalized),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Normalized < out[j].Normalized
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func promptCacheToolName(tool gjson.Result, idx int) string {
	for _, path := range []string{"name", "function.name"} {
		if name := strings.TrimSpace(tool.Get(path).String()); name != "" {
			return name
		}
	}
	return fmt.Sprintf("__tool_%d__", idx)
}

func normalizePromptCacheToolRaw(raw string) string {
	var value any
	if err := stdjson.Unmarshal([]byte(raw), &value); err != nil {
		return strings.TrimSpace(raw)
	}
	value = stripPromptCacheKeys(value, "cache_control")
	return canonicalPromptCacheJSON(value)
}

func promptCacheToolsHash(entries []promptCacheToolEntry) string {
	if len(entries) == 0 {
		return ""
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts = append(parts, entry.Name+":"+entry.Normalized)
	}
	return hashStringList(parts)
}

func promptCacheToolNames(entries []promptCacheToolEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name)
	}
	return names
}

func promptCachePerToolHashes(entries []promptCacheToolEntry) map[string]string {
	if len(entries) == 0 {
		return nil
	}
	out := make(map[string]string, len(entries))
	for _, entry := range entries {
		out[entry.Name] = entry.Hash
	}
	return out
}

func promptCacheExtraBodyHash(body []byte) string {
	var value map[string]any
	if err := stdjson.Unmarshal(body, &value); err != nil {
		return ""
	}
	for _, key := range []string{"model", "messages", "system", "tools", "prompt_cache_key", "input", "instructions", "previous_response_id"} {
		delete(value, key)
	}
	return hashJSONValue(value)
}

func buildPromptCacheTrackingKey(snapshot PromptCacheStateSnapshot) string {
	if snapshot.Provider == "" || snapshot.Path == "" {
		return ""
	}
	return strings.Join([]string{
		snapshot.Provider,
		snapshot.Path,
	}, "|")
}

func diffPromptCacheSnapshots(prev, curr PromptCacheStateSnapshot) ([]string, string) {
	reasons := make([]string, 0, 4)
	summary := make([]string, 0, 4)

	if prev.SystemHash != curr.SystemHash {
		reasons = append(reasons, "system_prompt_changed")
		summary = append(summary, fmt.Sprintf("system prompt hash %s -> %s", shortPromptCacheHash(prev.SystemHash), shortPromptCacheHash(curr.SystemHash)))
	}
	if prev.CacheControlHash != curr.CacheControlHash {
		reasons = append(reasons, "cache_control_changed")
		summary = append(summary, fmt.Sprintf("cache-control layout %s -> %s", shortPromptCacheHash(prev.CacheControlHash), shortPromptCacheHash(curr.CacheControlHash)))
	}
	if prev.ToolsHash != curr.ToolsHash {
		reasons = append(reasons, "tool_schemas_changed")
		if changed := diffPromptCacheToolNames(prev.PerToolHashes, curr.PerToolHashes); len(changed) > 0 {
			summary = append(summary, "tool schema changed: "+strings.Join(changed, ", "))
		} else {
			summary = append(summary, fmt.Sprintf("tool schema hash %s -> %s", shortPromptCacheHash(prev.ToolsHash), shortPromptCacheHash(curr.ToolsHash)))
		}
	}
	if prev.Model != curr.Model {
		reasons = append(reasons, "model_changed")
		summary = append(summary, fmt.Sprintf("model %s -> %s", prev.Model, curr.Model))
	}
	if prev.ExtraBodyHash != curr.ExtraBodyHash {
		reasons = append(reasons, "extra_body_changed")
		summary = append(summary, fmt.Sprintf("extra body hash %s -> %s", shortPromptCacheHash(prev.ExtraBodyHash), shortPromptCacheHash(curr.ExtraBodyHash)))
	}

	return reasons, strings.Join(summary, "; ")
}

func diffPromptCacheToolNames(prev, curr map[string]string) []string {
	if len(prev) == 0 && len(curr) == 0 {
		return nil
	}
	nameSet := make(map[string]struct{}, len(prev)+len(curr))
	for name := range prev {
		nameSet[name] = struct{}{}
	}
	for name := range curr {
		nameSet[name] = struct{}{}
	}
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		switch {
		case prev[name] == "":
			names = append(names, name+"(added)")
		case curr[name] == "":
			names = append(names, name+"(removed)")
		case prev[name] != curr[name]:
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func parsePromptCacheRawJSON(raw string) (any, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	var value any
	if err := stdjson.Unmarshal([]byte(raw), &value); err != nil {
		return nil, false
	}
	return value, true
}

func stripPromptCacheKeys(value any, keys ...string) any {
	if len(keys) == 0 {
		return value
	}
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	return stripPromptCacheKeysSet(value, keySet)
}

func stripPromptCacheKeysSet(value any, keySet map[string]struct{}) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, raw := range typed {
			if _, skip := keySet[key]; skip {
				continue
			}
			out[key] = stripPromptCacheKeysSet(raw, keySet)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, raw := range typed {
			out[i] = stripPromptCacheKeysSet(raw, keySet)
		}
		return out
	default:
		return value
	}
}

func canonicalPromptCacheJSON(value any) string {
	raw, err := stdjson.Marshal(value)
	if err != nil {
		return ""
	}
	return string(raw)
}

func hashStringList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	h := sha256.New()
	for _, value := range values {
		h.Write([]byte(value))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func hashJSONValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case map[string]any:
		if len(typed) == 0 {
			return ""
		}
	case []map[string]any:
		if len(typed) == 0 {
			return ""
		}
	}
	raw, err := stdjson.Marshal(value)
	if err != nil || len(raw) == 0 || string(raw) == "{}" || string(raw) == "[]" {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func hashString(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func shortPromptCacheHash(value string) string {
	if len(value) <= 8 {
		return value
	}
	return value[:8]
}

func withPromptCacheSnapshot(ctx context.Context, snapshot PromptCacheStateSnapshot) context.Context {
	return context.WithValue(ctx, promptCacheSnapshotContextKey{}, snapshot)
}

func promptCacheSnapshotFromContext(ctx context.Context) (PromptCacheStateSnapshot, bool) {
	if ctx == nil {
		return PromptCacheStateSnapshot{}, false
	}
	snapshot, ok := ctx.Value(promptCacheSnapshotContextKey{}).(PromptCacheStateSnapshot)
	return snapshot, ok
}
