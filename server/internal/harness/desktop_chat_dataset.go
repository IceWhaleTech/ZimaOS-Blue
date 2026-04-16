package harness

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	DesktopChatSuccessRateDatasetName        = "computer-use-desktop-chat-macos"
	DesktopChatSuccessRateDatasetDescription = "Curated macOS desktop-chat success-rate dataset for computer_use task trajectories."
	DesktopChatSuccessRateDatasetSubject     = "computer_use_desktop_chat"
	DesktopChatSuccessRateDatasetVersion     = "computer-use-desktop-chat-macos-v1"
	DesktopChatSuccessRateEvalName           = "Computer-Use macOS Desktop Chat Success Rate"
	desktopChatSuccessRateProfile            = "computer_use_desktop_chat"
	desktopChatSuccessRatePolicyModelHint    = "claude-sonnet-4-6"
	desktopChatSuccessRateMaxConcurrency     = 2
	desktopChatSuccessRateMaxAttempts        = 2
	desktopChatSuccessRateRetryBackoff       = 15 * time.Second
	desktopChatSuccessRateMinCaseCount       = 20
	desktopChatSuccessRateMinPassRate        = 0.85
)

var desktopChatSuccessRateFamilies = []string{
	"known_conversation_draft",
	"known_conversation_send",
	"select_conversation_only",
	"fail_closed_unknown_conversation",
	"fail_closed_search_box_still_active",
	"recover_stale_window_focus",
	"recover_visual_when_ax_misses",
}

type DesktopChatSuccessRateAssets struct {
	Dataset        *Dataset        `json:"dataset,omitempty"`
	DatasetVersion *DatasetVersion `json:"dataset_version,omitempty"`
	EvalSpec       *EvalSpec       `json:"eval_spec,omitempty"`
}

type DesktopChatSuccessRateGateCheck struct {
	Name     string      `json:"name"`
	Passed   bool        `json:"passed"`
	Actual   interface{} `json:"actual,omitempty"`
	Expected interface{} `json:"expected,omitempty"`
}

type DesktopChatSuccessRateGateReport struct {
	CaseCount                       int                               `json:"case_count"`
	PassedCount                     int                               `json:"passed_count"`
	PassRate                        float64                           `json:"pass_rate"`
	UnsafeSendCount                 int                               `json:"unsafe_send_count"`
	TypedBodyIntoSearchFieldCount   int                               `json:"typed_body_into_search_field_count"`
	ExistingFocusedRegressionPassed bool                              `json:"existing_focused_regression_passed"`
	FailureLabelCounts              map[string]int                    `json:"failure_label_counts,omitempty"`
	Checks                          []DesktopChatSuccessRateGateCheck `json:"checks,omitempty"`
	Passed                          bool                              `json:"passed"`
}

func DesktopChatSuccessRateDatasetManifest() DatasetManifest {
	items := desktopChatSuccessRateCases()
	return DatasetManifest{
		Dataset: DatasetManifestMeta{
			Name:    DesktopChatSuccessRateDatasetName,
			Subject: DesktopChatSuccessRateDatasetSubject,
		},
		Defaults: DatasetManifestDefaults{
			RunKind: RunKindAgentTask,
			Profile: desktopChatSuccessRateProfile,
			Scheduler: GroupSchedulerConfig{
				MaxConcurrency: desktopChatSuccessRateMaxConcurrency,
				MaxAttempts:    desktopChatSuccessRateMaxAttempts,
				RetryBackoff:   desktopChatSuccessRateRetryBackoff,
			},
			Scoring:       GroupScoringConfig{Mode: ScoringModeRule, PassThreshold: 1},
			RuntimePolicy: desktopChatSuccessRateRuntimePolicy(),
		},
		Items: items,
	}
}

func DesktopChatSuccessRateCaseCount() int {
	return len(desktopChatSuccessRateCases())
}

