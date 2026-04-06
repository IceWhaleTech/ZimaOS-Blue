package harness

import (
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestDatasetBundleHandlerRoutesIncludeSourceEndpoints(t *testing.T) {
	controller := newTestController(t)
	handler := NewHandler(controller)
	e := echo.New()
	group := e.Group("/harness")

	handler.RegisterRoutes(group)

	if !datasetBundleHandlerRouteExists(e, http.MethodPost, "/harness/dataset-bundles/preview-source") {
		t.Fatalf("expected preview-source route to be registered, got %#v", e.Routes())
	}
	if !datasetBundleHandlerRouteExists(e, http.MethodPost, "/harness/dataset-bundles/import-source") {
		t.Fatalf("expected import-source route to be registered, got %#v", e.Routes())
	}
}

func TestPreviewDatasetBundleImportRequestRejectsMissingManifest(t *testing.T) {
	req := testDatasetBundleImportRequest(t, "bundle-dataset", "v1", "bundle-profile", false)
	req.Version.Manifest = nil

	_, err := previewDatasetBundleImportRequest(&req)
	if err == nil {
		t.Fatal("expected missing manifest validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "manifest") {
		t.Fatalf("error = %v, want manifest validation guidance", err)
	}
}

func TestPreviewDatasetBundleImportRequestRejectsEvalSpecWithoutRunKindFallback(t *testing.T) {
	req := testDatasetBundleImportRequest(t, "bundle-dataset", "v1", "bundle-profile", false)
	req.Dataset.DefaultRunKind = ""
	req.EvalSpecs[0].RunKind = ""

	_, err := previewDatasetBundleImportRequest(&req)
	if err == nil {
		t.Fatal("expected run kind validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "run kind") {
		t.Fatalf("error = %v, want run kind validation guidance", err)
	}
}

func datasetBundleHandlerRouteExists(e *echo.Echo, method string, path string) bool {
	for _, route := range e.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
