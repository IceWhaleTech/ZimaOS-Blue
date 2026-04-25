package harness

import (
	"fmt"
	"strings"
	"time"
)

const (
	CUAOSWorldMacBenchmarkDatasetName        = "computer-use-cua-osworld-macos"
	CUAOSWorldMacBenchmarkDatasetDescription = "OSWorld-style macOS computer-use benchmark profile for Go-native CUA mode."
	CUAOSWorldMacBenchmarkDatasetSubject     = "computer_use_cua_osworld_macos"
	CUAOSWorldMacBenchmarkDatasetVersion     = "computer-use-cua-osworld-macos-v1"
	CUAOSWorldMacBenchmarkEvalName           = "Computer-Use Go-Native CUA OSWorld-Style macOS Benchmark"
	CUAOSWorldMacBenchmarkProfile            = "computer_use_cua_osworld_macos"
	cuaOSWorldMacBenchmarkMaxConcurrency     = 1
	cuaOSWorldMacBenchmarkMaxAttempts        = 1
	cuaOSWorldMacBenchmarkRetryBackoff       = 10 * time.Second
)

type CUAOSWorldMacBenchmarkAssets struct {
	Dataset        *Dataset        `json:"dataset,omitempty"`
	DatasetVersion *DatasetVersion `json:"dataset_version,omitempty"`
	EvalSpec       *EvalSpec       `json:"eval_spec,omitempty"`
}

func CUAOSWorldMacBenchmarkManifest() DatasetManifest {
	return DatasetManifest{
		Dataset: DatasetManifestMeta{
			Name:    CUAOSWorldMacBenchmarkDatasetName,
			Subject: CUAOSWorldMacBenchmarkDatasetSubject,
		},
		Defaults: DatasetManifestDefaults{
			RunKind: RunKindAgentTask,
			Profile: CUAOSWorldMacBenchmarkProfile,
			Scheduler: GroupSchedulerConfig{
				MaxConcurrency: cuaOSWorldMacBenchmarkMaxConcurrency,
				MaxAttempts:    cuaOSWorldMacBenchmarkMaxAttempts,
				RetryBackoff:   cuaOSWorldMacBenchmarkRetryBackoff,
			},
			Scoring:       GroupScoringConfig{Mode: ScoringModeRule, PassThreshold: 1},
			RuntimePolicy: cuaOSWorldMacBenchmarkRuntimePolicy(),
		},
		Items: cuaOSWorldMacBenchmarkCases(),
	}
}

func CUAOSWorldMacBenchmarkDatasetSpec(ownerUserID string) DatasetSpec {
	return DatasetSpec{
		Name:           CUAOSWorldMacBenchmarkDatasetName,
		Description:    CUAOSWorldMacBenchmarkDatasetDescription,
		OwnerUserID:    strings.TrimSpace(ownerUserID),
		Subject:        CUAOSWorldMacBenchmarkDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: CUAOSWorldMacBenchmarkProfile,
		Metadata: map[string]interface{}{
			"dataset_family":  "cua_osworld_macos",
			"benchmark_style": "osworld",
			"platform":        "darwin",
			"case_count":      len(cuaOSWorldMacBenchmarkCases()),
			"source":          "server/internal/harness/cua_benchmark_profile.go",
		},
	}
}

func CUAOSWorldMacBenchmarkDatasetVersionSpec(createdBy string) (DatasetVersionSpec, error) {
	manifest, err := datasetManifestMap(CUAOSWorldMacBenchmarkManifest())
	if err != nil {
		return DatasetVersionSpec{}, err
	}
	return DatasetVersionSpec{
		Version:    CUAOSWorldMacBenchmarkDatasetVersion,
		SourceType: "cua_osworld_macos",
		SourceRef:  "server/internal/harness/cua_benchmark_profile.go",
		Manifest:   manifest,
		Metadata: map[string]interface{}{
			"benchmark_style": "osworld",
			"platform":        "darwin",
			"case_count":      len(cuaOSWorldMacBenchmarkCases()),
		},
		CreatedBy: strings.TrimSpace(createdBy),
	}, nil
}

