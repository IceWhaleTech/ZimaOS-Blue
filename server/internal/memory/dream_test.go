package memory

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sessionctx "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
)

func newDreamServiceForTest(t *testing.T, cfg DreamConfig) (*DreamService, *LayeredMemoryService) {
	t.Helper()

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	memoryDir := filepath.Join(workspaceDir, "memory")

	mdBackend, err := NewPureMarkdownBackend(memoryDir)
	if err != nil {
		t.Fatalf("NewPureMarkdownBackend: %v", err)
	}
	unified := NewUnifiedMemoryService(mdBackend)
	layered, err := NewLayeredMemoryService(unified, LayeredMemoryConfig{
		BaseDir:            memoryDir,
		LongTermDir:        workspaceDir,
		DailyRetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewLayeredMemoryService: %v", err)
	}

	cfg.ArchiveDir = filepath.Join(t.TempDir(), "archives")
	dream, err := NewDreamService(layered, workspaceDir, cfg)
	if err != nil {
		t.Fatalf("NewDreamService: %v", err)
	}
	return dream, layered
}

func newDreamSession(messageCount int) *session.Session {
	sess := session.NewSession(session.SessionID{
		AgentID:   "chat",
		ChannelID: "web",
		PeerID:    "conv-123",
	}, 4096)
	sess.SetTitle("Dream Memory Rollout")
	sess.SetSummary("Important decision: archive historical memories into cold storage outside the primary memory.")
	sess.AddTag("memory")
	sess.AddTag("decision")

	messages := []sessionctx.Message{
		{Role: sessionctx.RoleUser, Content: "Please remember that historical memories should be compressed into cold archive storage."},
		{Role: sessionctx.RoleAssistant, Content: "Decision confirmed: dream capsules will archive old memories outside the primary memory."},
		{Role: sessionctx.RoleUser, Content: "Important action item: build a nightly dream pass to promote key facts."},
		{Role: sessionctx.RoleAssistant, Content: "Agreed. The nightly dream job will summarize and promote important memory candidates."},
	}
	for i := 0; i < messageCount && i < len(messages); i++ {
		sess.AddMessage(messages[i])
	}
	return sess
}

func readDreamCapsule(t *testing.T, path string) DreamSessionCapsule {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open capsule: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()

	var capsule DreamSessionCapsule
	if err := json.NewDecoder(gr).Decode(&capsule); err != nil {
		t.Fatalf("decode capsule: %v", err)
	}
	return capsule
}

func TestDreamServiceWorkspaceNamespaceIsDeterministic(t *testing.T) {
	cfg := DefaultDreamConfig()
	cfg.ArchiveDir = filepath.Join(t.TempDir(), "archives")

	workspaceDir := filepath.Join(t.TempDir(), "workspace", "alpha")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	first, err := NewDreamService(nil, workspaceDir, cfg)
	if err != nil {
		t.Fatalf("NewDreamService(first): %v", err)
	}
	second, err := NewDreamService(nil, workspaceDir, cfg)
	if err != nil {
		t.Fatalf("NewDreamService(second): %v", err)
	}

	if first.WorkspaceKey() == "" {
		t.Fatal("WorkspaceKey() should not be empty")
	}
	if first.WorkspaceKey() != second.WorkspaceKey() {
		t.Fatalf("WorkspaceKey mismatch: %q vs %q", first.WorkspaceKey(), second.WorkspaceKey())
	}
	if first.WorkspaceArchiveDir() != second.WorkspaceArchiveDir() {
		t.Fatalf("WorkspaceArchiveDir mismatch: %q vs %q", first.WorkspaceArchiveDir(), second.WorkspaceArchiveDir())
	}
	if !strings.Contains(first.WorkspaceArchiveDir(), first.WorkspaceKey()) {
		t.Fatalf("WorkspaceArchiveDir %q should contain workspace key %q", first.WorkspaceArchiveDir(), first.WorkspaceKey())
	}
}

func TestDreamServiceArchiveSessionWritesCompressedCapsule(t *testing.T) {
	cfg := DefaultDreamConfig()
	cfg.SessionMinMessages = 2

	dream, _ := newDreamServiceForTest(t, cfg)

	artifactID, err := dream.ArchiveSession(context.Background(), newDreamSession(4), session.EndReasonArchive)
	if err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}
	if artifactID == "" {
		t.Fatal("ArchiveSession() returned empty artifact ID")
	}

	matches, err := filepath.Glob(filepath.Join(dream.WorkspaceArchiveDir(), "sessions", "*", "*", "*", "*.json.gz"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("capsule count = %d, want 1", len(matches))
	}

	capsule := readDreamCapsule(t, matches[0])
	if capsule.ArtifactID != artifactID {
		t.Fatalf("capsule.ArtifactID = %q, want %q", capsule.ArtifactID, artifactID)
	}
	if capsule.Reason != string(session.EndReasonArchive) {
		t.Fatalf("capsule.Reason = %q, want %q", capsule.Reason, session.EndReasonArchive)
	}
	if capsule.SessionID == "" || capsule.WorkspaceKey != dream.WorkspaceKey() {
		t.Fatalf("capsule session/workspace mismatch: %+v", capsule)
	}
	if len(capsule.Candidates) == 0 {
		t.Fatal("capsule.Candidates should not be empty")
	}

	status, err := dream.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.PendingCapsules != 1 {
		t.Fatalf("PendingCapsules = %d, want 1", status.PendingCapsules)
	}
}

func TestDreamSessionHookArchivesSupportedReasonsAndSkipsDeleteAndSmallSessions(t *testing.T) {
	reasons := []struct {
		name      string
		reason    session.EndReason
		msgCount  int
		wantFiles int
	}{
		{name: "archive", reason: session.EndReasonArchive, msgCount: 4, wantFiles: 1},
		{name: "reset", reason: session.EndReasonReset, msgCount: 4, wantFiles: 1},
		{name: "new", reason: session.EndReasonNew, msgCount: 4, wantFiles: 1},
		{name: "timeout", reason: session.EndReasonTimeout, msgCount: 4, wantFiles: 1},
		{name: "delete", reason: session.EndReasonDelete, msgCount: 4, wantFiles: 0},
		{name: "too-small", reason: session.EndReasonArchive, msgCount: 1, wantFiles: 0},
	}

	for _, tc := range reasons {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultDreamConfig()
			cfg.SessionMinMessages = 2
			dream, _ := newDreamServiceForTest(t, cfg)
			hook := NewDreamSessionHook(dream)

			if err := hook.OnSessionEnd(context.Background(), newDreamSession(tc.msgCount), tc.reason); err != nil {
				t.Fatalf("OnSessionEnd: %v", err)
			}

			matches, err := filepath.Glob(filepath.Join(dream.WorkspaceArchiveDir(), "sessions", "*", "*", "*", "*.json.gz"))
			if err != nil {
				t.Fatalf("Glob: %v", err)
			}
			if len(matches) != tc.wantFiles {
				t.Fatalf("capsule count = %d, want %d", len(matches), tc.wantFiles)
			}
		})
	}
}

