package contextpack

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestResolverBuildsPromptWithAnnotationsAndReferenceMatch(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "openai", "docs", "responses-api", "DOC.md"), `---
id: openai/responses-api
type: doc
description: Responses API notes
source_trust: official
tags: [openai, responses, tools]
---

Primary responses guidance.`)
	writeTestFile(t, filepath.Join(root, "openai", "docs", "responses-api", "references", "tools.md"), `# Tools

Tool contract guidance.`)

	registry := NewRegistry(root)
	registry.SetRefreshTTL(0)
	store, err := NewAnnotationStore(filepath.Join(t.TempDir(), "annotations.db"))
	if err != nil {
		t.Fatalf("NewAnnotationStore() error = %v", err)
	}
	defer store.Close()
	if err := store.Upsert(context.Background(), Annotation{TenantID: "local", UserID: "user-1", EntryID: "openai/responses-api", Note: "Keep tool outputs compact."}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	resolver := NewResolver(registry, store, ResolverConfig{MaxFiles: 3, MaxTokens: 1200, SearchLimit: 5})
	ctx := context.Background()
	ctx = WithPromptQuery(ctx, "responses tools")
	ctx = WithSelectedSkill(ctx, "web_search")
	ctx = tools.WithLang(ctx, "en")
	ctx = tools.WithUserID(ctx, "user-1")

	prompt, selection, err := resolver.ResolvePrompt(ctx)
	if err != nil {
		t.Fatalf("ResolvePrompt() error = %v", err)
	}
	if selection == nil || len(selection.Files) == 0 {
		t.Fatal("expected non-empty selection")
	}
	if !strings.Contains(prompt, "<context_pack") {
		t.Fatalf("prompt missing context_pack block: %s", prompt)
	}
	if !strings.Contains(prompt, "Keep tool outputs compact") {
		t.Fatalf("prompt missing annotation text: %s", prompt)
	}
	if !strings.Contains(prompt, "<external-content") {
		t.Fatalf("prompt missing wrapped external content: %s", prompt)
	}
	if !selection.Files[0].Annotated {
		t.Fatalf("expected annotated selection: %+v", selection.Files[0])
	}
}
