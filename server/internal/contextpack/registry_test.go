package contextpack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegistrySearchAndGet(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "openai", "docs", "responses-api", "DOC.md"), `---
id: openai/responses-api
type: doc
description: Responses API notes
source_trust: official
tags: [openai, responses, tools]
references:
  - references/tools.md
---

# Responses API

Tool calling and continuation guidance.`)
	writeTestFile(t, filepath.Join(root, "openai", "docs", "responses-api", "references", "tools.md"), "# Tools\n\nStable tool contracts.")

	registry := NewRegistry(root)
	registry.SetRefreshTTL(0)
	results, err := registry.Search(context.Background(), SearchOptions{Query: "responses tools", Limit: 5})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].Entry.ID != "openai/responses-api" {
		t.Fatalf("top result id = %q", results[0].Entry.ID)
	}

	fetch, err := registry.Get(context.Background(), "openai/responses-api", GetOptions{Full: true})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(fetch.Files) != 2 {
		t.Fatalf("len(fetch.Files) = %d, want 2", len(fetch.Files))
	}
	if fetch.Files[0].Path != "DOC.md" {
		t.Fatalf("primary path = %q, want DOC.md", fetch.Files[0].Path)
	}
}

func TestRegistrySelectsLanguageAndVersionVariant(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "blue", "docs", "sdk", "en", "v1", "DOC.md"), `---
id: blue/sdk
type: doc
description: SDK v1 notes
language: en
version: v1
---

SDK v1`)

	registry := NewRegistry(root)
	registry.SetRefreshTTL(0)
	fetch, err := registry.Get(context.Background(), "blue/sdk", GetOptions{Language: "en", Version: "v1"})
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if fetch.Variant.Language != "en" || fetch.Variant.Version != "v1" {
		t.Fatalf("variant = %+v", fetch.Variant)
	}
	if got := fetch.Files[0].Path; got != "en/v1/DOC.md" {
		t.Fatalf("path = %q, want en/v1/DOC.md", got)
	}
}

func TestRegistryReportsMissingReferenceIssue(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "blue", "docs", "ops", "DOC.md"), `---
id: blue/ops
type: doc
references:
  - references/missing.md
---

Ops guidance`)

	registry := NewRegistry(root)
	registry.SetRefreshTTL(0)
	issues, err := registry.Issues(context.Background())
	if err != nil {
		t.Fatalf("Issues() error = %v", err)
	}
	if len(issues) == 0 {
		t.Fatal("expected validation issues")
	}
	if !strings.Contains(issues[0].Message, "reference") {
		t.Fatalf("unexpected issue: %+v", issues[0])
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
