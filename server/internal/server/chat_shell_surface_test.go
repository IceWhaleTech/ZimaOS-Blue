package server

import (
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestCollapseDefaultChatShellCompatDefs_PreservesExecWhenBashAbsent(t *testing.T) {
	defs := []tools.ToolDefinition{{Name: "exec"}}

	got := collapseDefaultChatShellCompatDefs(defs)

	if len(got) != 1 || got[0].Name != "exec" {
		t.Fatalf("collapseDefaultChatShellCompatDefs(%v) = %v, want lone exec preserved", defs, got)
	}
}

func TestCollapseDefaultChatShellCompatDefs_DropsExecWhenConcreteBashExists(t *testing.T) {
	defs := []tools.ToolDefinition{{Name: "bash"}, {Name: "exec"}}

	got := collapseDefaultChatShellCompatDefs(defs)

	if len(got) != 1 || got[0].Name != "bash" {
		t.Fatalf("collapseDefaultChatShellCompatDefs(%v) = %v, want only bash", defs, got)
	}
}
