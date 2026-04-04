package skillmarket

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
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

var nonSlugChars = regexp.MustCompile(`[^a-z0-9._-]+`)
var versionLikeCategoryPattern = regexp.MustCompile(`^(?:v?\d+(?:[._-]\d+){0,4}(?:[-+][a-z0-9._-]+)?|release[-_. ]?v?\d+(?:[._-]\d+){0,4}|latest|stable)$`)

var marketplaceCategoryAliases = map[string]string{
	"ai":                              "ai_intelligence",
	"ai_intelligence":                 "ai_intelligence",
	"ai_skills":                       "ai_intelligence",
	"ai_tools":                        "ai_intelligence",
	"agent":                           "ai_intelligence",
	"agentic":                         "ai_intelligence",
	"agents":                          "ai_intelligence",
	"assistant":                       "ai_intelligence",
	"chatbot":                         "ai_intelligence",
	"claude":                          "ai_intelligence",
	"copilot":                         "ai_intelligence",
	"gpt":                             "ai_intelligence",
	"llm":                             "ai_intelligence",
	"model":                           "ai_intelligence",
	"prompt":                          "ai_intelligence",
	"rag":                             "ai_intelligence",
	"workflow_ai":                     "ai_intelligence",
	"人工智能":                            "ai_intelligence",
	"智能":                              "ai_intelligence",
	"ai智能":                            "ai_intelligence",
	"ai_智能":                           "ai_intelligence",
	"analytics":                       "data_analysis",
	"analysis":                        "data_analysis",
	"browser":                         "data_analysis",
	"data":                            "data_analysis",
	"data_analysis":                   "data_analysis",
	"information":                     "data_analysis",
	"research":                        "data_analysis",
	"search":                          "data_analysis",
	"web":                             "data_analysis",
	"分析":                              "data_analysis",
	"数据":                              "data_analysis",
	"数据分析":                            "data_analysis",
	"开发":                              "development_tools",
	"开发工具":                            "development_tools",
	"api":                             "development_tools",
	"coding":                          "development_tools",
	"connector":                       "development_tools",
	"connectors":                      "development_tools",
	"developer":                       "development_tools",
	"developer_tools":                 "development_tools",
	"developer-tools":                 "development_tools",
	"development":                     "development_tools",
	"development_tools":               "development_tools",
	"dev":                             "development_tools",
	"devops":                          "development_tools",
	"extension":                       "development_tools",
	"github":                          "development_tools",
	"git":                             "development_tools",
	"infrastructure":                  "development_tools",
	"integration":                     "development_tools",
	"integrations":                    "development_tools",
	"plugin":                          "development_tools",
	"plugins":                         "development_tools",
	"sdk":                             "development_tools",
	"tooling":                         "development_tools",
	"webhook":                         "development_tools",
	"communication":                   "communication_collaboration",
	"communication_collaboration":     "communication_collaboration",
	"collaboration":                   "communication_collaboration",
	"crm":                             "communication_collaboration",
	"chat":                            "communication_collaboration",
	"discord":                         "communication_collaboration",
	"email":                           "communication_collaboration",
	"meeting":                         "communication_collaboration",
	"messaging":                       "communication_collaboration",
	"notion":                          "communication_collaboration",
	"slack":                           "communication_collaboration",
	"teams":                           "communication_collaboration",
	"telegram":                        "communication_collaboration",
	"协作":                              "communication_collaboration",
	"通讯协作":                            "communication_collaboration",
	"communication_and_collaboration": "communication_collaboration",
	"content":                         "content_creation",
	"content_creation":                "content_creation",
	"copywriting":                     "content_creation",
	"creative":                        "content_creation",
	"design":                          "content_creation",
	"documentation":                   "content_creation",
	"image":                           "content_creation",
	"marketing":                       "content_creation",
	"media":                           "content_creation",
	"presentation":                    "content_creation",
	"social":                          "content_creation",
	"translation":                     "content_creation",
	"video":                           "content_creation",
	"writing":                         "content_creation",
	"内容创作":                            "content_creation",
	"automation":                      "productivity",
	"efficiency":                      "productivity",
	"general":                         "productivity",
	"productivity":                    "productivity",
	"task":                            "productivity",
	"todo":                            "productivity",
	"tool":                            "productivity",
	"tools":                           "productivity",
	"utility":                         "productivity",
	"utilities":                       "productivity",
	"workflow":                        "productivity",
	"效率":                              "productivity",
	"效率提升":                            "productivity",
	"compliance":                      "security_compliance",
	"governance":                      "security_compliance",
	"ops":                             "security_compliance",
	"risk":                            "security_compliance",
	"security":                        "security_compliance",
	"security_compliance":             "security_compliance",
	"system":                          "security_compliance",
	"安全":                              "security_compliance",
	"安全合规":                            "security_compliance",
	"misc":                            "other",
	"miscellaneous":                   "other",
	"other":                           "other",
}

var categoryDisplayOrder = map[string]int{
	"ai_intelligence":             10,
	"development_tools":           20,
	"productivity":                30,
	"data_analysis":               40,
	"content_creation":            50,
	"security_compliance":         60,
	"communication_collaboration": 70,
	"other":                       999,
}

func normalizeSkillID(raw string) string {
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
