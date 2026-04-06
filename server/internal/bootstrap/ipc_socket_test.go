package bootstrap

import "testing"

func TestResolveIPCSocketPath_Default(t *testing.T) {
	t.Setenv("BLUE_IPC_SOCKET", "")
	got := resolveIPCSocketPath("/tmp/blue-home/.zimaos-blue/data")
	if got != "/tmp/blue-home/.zimaos-blue/data/blue.sock" {
		t.Fatalf("resolveIPCSocketPath() = %q", got)
	}
}

func TestResolveIPCSocketPath_EnvOverride(t *testing.T) {
	t.Setenv("BLUE_IPC_SOCKET", "/tmp/blue-webfetch-test.sock")
	got := resolveIPCSocketPath("/tmp/ignored")
	if got != "/tmp/blue-webfetch-test.sock" {
		t.Fatalf("resolveIPCSocketPath() = %q", got)
	}
}

func TestResolveIPCSocketPath_EmptyDataDirFallsBackToTmp(t *testing.T) {
	t.Setenv("BLUE_IPC_SOCKET", "")
	got := resolveIPCSocketPath("   ")
	if got != "/tmp/blue.sock" {
		t.Fatalf("resolveIPCSocketPath() = %q", got)
	}
}

func TestResolveAuditIPCSocketPath_Default(t *testing.T) {
	t.Setenv("BLUE_AUDIT_IPC_SOCKET", "")
	got := ResolveAuditIPCSocketPath("/tmp/blue-home/.zimaos-blue/data")
	if got != "/tmp/blue-home/.zimaos-blue/data/session_audit.sock" {
		t.Fatalf("ResolveAuditIPCSocketPath() = %q", got)
	}
}

func TestResolveAuditIPCSocketPath_EnvOverride(t *testing.T) {
	t.Setenv("BLUE_AUDIT_IPC_SOCKET", "/tmp/blue-audit.sock")
	got := ResolveAuditIPCSocketPath("/tmp/ignored")
	if got != "/tmp/blue-audit.sock" {
		t.Fatalf("ResolveAuditIPCSocketPath() = %q", got)
	}
}

func TestResolveAuditIPCSocketPath_EmptyDataDirFallsBackToTmp(t *testing.T) {
	t.Setenv("BLUE_AUDIT_IPC_SOCKET", "")
	got := ResolveAuditIPCSocketPath("   ")
	if got != "/tmp/session_audit.sock" {
		t.Fatalf("ResolveAuditIPCSocketPath() = %q", got)
	}
}