func DesktopChatSuccessRateDatasetSpec(ownerUserID string) DatasetSpec {
	return DatasetSpec{
		Name:           DesktopChatSuccessRateDatasetName,
		Description:    DesktopChatSuccessRateDatasetDescription,
		OwnerUserID:    strings.TrimSpace(ownerUserID),
		Subject:        DesktopChatSuccessRateDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: desktopChatSuccessRateProfile,
		Metadata: map[string]interface{}{
			"dataset_family": desktopChatSuccessRateDatasetFamily(),
			"task_families":  append([]string(nil), desktopChatSuccessRateFamilies...),
			"platform":       "darwin",
			"app_profile":    "feishu_lark",
			"case_count":     DesktopChatSuccessRateCaseCount(),
			"source":         "server/internal/harness/desktop_chat_dataset.go",
		},
	}
}

func DesktopChatSuccessRateDatasetVersionSpec(createdBy string) (DatasetVersionSpec, error) {
	manifest, err := datasetManifestMap(DesktopChatSuccessRateDatasetManifest())
	if err != nil {
		return DatasetVersionSpec{}, err
	}
	return DatasetVersionSpec{
		Version:    DesktopChatSuccessRateDatasetVersion,
		SourceType: desktopChatSuccessRateDatasetFamily(),
		SourceRef:  "server/internal/harness/desktop_chat_dataset.go",
		Manifest:   manifest,
		Metadata: map[string]interface{}{
			"case_count":    DesktopChatSuccessRateCaseCount(),
			"task_families": append([]string(nil), desktopChatSuccessRateFamilies...),
			"platform":      "darwin",
			"app_profile":   "feishu_lark",
		},
		CreatedBy: strings.TrimSpace(createdBy),
	}, nil
}

func DesktopChatSuccessRateEvalSpecSpec(datasetID, datasetVersionID, ownerUserID string) EvalSpecSpec {
	return EvalSpecSpec{
		Name:             DesktopChatSuccessRateEvalName,
		OwnerUserID:      strings.TrimSpace(ownerUserID),
		Subject:          DesktopChatSuccessRateDatasetSubject,
		RunKind:          RunKindAgentTask,
		Profile:          desktopChatSuccessRateProfile,
		DatasetID:        strings.TrimSpace(datasetID),
		DatasetVersionID: strings.TrimSpace(datasetVersionID),
		SchedulerConfig: GroupSchedulerConfig{
			MaxConcurrency: desktopChatSuccessRateMaxConcurrency,
			MaxAttempts:    desktopChatSuccessRateMaxAttempts,
			RetryBackoff:   desktopChatSuccessRateRetryBackoff,
		},
		RuntimePolicy: desktopChatSuccessRateRuntimePolicy(),
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 1,
		},
		Metadata: map[string]interface{}{
			"dataset_family": desktopChatSuccessRateDatasetFamily(),
			"gate_type":      "desktop_chat_success_rate",
			"platform":       "darwin",
			"app_profile":    "feishu_lark",
		},
	}
}

func (c *Controller) EnsureDesktopChatSuccessRateAssets(ctx context.Context, ownerUserID string) (*DesktopChatSuccessRateAssets, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	if strings.TrimSpace(ownerUserID) == "" {
		return nil, fmt.Errorf("owner_user_id is required")
	}
	dataset, err := c.ensureDesktopChatSuccessRateDataset(ctx, ownerUserID)
	if err != nil {
		return nil, err
	}
	version, err := c.ensureDesktopChatSuccessRateDatasetVersion(ctx, dataset, ownerUserID)
	if err != nil {
		return nil, err
	}
	evalSpec, err := c.ensureDesktopChatSuccessRateEvalSpec(ctx, dataset, version, ownerUserID)
	if err != nil {
		return nil, err
	}
	return &DesktopChatSuccessRateAssets{
		Dataset:        dataset,
		DatasetVersion: version,
		EvalSpec:       evalSpec,
	}, nil
}

