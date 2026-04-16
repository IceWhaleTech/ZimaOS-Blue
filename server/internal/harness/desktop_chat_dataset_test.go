package harness

import (
	"context"
	"reflect"
	"testing"
)

func TestDesktopChatSuccessRateDatasetManifest_DecodesAndCountsCuratedCases(t *testing.T) {
	manifest := DesktopChatSuccessRateDatasetManifest()
	if manifest.Dataset.Name != DesktopChatSuccessRateDatasetName {
		t.Fatalf("dataset name = %q, want %q", manifest.Dataset.Name, DesktopChatSuccessRateDatasetName)
	}
	if manifest.Dataset.Subject != DesktopChatSuccessRateDatasetSubject {
		t.Fatalf("dataset subject = %q, want %q", manifest.Dataset.Subject, DesktopChatSuccessRateDatasetSubject)
	}
	if manifest.Defaults.RunKind != RunKindAgentTask {
		t.Fatalf("defaults.run_kind = %q, want %q", manifest.Defaults.RunKind, RunKindAgentTask)
	}
	if manifest.Defaults.Profile != desktopChatSuccessRateProfile {
		t.Fatalf("defaults.profile = %q, want %q", manifest.Defaults.Profile, desktopChatSuccessRateProfile)
	}
	if got := metadataString(manifest.Defaults.RuntimePolicy, "gate_type"); got != "desktop_chat_success_rate" {
		t.Fatalf("runtime_policy.gate_type = %q, want desktop_chat_success_rate", got)
	}
	if got := metadataString(manifest.Defaults.RuntimePolicy, "platform"); got != "darwin" {
		t.Fatalf("runtime_policy.platform = %q, want darwin", got)
	}
	if len(manifest.Items) != desktopChatSuccessRateMinCaseCount {
		t.Fatalf("len(manifest.Items) = %d, want %d", len(manifest.Items), desktopChatSuccessRateMinCaseCount)
	}
	if DesktopChatSuccessRateCaseCount() != desktopChatSuccessRateMinCaseCount {
		t.Fatalf("DesktopChatSuccessRateCaseCount() = %d, want %d", DesktopChatSuccessRateCaseCount(), desktopChatSuccessRateMinCaseCount)
	}
}

func TestDesktopChatSuccessRateDatasetManifest_RequiresTrajectoryArtifactsAndMetadata(t *testing.T) {
	manifest := DesktopChatSuccessRateDatasetManifest()
	for _, item := range manifest.Items {
		if metadataString(item.Metadata, "primary_route") != "computer_use" {
			t.Fatalf("%s primary_route = %q, want computer_use", item.ID, metadataString(item.Metadata, "primary_route"))
		}
		if metadataString(item.Metadata, "platform") != "darwin" {
			t.Fatalf("%s platform = %q, want darwin", item.ID, metadataString(item.Metadata, "platform"))
		}
		if metadataString(item.Metadata, "app_profile") != "feishu_lark" {
			t.Fatalf("%s app_profile = %q, want feishu_lark", item.ID, metadataString(item.Metadata, "app_profile"))
		}
		if got := decodeStringSlice(item.Expected["required_tool_calls"]); !reflect.DeepEqual(got, []string{"computer_use"}) {
			t.Fatalf("%s required_tool_calls = %#v, want [computer_use]", item.ID, item.Expected["required_tool_calls"])
		}
		required := decodeStringSlice(item.Expected["required_observations"])
		if len(required) < 2 || required[0] != "task_stage_trace_emitted" {
			t.Fatalf("%s required_observations = %#v, want task_stage_trace_emitted first", item.ID, item.Expected["required_observations"])
		}
		artifacts := decodeHarnessExpectedArtifacts(item.Expected["expected_artifacts"])
		if len(artifacts) != 4 {
			t.Fatalf("%s expected_artifacts = %#v, want 4 artifact contracts", item.ID, artifacts)
		}
		wantLabels := []string{"stage_trace", "final_result", "key_screenshot", "failure_classification"}
		for idx, artifact := range artifacts {
			if artifact.Label != wantLabels[idx] || !artifact.MustExist {
				t.Fatalf("%s artifact[%d] = %#v, want label=%s must_exist=true", item.ID, idx, artifact, wantLabels[idx])
			}
		}
	}
}

