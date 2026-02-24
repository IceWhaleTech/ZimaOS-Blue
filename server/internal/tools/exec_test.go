package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

// --- Shell utilities ---

func TestGetShellConfig(t *testing.T) {
	shell, args := GetShellConfig()
	if shell == "" {
		t.Fatal("shell should not be empty")
	}
	if len(args) == 0 {
		t.Fatal("args should not be empty")
	}
	// Last arg should be "-c" on Unix.
	if args[len(args)-1] != "-c" && args[len(args)-1] != "-Command" {
		t.Fatalf("unexpected last arg: %s", args[len(args)-1])
	}
}

func TestSanitizeBinaryOutput(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"hello\nworld", "hello\nworld"},
		{"hello\x00world", "helloworld"},
		{"tab\there", "tab\there"},
		{"\x1b[31mred\x1b[0m", "[31mred[0m"}, // ESC stripped, brackets kept
	}
	for _, tt := range tests {
		got := SanitizeBinaryOutput(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeBinaryOutput(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateHostEnv(t *testing.T) {
	// Should reject dangerous vars.
	for _, key := range []string{"LD_PRELOAD", "DYLD_INSERT_LIBRARIES", "BASH_ENV", "PATH"} {
		err := ValidateHostEnv(map[string]string{key: "value"})
		if err == nil {
			t.Errorf("expected error for %s", key)
		}
	}
	// Should allow safe vars.
	err := ValidateHostEnv(map[string]string{"MY_VAR": "value", "HOME": "/tmp"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveWorkdir(t *testing.T) {
	// Valid dir.
	dir, warnings := ResolveWorkdir(os.TempDir())
	if dir != os.TempDir() {
		t.Errorf("expected %s, got %s", os.TempDir(), dir)
	}
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// Invalid dir falls back.
	dir, warnings = ResolveWorkdir("/nonexistent/path/xyz")
	if dir == "/nonexistent/path/xyz" {
		t.Error("should have fallen back from nonexistent path")
	}
	if len(warnings) == 0 {
		t.Error("expected warnings for invalid workdir")
	}
}

// --- Session registry ---

func TestSessionRegistry(t *testing.T) {
	reg := NewSessionRegistry()
	defer reg.Cleanup()

	session := &ProcessSession{
		ID:        "test-001",
		Command:   "echo hello",
		Workdir:   "/tmp",
		StartedAt: time.Now(),
		Stdout:    NewOutputBuffer(1024),
		Stderr:    NewOutputBuffer(1024),
		Status:    ProcessRunning,
	}
	reg.Add(session)

	// Get running.
	s := reg.Get("test-001")
	if s == nil {
		t.Fatal("session not found")
	}
	if s.Command != "echo hello" {
		t.Errorf("unexpected command: %s", s.Command)
	}

	// Append output.
	s.Stdout.Append("hello\n")
	if s.Stdout.String() != "hello\n" {
		t.Errorf("unexpected stdout: %q", s.Stdout.String())
	}

	// Drain.
	drained := s.Stdout.Drain()
	if drained != "hello\n" {
		t.Errorf("unexpected drain: %q", drained)
	}
	if s.Stdout.String() != "" {
		t.Error("buffer should be empty after drain")
	}

	// Mark exited.
	code := 0
	reg.MarkExited("test-001", &code, "", ProcessCompleted)
	if reg.Get("test-001") != nil {
		t.Error("session should no longer be in running")
	}
	f := reg.GetFinished("test-001")
	if f == nil {
		t.Fatal("session should be in finished")
	}
	if f.Status != ProcessCompleted {
		t.Errorf("unexpected status: %s", f.Status)
	}

	// List.
	running := reg.ListRunning()
	if len(running) != 0 {
		t.Errorf("expected 0 running, got %d", len(running))
	}
	finished := reg.ListFinished()
	if len(finished) != 1 {
		t.Errorf("expected 1 finished, got %d", len(finished))
	}
}

func TestOutputBufferTruncation(t *testing.T) {
	buf := NewOutputBuffer(20)
	buf.Append("12345678901234567890") // exactly 20
	if buf.Truncated() {
		t.Error("should not be truncated at cap")
	}
	buf.Append("extra")
	if !buf.Truncated() {
		t.Error("should be truncated after exceeding cap")
	}
	if buf.Len() > 20 {
		t.Errorf("buffer length %d exceeds cap 20", buf.Len())
	}
}

// --- Security ---

func TestAnalyzeCommand(t *testing.T) {
	tests := []struct {
		command string
		ok      bool
	}{
		{"echo hello", true},
		{"ls -la | grep foo", true},
		{"echo a && echo b", true},
		{"", false},
	}
	for _, tt := range tests {
		analysis := AnalyzeCommand(tt.command, "/tmp")
		if analysis.OK != tt.ok {
			t.Errorf("AnalyzeCommand(%q).OK = %v, want %v (reason: %s)", tt.command, analysis.OK, tt.ok, analysis.Reason)
		}
	}
}

func TestIsSafeBin(t *testing.T) {
	safeBins := BuildSafeBinsSet(DefaultSafeBins)
	if !IsSafeBin("ls", safeBins) {
		t.Error("ls should be safe")
	}
	if !IsSafeBin("grep", safeBins) {
		t.Error("grep should be safe")
	}
	if IsSafeBin("rm", safeBins) {
		t.Error("rm should not be safe")
	}
	if IsSafeBin("", safeBins) {
		t.Error("empty should not be safe")
	}
}

func TestEvaluateAllowlist(t *testing.T) {
	safeBins := BuildSafeBinsSet(DefaultSafeBins)

	// Safe bin should pass.
	analysis := AnalyzeCommand("ls -la", "/tmp")
	satisfied, _ := EvaluateAllowlist(analysis, nil, safeBins)
	if !satisfied {
		t.Error("safe bin should satisfy allowlist")
	}

	// Unknown bin should fail.
	analysis = AnalyzeCommand("custom-tool --flag", "/tmp")
	satisfied, _ = EvaluateAllowlist(analysis, nil, safeBins)
	if satisfied {
		t.Error("unknown bin should not satisfy empty allowlist")
	}
}

func TestAllowlistPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exec-approvals.json")

	// Load non-existent.
	cfg, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}

	// Add entry and save.
	AddAllowlistEntry(cfg, "/usr/bin/python3")
	if len(cfg.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(cfg.Entries))
	}
	if err := SaveAllowlist(path, cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Reload.
	cfg2, err := LoadAllowlist(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if len(cfg2.Entries) != 1 {
		t.Fatalf("expected 1 entry after reload, got %d", len(cfg2.Entries))
	}
	if cfg2.Entries[0].Pattern != "/usr/bin/python3" {
		t.Errorf("unexpected pattern: %s", cfg2.Entries[0].Pattern)
	}

	// Duplicate add.
	AddAllowlistEntry(cfg2, "/usr/bin/python3")
	if len(cfg2.Entries) != 1 {
		t.Error("duplicate should not be added")
	}
}

// --- Approval ---

func TestApprovalFlow(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	mgr := NewApprovalManager(broker)

	// Subscribe to SSE to capture the event.
	ch := broker.Subscribe("test-user")
	defer broker.Unsubscribe("test-user", ch)

	// Request approval in background.
	done := make(chan ApprovalDecision, 1)
	go func() {
		decision, _ := mgr.RequestApproval(context.Background(), ApprovalRequest{
			Command: "rm -rf /",
			Workdir: "/tmp",
			Host:    "local",
			UserID:  "test-user",
		})
		done <- decision
	}()

	// Wait for SSE event.
	select {
	case evt := <-ch:
		if evt.Type != "exec:approval-request" {
			t.Fatalf("unexpected event type: %s", evt.Type)
		}
		// Extract approval ID.
		data, _ := json.Marshal(evt.Data)
		var req ApprovalRequest
		json.Unmarshal(data, &req)
		if req.Command != "rm -rf /" {
			t.Fatalf("unexpected command: %s", req.Command)
		}
		// Resolve.
		mgr.ResolveApproval(req.ID, ApprovalDeny)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}

	// Check decision.
	select {
	case decision := <-done:
		if decision != ApprovalDeny {
			t.Errorf("expected deny, got %s", decision)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for decision")
	}
}

func TestApprovalTimeout(t *testing.T) {
	broker := sse.NewBroker()
	defer broker.Close()

	mgr := NewApprovalManager(broker)
	mgr.timeout = 100 * time.Millisecond // short timeout for test

	decision, err := mgr.RequestApproval(context.Background(), ApprovalRequest{
		Command: "test",
		UserID:  "test-user",
	})
	if decision != ApprovalDeny {
		t.Errorf("expected deny on timeout, got %s", decision)
	}
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got %v", err)
	}
}

// --- Exec tool ---

func TestExecSimpleCommand(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
		MaxOutput:      100_000,
	}, sessions, nil, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res execResult
	if err := json.Unmarshal([]byte(result.(string)), &res); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if res.Status != "completed" {
		t.Errorf("expected completed, got %s", res.Status)
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got %q", res.Stdout)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %v", res.ExitCode)
	}
}

func TestExecExitCode(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
	}, sessions, nil, nil, nil)

	result, _ := tool.Execute(context.Background(), map[string]interface{}{
		"command": "exit 42",
	})

	var res execResult
	json.Unmarshal([]byte(result.(string)), &res)
	if res.Status != "failed" {
		t.Errorf("expected failed, got %s", res.Status)
	}
	if res.ExitCode == nil || *res.ExitCode != 42 {
		t.Errorf("expected exit code 42, got %v", res.ExitCode)
	}
}

func TestExecTimeout(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 1 * time.Second,
		MaxTimeout:     2 * time.Second,
	}, sessions, nil, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "sleep 30",
		"timeout": float64(1),
	})
	// The tool returns both a result and an error on timeout.
	if err != nil && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got %v", err)
	}
	if result != nil {
		var res execResult
		json.Unmarshal([]byte(result.(string)), &res)
		if res.Status != "failed" {
			t.Errorf("expected failed status on timeout, got %s", res.Status)
		}
	}
}

