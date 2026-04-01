package harness

import (
	"context"
	"reflect"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
)

func TestBatch1ExecutionDatasetManifest_DecodesAndCountsCases(t *testing.T) {
	manifest := Batch1ExecutionDatasetManifest()
	if manifest.Dataset.Name != Batch1ExecutionDatasetName {
		t.Fatalf("dataset name = %q, want %q", manifest.Dataset.Name, Batch1ExecutionDatasetName)
	}
	if manifest.Dataset.Subject != Batch1ExecutionDatasetSubject {
		t.Fatalf("dataset subject = %q, want %q", manifest.Dataset.Subject, Batch1ExecutionDatasetSubject)
	}
	if manifest.Defaults.RunKind != RunKindAgentTask {
		t.Fatalf("defaults.run_kind = %q, want %q", manifest.Defaults.RunKind, RunKindAgentTask)
	}
	if manifest.Defaults.Profile != batch1ExecutionProfile {
		t.Fatalf("defaults.profile = %q, want %q", manifest.Defaults.Profile, batch1ExecutionProfile)
	}
	if manifest.Defaults.Scheduler.MaxConcurrency != batch1ExecutionMaxConcurrency {
		t.Fatalf("defaults.scheduler.max_concurrency = %d, want %d", manifest.Defaults.Scheduler.MaxConcurrency, batch1ExecutionMaxConcurrency)
	}
	if manifest.Defaults.Scoring.Mode != ScoringModeRule {
		t.Fatalf("defaults.scoring.mode = %q, want %q", manifest.Defaults.Scoring.Mode, ScoringModeRule)
	}
	if manifest.Defaults.Scoring.PassThreshold != 1 {
		t.Fatalf("defaults.scoring.pass_threshold = %#v, want 1", manifest.Defaults.Scoring.PassThreshold)
	}
	if len(manifest.Items) != Batch1ExecutionCaseCount() {
		t.Fatalf("items len = %d, want %d", len(manifest.Items), Batch1ExecutionCaseCount())
	}

	raw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}
	decoded, err := decodeDatasetManifest(raw)
	if err != nil {
		t.Fatalf("decodeDatasetManifest failed: %v", err)
	}
	if len(decoded.Items) != Batch1ExecutionCaseCount() {
		t.Fatalf("decoded items len = %d, want %d", len(decoded.Items), Batch1ExecutionCaseCount())
	}
	if metadataString(decoded.Defaults.RuntimePolicy, "gate_type") != "execution_equivalence" {
		t.Fatalf("runtime_policy.gate_type = %q, want execution_equivalence", metadataString(decoded.Defaults.RuntimePolicy, "gate_type"))
	}
	if metadataString(decoded.Defaults.RuntimePolicy, "migration_batch") != "batch1" {
		t.Fatalf("runtime_policy.migration_batch = %q, want batch1", metadataString(decoded.Defaults.RuntimePolicy, "migration_batch"))
	}
	if got := decodeStringSlice(decoded.Defaults.RuntimePolicy["migrated_skills"]); !reflect.DeepEqual(got, batch1ExecutionSkills) {
		t.Fatalf("runtime_policy.migrated_skills = %#v, want %#v", got, batch1ExecutionSkills)
	}
	if got := metadataString(decoded.Defaults.RuntimePolicy, "policy_model_hint"); got != batch1ExecutionPolicyModelHint {
		t.Fatalf("runtime_policy.policy_model_hint = %q, want %q", got, batch1ExecutionPolicyModelHint)
	}
}

func TestMaterializeEvalGroupSpec_Batch1ExecutionAllowsEnvOverrideForMaxConcurrency(t *testing.T) {
	t.Setenv(harnessExecutionMaxConcurrencyEnv, "6")

	manifest := Batch1ExecutionDatasetManifest()
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}

	dataset := &Dataset{ID: "dataset-exec-batch1", Name: Batch1ExecutionDatasetName, Subject: Batch1ExecutionDatasetSubject, DefaultRunKind: RunKindAgentTask, DefaultProfile: batch1ExecutionProfile}
	version := &DatasetVersion{ID: "dataset-version-exec-batch1", DatasetID: dataset.ID, Version: Batch1ExecutionDatasetVersion, Manifest: manifestRaw}
	evalSpec := &EvalSpec{ID: "eval-exec-batch1", Name: Batch1ExecutionEvalName, Subject: Batch1ExecutionDatasetSubject, RunKind: RunKindAgentTask, Profile: batch1ExecutionProfile, DatasetID: dataset.ID, DatasetVersionID: version.ID}

	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, EvalRunSpec{})
	if err != nil {
		t.Fatalf("materializeEvalGroupSpec failed: %v", err)
	}
	if groupSpec.SchedulerConfig.MaxConcurrency != 6 {
		t.Fatalf("group scheduler max_concurrency = %d, want 6", groupSpec.SchedulerConfig.MaxConcurrency)
	}
}

