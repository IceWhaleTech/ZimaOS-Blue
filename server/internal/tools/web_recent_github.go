package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type webRecentGitHubIssueSearchResponse struct {
	Items []struct {
		Title     string `json:"title"`
		HTMLURL   string `json:"html_url"`
		Body      string `json:"body"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Comments  int    `json:"comments"`
		User      struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"items"`
}

type webRecentGitHubRepoSearchResponse struct {
	Items []webRecentGitHubRepository `json:"items"`
}

type webRecentGitHubRepository struct {
	FullName        string `json:"full_name"`
	HTMLURL         string `json:"html_url"`
	Description     string `json:"description"`
	UpdatedAt       string `json:"updated_at"`
	DefaultBranch   string `json:"default_branch"`
	StargazersCount int    `json:"stargazers_count"`
	ForksCount      int    `json:"forks_count"`
	Owner           struct {
		Login string `json:"login"`
	} `json:"owner"`
}

type webRecentGitHubRelease struct {
	Name        string `json:"name"`
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	CreatedAt   string `json:"created_at"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	Author      struct {
		Login string `json:"login"`
	} `json:"author"`
}

type webRecentGitHubReadmeResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func (t *WebTool) collectRecentGitHubIssueItems(ctx context.Context, query string, maxResults int, headers map[string]string) ([]webRecentNormalizedItem, error) {
	params := url.Values{}
	params.Set("q", strings.TrimSpace(query)+" updated:>="+time.Now().UTC().AddDate(0, 0, -webRecentLookbackDays).Format("2006-01-02"))
	params.Set("sort", "updated")
	params.Set("order", "desc")
	params.Set("per_page", strconv.Itoa(maxResults))
	var payload webRecentGitHubIssueSearchResponse
	if err := webRecentHTTPJSON(ctx, "https://api.github.com/search/issues?"+params.Encode(), headers, &payload); err != nil {
		return nil, fmt.Errorf("issues: %w", err)
	}
	items := make([]webRecentNormalizedItem, 0, len(payload.Items))
	for _, entry := range payload.Items {
		publishedAt, ok := webRecentPublishedAt(entry.UpdatedAt, entry.CreatedAt)
		if !ok {
			continue
		}
		items = append(items, webRecentNormalizedItem{
			Source:         "github",
			Title:          strings.TrimSpace(entry.Title),
			URL:            strings.TrimSpace(entry.HTMLURL),
			Snippet:        compactWebRecentSnippet(entry.Body),
			PublishedAt:    publishedAt,
			Author:         strings.TrimSpace(entry.User.Login),
			EngagementHint: fmt.Sprintf("%d comments", entry.Comments),
		})
	}
	return items, nil
}

func (t *WebTool) collectRecentGitHubRepositories(ctx context.Context, query string, maxResults int, headers map[string]string) ([]webRecentGitHubRepository, error) {
	if maxResults <= 0 {
		return nil, nil
	}
	params := url.Values{}
	params.Set("q", strings.TrimSpace(query)+" pushed:>="+time.Now().UTC().AddDate(0, 0, -webRecentLookbackDays).Format("2006-01-02"))
	params.Set("sort", "updated")
	params.Set("order", "desc")
	params.Set("per_page", strconv.Itoa(maxResults))
	var payload webRecentGitHubRepoSearchResponse
	if err := webRecentHTTPJSON(ctx, "https://api.github.com/search/repositories?"+params.Encode(), headers, &payload); err != nil {
		return nil, fmt.Errorf("repositories: %w", err)
	}
	repos := make([]webRecentGitHubRepository, 0, len(payload.Items))
	for _, repo := range payload.Items {
		if _, ok := webRecentPublishedAt(repo.UpdatedAt); !ok {
			continue
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

func (t *WebTool) collectRecentGitHubRepositorySignals(ctx context.Context, repositories []webRecentGitHubRepository, headers map[string]string) ([]webRecentNormalizedItem, []webRecentNormalizedItem, error) {
	releases := make([]webRecentNormalizedItem, 0, len(repositories))
	readmes := make([]webRecentNormalizedItem, 0, len(repositories))
	errs := make([]error, 0, len(repositories)*2)
	for _, repo := range repositories {
		releaseItems, err := t.collectRecentGitHubReleaseItems(ctx, repo, headers)
		if err != nil {
			errs = append(errs, err)
		}
		releases = append(releases, releaseItems...)
		readmeItem, err := t.collectRecentGitHubReadmeItem(ctx, repo, headers)
		if err != nil {
			errs = append(errs, err)
		}
		if readmeItem != nil {
			readmes = append(readmes, *readmeItem)
		}
	}
	return releases, readmes, webRecentJoinErrors(errs...)
}

func (t *WebTool) collectRecentGitHubReleaseItems(ctx context.Context, repo webRecentGitHubRepository, headers map[string]string) ([]webRecentNormalizedItem, error) {
	var payload []webRecentGitHubRelease
	targetURL := fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=2", strings.TrimSpace(repo.FullName))
	if err := webRecentHTTPJSON(ctx, targetURL, headers, &payload); err != nil {
		return nil, fmt.Errorf("releases %s: %w", repo.FullName, err)
	}
	items := make([]webRecentNormalizedItem, 0, len(payload))
	for _, release := range payload {
		if release.Draft {
			continue
		}
		publishedAt, ok := webRecentPublishedAt(release.PublishedAt, release.CreatedAt)
		if !ok {
			continue
		}
		title := firstNonEmpty(strings.TrimSpace(release.Name), strings.TrimSpace(release.TagName))
		if title == "" {
			title = repo.FullName + " release"
		}
		engagement := strings.TrimSpace(release.TagName)
		if release.Prerelease {
			engagement = strings.TrimSpace(strings.TrimSpace(engagement) + " prerelease")
		}
		items = append(items, webRecentNormalizedItem{
			Source:         "github",
			Title:          title,
			URL:            strings.TrimSpace(release.HTMLURL),
			Snippet:        compactWebRecentSnippet(release.Body),
			PublishedAt:    publishedAt,
			Author:         strings.TrimSpace(release.Author.Login),
			EngagementHint: engagement,
		})
	}
	return items, nil
}

func (t *WebTool) collectRecentGitHubReadmeItem(ctx context.Context, repo webRecentGitHubRepository, headers map[string]string) (*webRecentNormalizedItem, error) {
	publishedAt, ok := webRecentPublishedAt(repo.UpdatedAt)
	if !ok {
		return nil, nil
	}
	var payload webRecentGitHubReadmeResponse
	targetURL := fmt.Sprintf("https://api.github.com/repos/%s/readme", strings.TrimSpace(repo.FullName))
	if err := webRecentHTTPJSON(ctx, targetURL, headers, &payload); err != nil {
		return nil, fmt.Errorf("readme %s: %w", repo.FullName, err)
	}
	snippet := compactWebRecentSnippet(webRecentGitHubDecodeReadme(payload))
	if snippet == "" {
		snippet = compactWebRecentSnippet(repo.Description)
	}
	if snippet == "" {
		return nil, nil
	}
	item := &webRecentNormalizedItem{
		Source:         "github",
		Title:          strings.TrimSpace(repo.FullName) + " README",
		URL:            strings.TrimSpace(repo.HTMLURL),
		Snippet:        snippet,
		PublishedAt:    publishedAt,
		Author:         strings.TrimSpace(repo.Owner.Login),
		EngagementHint: fmt.Sprintf("%d stars, %d forks", repo.StargazersCount, repo.ForksCount),
	}
	return item, nil
}

func webRecentGitHubDecodeReadme(payload webRecentGitHubReadmeResponse) string {
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(payload.Encoding), "base64") {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(content, "\n", ""))
		if err == nil {
			return string(decoded)
		}
	}
	return content
}

func webRecentSortDedupLimit(items []webRecentNormalizedItem, maxResults int) []webRecentNormalizedItem {
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].PublishedAt == items[j].PublishedAt {
			if items[i].Source == items[j].Source {
				return items[i].Title < items[j].Title
			}
			return items[i].Source < items[j].Source
		}
		return items[i].PublishedAt > items[j].PublishedAt
	})
	seen := map[string]struct{}{}
	deduped := make([]webRecentNormalizedItem, 0, minWebQueryInt(maxResults, len(items)))
	for _, item := range items {
		key := strings.TrimSpace(item.URL)
		if key == "" {
			key = strings.TrimSpace(item.Source + "::" + item.Title + "::" + item.PublishedAt)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, item)
		if maxResults > 0 && len(deduped) >= maxResults {
			break
		}
	}
	return deduped
}

func webRecentJoinErrors(errs ...error) error {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		parts = append(parts, strings.TrimSpace(err.Error()))
	}
	if len(parts) == 0 {
		return nil
	}
	return errors.New(strings.Join(parts, "; "))
}