func TestDreamServiceArchiveSessionFallsBackWithoutSummary(t *testing.T) {
	cfg := DefaultDreamConfig()
	cfg.SessionMinMessages = 2

	dream, _ := newDreamServiceForTest(t, cfg)
	sess := newDreamSession(4)
	sess.SetSummary("")

	artifactID, err := dream.ArchiveSession(context.Background(), sess, session.EndReasonArchive)
	if err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}
	if artifactID == "" {
		t.Fatal("ArchiveSession() returned empty artifact ID")
	}

	matches, err := filepath.Glob(filepath.Join(dream.WorkspaceArchiveDir(), "sessions", "*", "*", "*", "*.json.gz"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("capsule count = %d, want 1", len(matches))
	}

	capsule := readDreamCapsule(t, matches[0])
	if len(capsule.Candidates) == 0 {
		t.Fatal("capsule.Candidates should contain heuristic fallback candidates")
	}
	lower := strings.ToLower(capsule.Candidates[0].Content)
	if !strings.Contains(lower, "remember") && !strings.Contains(lower, "archive") {
		t.Fatalf("unexpected fallback candidate: %+v", capsule.Candidates[0])
	}
}

func TestDreamServiceRunConsolidationPromotesUniqueMemoryAndArchivesDailyLogs(t *testing.T) {
	cfg := DefaultDreamConfig()
	cfg.SessionMinMessages = 2
	cfg.PromoteDailyAfterDays = 2
	cfg.ArchiveDailyAfterDays = 2
	cfg.MaxPromotionsPerRun = 5

	dream, layered := newDreamServiceForTest(t, cfg)
	ctx := context.Background()

	oldDate := time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02")
	oldLogPath := filepath.Join(layered.dailyLogPath, oldDate+".md")
	oldLogContent := `# Daily Log - ` + oldDate + `

## 09:30:00

**Tags:** important, decision

Important decision: archive historical memories into cold storage outside the primary memory.

---
`
	if err := os.WriteFile(oldLogPath, []byte(oldLogContent), 0o644); err != nil {
		t.Fatalf("write old daily log: %v", err)
	}

	if _, err := dream.ArchiveSession(ctx, newDreamSession(4), session.EndReasonArchive); err != nil {
		t.Fatalf("ArchiveSession: %v", err)
	}

	run, err := dream.RunConsolidation(ctx)
	if err != nil {
		t.Fatalf("RunConsolidation: %v", err)
	}
	if run.RunID == "" {
		t.Fatal("RunConsolidation RunID should not be empty")
	}
	if run.ProcessedCapsules != 1 {
		t.Fatalf("ProcessedCapsules = %d, want 1", run.ProcessedCapsules)
	}
	if run.ArchivedDailyCount != 1 {
		t.Fatalf("ArchivedDailyCount = %d, want 1", run.ArchivedDailyCount)
	}
	if run.PromotedCount == 0 {
		t.Fatal("PromotedCount should be > 0")
	}

	longTerm, err := layered.GetLongTermMemory(ctx)
	if err != nil {
		t.Fatalf("GetLongTermMemory: %v", err)
	}
	if count := strings.Count(longTerm, "Important decision: archive historical memories into cold storage outside the primary memory."); count != 1 {
		t.Fatalf("long-term memory count = %d, want 1, content=%s", count, longTerm)
	}

	if _, err := os.Stat(oldLogPath); !os.IsNotExist(err) {
		t.Fatalf("old log should be removed after archive, stat err=%v", err)
	}

	dailyArchives, err := filepath.Glob(filepath.Join(dream.WorkspaceArchiveDir(), "daily", "*", "*", "*.md.gz"))
	if err != nil {
		t.Fatalf("Glob daily archives: %v", err)
	}
	if len(dailyArchives) != 1 {
		t.Fatalf("daily archive count = %d, want 1", len(dailyArchives))
	}

	secondRun, err := dream.RunConsolidation(ctx)
	if err != nil {
		t.Fatalf("RunConsolidation second run: %v", err)
	}
	if secondRun.ProcessedCapsules != 0 || secondRun.PromotedCount != 0 || secondRun.ArchivedDailyCount != 0 {
		t.Fatalf("second run = %+v, want zero work", secondRun)
	}

	status, err := dream.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.PendingCapsules != 0 {
		t.Fatalf("PendingCapsules = %d, want 0", status.PendingCapsules)
	}
	if status.ArchivedDailyLogs != 1 {
		t.Fatalf("ArchivedDailyLogs = %d, want 1", status.ArchivedDailyLogs)
	}
	if status.PromotedCount != 1 {
		t.Fatalf("PromotedCount = %d, want 1", status.PromotedCount)
	}
}