func CUAOSWorldMacBenchmarkEvalSpecSpec(datasetID, datasetVersionID, ownerUserID string) EvalSpecSpec {
	return EvalSpecSpec{
		Name:             CUAOSWorldMacBenchmarkEvalName,
		OwnerUserID:      strings.TrimSpace(ownerUserID),
		Subject:          CUAOSWorldMacBenchmarkDatasetSubject,
		RunKind:          RunKindAgentTask,
		Profile:          CUAOSWorldMacBenchmarkProfile,
		DatasetID:        strings.TrimSpace(datasetID),
		DatasetVersionID: strings.TrimSpace(datasetVersionID),
		SchedulerConfig: GroupSchedulerConfig{
			MaxConcurrency: cuaOSWorldMacBenchmarkMaxConcurrency,
			MaxAttempts:    cuaOSWorldMacBenchmarkMaxAttempts,
			RetryBackoff:   cuaOSWorldMacBenchmarkRetryBackoff,
		},
		RuntimePolicy: cuaOSWorldMacBenchmarkRuntimePolicy(),
		ScoringConfig: GroupScoringConfig{
			Mode:          ScoringModeRule,
			PassThreshold: 1,
		},
		Metadata: map[string]interface{}{
			"dataset_family":  "cua_osworld_macos",
			"benchmark_style": "osworld",
			"gate_type":       "cua_osworld_macos",
		},
	}
}

func cuaOSWorldMacBenchmarkRuntimePolicy() map[string]interface{} {
	return map[string]interface{}{
		"gate_type":       "cua_osworld_macos",
		"benchmark_style": "osworld",
		"platform":        "darwin",
		"implementation":  "go_native_cua",
		"report_metrics": []string{
			"pass_rate",
			"critical_pass_rate",
			"unsafe_send_count",
			"typed_body_into_search_field_count",
			"verification_backed_rate",
			"latency_p50_ms",
			"latency_p90_ms",
		},
	}
}

func cuaOSWorldMacBenchmarkCases() []DatasetManifestItem {
	cases := []struct {
		id       string
		family   string
		goal     string
		critical bool
		rubric   string
	}{
		{"chat_send_feishu_group", "chat", "In Feishu/Lark, select the visible test conversation and send a short verification greeting only after confirming the target conversation.", true, "message appears in the intended conversation and no unsafe send occurs"},
		{"browser_find_release_note", "browser", "Open the default browser, find the latest visible release note in the current project page, and record the version string.", false, "version string is recorded with verification-backed evidence"},
		{"settings_toggle_inspect", "settings", "Open macOS Settings, navigate to a visible settings pane, inspect a named toggle state, and return without changing unrelated settings.", false, "toggle state is read without unsafe side effects"},
		{"file_create_note", "files", "Create a small local text note in a safe temporary location with the task summary, then verify the file exists.", true, "file exists with expected text and path evidence"},
		{"visual_drag_reorder", "visual_drag", "Use visual coordinates to drag a clearly identified item from one position to another in a safe demo surface.", true, "drag action executes with visual before/after verification"},
	}
	items := make([]DatasetManifestItem, 0, len(cases))
	for _, c := range cases {
		items = append(items, DatasetManifestItem{
			ID:      c.id,
			RunKind: RunKindAgentTask,
			Profile: CUAOSWorldMacBenchmarkProfile,
			Input: map[string]interface{}{
				"goal":      c.goal,
				"action":    "task",
				"max_steps": 12,
			},
			Expected: map[string]interface{}{
				"rubric": c.rubric,
			},
			Metadata: map[string]interface{}{
				"family":          c.family,
				"critical":        c.critical,
				"benchmark_style": "osworld",
				"failure_labels": []string{
					"unsafe_send",
					"typed_body_into_search_field",
					"verification_missing",
					"max_steps",
				},
			},
		})
	}
	return items
}

func CUAOSWorldMacBenchmarkCaseResultFromSummary(id string, summary map[string]interface{}) CUABenchmarkCaseResult {
	return CUABenchmarkCaseResult{
		ID:                 strings.TrimSpace(id),
		Passed:             metadataBool(summary, "passed"),
		Critical:           metadataBool(summary, "critical"),
		VerificationBacked: metadataBool(summary, "verification_backed"),
		FailureLabel:       metadataString(summary, "failure_label"),
		LatencyMS:          metadataInt(summary, "latency_ms"),
	}
}

func metadataBool(meta map[string]interface{}, key string) bool {
	value, ok := meta[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || strings.TrimSpace(typed) == "1"
	default:
		return fmt.Sprint(value) == "true"
	}
}

func metadataInt(value interface{}, key string) int {
	if meta, ok := value.(map[string]interface{}); ok {
		return metadataInt(meta[key], "")
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		var out int
		_, _ = fmt.Sscanf(strings.TrimSpace(typed), "%d", &out)
		return out
	default:
		return 0
	}
}
