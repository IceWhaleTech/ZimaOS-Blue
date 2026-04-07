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
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

type catalogPage struct {
	URL         string
	Title       string
	Description string
	Author      string
	Text        string
	Links       []string
	Embedded    *catalogEmbeddedSkill
}

type catalogEmbeddedSkill struct {
	Name      string
	Author    string
	RepoURL   string
	SkillPath string
	RawSkill  string
}

var githubBlobPattern = skillbundle.GitHubBlobURLPattern
var githubRepoPattern = skillbundle.GitHubRepoURLPattern
var githubRawPattern = skillbundle.GitHubRawURLPattern

func (s *Service) discoverFromHTMLCatalog(ctx context.Context, source Source, processedSources int, total *DiscoverResult, run *CrawlRun) error {
	job, err := buildDiscoverJob(s, source, run)
	if err != nil {
		return err
	}
	return s.runDiscoverJobToCompletion(ctx, job)
}

func (s *Service) discoverGitHubRepoSeed(ctx context.Context, owner, repo string) error {
	record, err := s.prepareGitHubRepoSeedRecord(ctx, owner, repo, nil)
	if err != nil {
		return err
	}
	_, err = s.store.UpsertSkillBatch(ctx, []*SkillUpsertRecord{record})
	return err
}

func (s *Service) prepareGitHubRepoSeedRecord(ctx context.Context, owner, repo string, origin *Source) (*SkillUpsertRecord, error) {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(repo) == "" {
		return nil, fmt.Errorf("invalid github seed")
	}
	metaURL := fmt.Sprintf("%s/repos/%s/%s", strings.TrimRight(s.cfg.GitHubAPIBaseURL, "/"), owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metaURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(s.cfg.GitHubToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github repo status: %d", resp.StatusCode)
	}
	var repoMeta struct {
		HTMLURL       string    `json:"html_url"`
		DefaultBranch string    `json:"default_branch"`
		UpdatedAt     time.Time `json:"updated_at"`
		Stargazers    int       `json:"stargazers_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repoMeta); err != nil {
		return nil, err
	}
	contents, err := s.fetchGitHubRepoSkill(ctx, owner, repo, repoMeta.DefaultBranch)
	if err != nil {
		return nil, err
	}
	sourceID := "curated-github-seed"
	sourceName := "GitHub"
	sourceGroup := "github"
	originSourceID := ""
	originSourceName := ""
	originSourceURL := ""
	if origin != nil {
		sourceID = strings.TrimSpace(origin.ID)
		sourceName = defaultString(origin.DisplayName, origin.ID)
		sourceGroup = defaultString(origin.SourceGroup, origin.ID)
		originSourceID = strings.TrimSpace(origin.ID)
		originSourceName = defaultString(origin.DisplayName, origin.ID)
		originSourceURL = strings.TrimSpace(origin.BaseURL)
	}
	return s.prepareIngestRecord(ctx, ingestRequest{
		SourceID:         sourceID,
		SourceName:       sourceName,
		SourceGroup:      sourceGroup,
		OriginSourceID:   originSourceID,
		OriginSourceName: originSourceName,
		OriginSourceURL:  originSourceURL,
		SourceType:       "github_repo",
		RepoURL:          repoMeta.HTMLURL,
		Homepage:         repoMeta.HTMLURL,
		DownloadURL:      rawGitHubBlobURL(owner, repo, repoMeta.DefaultBranch, contents.Path),
		SourceURL:        repoMeta.HTMLURL,
		SkillPath:        contents.Path,
		SkillContent:     contents.Raw,
		CommitHash:       contents.Commit,
		Stars:            repoMeta.Stargazers,
		LastUpdated:      repoMeta.UpdatedAt,
		DefaultSkillID:   pathSkillID(repoMeta.HTMLURL, contents.Path, repo),
		Installable:      true,
		InstallType:      InstallTypeGitRepo,
		ArtifactKind:     ArtifactKindOpenSource,
	})
}

func (s *Service) discoverSkillURLSeed(ctx context.Context, rawURL string) error {
	record, err := s.prepareSkillURLSeedRecord(ctx, rawURL, nil)
	if err != nil {
		return err
	}
	_, err = s.store.UpsertSkillBatch(ctx, []*SkillUpsertRecord{record})
	return err
}

func (s *Service) prepareSkillURLSeedRecord(ctx context.Context, rawURL string, origin *Source) (*SkillUpsertRecord, error) {
	content, skillPath, repoURL, downloadURL, installType, err := s.fetchSkillReference(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	sourceID := "curated-skill-url"
	sourceName := "External Skill"
	sourceGroup := "external"
	originSourceID := ""
	originSourceName := ""
	originSourceURL := ""
	if origin != nil {
		sourceID = strings.TrimSpace(origin.ID)
		sourceName = defaultString(origin.DisplayName, origin.ID)
		sourceGroup = defaultString(origin.SourceGroup, origin.ID)
		originSourceID = strings.TrimSpace(origin.ID)
		originSourceName = defaultString(origin.DisplayName, origin.ID)
		originSourceURL = strings.TrimSpace(origin.BaseURL)
	}
	return s.prepareIngestRecord(ctx, ingestRequest{
		SourceID:         sourceID,
		SourceName:       sourceName,
		SourceGroup:      sourceGroup,
		OriginSourceID:   originSourceID,
		OriginSourceName: originSourceName,
		OriginSourceURL:  originSourceURL,
		SourceType:       "skill_url",
		RepoURL:          repoURL,
		Homepage:         rawURL,
		DownloadURL:      downloadURL,
		SourceURL:        rawURL,
		SkillPath:        skillPath,
		SkillContent:     content,
		DefaultSkillID:   pathSkillID(repoURL, skillPath, filepath.Base(skillPath)),
		LastUpdated:      timeutil.NowTime(),
		Installable:      true,
		InstallType:      installType,
		ArtifactKind:     ArtifactKindOpenSource,
	})
}

func (s *Service) upsertCatalogOnlySkill(ctx context.Context, source Source, page *catalogPage) error {
	record, err := s.prepareCatalogOnlySkillRecord(ctx, source, page)
	if err != nil {
		return err
	}
	_, err = s.store.UpsertSkillBatch(ctx, []*SkillUpsertRecord{record})
	return err
}

func (s *Service) prepareCatalogOnlySkillRecord(ctx context.Context, source Source, page *catalogPage) (*SkillUpsertRecord, error) {
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
	return s.prepareIngestRecord(ctx, ingestRequest{
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
}

func (s *Service) ingestEmbeddedCatalogSkill(ctx context.Context, source Source, page *catalogPage) (bool, error) {
	record, err := s.prepareEmbeddedCatalogSkillRecord(ctx, source, page)
	if err != nil {
		return false, err
	}
	result, err := s.store.UpsertSkillBatch(ctx, []*SkillUpsertRecord{record})
	if err != nil {
		return false, err
	}
	return result != nil && result.Updated > 0, nil
}

func (s *Service) prepareEmbeddedCatalogSkillRecord(ctx context.Context, source Source, page *catalogPage) (*SkillUpsertRecord, error) {
	if page == nil || page.Embedded == nil || strings.TrimSpace(page.Embedded.RawSkill) == "" {
		return nil, fmt.Errorf("embedded catalog skill unavailable")
	}

	name := strings.TrimSpace(firstNonBlank(page.Embedded.Name, trimCatalogSkillTitle(page.Title), page.Title))
	if name == "" {
		name = normalizeSkillID(page.URL)
	}
	explicitID := normalizeSkillID(source.SourceGroup + "-" + name)
	if explicitID == "" {
		explicitID = normalizeSkillID(name)
	}

	return s.prepareIngestRecord(ctx, ingestRequest{
		SourceID:        source.ID,
		SourceName:      defaultString(source.DisplayName, source.ID),
		SourceGroup:     defaultString(source.SourceGroup, source.ID),
		SourceType:      source.Type,
		RepoURL:         firstNonBlank(page.Embedded.RepoURL, page.URL),
		Homepage:        page.URL,
		DownloadURL:     page.URL,
		SourceURL:       page.URL,
		SkillPath:       firstNonBlank(page.Embedded.SkillPath, "SKILL.md"),
		SkillContent:    page.Embedded.RawSkill,
		LastUpdated:     timeutil.NowTime(),
		DefaultSkillID:  explicitID,
		ExplicitID:      explicitID,
		ExplicitName:    name,
		ExplicitAuthor:  firstNonBlank(page.Embedded.Author, page.Author),
		DescriptionHint: page.Description,
		Installable:     true,
		InstallType:     InstallTypeRawSkill,
		ArtifactKind:    ArtifactKindOpenSource,
	})
}

func (s *Service) fetchCatalogPage(ctx context.Context, source Source, rawURL string) (*catalogPage, error) {
	read, err := s.remoteReader.ReadURL(ctx, RemoteReadRequest{
		URL:         rawURL,
		Headers:     source.Headers,
		Format:      "text",
		MaxChars:    32_000,
		WantRawHTML: true,
	})
	if err != nil {
		return nil, err
	}
	htmlBody := read.RawHTML
	var doc *html.Node
	if strings.TrimSpace(htmlBody) == "" {
		if fallbackHTML, fallbackErr := s.fetchCatalogRawHTML(ctx, source, rawURL); fallbackErr == nil {
			htmlBody = fallbackHTML
		}
	}
	if strings.TrimSpace(htmlBody) != "" {
		doc, err = html.Parse(strings.NewReader(htmlBody))
		if err != nil {
			return nil, err
		}
	}
	title := strings.TrimSpace(read.Title)
	description := ""
	author := ""
	links := append([]string(nil), read.Links...)
	if doc != nil {
		if title == "" {
			title = strings.TrimSpace(extractHTMLTitle(doc))
		}
		description = strings.TrimSpace(extractHTMLMeta(doc, "description", "og:description", "twitter:description"))
		author = strings.TrimSpace(extractHTMLMeta(doc, "author", "article:author"))
		if len(links) == 0 {
			links = extractHTMLLinks(doc)
		}
	}
	if website := extractLLMSkillsFlightWebsite(rawURL, htmlBody); website != "" && !containsString(links, website) {
		links = append(links, website)
	}
	return &catalogPage{
		URL:         rawURL,
		Title:       title,
		Description: description,
		Author:      author,
		Text:        strings.TrimSpace(read.Content),
		Links:       links,
		Embedded:    extractCatalogEmbeddedSkill(rawURL, htmlBody),
	}, nil
}

func (s *Service) fetchCatalogRawHTML(ctx context.Context, source Source, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
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
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("catalog page status: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func trimCatalogSkillTitle(title string) string {
	title = strings.TrimSpace(title)
	for _, suffix := range []string{
		" - Claude Skill Details | SkillHub",
		" | SkillHub",
	} {
		if strings.HasSuffix(title, suffix) {
			title = strings.TrimSpace(strings.TrimSuffix(title, suffix))
		}
	}
	return title
}

func extractCatalogEmbeddedSkill(pageURL, htmlBody string) *catalogEmbeddedSkill {
	ensureSkillMarketCatalogRegexes()
	normalizedBody := strings.ReplaceAll(htmlBody, `\"`, `"`)
	if !strings.Contains(strings.ToLower(pageURL), "/skills/") || !strings.Contains(normalizedBody, `skillMdRaw":"$`) {
		return nil
	}

	refMatch := skillHubStringRefPattern.FindStringSubmatch(normalizedBody)
	if len(refMatch) != 2 {
		return nil
	}
	rawPattern := regexp.MustCompile(regexp.QuoteMeta(refMatch[1]) + `:T[0-9A-Fa-f]+,"((?:\\.|[^"\\])*)"`)
	rawMatch := rawPattern.FindStringSubmatch(normalizedBody)
	if len(rawMatch) != 2 {
		return nil
	}
	rawSkill := decodeEmbeddedCatalogString(rawMatch[1])
	if strings.TrimSpace(rawSkill) == "" {
		return nil
	}

	return &catalogEmbeddedSkill{
		Name:      decodeEmbeddedCatalogMatch(normalizedBody, `skillName":"((?:\\.|[^"\\])*)"`),
		RepoURL:   decodeEmbeddedCatalogMatch(normalizedBody, `repoUrl":"((?:\\.|[^"\\])*)"`),
		SkillPath: decodeEmbeddedCatalogMatch(normalizedBody, `skillPath":"((?:\\.|[^"\\])*)"`),
		RawSkill:  rawSkill,
	}
}

