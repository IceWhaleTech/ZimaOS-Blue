package harness

import "strings"

type discoverFirstObservation struct {
	SelectedCanonicalSkill            string
	NativeSurfaceMode                 string
	NativeSurfaceReason               string
	ExecutionProfile                  string
	ToolSurfaceAliasRewriteCount      int
	ToolSurfaceCacheInvalidationCount int
	ToolSurfaceExecCutoverCount       int
}

func decodeDiscoverFirstObservation(structured map[string]interface{}) discoverFirstObservation {
	discoveryRuntime := nestedMetadataMap(structured, "discovery_runtime")
	discoveryDecision := nestedMetadataMap(structured, "discovery_decision")
	skillDecision := nestedMetadataMap(structured, "skill_decision")

	return discoverFirstObservation{
		SelectedCanonicalSkill: normalizeDiscoverFirstCanonicalSkill(firstNonEmpty(
			metadataString(structured, "selected_canonical_skill"),
			metadataString(structured, "canonical_skill_id"),
			metadataString(discoveryRuntime, "canonical_target"),
			metadataString(discoveryDecision, "canonical_target"),
			metadataString(skillDecision, "selected_skill"),
		)),
		NativeSurfaceMode: normalizeDiscoverFirstNativeSurfaceMode(firstNonEmpty(
			metadataString(structured, "selected_native_surface_mode"),
			metadataString(discoveryRuntime, "selected_native_mode"),
			metadataString(discoveryRuntime, "native_surface_mode"),
			metadataString(discoveryDecision, "native_surface_mode"),
		)),
		NativeSurfaceReason: strings.TrimSpace(firstNonEmpty(
			metadataString(structured, "selected_native_surface_reason"),
			metadataString(discoveryRuntime, "surface_reason"),
		)),
		ExecutionProfile: normalizeDiscoverFirstExecutionProfile(firstNonEmpty(
			metadataString(structured, "execution_profile"),
			metadataString(discoveryRuntime, "execution_profile"),
			metadataString(discoveryDecision, "execution_profile"),
		)),
		ToolSurfaceAliasRewriteCount:      firstDiscoverFirstAuditCount("tool_surface_alias_rewrite_count", structured, discoveryRuntime),
		ToolSurfaceCacheInvalidationCount: firstDiscoverFirstAuditCount("tool_surface_cache_invalidation_count", structured, discoveryRuntime),
		ToolSurfaceExecCutoverCount: firstNonZeroDiscoverFirstAuditCount(
			firstDiscoverFirstAuditCount("tool_surface_exec_cutover_count", structured, discoveryRuntime),
			boolToDiscoverFirstAuditCount(
				metadataBoolValue(structured, "skill_exec_cutover") ||
					metadataBoolValue(discoveryRuntime, "skill_exec_cutover"),
			),
		),
	}
}

func firstDiscoverFirstAuditCount(key string, sources ...map[string]interface{}) int {
	key = strings.TrimSpace(key)
	if key == "" {
		return 0
	}
	for _, source := range sources {
		if len(source) == 0 {
			continue
		}
		if value := intMetadataPtr(source[key]); value != nil {
			return *value
		}
	}
	return 0
}

func firstNonZeroDiscoverFirstAuditCount(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func boolToDiscoverFirstAuditCount(value bool) int {
	if value {
		return 1
	}
	return 0
}

func accumulateDiscoverFirstAuditCounts(aliasRewriteCount, cacheInvalidationCount, execCutoverCount *int, observation discoverFirstObservation) {
	if aliasRewriteCount != nil {
		*aliasRewriteCount += observation.ToolSurfaceAliasRewriteCount
	}
	if cacheInvalidationCount != nil {
		*cacheInvalidationCount += observation.ToolSurfaceCacheInvalidationCount
	}
	if execCutoverCount != nil {
		*execCutoverCount += observation.ToolSurfaceExecCutoverCount
	}
}

func normalizeDiscoverFirstCanonicalSkill(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unknown":
		return ""
	case "web-search", "web_search", "search", "web_fetch", "web-read", "web_read", "web_extract", "web_crawl":
		return "web_query"
	case "deep-research", "research":
		return "deep_research"
	default:
		return strings.TrimSpace(value)
	}
}

func normalizeDiscoverFirstNativeSurfaceMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unknown":
		return ""
	case "legacy_native":
		return "legacy"
	default:
		return strings.TrimSpace(value)
	}
}

func normalizeDiscoverFirstExecutionProfile(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unknown":
		return ""
	default:
		return strings.TrimSpace(value)
	}
}

func incrementBreakdownValue(dest *map[string]int, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	if *dest == nil {
		*dest = make(map[string]int)
	}
	(*dest)[value]++
}
