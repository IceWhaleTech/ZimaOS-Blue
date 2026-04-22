package tools

import (
	"strings"
	"testing"
)

func TestBuildToolApprovalPresentation_FileWrite(t *testing.T) {
	req := ToolApprovalRequest{
		ToolName:  "file_write",
		RiskLevel: "high",
		Arguments: map[string]interface{}{
			"path":    "/tmp/demo.txt",
			"content": "hello world",
		},
	}

	presentation := buildToolApprovalPresentation(req, "en-US")
	if !strings.Contains(strings.ToLower(presentation.Purpose), "write") {
		t.Fatalf("Purpose = %q, want write-focused summary", presentation.Purpose)
	}
	if !strings.Contains(presentation.ScopeSummary, "/tmp/demo.txt") {
		t.Fatalf("ScopeSummary = %q, want target path", presentation.ScopeSummary)
	}
	if !strings.Contains(strings.ToLower(presentation.ExpectedEffects), "modify") &&
		!strings.Contains(strings.ToLower(presentation.ExpectedEffects), "create") {
		t.Fatalf("ExpectedEffects = %q, want file change summary", presentation.ExpectedEffects)
	}
	if len(presentation.AffectedTargets) != 1 || presentation.AffectedTargets[0] != "/tmp/demo.txt" {
		t.Fatalf("AffectedTargets = %#v, want [/tmp/demo.txt]", presentation.AffectedTargets)
	}
	if strings.TrimSpace(presentation.RiskSummary) == "" {
		t.Fatal("RiskSummary should not be empty")
	}
}

func TestBuildExecApprovalPresentation_Command(t *testing.T) {
	req := ApprovalRequest{
		Type:            "command",
		Command:         "python scripts/sync.py --write",
		Workdir:         "/Users/orca/workspace/demo",
		ReferencedPaths: []string{"/Users/orca/workspace/demo/config.yaml", "/tmp/report.md"},
		RiskLevel:       "high",
		Security:        "filesystem-write,network",
	}

	presentation := buildExecApprovalPresentation(req, "zh-CN")
	if !strings.Contains(presentation.Purpose, "命令") {
		t.Fatalf("Purpose = %q, want localized command summary", presentation.Purpose)
	}
	if !strings.Contains(presentation.ScopeSummary, "/Users/orca/workspace/demo") {
		t.Fatalf("ScopeSummary = %q, want workdir", presentation.ScopeSummary)
	}
	if !strings.Contains(presentation.ExpectedEffects, "写") &&
		!strings.Contains(strings.ToLower(presentation.ExpectedEffects), "write") {
		t.Fatalf("ExpectedEffects = %q, want write/network summary", presentation.ExpectedEffects)
	}
	if len(presentation.AffectedTargets) < 2 {
		t.Fatalf("AffectedTargets = %#v, want referenced targets", presentation.AffectedTargets)
	}
	if strings.TrimSpace(presentation.RiskSummary) == "" {
		t.Fatal("RiskSummary should not be empty")
	}
}