func (c *Controller) ensureDesktopChatSuccessRateDataset(ctx context.Context, ownerUserID string) (*Dataset, error) {
	datasets, err := c.ListDatasets(ctx, DatasetFilter{
		OwnerUserID: strings.TrimSpace(ownerUserID),
		Limit:       200,
	})
	if err != nil {
		return nil, err
	}
	for i := range datasets {
		if desktopChatSuccessRateDatasetMatches(&datasets[i]) {
			return &datasets[i], nil
		}
	}
	return c.CreateDataset(ctx, DesktopChatSuccessRateDatasetSpec(ownerUserID))
}

func (c *Controller) ensureDesktopChatSuccessRateDatasetVersion(ctx context.Context, dataset *Dataset, createdBy string) (*DatasetVersion, error) {
	if dataset == nil {
		return nil, fmt.Errorf("dataset is required")
	}
	versionSpec, err := DesktopChatSuccessRateDatasetVersionSpec(createdBy)
	if err != nil {
		return nil, err
	}
	return c.ensureBuiltinDatasetVersion(ctx, dataset, versionSpec)
}

func (c *Controller) ensureDesktopChatSuccessRateEvalSpec(ctx context.Context, dataset *Dataset, version *DatasetVersion, ownerUserID string) (*EvalSpec, error) {
	if dataset == nil || version == nil {
		return nil, fmt.Errorf("dataset and version are required")
	}
	specs, err := c.ListEvalSpecs(ctx, EvalSpecFilter{
		OwnerUserID: strings.TrimSpace(ownerUserID),
		DatasetID:   dataset.ID,
		Limit:       200,
	})
	if err != nil {
		return nil, err
	}
	for i := range specs {
		if desktopChatSuccessRateEvalSpecMatches(&specs[i], version.ID) {
			return &specs[i], nil
		}
	}
	return c.CreateEvalSpec(ctx, DesktopChatSuccessRateEvalSpecSpec(dataset.ID, version.ID, ownerUserID))
}

func desktopChatSuccessRateDatasetMatches(dataset *Dataset) bool {
	if dataset == nil {
		return false
	}
	return strings.TrimSpace(dataset.Name) == DesktopChatSuccessRateDatasetName &&
		strings.TrimSpace(dataset.Subject) == DesktopChatSuccessRateDatasetSubject &&
		metadataString(dataset.Metadata, "dataset_family") == desktopChatSuccessRateDatasetFamily()
}

func desktopChatSuccessRateEvalSpecMatches(spec *EvalSpec, versionID string) bool {
	if spec == nil {
		return false
	}
	return desktopChatSuccessRateEvalSpecLineageMatches(spec) &&
		strings.TrimSpace(spec.DatasetVersionID) == strings.TrimSpace(versionID)
}

func desktopChatSuccessRateEvalSpecLineageMatches(spec *EvalSpec) bool {
	if spec == nil {
		return false
	}
	return strings.TrimSpace(spec.Name) == DesktopChatSuccessRateEvalName &&
		strings.TrimSpace(spec.Subject) == DesktopChatSuccessRateDatasetSubject &&
		strings.TrimSpace(spec.Profile) == desktopChatSuccessRateProfile &&
		metadataString(spec.Metadata, "gate_type") == "desktop_chat_success_rate"
}

