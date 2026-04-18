package agentcore

import (
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestWriteToolsInfoTo_IncludesDesktopChatCompletionGuidance(t *testing.T) {
	registry := tools.NewRegistry()
	registry.Register(tools.NewMockTool("computer_use", "Native computer-use tool"))

	b := NewSystemPromptBuilder(&Config{})
	b.SetToolRegistry(registry)

	var sb strings.Builder
	if !b.writeToolsInfoTo(&sb, false) {
		t.Fatal("expected tool guidance to be written")
	}
	out := sb.String()

	required := []string{
		"desktop chat or messaging tasks",
		"`message`, `select`, and `type`",
		"Do not invent unsupported computer_use action names",
		"`activate`, `mouse_click`, `move_to`, `accessibility_tree`, or `ocr`",
		"`screenshot` only for evidence capture or last-resort debugging",
	}
	for _, want := range required {
		if !strings.Contains(out, want) {
			t.Fatalf("expected computer_use guidance to include %q, got: %s", want, out)
		}
	}
}
