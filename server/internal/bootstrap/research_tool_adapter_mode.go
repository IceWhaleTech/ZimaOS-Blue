package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
)

func toolResearchModeAndDepth(mode deepresearch.Mode) (string, string) {
	raw := strings.ToLower(strings.TrimSpace(string(mode)))
	switch raw {
	case "analyze", "ui_review":
		return raw, ""
	case "", "deep_research":
		return "deep_research", ""
	default:
		return "deep_research", raw
	}
}
