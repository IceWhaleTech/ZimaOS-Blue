package harness

import (
	"context"
	"reflect"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
)

func TestSelectorCuratedDatasetManifest_DecodesAndCountsCases(t *testing.T) {
	manifest := SelectorCuratedDatasetManifest()
	if manifest.Dataset.Name != SelectorCuratedDatasetName {
		t.Fatalf("dataset name = %q, want %q", manifest.Dataset.Name, SelectorCuratedDatasetName)
	}
	if manifest.Dataset.Subject != SelectorCuratedDatasetSubject {
		t.Fatalf("dataset subject = %q, want %q", manifest.Dataset.Subject, SelectorCuratedDatasetSubject)
	}
	if manifest.Defaults.RunKind != RunKindAgentTask {
		t.Fatalf("defaults.run_kind = %q, want %q", manifest.Defaults.RunKind, RunKindAgentTask)
	}
	if manifest.Defaults.Profile != selectorCuratedProfile {
		t.Fatalf("defaults.profile = %q, want %q", manifest.Defaults.Profile, selectorCuratedProfile)
	}
	if manifest.Defaults.Scheduler.MaxConcurrency != selectorCuratedMaxConcurrency {
		t.Fatalf("defaults.scheduler.max_concurrency = %d, want %d", manifest.Defaults.Scheduler.MaxConcurrency, selectorCuratedMaxConcurrency)
	}
	if len(manifest.Items) != SelectorCuratedCaseCount() {
		t.Fatalf("items len = %d, want %d", len(manifest.Items), SelectorCuratedCaseCount())
	}

	raw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}
	decoded, err := decodeDatasetManifest(raw)
	if err != nil {
		t.Fatalf("decodeDatasetManifest failed: %v", err)
	}
	if len(decoded.Items) != SelectorCuratedCaseCount() {
		t.Fatalf("decoded items len = %d, want %d", len(decoded.Items), SelectorCuratedCaseCount())
	}

	runtimePolicy := decoded.Defaults.RuntimePolicy
	if metadataString(runtimePolicy, "target_endpoint") != selectorDryRunTarget {
		t.Fatalf("target_endpoint = %q, want %q", metadataString(runtimePolicy, "target_endpoint"), selectorDryRunTarget)
	}
	if metadataString(runtimePolicy, "request_method") != selectorDryRunRequestMethod {
		t.Fatalf("request_method = %q, want %q", metadataString(runtimePolicy, "request_method"), selectorDryRunRequestMethod)
	}
	if got := decodeStringSlice(runtimePolicy["compare_fields"]); !reflect.DeepEqual(got, selectorDryRunCompareFields) {
		t.Fatalf("compare_fields = %#v, want %#v", got, selectorDryRunCompareFields)
	}

	seen := make(map[string]struct{}, len(decoded.Items))
	for _, item := range decoded.Items {
		if item.ID == "" {
			t.Fatal("manifest item has empty id")
		}
		if _, ok := seen[item.ID]; ok {
			t.Fatalf("duplicate manifest item id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
}

func TestMaterializeEvalGroupSpec_SelectorCuratedDatasetAllowsEnvOverrideForMaxConcurrency(t *testing.T) {
	t.Setenv(harnessSelectorMaxConcurrencyEnv, "16")

	manifest := SelectorCuratedDatasetManifest()
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}

	dataset := &Dataset{ID: "dataset-selector-curated", Name: SelectorCuratedDatasetName, Subject: SelectorCuratedDatasetSubject, DefaultRunKind: RunKindAgentTask, DefaultProfile: selectorCuratedProfile}
	version := &DatasetVersion{ID: "dataset-version-selector-curated", DatasetID: dataset.ID, Version: SelectorCuratedDatasetVersion, Manifest: manifestRaw}
	evalSpec := &EvalSpec{ID: "eval-selector-curated", Name: SelectorCuratedEvalName, Subject: SelectorCuratedDatasetSubject, RunKind: RunKindAgentTask, Profile: selectorCuratedProfile, DatasetID: dataset.ID, DatasetVersionID: version.ID}

	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, EvalRunSpec{})
	if err != nil {
		t.Fatalf("materializeEvalGroupSpec failed: %v", err)
	}
	if groupSpec.SchedulerConfig.MaxConcurrency != 16 {
		t.Fatalf("group scheduler max_concurrency = %d, want 16", groupSpec.SchedulerConfig.MaxConcurrency)
	}
}

