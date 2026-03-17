//go:build !windows

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

const (
	gatewayDaemonStartTimeout = 5 * time.Second
	gatewayDaemonStopTimeout  = 30 * time.Second
	gatewayDaemonRestartDelay = 2 * time.Second
	gatewayDaemonPollInterval = 200 * time.Millisecond
)

var errGatewayNotRunning = errors.New("gateway is not running")

type gatewayDaemonPaths struct {
	runDir    string
	pidFile   string
	stateFile string
	logFile   string
}

type gatewayDaemonState struct {
	SupervisorPID int       `json:"supervisor_pid"`
	ChildPID      int       `json:"child_pid,omitempty"`
	Status        string    `json:"status"`
	RestartCount  int       `json:"restart_count"`
	StartedAt     time.Time `json:"started_at"`
	LastStartAt   time.Time `json:"last_start_at,omitempty"`
	LastExitAt    time.Time `json:"last_exit_at,omitempty"`
	LastExitCode  int       `json:"last_exit_code,omitempty"`
	LastError     string    `json:"last_error,omitempty"`
	ConfigFile    string    `json:"config_file,omitempty"`
	Profile       string    `json:"profile,omitempty"`
	DevMode       bool      `json:"dev_mode,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type gatewayDaemonRuntime struct {
	State             *gatewayDaemonState
	SupervisorRunning bool
	ChildRunning      bool
}

type gatewayHealthStatus struct {
	Healthy bool
	Status  string
	Service string
	Error   string
	URL     string
}

func runGatewayStart(cmd *cobra.Command, args []string) {
	paths := gatewayDaemonFiles()
	runtime, err := readGatewayDaemonRuntime(paths)
	if err != nil {
		printGatewayError("Failed to inspect gateway daemon state", err)
		os.Exit(1)
	}
	if runtime.SupervisorRunning && runtime.State != nil {
		printGatewayStartResult("Gateway daemon is already running", runtime.State.SupervisorPID, paths.logFile)
		return
	}

	health := queryGatewayHealth(2 * time.Second)
	if health.Healthy {
		printGatewayError("Gateway is already running outside daemon mode", nil)
		os.Exit(1)
	}

	if err := startDetachedGatewaySupervisor(paths); err != nil {
		printGatewayError("Failed to start gateway daemon", err)
		os.Exit(1)
	}

	pid, err := waitForGatewaySupervisorPID(paths, gatewayDaemonStartTimeout)
	if err != nil {
		printGatewayError("Gateway daemon did not become ready in time", err)
		os.Exit(1)
	}

	printGatewayStartResult("Gateway daemon started", pid, paths.logFile)
}

func runGatewayStop(cmd *cobra.Command, args []string) {
	paths := gatewayDaemonFiles()
	stopped, err := stopGatewayManagedInstance(paths)
	if err != nil {
		if errors.Is(err, errGatewayNotRunning) {
			printGatewayError("Gateway is not running", nil)
		} else {
			printGatewayError("Failed to stop gateway", err)
		}
		if errors.Is(err, errGatewayNotRunning) {
			return
		}
		os.Exit(1)
	}
	if !stopped {
		printGatewayError("Gateway is not running", nil)
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": true,
			"message": "Gateway stopped",
		})
		return
	}

	fmt.Println("Gateway stopped")
}

func runGatewayRestart(cmd *cobra.Command, args []string) {
	paths := gatewayDaemonFiles()
	stopped, err := stopGatewayManagedInstance(paths)
	if err != nil && !errors.Is(err, errGatewayNotRunning) {
		printGatewayError("Failed to restart gateway", err)
		os.Exit(1)
	}

	if stopped {
		deadline := time.Now().Add(gatewayDaemonStartTimeout)
		for time.Now().Before(deadline) {
			runtime, readErr := readGatewayDaemonRuntime(paths)
			if readErr == nil && !runtime.SupervisorRunning {
				break
			}
			time.Sleep(gatewayDaemonPollInterval)
		}
	}

	if err := startDetachedGatewaySupervisor(paths); err != nil {
		printGatewayError("Failed to restart gateway", err)
		os.Exit(1)
	}

	pid, err := waitForGatewaySupervisorPID(paths, gatewayDaemonStartTimeout)
	if err != nil {
		printGatewayError("Gateway daemon did not become ready in time", err)
		os.Exit(1)
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success":        true,
			"message":        "Gateway daemon restarted",
			"supervisor_pid": pid,
			"log_file":       paths.logFile,
			"url":            getServiceBaseURL(),
		})
		return
	}

	fmt.Printf("Gateway daemon restarted (pid %d)\n", pid)
	fmt.Printf("Logs: %s\n", paths.logFile)
	fmt.Printf("URL:  %s\n", getServiceBaseURL())
}

func runGatewayStatus(cmd *cobra.Command, args []string) {
	paths := gatewayDaemonFiles()
	runtime, err := readGatewayDaemonRuntime(paths)
	if err != nil {
		printGatewayError("Failed to inspect gateway status", err)
		os.Exit(1)
	}

	health := queryGatewayHealth(2 * time.Second)
	mode := "stopped"
	if runtime.SupervisorRunning {
		mode = "daemon"
	} else if health.Healthy {
		mode = "foreground"
	}

	state := runtime.State
	if jsonOutput {
		payload := map[string]interface{}{
			"mode":               mode,
			"daemon_running":     runtime.SupervisorRunning,
			"worker_running":     runtime.ChildRunning,
			"healthy":            health.Healthy,
			"health_status":      health.Status,
			"url":                getServiceBaseURL(),
			"log_file":           paths.logFile,
			"supervisor_pid":     0,
			"worker_pid":         0,
			"restart_count":      0,
			"daemon_status":      "",
			"last_exit_code":     0,
			"last_error":         "",
			"last_exit_at":       nil,
			"last_start_at":      nil,
			"daemon_started_at":  nil,
			"health_error":       health.Error,
			"health_service":     health.Service,
			"state_file_present": state != nil,
		}
		if state != nil {
			payload["supervisor_pid"] = state.SupervisorPID
			payload["worker_pid"] = state.ChildPID
			payload["restart_count"] = state.RestartCount
			payload["daemon_status"] = state.Status
			payload["last_exit_code"] = state.LastExitCode
			payload["last_error"] = state.LastError
			if !state.LastExitAt.IsZero() {
				payload["last_exit_at"] = state.LastExitAt
			}
			if !state.LastStartAt.IsZero() {
				payload["last_start_at"] = state.LastStartAt
			}
			if !state.StartedAt.IsZero() {
				payload["daemon_started_at"] = state.StartedAt
			}
		}
		printJSON(payload)
		return
	}

	fmt.Printf("Mode: %s\n", mode)
	if state != nil && state.SupervisorPID > 0 {
		if runtime.SupervisorRunning {
			fmt.Printf("Supervisor: running (pid %d)\n", state.SupervisorPID)
		} else {
			fmt.Printf("Supervisor: stopped (last pid %d)\n", state.SupervisorPID)
		}
	}
	if state != nil && state.ChildPID > 0 {
		switch {
		case runtime.ChildRunning:
			fmt.Printf("Worker: running (pid %d)\n", state.ChildPID)
		default:
			fmt.Printf("Worker: last pid %d (%s)\n", state.ChildPID, state.Status)
		}
	}
	if state != nil {
		fmt.Printf("Daemon status: %s\n", state.Status)
		fmt.Printf("Restarts: %d\n", state.RestartCount)
		if state.LastExitCode != 0 || state.LastError != "" {
			fmt.Printf("Last exit: code=%d\n", state.LastExitCode)
			if state.LastError != "" {
				fmt.Printf("Last error: %s\n", state.LastError)
			}
		}
	}
	if health.Healthy {
		fmt.Printf("Health: %s\n", health.Status)
	} else if health.Error != "" {
		fmt.Printf("Health: unreachable (%s)\n", health.Error)
	} else {
		fmt.Println("Health: unreachable")
	}
	fmt.Printf("URL: %s\n", getServiceBaseURL())
	fmt.Printf("Logs: %s\n", paths.logFile)
}

func runGatewaySupervisor(cmd *cobra.Command, args []string) {
	if err := gatewaySupervisorMain(); err != nil {
		fmt.Fprintf(os.Stderr, "gateway supervisor: %v\n", err)
		os.Exit(1)
	}
}

func gatewaySupervisorMain() error {
	paths := gatewayDaemonFiles()
	if err := os.MkdirAll(paths.runDir, 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.logFile), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	runtime, err := readGatewayDaemonRuntime(paths)
	if err == nil && runtime.SupervisorRunning && runtime.State != nil && runtime.State.SupervisorPID != os.Getpid() {
		return fmt.Errorf("gateway daemon already running with pid %d", runtime.State.SupervisorPID)
	}

	now := time.Now().UTC()
	state := gatewayDaemonState{
		SupervisorPID: os.Getpid(),
		Status:        "starting",
		StartedAt:     now,
		ConfigFile:    cfgFile,
		Profile:       profile,
		DevMode:       devMode,
		UpdatedAt:     now,
	}
	if err := writeGatewayPIDFile(paths.pidFile, state.SupervisorPID); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}
	if err := writeGatewayDaemonState(paths, state); err != nil {
		_ = os.Remove(paths.pidFile)
		return fmt.Errorf("write state file: %w", err)
	}
	defer func() {
		_ = os.Remove(paths.pidFile)
	}()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	restartCount := 0
	for {
		childCmd, err := startGatewayWorkerProcess()
		if err != nil {
			state.Status = "error"
			state.ChildPID = 0
			state.LastError = err.Error()
			state.UpdatedAt = time.Now().UTC()
			_ = writeGatewayDaemonState(paths, state)
			return fmt.Errorf("start worker: %w", err)
		}

		state.ChildPID = childCmd.Process.Pid
		state.Status = "running"
		state.RestartCount = restartCount
		state.LastStartAt = time.Now().UTC()
		state.UpdatedAt = state.LastStartAt
		state.LastError = ""
		if err := writeGatewayDaemonState(paths, state); err != nil {
			_ = childCmd.Process.Kill()
			return fmt.Errorf("update running state: %w", err)
		}

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- childCmd.Wait()
		}()

		select {
		case sig := <-sigCh:
			_ = sig
			state.Status = "stopping"
			state.UpdatedAt = time.Now().UTC()
			_ = writeGatewayDaemonState(paths, state)
			_ = childCmd.Process.Signal(syscall.SIGTERM)
			waitErr := waitForGatewayWorkerStop(waitCh, childCmd.Process)
			state.ChildPID = 0
			state.LastExitAt = time.Now().UTC()
			state.LastExitCode = gatewayExitCode(waitErr)
			state.LastError = gatewayExitError(waitErr)
			state.Status = "stopped"
			state.UpdatedAt = state.LastExitAt
			_ = writeGatewayDaemonState(paths, state)
			return nil
		case waitErr := <-waitCh:
			state.ChildPID = 0
			state.LastExitAt = time.Now().UTC()
			state.LastExitCode = gatewayExitCode(waitErr)
			state.LastError = gatewayExitError(waitErr)
			restartCount++
			state.RestartCount = restartCount
			state.Status = "restarting"
			state.UpdatedAt = state.LastExitAt
			_ = writeGatewayDaemonState(paths, state)
		}

		select {
		case <-time.After(gatewayDaemonRestartDelay):
		case <-sigCh:
			state.Status = "stopped"
			state.UpdatedAt = time.Now().UTC()
			_ = writeGatewayDaemonState(paths, state)
			return nil
		}
	}
}

func startDetachedGatewaySupervisor(paths gatewayDaemonPaths) error {
	if err := os.MkdirAll(paths.runDir, 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.logFile), 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	logFile, err := os.OpenFile(paths.logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer devNull.Close()

	cmd := exec.Command(exePath, gatewaySupervisorArgs()...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch supervisor: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

func startGatewayWorkerProcess() (*exec.Cmd, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve executable: %w", err)
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer devNull.Close()

	cmd := exec.Command(exePath, gatewayWorkerArgs()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = devNull
	cmd.Env = append(os.Environ(), "BLUE_GATEWAY_SUPERVISOR=1")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func waitForGatewayWorkerStop(waitCh <-chan error, proc *os.Process) error {
	select {
	case err := <-waitCh:
		return err
	case <-time.After(gatewayDaemonStopTimeout):
		_ = proc.Kill()
		return <-waitCh
	}
}

func stopGatewayManagedInstance(paths gatewayDaemonPaths) (bool, error) {
	runtime, err := readGatewayDaemonRuntime(paths)
	if err != nil {
		return false, err
	}

	if runtime.SupervisorRunning && runtime.State != nil {
		if err := signalGatewayProcess(runtime.State.SupervisorPID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return false, fmt.Errorf("signal supervisor: %w", err)
		}
		if err := waitForGatewaySupervisorStop(paths, gatewayDaemonStopTimeout); err != nil {
			if runtime.State.ChildPID > 0 {
				_ = signalGatewayProcess(runtime.State.ChildPID, syscall.SIGKILL)
			}
			return false, err
		}
		return true, nil
	}

	if runtime.State != nil && runtime.State.ChildPID > 0 && runtime.ChildRunning {
		if err := signalGatewayProcess(runtime.State.ChildPID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
			return false, fmt.Errorf("signal orphaned worker: %w", err)
		}
		return true, nil
	}

	health := queryGatewayHealth(2 * time.Second)
	if health.Healthy {
		if err := requestGatewayShutdown(10 * time.Second); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, errGatewayNotRunning
}

func waitForGatewaySupervisorPID(paths gatewayDaemonPaths, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pid, err := readGatewayPIDFile(paths.pidFile)
		if err == nil && pid > 0 && gatewayProcessAlive(pid) {
			return pid, nil
		}
		time.Sleep(gatewayDaemonPollInterval)
	}
	return 0, fmt.Errorf("pid file %s was not created", paths.pidFile)
}

func waitForGatewaySupervisorStop(paths gatewayDaemonPaths, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		runtime, err := readGatewayDaemonRuntime(paths)
		if err == nil && !runtime.SupervisorRunning {
			return nil
		}
		time.Sleep(gatewayDaemonPollInterval)
	}
	return fmt.Errorf("gateway daemon did not stop within %s", timeout)
}

func requestGatewayShutdown(timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequest(http.MethodPost, getServiceAPIBaseURL("/api/v1/shutdown"), nil)
	if err != nil {
		return fmt.Errorf("build shutdown request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request shutdown: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("shutdown request returned %s", resp.Status)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !queryGatewayHealth(time.Second).Healthy {
			return nil
		}
		time.Sleep(gatewayDaemonPollInterval)
	}
	return fmt.Errorf("gateway still reported healthy after shutdown request")
}

func queryGatewayHealth(timeout time.Duration) gatewayHealthStatus {
	status := gatewayHealthStatus{
		URL: getServiceBaseURL(),
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(status.URL + "/health")
	if err != nil {
		status.Status = "unreachable"
		status.Error = err.Error()
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		status.Status = resp.Status
		status.Error = resp.Status
		return status
	}

	var payload struct {
		Status  string `json:"status"`
		Service string `json:"service"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		status.Status = "invalid_response"
		status.Error = err.Error()
		return status
	}

	status.Status = payload.Status
	status.Service = payload.Service
	status.Healthy = strings.EqualFold(payload.Status, "ok") && payload.Service == "zimaos-blue"
	if !status.Healthy && status.Error == "" {
		status.Error = "health response did not match zimaos-blue"
	}
	return status
}

