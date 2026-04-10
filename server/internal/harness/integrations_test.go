package harness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildJudgeUserPromptIncludesItemMetadataAndWorkspaceSummary(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "report.md"), []byte("# Report\n\nPinned findings.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile report.md: %v", err)
	}

	prompt := buildJudgeUserPrompt(JudgeEvaluationRequest{
		Model: "claude-sonnet-4-6",
		Group: &RunGroup{
			Subject: "pinchbench",
		},
		Item: &RunGroupItem{
			Profile: "pinchbench",
			Input: map[string]interface{}{
				"goal": "Create the report",
			},
			Expected: map[string]interface{}{
				"status": "completed",
			},
			Metadata: map[string]interface{}{
				"success_rubric":              "Report should cover the required findings.",
				"pinchbench_expected_behavior": "Write a concise report to report.md.",
			},
		},
		Run: &Run{
			Status:        RunStatusCompleted,
			Result:        "Done.",
			WorkspaceRoot: workspace,
		},
	})

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(prompt), &payload); err != nil {
		t.Fatalf("json.Unmarshal prompt: %v", err)
	}

	itemMeta, ok := payload["item_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("item_metadata = %#v, want object", payload["item_metadata"])
	}
	if got := itemMeta["success_rubric"]; got != "Report should cover the required findings." {
		t.Fatalf("item_metadata.success_rubric = %#v, want preserved rubric", got)
	}
	if got := itemMeta["pinchbench_expected_behavior"]; got != "Write a concise report to report.md." {
		t.Fatalf("item_metadata.pinchbench_expected_behavior = %#v, want preserved expected behavior", got)
	}

	workspaceSummary, ok := payload["workspace_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("workspace_summary = %#v, want object", payload["workspace_summary"])
	}
	files, ok := workspaceSummary["files"].([]interface{})
	if !ok || len(files) == 0 {
		t.Fatalf("workspace_summary.files = %#v, want at least one summarized file", workspaceSummary["files"])
	}
	firstFile, ok := files[0].(map[string]interface{})
	if !ok {
		t.Fatalf("workspace_summary.files[0] = %#v, want object", files[0])
	}
	if got := firstFile["path"]; got != "report.md" {
		t.Fatalf("workspace_summary.files[0].path = %#v, want report.md", got)
	}
	if got := firstFile["snippet"]; got == nil || got == "" {
		t.Fatalf("workspace_summary.files[0].snippet = %#v, want non-empty snippet", got)
	}
}
