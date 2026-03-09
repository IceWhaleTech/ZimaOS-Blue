package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	cronpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func setupCronCLIServer(t *testing.T) (*cronpkg.Service, string) {
	t.Helper()

	svc := cronpkg.NewService(cronpkg.DefaultConfig(), zap.NewNop())
	svc.RegisterBuiltinHandlers()
	svc.RegisterHandler("test-handler", func(ctx context.Context, job *cronpkg.Job) (interface{}, error) {
		return map[string]string{"job_id": job.ID}, nil
	})
	if err := svc.Start(); err != nil {
		t.Fatalf("failed to start cron service: %v", err)
	}

	e := echo.New()
	h := cronpkg.NewHandler(svc, zap.NewNop())
	h.RegisterRoutes(e.Group("/api"))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: e}
	go func() {
		_ = server.Serve(listener)
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		_ = svc.Stop(ctx)
	})

	return svc, host + ":" + port
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()

	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func TestResolveServerPortPrefersEnv(t *testing.T) {
	t.Setenv("BLUE_SERVER_PORT", "19090")
	oldCfg := cfgFile
	cfgFile = ""
	defer func() { cfgFile = oldCfg }()

	if got := resolveServerPort(); got != 19090 {
		t.Fatalf("resolveServerPort()=%d, want 19090", got)
	}
}

func TestCronCLIAddListStatusAndRuns(t *testing.T) {
	_, addr := setupCronCLIServer(t)
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("atoi: %v", err)
	}

	t.Setenv("BLUE_SERVER_HOST", host)
	t.Setenv("BLUE_SERVER_PORT", strconv.Itoa(port))

	oldJSON, oldNoColor, oldVerbose, oldDev := jsonOutput, noColor, verbose, devMode
	jsonOutput, noColor, verbose, devMode = true, true, false, false
	defer func() {
		jsonOutput, noColor, verbose, devMode = oldJSON, oldNoColor, oldVerbose, oldDev
	}()

	cronName = "cli-test-job"
	cronSchedule = "*/5 * * * *"
	cronHandler = "test-handler"
	cronPayload = ""

	addOut := captureStdout(t, func() { runCronAdd(nil, nil) })
	var addResp struct {
		Success bool    `json:"success"`
		Job     CronJob `json:"job"`
	}
	if err := json.Unmarshal([]byte(addOut), &addResp); err != nil {
		t.Fatalf("decode add response: %v\n%s", err, addOut)
	}
	if !addResp.Success {
		t.Fatalf("expected add success, got: %s", addOut)
	}
	if addResp.Job.Name != "cli-test-job" {
		t.Fatalf("unexpected job name: %s", addResp.Job.Name)
	}
	if !strings.HasPrefix(addResp.Job.Schedule, "0 ") {
		t.Fatalf("expected normalized schedule, got %s", addResp.Job.Schedule)
	}

	statusOut := captureStdout(t, func() { runCronStatus(nil, nil) })
	var statusResp struct {
		Running    bool `json:"running"`
		JobCount   int  `json:"job_count"`
		ActiveJobs int  `json:"active_jobs"`
	}
	if err := json.Unmarshal([]byte(statusOut), &statusResp); err != nil {
		t.Fatalf("decode status response: %v\n%s", err, statusOut)
	}
	if !statusResp.Running || statusResp.JobCount != 1 || statusResp.ActiveJobs != 1 {
		t.Fatalf("unexpected status response: %+v", statusResp)
	}

	listOut := captureStdout(t, func() { runCronList(nil, nil) })
	var listResp struct {
		Jobs []CronJob `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(listOut), &listResp); err != nil {
		t.Fatalf("decode list response: %v\n%s", err, listOut)
	}
	if len(listResp.Jobs) != 1 || listResp.Jobs[0].ID != addResp.Job.ID {
		t.Fatalf("unexpected list response: %+v", listResp)
	}

	runOut := captureStdout(t, func() { runCronRun(nil, []string{addResp.Job.ID}) })
	var runResp struct {
		Success   bool   `json:"success"`
		ID        string `json:"id"`
		Triggered bool   `json:"triggered"`
	}
	if err := json.Unmarshal([]byte(runOut), &runResp); err != nil {
		t.Fatalf("decode run response: %v\n%s", err, runOut)
	}
	if !runResp.Success || !runResp.Triggered || runResp.ID != addResp.Job.ID {
		t.Fatalf("unexpected run response: %+v", runResp)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runsOut := captureStdout(t, func() { runCronRuns(nil, []string{addResp.Job.ID}) })
		var runsResp struct {
			Executions []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"executions"`
		}
		if err := json.Unmarshal([]byte(runsOut), &runsResp); err != nil {
			t.Fatalf("decode runs response: %v\n%s", err, runsOut)
		}
		if len(runsResp.Executions) == 1 && runsResp.Executions[0].Status != "running" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("execution history did not show completed run in time")
}
