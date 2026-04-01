package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

const defaultRuntimeModel = "gpt-5.3-codex-spark"

func shouldUseDefaultRuntimeModel(pool *providerpool.Pool, modelID string) bool {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}
	if pool == nil || pool.Discovery == nil {
		return true
	}
	m, _, err := pool.Discovery.FindModel(modelID)
	return err == nil && m != nil && m.Enabled
}

func resolveDefaultRuntimeModel(model string, pool *providerpool.Pool) string {
	normalized := strings.TrimSpace(model)
	if normalized == "" {
		if shouldUseDefaultRuntimeModel(pool, defaultRuntimeModel) {
			return defaultRuntimeModel
		}
		return "auto"
	}
	if strings.EqualFold(normalized, "auto") {
		return "auto"
	}
	return model
}
