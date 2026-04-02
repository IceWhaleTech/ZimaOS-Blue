package skillmarket

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillstore"
)

func TestDiscoverFromHTMLCatalogFindsInstallableAndCatalogOnlySkills(t *testing.T) {
	rawSkill := `---
id: git-expert
name: Git Expert
version: 1.0.0
description: Git workflow helper
---
Help with rebase, review, and branch cleanup.
`

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/catalog", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html>
  <head><title>SkillHub Fixture</title><meta name="description" content="fixture catalog"></head>
  <body>
    <a href="/skills/git-expert">Git Expert</a>
    <a href="/skills/manual-only">Manual Only</a>
  </body>
</html>`))
	})
	mux.HandleFunc("/skills/git-expert", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html>
  <head><title>Git Expert</title><meta name="description" content="Installable git helper"></head>
  <body><a href="https://github.com/demo/git-expert">repo</a></body>
</html>`))
	})
	mux.HandleFunc("/skills/manual-only", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html>
  <head><title>Manual Only Skill</title><meta name="description" content="Directory listing only"></head>
  <body><p>Read the external instructions to install this skill manually.</p></body>
</html>`))
	})
	mux.HandleFunc("/github/repos/demo/git-expert", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"html_url":         "https://github.com/demo/git-expert",
			"default_branch":   "main",
			"updated_at":       "2026-03-01T00:00:00Z",
			"stargazers_count": 42,
		})
	})
	mux.HandleFunc("/github/repos/demo/git-expert/contents/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"name": "SKILL.md",
				"path": "SKILL.md",
				"type": "file",
				"url":  server.URL + "/github/blob/git-expert",
			},
		})
	})
	mux.HandleFunc("/github/blob/git-expert", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"encoding": "base64",
			"content":  base64.StdEncoding.EncodeToString([]byte(rawSkill)),
		})
	})

	tempDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	cfg := DefaultConfig(tempDir, filepath.Join(tempDir, "active"))
	cfg.CacheRoot = filepath.Join(tempDir, "cache")
	cfg.CuratedConfigPath = filepath.Join(tempDir, "missing-curations.yaml")
	cfg.CuratedConfigURLs = nil
	cfg.DiscoveryPageURLs = nil
	cfg.GitHubAPIBaseURL = server.URL + "/github"

	svc, err := NewService(db, Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(filepath.Join(tempDir, "active")),
		Scanner:      NewScanner(nil),
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	run := &CrawlRun{}
	source := Source{
		ID:          "skillhub-club",
		Type:        "html_catalog",
		BaseURL:     server.URL + "/catalog",
		DisplayName: "SkillHub Club",
		SourceGroup: "skillhub",
		Enabled:     true,
	}
	if err := svc.discoverFromHTMLCatalog(context.Background(), source, 0, &DiscoverResult{}, run); err != nil {
		t.Fatalf("discover html catalog: %v", err)
	}
	if run.Discovered != 3 {
		t.Fatalf("discovered = %d, want 3", run.Discovered)
	}

	result, err := svc.Search(context.Background(), SearchQuery{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	var foundInstallable bool
	var foundCatalogOnly bool
	for _, item := range result.Skills {
		switch {
		case strings.EqualFold(item.Skill.Name, "Git Expert"):
			foundInstallable = true
			if !item.Skill.Installable {
				t.Fatal("expected Git Expert to be installable")
			}
			if item.Skill.SourceGroup != "github" {
				t.Fatalf("source group = %q, want github", item.Skill.SourceGroup)
			}
			if item.Skill.InstallType != InstallTypeGitRepo {
				t.Fatalf("install type = %q, want %q", item.Skill.InstallType, InstallTypeGitRepo)
			}
		case strings.EqualFold(item.Skill.Name, "Manual Only Skill"):
			foundCatalogOnly = true
			if item.Skill.Installable {
				t.Fatal("expected Manual Only Skill to be catalog-only")
			}
			if item.Skill.InstallType != InstallTypeManualExternal {
				t.Fatalf("install type = %q, want %q", item.Skill.InstallType, InstallTypeManualExternal)
			}
			if item.Skill.SourceGroup != "skillhub" {
				t.Fatalf("source group = %q, want skillhub", item.Skill.SourceGroup)
			}
		}
	}

	if !foundInstallable {
		t.Fatal("expected installable GitHub-backed skill to be indexed")
	}
	if !foundCatalogOnly {
		t.Fatal("expected catalog-only directory entry to be indexed")
	}
}

func TestDiscoverFromHTMLCatalogUsesEmbeddedSkillPageContent(t *testing.T) {
	rawSkill := `---
name: file-search
description: Embedded file search skill
---

Use bash to inspect files quickly.
`
	encodedSkill := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
	).Replace(rawSkill)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/catalog", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`
<html>
  <head><title>SkillHub Fixture</title><meta name="description" content="fixture catalog"></head>
  <body>
    <a href="/skills/file-search">file-search</a>
  </body>
</html>`))
	})
	mux.HandleFunc("/skills/file-search", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fmt.Sprintf(`
<html>
  <head>
    <title>file-search - Claude Skill Details | SkillHub</title>
    <meta name="description" content="Embedded skill detail">
    <meta property="article:author" content="massgen">
  </head>
  <body>
    <script>self.__next_f.push([1,"36:[\"$\",\"$L3b\",null,{\"skillName\":\"file-search\",\"skillMdRaw\":\"$3c\",\"repoUrl\":\"https://github.com/massgen/MassGen\",\"skillPath\":\"massgen/skills/file-search\"}]"])</script>
    <script>self.__next_f.push([1,"3c:T10,\"%s\""])</script>
  </body>
</html>`, encodedSkill)))
	})

	tempDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	cfg := DefaultConfig(tempDir, filepath.Join(tempDir, "active"))
	cfg.CacheRoot = filepath.Join(tempDir, "cache")
	cfg.CuratedConfigPath = filepath.Join(tempDir, "missing-curations.yaml")
	cfg.CuratedConfigURLs = nil
	cfg.DiscoveryPageURLs = nil

	svc, err := NewService(db, Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(filepath.Join(tempDir, "active")),
		Scanner:      NewScanner(nil),
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	run := &CrawlRun{}
	source := Source{
		ID:          "skillhub-club",
		Type:        "html_catalog",
		BaseURL:     server.URL + "/catalog",
		DisplayName: "SkillHub Club",
		SourceGroup: "skillhub",
		Enabled:     true,
	}
	if err := svc.discoverFromHTMLCatalog(context.Background(), source, 0, &DiscoverResult{}, run); err != nil {
		t.Fatalf("discover html catalog: %v", err)
	}

	detail, err := svc.GetSkill(context.Background(), "skillhub-file-search")
	if err != nil {
		t.Fatalf("GetSkill(skillhub-file-search) error = %v", err)
	}
	if detail == nil {
		t.Fatal("expected embedded skill detail")
	}
	if !detail.Skill.Installable {
		t.Fatalf("expected embedded skill page to be installable, got skill=%+v version=%+v security=%+v", detail.Skill, detail.Version, detail.Security)
	}
	if detail.Skill.InstallType != InstallTypeRawSkill {
		t.Fatalf("install type = %q, want %q", detail.Skill.InstallType, InstallTypeRawSkill)
	}
	if detail.Skill.ArtifactKind != ArtifactKindOpenSource {
		t.Fatalf("artifact kind = %q, want %q", detail.Skill.ArtifactKind, ArtifactKindOpenSource)
	}
	if detail.Skill.SourceGroup != "skillhub" {
		t.Fatalf("source group = %q, want skillhub", detail.Skill.SourceGroup)
	}
	if detail.Skill.Author != "massgen" {
		t.Fatalf("author = %q, want massgen", detail.Skill.Author)
	}
	if detail.Skill.RepoURL != "https://github.com/massgen/MassGen" {
		t.Fatalf("repo url = %q, want GitHub repo", detail.Skill.RepoURL)
	}
	if detail.Version == nil || strings.TrimSpace(detail.Version.RawSkillMD) != strings.TrimSpace(rawSkill) {
		got := ""
		if detail.Version != nil {
			got = detail.Version.RawSkillMD
		}
		t.Fatalf("raw skill = %q, want embedded markdown", got)
	}
	if detail.Security == nil {
		t.Fatal("expected security report for embedded skill page")
	}
}

func TestDiscoverFromHTMLCatalogTraversesBeyondLegacyPageCap(t *testing.T) {
	const totalPages = 35

	pageHits := make(map[string]int)
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/skills/", func(w http.ResponseWriter, r *http.Request) {
		pageID := strings.TrimPrefix(r.URL.Path, "/skills/page-")
		pageHits[pageID]++
		index := 0
		if _, err := fmt.Sscanf(pageID, "%d", &index); err != nil || index < 1 || index > totalPages {
			http.NotFound(w, r)
			return
		}

		nextLink := ""
		if index < totalPages {
			nextLink = fmt.Sprintf(`<a href="/skills/page-%d">Next</a>`, index+1)
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`
<html>
  <head>
    <title>Catalog Skill %d</title>
    <meta name="description" content="Catalog-only skill page %d">
  </head>
  <body>
    <p>Catalog page %d</p>
    %s
  </body>
</html>`, index, index, index, nextLink)))
	})

	tempDir := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tempDir, "skillmarket.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	cfg := DefaultConfig(tempDir, filepath.Join(tempDir, "active"))
	cfg.CacheRoot = filepath.Join(tempDir, "cache")
	cfg.CuratedConfigPath = filepath.Join(tempDir, "missing-curations.yaml")
	cfg.CuratedConfigURLs = nil
	cfg.DiscoveryPageURLs = nil
	cfg.HTMLCatalogCrawlBatchPages = 3
	cfg.HTMLCatalogCrawlMaxPages = 40

	svc, err := NewService(db, Options{
		Config:       cfg,
		Registry:     skill.NewRegistry(),
		LocalScanner: skillstore.NewLocalSkillScanner(filepath.Join(tempDir, "active")),
		Scanner:      NewScanner(nil),
		HTTPClient:   server.Client(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	run := &CrawlRun{}
	source := Source{
		ID:          "skillhub-club",
		Type:        "html_catalog",
		BaseURL:     server.URL + "/skills/page-1",
		DisplayName: "SkillHub Club",
		SourceGroup: "skillhub",
		Enabled:     true,
	}
	if err := svc.discoverFromHTMLCatalog(context.Background(), source, 0, &DiscoverResult{}, run); err != nil {
		t.Fatalf("discover html catalog: %v", err)
	}
	if run.Discovered != totalPages {
		t.Fatalf("run.Discovered = %d, want %d", run.Discovered, totalPages)
	}
	if pageHits["35"] == 0 {
		t.Fatalf("expected crawler to reach page 35, hits=%v", pageHits)
	}

	detail, err := svc.GetSkill(context.Background(), "skillhub-catalog-skill-35")
	if err != nil {
		t.Fatalf("GetSkill(skillhub-catalog-skill-35) error = %v", err)
	}
	if detail == nil {
		t.Fatal("expected final catalog page to be indexed")
	}
	if detail.Skill.Installable {
		t.Fatalf("expected catalog-only page to remain non-installable, got %+v", detail.Skill)
	}
}
