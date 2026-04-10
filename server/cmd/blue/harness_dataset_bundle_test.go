package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	harnesspkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRunHarnessDatasetImportEImportsLocalBundle(t *testing.T) {
	resetHarnessCLIState(t)

	bundleDir := writeHarnessDatasetBundleFixture(t, t.TempDir())
	var importCalls int

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/dataset-bundles/import", func(w http.ResponseWriter, r *http.Request) {
		importCalls++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		var req harnesspkg.ImportDatasetBundleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode import request: %v", err)
		}
		if req.SourceType != "dataset_bundle_local" {
			t.Fatalf("source_type = %q, want dataset_bundle_local", req.SourceType)
		}
		if req.SourceRef != bundleDir {
			t.Fatalf("source_ref = %q, want %q", req.SourceRef, bundleDir)
		}
		if req.Dataset.Name != "demo-bundle" || req.Dataset.OwnerUserID != "bundle-user" {
			t.Fatalf("dataset = %#v, want demo-bundle owned by bundle-user", req.Dataset)
		}
		if req.Version.Version != "v1" || req.Version.SourceType != "dataset_bundle_local" || req.Version.SourceRef != bundleDir {
			t.Fatalf("version = %#v, want v1 local bundle provenance", req.Version)
		}
		if len(req.EvalSpecs) != 1 || req.EvalSpecs[0].Name != "Default Demo Eval" {
			t.Fatalf("eval_specs = %#v, want Default Demo Eval", req.EvalSpecs)
		}

		_ = json.NewEncoder(w).Encode(harnesspkg.ImportDatasetBundleResult{
			Dataset:        &harnesspkg.Dataset{ID: "dataset-1", Name: req.Dataset.Name, OwnerUserID: req.Dataset.OwnerUserID},
			DatasetVersion: &harnesspkg.DatasetVersion{ID: "version-1", Version: req.Version.Version, SourceType: req.SourceType, SourceRef: req.SourceRef},
			EvalSpecs: []harnesspkg.EvalSpec{
				{ID: "eval-spec-1", Name: req.EvalSpecs[0].Name, OwnerUserID: req.Dataset.OwnerUserID, DatasetVersionID: "version-1"},
			},
		})
	})

	addr := setupHarnessCLIServer(t, mux)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", portStr)
	t.Setenv("BLUE_USER_ID", "bundle-user")

	harnessDatasetBundleLocalPath = bundleDir

	out := captureStdout(t, func() {
		err = runHarnessDatasetImportE(nil, nil)
	})
	if err != nil {
		t.Fatalf("runHarnessDatasetImportE error: %v", err)
	}
	if importCalls != 1 {
		t.Fatalf("importCalls = %d, want 1", importCalls)
	}

	var result harnesspkg.ImportDatasetBundleResult
	if decodeErr := json.Unmarshal([]byte(out), &result); decodeErr != nil {
		t.Fatalf("decode import output: %v\n%s", decodeErr, out)
	}
	if result.Dataset == nil || result.Dataset.Name != "demo-bundle" {
		t.Fatalf("result.dataset = %#v, want demo-bundle", result.Dataset)
	}
	if result.DatasetVersion == nil || result.DatasetVersion.SourceRef != bundleDir {
		t.Fatalf("result.dataset_version = %#v, want source_ref %q", result.DatasetVersion, bundleDir)
	}
}

