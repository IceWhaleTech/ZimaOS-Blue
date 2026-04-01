package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
)

type contextCommandExitPanic struct {
	code int
}

func resetContextCLIState(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	oldCfgFile, oldProfile := cfgFile, profile
	oldDevMode, oldJSONOutput := devMode, jsonOutput
	oldContextLang, oldContextVersion := contextLang, contextVersion
	oldContextFile, oldContextTenant := contextFile, contextTenant
	oldContextUser, oldContextClear := contextUser, contextClear
	oldContextList, oldContextFull := contextList, contextFull
	t.Cleanup(func() {
		cfgFile, profile = oldCfgFile, oldProfile
		devMode, jsonOutput = oldDevMode, oldJSONOutput
		contextLang, contextVersion = oldContextLang, oldContextVersion
		contextFile, contextTenant = oldContextFile, oldContextTenant
		contextUser, contextClear = oldContextUser, oldContextClear
		contextList, contextFull = oldContextList, oldContextFull
	})

	cfgFile = ""
	profile = ""
	devMode = false
	jsonOutput = true
	contextLang = ""
	contextVersion = ""
	contextFile = ""
	contextTenant = ""
	contextUser = ""
	contextClear = false
	contextList = false
	contextFull = false
	return home
}

func writeContextTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestOpenLocalContextRuntimeReleasesEmbeddedPacks(t *testing.T) {
	home := resetContextCLIState(t)

	rt, err := openLocalContextRuntime()
	if err != nil {
		t.Fatalf("openLocalContextRuntime() error = %v", err)
	}
	defer closeLocalContextRuntime(rt)

	packPath := filepath.Join(home, ".zimaos-blue", "data", "workspace", ".blue", "context", "openai", "docs", "responses-api", "DOC.md")
	if _, err := os.Stat(packPath); err != nil {
		t.Fatalf("expected embedded context pack at %s: %v", packPath, err)
	}

	results, err := rt.registry.Search(context.Background(), contextpack.SearchOptions{Query: "responses tools", Limit: 5})
	if err != nil {
		t.Fatalf("registry.Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one embedded context pack search result")
	}
	if results[0].Entry.ID != "openai/responses-api" {
		t.Fatalf("top result id = %s, want openai/responses-api", results[0].Entry.ID)
	}
}