func TestMaterializeEvalGroupSpec_Batch1ExecutionAllowsGlobalEnvOverrideForMaxConcurrency(t *testing.T) {
	t.Setenv(harnessMaxConcurrencyEnv, "7")

	manifest := Batch1ExecutionDatasetManifest()
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}

	dataset := &Dataset{ID: "dataset-exec-batch1", Name: Batch1ExecutionDatasetName, Subject: Batch1ExecutionDatasetSubject, DefaultRunKind: RunKindAgentTask, DefaultProfile: batch1ExecutionProfile}
	version := &DatasetVersion{ID: "dataset-version-exec-batch1", DatasetID: dataset.ID, Version: Batch1ExecutionDatasetVersion, Manifest: manifestRaw}
	evalSpec := &EvalSpec{ID: "eval-exec-batch1", Name: Batch1ExecutionEvalName, Subject: Batch1ExecutionDatasetSubject, RunKind: RunKindAgentTask, Profile: batch1ExecutionProfile, DatasetID: dataset.ID, DatasetVersionID: version.ID}

	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, EvalRunSpec{})
	if err != nil {
		t.Fatalf("materializeEvalGroupSpec failed: %v", err)
	}
	if groupSpec.SchedulerConfig.MaxConcurrency != 7 {
		t.Fatalf("group scheduler max_concurrency = %d, want 7", groupSpec.SchedulerConfig.MaxConcurrency)
	}
}

func TestBatch1ExecutionDatasetManifest_LocalizedCoverageSpansAllSupportedLocales(t *testing.T) {
	manifest := Batch1ExecutionDatasetManifest()

	for _, skill := range batch1ExecutionSkills {
		expectedLocales := make(map[string]struct{}, len(routingcue.SupportedLocales()))
		for _, locale := range routingcue.SupportedLocales() {
			expectedLocales[locale] = struct{}{}
		}

		seen := make(map[string]struct{}, len(expectedLocales))
		for _, item := range manifest.Items {
			if metadataString(item.Metadata, "execution_case_type") != "localized_route" {
				continue
			}
			if metadataString(item.Metadata, "primary_route") != skill {
				continue
			}
			locale := metadataString(item.Metadata, "locale")
			if locale == "" {
				t.Fatalf("%s localized case has empty locale: %+v", skill, item)
			}
			if _, ok := seen[locale]; ok {
				t.Fatalf("%s localized coverage duplicates locale %s", skill, locale)
			}
			if item.Expected["status"] != "completed" {
				t.Fatalf("%s localized expected.status = %#v, want completed", locale, item.Expected["status"])
			}
			if got := decodeStringSlice(item.Expected["forbidden_observations"]); len(got) == 0 || got[0] != "clarification_requested" {
				t.Fatalf("%s localized forbidden_observations = %#v, want clarification_requested", locale, item.Expected["forbidden_observations"])
			}
			if skill == "reminder" {
				if got := decodeStringSlice(item.Expected["required_observations"]); !reflect.DeepEqual(got, []string{"session_context_propagated"}) {
					t.Fatalf("%s localized required_observations = %#v, want [session_context_propagated]", locale, item.Expected["required_observations"])
				}
				if sessionID := metadataString(item.Input, "session_id"); sessionID == "" {
					t.Fatalf("%s localized reminder input.session_id = %q, want non-empty", locale, sessionID)
				}
			}
			seen[locale] = struct{}{}
		}

		if len(seen) != len(expectedLocales) {
			t.Fatalf("%s localized locales = %d, want %d", skill, len(seen), len(expectedLocales))
		}
		for locale := range expectedLocales {
			if _, ok := seen[locale]; !ok {
				t.Fatalf("%s localized coverage missing locale %s", skill, locale)
			}
		}
	}
}