func TestRunHarnessDatasetPullEFetchesGitHubBundleWithFallback(t *testing.T) {
	resetHarnessCLIState(t)

	datasetYAML, manifestJSON, evalSpecYAML := harnessDatasetBundleFixtureContents()
	var importCalls int
	var requests []string

	oldClient := harnessDatasetBundleHTTPClient
	harnessDatasetBundleHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests = append(requests, req.URL.String())
			switch req.URL.Host {
			case "raw.githubusercontent.com", "raw.gitmirror.com", "cdn.jsdelivr.net":
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			case "ghproxy.com":
				body := ""
				switch {
				case strings.Contains(req.URL.String(), "/demo-bundle/dataset.yaml"):
					body = datasetYAML
				case strings.Contains(req.URL.String(), "/demo-bundle/versions/v1/manifest.json"):
					body = manifestJSON
				case strings.Contains(req.URL.String(), "/demo-bundle/versions/v1/eval-specs/default.yaml"):
					body = evalSpecYAML
				default:
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Body:       io.NopCloser(strings.NewReader("not found")),
						Header:     make(http.Header),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
					Header:     make(http.Header),
				}, nil
			default:
				return nil, fmt.Errorf("unexpected host %q", req.URL.Host)
			}
		}),
	}
	t.Cleanup(func() {
		harnessDatasetBundleHTTPClient = oldClient
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/harness/dataset-bundles/import", func(w http.ResponseWriter, r *http.Request) {
		importCalls++
		var req harnesspkg.ImportDatasetBundleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode import request: %v", err)
		}
		if req.SourceType != "dataset_bundle_github" {
			t.Fatalf("source_type = %q, want dataset_bundle_github", req.SourceType)
		}
		if req.SourceRef != "https://github.com/example/harness-datasets/tree/main/demo-bundle" {
			t.Fatalf("source_ref = %q, want canonical tree url", req.SourceRef)
		}
		if req.Dataset.Name != "demo-bundle" || req.Version.Version != "v1" {
			t.Fatalf("request = %#v, want demo-bundle/v1", req)
		}
		_ = json.NewEncoder(w).Encode(harnesspkg.ImportDatasetBundleResult{
			Dataset:        &harnesspkg.Dataset{ID: "dataset-1", Name: req.Dataset.Name, OwnerUserID: req.Dataset.OwnerUserID},
			DatasetVersion: &harnesspkg.DatasetVersion{ID: "version-1", Version: req.Version.Version, SourceType: req.SourceType, SourceRef: req.SourceRef},
			EvalSpecs: []harnesspkg.EvalSpec{
				{ID: "eval-spec-1", Name: "Default Demo Eval", OwnerUserID: req.Dataset.OwnerUserID, DatasetVersionID: "version-1"},
			},
		})
	})

	addr := setupHarnessCLIServer(t, mux)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", portStr)
	t.Setenv("BLUE_USER_ID", "bundle-user")

	harnessDatasetBundleSource = "https://github.com/example/harness-datasets/tree/main/demo-bundle"

	out := captureStdout(t, func() {
		err = runHarnessDatasetPullE(nil, nil)
	})
	if err != nil {
		t.Fatalf("runHarnessDatasetPullE error: %v", err)
	}
	if importCalls != 1 {
		t.Fatalf("importCalls = %d, want 1", importCalls)
	}
	if len(requests) < 4 {
		t.Fatalf("requests = %v, want at least four fallback attempts", requests)
	}
	if !strings.Contains(requests[0], "raw.githubusercontent.com") || !strings.Contains(requests[1], "raw.gitmirror.com") || !strings.Contains(requests[2], "cdn.jsdelivr.net/gh") || !strings.Contains(requests[3], "ghproxy.com") {
		t.Fatalf("fallback order = %v, want raw -> gitmirror -> jsdelivr -> ghproxy", requests[:minInt(len(requests), 4)])
	}

	var result harnesspkg.ImportDatasetBundleResult
	if decodeErr := json.Unmarshal([]byte(out), &result); decodeErr != nil {
		t.Fatalf("decode pull output: %v\n%s", decodeErr, out)
	}
	if result.Dataset == nil || result.Dataset.Name != "demo-bundle" {
		t.Fatalf("result.dataset = %#v, want demo-bundle", result.Dataset)
	}
}

