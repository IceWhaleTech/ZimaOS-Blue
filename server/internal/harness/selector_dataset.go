package harness

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
)

const (
	SelectorCuratedDatasetName        = "selector-curated"
	SelectorCuratedDatasetDescription = "Curated selector dry-run routing regression dataset."
	SelectorCuratedDatasetSubject     = "selector_dry_run"
	SelectorCuratedDatasetVersion     = "selector-curated-v2"
	SelectorCuratedEvalName           = "Selector Curated Dry Run"
	selectorCuratedMaxConcurrency     = 8

	selectorCuratedProfile      = "selector_dry_run"
	selectorDryRunTarget        = "/api/settings/selector/dry-run"
	selectorDryRunRequestMethod = "POST"
)

var selectorCuratedRouteSkills = []string{
	"web_query",
	"analyze",
	"reminder",
	"browser",
	"ui_reviewer",
}

var selectorDryRunCompareFields = []string{
	"canonical_skill_id",
	"skill_route_outcome",
	"skill_need_clarify",
}

// SelectorCuratedDatasetManifest returns the built-in selector routing dataset
// that freezes curated dry-run expectations into a reusable Harness manifest.
func SelectorCuratedDatasetManifest() DatasetManifest {
	items := make([]DatasetManifestItem, 0, SelectorCuratedCaseCount())
	for _, skill := range selectorCuratedRouteSkills {
		for _, example := range routingcue.LocalizedExamples(skill) {
			items = append(items, DatasetManifestItem{
				ID:       selectorLocalizedCaseID(skill, example.Locale),
				Input:    selectorDryRunInput(example.Query, example.Locale, fmt.Sprintf("Route this query to %s without clarification.", skill)),
				Expected: selectorSelectedExpected(skill),
				Metadata: selectorSelectedMetadata(skill, example.Locale),
			})
		}
	}
	items = append(items, selectorCuratedCriticalCases()...)

	return DatasetManifest{
		Dataset: DatasetManifestMeta{
			Name:    SelectorCuratedDatasetName,
			Subject: SelectorCuratedDatasetSubject,
		},
		Defaults: DatasetManifestDefaults{
			RunKind:       RunKindAgentTask,
			Profile:       selectorCuratedProfile,
			Scheduler:     GroupSchedulerConfig{MaxConcurrency: selectorCuratedMaxConcurrency},
			RuntimePolicy: selectorDryRunRuntimePolicy(),
		},
		Items: items,
	}
}

// SelectorCuratedCaseCount returns the stable case count for the built-in
// selector routing dataset.
func SelectorCuratedCaseCount() int {
	return len(routingcue.SupportedLocales())*len(selectorCuratedRouteSkills) + len(selectorCuratedCriticalCases())
}

// SelectorCuratedDatasetSpec provides the reusable dataset shell for the
// built-in selector routing manifest.
func SelectorCuratedDatasetSpec(ownerUserID string) DatasetSpec {
	return DatasetSpec{
		Name:           SelectorCuratedDatasetName,
		Description:    SelectorCuratedDatasetDescription,
		OwnerUserID:    strings.TrimSpace(ownerUserID),
		Subject:        SelectorCuratedDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: selectorCuratedProfile,
		Metadata: map[string]interface{}{
			"dataset_family": "builtin_selector_curated",
			"source":         "server/internal/harness/selector_dataset.go",
			"languages":      append([]string(nil), routingcue.SupportedLocales()...),
		},
	}
}

// SelectorCuratedDatasetVersionSpec freezes the current built-in selector
// manifest into a versioned DatasetVersionSpec.
func SelectorCuratedDatasetVersionSpec(createdBy string) (DatasetVersionSpec, error) {
	manifest, err := datasetManifestMap(SelectorCuratedDatasetManifest())
	if err != nil {
		return DatasetVersionSpec{}, err
	}
	return DatasetVersionSpec{
		Version:    SelectorCuratedDatasetVersion,
		SourceType: "builtin_selector_curated",
		SourceRef:  "server/internal/harness/selector_dataset.go",
		Manifest:   manifest,
		Metadata: map[string]interface{}{
			"case_count": SelectorCuratedCaseCount(),
			"languages":  append([]string(nil), routingcue.SupportedLocales()...),
		},
		CreatedBy: strings.TrimSpace(createdBy),
	}, nil
}