func extractLLMSkillsFlightWebsite(pageURL, htmlBody string) string {
	ensureSkillMarketCatalogRegexes()
	if !strings.Contains(strings.ToLower(pageURL), "/skill/") {
		return ""
	}
	normalizedBody := strings.ReplaceAll(htmlBody, `\"`, `"`)
	match := llmSkillsWebsitePattern.FindStringSubmatch(normalizedBody)
	if len(match) != 3 {
		return ""
	}
	website := decodeEmbeddedCatalogString(match[1])
	installPath := decodeEmbeddedCatalogString(match[2])
	if strings.TrimSpace(installPath) == "" || !looksLikeSkillURL(website) {
		return ""
	}
	return website
}

func decodeEmbeddedCatalogMatch(htmlBody, pattern string) string {
	re := regexp.MustCompile(pattern)
	match := re.FindStringSubmatch(htmlBody)
	if len(match) != 2 {
		return ""
	}
	return decodeEmbeddedCatalogString(match[1])
}

func decodeEmbeddedCatalogString(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	decoded, err := strconv.Unquote(`"` + value + `"`)
	if err == nil {
		return strings.TrimSpace(decoded)
	}
	return strings.TrimSpace(strings.ReplaceAll(value, `\/`, `/`))
}

func (s *Service) fetchSkillReference(ctx context.Context, rawURL string) (content, skillPath, repoURL, downloadURL, installType string, err error) {
	if matches := githubBlobPattern.FindStringSubmatch(rawURL); len(matches) == 5 {
		body, _, err := s.fetchGitHubRawContent(ctx, matches[1], matches[2], matches[3], matches[4])
		downloadURL = skillbundle.RawGitHubBlobURL(matches[1], matches[2], matches[3], matches[4])
		if err != nil {
			return "", "", "", "", "", err
		}
		return body, matches[4], fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2]), downloadURL, InstallTypeRawSkill, nil
	}
	if matches := githubRawPattern.FindStringSubmatch(rawURL); len(matches) == 5 {
		body, _, err := s.fetchGitHubRawContent(ctx, matches[1], matches[2], matches[3], matches[4])
		if err != nil {
			return "", "", "", "", "", err
		}
		return body, matches[4], fmt.Sprintf("https://github.com/%s/%s", matches[1], matches[2]), rawURL, InstallTypeRawSkill, nil
	}
	if owner, repo, branch, skillRoot, ok := skillbundle.ParseGitHubTreeURL(rawURL); ok {
		found, err := s.fetchGitHubRepoSkillAtPath(ctx, owner, repo, branch, skillRoot)
		if err != nil {
			return "", "", "", "", "", err
		}
		downloadURL = rawGitHubBlobURL(owner, repo, branch, found.Path)
		return found.Raw, found.Path, fmt.Sprintf("https://github.com/%s/%s", owner, repo), downloadURL, InstallTypeGitRepo, nil
	}
	read, err := s.remoteReader.ReadURL(ctx, RemoteReadRequest{
		URL:      rawURL,
		Format:   "text",
		MaxChars: 256_000,
	})
	if err != nil {
		return "", "", "", "", "", err
	}
	return read.Content, filepath.Base(rawURL), rawURL, rawURL, InstallTypeRawSkill, nil
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
	return skillbundle.RawGitHubBlobURL(owner, repo, branch, path)
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func looksLikeCatalogPlaceholderPage(page *catalogPage) bool {
	if page == nil {
		return false
	}

	title := strings.ToLower(strings.TrimSpace(page.Title))
	description := strings.ToLower(strings.TrimSpace(page.Description))
	text := strings.ToLower(strings.TrimSpace(page.Text))
	combined := title + "\n" + description + "\n" + text

	if title == "" && description == "" && text == "" {
		return false
	}

	titleMarkers := []string{
		"under construction",
		"page not found",
		"404",
		"maintenance",
	}
	textMarkers := []string{
		"we're making things better",
		"we are making things better",
		"currently under construction",
		"we'll be back soon",
		"we will be back soon",
		"coming soon",
		"page not found",
		"temporarily unavailable",
	}

	for _, marker := range titleMarkers {
		if strings.Contains(title, marker) {
			return true
		}
	}
	for _, marker := range textMarkers {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
}
