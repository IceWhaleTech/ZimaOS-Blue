package optimization

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNormalizeGitHubRepoInputAcceptsOwnerRepoAndHTTPSURLs(t *testing.T) {
	for _, input := range []string{
		"IceWhaleTech/ZimaOS-Blue",
		"https://github.com/IceWhaleTech/ZimaOS-Blue",
		"https://github.com/IceWhaleTech/ZimaOS-Blue.git",
	} {
		repo, err := NormalizeGitHubRepo(input)
		if err != nil {
			t.Fatalf("NormalizeGitHubRepo(%q) error = %v", input, err)
		}
		if repo.Owner != "IceWhaleTech" || repo.Name != "ZimaOS-Blue" {
			t.Fatalf("NormalizeGitHubRepo(%q) = %#v", input, repo)
		}
	}
}

func TestNormalizeGitHubRepoRejectsNonPublicSources(t *testing.T) {
	for _, input := range []string{
		"git@github.com:IceWhaleTech/ZimaOS-Blue.git",
		"https://example.com/IceWhaleTech/ZimaOS-Blue",
		"https://github.com/IceWhaleTech",
	} {
		if _, err := NormalizeGitHubRepo(input); err == nil {
			t.Fatalf("NormalizeGitHubRepo(%q) succeeded, want error", input)
		}
	}
}

func TestDetectRequiredGoVersionFallsBackToBaseline(t *testing.T) {
	if got := DetectRequiredGoVersion([]byte("module example.com/test\n")); got != DefaultGoVersion {
		t.Fatalf("DetectRequiredGoVersion fallback = %q, want %q", got, DefaultGoVersion)
	}
	if got := DetectRequiredGoVersion([]byte("module example.com/test\n\ngo 1.25.3\n")); got != "1.25.3" {
		t.Fatalf("DetectRequiredGoVersion parsed = %q, want 1.25.3", got)
	}
}

func TestGoToolchainArtifactNamingMatchesPlatform(t *testing.T) {
	archive, err := GoToolchainArchive("1.24.0", Platform{GOOS: "windows", GOARCH: "amd64"})
	if err != nil {
		t.Fatalf("GoToolchainArchive windows error = %v", err)
	}
	if archive.FileName != "go1.24.0.windows-amd64.zip" {
		t.Fatalf("windows filename = %q", archive.FileName)
	}

	archive, err = GoToolchainArchive("1.24.0", Platform{GOOS: "darwin", GOARCH: "arm64"})
	if err != nil {
		t.Fatalf("GoToolchainArchive darwin error = %v", err)
	}
	if archive.FileName != "go1.24.0.darwin-arm64.tar.gz" {
		t.Fatalf("darwin filename = %q", archive.FileName)
	}
}

func TestManagedLayoutUsesExpectedCacheSubdirectories(t *testing.T) {
	layout := NewManagedLayout(filepath.Join(t.TempDir(), "agentcore-runner"))
	if !strings.HasSuffix(layout.ReposDir, filepath.Join("agentcore-runner", "repos")) {
		t.Fatalf("repos dir = %q", layout.ReposDir)
	}
	if !strings.HasSuffix(layout.GoToolchainsDir, filepath.Join("agentcore-runner", "toolchains", "go")) {
		t.Fatalf("go toolchain dir = %q", layout.GoToolchainsDir)
	}
	if !strings.HasSuffix(layout.BinDir, filepath.Join("agentcore-runner", "bin")) {
		t.Fatalf("bin dir = %q", layout.BinDir)
	}
}

func TestRunnerBinaryNameFollowsHostPlatform(t *testing.T) {
	got := RunnerBinaryName(runtime.GOOS)
	if runtime.GOOS == "windows" {
		if got != "agentcore-runner.exe" {
			t.Fatalf("RunnerBinaryName(windows) = %q", got)
		}
		return
	}
	if got != "agentcore-runner" {
		t.Fatalf("RunnerBinaryName(%s) = %q", runtime.GOOS, got)
	}
}