func TestBatch1ExecutionDatasetManifest_CriticalCasesPreserveBatch1Expectations(t *testing.T) {
	manifest := Batch1ExecutionDatasetManifest()

	webLatest := findExecutionManifestItem(t, manifest.Items, "critical-web-search-latest-docs-zh-cn")
	if webLatest.Expected["status"] != "completed" {
		t.Fatalf("web latest expected.status = %#v, want completed", webLatest.Expected["status"])
	}
	if metadataString(webLatest.Metadata, "primary_route") != harnessCanonicalWebQuerySkill {
		t.Fatalf("web latest primary_route = %q, want %s", metadataString(webLatest.Metadata, "primary_route"), harnessCanonicalWebQuerySkill)
	}
	if got := decodeStringSlice(webLatest.Expected["required_observations"]); len(got) != 1 || got[0] != "evidence_tool_used" {
		t.Fatalf("web latest required_observations = %#v, want evidence_tool_used", webLatest.Expected["required_observations"])
	}
	if webLatest.Metadata["critical"] != true {
		t.Fatalf("web latest critical = %#v, want true", webLatest.Metadata["critical"])
	}

	webLatestMemoryGuard := findExecutionManifestItem(t, manifest.Items, "critical-web-search-latest-docs-memory-guard-zh-cn")
	if got := decodeStringSlice(webLatestMemoryGuard.Expected["required_observations"]); !reflect.DeepEqual(got, []string{"evidence_tool_used", "planner_memory_skipped"}) {
		t.Fatalf("web latest memory guard required_observations = %#v, want [evidence_tool_used planner_memory_skipped]", webLatestMemoryGuard.Expected["required_observations"])
	}
	if seed := metadataObjectSlice(webLatestMemoryGuard.Metadata["harness_memory_seed"]); len(seed) != 1 {
		t.Fatalf("web latest memory guard harness_memory_seed = %#v, want 1 entry", webLatestMemoryGuard.Metadata["harness_memory_seed"])
	}

	urlSummary := findExecutionManifestItem(t, manifest.Items, "critical-analyze-url-summary-zh-cn")
	if metadataString(urlSummary.Metadata, "primary_route") != "analyze" {
		t.Fatalf("url summary primary_route = %q, want analyze", metadataString(urlSummary.Metadata, "primary_route"))
	}
	if metadataString(urlSummary.Metadata, "execution_case_type") != "url_summary_analysis" {
		t.Fatalf("url summary execution_case_type = %q, want url_summary_analysis", metadataString(urlSummary.Metadata, "execution_case_type"))
	}
	if got := decodeStringSlice(urlSummary.Expected["forbidden_observations"]); !reflect.DeepEqual(got, []string{"clarification_requested", "evidence_tool_used"}) {
		t.Fatalf("url summary forbidden_observations = %#v, want [clarification_requested evidence_tool_used]", urlSummary.Expected["forbidden_observations"])
	}
	if urlSummary.Metadata["critical"] != true {
		t.Fatalf("url summary critical = %#v, want true", urlSummary.Metadata["critical"])
	}

	urlReport := findExecutionManifestItem(t, manifest.Items, "critical-analyze-url-report-en-us")
	if metadataString(urlReport.Metadata, "execution_case_type") != "url_report_analysis" {
		t.Fatalf("url report execution_case_type = %q, want url_report_analysis", metadataString(urlReport.Metadata, "execution_case_type"))
	}

	urlReportMemoryGuard := findExecutionManifestItem(t, manifest.Items, "critical-analyze-url-report-memory-guard-en-us")
	if got := decodeStringSlice(urlReportMemoryGuard.Expected["required_observations"]); !reflect.DeepEqual(got, []string{"planner_memory_skipped"}) {
		t.Fatalf("url report memory guard required_observations = %#v, want [planner_memory_skipped]", urlReportMemoryGuard.Expected["required_observations"])
	}
	if got := decodeStringSlice(urlReportMemoryGuard.Expected["forbidden_observations"]); !reflect.DeepEqual(got, []string{"clarification_requested", "evidence_tool_used", "planner_memory_used"}) {
		t.Fatalf("url report memory guard forbidden_observations = %#v, want [clarification_requested evidence_tool_used planner_memory_used]", urlReportMemoryGuard.Expected["forbidden_observations"])
	}
	if seed := metadataObjectSlice(urlReportMemoryGuard.Metadata["harness_memory_seed"]); len(seed) != 1 {
		t.Fatalf("url report memory guard harness_memory_seed = %#v, want 1 entry", urlReportMemoryGuard.Metadata["harness_memory_seed"])
	}

	reminder := findExecutionManifestItem(t, manifest.Items, "critical-reminder-tomorrow-9-zh-cn")
	if metadataString(reminder.Metadata, "primary_route") != "reminder" {
		t.Fatalf("reminder primary_route = %q, want reminder", metadataString(reminder.Metadata, "primary_route"))
	}
	if got := decodeStringSlice(reminder.Expected["forbidden_observations"]); len(got) != 1 || got[0] != "clarification_requested" {
		t.Fatalf("reminder forbidden_observations = %#v, want clarification_requested", reminder.Expected["forbidden_observations"])
	}
	if got := decodeStringSlice(reminder.Expected["required_observations"]); !reflect.DeepEqual(got, []string{"session_context_propagated"}) {
		t.Fatalf("reminder required_observations = %#v, want [session_context_propagated]", reminder.Expected["required_observations"])
	}
	if sessionID := metadataString(reminder.Input, "session_id"); sessionID == "" {
		t.Fatalf("reminder input.session_id = %q, want non-empty", sessionID)
	}
	if reminder.Metadata["critical"] != true {
		t.Fatalf("reminder critical = %#v, want true", reminder.Metadata["critical"])
	}
}

