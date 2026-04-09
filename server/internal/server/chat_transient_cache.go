package server

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const (
	defaultConversationCacheMaxBytes       = 32 * 1024 * 1024
	warmupCacheMaxEntries                  = 16
	warmupCacheMaxBytes                    = 8 * 1024 * 1024
	promptCacheToolSurfaceMaxRefs          = 2048
	promptCacheToolSurfaceMaxSharedEntries = 128
	chatTransientCacheJanitorInterval      = 15 * time.Second
)

type promptCacheToolSurfaceRef struct {
	Key       string
	ExpiresAt time.Time
	UpdatedAt time.Time
}

type promptCacheToolSurfaceSharedEntry struct {
	Surface   promptCacheToolSurface
	RefCount  int
	SizeBytes uint64
	UpdatedAt time.Time
}

func (h *ChatHandler) runTransientCacheJanitor() {
	ticker := time.NewTicker(chatTransientCacheJanitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.eventStop:
			return
		case <-ticker.C:
			h.cleanupTransientCaches(timeutil.NowTime())
		}
	}
}

func (h *ChatHandler) cleanupTransientCaches(now time.Time) {
	if h == nil {
		return
	}
	current := timeutil.NowTime()
	if now.IsZero() || now.Before(current) {
		now = current
	}

	h.cleanupWarmupCache(now)
	h.cleanupProviderAffinities(now)
	h.cleanupPromptCacheToolSurfaces(now)
}

func (h *ChatHandler) cleanupWarmupCache(now time.Time) {
	h.warmupMu.Lock()
	defer h.warmupMu.Unlock()

	for convID, result := range h.warmupCache {
		if result == nil || now.Sub(result.createdAt) > warmupTTL {
			h.deleteWarmupLocked(convID)
		}
	}
	h.enforceWarmupBudgetLocked()
}

func (h *ChatHandler) enforceWarmupBudgetLocked() {
	for len(h.warmupCache) > warmupCacheMaxEntries || h.warmupCacheBytes > warmupCacheMaxBytes {
		oldestID := ""
		var oldestTime time.Time
		for convID, result := range h.warmupCache {
			if result == nil {
				oldestID = convID
				break
			}
			if oldestID == "" || result.createdAt.Before(oldestTime) {
				oldestID = convID
				oldestTime = result.createdAt
			}
		}
		if oldestID == "" {
			break
		}
		h.deleteWarmupLocked(oldestID)
	}
}

func (h *ChatHandler) deleteWarmupLocked(convID string) {
	result, ok := h.warmupCache[convID]
	if !ok {
		return
	}
	delete(h.warmupCache, convID)
	if result != nil {
		if result.sizeBytes == 0 {
			result.sizeBytes = estimateWarmupResultBytes(result)
		}
		if h.warmupCacheBytes >= result.sizeBytes {
			h.warmupCacheBytes -= result.sizeBytes
		} else {
			h.warmupCacheBytes = 0
		}
	}
}

func (h *ChatHandler) cleanupProviderAffinities(now time.Time) {
	h.providerAffinityMu.Lock()
	defer h.providerAffinityMu.Unlock()

	for convID, affinity := range h.providerAffinityMap {
		if affinity == nil || now.After(affinity.ExpiresAt) {
			delete(h.providerAffinityMap, convID)
		}
	}
}

func (h *ChatHandler) cleanupPromptCacheToolSurfaces(now time.Time) {
	h.promptCacheToolSurfaceMu.Lock()
	defer h.promptCacheToolSurfaceMu.Unlock()

	h.cleanupPromptCacheToolSurfacesLocked(now)
	h.enforcePromptCacheToolSurfaceBudgetsLocked(now)
}

func (h *ChatHandler) cleanupPromptCacheToolSurfacesLocked(now time.Time) {
	for convID, ref := range h.promptCacheToolSurfaceMap {
		if ref == nil || now.After(ref.ExpiresAt) {
			h.removePromptCacheToolSurfaceRefLocked(convID)
		}
	}
	for key, shared := range h.promptCacheToolSurfaceShared {
		if shared == nil {
			delete(h.promptCacheToolSurfaceShared, key)
			continue
		}
		if shared.RefCount <= 0 && now.After(shared.Surface.ExpiresAt) {
			delete(h.promptCacheToolSurfaceShared, key)
		}
	}
}

func (h *ChatHandler) enforcePromptCacheToolSurfaceBudgetsLocked(now time.Time) {
	for len(h.promptCacheToolSurfaceMap) > promptCacheToolSurfaceMaxRefs {
		oldestID := h.oldestPromptCacheToolSurfaceRefLocked()
		if oldestID == "" {
			break
		}
		h.removePromptCacheToolSurfaceRefLocked(oldestID)
	}

	for len(h.promptCacheToolSurfaceShared) > promptCacheToolSurfaceMaxSharedEntries {
		if sharedKey := h.oldestPromptCacheToolSurfaceSharedKeyLocked(true); sharedKey != "" {
			delete(h.promptCacheToolSurfaceShared, sharedKey)
			continue
		}
		oldestID := h.oldestPromptCacheToolSurfaceRefLocked()
		if oldestID == "" {
			break
		}
		h.removePromptCacheToolSurfaceRefLocked(oldestID)
		h.cleanupPromptCacheToolSurfacesLocked(now)
	}
}