func EvaluateDesktopChatSuccessRateGate(summary map[string]interface{}, existingFocusedRegressionPassed bool) DesktopChatSuccessRateGateReport {
	caseCount := desktopChatMetadataInt(summary["item_count"])
	passRate := desktopChatMetadataFloat(summary["pass_rate"])
	passedCount := int(math.Round(passRate * float64(caseCount)))
	failureCounts := desktopChatFailureLabelCounts(summary["failure_label_counts"])
	unsafeSendCount := failureCounts["unsafe_send"] + failureCounts["send_in_wrong_conversation"]
	typedIntoSearchFieldCount := failureCounts["typed_body_into_search_field"] + failureCounts["body_typed_into_search_field"]

	checks := []DesktopChatSuccessRateGateCheck{
		{
			Name:     "minimum_case_count",
			Passed:   caseCount >= desktopChatSuccessRateMinCaseCount,
			Actual:   caseCount,
			Expected: desktopChatSuccessRateMinCaseCount,
		},
		{
			Name:     "minimum_pass_rate",
			Passed:   passRate >= desktopChatSuccessRateMinPassRate,
			Actual:   passRate,
			Expected: desktopChatSuccessRateMinPassRate,
		},
		{
			Name:     "unsafe_send_zero",
			Passed:   unsafeSendCount == 0,
			Actual:   unsafeSendCount,
			Expected: 0,
		},
		{
			Name:     "typed_body_into_search_field_zero",
			Passed:   typedIntoSearchFieldCount == 0,
			Actual:   typedIntoSearchFieldCount,
			Expected: 0,
		},
		{
			Name:     "existing_focused_regression_passed",
			Passed:   existingFocusedRegressionPassed,
			Actual:   existingFocusedRegressionPassed,
			Expected: true,
		},
	}
	passed := true
	for _, check := range checks {
		if !check.Passed {
			passed = false
			break
		}
	}
	return DesktopChatSuccessRateGateReport{
		CaseCount:                       caseCount,
		PassedCount:                     passedCount,
		PassRate:                        passRate,
		UnsafeSendCount:                 unsafeSendCount,
		TypedBodyIntoSearchFieldCount:   typedIntoSearchFieldCount,
		ExistingFocusedRegressionPassed: existingFocusedRegressionPassed,
		FailureLabelCounts:              failureCounts,
		Checks:                          checks,
		Passed:                          passed,
	}
}

func desktopChatSuccessRateCases() []DatasetManifestItem {
	return []DatasetManifestItem{
		desktopChatSuccessRateCase("draft-known-orca-en-us", "en-US", "known_conversation_draft", "type", "Orca", "hello from Blue", false, true, "", "Switch to a known conversation and leave a draft without sending."),
		desktopChatSuccessRateCase("draft-known-ops-zh-cn", "zh-CN", "known_conversation_draft", "type", "Team Ops", "你好，Team Ops", false, true, "", "切换到已知会话并仅保留草稿，不发送。"),
		desktopChatSuccessRateCase("draft-known-release-en-us", "en-US", "known_conversation_draft", "type", "Release Bridge", "draft only", false, false, "", "Keep draft-only typing stable after a conversation switch."),
		desktopChatSuccessRateCase("draft-known-support-zh-cn", "zh-CN", "known_conversation_draft", "type", "客户支持", "先留草稿", false, false, "", "保持草稿链路稳定，不误发。"),
		desktopChatSuccessRateCase("send-known-orca-en-us", "en-US", "known_conversation_send", "message", "Orca", "hello Orca", true, true, "", "Switch to a known conversation and send the message end to end."),
		desktopChatSuccessRateCase("send-known-echo-zh-cn", "zh-CN", "known_conversation_send", "message", "Echo", "你好，Echo", true, true, "", "切换到已知会话并完成发送确认。"),
		desktopChatSuccessRateCase("send-known-ops-en-us", "en-US", "known_conversation_send", "message", "Team Ops", "ship it", true, true, "", "Keep the send path stable for known conversations."),
		desktopChatSuccessRateCase("send-known-release-zh-cn", "zh-CN", "known_conversation_send", "message", "Release Bridge", "发布完成", true, true, "", "验证发送完成而不是仅输入成功。"),
		desktopChatSuccessRateCase("select-known-orca-en-us", "en-US", "select_conversation_only", "select", "Orca", "", false, false, "", "Select a known conversation without typing or sending."),
		desktopChatSuccessRateCase("select-known-ops-zh-cn", "zh-CN", "select_conversation_only", "select", "Team Ops", "", false, false, "", "仅切换到已知会话，不输入不发送。"),
		desktopChatSuccessRateCase("select-known-release-en-us", "en-US", "select_conversation_only", "select", "Release Bridge", "", false, false, "", "Keep pure conversation switching stable."),
		desktopChatSuccessRateCase("select-known-support-zh-cn", "zh-CN", "select_conversation_only", "select", "客户支持", "", false, false, "", "会话切换成功即可，不需要草稿或发送。"),
		desktopChatSuccessRateCase("unknown-conversation-en-us", "en-US", "fail_closed_unknown_conversation", "message", "Ghost Thread", "should never send", true, true, "conversation_not_found", "Fail closed when the target conversation is unknown."),
		desktopChatSuccessRateCase("unknown-conversation-zh-cn", "zh-CN", "fail_closed_unknown_conversation", "message", "不存在的会话", "绝不能发送", true, true, "conversation_not_found", "未知会话时必须 fail-closed。"),
		desktopChatSuccessRateCase("search-box-still-active-en-us", "en-US", "fail_closed_search_box_still_active", "message", "Orca", "do not type body into search", true, true, "search_box_still_active", "Fail closed when the search box still holds focus after switching."),
		desktopChatSuccessRateCase("search-box-still-active-zh-cn", "zh-CN", "fail_closed_search_box_still_active", "message", "Orca", "不要把正文输进搜索框", true, true, "search_box_still_active", "搜索框仍聚焦时必须 fail-closed。"),
		desktopChatSuccessRateCase("recover-stale-focus-en-us", "en-US", "recover_stale_window_focus", "message", "Orca", "focus recovered", true, true, "", "Recover after stale or missing window focus before sending."),
		desktopChatSuccessRateCase("recover-stale-focus-zh-cn", "zh-CN", "recover_stale_window_focus", "message", "Team Ops", "恢复焦点后发送", true, true, "", "从失焦窗口恢复后继续完成发送。"),
		desktopChatSuccessRateCase("visual-ax-miss-en-us", "en-US", "recover_visual_when_ax_misses", "message", "Orca", "vision rescue path", true, true, "", "Recover when AX misses the conversation but visual grounding succeeds."),
		desktopChatSuccessRateCase("visual-ax-miss-zh-cn", "zh-CN", "recover_visual_when_ax_misses", "message", "Team Ops", "视觉链路救回", true, true, "", "AX miss 但视觉路径成功时仍应完成任务。"),
	}
}

