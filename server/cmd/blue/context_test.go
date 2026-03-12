package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
)

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