func TestDreamServiceArchiveFailureLeavesDailyLogUntouched(t *testing.T) {
	cfg := DefaultDreamConfig()
	cfg.PromoteDailyAfterDays = 1
	cfg.ArchiveDailyAfterDays = 1

	dream, layered := newDreamServiceForTest(t, cfg)
	ctx := context.Background()

	oldDate := time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
	oldLogPath := filepath.Join(layered.dailyLogPath, oldDate+".md")
	if err := os.WriteFile(oldLogPath, []byte("# Daily Log\n\n## 08:00:00\n\nimportant task\n"), 0o644); err != nil {
		t.Fatalf("write old log: %v", err)
	}

	blockingPath := filepath.Join(dream.WorkspaceArchiveDir(), "daily", time.Now().UTC().AddDate(0, 0, -2).Format("2006"))
	if err := os.MkdirAll(filepath.Dir(blockingPath), 0o755); err != nil {
		t.Fatalf("mkdir blocking parent: %v", err)
	}
	if err := os.WriteFile(blockingPath, []byte("not-a-directory"), 0o644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	if _, err := dream.RunConsolidation(ctx); err == nil {
		t.Fatal("RunConsolidation should fail when archive destination is blocked")
	}
	if _, err := os.Stat(oldLogPath); err != nil {
		t.Fatalf("old log should remain after archive failure, stat err=%v", err)
	}
}
