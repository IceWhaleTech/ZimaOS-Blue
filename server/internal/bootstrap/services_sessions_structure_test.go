package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServicesSessions_AreSplitByRole(t *testing.T) {
	files := map[string]struct {
		maxLines int
		tokens   []string
	}{
		"services_sessions.go": {
			maxLines: 55,
			tokens: []string{
				"type sessionListAdapter struct",
				"func sessionScopedUserID(",
				"func buildSessionSummary(",
				"func buildSessionMessage(",
			},
		},
		"services_sessions_read.go": {
			maxLines: 65,
			tokens: []string{
				"func (a sessionListAdapter) ListSessions(",
				"func (a sessionListAdapter) GetSession(",
				"func (a sessionListAdapter) GetSessionMessages(",
			},
		},
		"services_sessions_write.go": {
			maxLines: 65,
			tokens: []string{
				"func (a sessionListAdapter) CreateSession(",
				"func (a sessionListAdapter) AppendSessionMessage(",
			},
		},
		"services_sessions_runtime.go": {
			maxLines: 35,
			tokens: []string{
				"func (a sessionListAdapter) HandleRuntimeSessionAction(",
				"return a.handleRuntimeSessionListAction(ctx, args)",
				"return a.handleRuntimeSessionSpawnAction(ctx, args)",
			},
		},
		"services_sessions_runtime_args.go": {
			maxLines: 65,
			tokens: []string{
				"func runtimeCompatString(",
				"func runtimeCompatInt(",
				"func runtimeCompatTitle(",
				"func runtimeSessionUserID(",
			},
		},
		"services_sessions_runtime_read.go": {
			maxLines: 60,
			tokens: []string{
				"func runtimeSessionProtocol(",
				"func (a sessionListAdapter) handleRuntimeSessionListAction(",
				"func (a sessionListAdapter) handleRuntimeSessionHistoryAction(",
				"func (a sessionListAdapter) handleRuntimeSessionStatusAction(",
			},
		},
		"services_sessions_runtime_write.go": {
			maxLines: 80,
			tokens: []string{
				"func runtimeSessionProfileID(",
				"func runtimeSessionTitle(",
				"func (a sessionListAdapter) handleRuntimeSessionSpawnAction(",
				"func (a sessionListAdapter) handleRuntimeSessionSendAction(",
			},
		},
	}

	for name, expectation := range files {
		content, err := os.ReadFile(filepath.Join(name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		source := string(content)
		if lines := strings.Count(source, "\n") + 1; lines > expectation.maxLines {
			t.Fatalf("expected %s to stay below %d lines, got %d", name, expectation.maxLines, lines)
		}
		for _, token := range expectation.tokens {
			if !strings.Contains(source, token) {
				t.Fatalf("expected %s to contain token %q", name, token)
			}
		}
	}
}

func TestServicesSessions_MainFilesStayThin(t *testing.T) {
	sessionContent, err := os.ReadFile(filepath.Join("services_sessions.go"))
	if err != nil {
		t.Fatalf("read services_sessions.go: %v", err)
	}
	sessionSource := string(sessionContent)
	for _, token := range []string{
		"func (a sessionListAdapter) ListSessions(",
		"func (a sessionListAdapter) CreateSession(",
		"func (a sessionListAdapter) HandleRuntimeSessionAction(",
	} {
		if strings.Contains(sessionSource, token) {
			t.Fatalf("expected services_sessions.go to delegate token %q", token)
		}
	}

	runtimeContent, err := os.ReadFile(filepath.Join("services_sessions_runtime.go"))
	if err != nil {
		t.Fatalf("read services_sessions_runtime.go: %v", err)
	}
	runtimeSource := string(runtimeContent)
	for _, token := range []string{
		"func runtimeCompatString(",
		"func runtimeSessionProfileID(",
		"func (a sessionListAdapter) handleRuntimeSessionListAction(",
	} {
		if strings.Contains(runtimeSource, token) {
			t.Fatalf("expected services_sessions_runtime.go to delegate token %q", token)
		}
	}
}
