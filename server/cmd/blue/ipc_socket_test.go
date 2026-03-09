package main

import (
	"reflect"
	"testing"
)

func TestCandidateIPCSocketPaths_Defaults(t *testing.T) {
	t.Setenv("BLUE_IPC_SOCKET", "")
	got := candidateIPCSocketPaths()
	want := []string{"/tmp/blue.sock", getDataDir() + "/blue.sock"}
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
