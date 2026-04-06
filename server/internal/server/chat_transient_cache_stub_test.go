//go:build codex_targeted

package server

import (
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

const defaultConversationCacheMaxBytes = 32 * 1024 * 1024

type promptCacheToolSurfaceRef struct {
	Key       string
	ExpiresAt time.Time
	UpdatedAt time.Time
}

type promptCacheToolSurfaceSharedEntry struct {
	Surface   promptCacheToolSurface
	SizeBytes uint64
	UpdatedAt time.Time
	RefCount  int
}

func promptCacheToolSurfaceCacheKey(_ *promptCacheToolSurface) string {
	return ""
}

func (h *ChatHandler) runTransientCacheJanitor() {}

func (h *ChatHandler) removePromptCacheToolSurfaceRefLocked(_ string) {}

func estimateToolDefinitionsBytes(_ []tools.ToolDefinition) uint64 {
	return 0
}

func (h *ChatHandler) cleanupPromptCacheToolSurfacesLocked(_ time.Time) {}

func (h *ChatHandler) enforcePromptCacheToolSurfaceBudgetsLocked(_ time.Time) {}

func cloneCacheableMessages(messages []memory.Message) ([]memory.Message, uint64) {
	if len(messages) == 0 {
		return nil, 0
	}
	out := make([]memory.Message, len(messages))
	copy(out, messages)
	return out, 0
}

func estimateLLMMessagesBytes(_ []llm.Message) uint64 {
	return 0
}

func (h *ChatHandler) enforceWarmupBudgetLocked() {}

func (h *ChatHandler) deleteWarmupLocked(convID string) {
	if h == nil || h.warmupCache == nil {
		return
	}
	delete(h.warmupCache, convID)
}