func TestRunContextSearchReturnsEmbeddedResults(t *testing.T) {
	resetContextCLIState(t)

	out := captureStdout(t, func() {
		runContextSearch(nil, []string{"responses", "tools"})
	})

	var resp struct {
		Query   string `json:"query"`
		Results []struct {
			Entry struct {
				ID string `json:"id"`
			} `json:"entry"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode context search response: %v\n%s", err, out)
	}
	if resp.Query != "responses tools" {
		t.Fatalf("query = %q, want %q", resp.Query, "responses tools")
	}
	if len(resp.Results) == 0 {
		t.Fatal("expected embedded search results")
	}

	var found bool
	for _, result := range resp.Results {
		if result.Entry.ID == "openai/responses-api" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected openai/responses-api in search results: %+v", resp.Results)
	}
}

func TestRunContextGetFullIncludesReferencesAndAnnotations(t *testing.T) {
	resetContextCLIState(t)

	annotateOut := captureStdout(t, func() {
		runContextAnnotate(nil, []string{"openai/responses-api", "Prefer compact Responses API tool outputs."})
	})
	if annotateOut == "" {
		t.Fatal("expected annotate command to emit json output")
	}

	contextFull = true
	out := captureStdout(t, func() {
		runContextGet(nil, []string{"openai/responses-api"})
	})

	var resp struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
		Annotations []struct {
			Note string `json:"note"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode context get response: %v\n%s", err, out)
	}
	if len(resp.Files) < 2 {
		t.Fatalf("expected full fetch to include primary file and reference, got %d", len(resp.Files))
	}

	var sawReference bool
	for _, file := range resp.Files {
		if file.Path == "references/tools.md" {
			sawReference = true
			break
		}
	}
	if !sawReference {
		t.Fatalf("expected references/tools.md in full fetch, got %+v", resp.Files)
	}
	if len(resp.Annotations) != 1 || resp.Annotations[0].Note != "Prefer compact Responses API tool outputs." {
		t.Fatalf("unexpected annotations: %+v", resp.Annotations)
	}
}

func TestRunContextImportSearchAndValidateWorkspaceRegistry(t *testing.T) {
	home := resetContextCLIState(t)

	importRoot := filepath.Join(t.TempDir(), "ops-team")
	writeContextTestFile(t, filepath.Join(importRoot, "docs", "release-playbook", "DOC.md"), `---
id: acme/release-playbook
type: doc
description: Deployment rollback checklist
source_trust: local
tags: [deploy, rollback]
references: [references/review.md]
---

Check rollback gates, tenant isolation, and on-call handoff before production deploys.`)
	writeContextTestFile(t, filepath.Join(importRoot, "docs", "release-playbook", "references", "review.md"), "Review rollback comms before proceeding.")

	importOut := captureStdout(t, func() {
		runContextImport(nil, []string{importRoot})
	})
	var importResp struct {
		Imported bool                          `json:"imported"`
		Path     string                        `json:"path"`
		Issues   []contextpack.ValidationIssue `json:"issues"`
	}
	if err := json.Unmarshal([]byte(importOut), &importResp); err != nil {
		t.Fatalf("decode context import response: %v\n%s", err, importOut)
	}
	if !importResp.Imported {
		t.Fatalf("expected import success, got: %s", importOut)
	}
	if importResp.Path != importRoot {
		t.Fatalf("import path = %q, want %q", importResp.Path, importRoot)
	}
	if len(importResp.Issues) != 0 {
		t.Fatalf("expected no validation issues on import, got %+v", importResp.Issues)
	}

	searchOut := captureStdout(t, func() {
		runContextSearch(nil, []string{"tenant", "isolation", "handoff"})
	})
	var searchResp struct {
		Results []struct {
			Entry struct {
				ID string `json:"id"`
			} `json:"entry"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(searchOut), &searchResp); err != nil {
		t.Fatalf("decode imported search response: %v\n%s", err, searchOut)
	}
	var foundImported bool
	for _, result := range searchResp.Results {
		if result.Entry.ID == "acme/release-playbook" {
			foundImported = true
			break
		}
	}
	if !foundImported {
		t.Fatalf("expected imported pack in search results: %+v", searchResp.Results)
	}

	validateOut := captureStdout(t, func() {
		runContextValidate(nil, nil)
	})
	var validateResp struct {
		Root   string                        `json:"root"`
		Issues []contextpack.ValidationIssue `json:"issues"`
	}
	if err := json.Unmarshal([]byte(validateOut), &validateResp); err != nil {
		t.Fatalf("decode workspace validate response: %v\n%s", err, validateOut)
	}
	wantRoot := filepath.Join(home, ".zimaos-blue", "data", "workspace", ".blue", "context")
	if validateResp.Root != wantRoot {
		t.Fatalf("validate root = %q, want %q", validateResp.Root, wantRoot)
	}
	if len(validateResp.Issues) != 0 {
		t.Fatalf("expected workspace registry to validate cleanly, got %+v", validateResp.Issues)
	}
}

func TestRunContextValidateReportsCandidateIssues(t *testing.T) {
	resetContextCLIState(t)

	candidateRoot := t.TempDir()
	writeContextTestFile(t, filepath.Join(candidateRoot, "acme", "docs", "broken-pack", "DOC.md"), `---
id: acme/broken-pack
type: doc
references: [references/missing.md]
---

Broken pack for validation testing.`)

	out := captureStdout(t, func() {
		runContextValidate(nil, []string{candidateRoot})
	})
	var resp struct {
		Root   string                        `json:"root"`
		Issues []contextpack.ValidationIssue `json:"issues"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("decode candidate validate response: %v\n%s", err, out)
	}
	if resp.Root != candidateRoot {
		t.Fatalf("validate root = %q, want %q", resp.Root, candidateRoot)
	}
	if len(resp.Issues) == 0 {
		t.Fatal("expected validation issues for candidate directory")
	}

	var foundMissingReference bool
	for _, issue := range resp.Issues {
		if issue.Message == "frontmatter reference does not exist" && strings.HasSuffix(filepath.ToSlash(issue.Path), "/references/missing.md") {
			foundMissingReference = true
			break
		}
	}
	if !foundMissingReference {
		t.Fatalf("expected missing reference validation issue, got %+v", resp.Issues)
	}
}

func TestExecuteContextCLIIPCAction_UsesSharedRuntimeWithoutClosingStore(t *testing.T) {
	resetContextCLIState(t)

	rt, err := openLocalContextRuntime()
	if err != nil {
		t.Fatalf("openLocalContextRuntime() error = %v", err)
	}
	defer closeLocalContextRuntime(rt)

	stdout, exitCode, err := executeContextCLIIPCAction(&localContextRuntime{
		workspace: rt.workspace,
		registry:  rt.registry,
		store:     rt.store,
	}, "annotate", map[string]string{
		"id":          "openai/responses-api",
		"note":        "Persist over IPC",
		"__blue_json": "true",
	})
	if err != nil {
		t.Fatalf("executeContextCLIIPCAction() error = %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0; stdout=%s", exitCode, stdout)
	}

	var resp struct {
		Saved   bool   `json:"saved"`
		EntryID string `json:"entry_id"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("decode ipc annotate response: %v\n%s", err, stdout)
	}
	if !resp.Saved || resp.EntryID != "openai/responses-api" {
		t.Fatalf("unexpected annotate response: %+v", resp)
	}

	anns, err := rt.store.List(context.Background(), contextpack.AnnotationFilter{EntryID: "openai/responses-api"})
	if err != nil {
		t.Fatalf("store.List() error = %v", err)
	}
	if len(anns) != 1 || anns[0].Note != "Persist over IPC" {
		t.Fatalf("unexpected annotations after IPC action: %+v", anns)
	}
}

func TestContextCommands_UseIPCWrappers(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{name: "search", got: contextSearchCmd.Run, want: runContextSearchCommand},
		{name: "get", got: contextGetCmd.Run, want: runContextGetCommand},
		{name: "annotate", got: contextAnnotateCmd.Run, want: runContextAnnotateCommand},
		{name: "import", got: contextImportCmd.Run, want: runContextImportCommand},
		{name: "validate", got: contextValidateCmd.Run, want: runContextValidateCommand},
	} {
		if tc.got == nil {
			t.Fatalf("%s command Run is nil", tc.name)
		}
		if reflect.ValueOf(tc.got).Pointer() != reflect.ValueOf(tc.want).Pointer() {
			t.Fatalf("%s command Run does not use IPC wrapper", tc.name)
		}
	}
}

func TestRunContextSearchCommand_UsesIPCAndSkipsLocalRuntime(t *testing.T) {
	resetContextCLIState(t)

	oldRoundTrip := ipcRoundTripFunc
	oldIPCExit := ipcExit
	oldOpener := contextRuntimeOpener
	defer func() {
		ipcRoundTripFunc = oldRoundTrip
		ipcExit = oldIPCExit
		contextRuntimeOpener = oldOpener
	}()

	contextRuntimeOpener = func() (*localContextRuntime, error) {
		return nil, fmt.Errorf("context runtime opener should not be used from cobra command")
	}

	var capturedReq *sockipc.Request
	ipcRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		capturedReq = req
		return sockipc.OkResponse(map[string]string{
			"__stdout":    "context ipc ok\n",
			"__exit_code": "0",
		}), nil
	}
	ipcExit = func(code int) { panic(contextCommandExitPanic{code: code}) }

	exitCode, stdout := runContextCommandForTest(func() {
		runContextSearchCommand(nil, []string{"responses", "tools"})
	})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout=%q)", exitCode, stdout)
	}
	if capturedReq == nil {
		t.Fatal("expected IPC request to be sent")
	}
	if capturedReq.Cmd != "context.search" {
		t.Fatalf("cmd = %q, want %q", capturedReq.Cmd, "context.search")
	}
	if got := capturedReq.Params["query"]; got != "responses tools" {
		t.Fatalf("query = %q, want %q", got, "responses tools")
	}
	if !strings.Contains(stdout, "context ipc ok") {
		t.Fatalf("stdout = %q, want IPC output", stdout)
	}
}

func TestRunContextAnnotateCommand_UsesIPCWithScopeFlags(t *testing.T) {
	resetContextCLIState(t)
	contextLang = "en"
	contextVersion = "v2"
	contextFile = "DOC.md"
	contextTenant = "tenant-a"
	contextUser = "user-a"

	oldRoundTrip := ipcRoundTripFunc
	oldIPCExit := ipcExit
	oldOpener := contextRuntimeOpener
	defer func() {
		ipcRoundTripFunc = oldRoundTrip
		ipcExit = oldIPCExit
		contextRuntimeOpener = oldOpener
	}()

	contextRuntimeOpener = func() (*localContextRuntime, error) {
		return nil, fmt.Errorf("context runtime opener should not be used from cobra command")
	}

	var capturedReq *sockipc.Request
	ipcRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		capturedReq = req
		return sockipc.OkResponse(map[string]string{
			"__stdout":    "annotate ipc ok\n",
			"__exit_code": "0",
		}), nil
	}
	ipcExit = func(code int) { panic(contextCommandExitPanic{code: code}) }

	exitCode, stdout := runContextCommandForTest(func() {
		runContextAnnotateCommand(nil, []string{"openai/responses-api", "Persist over IPC"})
	})
	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0 (stdout=%q)", exitCode, stdout)
	}
	if capturedReq == nil {
		t.Fatal("expected IPC request to be sent")
	}
	if capturedReq.Cmd != "context.annotate" {
		t.Fatalf("cmd = %q, want %q", capturedReq.Cmd, "context.annotate")
	}
	for key, want := range map[string]string{
		"id":      "openai/responses-api",
		"note":    "Persist over IPC",
		"lang":    "en",
		"version": "v2",
		"file":    "DOC.md",
		"tenant":  "tenant-a",
		"user":    "user-a",
	} {
		if got := capturedReq.Params[key]; got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestRunContextSearchCommand_RequiresRunningServiceWhenIPCUnavailable(t *testing.T) {
	resetContextCLIState(t)

	oldRoundTrip := ipcRoundTripFunc
	oldIPCExit := ipcExit
	oldOpener := contextRuntimeOpener
	defer func() {
		ipcRoundTripFunc = oldRoundTrip
		ipcExit = oldIPCExit
		contextRuntimeOpener = oldOpener
	}()

	contextRuntimeOpener = func() (*localContextRuntime, error) {
		return nil, fmt.Errorf("context runtime opener should not be used from cobra command")
	}
	ipcRoundTripFunc = func(req *sockipc.Request) (*sockipc.Response, error) {
		return nil, fmt.Errorf("dial unix /tmp/missing-blue.sock: connect: no such file or directory")
	}
	ipcExit = func(code int) { panic(contextCommandExitPanic{code: code}) }

	exitCode, stdout := runContextCommandForTest(func() {
		runContextSearchCommand(nil, []string{"responses", "tools"})
	})
	if exitCode != 1 {
		t.Fatalf("exitCode = %d, want 1 (stdout=%q)", exitCode, stdout)
	}
	if !strings.Contains(stdout, "running Blue service is required for context.search") {
		t.Fatalf("stdout = %q, want missing-service IPC error", stdout)
	}
}

func runContextCommandForTest(fn func()) (exitCode int, stdout string) {
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
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		stdout = buf.String()
		if rec := recover(); rec != nil {
			if exit, ok := rec.(contextCommandExitPanic); ok {
				exitCode = exit.code
				return
			}
			panic(rec)
		}
	}()

	fn()
	return 0, ""
}
