//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func resetGatewayDaemonCLIState(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	oldCfgFile, oldDevMode, oldProfile := cfgFile, devMode, profile
	oldNoColor, oldVerbose := noColor, verbose
	oldGatewayPort, oldGatewayBind, oldSessionAuditDB := gatewayPort, gatewayBind, sessionAuditDB
	t.Cleanup(func() {
		cfgFile, devMode, profile = oldCfgFile, oldDevMode, oldProfile
		noColor, verbose = oldNoColor, oldVerbose
		gatewayPort, gatewayBind, sessionAuditDB = oldGatewayPort, oldGatewayBind, oldSessionAuditDB
	})

	cfgFile = ""
	devMode = false
	profile = ""
	noColor = false
	verbose = false
	gatewayPort = 0
	gatewayBind = ""
	sessionAuditDB = ""
}

func TestGatewayProcessArgsIncludeGlobalFlags(t *testing.T) {
	resetGatewayDaemonCLIState(t)

	cfgFile = filepath.Join(t.TempDir(), "config.yaml")
	devMode = true
	profile = "ops"
	noColor = true
	verbose = true

	wantWorker := []string{"--config", cfgFile, "--dev", "--profile", "ops", "--verbose", "--no-color", "gateway", "run"}
	if got := gatewayWorkerArgs(); !reflect.DeepEqual(got, wantWorker) {
		t.Fatalf("gatewayWorkerArgs() = %#v, want %#v", got, wantWorker)
	}

	wantSupervisor := []string{"--config", cfgFile, "--dev", "--profile", "ops", "--verbose", "--no-color", "gateway", "supervise"}
	if got := gatewaySupervisorArgs(); !reflect.DeepEqual(got, wantSupervisor) {
		t.Fatalf("gatewaySupervisorArgs() = %#v, want %#v", got, wantSupervisor)
	}
}

func TestGatewayProcessArgsIncludeBindAndPortOverrides(t *testing.T) {
	resetGatewayDaemonCLIState(t)

	gatewayPort = 18080
	gatewayBind = "127.0.0.1"

	wantWorker := []string{"--port", "18080", "--bind", "127.0.0.1", "gateway", "run"}
	if got := gatewayWorkerArgs(); !reflect.DeepEqual(got, wantWorker) {
		t.Fatalf("gatewayWorkerArgs() = %#v, want %#v", got, wantWorker)
	}

	wantSupervisor := []string{"--port", "18080", "--bind", "127.0.0.1", "gateway", "supervise"}
	if got := gatewaySupervisorArgs(); !reflect.DeepEqual(got, wantSupervisor) {
		t.Fatalf("gatewaySupervisorArgs() = %#v, want %#v", got, wantSupervisor)
	}
}

func TestGatewayProcessArgsIncludeSessionAuditDBOverride(t *testing.T) {
	resetGatewayDaemonCLIState(t)

	sessionAuditDB = "audit/pinchbench.db"

	wantWorker := []string{"--session-audit-db", "audit/pinchbench.db", "gateway", "run"}
	if got := gatewayWorkerArgs(); !reflect.DeepEqual(got, wantWorker) {
		t.Fatalf("gatewayWorkerArgs() = %#v, want %#v", got, wantWorker)
	}

	wantSupervisor := []string{"--session-audit-db", "audit/pinchbench.db", "gateway", "supervise"}
	if got := gatewaySupervisorArgs(); !reflect.DeepEqual(got, wantSupervisor) {
		t.Fatalf("gatewaySupervisorArgs() = %#v, want %#v", got, wantSupervisor)
	}
}

func TestGatewayDaemonStateRoundTrip(t *testing.T) {
	resetGatewayDaemonCLIState(t)

	paths := gatewayDaemonFiles()
	now := time.Now().UTC().Truncate(time.Second)
	state := gatewayDaemonState{
		SupervisorPID: 101,
		ChildPID:      202,
		Status:        "running",
		RestartCount:  3,
		StartedAt:     now,
		LastStartAt:   now,
		LastExitAt:    now,
		LastExitCode:  9,
		LastError:     "boom",
		Profile:       "ops",
		DevMode:       true,
		UpdatedAt:     now,
	}

	if err := writeGatewayDaemonState(paths, state); err != nil {
		t.Fatalf("writeGatewayDaemonState() error = %v", err)
	}

	got, err := readGatewayDaemonState(paths)
	if err != nil {
		t.Fatalf("readGatewayDaemonState() error = %v", err)
	}
	if got.SupervisorPID != state.SupervisorPID || got.ChildPID != state.ChildPID || got.Status != state.Status {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, state)
	}
	if got.RestartCount != state.RestartCount || got.LastExitCode != state.LastExitCode || got.LastError != state.LastError {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, state)
	}
}

func TestReadGatewayDaemonRuntimeReportsLivePIDs(t *testing.T) {
	resetGatewayDaemonCLIState(t)

	paths := gatewayDaemonFiles()
	pid := os.Getpid()
	state := gatewayDaemonState{
		SupervisorPID: pid,
		ChildPID:      pid,
		Status:        "running",
		StartedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}
	if err := writeGatewayDaemonState(paths, state); err != nil {
		t.Fatalf("writeGatewayDaemonState() error = %v", err)
	}
	if err := writeGatewayPIDFile(paths.pidFile, pid); err != nil {
		t.Fatalf("writeGatewayPIDFile() error = %v", err)
	}

	runtime, err := readGatewayDaemonRuntime(paths)
	if err != nil {
		t.Fatalf("readGatewayDaemonRuntime() error = %v", err)
	}
	if !runtime.SupervisorRunning {
		t.Fatal("expected supervisor to be reported as running")
	}
	if !runtime.ChildRunning {
		t.Fatal("expected child to be reported as running")
	}
	if runtime.State == nil || runtime.State.SupervisorPID != pid {
		t.Fatalf("unexpected state: %+v", runtime.State)
	}
}