func gatewayWorkerArgs() []string {
	args := append([]string{}, gatewayGlobalArgs()...)
	return append(args, "gateway", "run")
}

func gatewaySupervisorArgs() []string {
	args := append([]string{}, gatewayGlobalArgs()...)
	return append(args, "gateway", "supervise")
}

func gatewayGlobalArgs() []string {
	var args []string
	if cfgFile != "" {
		args = append(args, "--config", cfgFile)
	}
	if devMode {
		args = append(args, "--dev")
	}
	if profile != "" {
		args = append(args, "--profile", profile)
	}
	if verbose {
		args = append(args, "--verbose")
	}
	if noColor {
		args = append(args, "--no-color")
	}
	return args
}

func gatewayDaemonFiles() gatewayDaemonPaths {
	runDir := filepath.Join(getConfigDir(), "run")
	return gatewayDaemonPaths{
		runDir:    runDir,
		pidFile:   filepath.Join(runDir, "gateway-daemon.pid"),
		stateFile: filepath.Join(runDir, "gateway-daemon.json"),
		logFile:   filepath.Join(getLogsDir(), "blue.log"),
	}
}

func readGatewayDaemonRuntime(paths gatewayDaemonPaths) (gatewayDaemonRuntime, error) {
	var runtime gatewayDaemonRuntime

	state, err := readGatewayDaemonState(paths)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return runtime, err
	}
	runtime.State = state

	pid, err := readGatewayPIDFile(paths.pidFile)
	switch {
	case err == nil:
		runtime.SupervisorRunning = gatewayProcessAlive(pid)
		if runtime.State == nil {
			runtime.State = &gatewayDaemonState{SupervisorPID: pid}
		} else if runtime.State.SupervisorPID == 0 {
			runtime.State.SupervisorPID = pid
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return runtime, err
	}

	if runtime.State != nil && runtime.State.ChildPID > 0 {
		runtime.ChildRunning = gatewayProcessAlive(runtime.State.ChildPID)
	}

	return runtime, nil
}

