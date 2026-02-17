package downloader

import (
	"fmt"
	"regexp"
)

var (
	reJsDelivr  = regexp.MustCompile(`^https://cdn\.jsdelivr\.net/gh/([^/]+)/([^@]+)@([^/]+)/(.*)$`)
	reGitHubRaw = regexp.MustCompile(`^https://raw\.githubusercontent\.com/([^/]+)/([^/]+)/([^/]+)/(.*)$`)
)

type GitHubParts struct {
	User   string
	Repo   string
	Branch string
	Path   string
	Type   string // raw, jsdelivr
}

// ExpandGitHubURL generates all mirror URLs with the input format first.
func ExpandGitHubURL(input string) []string {
	var p *GitHubParts
	switch {
	case reGitHubRaw.MatchString(input):
		m := reGitHubRaw.FindStringSubmatch(input)
		p = &GitHubParts{User: m[1], Repo: m[2], Branch: m[3], Path: m[4], Type: "raw"}
	case reJsDelivr.MatchString(input):
		m := reJsDelivr.FindStringSubmatch(input)
		p = &GitHubParts{User: m[1], Repo: m[2], Branch: m[3], Path: m[4], Type: "jsdelivr"}
	default:
		return []string{input}
	}

	all := map[string]string{
		"jsdelivr": fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s/%s@%s/%s", p.User, p.Repo, p.Branch, p.Path),
		"raw":      fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", p.User, p.Repo, p.Branch, p.Path),
	}

	order := []string{p.Type}
	for _, t := range []string{"jsdelivr", "raw"} {
		if t != p.Type {
			order = append(order, t)
		}
	}

	var result []string
	for _, t := range order {
		result = append(result, all[t])
	}
	return result
}