// SelectorCuratedEvalSpecSpec provides a reusable EvalSpec template wired to
// the selector dry-run contract. The concrete dataset/version ids must be
// supplied by the caller.
func SelectorCuratedEvalSpecSpec(datasetID, datasetVersionID, ownerUserID string) EvalSpecSpec {
	return EvalSpecSpec{
		Name:             SelectorCuratedEvalName,
		OwnerUserID:      strings.TrimSpace(ownerUserID),
		Subject:          SelectorCuratedDatasetSubject,
		RunKind:          RunKindAgentTask,
		Profile:          selectorCuratedProfile,
		DatasetID:        strings.TrimSpace(datasetID),
		DatasetVersionID: strings.TrimSpace(datasetVersionID),
		SchedulerConfig:  GroupSchedulerConfig{MaxConcurrency: selectorCuratedMaxConcurrency},
		RuntimePolicy:    selectorDryRunRuntimePolicy(),
		Metadata: map[string]interface{}{
			"dataset_family": "builtin_selector_curated",
			"gate_type":      "selection",
		},
	}
}

func selectorCuratedCriticalCases() []DatasetManifestItem {
	items := []DatasetManifestItem{
		selectorCriticalSelectedCase(
			"selected-web-search-latest-docs-zh-cn",
			"web_query",
			"zh-CN",
			"搜索最新 OpenAI Responses API 文档。",
			"Keep latest-documentation requests routed to web_query instead of local workspace analysis.",
			"latest_docs_web",
		),
		selectorCriticalSelectedCase(
			"selected-analyze-workspace-report-en-us",
			"exec",
			"en-US",
			"Review files in the workspace and summarize the report.",
			"Keep workspace report summarization routed to exec instead of web_query or analyze.",
			"workspace_report_analysis",
		),
		selectorCriticalSelectedCase(
			"selected-analyze-workspace-readme-zh-cn",
			"exec",
			"zh-CN",
			"看下 workspace 里的 README，总结一下项目是做什么的。",
			"Keep local workspace README requests routed to exec instead of web_query or analyze.",
			"workspace_readme_local",
		),
		selectorCriticalSelectedCase(
			"selected-reminder-tomorrow-9-zh-cn",
			"reminder",
			"zh-CN",
			"帮我明早 9 点提醒我发送周报。",
			"Keep reminder scheduling requests routed to reminder instead of clarify or analyze.",
			"reminder_schedule",
		),
		selectorCriticalSelectedCase(
			"selected-himalaya-email-cli-en-us",
			"himalaya",
			"en-US",
			"Search my IMAP inbox for unread mail from Alice and reply from the terminal.",
			"Keep real email CLI requests routed to himalaya instead of web_query.",
			"email_cli",
		),
		{
			ID: "clarify-mixed-local-web-zh-cn",
			Input: selectorDryRunInput(
				"看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？",
				"zh-CN",
				"Ask for clarification instead of guessing between a local workspace read and live web search.",
			),
			Expected: map[string]interface{}{
				"skill_route_outcome": "clarify",
				"skill_need_clarify":  true,
			},
			Metadata: map[string]interface{}{
				"critical":                   true,
				"selector_case_type":         "mixed_intent_clarify",
				"locale":                     "zh-CN",
				"primary_route":              "exec",
				"allowed_alternative_routes": []string{"web_query", "analyze"},
				"should_clarify":             true,
				"expected_cli_action":        "clarify_before_route",
				"allow_fallback":             false,
				"success_rubric":             "Selector should ask a clarifying question before choosing between local exec/file inspection and live web search.",
				"required_cards":             []string{"clarify_reason"},
				"latency_budget_ms":          1500,
				"compare_fields":             []string{"skill_route_outcome", "skill_need_clarify"},
			},
		},
	}
	items = append(items, selectorURLBypassCases("analyze")...)
	items = append(items, selectorURLBypassCases("ui_reviewer")...)
	return items
}

func selectorCriticalSelectedCase(id, skill, locale, query, goal, caseType string) DatasetManifestItem {
	return DatasetManifestItem{
		ID:       id,
		Input:    selectorDryRunInput(query, locale, goal),
		Expected: selectorSelectedExpected(skill),
		Metadata: map[string]interface{}{
			"critical":                   true,
			"selector_case_type":         strings.TrimSpace(caseType),
			"locale":                     strings.TrimSpace(locale),
			"primary_route":              strings.TrimSpace(skill),
			"allowed_alternative_routes": []string{},
			"should_clarify":             false,
			"expected_cli_action":        selectorExpectedCLIAction(skill),
			"allow_fallback":             false,
			"success_rubric":             strings.TrimSpace(goal),
			"required_cards":             []string{"skill_prompt_hint"},
			"latency_budget_ms":          1500,
			"compare_fields":             append([]string(nil), selectorDryRunCompareFields...),
		},
	}
}

