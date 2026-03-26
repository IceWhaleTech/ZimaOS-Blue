package agentcore

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SkillDoc is a compact, selector-friendly representation of a skill.
type SkillDoc struct {
	Name         string
	Description  string
	Tags         []string
	Category     string
	Environment  []string
	Example      string
	Setup        string
	ScriptPaths  []string
	InstallSteps []string
	UsageSteps   []string
	TaskRoutes   []SkillRoute
	ErrorRules   []SkillError
	Body         string
	SourcePath   string
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
			se, data, err := readSkillEntryFromDir(filepath.Join(root, skillID))
			if err != nil {
				continue
			}
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
	body := content
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			body = strings.TrimSpace(parts[2])
		}
	}

	example := strings.TrimSpace(se.Example)
	if example == "" {
		example = extractSkillExample(body, se.Name)
	}
	if se.Description == "" {
		se.Description = extractFirstParagraph(body)
	}

	return SkillDoc{
		Name:         strings.TrimSpace(se.Name),
		Description:  strings.TrimSpace(se.Description),
		Tags:         append([]string(nil), se.Tags...),
		Category:     strings.TrimSpace(se.Category),
		Environment:  append([]string(nil), se.Environment...),
		Example:      truncateForIndex(example, 180),
		Setup:        se.Setup,
		ScriptPaths:  append([]string(nil), se.ScriptPaths...),
		InstallSteps: append([]string(nil), se.InstallSteps...),
		UsageSteps:   append([]string(nil), se.UsageSteps...),
		TaskRoutes:   append([]SkillRoute(nil), se.TaskRoutes...),
		ErrorRules:   append([]SkillError(nil), se.ErrorRules...),
		Body:         truncateForIndex(body, 400),
		SourcePath:   se.Location,
	}
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
