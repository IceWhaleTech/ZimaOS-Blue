package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

type datasetBundleRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn datasetBundleRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestLoadImportDatasetBundleRequestFromGitHubSourceUsesFallback(t *testing.T) {
	datasetYAML, manifestJSON, evalSpecYAML := testDatasetBundleFixtureContents()
	oldClient := datasetBundleLoaderHTTPClient
	datasetBundleLoaderHTTPClient = &http.Client{
		Transport: datasetBundleRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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
		datasetBundleLoaderHTTPClient = oldClient
	})

	req, err := loadImportDatasetBundleRequestFromGitHubSource(
		"https://github.com/example/harness-datasets/tree/main/demo-bundle",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("loadImportDatasetBundleRequestFromGitHubSource: %v", err)
	}
	if req.SourceType != "dataset_bundle_github" {
		t.Fatalf("source_type = %q, want dataset_bundle_github", req.SourceType)
	}
	if req.SourceRef != "https://github.com/example/harness-datasets/tree/main/demo-bundle" {
		t.Fatalf("source_ref = %q, want canonical tree url", req.SourceRef)
	}
	if req.Version.Version != "v1" || len(req.EvalSpecs) != 1 || req.EvalSpecs[0].Name != "Default Demo Eval" {
		t.Fatalf("request = %#v, want v1 with imported eval spec", req)
	}
}

