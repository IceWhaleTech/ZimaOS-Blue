package skillmarket

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type catalogPage struct {
	URL         string
	Title       string
	Description string
	Author      string
	Text        string
	Links       []string
}

var githubBlobPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.+)$`)
var githubRepoPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
var githubRawPattern = regexp.MustCompile(`^https?://raw\.githubusercontent\.com/([^/]+)/([^/]+)/([^/]+)/(.+)$`)

func (s *Service) discoverFromHTMLCatalog(ctx context.Context, source Source, run *CrawlRun) error {
	root, err := s.fetchCatalogPage(ctx, source, source.BaseURL)
	if err != nil {
		return err
	}
	pages := []*catalogPage{root}
	seenPages := map[string]struct{}{root.URL: {}}
	for _, link := range root.Links {
		resolved, ok := resolveCatalogLink(root.URL, link)
		if !ok || !looksLikeCatalogPage(resolved, source.BaseURL) {
			continue
		}
		if _, ok := seenPages[resolved]; ok {
			continue
		}
		seenPages[resolved] = struct{}{}
		page, err := s.fetchCatalogPage(ctx, source, resolved)
		if err != nil {
			run.Failed++
			continue
		}
		pages = append(pages, page)
		if len(pages) >= 30 {
			break
		}
	}

	seenSeeds := make(map[string]struct{})
	for _, page := range pages {
		hadInstallable := false
		for _, link := range page.Links {
			resolved, ok := resolveCatalogLink(page.URL, link)
			if !ok {
				continue
			}
			seedKey := resolved
			if _, ok := seenSeeds[seedKey]; ok {
				continue
			}
			switch {
			case looksLikeGitHubRepoURL(resolved):
				seenSeeds[seedKey] = struct{}{}
				if err := s.discoverGitHubRepoSeed(ctx, repoOwner(resolved), repoName(resolved)); err != nil {
					run.Failed++
					continue
				}
				hadInstallable = true
				run.Discovered++
			case looksLikeSkillURL(resolved):
				seenSeeds[seedKey] = struct{}{}
				if err := s.discoverSkillURLSeed(ctx, resolved); err != nil {
					run.Failed++
					continue
				}
				hadInstallable = true
				run.Discovered++
			}
		}
		if hadInstallable {
			continue
		}
		if err := s.upsertCatalogOnlySkill(ctx, source, page); err != nil {
			run.Failed++
			continue
		}
		run.Discovered++
	}
	return nil
}

