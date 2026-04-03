package harness

import (
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
)

const (
	Batch1ExecutionDatasetName        = "skill-exec-batch1"
	Batch1ExecutionDatasetDescription = "Curated batch-1 execution equivalence dataset for tool-to-skill migration."
	Batch1ExecutionDatasetSubject     = "skill_execution_batch1"
	Batch1ExecutionDatasetVersion     = "skill-exec-batch1-v6"
	Batch1ExecutionEvalName           = "Skill Execution Batch 1"
	batch1ExecutionMaxConcurrency     = 4
	batch1ExecutionMaxAttempts        = 3
	batch1ExecutionRetryBackoff       = 20 * time.Second

	batch1ExecutionProfile         = "skill_execution_batch1"
	batch1ExecutionPolicyModelHint = "claude-sonnet-4-6"
)

var batch1ExecutionSkills = []string{
	"web_query",
	"analyze",
	"reminder",
}

func Batch1ExecutionDatasetManifest() DatasetManifest {
	items := make([]DatasetManifestItem, 0, Batch1ExecutionCaseCount())
	for _, skill := range batch1ExecutionSkills {
		for _, example := range routingcue.LocalizedExamples(skill) {
			items = append(items, batch1ExecutionLocalizedCase(skill, example.Locale, example.Query))
		}
	}
	items = append(items, batch1ExecutionCriticalCases()...)

	return DatasetManifest{
		Dataset: DatasetManifestMeta{
			Name:    Batch1ExecutionDatasetName,
			Subject: Batch1ExecutionDatasetSubject,
		},
		Defaults: DatasetManifestDefaults{
			RunKind: RunKindAgentTask,
			Profile: batch1ExecutionProfile,
			Scheduler: GroupSchedulerConfig{
				MaxConcurrency: batch1ExecutionMaxConcurrency,
				MaxAttempts:    batch1ExecutionMaxAttempts,
				RetryBackoff:   batch1ExecutionRetryBackoff,
			},
			Scoring:       GroupScoringConfig{Mode: ScoringModeRule, PassThreshold: 1},
			RuntimePolicy: batch1ExecutionRuntimePolicy(),
		},
		Items: items,
	}
}

func Batch1ExecutionCaseCount() int {
	return len(routingcue.SupportedLocales())*len(batch1ExecutionSkills) + len(batch1ExecutionCriticalCases())
}

func Batch1ExecutionDatasetSpec(ownerUserID string) DatasetSpec {
	return DatasetSpec{
		Name:           Batch1ExecutionDatasetName,
		Description:    Batch1ExecutionDatasetDescription,
		OwnerUserID:    strings.TrimSpace(ownerUserID),
		Subject:        Batch1ExecutionDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: batch1ExecutionProfile,
		Metadata: map[string]interface{}{
			"dataset_family":  "builtin_batch1_execution",
			"migration_batch": "batch1",
			"migrated_skills": append([]string(nil), batch1ExecutionSkills...),
			"languages":       append([]string(nil), routingcue.SupportedLocales()...),
			"source":          "server/internal/harness/execution_dataset.go",
		},
	}
}

func Batch1ExecutionDatasetVersionSpec(createdBy string) (DatasetVersionSpec, error) {
	manifest, err := datasetManifestMap(Batch1ExecutionDatasetManifest())
	if err != nil {
		return DatasetVersionSpec{}, err
	}
	return DatasetVersionSpec{
		Version:    Batch1ExecutionDatasetVersion,
		SourceType: "builtin_batch1_execution",
		SourceRef:  "server/internal/harness/execution_dataset.go",
		Manifest:   manifest,
		Metadata: map[string]interface{}{
			"case_count":      Batch1ExecutionCaseCount(),
			"migration_batch": "batch1",
			"migrated_skills": append([]string(nil), batch1ExecutionSkills...),
			"languages":       append([]string(nil), routingcue.SupportedLocales()...),
		},
		CreatedBy: strings.TrimSpace(createdBy),
	}, nil
}

