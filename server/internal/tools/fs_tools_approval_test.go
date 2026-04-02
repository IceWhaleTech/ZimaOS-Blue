package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

func approvalAwareReadTool(allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) *FileReadTool {
	tool := NewFileReadTool(allowedPaths, 0)
	tool.scope = tool.scope.withApprovalFlow(approvals, dirStore)
	return tool
}

func approvalAwareWriteTool(allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) *FileWriteTool {
	tool := NewFileWriteTool(allowedPaths, 0)
	tool.scope = tool.scope.withApprovalFlow(approvals, dirStore)
	return tool
}

func approvalAwareDeleteTool(allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) *FileDeleteTool {
	tool := NewFileDeleteTool(allowedPaths)
	tool.scope = tool.scope.withApprovalFlow(approvals, dirStore)
	return tool
}

func approvalAwareLsTool(allowedPaths []string, approvals *ApprovalManager, dirStore *DirAllowlistStore) *LsTool {
	tool := NewLsTool(allowedPaths)
	tool.Scope = tool.Scope.withApprovalFlow(approvals, dirStore)
	return tool
}

func waitApprovalRequest(t *testing.T, ch <-chan sse.Event) ApprovalRequest {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case evt := <-ch:
			if evt.Type != "exec:approval-request" {
				continue
			}
			data, err := json.Marshal(evt.Data)
			if err != nil {
				t.Fatalf("marshal approval request: %v", err)
			}
			var req ApprovalRequest
			if err := json.Unmarshal(data, &req); err != nil {
				t.Fatalf("unmarshal approval request: %v", err)
			}
			return req
		case <-deadline:
			t.Fatal("timeout waiting for approval request")
		}
	}
}

func TestFSToolsExternalAbsolutePathsRequestApproval(t *testing.T) {
	db := newTestDB(t)
	dirStore, err := NewDirAllowlistStore(db)
	if err != nil {
		t.Fatal(err)
	}
	broker := sse.NewBroker()
	defer broker.Close()
	approvals := NewApprovalManager(broker)
	ch := broker.Subscribe("default")
	defer broker.Unsubscribe("default", ch)

	workspaceRoot := t.TempDir()
	externalRoot := t.TempDir()
	externalFile := filepath.Join(externalRoot, "notes.txt")
	writeFile(t, externalFile, "external note")

	tests := []struct {
		name             string
		run              func() (interface{}, error)
		wantCommand      string
		wantApprovedPath string
	}{
		{
			name: "file_read",
			run: func() (interface{}, error) {
				return approvalAwareReadTool([]string{workspaceRoot}, approvals, dirStore).Execute(context.Background(), map[string]interface{}{
					"path": externalFile,
				})
			},
			wantCommand:      "file_read " + externalFile,
			wantApprovedPath: externalRoot,
		},
		{
			name: "file_write",
			run: func() (interface{}, error) {
				return approvalAwareWriteTool([]string{workspaceRoot}, approvals, dirStore).Execute(context.Background(), map[string]interface{}{
					"path":    filepath.Join(externalRoot, "written.txt"),
					"content": "hello",
				})
			},
			wantCommand:      "file_write " + filepath.Join(externalRoot, "written.txt"),
			wantApprovedPath: externalRoot,
		},
		{
			name: "ls",
			run: func() (interface{}, error) {
				return approvalAwareLsTool([]string{workspaceRoot}, approvals, dirStore).Execute(context.Background(), map[string]interface{}{
					"path": externalRoot,
				})
			},
			wantCommand:      "ls " + externalRoot,
			wantApprovedPath: externalRoot,
		},
		{
			name: "file_delete",
			run: func() (interface{}, error) {
				target := filepath.Join(externalRoot, "delete-me.txt")
				writeFile(t, target, "bye")
				return approvalAwareDeleteTool([]string{workspaceRoot}, approvals, dirStore).Execute(context.Background(), map[string]interface{}{
					"path": target,
				})
			},
			wantCommand:      "file_delete " + filepath.Join(externalRoot, "delete-me.txt"),
			wantApprovedPath: externalRoot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, err := tt.run()
				done <- err
			}()

			req := waitApprovalRequest(t, ch)
			if req.Type != "directory" {
				t.Fatalf("approval type = %q, want directory", req.Type)
			}
			if req.Directory != filepath.Clean(tt.wantApprovedPath) {
				t.Fatalf("approval directory = %q, want %q", req.Directory, filepath.Clean(tt.wantApprovedPath))
			}
			if req.Command != tt.wantCommand {
				t.Fatalf("approval command = %q, want %q", req.Command, tt.wantCommand)
			}
			approvals.ResolveApprovalWithBinding(req.ID, ApprovalAllowOnce, req.BindingHash)

			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("tool execution failed after approval: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for tool execution")
			}
		})
	}
}