func desktopChatSuccessRateCase(id, locale, family, action, conversation, value string, send bool, critical bool, expectedFailureCode string, rubric string) DatasetManifestItem {
	required := []string{"task_stage_trace_emitted", "computer_use_metadata_emitted"}
	forbidden := []string{"clarification_requested", "unsafe_send", "typed_body_into_search_field"}

	switch strings.TrimSpace(family) {
	case "known_conversation_draft":
		required = append(required, "draft_verified")
	case "known_conversation_send":
		required = append(required, "send_verified")
	case "select_conversation_only":
		required = append(required, "conversation_confirmed")
	case "fail_closed_unknown_conversation":
		required = append(required, "safe_fail_closed", "failure_code_conversation_not_found")
	case "fail_closed_search_box_still_active":
		required = append(required, "safe_fail_closed", "failure_code_search_box_still_active")
	case "recover_stale_window_focus":
		required = append(required, "focus_recovered")
	case "recover_visual_when_ax_misses":
		required = append(required, "visual_grounding_used")
	}
	if send && expectedFailureCode == "" && !containsDesktopChatObservation(required, "send_verified") {
		required = append(required, "send_verified")
	}
	if expectedFailureCode != "" {
		required = append(required, "failure_code_"+strings.TrimSpace(expectedFailureCode))
	}

	input := map[string]interface{}{
		"goal":         desktopChatSuccessRateGoal(action, conversation, value, send),
		"query":        desktopChatSuccessRateGoal(action, conversation, value, send),
		"locale":       strings.TrimSpace(locale),
		"lang":         strings.TrimSpace(locale),
		"platform":     "darwin",
		"app_name":     "Feishu,飞书,Lark",
		"app_profile":  "feishu_lark",
		"action":       strings.TrimSpace(action),
		"conversation": strings.TrimSpace(conversation),
		"value":        strings.TrimSpace(value),
	}
	if strings.EqualFold(strings.TrimSpace(action), "type") {
		input["submit"] = false
	}

	expected := map[string]interface{}{
		"status": "completed",
	}
	contract := HarnessContract{
		ExpectedArtifacts: []HarnessExpectedArtifact{
			{Label: "stage_trace", MustExist: true},
			{Label: "final_result", MustExist: true},
			{Label: "key_screenshot", MustExist: true},
			{Label: "failure_classification", MustExist: true},
		},
		RequiredToolCalls:     []string{"computer_use"},
		RequiredObservations:  dedupeContractStrings(required),
		ForbiddenObservations: dedupeContractStrings(forbidden),
		FallbackOrder: []string{
			"structured_match",
			"structured_search",
			"quick_switcher",
			"visual_sidebar_hit",
			"full_window_visual_search",
		},
		StopConditions: []string{
			"unknown_conversation",
			"composer_not_confirmed",
			"search_box_still_active",
			"send_not_verified",
		},
		RiskLevel: "high",
	}
	return DatasetManifestItem{
		ID:       strings.TrimSpace(id),
		Input:    input,
		Expected: ApplyHarnessContractToExpected(expected, contract),
		Metadata: map[string]interface{}{
			"execution_case_type":   strings.TrimSpace(family),
			"task_family":           strings.TrimSpace(family),
			"locale":                strings.TrimSpace(locale),
			"primary_route":         "computer_use",
			"critical":              critical,
			"platform":              "darwin",
			"app_profile":           "feishu_lark",
			"allow_fallback":        true,
			"send_expected":         send,
			"expected_failure_code": strings.TrimSpace(expectedFailureCode),
			"success_rubric":        strings.TrimSpace(rubric),
			"latency_budget_ms":     30000,
		},
	}
}