func (s *Service) discoverGitHubRepoSeed(ctx context.Context, owner, repo string) error {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" {
		return fmt.Errorf("invalid github seed")
	}
	metaURL := fmt.Sprintf("%s/repos/%s/%s", strings.TrimRight(s.cfg.GitHubAPIBaseURL, "/"), owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(s.cfg.GitHubToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github repo status: %d", resp.StatusCode)
	}
	var repoMeta struct {
		HTMLURL       string    `json:"html_url"`
		DefaultBranch string    `json:"default_branch"`
		UpdatedAt     time.Time `json:"updated_at"`
		Stargazers    int       `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoMeta); err != nil {
		return err
	}
	contents, err := s.fetchGitHubRepoSkill(ctx, owner, repo, repoMeta.DefaultBranch)
	if err != nil {
		return err
	}
	_, err = s.ingestSkillContent(ctx, ingestRequest{
		SourceID:       "curated-github-seed",
		SourceName:     "GitHub",
		SourceGroup:    "github",
		SourceType:     "github_repo",
		RepoURL:        repoMeta.HTMLURL,
		Homepage:       repoMeta.HTMLURL,
		DownloadURL:    rawGitHubBlobURL(owner, repo, repoMeta.DefaultBranch, contents.Path),
		SourceURL:      repoMeta.HTMLURL,
		SkillPath:      contents.Path,
		SkillContent:   contents.Raw,
		CommitHash:     contents.Commit,
		Stars:          repoMeta.Stargazers,
		LastUpdated:    repoMeta.UpdatedAt,
		DefaultSkillID: pathSkillID(repoMeta.HTMLURL, contents.Path, repo),
		Installable:    true,
		InstallType:    InstallTypeGitRepo,
		ArtifactKind:   ArtifactKindOpenSource,
	})
	return err
}

func (s *Service) discoverSkillURLSeed(ctx context.Context, rawURL string) error {
	content, skillPath, repoURL, downloadURL, installType, err := s.fetchSkillReference(ctx, rawURL)
	if err != nil {
		return err
	}
	_, err = s.ingestSkillContent(ctx, ingestRequest{
		SourceID:       "curated-skill-url",
		SourceName:     "External Skill",
		SourceGroup:    "external",
		SourceType:     "skill_url",
		RepoURL:        repoURL,
		Homepage:       rawURL,
		DownloadURL:    downloadURL,
		SourceURL:      rawURL,
		SkillPath:      skillPath,
		SkillContent:   content,
		DefaultSkillID: pathSkillID(repoURL, skillPath, filepath.Base(skillPath)),
		LastUpdated:    timeutil.NowTime(),
		Installable:    true,
		InstallType:    installType,
		ArtifactKind:   ArtifactKindOpenSource,
	})
	return err
}

func (s *Service) upsertCatalogOnlySkill(ctx context.Context, source Source, page *catalogPage) error {
	name := strings.TrimSpace(page.Title)
	if name == "" {
		name = normalizeSkillID(page.URL)
	}
	description := strings.TrimSpace(page.Description)
	if description == "" {
		description = summarizeText(page.Text)
	}
	raw := fmt.Sprintf("---\nid: %s\nname: %s\nversion: catalog\ndescription: %s\ncategory: general\n---\n\n# %s\n\n%s\n",
		normalizeSkillID(source.SourceGroup+"-"+name), escapeYAMLText(name), escapeYAMLText(description), name, description)
	_, err := s.ingestSkillContent(ctx, ingestRequest{
		SourceID:        source.ID,
		SourceName:      defaultString(source.DisplayName, source.ID),
		SourceGroup:     defaultString(source.SourceGroup, source.ID),
		SourceType:      source.Type,
		RepoURL:         page.URL,
		Homepage:        page.URL,
		DownloadURL:     "",
		SourceURL:       page.URL,
		SkillPath:       "SKILL.md",
		SkillContent:    raw,
		LastUpdated:     timeutil.NowTime(),
		DefaultSkillID:  normalizeSkillID(source.SourceGroup + "-" + name),
		ExplicitID:      normalizeSkillID(source.SourceGroup + "-" + name),
		ExplicitName:    name,
		ExplicitVersion: "catalog",
		Installable:     false,
		InstallType:     InstallTypeManualExternal,
		ArtifactKind:    ArtifactKindUnknown,
	})
	return err
}

func (s *Service) fetchCatalogPage(ctx context.Context, source Source, rawURL string) (*catalogPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ZimaOS-SkillMarket/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	for key, value := range source.Headers {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("catalog page status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	return &catalogPage{
		URL:         rawURL,
		Title:       strings.TrimSpace(extractHTMLTitle(doc)),
		Description: strings.TrimSpace(extractHTMLMeta(doc, "description", "og:description", "twitter:description")),
		Author:      strings.TrimSpace(extractHTMLMeta(doc, "author")),
		Text:        strings.TrimSpace(extractHTMLText(doc)),
		Links:       extractHTMLLinks(doc),
	}, nil
}

func (s *Service) fetchSkillReference(ctx context.Context, rawURL string) (content, skillPath, repoURL, downloadURL, installType string, err error) {
	if matches := githubBlobPattern.FindStringSubmatch(rawURL); len(matches) == 5 {
		downloadURL = rawGitHubBlobURL(matches[1], matches[2], matches[3], matches[4])
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		if err != nil {
			return "", "", "", "", "", err
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return "", "", "", "", "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", "", "", "", "", fmt.Errorf("github raw status: %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", "", "", "", "", err
		}
		return string(body), matches[4], fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2]), downloadURL, InstallTypeRawSkill, nil
	}
	if matches := githubRawPattern.FindStringSubmatch(rawURL); len(matches) == 5 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return "", "", "", "", "", err
		}
		resp, err := s.httpClient.Do(req)
		if err != nil {
			return "", "", "", "", "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", "", "", "", "", fmt.Errorf("github raw status: %d", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", "", "", "", "", err
		}
		return string(body), matches[4], fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2]), rawURL, InstallTypeRawSkill, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", "", "", "", "", err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", "", "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", "", "", "", fmt.Errorf("skill URL status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", "", "", err
	}
	return string(body), filepath.Base(rawURL), rawURL, rawURL, InstallTypeRawSkill, nil
}

func resolveCatalogLink(baseURL, href string) (string, bool) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", false
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	resolved := base.ResolveReference(ref)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return "", false
	}
	return resolved.String(), true
}

func looksLikeCatalogPage(link, base string) bool {
	resolved, err := url.Parse(link)
	if err != nil {
		return false
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return false
	}
	if resolved.Host != baseURL.Host {
		return false
	}
	lower := strings.ToLower(link)
	return strings.Contains(lower, "skill") || strings.Contains(lower, "market") || strings.Contains(lower, "store")
}

func looksLikeGitHubRepoURL(link string) bool {
	return githubRepoPattern.MatchString(link)
}

func repoOwner(link string) string {
	if matches := githubRepoPattern.FindStringSubmatch(link); len(matches) == 3 {
		return matches[1]
	}
	return ""
}

func repoName(link string) string {
	if matches := githubRepoPattern.FindStringSubmatch(link); len(matches) == 3 {
		return matches[2]
	}
	return ""
}

func rawGitHubBlobURL(owner, repo, branch, path string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, path)
}

func extractHTMLTitle(doc *html.Node) string {
	var title string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if title != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = n.FirstChild.Data
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return title
}

func extractHTMLMeta(doc *html.Node, names ...string) string {
	lookup := make(map[string]struct{}, len(names))
	for _, name := range names {
		lookup[strings.ToLower(name)] = struct{}{}
	}
	var value string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if value != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "meta" {
			key := ""
			content := ""
			for _, attr := range n.Attr {
				switch strings.ToLower(attr.Key) {
				case "name", "property":
					key = strings.ToLower(strings.TrimSpace(attr.Val))
				case "content":
					content = strings.TrimSpace(attr.Val)
				}
			}
			if _, ok := lookup[key]; ok && content != "" {
				value = content
				return
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return value
}

func extractHTMLLinks(doc *html.Node) []string {
	seen := make(map[string]struct{})
	var out []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href := strings.TrimSpace(attr.Val)
					if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") {
						continue
					}
					if _, ok := seen[href]; ok {
						continue
					}
					seen[href] = struct{}{}
					out = append(out, href)
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return out
}

func extractHTMLText(doc *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "svg":
				return
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				b.WriteString(text)
				b.WriteString(" ")
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return b.String()
}

func summarizeText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "AI agent skill catalog entry"
	}
	if len(text) > 240 {
		text = text[:240]
	}
	return strings.TrimSpace(text)
}

func escapeYAMLText(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "\n", " ")
}