func selectorURLBypassCase(id, skill, locale, query string) DatasetManifestItem {
	return DatasetManifestItem{
		ID:       id,
		Input:    selectorDryRunInput(query, locale, fmt.Sprintf("Keep %s selected even though the request contains a raw URL.", skill)),
		Expected: selectorSelectedExpected(skill),
		Metadata: map[string]interface{}{
			"critical":                   true,
			"selector_case_type":         "url_bypass",
			"locale":                     locale,
			"primary_route":              skill,
			"allowed_alternative_routes": []string{},
			"should_clarify":             false,
			"expected_cli_action":        selectorExpectedCLIAction(skill),
			"allow_fallback":             false,
			"success_rubric":             fmt.Sprintf("Selector should keep %s instead of falling back to the raw-URL browser rule.", skill),
			"required_cards":             []string{"skill_prompt_hint"},
			"latency_budget_ms":          1500,
			"compare_fields":             append([]string(nil), selectorDryRunCompareFields...),
		},
	}
}

func selectorURLBypassCases(skill string) []DatasetManifestItem {
	examples := routingcue.LocalizedURLBypassExamples(skill)
	items := make([]DatasetManifestItem, 0, len(examples))
	for _, example := range examples {
		items = append(items, selectorURLBypassCase(
			selectorURLBypassCaseID(skill, example.Locale),
			skill,
			example.Locale,
			example.Query,
		))
	}
	return items
}

func selectorURLBypassCaseID(skill, locale string) string {
	prefix := "skill"
	switch strings.ToLower(strings.TrimSpace(skill)) {
	case "analyze":
		prefix = "analyze"
	case "ui_reviewer":
		prefix = "ui-review"
	}
	return fmt.Sprintf("%s-url-%s", prefix, selectorLocaleToken(locale))
}

func selectorLocalizedCaseID(skill, locale string) string {
	return fmt.Sprintf("selected-%s-%s", strings.ToLower(strings.TrimSpace(skill)), selectorLocaleToken(locale))
}

func selectorLocaleToken(locale string) string {
	return strings.ToLower(strings.TrimSpace(locale))
}

func selectorDryRunInput(query, locale, goal string) map[string]interface{} {
	return map[string]interface{}{
		"goal":   strings.TrimSpace(goal),
		"query":  strings.TrimSpace(query),
		"model":  "auto",
		"locale": strings.TrimSpace(locale),
	}
}

func selectorSelectedExpected(skill string) map[string]interface{} {
	return map[string]interface{}{
		"canonical_skill_id":  strings.TrimSpace(skill),
		"skill_route_outcome": "selected",
		"skill_need_clarify":  false,
	}
}

func selectorSelectedMetadata(skill, locale string) map[string]interface{} {
	return map[string]interface{}{
		"selector_case_type":         "localized_selected",
		"locale":                     strings.TrimSpace(locale),
		"primary_route":              strings.TrimSpace(skill),
		"allowed_alternative_routes": []string{},
		"should_clarify":             false,
		"expected_cli_action":        selectorExpectedCLIAction(skill),
		"allow_fallback":             false,
		"success_rubric":             fmt.Sprintf("Selector should route directly to %s without clarification.", skill),
		"required_cards":             []string{"skill_prompt_hint"},
		"latency_budget_ms":          1500,
		"compare_fields":             append([]string(nil), selectorDryRunCompareFields...),
	}
}

func selectorExpectedCLIAction(skill string) string {
	switch strings.TrimSpace(skill) {
	case "browser":
		return "blue browser.navigate"
	case "exec":
		return "blue exec"
	case "reminder":
		return "blue reminder.add"
	default:
		return "blue " + strings.TrimSpace(skill)
	}
}

func selectorDryRunRuntimePolicy() map[string]interface{} {
	return map[string]interface{}{
		"driver":           "selector_dry_run",
		"target_endpoint":  selectorDryRunTarget,
		"request_method":   selectorDryRunRequestMethod,
		"request_defaults": map[string]interface{}{"model": "auto"},
		"compare_fields":   append([]string(nil), selectorDryRunCompareFields...),
		"required_fields": []string{
			"selected_tools",
			"skill_decision",
			"skill_prompt_hint",
			"canonical_skill_id",
			"skill_need_clarify",
			"skill_route_outcome",
		},
	}
}
