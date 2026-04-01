package deepresearch

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	deepResearchConcurrentSlotPollInterval = 100 * time.Millisecond

	deepResearchMaxConcurrentEnv       = "ZIMA_DEEP_RESEARCH_MAX_CONCURRENT_PER_USER"
	legacyDeepResearchMaxConcurrentEnv = "BLUE_DEEP_RESEARCH_MAX_CONCURRENT_PER_USER"
)

func resolveDefaultMaxConcurrentPerUser() int {
	for _, envName := range []string{deepResearchMaxConcurrentEnv, legacyDeepResearchMaxConcurrentEnv} {
		raw := strings.TrimSpace(os.Getenv(envName))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			continue
		}
		return value
	}
	return deepResearchMaxConcurrentPerUser
}