func TestExecSecurityDeny(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security: ExecSecurityDeny,
	}, sessions, nil, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hello",
	})
	if err == nil || !strings.Contains(err.Error(), "deny") {
		t.Errorf("expected deny error, got %v", err)
	}
}

func TestExecDangerousEnv(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 5 * time.Second,
		MaxTimeout:     10 * time.Second,
	}, sessions, nil, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hello",
		"env":     map[string]interface{}{"LD_PRELOAD": "/evil.so"},
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Errorf("expected forbidden error, got %v", err)
	}
}

// --- Process tool ---

func TestProcessToolList(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	sessions.Add(&ProcessSession{
		ID:        "s1",
		Command:   "sleep 100",
		StartedAt: time.Now(),
		Stdout:    NewOutputBuffer(1024),
		Stderr:    NewOutputBuffer(1024),
		Status:    ProcessRunning,
		PID:       12345,
	})

	tool := NewProcessTool(sessions)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "list",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res processListResult
	json.Unmarshal([]byte(result.(string)), &res)
	if len(res.Running) != 1 {
		t.Errorf("expected 1 running, got %d", len(res.Running))
	}
	if res.Running[0].SessionID != "s1" {
		t.Errorf("unexpected session ID: %s", res.Running[0].SessionID)
	}
}