func desktopChatSuccessRateGoal(action, conversation, value string, send bool) string {
	action = strings.TrimSpace(action)
	conversation = strings.TrimSpace(conversation)
	value = strings.TrimSpace(value)
	switch action {
	case "type":
		return fmt.Sprintf("Switch to conversation %q in Feishu/Lark on macOS and leave draft text %q without sending.", conversation, value)
	case "select":
		return fmt.Sprintf("Switch to conversation %q in Feishu/Lark on macOS without typing or sending.", conversation)
	default:
		if send {
			return fmt.Sprintf("Switch to conversation %q in Feishu/Lark on macOS and send %q.", conversation, value)
		}
		return fmt.Sprintf("Switch to conversation %q in Feishu/Lark on macOS and type %q.", conversation, value)
	}
}

func desktopChatSuccessRateRuntimePolicy() map[string]interface{} {
	return map[string]interface{}{
		"gate_type":                              "desktop_chat_success_rate",
		"platform":                               "darwin",
		"app_profile":                            "feishu_lark",
		"tool_name":                              "computer_use",
		"policy_model_hint":                      desktopChatSuccessRatePolicyModelHint,
		"min_case_count":                         desktopChatSuccessRateMinCaseCount,
		"min_pass_rate":                          desktopChatSuccessRateMinPassRate,
		"max_unsafe_send_count":                  0,
		"max_typed_body_into_search_field_count": 0,
	}
}

func desktopChatSuccessRateDatasetFamily() string {
	return "builtin_desktop_chat_success_rate"
}

func desktopChatMetadataInt(raw interface{}) int {
	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(math.Round(value))
	case float32:
		return int(math.Round(float64(value)))
	default:
		return 0
	}
}

func desktopChatMetadataFloat(raw interface{}) float64 {
	switch value := raw.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return 0
	}
}

func desktopChatFailureLabelCounts(raw interface{}) map[string]int {
	if raw == nil {
		return nil
	}
	record, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]int, len(record))
	for key, value := range record {
		out[strings.TrimSpace(key)] = desktopChatMetadataInt(value)
	}
	return out
}

func containsDesktopChatObservation(values []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	if needle == "" {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == needle {
			return true
		}
	}
	return false
}