func TestHandler_ImportDatasetBundleFromLocalSource(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	e := echo.New()
	bundleDir := writeTestDatasetBundleFixture(t, t.TempDir())

	body, err := json.Marshal(ImportDatasetBundleFromSourceRequest{
		SourceType: "local",
		Path:       bundleDir,
	})
	if err != nil {
		t.Fatalf("json.Marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/dataset-bundles/import-source", strings.NewReader(string(body)))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ImportDatasetBundleFromSource(c); err != nil {
		t.Fatalf("ImportDatasetBundleFromSource returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result ImportDatasetBundleResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Dataset == nil || result.Dataset.OwnerUserID != "user-1" {
		t.Fatalf("dataset = %#v, want owner user-1", result.Dataset)
	}
	if result.DatasetVersion == nil || result.DatasetVersion.SourceType != "dataset_bundle_local" {
		t.Fatalf("dataset version = %#v, want local imported version", result.DatasetVersion)
	}
	if len(result.EvalSpecs) != 1 || result.EvalSpecs[0].OwnerUserID != "user-1" {
		t.Fatalf("eval specs = %#v, want imported scoped eval spec", result.EvalSpecs)
	}
}

func TestHandler_PreviewDatasetBundleFromLocalSource(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	e := echo.New()
	bundleDir := writeTestDatasetBundleFixture(t, t.TempDir())

	body, err := json.Marshal(ImportDatasetBundleFromSourceRequest{
		SourceType: "local",
		Path:       bundleDir,
	})
	if err != nil {
		t.Fatalf("json.Marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/dataset-bundles/preview-source", strings.NewReader(string(body)))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.PreviewDatasetBundleFromSource(c); err != nil {
		t.Fatalf("PreviewDatasetBundleFromSource returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result DatasetBundleSourcePreview
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Dataset.Name != "demo-bundle" {
		t.Fatalf("dataset name = %q, want demo-bundle", result.Dataset.Name)
	}
	if result.Version.Version != "v1" {
		t.Fatalf("version = %q, want v1", result.Version.Version)
	}
	if result.Version.ItemCount != 1 {
		t.Fatalf("item_count = %d, want 1", result.Version.ItemCount)
	}
	if len(result.EvalSpecs) != 1 || result.EvalSpecs[0].Name != "Default Demo Eval" {
		t.Fatalf("eval specs = %#v, want imported preview eval spec", result.EvalSpecs)
	}
}

func writeTestDatasetBundleFixture(t *testing.T, root string) string {
	t.Helper()

	bundleDir := filepath.Join(root, "demo-bundle")
	if err := os.MkdirAll(filepath.Join(bundleDir, "versions", "v1", "eval-specs"), 0o755); err != nil {
		t.Fatalf("MkdirAll fixture: %v", err)
	}
	datasetYAML, manifestJSON, evalSpecYAML := testDatasetBundleFixtureContents()
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

func testDatasetBundleFixtureContents() (string, string, string) {
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

func TestHandler_PreviewDatasetBundleFromGitHubSource(t *testing.T) {
	datasetYAML, manifestJSON, evalSpecYAML := testDatasetBundleFixtureContents()
	oldClient := datasetBundleLoaderHTTPClient
	datasetBundleLoaderHTTPClient = &http.Client{
		Transport: datasetBundleRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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
	t.Cleanup(func() { datasetBundleLoaderHTTPClient = oldClient })

	controller := newTestController(t)
	handler := NewHandler(controller)
	e := echo.New()

	body, err := json.Marshal(ImportDatasetBundleFromSourceRequest{
		SourceType: "github",
		Source:     "https://github.com/example/harness-datasets/tree/main/demo-bundle",
	})
	if err != nil {
		t.Fatalf("json.Marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dataset-bundles/preview-source", strings.NewReader(string(body)))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.PreviewDatasetBundleFromSource(c); err != nil {
		t.Fatalf("PreviewDatasetBundleFromSource returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result DatasetBundleSourcePreview
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Dataset.Name != "demo-bundle" {
		t.Fatalf("dataset name = %q, want demo-bundle", result.Dataset.Name)
	}
	if result.SourceType != "dataset_bundle_github" {
		t.Fatalf("source_type = %q, want dataset_bundle_github", result.SourceType)
	}
	if result.SourceRef != "https://github.com/example/harness-datasets/tree/main/demo-bundle" {
		t.Fatalf("source_ref = %q, want canonical tree url", result.SourceRef)
	}
	if result.Version.Version != "v1" {
		t.Fatalf("version = %q, want v1", result.Version.Version)
	}
	if result.Version.ItemCount != 1 {
		t.Fatalf("item_count = %d, want 1", result.Version.ItemCount)
	}
	if len(result.EvalSpecs) != 1 || result.EvalSpecs[0].Name != "Default Demo Eval" {
		t.Fatalf("eval specs = %#v, want imported preview eval spec", result.EvalSpecs)
	}
}

func TestHandler_PreviewDatasetBundleFromGitHubSourceUsesVersionParam(t *testing.T) {
	datasetYAML, manifestJSON, evalSpecYAML := testDatasetBundleFixtureContents()
	oldClient := datasetBundleLoaderHTTPClient
	datasetBundleLoaderHTTPClient = &http.Client{
		Transport: datasetBundleRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Host {
			case "raw.githubusercontent.com":
				body := ""
				switch {
				case strings.Contains(req.URL.String(), "/demo-bundle/dataset.yaml"):
					body = datasetYAML
				case strings.Contains(req.URL.String(), "/demo-bundle/versions/v2/manifest.json"):
					body = strings.Replace(manifestJSON, `"version": "v1"`, `"version": "v2"`, 1)
				case strings.Contains(req.URL.String(), "/demo-bundle/versions/v1/manifest.json"):
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Body:       io.NopCloser(strings.NewReader("not found")),
						Header:     make(http.Header),
					}, nil
				case strings.Contains(req.URL.String(), "/demo-bundle/versions/v2/eval-specs/default.yaml"):
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
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			}
		}),
	}
	t.Cleanup(func() { datasetBundleLoaderHTTPClient = oldClient })

	controller := newTestController(t)
	handler := NewHandler(controller)
	e := echo.New()

	body, err := json.Marshal(ImportDatasetBundleFromSourceRequest{
		SourceType: "github",
		Source:     "https://github.com/example/harness-datasets/tree/main/demo-bundle",
		Version:    "v2",
	})
	if err != nil {
		t.Fatalf("json.Marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/dataset-bundles/preview-source", strings.NewReader(string(body)))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.PreviewDatasetBundleFromSource(c); err != nil {
		t.Fatalf("PreviewDatasetBundleFromSource returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var result DatasetBundleSourcePreview
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Version.Version != "v2" {
		t.Fatalf("version = %q, want v2", result.Version.Version)
	}
}
