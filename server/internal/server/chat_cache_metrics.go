package server

// ChatCacheFootprint reports current cache residency for long-lived chat state.
type ChatCacheFootprint struct {
	ConversationCacheEntries       int
	ConversationCacheBytes         uint64
	WarmupCacheEntries             int
	WarmupCacheBytes               uint64
	PromptToolSurfaceRefs          int
	PromptToolSurfaceSharedEntries int
	PromptToolSurfaceSharedBytes   uint64
	ProviderAffinityEntries        int
}

// ChatCacheFootprintProvider exposes runtime chat cache metrics to other surfaces.
type ChatCacheFootprintProvider interface {
	ChatCacheFootprint() ChatCacheFootprint
}

func (h *ChatHandler) ChatCacheFootprint() ChatCacheFootprint {
	if h == nil {
		return ChatCacheFootprint{}
	}
	footprint := ChatCacheFootprint{}
	if h.conversationCache != nil {
		stats := h.conversationCache.Stats()
		footprint.ConversationCacheEntries = stats.Size
		footprint.ConversationCacheBytes = stats.Bytes
	}

	h.warmupMu.Lock()
	footprint.WarmupCacheEntries = len(h.warmupCache)
	footprint.WarmupCacheBytes = h.warmupCacheBytes
	h.warmupMu.Unlock()

	h.providerAffinityMu.Lock()
	footprint.ProviderAffinityEntries = len(h.providerAffinityMap)
	h.providerAffinityMu.Unlock()

	h.promptCacheToolSurfaceMu.Lock()
	footprint.PromptToolSurfaceRefs = len(h.promptCacheToolSurfaceMap)
	footprint.PromptToolSurfaceSharedEntries = len(h.promptCacheToolSurfaceShared)
	for _, shared := range h.promptCacheToolSurfaceShared {
		if shared == nil {
			continue
		}
		sizeBytes := shared.SizeBytes
		if sizeBytes == 0 {
			sizeBytes = estimateToolDefinitionsBytes(shared.Surface.Tools)
		}
		footprint.PromptToolSurfaceSharedBytes += sizeBytes
	}
	h.promptCacheToolSurfaceMu.Unlock()

	return footprint
}