func TestLoadHarnessDatasetBundleFromGitHubRepoURLWithBundlePath(t *testing.T) {
	resetHarnessCLIState(t)

	datasetYAML, manifestJSON, evalSpecYAML := harnessDatasetBundleFixtureContents()
	oldClient := harnessDatasetBundleHTTPClient
	harnessDatasetBundleHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body string
			switch {
			case strings.Contains(req.URL.String(), "/main/demo-bundle/dataset.yaml"):
				body = datasetYAML
			case strings.Contains(req.URL.String(), "/main/demo-bundle/versions/v1/manifest.json"):
				body = manifestJSON
			case strings.Contains(req.URL.String(), "/main/demo-bundle/versions/v1/eval-specs/default.yaml"):
				body = evalSpecYAML
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	t.Cleanup(func() {
		harnessDatasetBundleHTTPClient = oldClient
	})

	req, err := loadHarnessDatasetBundleFromGitHubSource("https://github.com/example/harness-datasets", "demo-bundle", "")
	if err != nil {
		t.Fatalf("loadHarnessDatasetBundleFromGitHubSource: %v", err)
	}
	if req.SourceType != "dataset_bundle_github" {
		t.Fatalf("source_type = %q, want dataset_bundle_github", req.SourceType)
	}
	if req.SourceRef != "https://github.com/example/harness-datasets/tree/main/demo-bundle" {
		t.Fatalf("source_ref = %q, want canonical main tree url", req.SourceRef)
	}
	if req.Dataset.Name != "demo-bundle" || req.Version.Version != "v1" {
		t.Fatalf("request = %#v, want demo-bundle/v1", req)
	}
}

func TestLoadHarnessDatasetBundleFromDirUsesRequestedVersionOverride(t *testing.T) {
	bundleDir := writeHarnessDatasetBundleMultiVersionFixture(t, t.TempDir())

	req, err := loadHarnessDatasetBundleFromDir(bundleDir, "v2")
	if err != nil {
		t.Fatalf("loadHarnessDatasetBundleFromDir: %v", err)
	}
	if req.Version.Version != "v2" {
		t.Fatalf("version = %q, want v2", req.Version.Version)
	}
	if len(req.EvalSpecs) != 1 || req.EvalSpecs[0].Name != "V2 Demo Eval" {
		t.Fatalf("eval_specs = %#v, want V2 Demo Eval", req.EvalSpecs)
	}
}

func TestLoadHarnessDatasetBundleFromGitHubSourceUsesRequestedVersionOverride(t *testing.T) {
	resetHarnessCLIState(t)

	datasetYAML, v1ManifestJSON, v1EvalSpecYAML, v2ManifestJSON, v2EvalSpecYAML := harnessDatasetBundleMultiVersionFixtureContents()
	oldClient := harnessDatasetBundleHTTPClient
	harnessDatasetBundleHTTPClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			var body string
			switch {
			case strings.Contains(req.URL.String(), "/demo-bundle/dataset.yaml"):
				body = datasetYAML
			case strings.Contains(req.URL.String(), "/demo-bundle/versions/v1/manifest.json"):
				body = v1ManifestJSON
			case strings.Contains(req.URL.String(), "/demo-bundle/versions/v1/eval-specs/default.yaml"):
				body = v1EvalSpecYAML
			case strings.Contains(req.URL.String(), "/demo-bundle/versions/v2/manifest.json"):
				body = v2ManifestJSON
			case strings.Contains(req.URL.String(), "/demo-bundle/versions/v2/eval-specs/default.yaml"):
				body = v2EvalSpecYAML
			default:
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	t.Cleanup(func() {
		harnessDatasetBundleHTTPClient = oldClient
	})

	req, err := loadHarnessDatasetBundleFromGitHubSource("https://github.com/example/harness-datasets/tree/main/demo-bundle", "", "v2")
	if err != nil {
		t.Fatalf("loadHarnessDatasetBundleFromGitHubSource: %v", err)
	}
	if req.Version.Version != "v2" {
		t.Fatalf("version = %q, want v2", req.Version.Version)
	}
	if len(req.EvalSpecs) != 1 || req.EvalSpecs[0].Name != "V2 Demo Eval" {
		t.Fatalf("eval_specs = %#v, want V2 Demo Eval", req.EvalSpecs)
	}
}

func TestLoadHarnessDatasetBundleFromDirRejectsMissingManifest(t *testing.T) {
	bundleDir := filepath.Join(t.TempDir(), "broken-bundle")
	if err := os.MkdirAll(filepath.Join(bundleDir, "versions", "v1"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "dataset.yaml"), []byte("api_version: harness.blue/v1alpha1\nkind: dataset_bundle\nname: broken\ndefault_version: v1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile dataset.yaml: %v", err)
	}

	_, err := loadHarnessDatasetBundleFromDir(bundleDir, "")
	if err == nil {
		t.Fatal("expected missing manifest error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "manifest") {
		t.Fatalf("error = %v, want manifest-related failure", err)
	}
}

func TestLoadHarnessDatasetBundleFromDirRejectsInvalidDatasetYAML(t *testing.T) {
	bundleDir := filepath.Join(t.TempDir(), "broken-bundle")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "dataset.yaml"), []byte(":\n  invalid"), 0o600); err != nil {
		t.Fatalf("WriteFile dataset.yaml: %v", err)
	}

	_, err := loadHarnessDatasetBundleFromDir(bundleDir, "")
	if err == nil {
		t.Fatal("expected invalid dataset yaml error")
	}
}

func TestLoadHarnessDatasetBundleFromDirRejectsInvalidEvalSpecYAML(t *testing.T) {
	bundleDir := filepath.Join(t.TempDir(), "broken-bundle")
	if err := os.MkdirAll(filepath.Join(bundleDir, "versions", "v1", "eval-specs"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "dataset.yaml"), []byte(`api_version: harness.blue/v1alpha1
kind: dataset_bundle
name: broken
default_version: v1
versions:
  v1:
    eval_specs:
      - invalid.yaml
`), 0o600); err != nil {
		t.Fatalf("WriteFile dataset.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "versions", "v1", "manifest.json"), []byte(`{"items":[]}`), 0o600); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "versions", "v1", "eval-specs", "invalid.yaml"), []byte("subject: demo_subject\n"), 0o600); err != nil {
		t.Fatalf("WriteFile invalid eval spec: %v", err)
	}

	_, err := loadHarnessDatasetBundleFromDir(bundleDir, "")
	if err == nil {
		t.Fatal("expected invalid eval spec yaml error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "eval spec") && !strings.Contains(strings.ToLower(err.Error()), "name") {
		t.Fatalf("error = %v, want eval-spec validation failure", err)
	}
}

func TestLoadHarnessDatasetBundleFromCommittedFixture(t *testing.T) {
	workdir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	bundleDir := filepath.Join(workdir, "..", "..", "..", "harness", "datasets", "demo-bundle")

	req, err := loadHarnessDatasetBundleFromDir(bundleDir, "")
	if err != nil {
		t.Fatalf("loadHarnessDatasetBundleFromDir fixture: %v", err)
	}
	if req.SourceType != "dataset_bundle_local" {
		t.Fatalf("source_type = %q, want dataset_bundle_local", req.SourceType)
	}
	if req.Dataset.Name != "demo-bundle" || req.Version.Version != "v1" {
		t.Fatalf("request = %#v, want demo-bundle/v1", req)
	}
	if len(req.EvalSpecs) != 1 || req.EvalSpecs[0].Name != "Default Demo Eval" {
		t.Fatalf("eval_specs = %#v, want one Default Demo Eval", req.EvalSpecs)
	}
}

func TestLoadHarnessDatasetBundleFromCommittedPinchBenchSamplesFixture(t *testing.T) {
	workdir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	bundleDir := filepath.Join(workdir, "..", "..", "..", "harness", "datasets", "pinchbench-samples")

	req, err := loadHarnessDatasetBundleFromDir(bundleDir, "")
	if err != nil {
		t.Fatalf("loadHarnessDatasetBundleFromDir fixture: %v", err)
	}
	if req.SourceType != "dataset_bundle_local" {
		t.Fatalf("source_type = %q, want dataset_bundle_local", req.SourceType)
	}
	if req.Dataset.Name != "pinchbench-samples" || req.Version.Version != "v1" {
		t.Fatalf("request = %#v, want pinchbench-samples/v1", req)
	}
	if req.Dataset.Subject != "pinchbench_samples" {
		t.Fatalf("dataset.subject = %q, want pinchbench_samples", req.Dataset.Subject)
	}
	if req.Dataset.DefaultRunKind != harnesspkg.RunKindAgentTask {
		t.Fatalf("dataset.default_run_kind = %q, want %q", req.Dataset.DefaultRunKind, harnesspkg.RunKindAgentTask)
	}
	if req.Dataset.DefaultProfile != "pinchbench_samples" {
		t.Fatalf("dataset.default_profile = %q, want pinchbench_samples", req.Dataset.DefaultProfile)
	}
	if got := fmt.Sprint(req.Dataset.Metadata["source_benchmark"]); got != "pinchbench" {
		t.Fatalf("dataset.metadata.source_benchmark = %q, want pinchbench", got)
	}
	if got := fmt.Sprint(req.Dataset.Metadata["sample_scope"]); got != "deterministic_v1" {
		t.Fatalf("dataset.metadata.sample_scope = %q, want deterministic_v1", got)
	}

	if len(req.EvalSpecs) != 1 || req.EvalSpecs[0].Name != "PinchBench Samples v1" {
		t.Fatalf("eval_specs = %#v, want one PinchBench Samples v1", req.EvalSpecs)
	}
	if req.EvalSpecs[0].RunKind != harnesspkg.RunKindAgentTask {
		t.Fatalf("eval_specs[0].run_kind = %q, want %q", req.EvalSpecs[0].RunKind, harnesspkg.RunKindAgentTask)
	}
	if req.EvalSpecs[0].Profile != "pinchbench_samples" {
		t.Fatalf("eval_specs[0].profile = %q, want pinchbench_samples", req.EvalSpecs[0].Profile)
	}
	if req.EvalSpecs[0].ScoringConfig.Mode != harnesspkg.ScoringModeRule || req.EvalSpecs[0].ScoringConfig.PassThreshold != 1 {
		t.Fatalf("eval_specs[0].scoring = %#v, want rule/1", req.EvalSpecs[0].ScoringConfig)
	}
	if req.EvalSpecs[0].SchedulerConfig.MaxConcurrency != 2 || req.EvalSpecs[0].SchedulerConfig.MaxAttempts != 2 {
		t.Fatalf("eval_specs[0].scheduler = %#v, want max_concurrency/max_attempts = 2", req.EvalSpecs[0].SchedulerConfig)
	}

	manifest, err := (&harnesspkg.DatasetVersion{Manifest: req.Version.Manifest}).DecodeManifest()
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if len(manifest.Items) != 6 {
		t.Fatalf("manifest items len = %d, want 6", len(manifest.Items))
	}

	seenIDs := make(map[string]struct{}, len(manifest.Items))
	seenLocales := map[string]bool{
		"en-US": false,
		"zh-CN": false,
	}
	memoryGuardCount := 0

	for _, item := range manifest.Items {
		if item.ID == "" {
			t.Fatal("manifest item has empty id")
		}
		if _, exists := seenIDs[item.ID]; exists {
			t.Fatalf("duplicate manifest item id %q", item.ID)
		}
		seenIDs[item.ID] = struct{}{}

		locale := fmt.Sprint(item.Metadata["locale"])
		if _, ok := seenLocales[locale]; ok {
			seenLocales[locale] = true
		}

		if fmt.Sprint(item.Metadata["sample_family"]) == "memory_guard" {
			memoryGuardCount++
			if len(harnesspkg.DecodeHarnessContract(item.Metadata).RequiredObservations) == 0 && len(harnesspkg.DecodeHarnessContract(item.Expected).RequiredObservations) == 0 {
				t.Fatalf("%s missing required observations", item.ID)
			}
			if len(harnesspkg.DecodeHarnessContract(item.Metadata).ForbiddenObservations) == 0 && len(harnesspkg.DecodeHarnessContract(item.Expected).ForbiddenObservations) == 0 {
				t.Fatalf("%s missing forbidden observations", item.ID)
			}
			if _, ok := item.Metadata["harness_memory_seed"]; !ok {
				t.Fatalf("%s missing harness_memory_seed metadata", item.ID)
			}
		}
	}

	for locale, seen := range seenLocales {
		if !seen {
			t.Fatalf("locale %s missing from manifest", locale)
		}
	}
	if memoryGuardCount != 2 {
		t.Fatalf("memory guard case count = %d, want 2", memoryGuardCount)
	}
}

func TestLoadHarnessDatasetBundleFromCommittedPinchBenchFixture(t *testing.T) {
	workdir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	bundleDir := filepath.Join(workdir, "..", "..", "..", "harness", "datasets", "pinchbench")

	req, err := loadHarnessDatasetBundleFromDir(bundleDir, "")
	if err != nil {
		t.Fatalf("loadHarnessDatasetBundleFromDir fixture: %v", err)
	}
	if req.SourceType != "dataset_bundle_local" {
		t.Fatalf("source_type = %q, want dataset_bundle_local", req.SourceType)
	}
	if req.Dataset.Name != "pinchbench" || req.Version.Version != "v1" {
		t.Fatalf("request = %#v, want pinchbench/v1", req)
	}
	if req.Dataset.Subject != "pinchbench" {
		t.Fatalf("dataset.subject = %q, want pinchbench", req.Dataset.Subject)
	}
	if req.Dataset.DefaultRunKind != harnesspkg.RunKindAgentTask {
		t.Fatalf("dataset.default_run_kind = %q, want %q", req.Dataset.DefaultRunKind, harnesspkg.RunKindAgentTask)
	}
	if req.Dataset.DefaultProfile != "pinchbench" {
		t.Fatalf("dataset.default_profile = %q, want pinchbench", req.Dataset.DefaultProfile)
	}
	if got := fmt.Sprint(req.Dataset.Metadata["source_benchmark"]); got != "pinchbench" {
		t.Fatalf("dataset.metadata.source_benchmark = %q, want pinchbench", got)
	}
	if got := fmt.Sprint(req.Dataset.Metadata["task_count"]); got != "23" {
		t.Fatalf("dataset.metadata.task_count = %q, want 23", got)
	}

	if len(req.EvalSpecs) != 1 || req.EvalSpecs[0].Name != "PinchBench v1" {
		t.Fatalf("eval_specs = %#v, want one PinchBench v1", req.EvalSpecs)
	}
	if req.EvalSpecs[0].RunKind != harnesspkg.RunKindAgentTask {
		t.Fatalf("eval_specs[0].run_kind = %q, want %q", req.EvalSpecs[0].RunKind, harnesspkg.RunKindAgentTask)
	}
	if req.EvalSpecs[0].Profile != "pinchbench" {
		t.Fatalf("eval_specs[0].profile = %q, want pinchbench", req.EvalSpecs[0].Profile)
	}
	if req.EvalSpecs[0].ScoringConfig.Mode != harnesspkg.ScoringModeHybrid {
		t.Fatalf("eval_specs[0].scoring.mode = %q, want hybrid", req.EvalSpecs[0].ScoringConfig.Mode)
	}
	if req.EvalSpecs[0].ScoringConfig.JudgeModel != "claude-sonnet-4-6" {
		t.Fatalf("eval_specs[0].scoring.judge_model = %q, want claude-sonnet-4-6", req.EvalSpecs[0].ScoringConfig.JudgeModel)
	}
	if req.EvalSpecs[0].SchedulerConfig.MaxAttempts != 2 {
		t.Fatalf("eval_specs[0].scheduler.max_attempts = %d, want 2", req.EvalSpecs[0].SchedulerConfig.MaxAttempts)
	}

	manifest, err := (&harnesspkg.DatasetVersion{Manifest: req.Version.Manifest}).DecodeManifest()
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	if len(manifest.Items) != 23 {
		t.Fatalf("manifest items len = %d, want 23", len(manifest.Items))
	}

	var dailySummary *harnesspkg.DatasetManifestItem
	var eli5PDF *harnesspkg.DatasetManifestItem
	for i := range manifest.Items {
		item := &manifest.Items[i]
		switch item.ID {
		case "task_15_daily_summary":
			dailySummary = item
		case "task_20_eli5_pdf_summary":
			eli5PDF = item
		}
	}
	if dailySummary == nil {
		t.Fatal("expected task_15_daily_summary item in manifest")
	}
	if eli5PDF == nil {
		t.Fatal("expected task_20_eli5_pdf_summary item in manifest")
	}
	if got := fmt.Sprint(dailySummary.Metadata["pinchbench_grading_type"]); got != "llm_judge" {
		t.Fatalf("task_15 metadata grading_type = %q, want llm_judge", got)
	}
	if files, ok := dailySummary.Metadata["harness_workspace_files"].([]interface{}); !ok || len(files) != 5 {
		t.Fatalf("task_15 harness_workspace_files = %#v, want 5 entries", dailySummary.Metadata["harness_workspace_files"])
	}
	if artifacts, ok := eli5PDF.Expected["expected_artifacts"].([]interface{}); !ok || len(artifacts) == 0 || fmt.Sprint(artifacts[0]) != "eli5_summary.txt" {
		t.Fatalf("task_20 expected_artifacts = %#v, want eli5_summary.txt", eli5PDF.Expected["expected_artifacts"])
	}
	if files, ok := eli5PDF.Metadata["harness_workspace_files"].([]interface{}); !ok || len(files) != 1 {
		t.Fatalf("task_20 harness_workspace_files = %#v, want 1 entry", eli5PDF.Metadata["harness_workspace_files"])
	}
}

func TestResolveHarnessDatasetGitHubSourceRequiresBundlePathForRepoURL(t *testing.T) {
	_, err := resolveHarnessDatasetGitHubSource("https://github.com/example/harness-datasets", "")
	if err == nil {
		t.Fatal("expected missing bundle-path error")
	}
	if !strings.Contains(err.Error(), "--bundle-path is required") {
		t.Fatalf("error = %v, want bundle-path guidance", err)
	}
}

func writeHarnessDatasetBundleFixture(t *testing.T, root string) string {
	t.Helper()

	bundleDir := filepath.Join(root, "demo-bundle")
	if err := os.MkdirAll(filepath.Join(bundleDir, "versions", "v1", "eval-specs"), 0o755); err != nil {
		t.Fatalf("MkdirAll fixture: %v", err)
	}
	datasetYAML, manifestJSON, evalSpecYAML := harnessDatasetBundleFixtureContents()
	if err := os.WriteFile(filepath.Join(bundleDir, "dataset.yaml"), []byte(datasetYAML), 0o600); err != nil {
		t.Fatalf("WriteFile dataset.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "versions", "v1", "manifest.json"), []byte(manifestJSON), 0o600); err != nil {
		t.Fatalf("WriteFile manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "versions", "v1", "eval-specs", "default.yaml"), []byte(evalSpecYAML), 0o600); err != nil {
		t.Fatalf("WriteFile eval spec: %v", err)
	}
	return bundleDir
}

func writeHarnessDatasetBundleMultiVersionFixture(t *testing.T, root string) string {
	t.Helper()

	bundleDir := filepath.Join(root, "demo-bundle")
	if err := os.MkdirAll(filepath.Join(bundleDir, "versions", "v1", "eval-specs"), 0o755); err != nil {
		t.Fatalf("MkdirAll v1 fixture: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(bundleDir, "versions", "v2", "eval-specs"), 0o755); err != nil {
		t.Fatalf("MkdirAll v2 fixture: %v", err)
	}
	datasetYAML, v1ManifestJSON, v1EvalSpecYAML, v2ManifestJSON, v2EvalSpecYAML := harnessDatasetBundleMultiVersionFixtureContents()
	if err := os.WriteFile(filepath.Join(bundleDir, "dataset.yaml"), []byte(datasetYAML), 0o600); err != nil {
		t.Fatalf("WriteFile dataset.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "versions", "v1", "manifest.json"), []byte(v1ManifestJSON), 0o600); err != nil {
		t.Fatalf("WriteFile v1 manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "versions", "v1", "eval-specs", "default.yaml"), []byte(v1EvalSpecYAML), 0o600); err != nil {
		t.Fatalf("WriteFile v1 eval spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "versions", "v2", "manifest.json"), []byte(v2ManifestJSON), 0o600); err != nil {
		t.Fatalf("WriteFile v2 manifest.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "versions", "v2", "eval-specs", "default.yaml"), []byte(v2EvalSpecYAML), 0o600); err != nil {
		t.Fatalf("WriteFile v2 eval spec: %v", err)
	}
	return bundleDir
}

func harnessDatasetBundleFixtureContents() (string, string, string) {
	return `api_version: harness.blue/v1alpha1
kind: dataset_bundle
name: demo-bundle
description: Demo bundle dataset
subject: demo_subject
default_run_kind: agent_task
default_profile: demo-profile
default_version: v1
metadata:
  suite: demo
versions:
  v1:
    eval_specs:
      - default.yaml
`,
		`{
  "dataset": {
    "name": "demo-bundle",
    "subject": "demo_subject"
  },
  "defaults": {
    "run_kind": "agent_task",
    "profile": "demo-profile",
    "scoring": {
      "mode": "rule",
      "pass_threshold": 1
    }
  },
  "items": [
    {
      "id": "case-1",
      "run_kind": "agent_task",
      "profile": "demo-profile",
      "input": {
        "goal": "Run demo imported bundle"
      },
      "expected": {
        "result": "completed"
      },
      "metadata": {
        "critical": true
      }
    }
  ]
}
`,
		`name: Default Demo Eval
subject: demo_subject
run_kind: agent_task
profile: demo-profile
scoring:
  mode: rule
  pass_threshold: 1
metadata:
  lane: default
`
}

func harnessDatasetBundleMultiVersionFixtureContents() (string, string, string, string, string) {
	return `api_version: harness.blue/v1alpha1
kind: dataset_bundle
name: demo-bundle
description: Demo bundle dataset
subject: demo_subject
default_run_kind: agent_task
default_profile: demo-profile
default_version: v1
metadata:
  suite: demo
versions:
  v1:
    eval_specs:
      - default.yaml
  v2:
    eval_specs:
      - default.yaml
`,
		`{
  "dataset": {
    "name": "demo-bundle",
    "subject": "demo_subject"
  },
  "defaults": {
    "run_kind": "agent_task",
    "profile": "demo-profile"
  },
  "items": [
    {
      "id": "case-v1",
      "run_kind": "agent_task",
      "profile": "demo-profile",
      "input": {
        "goal": "Run demo v1 bundle"
      }
    }
  ]
}
`,
		`name: Default Demo Eval
subject: demo_subject
run_kind: agent_task
profile: demo-profile
scoring:
  mode: rule
  pass_threshold: 1
metadata:
  lane: v1
`,
		`{
  "dataset": {
    "name": "demo-bundle",
    "subject": "demo_subject"
  },
  "defaults": {
    "run_kind": "agent_task",
    "profile": "demo-profile-v2"
  },
  "items": [
    {
      "id": "case-v2",
      "run_kind": "agent_task",
      "profile": "demo-profile-v2",
      "input": {
        "goal": "Run demo v2 bundle"
      }
    }
  ]
}
`,
		`name: V2 Demo Eval
subject: demo_subject
run_kind: agent_task
profile: demo-profile-v2
scoring:
  mode: rule
  pass_threshold: 1
metadata:
  lane: v2
`
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