func TestMaterializeEvalGroupSpec_Batch1ExecutionPreservesCaseIDsAndMetadata(t *testing.T) {
	manifest := Batch1ExecutionDatasetManifest()
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}

	dataset := &Dataset{
		ID:             "dataset-batch1-exec",
		Name:           Batch1ExecutionDatasetName,
		Subject:        Batch1ExecutionDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: batch1ExecutionProfile,
	}
	version := &DatasetVersion{
		ID:        "dataset-version-batch1-exec",
		DatasetID: dataset.ID,
		Version:   Batch1ExecutionDatasetVersion,
		Manifest:  manifestRaw,
	}
	evalSpec := &EvalSpec{
		ID:               "eval-batch1-exec",
		Name:             Batch1ExecutionEvalName,
		Subject:          Batch1ExecutionDatasetSubject,
		RunKind:          RunKindAgentTask,
		Profile:          batch1ExecutionProfile,
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RuntimePolicy:    batch1ExecutionRuntimePolicy(),
	}

	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, EvalRunSpec{})
	if err != nil {
		t.Fatalf("materializeEvalGroupSpec failed: %v", err)
	}
	if len(groupSpec.Items) != Batch1ExecutionCaseCount() {
		t.Fatalf("group items len = %d, want %d", len(groupSpec.Items), Batch1ExecutionCaseCount())
	}
	if metadataString(groupSpec.Metadata, "gate_type") != "execution_equivalence" {
		t.Fatalf("group metadata gate_type = %q, want execution_equivalence", metadataString(groupSpec.Metadata, "gate_type"))
	}
	if groupSpec.SchedulerConfig.MaxConcurrency != batch1ExecutionMaxConcurrency {
		t.Fatalf("group scheduler max_concurrency = %d, want %d", groupSpec.SchedulerConfig.MaxConcurrency, batch1ExecutionMaxConcurrency)
	}

	selected := findExecutionGroupItem(t, groupSpec.Items, "exec-web_search-en-us")
	if selected.Expected["status"] != "completed" {
		t.Fatalf("selected expected.status = %#v, want completed", selected.Expected["status"])
	}
	if got := decodeStringSlice(selected.Expected["required_observations"]); len(got) != 1 || got[0] != "evidence_tool_used" {
		t.Fatalf("selected required_observations = %#v, want evidence_tool_used", selected.Expected["required_observations"])
	}
	if metadataString(selected.Metadata, "dataset_case_id") != canonicalHarnessDatasetCaseID("exec-web_search-en-us") {
		t.Fatalf("selected dataset_case_id = %q, want %s", metadataString(selected.Metadata, "dataset_case_id"), canonicalHarnessDatasetCaseID("exec-web_search-en-us"))
	}
	if metadataString(selected.Metadata, "primary_route") != harnessCanonicalWebQuerySkill {
		t.Fatalf("selected primary_route = %q, want %s", metadataString(selected.Metadata, "primary_route"), harnessCanonicalWebQuerySkill)
	}

	critical := findExecutionGroupItem(t, groupSpec.Items, "critical-analyze-url-summary-zh-cn")
	if critical.Metadata["critical"] != true {
		t.Fatalf("critical item metadata.critical = %#v, want true", critical.Metadata["critical"])
	}
	if metadataString(critical.Metadata, "execution_case_type") != "url_summary_analysis" {
		t.Fatalf("critical item execution_case_type = %q, want url_summary_analysis", metadataString(critical.Metadata, "execution_case_type"))
	}

	reminder := findExecutionGroupItem(t, groupSpec.Items, "critical-reminder-tomorrow-9-zh-cn")
	if got := decodeStringSlice(reminder.Expected["required_observations"]); !reflect.DeepEqual(got, []string{"session_context_propagated"}) {
		t.Fatalf("reminder required_observations = %#v, want [session_context_propagated]", reminder.Expected["required_observations"])
	}
	if sessionID := metadataString(reminder.Input, "session_id"); sessionID == "" {
		t.Fatalf("reminder input.session_id = %q, want non-empty", sessionID)
	}
}

