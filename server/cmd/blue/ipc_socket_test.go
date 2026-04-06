package main

import (
	"reflect"
	"testing"
)

func TestCandidateIPCSocketPaths_Defaults(t *testing.T) {
	t.Setenv("BLUE_IPC_SOCKET", "")
	got := candidateIPCSocketPaths()
	want := []string{getDataDir() + "/blue.sock", "/tmp/blue.sock"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidateIPCSocketPaths() = %v, want %v", got, want)
	}
}

func TestCandidateIPCSocketPaths_EnvOverride(t *testing.T) {
	t.Setenv("BLUE_IPC_SOCKET", "/tmp/blue-webfetch-test.sock")
	got := candidateIPCSocketPaths()
	want := []string{"/tmp/blue-webfetch-test.sock"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidateIPCSocketPaths() = %v, want %v", got, want)
	}
}

func TestAppendSocketCandidate_DeduplicatesAndSkipsEmpty(t *testing.T) {
	got := appendSocketCandidate(nil, "", "/tmp/blue.sock", " /tmp/blue.sock ", "/tmp/other.sock")
	want := []string{"/tmp/blue.sock", "/tmp/other.sock"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appendSocketCandidate() = %v, want %v", got, want)
	}
}

func TestCandidateAuditIPCSocketPaths_Defaults(t *testing.T) {
	t.Setenv("BLUE_AUDIT_IPC_SOCKET", "")
	got := candidateAuditIPCSocketPaths()
	want := []string{getDataDir() + "/session_audit.sock", "/tmp/session_audit.sock"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidateAuditIPCSocketPaths() = %v, want %v", got, want)
	}
}

func TestCandidateAuditIPCSocketPaths_EnvOverride(t *testing.T) {
	t.Setenv("BLUE_AUDIT_IPC_SOCKET", "/tmp/blue-audit.sock")
	got := candidateAuditIPCSocketPaths()
	want := []string{"/tmp/blue-audit.sock"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidateAuditIPCSocketPaths() = %v, want %v", got, want)
	}
}
