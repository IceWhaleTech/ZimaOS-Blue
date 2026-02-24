package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// SkillEntry holds parsed metadata from a SKILL.md file for prompt injection.
type SkillEntry struct {
	Name        string
	Description string
	Location    string // absolute path to SKILL.md
	Enabled     bool
	OS          []string // platform filter (empty = all platforms)
}

// ScanSkillsDir scans a skills directory for SKILL.md files and returns
// enabled, platform-matching entries sorted by name.
// Directory structure: {skillsDir}/{skill-name}/SKILL.md
func ScanSkillsDir(skillsDir string) []SkillEntry {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return nil
	}

	var skills []SkillEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		mdPath := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}

		se := parseSkillEntry(entry.Name(), mdPath, data)

		// Skip disabled skills
		if !se.Enabled {
			continue
		}

		// Skip platform-mismatched skills
		if !skillPlatformMatch(se.OS) {
			continue
		}

		skills = append(skills, se)
	}

	sort.Slice(skills, func(i, j int) bool {
		pi, pj := skillSortPriority(skills[i].Name), skillSortPriority(skills[j].Name)
		if pi != pj {
			return pi < pj
		}
		return skills[i].Name < skills[j].Name
	})

	return skills
}

// parseSkillEntry extracts name, description, enabled, and os from a SKILL.md file.
func parseSkillEntry(dirName, mdPath string, data []byte) SkillEntry {
	se := SkillEntry{
		Name:     dirName,
		Location: mdPath,
		Enabled:  true, // default enabled
	}

	content := string(data)

	// Parse YAML frontmatter if present
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			parseFrontmatterFields(parts[1], &se)
			content = strings.TrimSpace(parts[2])
		}
	}

	// Extract description from first paragraph if not in frontmatter
	if se.Description == "" {
		se.Description = extractFirstParagraph(content)
	}

	// Use directory name as name if frontmatter didn't provide one
	if se.Name == "" {
		se.Name = dirName
	}

	return se
}

// parseFrontmatterFields extracts relevant fields from YAML frontmatter.
func parseFrontmatterFields(frontmatter string, se *SkillEntry) {
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "name":
			se.Name = strings.Trim(value, `"'`)
		case "description":
			se.Description = strings.Trim(value, `"'`)
		case "enabled":
			v := strings.ToLower(strings.Trim(value, `"'`))
			se.Enabled = v != "false" && v != "no" && v != "0"
		case "os":
			se.OS = parseSkillOSList(value)
		}
	}
}

// extractFirstParagraph extracts the first non-heading paragraph from markdown.
// Looks for the first line after a heading that contains text.
func extractFirstParagraph(content string) string {
	lines := strings.Split(content, "\n")
	pastHeading := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			pastHeading = true
			continue
		}
		if pastHeading {
			// Truncate long descriptions
			if len(trimmed) > 200 {
				return trimmed[:197] + "..."
			}
			return trimmed
		}
	}
	return ""
}

// parseSkillOSList parses os field: `["darwin"]`, `["darwin","linux"]`, or `darwin`
func parseSkillOSList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// JSON array format: ["darwin", "linux"]
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := value[1 : len(value)-1]
		var result []string
		for _, item := range strings.Split(inner, ",") {
			item = strings.TrimSpace(item)
			item = strings.Trim(item, `"'`)
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	}
	// Single value
	return []string{strings.Trim(value, `"'`)}
}

// skillPlatformMatch returns true if the current OS matches the skill's os list.
// Empty list means all platforms.
func skillPlatformMatch(osList []string) bool {
	if len(osList) == 0 {
		return true
	}
	for _, os := range osList {
		if os == runtime.GOOS {
			return true
		}
	}
	return false
}

// skillSortPriority returns sort priority for a skill name.
// Lower = appears first. Core skills (browser, web search) are pinned to the top.
var skillPriorityMap = map[string]int{
	"browser":    0,
	"web_search": 1,
	"web-search": 1,
	"websearch":  1,
}

func skillSortPriority(name string) int {
	if p, ok := skillPriorityMap[strings.ToLower(name)]; ok {
		return p
	}
	return 100 // default: alphabetical after pinned skills
}

// FormatSkillsPrompt generates the XML skill index for system prompt injection.
// Returns empty string if no skills are available.
func FormatSkillsPrompt(skills []SkillEntry) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, s := range skills {
		sb.WriteString("  <skill>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", xmlEscape(s.Name)))
		if s.Description != "" {
			sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", xmlEscape(s.Description)))
		}
		sb.WriteString(fmt.Sprintf("    <location>%s</location>\n", xmlEscape(s.Location)))
		sb.WriteString("  </skill>\n")
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
