package claudecode

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillDoc is a compact, selector-friendly representation of a skill.
type SkillDoc struct {
	Name        string
	Description string
	Tags        []string
	Category    string
	Example     string
	Body        string
	SourcePath  string
}

// BuildSkillIndex scans workspace and user default skill roots and builds a
// deduplicated index by skill name (workspace root wins on conflicts).
func BuildSkillIndex(workspaceDir string) ([]SkillDoc, error) {
	roots := resolveSkillRoots(workspaceDir)
	if len(roots) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{})
	docs := make([]SkillDoc, 0, 64)

	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillID := entry.Name()
			if _, ok := seen[strings.ToLower(skillID)]; ok {
				continue
			}
			mdPath := filepath.Join(root, skillID, "SKILL.md")
			data, err := os.ReadFile(mdPath)
			if err != nil {
				continue
			}

			se := parseSkillEntry(skillID, mdPath, data)
			if !se.Enabled || !skillPlatformMatch(se.OS) {
				continue
			}

			doc := buildSkillDoc(se, data)
			if doc.Name == "" {
				continue
			}
			key := strings.ToLower(doc.Name)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			docs = append(docs, doc)
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		pi, pj := skillSortPriority(docs[i].Name), skillSortPriority(docs[j].Name)
		if pi != pj {
			return pi < pj
		}
		return docs[i].Name < docs[j].Name
	})

	return docs, nil
}

func buildSkillDoc(se SkillEntry, raw []byte) SkillDoc {
	content := string(raw)
	frontmatter := ""
	body := content
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter = parts[1]
			body = strings.TrimSpace(parts[2])
		}
	}

	tags, category := parseSkillFrontmatterExtras(frontmatter)
	example := extractSkillExample(body, se.Name)
	if se.Description == "" {
		se.Description = extractFirstParagraph(body)
	}

	return SkillDoc{
		Name:        strings.TrimSpace(se.Name),
		Description: strings.TrimSpace(se.Description),
		Tags:        tags,
		Category:    category,
		Example:     example,
		Body:        truncateForIndex(body, 400),
		SourcePath:  se.Location,
	}
}

func parseSkillFrontmatterExtras(frontmatter string) ([]string, string) {
	if strings.TrimSpace(frontmatter) == "" {
		return nil, ""
	}
	var tags []string
	category := ""
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		value := strings.TrimSpace(line[idx+1:])
		switch key {
		case "tags":
			tags = parseSkillOSList(value)
			if len(tags) == 0 {
				for _, t := range strings.Split(value, ",") {
					t = strings.TrimSpace(strings.Trim(t, `"'[]`))
					if t != "" {
						tags = append(tags, t)
					}
				}
			}
		case "category":
			category = strings.Trim(value, `"'`)
		}
	}
	return tags, category
}

func extractSkillExample(body, skillName string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "blue ") || strings.HasPrefix(trimmed, skillName+" ") {
			return truncateForIndex(trimmed, 180)
		}
		if strings.Contains(trimmed, "`blue ") || strings.Contains(trimmed, "`"+skillName+" ") {
			trimmed = strings.Trim(trimmed, "`")
			return truncateForIndex(trimmed, 180)
		}
	}
	return ""
}

func truncateForIndex(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max-3]) + "..."
}