func TestMaterializeEvalGroupSpec_SelectorCuratedDatasetAllowsGlobalEnvOverrideForMaxConcurrency(t *testing.T) {
	t.Setenv(harnessMaxConcurrencyEnv, "12")

	manifest := SelectorCuratedDatasetManifest()
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}

	dataset := &Dataset{ID: "dataset-selector-curated", Name: SelectorCuratedDatasetName, Subject: SelectorCuratedDatasetSubject, DefaultRunKind: RunKindAgentTask, DefaultProfile: selectorCuratedProfile}
	version := &DatasetVersion{ID: "dataset-version-selector-curated", DatasetID: dataset.ID, Version: SelectorCuratedDatasetVersion, Manifest: manifestRaw}
	evalSpec := &EvalSpec{ID: "eval-selector-curated", Name: SelectorCuratedEvalName, Subject: SelectorCuratedDatasetSubject, RunKind: RunKindAgentTask, Profile: selectorCuratedProfile, DatasetID: dataset.ID, DatasetVersionID: version.ID}

	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, EvalRunSpec{})
	if err != nil {
		t.Fatalf("materializeEvalGroupSpec failed: %v", err)
	}
	if groupSpec.SchedulerConfig.MaxConcurrency != 12 {
		t.Fatalf("group scheduler max_concurrency = %d, want 12", groupSpec.SchedulerConfig.MaxConcurrency)
	}
}