func TestBatch1ExecutionDatasetSpecs_CreateReusableEvalAssets(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), Batch1ExecutionDatasetSpec("user-1"))
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	versionSpec, err := Batch1ExecutionDatasetVersionSpec("tester")
	if err != nil {
		t.Fatalf("Batch1ExecutionDatasetVersionSpec failed: %v", err)
	}
	version, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, versionSpec)
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}
	if version.ItemCount != Batch1ExecutionCaseCount() {
		t.Fatalf("version item_count = %d, want %d", version.ItemCount, Batch1ExecutionCaseCount())
	}

	evalSpec, err := controller.CreateEvalSpec(context.Background(), Batch1ExecutionEvalSpecSpec(dataset.ID, version.ID, "user-1"))
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}
	if evalSpec.DatasetVersionID != version.ID {
		t.Fatalf("eval spec dataset_version_id = %q, want %q", evalSpec.DatasetVersionID, version.ID)
	}
	if evalSpec.Profile != batch1ExecutionProfile {
		t.Fatalf("eval spec profile = %q, want %q", evalSpec.Profile, batch1ExecutionProfile)
	}
	if evalSpec.SchedulerConfig.MaxConcurrency != batch1ExecutionMaxConcurrency {
		t.Fatalf("eval spec scheduler max_concurrency = %d, want %d", evalSpec.SchedulerConfig.MaxConcurrency, batch1ExecutionMaxConcurrency)
	}
	if metadataString(evalSpec.RuntimePolicy, "gate_type") != "execution_equivalence" {
		t.Fatalf("eval spec runtime_policy.gate_type = %q, want execution_equivalence", metadataString(evalSpec.RuntimePolicy, "gate_type"))
	}
	if got := metadataString(evalSpec.RuntimePolicy, "policy_model_hint"); got != batch1ExecutionPolicyModelHint {
		t.Fatalf("eval spec runtime_policy.policy_model_hint = %q, want %q", got, batch1ExecutionPolicyModelHint)
	}
}

func findExecutionManifestItem(t *testing.T, items []DatasetManifestItem, id string) DatasetManifestItem {
	t.Helper()
	id = canonicalHarnessDatasetCaseID(id)
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("manifest item %q not found", id)
	return DatasetManifestItem{}
}

func findExecutionGroupItem(t *testing.T, items []RunGroupItemSpec, caseID string) RunGroupItemSpec {
	t.Helper()
	caseID = canonicalHarnessDatasetCaseID(caseID)
	for _, item := range items {
		if metadataString(item.Metadata, "dataset_case_id") == caseID {
			return item
		}
	}
	t.Fatalf("group item %q not found", caseID)
	return RunGroupItemSpec{}
}
