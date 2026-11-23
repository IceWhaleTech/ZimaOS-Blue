package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestResearchWorkflowHonorsSourceScope(t *testing.T) {
	cases := []struct {
		name, query       string
		research, minimum bool
	}{
		{"local_json", `{"corpus":"/tmp/data","task":{"question":"RELEASE2041 发布前需要完成什么？"},"answerSchema":{"type":"object","properties":{"citations":{"type":"array"}},"enum":["insufficient_evidence"]},"instruction":"根据 corpus 内文件回答，输出 JSON 并引用证据。"}`, false, false},
		{"local_text", "Read release-notes.txt in the local directory and answer with citations from that file.", false, false},
		{"local_latest", "Read the latest release notes in the workspace and summarize with citations.", false, false},
		{"local_research", "Do deep research on the provided local files in the directory and return the findings with sources.", false, false},
		{"public_json", `{"query":"Deep research on Blue latest releases with citations","answerSchema":{"type":"object","properties":{"answer":{"type":"string"}}}}`, true, true},
		{"lightweight_schema", `{"query":"Blue news","answerSchema":{"type":"object","properties":{"citations":{"type":"array"}}}}`, true, false},
		{"mixed", "Read local files in the workspace, then search the web for the latest release notes and give a complete report with citations.", true, true},
		{"public_project", "Deep research on the Blue project latest releases with sources.", true, true},
		{"web_forbidden", "不要联网；读取目录中的发布说明并附引用。", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldPreferDeepSearchReport(tc.query); got != tc.research {
				t.Errorf("research = %v, want %v", got, tc.research)
			}
			if got := shouldEnforceDeepSearchMinRounds(tc.query); got != tc.minimum {
				t.Errorf("minimum = %v, want %v", got, tc.minimum)
			}
			state := newDeepSearchLoopState(tc.query, []tools.ToolDefinition{{Name: "bash"}})
			if force, _ := state.shouldForceAnotherSearch(2, 10); force != tc.minimum {
				t.Errorf("forced continuation = %v, want %v", force, tc.minimum)
			}
		})
	}
}

func TestDeepSearchContinuationPreservesRequestedOutput(t *testing.T) {
	nudge := buildDeepSearchMinRoundsNudge(nil)
	if !strings.Contains(nudge, "requested output format") {
		t.Fatalf("research continuation must preserve the user's output format: %s", nudge)
	}
}

func TestShellEvidenceSurvivesCompaction(t *testing.T) {
	stdout := strings.Repeat("source context\n", 80) + "Hardware\t2100.20\nService\t1482.50"
	raw, err := json.Marshal(map[string]interface{}{"status": "completed", "exit_code": 0, "stdout": stdout})
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"bash", "exec"} {
		t.Run(tool, func(t *testing.T) {
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(compactToolResultContentForLLM(tool, string(raw))), &payload); err != nil {
				t.Fatal(err)
			}
			if payload["stdout"] != stdout {
				t.Fatalf("source evidence lost from %s stdout: %v", tool, payload["stdout"])
			}
		})
	}
}