func TestProcessToolPoll(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	s := &ProcessSession{
		ID:        "s2",
		Command:   "echo test",
		StartedAt: time.Now(),
		Stdout:    NewOutputBuffer(1024),
		Stderr:    NewOutputBuffer(1024),
		Status:    ProcessRunning,
	}
	s.Stdout.Append("test output\n")
	sessions.Add(s)

	tool := NewProcessTool(sessions)
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":     "poll",
		"session_id": "s2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var res processPollResult
	json.Unmarshal([]byte(result.(string)), &res)
	if !strings.Contains(res.Stdout, "test output") {
		t.Errorf("expected stdout to contain 'test output', got %q", res.Stdout)
	}
}

// --- Workdir restriction ---

func TestExecAllowedDirs(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	allowedDir := t.TempDir()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
		AllowedDirs:    []string{allowedDir},
	}, sessions, nil, nil, nil)

	// Allowed workdir should succeed.
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo ok",
		"workdir": allowedDir,
	})
	if err != nil {
		t.Fatalf("expected success for allowed dir, got %v", err)
	}
	var res execResult
	json.Unmarshal([]byte(result.(string)), &res)
	if res.Status != "completed" {
		t.Errorf("expected completed, got %s", res.Status)
	}

	// Subdirectory of allowed dir should also succeed.
	subDir := filepath.Join(allowedDir, "sub")
	os.MkdirAll(subDir, 0755)
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo ok",
		"workdir": subDir,
	})
	if err != nil {
		t.Errorf("expected success for subdir, got %v", err)
	}

	// Outside allowed dir should be denied.
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo bad",
		"workdir": "/",
	})
	if err == nil || !strings.Contains(err.Error(), "outside allowed") {
		t.Errorf("expected 'outside allowed' error, got %v", err)
	}
}