func (h *ChatHandler) oldestPromptCacheToolSurfaceRefLocked() string {
	oldestID := ""
	var oldestTime time.Time
	for convID, ref := range h.promptCacheToolSurfaceMap {
		if ref == nil {
			return convID
		}
		if oldestID == "" || ref.UpdatedAt.Before(oldestTime) {
			oldestID = convID
			oldestTime = ref.UpdatedAt
		}
	}
	return oldestID
}

func (h *ChatHandler) oldestPromptCacheToolSurfaceSharedKeyLocked(evictableOnly bool) string {
	oldestKey := ""
	var oldestTime time.Time
	for key, shared := range h.promptCacheToolSurfaceShared {
		if shared == nil {
			return key
		}
		if evictableOnly && shared.RefCount > 0 {
			continue
		}
		if oldestKey == "" || shared.UpdatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = shared.UpdatedAt
		}
	}
	return oldestKey
}

func (h *ChatHandler) removePromptCacheToolSurfaceRefLocked(convID string) {
	ref, ok := h.promptCacheToolSurfaceMap[convID]
	if !ok {
		return
	}
	delete(h.promptCacheToolSurfaceMap, convID)
	if ref == nil {
		return
	}
	shared := h.promptCacheToolSurfaceShared[ref.Key]
	if shared == nil {
		return
	}
	if shared.RefCount > 0 {
		shared.RefCount--
	}
	if shared.RefCount <= 0 {
		shared.RefCount = 0
		if timeutil.NowTime().After(shared.Surface.ExpiresAt) {
			delete(h.promptCacheToolSurfaceShared, ref.Key)
		}
	}
}

func promptCacheToolSurfaceCacheKey(surface *promptCacheToolSurface) string {
	if surface == nil {
		return ""
	}
	return strings.Join([]string{
		strings.TrimSpace(surface.ProviderID),
		uint64Key(surface.RegistryVersion),
		strings.TrimSpace(surface.PromptPolicyHash),
		strings.TrimSpace(surface.WebSearchEnabled),
		strings.TrimSpace(surface.ResearchEnabled),
	}, "\x00")
}

func uint64Key(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func estimateWarmupResultBytes(result *warmupResult) uint64 {
	if result == nil {
		return 0
	}
	return estimateLLMMessagesBytes(result.systemPromptMessages) + estimateMessagesBytes(result.preloadedMessages)
}

func estimateLLMMessagesBytes(messages []llm.Message) uint64 {
	var total uint64
	for _, msg := range messages {
		total += uint64(len(msg.Role) + len(msg.Content) + len(msg.ToolCallID) + len(msg.ToolName))
		for _, part := range msg.ContentParts {
			total += uint64(len(part.Type) + len(part.Text) + len(part.MediaType) + len(part.Data))
		}
		for _, call := range msg.ToolCalls {
			total += uint64(len(call.ID) + len(call.Name) + len(call.Arguments))
		}
	}
	return total
}

func cloneCacheableMessages(messages []memory.Message) ([]memory.Message, uint64) {
	if len(messages) == 0 {
		return nil, 0
	}
	out := make([]memory.Message, len(messages))
	var total uint64
	for i, msg := range messages {
		cloned, sizeBytes := cloneCacheableMessage(msg)
		out[i] = cloned
		total += sizeBytes
	}
	return out, total
}

func cloneCacheableMessage(msg memory.Message) (memory.Message, uint64) {
	out := memory.Message{
		Role:       msg.Role,
		Content:    msg.Content,
		ToolCallID: msg.ToolCallID,
		ToolName:   msg.ToolName,
	}
	sizeBytes := uint64(len(out.Role) + len(out.Content) + len(out.ToolCallID) + len(out.ToolName))
	if len(msg.ToolCalls) > 0 {
		out.ToolCalls = make([]memory.ToolCall, len(msg.ToolCalls))
		copy(out.ToolCalls, msg.ToolCalls)
		for _, call := range msg.ToolCalls {
			sizeBytes += uint64(len(call.ID) + len(call.Name) + len(call.Arguments))
		}
	}
	if len(msg.Attachments) > 0 {
		out.Attachments = make([]memory.MessageAttachment, len(msg.Attachments))
		copy(out.Attachments, msg.Attachments)
		for _, att := range msg.Attachments {
			sizeBytes += uint64(len(att.Type) + len(att.Name) + len(att.MimeType) + len(att.Data))
		}
	}
	return out, sizeBytes
}

func estimateToolDefinitionsBytes(defs []tools.ToolDefinition) uint64 {
	if len(defs) == 0 {
		return 0
	}
	body, err := json.Marshal(defs)
	if err == nil {
		return uint64(len(body))
	}

	var total uint64
	for _, def := range defs {
		total += uint64(len(def.Name) + len(def.Description) + len(def.Icon) + len(def.RiskLevel))
		for _, value := range def.Aliases {
			total += uint64(len(value))
		}
		for _, value := range def.SearchHints {
			total += uint64(len(value))
		}
		for _, value := range def.VisibilityAllowlist {
			total += uint64(len(value))
		}
	}
	return total
}