func TestResolveRunnerWorkspaceRootSupportsNestedServerModule(t *testing.T) {
	repoDir := t.TempDir()
	workspaceDir := filepath.Join(repoDir, "server")
	if err := os.MkdirAll(filepath.Join(workspaceDir, "cmd", "agentcore-runner"), 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "go.mod"), []byte("module example.com/runner\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	got, err := resolveRunnerWorkspaceRoot(repoDir)
	if err != nil {
		t.Fatalf("resolveRunnerWorkspaceRoot error = %v", err)
	}
	if got != workspaceDir {
		t.Fatalf("resolveRunnerWorkspaceRoot = %q, want %q", got, workspaceDir)
	}
}

func TestResolveRunnerWorkspaceRootKeepsRepoRootWhenRunnerLivesThere(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "cmd", "agentcore-runner"), 0o755); err != nil {
		t.Fatalf("mkdir runner cmd: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/runner\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	got, err := resolveRunnerWorkspaceRoot(repoDir)
	if err != nil {
		t.Fatalf("resolveRunnerWorkspaceRoot error = %v", err)
	}
	if got != repoDir {
		t.Fatalf("resolveRunnerWorkspaceRoot = %q, want %q", got, repoDir)
	}
}

func TestCompactRunnerWorkspaceInPlaceFlattensNestedServerWorkspace(t *testing.T) {
	repoDir := t.TempDir()
	workspaceDir := filepath.Join(repoDir, "server")
	if err := os.MkdirAll(filepath.Join(workspaceDir, "cmd", "agentcore-runner"), 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "web"), 0o755); err != nil {
		t.Fatalf("mkdir web: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "go.mod"), []byte("module example.com/runner\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "web", "index.html"), []byte("unused"), 0o644); err != nil {
		t.Fatalf("write web file: %v", err)
	}

	if err := compactRunnerWorkspaceInPlace(repoDir); err != nil {
		t.Fatalf("compactRunnerWorkspaceInPlace error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "go.mod")); err != nil {
		t.Fatalf("root go.mod missing after compaction: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "cmd", "agentcore-runner")); err != nil {
		t.Fatalf("root cmd/agentcore-runner missing after compaction: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "web")); !os.IsNotExist(err) {
		t.Fatalf("web directory exists after compaction, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "server")); !os.IsNotExist(err) {
		t.Fatalf("nested server directory exists after compaction, err = %v", err)
	}
}

func TestCompactRunnerWorkspaceInPlaceLeavesRepoRootWorkspaceUntouched(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, "cmd", "agentcore-runner"), 0o755); err != nil {
		t.Fatalf("mkdir runner cmd: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "internal"), 0o755); err != nil {
		t.Fatalf("mkdir internal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "go.mod"), []byte("module example.com/runner\n\ngo 1.24.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "internal", "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write keep file: %v", err)
	}

	if err := compactRunnerWorkspaceInPlace(repoDir); err != nil {
		t.Fatalf("compactRunnerWorkspaceInPlace error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, "internal", "keep.txt")); err != nil {
		t.Fatalf("repo-root workspace content missing after no-op compaction: %v", err)
	}
}

func TestEnsureGoToolchainVersionDirCreatesMissingParent(t *testing.T) {
	layout := NewManagedLayout(t.TempDir())
	versionDir := filepath.Join(layout.GoToolchainsDir, "1.24.0")
	if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
		t.Fatalf("version dir precondition err = %v, want not exist", err)
	}

	got, err := ensureGoToolchainVersionDir(layout, "1.24.0")
	if err != nil {
		t.Fatalf("ensureGoToolchainVersionDir error = %v", err)
	}
	if got != versionDir {
		t.Fatalf("ensureGoToolchainVersionDir = %q, want %q", got, versionDir)
	}
	if stat, err := os.Stat(versionDir); err != nil || !stat.IsDir() {
		t.Fatalf("version dir stat err = %v isDir=%v", err, err == nil && stat.IsDir())
	}
}

func TestExecutePreparedRunnerACPRejectsChecksumMismatch(t *testing.T) {
	manager, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	manager.status = Status{
		BinaryReady:  true,
		BinaryPath:   buildRunnerBinaryForOptimizationTest(t),
		BinarySHA256: strings.Repeat("0", 64),
	}

	_, err = manager.ExecutePreparedRunnerACP(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("ExecutePreparedRunnerACP error = %v, want checksum mismatch", err)
	}
}

func TestGetStatusIncludesLastOptimizationRunSummary(t *testing.T) {
	root := t.TempDir()
	prepareAt := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	writeOptimizationStatusFixture(t, root, Status{
		BinaryReady:           true,
		LastPrepareAt:         &prepareAt,
		LastOptimizationRunID: "opt-123",
	})
	writeOptimizationRunFixture(t, root, "opt-123", map[string]interface{}{
		"id":                 "opt-123",
		"created_at":         "2026-04-03T13:14:15Z",
		"runner_stop_reason": "completed",
		"runner_response_text": strings.Repeat(
			"agentcore-runner received optimization evidence. ",
			8,
		),
	})

	manager, err := NewManager(root)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	status := manager.GetStatus(context.Background())
	if got := status.LastOptimizationState; got != "completed" {
		t.Fatalf("LastOptimizationState = %q, want completed", got)
	}
	if status.LastOptimizationAt == nil || !status.LastOptimizationAt.Equal(time.Date(2026, 4, 3, 13, 14, 15, 0, time.UTC)) {
		t.Fatalf("LastOptimizationAt = %#v", status.LastOptimizationAt)
	}
	if got := status.LastOptimizationSummary; !strings.Contains(got, "agentcore-runner received optimization evidence") {
		t.Fatalf("LastOptimizationSummary = %q", got)
	}
	if len(status.LastOptimizationSummary) > 160 {
		t.Fatalf("LastOptimizationSummary too long: %d", len(status.LastOptimizationSummary))
	}
}

func TestGetStatusIncludesFailedLastOptimizationRunSummary(t *testing.T) {
	root := t.TempDir()
	writeOptimizationStatusFixture(t, root, Status{
		LastOptimizationRunID: "opt-bad",
	})
	writeOptimizationRunFixture(t, root, "opt-bad", map[string]interface{}{
		"id":           "opt-bad",
		"created_at":   "2026-04-03T15:16:17Z",
		"runner_error": "runner binary checksum mismatch: got abc want def",
	})

	manager, err := NewManager(root)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	status := manager.GetStatus(context.Background())
	if got := status.LastOptimizationState; got != "failed" {
		t.Fatalf("LastOptimizationState = %q, want failed", got)
	}
	if got := status.LastOptimizationSummary; !strings.Contains(got, "checksum mismatch") {
		t.Fatalf("LastOptimizationSummary = %q", got)
	}
}

func buildRunnerBinaryForOptimizationTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	serverDir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	bin := filepath.Join(t.TempDir(), "agentcore-runner")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/agentcore-runner")
	cmd.Dir = serverDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build runner failed: %v\n%s", err, output)
	}
	return bin
}

func writeOptimizationStatusFixture(t *testing.T, root string, status Status) {
	t.Helper()
	layout := NewManagedLayout(root)
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(layout.StatusPath), 0o755); err != nil {
		t.Fatalf("mkdir status dir: %v", err)
	}
	if err := os.WriteFile(layout.StatusPath, data, 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
}

func writeOptimizationRunFixture(t *testing.T, root string, id string, payload map[string]interface{}) {
	t.Helper()
	dir := filepath.Join(root, "optimization-runs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir optimization-runs: %v", err)
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal run payload: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o644); err != nil {
		t.Fatalf("write optimization run: %v", err)
	}
}