func readGatewayDaemonState(paths gatewayDaemonPaths) (*gatewayDaemonState, error) {
	data, err := os.ReadFile(paths.stateFile)
	if err != nil {
		return nil, err
	}

	var state gatewayDaemonState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode state file: %w", err)
	}
	return &state, nil
}

func writeGatewayDaemonState(paths gatewayDaemonPaths, state gatewayDaemonState) error {
	if err := os.MkdirAll(filepath.Dir(paths.stateFile), 0o755); err != nil {
		return err
	}
	state.UpdatedAt = time.Now().UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := paths.stateFile + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmpPath, paths.stateFile)
}

func readGatewayPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse pid file %s: %w", path, err)
	}
	return pid, nil
}

func writeGatewayPIDFile(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

func gatewayProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func signalGatewayProcess(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return syscall.ESRCH
	}
	return syscall.Kill(pid, sig)
}

func gatewayExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return -1
}

func gatewayExitError(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func printGatewayStartResult(message string, pid int, logFile string) {
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success":        true,
			"message":        message,
			"mode":           "daemon",
			"supervisor_pid": pid,
			"log_file":       logFile,
			"url":            getServiceBaseURL(),
		})
		return
	}

	fmt.Printf("%s (pid %d)\n", message, pid)
	fmt.Printf("Logs: %s\n", logFile)
	fmt.Printf("URL:  %s\n", getServiceBaseURL())
}

func printGatewayError(message string, err error) {
	if jsonOutput {
		payload := map[string]interface{}{
			"success": false,
			"error":   message,
		}
		if err != nil {
			payload["details"] = err.Error()
		}
		printJSON(payload)
		return
	}

	fmt.Println(message)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}