func TestFSToolAllowOnceOnlyAppliesToCurrentCall(t *testing.T) {
	db := newTestDB(t)
	dirStore, err := NewDirAllowlistStore(db)
	if err != nil {
		t.Fatal(err)
	}
	broker := sse.NewBroker()
	defer broker.Close()
	approvals := NewApprovalManager(broker)
	ch := broker.Subscribe("default")
	defer broker.Unsubscribe("default", ch)

	workspaceRoot := t.TempDir()
	externalRoot := t.TempDir()
	externalFile := filepath.Join(externalRoot, "notes.txt")
	writeFile(t, externalFile, "external note")

	tool := approvalAwareReadTool([]string{workspaceRoot}, approvals, dirStore)

	firstDone := make(chan error, 1)
	go func() {
		_, err := tool.Execute(context.Background(), map[string]interface{}{"path": externalFile})
		firstDone <- err
	}()
	firstReq := waitApprovalRequest(t, ch)
	approvals.ResolveApprovalWithBinding(firstReq.ID, ApprovalAllowOnce, firstReq.BindingHash)
	if err := <-firstDone; err != nil {
		t.Fatalf("first execution failed: %v", err)
	}

	secondDone := make(chan error, 1)
	go func() {
		_, err := tool.Execute(context.Background(), map[string]interface{}{"path": externalFile})
		secondDone <- err
	}()
	secondReq := waitApprovalRequest(t, ch)
	if secondReq.ID == firstReq.ID {
		t.Fatalf("expected a new approval request ID, got reused %q", secondReq.ID)
	}
	approvals.ResolveApprovalWithBinding(secondReq.ID, ApprovalAllowOnce, secondReq.BindingHash)
	if err := <-secondDone; err != nil {
		t.Fatalf("second execution failed: %v", err)
	}
}

func TestFSToolAllowAlwaysPersistsDirectory(t *testing.T) {
	db := newTestDB(t)
	dirStore, err := NewDirAllowlistStore(db)
	if err != nil {
		t.Fatal(err)
	}
	broker := sse.NewBroker()
	defer broker.Close()
	approvals := NewApprovalManager(broker)
	ch := broker.Subscribe("default")
	defer broker.Unsubscribe("default", ch)

	workspaceRoot := t.TempDir()
	externalRoot := t.TempDir()
	firstFile := filepath.Join(externalRoot, "first.txt")
	secondFile := filepath.Join(externalRoot, "second.txt")
	writeFile(t, firstFile, "one")
	writeFile(t, secondFile, "two")

	tool := approvalAwareReadTool([]string{workspaceRoot}, approvals, dirStore)

	done := make(chan error, 1)
	go func() {
		_, err := tool.Execute(context.Background(), map[string]interface{}{"path": firstFile})
		done <- err
	}()
	req := waitApprovalRequest(t, ch)
	approvals.ResolveApprovalWithBinding(req.ID, ApprovalAllowAlways, req.BindingHash)
	if err := <-done; err != nil {
		t.Fatalf("first execution failed: %v", err)
	}

	entries, err := dirStore.List()
	if err != nil {
		t.Fatalf("list allowlist: %v", err)
	}
	found := false
	for _, entry := range entries {
		if entry.Path == filepath.Clean(externalRoot) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %q in persistent allowlist, got %+v", filepath.Clean(externalRoot), entries)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{"path": secondFile}); err != nil {
		t.Fatalf("second execution failed without prompt: %v", err)
	}
	select {
	case evt := <-ch:
		t.Fatalf("unexpected approval event after allow-always persisted: %+v", evt)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestFSToolApprovalDenyAndTimeoutReturnHelpfulErrors(t *testing.T) {
	t.Run("deny", func(t *testing.T) {
		db := newTestDB(t)
		dirStore, err := NewDirAllowlistStore(db)
		if err != nil {
			t.Fatal(err)
		}
		broker := sse.NewBroker()
		defer broker.Close()
		approvals := NewApprovalManager(broker)
		ch := broker.Subscribe("default")
		defer broker.Unsubscribe("default", ch)

		workspaceRoot := t.TempDir()
		externalFile := filepath.Join(t.TempDir(), "deny.txt")
		writeFile(t, externalFile, "deny")
		tool := approvalAwareReadTool([]string{workspaceRoot}, approvals, dirStore)

		done := make(chan error, 1)
		go func() {
			_, err := tool.Execute(context.Background(), map[string]interface{}{"path": externalFile})
			done <- err
		}()
		req := waitApprovalRequest(t, ch)
		approvals.ResolveApprovalWithBinding(req.ID, ApprovalDeny, req.BindingHash)

		err = <-done
		if err == nil || !strings.Contains(err.Error(), "path access denied") {
			t.Fatalf("deny error = %v, want helpful access denied message", err)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		db := newTestDB(t)
		dirStore, err := NewDirAllowlistStore(db)
		if err != nil {
			t.Fatal(err)
		}
		broker := sse.NewBroker()
		defer broker.Close()
		sub := broker.Subscribe("default")
		defer broker.Unsubscribe("default", sub)
		approvals := NewApprovalManager(broker)
		approvals.timeout = 25 * time.Millisecond

		workspaceRoot := t.TempDir()
		externalFile := filepath.Join(t.TempDir(), "timeout.txt")
		writeFile(t, externalFile, "timeout")
		tool := approvalAwareReadTool([]string{workspaceRoot}, approvals, dirStore)

		_, err = tool.Execute(context.Background(), map[string]interface{}{"path": externalFile})
		if err == nil || !strings.Contains(err.Error(), "path approval failed") || !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("timeout error = %v, want approval timeout message", err)
		}
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
