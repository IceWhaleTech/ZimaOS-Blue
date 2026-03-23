package skillbundle

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	GitHubBlobURLPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/blob/([^/]+)/(.+)$`)
	GitHubRawURLPattern  = regexp.MustCompile(`^https?://raw\.githubusercontent\.com/([^/]+)/([^/]+)/([^/]+)/(.+)$`)
	GitHubRepoURLPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)
	GitHubTreeURLPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/tree/([^/]+)/(.+)$`)
)

func RawGitHubBlobURL(owner, repo, ref, path string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path)
}

func GitHubRawURLCandidates(owner, repo, ref, path string) []string {
	path = strings.TrimPrefix(path, "/")
	return []string{
		RawGitHubBlobURL(owner, repo, ref, path),
		fmt.Sprintf("https://raw.gitmirror.com/%s/%s/%s/%s", owner, repo, ref, path),
		fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", owner, repo, ref, path),
		fmt.Sprintf("https://ghproxy.com/https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path),
	}
}

func ParseGitHubBlobURL(rawURL string) (string, string, string, string, bool) {
	matches := GitHubBlobURLPattern.FindStringSubmatch(strings.TrimSpace(rawURL))
	if len(matches) != 5 {
		return "", "", "", "", false
	}
	return matches[1], matches[2], matches[3], matches[4], true
}

func ParseGitHubRawURL(rawURL string) (string, string, string, string, bool) {
	matches := GitHubRawURLPattern.FindStringSubmatch(strings.TrimSpace(rawURL))
	if len(matches) != 5 {
		return "", "", "", "", false
	}
	return matches[1], matches[2], matches[3], matches[4], true
}

func ParseGitHubTreeURL(rawURL string) (string, string, string, string, bool) {
	matches := GitHubTreeURLPattern.FindStringSubmatch(strings.TrimSpace(rawURL))
	if len(matches) != 5 {
		return "", "", "", "", false
	}
	return matches[1], matches[2], matches[3], matches[4], true
}

func IsGitHubRepoURL(rawURL string) bool {
	return GitHubRepoURLPattern.MatchString(strings.TrimSpace(rawURL))
}