func TestSelectorCuratedDatasetManifest_CriticalCasesPreserveExpectations(t *testing.T) {
	manifest := SelectorCuratedDatasetManifest()

	selected := findSelectorManifestItem(t, manifest.Items, "selected-web_search-en-us")
	if selected.Expected["canonical_skill_id"] != harnessCanonicalWebQuerySkill {
		t.Fatalf("selected expected canonical_skill_id = %#v, want %s", selected.Expected["canonical_skill_id"], harnessCanonicalWebQuerySkill)
	}
	if selected.Expected["skill_route_outcome"] != "selected" {
		t.Fatalf("selected expected skill_route_outcome = %#v, want selected", selected.Expected["skill_route_outcome"])
	}
	if selected.Expected["skill_need_clarify"] != false {
		t.Fatalf("selected expected skill_need_clarify = %#v, want false", selected.Expected["skill_need_clarify"])
	}

	clarify := findSelectorManifestItem(t, manifest.Items, "clarify-mixed-local-web-zh-cn")
	if _, ok := clarify.Expected["canonical_skill_id"]; ok {
		t.Fatalf("clarify case unexpectedly pins canonical_skill_id: %#v", clarify.Expected["canonical_skill_id"])
	}
	if clarify.Expected["skill_route_outcome"] != "clarify" {
		t.Fatalf("clarify route outcome = %#v, want clarify", clarify.Expected["skill_route_outcome"])
	}
	if clarify.Expected["skill_need_clarify"] != true {
		t.Fatalf("clarify need_clarify = %#v, want true", clarify.Expected["skill_need_clarify"])
	}
	if got := decodeStringSlice(clarify.Metadata["compare_fields"]); !reflect.DeepEqual(got, []string{"skill_route_outcome", "skill_need_clarify"}) {
		t.Fatalf("clarify compare_fields = %#v, want %#v", got, []string{"skill_route_outcome", "skill_need_clarify"})
	}
	if clarify.Metadata["critical"] != true {
		t.Fatalf("clarify critical = %#v, want true", clarify.Metadata["critical"])
	}

	workspaceAnalyze := findSelectorManifestItem(t, manifest.Items, "selected-analyze-workspace-report-en-us")
	if workspaceAnalyze.Expected["canonical_skill_id"] != "exec" {
		t.Fatalf("workspace analyze expected canonical_skill_id = %#v, want exec", workspaceAnalyze.Expected["canonical_skill_id"])
	}
	if metadataString(workspaceAnalyze.Metadata, "selector_case_type") != "workspace_report_analysis" {
		t.Fatalf("workspace analyze selector_case_type = %q, want workspace_report_analysis", metadataString(workspaceAnalyze.Metadata, "selector_case_type"))
	}
	if workspaceAnalyze.Metadata["critical"] != true {
		t.Fatalf("workspace analyze critical = %#v, want true", workspaceAnalyze.Metadata["critical"])
	}

	workspaceReadme := findSelectorManifestItem(t, manifest.Items, "selected-analyze-workspace-readme-zh-cn")
	if workspaceReadme.Expected["canonical_skill_id"] != "exec" {
		t.Fatalf("workspace README expected canonical_skill_id = %#v, want exec", workspaceReadme.Expected["canonical_skill_id"])
	}
	if metadataString(workspaceReadme.Metadata, "selector_case_type") != "workspace_readme_local" {
		t.Fatalf("workspace README selector_case_type = %q, want workspace_readme_local", metadataString(workspaceReadme.Metadata, "selector_case_type"))
	}
	if workspaceReadme.Metadata["critical"] != true {
		t.Fatalf("workspace README critical = %#v, want true", workspaceReadme.Metadata["critical"])
	}

	webLatest := findSelectorManifestItem(t, manifest.Items, "selected-web-search-latest-docs-zh-cn")
	if webLatest.Expected["canonical_skill_id"] != harnessCanonicalWebQuerySkill {
		t.Fatalf("web latest expected canonical_skill_id = %#v, want %s", webLatest.Expected["canonical_skill_id"], harnessCanonicalWebQuerySkill)
	}
	if metadataString(webLatest.Metadata, "selector_case_type") != "latest_docs_web" {
		t.Fatalf("web latest selector_case_type = %q, want latest_docs_web", metadataString(webLatest.Metadata, "selector_case_type"))
	}
	if webLatest.Metadata["critical"] != true {
		t.Fatalf("web latest critical = %#v, want true", webLatest.Metadata["critical"])
	}

	reminder := findSelectorManifestItem(t, manifest.Items, "selected-reminder-tomorrow-9-zh-cn")
	if reminder.Expected["canonical_skill_id"] != "reminder" {
		t.Fatalf("reminder expected canonical_skill_id = %#v, want reminder", reminder.Expected["canonical_skill_id"])
	}
	if metadataString(reminder.Metadata, "selector_case_type") != "reminder_schedule" {
		t.Fatalf("reminder selector_case_type = %q, want reminder_schedule", metadataString(reminder.Metadata, "selector_case_type"))
	}
	if reminder.Metadata["critical"] != true {
		t.Fatalf("reminder critical = %#v, want true", reminder.Metadata["critical"])
	}

	emailCLI := findSelectorManifestItem(t, manifest.Items, "selected-himalaya-email-cli-en-us")
	if emailCLI.Expected["canonical_skill_id"] != "himalaya" {
		t.Fatalf("email CLI expected canonical_skill_id = %#v, want himalaya", emailCLI.Expected["canonical_skill_id"])
	}
	if metadataString(emailCLI.Metadata, "selector_case_type") != "email_cli" {
		t.Fatalf("email CLI selector_case_type = %q, want email_cli", metadataString(emailCLI.Metadata, "selector_case_type"))
	}
	if emailCLI.Metadata["critical"] != true {
		t.Fatalf("email CLI critical = %#v, want true", emailCLI.Metadata["critical"])
	}

	bypass := findSelectorManifestItem(t, manifest.Items, "ui-review-url-zh-cn")
	if bypass.Expected["canonical_skill_id"] != "ui_reviewer" {
		t.Fatalf("bypass expected canonical_skill_id = %#v, want ui_reviewer", bypass.Expected["canonical_skill_id"])
	}
	if metadataString(bypass.Metadata, "selector_case_type") != "url_bypass" {
		t.Fatalf("bypass selector_case_type = %q, want url_bypass", metadataString(bypass.Metadata, "selector_case_type"))
	}
	if bypass.Metadata["critical"] != true {
		t.Fatalf("bypass critical = %#v, want true", bypass.Metadata["critical"])
	}
}

