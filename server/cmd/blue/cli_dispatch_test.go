package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
)

type cliDispatchExitPanic struct {
	code int
}

func TestCLIDispatch_ContextSearchRoutesThroughIPC(t *testing.T) {
	oldRoundTrip := ipcRoundTripFunc
	oldIPCExit := ipcExit
	oldCLIExit := cliDispatchExit
	oldJSONOutput := jsonOutput
	defer func() {
		ipcRoundTripFunc = oldRoundTrip
		ipcExit = oldIPCExit
		cliDispatchExit = oldCLIExit
		jsonOutput = oldJSONOutput
	}()

	var capturedReq *sockipc.Request
	ipcRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		capturedReq = req
		return sockipc.OkResponse(map[string]string{
			"__stdout":    "ipc ok\n",
			"__exit_code": "0",
		}), nil
	}
	ipcExit = func(code int) { panic(cliDispatchExitPanic{code: code}) }
	cliDispatchExit = func(code int) { panic(cliDispatchExitPanic{code: code}) }

	handled, exitCode, stdout := runCLIDispatchForTest([]string{"context", "search", "responses", "tools"})
	if !handled {
		t.Fatal("expected cliDispatch to handle context search")
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout=%q)", exitCode, stdout)
	}
	if capturedReq == nil {
		t.Fatal("expected IPC request to be issued")
	}
	if capturedReq.Cmd != "context.search" {
		t.Fatalf("cmd = %q, want %q", capturedReq.Cmd, "context.search")
	}
	if got := capturedReq.Params["query"]; got != "responses tools" {
		t.Fatalf("query = %q, want %q", got, "responses tools")
	}
	if !strings.Contains(stdout, "ipc ok") {
		t.Fatalf("stdout = %q, want IPC output", stdout)
	}
}

func TestCLIDispatch_ContextSearchRequiresRunningServiceWhenIPCUnavailable(t *testing.T) {
	oldRoundTrip := ipcRoundTripFunc
	oldIPCExit := ipcExit
	oldCLIExit := cliDispatchExit
	oldJSONOutput := jsonOutput
	defer func() {
		ipcRoundTripFunc = oldRoundTrip
		ipcExit = oldIPCExit
		cliDispatchExit = oldCLIExit
		jsonOutput = oldJSONOutput
	}()

	ipcRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		return nil, fmt.Errorf("dial unix /tmp/missing-blue.sock: connect: no such file or directory")
	}
	ipcExit = func(code int) { panic(cliDispatchExitPanic{code: code}) }
	cliDispatchExit = func(code int) { panic(cliDispatchExitPanic{code: code}) }

	handled, exitCode, stdout := runCLIDispatchForTest([]string{"context", "search", "responses", "tools"})
	if !handled {
		t.Fatal("expected cliDispatch to handle missing-service context search")
	}
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1 (stdout=%q)", exitCode, stdout)
	}
	if !strings.Contains(stdout, "running Blue service is required for context.search") {
		t.Fatalf("stdout = %q, want missing-service IPC error", stdout)
	}
}

func TestCLIDispatch_BrowserNavigateRoutesThroughIPC(t *testing.T) {
	oldRoundTrip := ipcRoundTripFunc
	oldIPCExit := ipcExit
	oldCLIExit := cliDispatchExit
	oldJSONOutput := jsonOutput
	defer func() {
		ipcRoundTripFunc = oldRoundTrip
		ipcExit = oldIPCExit
		cliDispatchExit = oldCLIExit
		jsonOutput = oldJSONOutput
	}()

	var capturedReq *sockipc.Request
	ipcRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		capturedReq = req
		return sockipc.OkResponse(map[string]string{
			"__stdout":    "browser ok\n",
			"__exit_code": "0",
		}), nil
	}
	ipcExit = func(code int) { panic(cliDispatchExitPanic{code: code}) }
	cliDispatchExit = func(code int) { panic(cliDispatchExitPanic{code: code}) }

	handled, exitCode, stdout := runCLIDispatchForTest([]string{"browser", "navigate", "url=https://example.com"})
	if !handled {
		t.Fatal("expected cliDispatch to handle browser navigate")
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout=%q)", exitCode, stdout)
	}
	if capturedReq == nil {
		t.Fatal("expected IPC request to be issued")
	}
	if capturedReq.Cmd != "browser.navigate" {
		t.Fatalf("cmd = %q, want %q", capturedReq.Cmd, "browser.navigate")
	}
	if got := capturedReq.Params["url"]; got != "https://example.com" {
		t.Fatalf("url = %q, want %q", got, "https://example.com")
	}
	if !strings.Contains(stdout, "browser ok") {
		t.Fatalf("stdout = %q, want IPC output", stdout)
	}
}

func TestCLIDispatch_SkillAliasFallsThroughToCobra(t *testing.T) {
	oldRoundTrip := ipcRoundTripFunc
	oldIPCExit := ipcExit
	oldCLIExit := cliDispatchExit
	oldJSONOutput := jsonOutput
	defer func() {
		ipcRoundTripFunc = oldRoundTrip
		ipcExit = oldIPCExit
		cliDispatchExit = oldCLIExit
		jsonOutput = oldJSONOutput
	}()

	ipcRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		t.Fatalf("unexpected IPC request: %+v", req)
		return nil, nil
	}
	ipcExit = func(code int) { panic(cliDispatchExitPanic{code: code}) }
	cliDispatchExit = func(code int) { panic(cliDispatchExitPanic{code: code}) }

	handled, exitCode, stdout := runCLIDispatchForTest([]string{"skill", "install", "humanizer"})
	if handled {
		t.Fatalf("expected cliDispatch to fall through to cobra, handled=true exitCode=%d stdout=%q", exitCode, stdout)
	}
}

func runCLIDispatchForTest(args []string) (handled bool, exitCode int, stdout string) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		_ = r.Close()
		stdout = buf.String()
		if rec := recover(); rec != nil {
			if exit, ok := rec.(cliDispatchExitPanic); ok {
				handled = true
				exitCode = exit.code
				return
			}
			panic(rec)
		}
	}()

	handled = cliDispatch(args)
	return handled, exitCode, stdout
}