func Batch1ExecutionEvalSpecSpec(datasetID, datasetVersionID, ownerUserID string) EvalSpecSpec {
	return EvalSpecSpec{
		Name:             Batch1ExecutionEvalName,
		OwnerUserID:      strings.TrimSpace(ownerUserID),
		Subject:          Batch1ExecutionDatasetSubject,
		RunKind:          RunKindAgentTask,
		Profile:          batch1ExecutionProfile,
		DatasetID:        strings.TrimSpace(datasetID),
		DatasetVersionID: strings.TrimSpace(datasetVersionID),
		SchedulerConfig: GroupSchedulerConfig{
			MaxConcurrency: batch1ExecutionMaxConcurrency,
			MaxAttempts:    batch1ExecutionMaxAttempts,
			RetryBackoff:   batch1ExecutionRetryBackoff,
		},
		RuntimePolicy: batch1ExecutionRuntimePolicy(),
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 1,
		},
		Metadata: map[string]interface{}{
			"dataset_family":  "builtin_batch1_execution",
			"gate_type":       "execution_equivalence",
			"migration_batch": "batch1",
		},
	}
}

func batch1ExecutionLocalizedCase(skill, locale, query string) DatasetManifestItem {
	input := batch1ExecutionInput(query, locale)
	if sessionID := batch1ExecutionSessionID(skill, locale, "localized_route"); sessionID != "" {
		input["session_id"] = sessionID
	}
	return DatasetManifestItem{
		ID:       fmt.Sprintf("exec-%s-%s", strings.ToLower(strings.TrimSpace(skill)), selectorLocaleToken(locale)),
		Input:    input,
		Expected: batch1ExecutionExpected(skill),
		Metadata: batch1ExecutionMetadata(skill, locale, "localized_route", false, fmt.Sprintf("The %s execution flow should complete without regressing from the current baseline.", skill)),
	}
}

func batch1ExecutionCriticalCases() []DatasetManifestItem {
	return []DatasetManifestItem{
		batch1ExecutionCriticalCase(
			"critical-web-search-latest-docs-zh-cn",
			"web_query",
			"zh-CN",
			"搜索 OpenAI Responses API 的最新文档。",
			"Keep latest-documentation execution working for live web search requests.",
			"latest_docs_web",
		),
		batch1ExecutionCriticalCaseWithOptions(
			"critical-web-search-latest-docs-memory-guard-zh-cn",
			"web_query",
			"zh-CN",
			"搜索 OpenAI Responses API 的最新文档。",
			"Keep latest-documentation execution grounded in live web evidence even when stale long-term memory exists.",
			"latest_docs_memory_guard",
			[]string{"planner_memory_skipped"},
			nil,
			map[string]interface{}{
				"harness_memory_seed": []map[string]interface{}{
					{
						"content": "This documentation belongs to Cursor, not ZimaOS.",
						"score":   0.97,
						"tags":    []string{"longterm", "docs"},
					},
				},
			},
		),
		batch1ExecutionCriticalCase(
			"critical-analyze-url-summary-zh-cn",
			"analyze",
			"zh-CN",
			"总结 https://example.com/blog 并提炼关键要点。",
			"Keep URL-based summarize flows on analyze without bouncing to browser or web_query.",
			"url_summary_analysis",
		),
		batch1ExecutionCriticalCase(
			"critical-analyze-url-report-en-us",
			"analyze",
			"en-US",
			"Summarize https://example.com/blog and extract the key points.",
			"Keep URL report analysis execution working for analyze.",
			"url_report_analysis",
		),
		batch1ExecutionCriticalCaseWithOptions(
			"critical-analyze-url-report-memory-guard-en-us",
			"analyze",
			"en-US",
			"Summarize https://example.com/blog and extract the key points.",
			"Keep URL analysis grounded in the provided content while skipping conflicting long-term memory.",
			"url_report_memory_guard",
			[]string{"planner_memory_skipped"},
			[]string{"planner_memory_used"},
			map[string]interface{}{
				"harness_memory_seed": []map[string]interface{}{
					{
						"content": "This page says Blue is a calendar-only mobile app with no CLI.",
						"score":   0.92,
						"tags":    []string{"longterm", "docs"},
					},
				},
			},
		),
		batch1ExecutionCriticalCase(
			"critical-reminder-tomorrow-9-zh-cn",
			"reminder",
			"zh-CN",
			"帮我明早 9 点提醒我发送周报。",
			"Keep reminder scheduling execution working for time-based requests.",
			"reminder_schedule",
		),
	}
}