func TestExecAllowedDirsEmpty(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	// Empty AllowedDirs = unrestricted.
	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
		AllowedDirs:    nil,
	}, sessions, nil, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo ok",
		"workdir": os.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected success with empty AllowedDirs, got %v", err)
	}
	var res execResult
	json.Unmarshal([]byte(result.(string)), &res)
	if res.Status != "completed" {
		t.Errorf("expected completed, got %s", res.Status)
	}
}

// --- Lang parameter ---

func TestExecLang(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
	}, sessions, nil, nil, nil)

	// Use lang=en-US to set locale for a command.
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo $LANG",
		"lang":    "en-US",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var res execResult
	json.Unmarshal([]byte(result.(string)), &res)
	if !strings.Contains(res.Stdout, "en_US.UTF-8") {
		t.Errorf("expected LANG=en_US.UTF-8 in stdout, got %q", res.Stdout)
	}
}

func TestLocaleFromTag(t *testing.T) {
	tests := []struct {
		tag, want string
	}{
		{"en-US", "en_US.UTF-8"},
		{"zh-CN", "zh_CN.UTF-8"},
		{"ja-JP", "ja_JP.UTF-8"},
		{"en", "en.UTF-8"},
		{"en_US", "en_US.UTF-8"},
		{"en_US.UTF-8", "en_US.UTF-8"},
		{"", "en_US.UTF-8"},
	}
	for _, tt := range tests {
		got := localeFromTag(tt.tag)
		if got != tt.want {
			t.Errorf("localeFromTag(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}

// --- Directory allowlist store ---

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestDirAllowlistStore(t *testing.T) {
	db := newTestDB(t)
	store, err := NewDirAllowlistStore(db)
	if err != nil {
		t.Fatal(err)
	}

	// Empty store — no match.
	if store.Match("/foo/bar") != nil {
		t.Error("expected no match on empty store")
	}

	// Add /foo/bar.
	if err := store.Add("/foo/bar", "user1"); err != nil {
		t.Fatal(err)
	}

	// Exact match.
	if e := store.Match("/foo/bar"); e == nil {
		t.Error("expected match for /foo/bar")
	}

	// Subdirectory match.
	if e := store.Match("/foo/bar/baz"); e == nil {
		t.Error("expected match for /foo/bar/baz (child of /foo/bar)")
	}

	// Parent should NOT match.
	if e := store.Match("/foo"); e != nil {
		t.Error("expected no match for /foo (parent of /foo/bar)")
	}

	// Add parent /foo — should NOT merge with /foo/bar.
	if err := store.Add("/foo", "user1"); err != nil {
		t.Fatal(err)
	}
	entries, _ := store.List()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (no merge), got %d", len(entries))
	}

	// Now /foo should match.
	if e := store.Match("/foo"); e == nil {
		t.Error("expected match for /foo after adding it")
	}

	// Duplicate add should be ignored.
	if err := store.Add("/foo/bar", "user2"); err != nil {
		t.Fatal(err)
	}
	entries, _ = store.List()
	if len(entries) != 2 {
		t.Errorf("expected 2 entries after duplicate add, got %d", len(entries))
	}

	// Delete.
	if err := store.Delete(entries[0].ID); err != nil {
		t.Fatal(err)
	}
	entries, _ = store.List()
	if len(entries) != 1 {
		t.Errorf("expected 1 entry after delete, got %d", len(entries))
	}
}

func TestExecDirApprovalFlow(t *testing.T) {
	db := newTestDB(t)
	dirStore, err := NewDirAllowlistStore(db)
	if err != nil {
		t.Fatal(err)
	}

	broker := sse.NewBroker()
	defer broker.Close()
	approvals := NewApprovalManager(broker)

	allowedDir := t.TempDir()
	outsideDir := t.TempDir()

	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
		AllowedDirs:    []string{allowedDir},
	}, sessions, approvals, broker, dirStore)

	// Allowed dir should work without approval.
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo ok",
		"workdir": allowedDir,
	})
	if err != nil {
		t.Fatalf("expected success for allowed dir, got %v", err)
	}
	var res execResult
	json.Unmarshal([]byte(result.(string)), &res)
	if res.Status != "completed" {
		t.Errorf("expected completed, got %s", res.Status)
	}

	// Outside dir should trigger approval. Simulate "allow-always".
	ch := broker.Subscribe("default")
	defer broker.Unsubscribe("default", ch)

	done := make(chan error, 1)
	go func() {
		_, err := tool.Execute(context.Background(), map[string]interface{}{
			"command": "echo approved",
			"workdir": outsideDir,
		})
		done <- err
	}()

	// Wait for approval SSE event.
	select {
	case evt := <-ch:
		if evt.Type != "exec:approval-request" {
			// Could be exec:started — skip and wait for approval.
			evt = <-ch
		}
		data, _ := json.Marshal(evt.Data)
		var req ApprovalRequest
		json.Unmarshal(data, &req)
		if req.Type != "directory" {
			t.Errorf("expected type=directory, got %s", req.Type)
		}
		approvals.ResolveApproval(req.ID, ApprovalAllowAlways)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for approval SSE event")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected success after approval, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for exec result")
	}

	// After "allow-always", the dir should be persisted — no approval needed.
	entries, _ := dirStore.List()
	found := false
	for _, e := range entries {
		if e.Path == filepath.Clean(outsideDir) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected outsideDir to be persisted in dir allowlist")
	}

	// Second exec to same dir should succeed without approval.
	result, err = tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo cached",
		"workdir": outsideDir,
	})
	if err != nil {
		t.Fatalf("expected success for persisted dir, got %v", err)
	}
	json.Unmarshal([]byte(result.(string)), &res)
	if res.Status != "completed" {
		t.Errorf("expected completed, got %s", res.Status)
	}
}