func TestDesktopChatSuccessRateDatasetManifest_CoversProgramFamilies(t *testing.T) {
	manifest := DesktopChatSuccessRateDatasetManifest()
	counts := map[string]int{}
	for _, item := range manifest.Items {
		counts[metadataString(item.Metadata, "task_family")]++
	}
	want := map[string]int{
		"known_conversation_draft":            4,
		"known_conversation_send":             4,
		"select_conversation_only":            4,
		"fail_closed_unknown_conversation":    2,
		"fail_closed_search_box_still_active": 2,
		"recover_stale_window_focus":          2,
		"recover_visual_when_ax_misses":       2,
	}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("task family counts = %#v, want %#v", counts, want)
	}

	unknown := findExecutionManifestItem(t, manifest.Items, "unknown-conversation-en-us")
	if got := decodeStringSlice(unknown.Expected["required_observations"]); !containsAllStrings(got, []string{"safe_fail_closed", "failure_code_conversation_not_found"}) {
		t.Fatalf("unknown required_observations = %#v, want fail-closed markers", unknown.Expected["required_observations"])
	}
	searchBox := findExecutionManifestItem(t, manifest.Items, "search-box-still-active-en-us")
	if got := decodeStringSlice(searchBox.Expected["required_observations"]); !containsAllStrings(got, []string{"safe_fail_closed", "failure_code_search_box_still_active"}) {
		t.Fatalf("search-box required_observations = %#v, want fail-closed markers", searchBox.Expected["required_observations"])
	}
	visual := findExecutionManifestItem(t, manifest.Items, "visual-ax-miss-en-us")
	if got := decodeStringSlice(visual.Expected["required_observations"]); !containsAllStrings(got, []string{"visual_grounding_used", "send_verified"}) {
		t.Fatalf("visual required_observations = %#v, want visual grounding + send verification", visual.Expected["required_observations"])
	}
}

func TestDesktopChatSuccessRateDatasetVersionAndEvalSpec_EmbedGatePolicy(t *testing.T) {
	versionSpec, err := DesktopChatSuccessRateDatasetVersionSpec("user-1")
	if err != nil {
		t.Fatalf("DesktopChatSuccessRateDatasetVersionSpec failed: %v", err)
	}
	if versionSpec.Version != DesktopChatSuccessRateDatasetVersion {
		t.Fatalf("version = %q, want %q", versionSpec.Version, DesktopChatSuccessRateDatasetVersion)
	}
	if metadataString(versionSpec.Metadata, "platform") != "darwin" {
		t.Fatalf("version metadata platform = %q, want darwin", metadataString(versionSpec.Metadata, "platform"))
	}
	if got := metadataString(versionSpec.Metadata, "app_profile"); got != "feishu_lark" {
		t.Fatalf("version metadata app_profile = %q, want feishu_lark", got)
	}
	if got := DesktopChatSuccessRateEvalSpecSpec("dataset-1", "version-1", "user-1"); metadataString(got.RuntimePolicy, "gate_type") != "desktop_chat_success_rate" {
		t.Fatalf("eval spec runtime_policy.gate_type = %q, want desktop_chat_success_rate", metadataString(got.RuntimePolicy, "gate_type"))
	}
}

func TestEvaluateDesktopChatSuccessRateGate_PassesAtThreshold(t *testing.T) {
	report := EvaluateDesktopChatSuccessRateGate(map[string]interface{}{
		"item_count": 20,
		"pass_rate":  0.85,
		"failure_label_counts": map[string]interface{}{
			"conversation_not_found": 2,
		},
	}, true)
	if !report.Passed {
		t.Fatalf("report = %#v, want pass", report)
	}
	if report.CaseCount != 20 || report.PassedCount != 17 {
		t.Fatalf("report counts = %#v, want case_count=20 passed_count=17", report)
	}
}

func TestEvaluateDesktopChatSuccessRateGate_FailsOnUnsafeSendAndSearchFieldTyping(t *testing.T) {
	report := EvaluateDesktopChatSuccessRateGate(map[string]interface{}{
		"item_count": 20,
		"pass_rate":  0.95,
		"failure_label_counts": map[string]interface{}{
			"unsafe_send":                  1,
			"typed_body_into_search_field": 1,
		},
	}, false)
	if report.Passed {
		t.Fatalf("report = %#v, want fail", report)
	}
	if report.UnsafeSendCount != 1 {
		t.Fatalf("unsafe_send_count = %d, want 1", report.UnsafeSendCount)
	}
	if report.TypedBodyIntoSearchFieldCount != 1 {
		t.Fatalf("typed_body_into_search_field_count = %d, want 1", report.TypedBodyIntoSearchFieldCount)
	}
}

func TestController_EnsureDesktopChatSuccessRateAssets_ReusesBuiltins(t *testing.T) {
	controller := newTestController(t)
	first, err := controller.EnsureDesktopChatSuccessRateAssets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("EnsureDesktopChatSuccessRateAssets(first) failed: %v", err)
	}
	second, err := controller.EnsureDesktopChatSuccessRateAssets(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("EnsureDesktopChatSuccessRateAssets(second) failed: %v", err)
	}
	if first.Dataset.ID != second.Dataset.ID {
		t.Fatalf("dataset reuse = %q vs %q, want same dataset", first.Dataset.ID, second.Dataset.ID)
	}
	if first.DatasetVersion.ID != second.DatasetVersion.ID {
		t.Fatalf("dataset version reuse = %q vs %q, want same version", first.DatasetVersion.ID, second.DatasetVersion.ID)
	}
	if first.EvalSpec.ID != second.EvalSpec.ID {
		t.Fatalf("eval spec reuse = %q vs %q, want same eval spec", first.EvalSpec.ID, second.EvalSpec.ID)
	}
}

func containsAllStrings(values []string, required []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range required {
		if _, ok := seen[value]; !ok {
			return false
		}
	}
	return true
}