func TestSelectorCuratedDatasetManifest_URLBypassCasesCoverAllSupportedLocales(t *testing.T) {
	manifest := SelectorCuratedDatasetManifest()

	for _, skill := range []string{"analyze", "ui_reviewer"} {
		expectedLocales := make(map[string]struct{}, len(routingcue.SupportedLocales()))
		for _, locale := range routingcue.SupportedLocales() {
			expectedLocales[locale] = struct{}{}
		}

		seen := make(map[string]struct{}, len(expectedLocales))
		for _, item := range manifest.Items {
			if metadataString(item.Metadata, "selector_case_type") != "url_bypass" {
				continue
			}
			if metadataString(item.Metadata, "primary_route") != skill {
				continue
			}
			locale := metadataString(item.Metadata, "locale")
			if locale == "" {
				t.Fatalf("%s URL bypass case has empty locale: %+v", skill, item)
			}
			if _, ok := seen[locale]; ok {
				t.Fatalf("%s URL bypass duplicates locale %s", skill, locale)
			}
			if item.Expected["canonical_skill_id"] != skill {
				t.Fatalf("%s URL bypass canonical_skill_id = %#v, want %s", locale, item.Expected["canonical_skill_id"], skill)
			}
			if item.Expected["skill_route_outcome"] != "selected" {
				t.Fatalf("%s URL bypass route_outcome = %#v, want selected", locale, item.Expected["skill_route_outcome"])
			}
			if item.Expected["skill_need_clarify"] != false {
				t.Fatalf("%s URL bypass skill_need_clarify = %#v, want false", locale, item.Expected["skill_need_clarify"])
			}
			seen[locale] = struct{}{}
		}

		if len(seen) != len(expectedLocales) {
			t.Fatalf("%s URL bypass locales = %d, want %d", skill, len(seen), len(expectedLocales))
		}
		for locale := range expectedLocales {
			if _, ok := seen[locale]; !ok {
				t.Fatalf("%s URL bypass missing locale %s", skill, locale)
			}
		}
	}
}

func TestMaterializeEvalGroupSpec_SelectorCuratedDatasetPreservesCaseIDsAndExpectedFields(t *testing.T) {
	manifest := SelectorCuratedDatasetManifest()
	manifestRaw, err := datasetManifestMap(manifest)
	if err != nil {
		t.Fatalf("datasetManifestMap failed: %v", err)
	}

	dataset := &Dataset{
		ID:             "dataset-selector-curated",
		Name:           SelectorCuratedDatasetName,
		Subject:        SelectorCuratedDatasetSubject,
		DefaultRunKind: RunKindAgentTask,
		DefaultProfile: selectorCuratedProfile,
	}
	version := &DatasetVersion{
		ID:        "dataset-version-selector-curated",
		DatasetID: dataset.ID,
		Version:   SelectorCuratedDatasetVersion,
		Manifest:  manifestRaw,
	}
	evalSpec := &EvalSpec{
		ID:               "eval-selector-curated",
		Name:             SelectorCuratedEvalName,
		Subject:          SelectorCuratedDatasetSubject,
		RunKind:          RunKindAgentTask,
		Profile:          selectorCuratedProfile,
		DatasetID:        dataset.ID,
		DatasetVersionID: version.ID,
		RuntimePolicy:    selectorDryRunRuntimePolicy(),
	}

	groupSpec, err := materializeEvalGroupSpec(evalSpec, dataset, version, EvalRunSpec{})
	if err != nil {
		t.Fatalf("materializeEvalGroupSpec failed: %v", err)
	}
	if len(groupSpec.Items) != SelectorCuratedCaseCount() {
		t.Fatalf("group items len = %d, want %d", len(groupSpec.Items), SelectorCuratedCaseCount())
	}
	if metadataString(groupSpec.Metadata, "target_endpoint") != selectorDryRunTarget {
		t.Fatalf("group target_endpoint = %q, want %q", metadataString(groupSpec.Metadata, "target_endpoint"), selectorDryRunTarget)
	}
	if groupSpec.SchedulerConfig.MaxConcurrency != selectorCuratedMaxConcurrency {
		t.Fatalf("group scheduler max_concurrency = %d, want %d", groupSpec.SchedulerConfig.MaxConcurrency, selectorCuratedMaxConcurrency)
	}

	selected := findSelectorGroupItem(t, groupSpec.Items, "selected-browser-zh-cn")
	if selected.Expected["canonical_skill_id"] != "browser" {
		t.Fatalf("selected group expected canonical_skill_id = %#v, want browser", selected.Expected["canonical_skill_id"])
	}
	if metadataString(selected.Metadata, "dataset_case_id") != "selected-browser-zh-cn" {
		t.Fatalf("selected dataset_case_id = %q, want %q", metadataString(selected.Metadata, "dataset_case_id"), "selected-browser-zh-cn")
	}
	if metadataString(selected.Metadata, "target_endpoint") != selectorDryRunTarget {
		t.Fatalf("selected target_endpoint = %q, want %q", metadataString(selected.Metadata, "target_endpoint"), selectorDryRunTarget)
	}

	clarify := findSelectorGroupItem(t, groupSpec.Items, "clarify-mixed-local-web-zh-cn")
	if clarify.Expected["skill_route_outcome"] != "clarify" {
		t.Fatalf("clarify group route outcome = %#v, want clarify", clarify.Expected["skill_route_outcome"])
	}
	if clarify.Expected["skill_need_clarify"] != true {
		t.Fatalf("clarify group need_clarify = %#v, want true", clarify.Expected["skill_need_clarify"])
	}
	if metadataString(clarify.Metadata, "dataset_case_id") != "clarify-mixed-local-web-zh-cn" {
		t.Fatalf("clarify dataset_case_id = %q, want %q", metadataString(clarify.Metadata, "dataset_case_id"), "clarify-mixed-local-web-zh-cn")
	}
}

