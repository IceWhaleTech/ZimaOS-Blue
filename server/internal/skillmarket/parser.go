package skillmarket

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type frontmatter struct {
	ID          string      `yaml:"id"`
	Name        string      `yaml:"name"`
	Version     string      `yaml:"version"`
	Description string      `yaml:"description"`
	Author      string      `yaml:"author"`
	Category    string      `yaml:"category"`
	Tags        interface{} `yaml:"tags"`
	Permissions interface{} `yaml:"permissions"`
}

type normalizedSkill struct {
	Manifest  *skill.Manifest
	Content   string
	Tags      []string
	Checksum  string
	Version   string
	SearchDoc string
}

func normalizeSkillID(raw string) string {
	ensureSkillMarketParserGlobals()
	s := strings.TrimSpace(strings.ToLower(raw))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "/", "-")
	s = nonSlugChars.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-.")
	if s == "" {
		return "skill"
	}
	return s
}

func splitStringList(raw interface{}) []string {
	switch v := raw.(type) {
	case string:
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			str := strings.TrimSpace(fmt.Sprint(item))
			if str != "" {
				out = append(out, str)
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func normalizeTags(category string, tags []string) []string {
	set := make(map[string]struct{})
	for _, value := range append(tags, category) {
		value = normalizeSkillID(value)
		if value == "" || value == "skill" {
			continue
		}
		set[value] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func looksLikeHTMLSkillContent(raw string) bool {
	snippet := strings.ToLower(strings.TrimSpace(raw))
	if len(snippet) > 2048 {
		snippet = snippet[:2048]
	}
	if snippet == "" {
		return false
	}
	if strings.HasPrefix(snippet, "<!doctype html") || strings.HasPrefix(snippet, "<html") {
		return true
	}
	return strings.Contains(snippet, "<head") && strings.Contains(snippet, "<body")
}

func parseSkillMarkdown(raw string, fallbackID string) (*normalizedSkill, error) {
	if looksLikeHTMLSkillContent(raw) {
		return nil, fmt.Errorf("skill content is html, not markdown")
	}

	manifest := &skill.Manifest{
		Version:  "0.1.0",
		Metadata: map[string]string{},
	}
	content := raw
	if strings.HasPrefix(raw, "---") {
		parts := strings.SplitN(raw, "---", 3)
		if len(parts) >= 3 {
			var fm frontmatter
			if err := yaml.Unmarshal([]byte(parts[1]), &fm); err != nil {
				return nil, fmt.Errorf("parse frontmatter: %w", err)
			}
			manifest.ID = fm.ID
			manifest.Name = fm.Name
			manifest.Version = defaultString(fm.Version, manifest.Version)
			manifest.Description = fm.Description
			manifest.Author = fm.Author
			manifest.Category = fm.Category
			manifest.Tags = splitStringList(fm.Tags)
			manifest.Permissions = splitStringList(fm.Permissions)
			content = strings.TrimSpace(parts[2])
		}
	}

	if manifest.ID == "" {
		manifest.ID = fallbackID
	}
	manifest.ID = normalizeSkillID(manifest.ID)
	if manifest.Name == "" {
		manifest.Name = manifest.ID
	}
	if manifest.Description == "" {
		manifest.Description = inferDescription(content)
	}
	manifest.Category = normalizeCategory(manifest.Category, content, manifest.Tags)
	manifest.Tags = normalizeTags(manifest.Category, manifest.Tags)

	sum := sha256.Sum256([]byte(raw))
	checksum := hex.EncodeToString(sum[:])
	doc := strings.TrimSpace(strings.Join([]string{
		manifest.Name,
		manifest.Description,
		manifest.Author,
		manifest.Category,
		strings.Join(manifest.Tags, " "),
		content,
	}, "\n"))

	return &normalizedSkill{
		Manifest:  manifest,
		Content:   content,
		Tags:      manifest.Tags,
		Checksum:  checksum,
		Version:   manifest.Version,
		SearchDoc: doc,
	}, nil
}

func inferDescription(content string) string {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if line != "" {
			return line
		}
	}
	return "AI agent skill"
}

func inferCategory(content string, tags []string) string {
	lower := strings.ToLower(strings.Join(append(tags, content), " "))
	switch {
	case containsAny(lower,
		"git", "github", "repo", "pull request", "rebase", "commit", "docker", "kubernetes",
		"terminal", "shell", "cli", "sdk", "code", "coding", "devops", "ci/cd", "workflow file",
	):
		return "development_tools"
	case containsAny(lower, "dashboard", "analytics", "metrics", "sql", "csv", "database", "spreadsheet", "data", "crawl", "scrape", "research", "search", "insight"):
		return "data_analysis"
	case containsAny(lower, "email", "slack", "discord", "telegram", "message", "calendar", "meeting", "notion", "collaboration", "team", "crm"):
		return "communication_collaboration"
	case containsAny(lower, "write", "writer", "blog", "article", "copy", "marketing", "image", "video", "presentation", "slide", "content", "translate", "translation", "design"):
		return "content_creation"
	case containsAny(lower, "security", "audit", "auth", "sandbox", "secret", "vulnerability", "permission", "firewall"):
		return "security_compliance"
	case containsAny(lower, "cron", "scheduler", "task", "todo", "productivity", "automation", "reminder", "workflow", "organize"):
		return "productivity"
	case containsAny(lower, "llm", "agent", "prompt", "reasoning", "model", "inference", "embedding", "knowledge base"):
		return "ai_intelligence"
	default:
		return "productivity"
	}
}

func containsAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if needle != "" && strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func looksVersionLikeCategory(raw string) bool {
	ensureSkillMarketParserGlobals()
	value := normalizeSkillID(raw)
	if value == "" {
		return false
	}
	return versionLikeCategoryPattern.MatchString(value)
}

func categoryAliasKey(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"&", " ",
		"/", " ",
		"\\", " ",
		"-", " ",
		".", " ",
		",", " ",
		":", " ",
	)
	value = replacer.Replace(value)
	value = strings.Join(strings.Fields(value), "_")
	return strings.Trim(value, "_")
}

func normalizeCategory(raw, content string, tags []string) string {
	ensureSkillMarketParserGlobals()
	if strings.TrimSpace(raw) == "" {
		return inferCategory(content, tags)
	}
	aliasValue := categoryAliasKey(raw)
	if mapped, ok := marketplaceCategoryAliases[aliasValue]; ok && !looksVersionLikeCategory(mapped) {
		return mapped
	}
	value := normalizeSkillID(raw)
	if mapped, ok := marketplaceCategoryAliases[value]; ok && !looksVersionLikeCategory(mapped) {
		return mapped
	}
	if value == "" || looksVersionLikeCategory(value) {
		return inferCategory(content, tags)
	}
	if strings.Count(value, ".") >= 2 {
		return inferCategory(content, tags)
	}
	return value
}

func NormalizeMarketplaceCategory(raw, content string, tags []string) string {
	return normalizeCategory(raw, content, tags)
}

func MarketplaceCategoryOrder(category string) int {
	if rank, ok := categoryDisplayOrder[normalizeCategory(category, "", nil)]; ok {
		return rank
	}
	return categoryDisplayOrder["other"]
}

func manifestJSON(manifest *skill.Manifest) string {
	if manifest == nil {
		return "{}"
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func pathSkillID(repoURL, skillPath, fallbackID string) string {
	base := strings.TrimSuffix(filepath.Base(skillPath), filepath.Ext(skillPath))
	if base == "" || strings.EqualFold(base, "skill") || strings.EqualFold(base, "claude") || strings.EqualFold(base, "agent") {
		base = filepath.Base(filepath.Dir(skillPath))
	}
	if base == "." || base == "/" || base == "" {
		base = fallbackID
	}
	if repoURL == "" {
		return normalizeSkillID(base)
	}
	repo := normalizeSkillID(filepath.Base(repoURL))
	if repo == "" {
		return normalizeSkillID(base)
	}
	return normalizeSkillID(repo + "-" + base)
}
