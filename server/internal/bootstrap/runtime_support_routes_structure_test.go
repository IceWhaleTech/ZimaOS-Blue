package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeSupportRoutesGo_DelegatesBrowserExecAndQuestionLanes(t *testing.T) {
	browserContent, err := os.ReadFile(filepath.Join("runtime_support_routes_browser.go"))
	if err != nil {
		t.Fatalf("read runtime_support_routes_browser.go: %v", err)
	}
	browserSource := string(browserContent)

	execContent, err := os.ReadFile(filepath.Join("runtime_support_routes_exec.go"))
	if err != nil {
		t.Fatalf("read runtime_support_routes_exec.go: %v", err)
	}
	execSource := string(execContent)

	questionContent, err := os.ReadFile(filepath.Join("runtime_support_routes_question.go"))
	if err != nil {
		t.Fatalf("read runtime_support_routes_question.go: %v", err)
	}
	questionSource := string(questionContent)

	if lines := strings.Count(browserSource, "\n") + 1; lines > 95 {
		t.Fatalf("expected runtime_support_routes_browser.go to stay below 95 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(execSource, "\n") + 1; lines > 130 {
		t.Fatalf("expected runtime_support_routes_exec.go to stay below 130 lines after extraction, got %d", lines)
	}
	if lines := strings.Count(questionSource, "\n") + 1; lines > 60 {
		t.Fatalf("expected runtime_support_routes_question.go to stay below 60 lines after extraction, got %d", lines)
	}

	requiredBrowser := []string{
		"func registerBrowserApprovalRoutes(",
	}
	for _, token := range requiredBrowser {
		if !strings.Contains(browserSource, token) {
			t.Fatalf("expected runtime_support_routes_browser.go to contain token %q", token)
		}
	}

	requiredExec := []string{
		"func registerExecApprovalRoutes(",
		"func registerExecDirectoryApprovalRoutes(",
	}
	for _, token := range requiredExec {
		if !strings.Contains(execSource, token) {
			t.Fatalf("expected runtime_support_routes_exec.go to contain token %q", token)
		}
	}

	requiredQuestion := []string{
		"func registerAskUserQuestionRoutes(",
	}
	for _, token := range requiredQuestion {
		if !strings.Contains(questionSource, token) {
			t.Fatalf("expected runtime_support_routes_question.go to contain token %q", token)
		}
	}

	forbiddenBrowser := []string{
		"func registerExecApprovalRoutes(",
		"func registerAskUserQuestionRoutes(",
	}
	for _, token := range forbiddenBrowser {
		if strings.Contains(browserSource, token) {
			t.Fatalf("expected runtime_support_routes_browser.go to delegate token %q", token)
		}
	}

	forbiddenExec := []string{
		"func registerBrowserApprovalRoutes(",
		"func registerAskUserQuestionRoutes(",
	}
	for _, token := range forbiddenExec {
		if strings.Contains(execSource, token) {
			t.Fatalf("expected runtime_support_routes_exec.go to delegate token %q", token)
		}
	}

	forbiddenQuestion := []string{
		"func registerBrowserApprovalRoutes(",
		"func registerExecApprovalRoutes(",
	}
	for _, token := range forbiddenQuestion {
		if strings.Contains(questionSource, token) {
			t.Fatalf("expected runtime_support_routes_question.go to delegate token %q", token)
		}
	}
}