func TestSelectorCuratedDatasetSpecs_CreateReusableEvalAssets(t *testing.T) {
	controller := newTestController(t)

	dataset, err := controller.CreateDataset(context.Background(), SelectorCuratedDatasetSpec("user-1"))
	if err != nil {
		t.Fatalf("CreateDataset failed: %v", err)
	}

	versionSpec, err := SelectorCuratedDatasetVersionSpec("tester")
	if err != nil {
		t.Fatalf("SelectorCuratedDatasetVersionSpec failed: %v", err)
	}
	version, err := controller.CreateDatasetVersion(context.Background(), dataset.ID, versionSpec)
	if err != nil {
		t.Fatalf("CreateDatasetVersion failed: %v", err)
	}
	if version.ItemCount != SelectorCuratedCaseCount() {
		t.Fatalf("version item_count = %d, want %d", version.ItemCount, SelectorCuratedCaseCount())
	}
	if dataset.ActiveVersionID == "" {
		refreshed, getErr := controller.GetDataset(context.Background(), dataset.ID)
		if getErr != nil {
			t.Fatalf("GetDataset failed: %v", getErr)
		}
		dataset = refreshed
	}
	if dataset.ActiveVersionID != version.ID {
		t.Fatalf("dataset active_version_id = %q, want %q", dataset.ActiveVersionID, version.ID)
	}

	evalSpec, err := controller.CreateEvalSpec(context.Background(), SelectorCuratedEvalSpecSpec(dataset.ID, version.ID, "user-1"))
	if err != nil {
		t.Fatalf("CreateEvalSpec failed: %v", err)
	}
	if evalSpec.DatasetVersionID != version.ID {
		t.Fatalf("eval spec dataset_version_id = %q, want %q", evalSpec.DatasetVersionID, version.ID)
	}
	if evalSpec.Profile != selectorCuratedProfile {
		t.Fatalf("eval spec profile = %q, want %q", evalSpec.Profile, selectorCuratedProfile)
	}
	if evalSpec.SchedulerConfig.MaxConcurrency != selectorCuratedMaxConcurrency {
		t.Fatalf("eval spec scheduler max_concurrency = %d, want %d", evalSpec.SchedulerConfig.MaxConcurrency, selectorCuratedMaxConcurrency)
	}
	if metadataString(evalSpec.RuntimePolicy, "target_endpoint") != selectorDryRunTarget {
		t.Fatalf("eval spec target_endpoint = %q, want %q", metadataString(evalSpec.RuntimePolicy, "target_endpoint"), selectorDryRunTarget)
	}
}

func findSelectorManifestItem(t *testing.T, items []DatasetManifestItem, id string) DatasetManifestItem {
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

func findSelectorGroupItem(t *testing.T, items []RunGroupItemSpec, caseID string) RunGroupItemSpec {
	t.Helper()
	for _, item := range items {
		if metadataString(item.Metadata, "dataset_case_id") == caseID {
			return item
		}
	}
	t.Fatalf("group item %q not found", caseID)
	return RunGroupItemSpec{}
}