func batch1ExecutionCriticalCase(id, skill, locale, query, rubric, caseType string) DatasetManifestItem {
	return batch1ExecutionCriticalCaseWithOptions(id, skill, locale, query, rubric, caseType, nil, nil, nil)
}

func batch1ExecutionCriticalCaseWithOptions(id, skill, locale, query, rubric, caseType string, requiredObservations []string, forbiddenObservations []string, extraMetadata map[string]interface{}) DatasetManifestItem {
	input := batch1ExecutionInput(query, locale)
	if sessionID := batch1ExecutionSessionID(skill, locale, caseType); sessionID != "" {
		input["session_id"] = sessionID
	}
	return DatasetManifestItem{
		ID:       strings.TrimSpace(id),
		Input:    input,
		Expected: batch1ExecutionExpectedWithObservations(skill, requiredObservations, forbiddenObservations),
		Metadata: mergeMetadataMaps(batch1ExecutionMetadata(skill, locale, caseType, true, rubric), extraMetadata),
	}
}

func batch1ExecutionInput(query, locale string) map[string]interface{} {
	return map[string]interface{}{
		"goal":   strings.TrimSpace(query),
		"query":  strings.TrimSpace(query),
		"locale": strings.TrimSpace(locale),
		"lang":   strings.TrimSpace(locale),
	}
}

func batch1ExecutionExpected(skill string) map[string]interface{} {
	expected := map[string]interface{}{
		"status": "completed",
	}
	return ApplyHarnessContractToExpected(expected, batch1ExecutionContract(skill))
}

func batch1ExecutionExpectedWithObservations(skill string, required []string, forbidden []string) map[string]interface{} {
	expected := batch1ExecutionExpected(skill)
	if merged := dedupeContractStrings(append(decodeStringSlice(expected["required_observations"]), required...)); len(merged) > 0 {
		expected["required_observations"] = merged
	}
	if merged := dedupeContractStrings(append(decodeStringSlice(expected["forbidden_observations"]), forbidden...)); len(merged) > 0 {
		expected["forbidden_observations"] = merged
	}
	return expected
}

func batch1ExecutionContract(skill string) HarnessContract {
	contract := HarnessContract{
		ForbiddenObservations: []string{"clarification_requested"},
	}
	switch strings.TrimSpace(skill) {
	case "web_query":
		contract.RequiredObservations = []string{"evidence_tool_used"}
	case "analyze":
		contract.ForbiddenObservations = append(contract.ForbiddenObservations, "evidence_tool_used")
	case "reminder":
		contract.RequiredObservations = []string{"session_context_propagated"}
	}
	return contract
}

func batch1ExecutionMetadata(skill, locale, caseType string, critical bool, rubric string) map[string]interface{} {
	return map[string]interface{}{
		"execution_case_type": strings.TrimSpace(caseType),
		"locale":              strings.TrimSpace(locale),
		"primary_route":       strings.TrimSpace(skill),
		"critical":            critical,
		"expected_cli_action": selectorExpectedCLIAction(skill),
		"allow_fallback":      false,
		"migration_batch":     "batch1",
		"success_rubric":      strings.TrimSpace(rubric),
		"latency_budget_ms":   10000,
	}
}

func batch1ExecutionRuntimePolicy() map[string]interface{} {
	return map[string]interface{}{
		"gate_type":         "execution_equivalence",
		"migration_batch":   "batch1",
		"migrated_skills":   append([]string(nil), batch1ExecutionSkills...),
		"policy_model_hint": batch1ExecutionPolicyModelHint,
	}
}

func batch1ExecutionSessionID(skill, locale, caseType string) string {
	if strings.TrimSpace(skill) != "reminder" {
		return ""
	}
	token := selectorLocaleToken(locale)
	if token == "" {
		token = "default"
	}
	caseType = strings.TrimSpace(caseType)
	if caseType == "" {
		caseType = "default"
	}
	return fmt.Sprintf("batch1-%s-%s-%s", strings.TrimSpace(skill), token, caseType)
}