// --- Command path validation ---

func TestExtractAbsolutePaths(t *testing.T) {
	tests := []struct {
		command string
		want    []string
	}{
		{"echo hello", nil},
		{"cat /etc/passwd", []string{"/etc/passwd"}},
		{"ls /home/user/docs", []string{"/home/user/docs"}},
		{"cd /var/log && cat syslog", []string{"/var/log"}},
		{"echo /dev/null", nil}, // /dev/null skipped
		{"ls /foo/bar /baz/qux", []string{"/foo/bar", "/baz/qux"}},
	}
	for _, tt := range tests {
		got := extractAbsolutePaths(tt.command)
		if len(got) != len(tt.want) {
			t.Errorf("extractAbsolutePaths(%q) = %v, want %v", tt.command, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("extractAbsolutePaths(%q)[%d] = %q, want %q", tt.command, i, got[i], tt.want[i])
			}
		}
	}
}

func TestValidateCommandPathsBlocks(t *testing.T) {
	allowedDir := t.TempDir()
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
		AllowedDirs:    []string{allowedDir},
	}, sessions, nil, nil, nil)

	// Command referencing allowed dir should succeed.
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "ls " + allowedDir,
		"workdir": allowedDir,
	})
	if err != nil {
		t.Fatalf("expected success for allowed path in command, got %v", err)
	}
	var res execResult
	json.Unmarshal([]byte(result.(string)), &res)
	if res.Status != "completed" {
		t.Errorf("expected completed, got %s", res.Status)
	}

	// Command referencing /etc should be denied (no approval manager).
	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"command": "cat /etc/passwd",
		"workdir": allowedDir,
	})
	if err == nil || !strings.Contains(err.Error(), "outside allowed") {
		t.Errorf("expected 'outside allowed' error for /etc/passwd, got %v", err)
	}
}

