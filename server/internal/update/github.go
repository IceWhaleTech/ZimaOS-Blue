package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"time"
)

// GitHubClient handles version discovery from GitHub
type GitHubClient struct {
	httpClient  *http.Client
	owner       string
	repo        string
	primaryURL  string
	fallbackURL string
}

// NewGitHubClient creates a new GitHub client
func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		owner:       "IceWhaleTech",
		repo:        "ZimaOS-Blue",
		primaryURL:  "https://api.github.com/repos/IceWhaleTech/ZimaOS-Blue/contents/release-note",
		fallbackURL: "https://cdn.jsdelivr.net/gh/IceWhaleTech/ZimaOS-Blue@main/release-note/",
	}
}

// GitHubContent represents GitHub API content response
type GitHubContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
}

// ListVersions lists all available versions from release-note directory
func (c *GitHubClient) ListVersions(ctx context.Context) ([]*Version, error) {
	versions, err := c.listFromGitHub(ctx)
	if err != nil {
		// Try fallback
		versions, err = c.listFromCDN(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list versions: %w", err)
		}
	}
	return versions, nil
}

func (c *GitHubClient) listFromGitHub(ctx context.Context) ([]*Version, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.primaryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var contents []GitHubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	var versions []*Version
	for _, content := range contents {
		if content.Type == "dir" {
			v, err := ParseVersion(content.Name)
			if err == nil {
				versions = append(versions, v)
			}
		}
	}

	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Compare(versions[j]) > 0
	})

	return versions, nil
}

func (c *GitHubClient) listFromCDN(ctx context.Context) ([]*Version, error) {
	// CDN fallback - simplified implementation
	return nil, fmt.Errorf("CDN fallback not implemented")
}

// GetLatestVersion returns the latest version for a given channel
func (c *GitHubClient) GetLatestVersion(ctx context.Context, channel string) (*Version, error) {
	versions, err := c.ListVersions(ctx)
	if err != nil {
		return nil, err
	}

	for _, v := range versions {
		if channel == ChannelStable && v.Prerelease == "" {
			return v, nil
		}
		if channel == ChannelBeta && (v.Prerelease == "" || v.Prerelease == "beta") {
			return v, nil
		}
		if channel == ChannelAlpha {
			return v, nil
		}
	}

	return nil, fmt.Errorf("no version found for channel %s", channel)
}

// GetDownloadURL returns the download URL for a specific version
func (c *GitHubClient) GetDownloadURL(version string) string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	return fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/v%s/zimaos-blue-%s-%s",
		c.owner, c.repo, version, os, arch,
	)
}

// GetReleaseNotes fetches release notes for a version
func (c *GitHubClient) GetReleaseNotes(ctx context.Context, version string) (string, error) {
	url := fmt.Sprintf("%s/%s/CHANGELOG.md", c.primaryURL, version)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil // No release notes available
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Handle base64 encoded content from GitHub API
	var content struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(body, &content); err == nil && content.Content != "" {
		return strings.TrimSpace(content.Content), nil
	}

	return string(body), nil
}