func TestValidateCommandPathsApproval(t *testing.T) {
	db := newTestDB(t)
	dirStore, err := NewDirAllowlistStore(db)
	if err != nil {
		t.Fatal(err)
	}

	broker := sse.NewBroker()
	defer broker.Close()
	approvals := NewApprovalManager(broker)

	allowedDir := t.TempDir()
	targetDir := t.TempDir()

	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
		AllowedDirs:    []string{allowedDir},
	}, sessions, approvals, broker, dirStore)

	// Command referencing targetDir should trigger approval.
	ch := broker.Subscribe("default")
	defer broker.Unsubscribe("default", ch)

	done := make(chan error, 1)
	go func() {
		_, err := tool.Execute(context.Background(), map[string]interface{}{
			"command": "ls " + targetDir,
			"workdir": allowedDir,
		})
		done <- err
	}()

	// Wait for approval event and approve.
	select {
	case evt := <-ch:
		if evt.Type != "exec:approval-request" {
			evt = <-ch // skip exec:started
		}
		data, _ := json.Marshal(evt.Data)
		var req ApprovalRequest
		json.Unmarshal(data, &req)
		if req.Type != "directory" {
			t.Errorf("expected type=directory, got %s", req.Type)
		}
		approvals.ResolveApproval(req.ID, ApprovalAllowAlways)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for approval")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected success after approval, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for exec")
	}

	// After allow-always, the dir should be persisted.
	entries, _ := dirStore.List()
	found := false
	for _, e := range entries {
		if e.Path == filepath.Clean(targetDir) {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected targetDir to be persisted in dir allowlist")
	}
}

func TestExecDefaultWorkdirInsideAllowedDirs(t *testing.T) {
	allowedDir := t.TempDir()
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
		AllowedDirs:    []string{allowedDir},
	}, sessions, nil, nil, nil)

	// No workdir specified — should default to AllowedDirs[0], not os.Getwd().
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "pwd",
	})
	if err != nil {
		t.Fatalf("expected success with default workdir, got %v", err)
	}
	var res execResult
	json.Unmarshal([]byte(result.(string)), &res)
	if !strings.Contains(res.Stdout, allowedDir) {
		t.Errorf("expected pwd output to contain %q, got %q", allowedDir, res.Stdout)
	}
}

func TestExecSandboxModeNoManager(t *testing.T) {
	sessions := NewSessionRegistry()
	defer sessions.Cleanup()

	tool := NewExecTool(ExecConfig{
		Security:       ExecSecurityFull,
		DefaultTimeout: 10 * time.Second,
		MaxTimeout:     30 * time.Second,
	}, sessions, nil, nil, nil)

	// Requesting sandbox mode without a sandbox manager should fail.
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"command": "echo hello",
		"host":    "sandbox",
	})
	if err == nil || !strings.Contains(err.Error(), "no sandbox manager") {
		t.Errorf("expected 'no sandbox manager' error, got %v", err)
	}
}
