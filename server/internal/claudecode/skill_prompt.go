package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

var skillScriptPathRegexp = regexp.MustCompile(`(?i)(?:^|[\s` + "`" + `\(\[])((?:\./)?scripts/[a-z0-9._/\-]+)`)

// SkillRoute maps a user intent to the action/command in a skill.
type SkillRoute struct {
	Intent string
	Action string
}

// SkillError maps an error case to suggested resolution in a skill.
type SkillError struct {
	Error      string
	Resolution string
}

// SkillEntry holds parsed metadata from a SKILL.md file for prompt injection.
type SkillEntry struct {
	Name         string
	Description  string
	Location     string // absolute path to SKILL.md
	Enabled      bool
	OS           []string // platform filter (empty = all platforms)
	Environment  []string
	Tags         []string
	Category     string
	Setup        string
	ScriptPaths  []string
	InstallSteps []string
	UsageSteps   []string
	TaskRoutes   []SkillRoute
	ErrorRules   []SkillError
	Example      string
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

// parseSkillEntry extracts metadata and structured fields from a SKILL.md file.
func parseSkillEntry(dirName, mdPath string, data []byte) SkillEntry {
	se := SkillEntry{
		Name:     dirName,
		Location: mdPath,
		Enabled:  true, // default enabled
	}

	content := string(data)
	body := strings.TrimSpace(content)

	// Parse YAML frontmatter if present
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			parseFrontmatterFields(parts[1], &se)
			body = strings.TrimSpace(parts[2])
		}
	}

	parseSkillBodyFields(body, &se)

	// Extract description from first paragraph if not in frontmatter
	if se.Description == "" {
		se.Description = extractFirstParagraph(body)
	}

	// Use directory name as name if frontmatter didn't provide one
	if se.Name == "" {
		se.Name = dirName
	}

	return se
}

// parseFrontmatterFields extracts relevant fields from YAML frontmatter.
func parseFrontmatterFields(frontmatter string, se *SkillEntry) {
	currentListKey := ""
	for _, rawLine := range strings.Split(frontmatter, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "-") && currentListKey != "" {
			item := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			item = strings.Trim(item, `"'`)
			if item != "" {
				appendFrontmatterList(currentListKey, []string{item}, se)
			}
			continue
		}

		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			currentListKey = ""
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		value := strings.TrimSpace(line[colonIdx+1:])
		if value == "" {
			currentListKey = key
			continue
		}
		currentListKey = ""

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
		case "tags":
			se.Tags = parseSkillStringList(value)
		case "category":
			se.Category = strings.Trim(value, `"'`)
		case "env", "environment", "applicable_env", "applicable-environment":
			se.Environment = parseSkillStringList(value)
		}
	}
}

func appendFrontmatterList(key string, values []string, se *SkillEntry) {
	switch key {
	case "os":
		se.OS = appendUnique(se.OS, values...)
	case "tags":
		se.Tags = appendUnique(se.Tags, values...)
	case "env", "environment", "applicable_env", "applicable-environment":
		se.Environment = appendUnique(se.Environment, values...)
	}
}

func parseSkillStringList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// JSON-like array format: ["a", "b"]
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		inner := strings.TrimSpace(value[1 : len(value)-1])
		if inner == "" {
			return nil
		}
		var result []string
		for _, item := range strings.Split(inner, ",") {
			item = strings.TrimSpace(item)
			item = strings.Trim(item, `"'`)
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, item)
			}
		}
		return result
	}
	// CSV-like one-liner: a, b
	if strings.Contains(value, ",") {
		var result []string
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			item = strings.Trim(item, `"'`)
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, item)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	// Single value
	v := strings.Trim(value, `"'`)
	if v == "" {
		return nil
	}
	return []string{v}
}

func parseSkillBodyFields(body string, se *SkillEntry) {
	sections := splitSkillSections(body)

	setup := firstSectionContent(sections,
		"setup", "installation", "install", "环境准备", "安装", "前置条件", "准备")
	if setup != "" {
		se.Setup = truncateForIndex(stripMarkdownTableRows(setup), 320)
		se.InstallSteps = extractCommands(setup)
	}

	availableScripts := firstSectionContent(sections,
		"available scripts", "scripts", "脚本", "可用脚本")
	scriptUsage := firstSectionContent(sections,
		"script usage", "command usage", "usage", "命令用法", "脚本用法", "使用脚本", "用法")
	taskRouting := firstSectionContent(sections,
		"task routing", "routing", "intent routing", "任务路由", "任务分流", "路由")
	errorHandling := firstSectionContent(sections,
		"error handling", "errors", "错误处理", "故障处理")

	se.ScriptPaths = appendUnique(se.ScriptPaths,
		extractScriptPaths(availableScripts)...,
	)
	se.ScriptPaths = appendUnique(se.ScriptPaths,
		extractScriptPaths(scriptUsage)...,
	)
	// Global fallback if section headings vary.
	se.ScriptPaths = appendUnique(se.ScriptPaths,
		extractScriptPaths(body)...,
	)

	if scriptUsage != "" {
		se.UsageSteps = appendUnique(se.UsageSteps, extractCommands(scriptUsage)...)
	} else {
		// Fallback for skills that only provide "Command Usage" as fenced blocks
		// without a stable heading.
		se.UsageSteps = appendUnique(se.UsageSteps, extractCommands(body)...)
	}

	se.TaskRoutes = append(se.TaskRoutes, parseTaskRoutes(taskRouting)...)
	se.ErrorRules = append(se.ErrorRules, parseErrorRules(errorHandling)...)

	if se.Example == "" {
		if len(se.UsageSteps) > 0 {
			se.Example = truncateForIndex(se.UsageSteps[0], 180)
		} else {
			se.Example = extractSkillExample(body, se.Name)
		}
	}
}

func splitSkillSections(body string) map[string]string {
	sections := make(map[string]string)
	lines := strings.Split(body, "\n")
	currentTitle := ""
	var current []string

	flush := func() {
		if currentTitle == "" {
			return
		}
		joined := strings.TrimSpace(strings.Join(current, "\n"))
		if joined != "" {
			sections[currentTitle] = joined
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			flush()
			currentTitle = normalizeSkillHeading(strings.TrimSpace(strings.TrimPrefix(trimmed, "## ")))
			current = current[:0]
			continue
		}
		if currentTitle != "" {
			current = append(current, line)
		}
	}
	flush()
	return sections
}

func normalizeSkillHeading(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	title = strings.Trim(title, ":-_[]()")
	title = strings.Join(strings.Fields(title), " ")
	return title
}

func firstSectionContent(sections map[string]string, aliases ...string) string {
	for _, a := range aliases {
		if v, ok := sections[normalizeSkillHeading(a)]; ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func stripMarkdownTableRows(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			continue
		}
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractScriptPaths(content string) []string {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	matches := skillScriptPathRegexp.FindAllStringSubmatch(content, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		p := strings.TrimSpace(m[1])
		p = strings.TrimRight(p, `"'.,;:)]}`)
		if p != "" {
			out = append(out, p)
		}
	}
	return appendUnique(nil, out...)
}

func extractCommands(content string) []string {
	lines := strings.Split(content, "\n")
	inFence := false
	var commands []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "$ ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "$ "))
		}
		commands = append(commands, truncateForIndex(trimmed, 220))
	}
	return appendUnique(nil, commands...)
}

func parseTaskRoutes(section string) []SkillRoute {
	rows := parseMarkdownTable(section)
	if len(rows) < 2 {
		return nil
	}
	header := normalizeTableHeaderMap(rows[0])
	intentIdx := findTableColumn(header, "user intent", "intent", "用户意图", "意图", "需求")
	actionIdx := findTableColumn(header, "action", "command", "操作", "执行", "动作")
	if intentIdx < 0 || actionIdx < 0 {
		return nil
	}

	var out []SkillRoute
	for _, row := range rows[1:] {
		if intentIdx >= len(row) || actionIdx >= len(row) {
			continue
		}
		intent := strings.TrimSpace(row[intentIdx])
		action := strings.TrimSpace(row[actionIdx])
		if intent == "" || action == "" {
			continue
		}
		out = append(out, SkillRoute{Intent: intent, Action: action})
	}
	return out
}

func parseErrorRules(section string) []SkillError {
	rows := parseMarkdownTable(section)
	if len(rows) < 2 {
		return nil
	}
	header := normalizeTableHeaderMap(rows[0])
	errorIdx := findTableColumn(header, "error", "issue", "problem", "错误")
	resolutionIdx := findTableColumn(header, "resolution", "handling", "fix", "解决", "处理")
	if errorIdx < 0 || resolutionIdx < 0 {
		return nil
	}

	var out []SkillError
	for _, row := range rows[1:] {
		if errorIdx >= len(row) || resolutionIdx >= len(row) {
			continue
		}
		errText := strings.TrimSpace(row[errorIdx])
		resText := strings.TrimSpace(row[resolutionIdx])
		if errText == "" || resText == "" {
			continue
		}
		out = append(out, SkillError{Error: errText, Resolution: resText})
	}
	return out
}

func parseMarkdownTable(section string) [][]string {
	if strings.TrimSpace(section) == "" {
		return nil
	}
	lines := strings.Split(section, "\n")
	rows := make([][]string, 0, 8)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
			continue
		}
		row := splitMarkdownTableRow(trimmed)
		if len(row) == 0 {
			continue
		}
		if isMarkdownTableSeparator(row) {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func splitMarkdownTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")

	var (
		cells []string
		buf   strings.Builder
		esc   bool
	)
	for _, r := range line {
		if esc {
			buf.WriteRune(r)
			esc = false
			continue
		}
		if r == '\\' {
			esc = true
			continue
		}
		if r == '|' {
			cells = append(cells, strings.TrimSpace(buf.String()))
			buf.Reset()
			continue
		}
		buf.WriteRune(r)
	}
	cells = append(cells, strings.TrimSpace(buf.String()))
	return cells
}

func isMarkdownTableSeparator(row []string) bool {
	if len(row) == 0 {
		return false
	}
	for _, c := range row {
		c = strings.TrimSpace(c)
		c = strings.Trim(c, ":")
		if c == "" {
			continue
		}
		for _, r := range c {
			if r != '-' {
				return false
			}
		}
	}
	return true
}

func normalizeTableHeaderMap(header []string) map[string]int {
	out := make(map[string]int, len(header))
	for i, h := range header {
		out[normalizeSkillHeading(h)] = i
	}
	return out
}

func findTableColumn(header map[string]int, aliases ...string) int {
	for _, a := range aliases {
		if idx, ok := header[normalizeSkillHeading(a)]; ok {
			return idx
		}
	}
	return -1
}

func appendUnique(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, v := range dst {
		key := strings.ToLower(strings.TrimSpace(v))
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		dst = append(dst, v)
	}
	return dst
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
	return parseSkillStringList(value)
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
	"browser":       0,
	"web_search":    1,
	"web-search":    1,
	"websearch":     1,
	"deep_research": 2,
}

func skillSortPriority(name string) int {
	if p, ok := skillPriorityMap[strings.ToLower(name)]; ok {
		return p
	}
	return 100 // default: alphabetical after pinned skills
}

// FormatSkillsPrompt generates the XML skill index for system prompt injection.
// Uses compact attribute format to minimize token usage.
// Returns empty string if no skills are available.
func FormatSkillsPrompt(skills []SkillEntry) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, s := range skills {
		desc := s.Description
		if desc == "" {
			desc = s.Name
		}
		sb.WriteString(fmt.Sprintf("  <skill name=%q desc=%q cmd=%q />\n",
			s.Name, desc, xmlEscape(s.Name)))
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}

// pinnedSkills lists the skill names that are always shown in the system prompt.
// Only these get their description injected — everything else the LLM discovers
// by reading .claude/skills/<name>/SKILL.md on demand.
var pinnedSkills = []string{
	"ask",
	"browser",
	"web_search",
	"deep_research",
	"analyze",
	"ui_reviewer",
	"mgmt",
	"mediagen",
}

// PinnedSkills returns the list of pinned skill names for short-circuit handling.
func PinnedSkills() []string {
	return pinnedSkills
}

// FormatPinnedSkills reads only the pinned skills from disk and formats them
// as compact XML for the system prompt. Returns empty string if none found.
func FormatPinnedSkills(workspaceDir string) string {
	skillRoots := resolveSkillRoots(workspaceDir)
	if len(skillRoots) == 0 {
		return ""
	}

	foundByName := make(map[string]SkillEntry, len(pinnedSkills))
	for _, root := range skillRoots {
		for _, name := range pinnedSkills {
			// First hit wins: workspace skill overrides default ~/.claude/skills.
			if _, exists := foundByName[name]; exists {
				continue
			}
			mdPath := filepath.Join(root, name, "SKILL.md")
			data, err := os.ReadFile(mdPath)
			if err != nil {
				continue
			}
			se := parseSkillEntry(name, mdPath, data)
			if !se.Enabled || !skillPlatformMatch(se.OS) {
				continue
			}
			foundByName[name] = se
		}
	}

	var found []SkillEntry
	for _, name := range pinnedSkills {
		if se, ok := foundByName[name]; ok {
			found = append(found, se)
		}
	}

	if len(found) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<pinned_skills>\n")
	for _, s := range found {
		desc := s.Description
		if desc == "" {
			desc = s.Name
		}
		sb.WriteString(fmt.Sprintf("  <skill name=%q desc=%q cmd=%q />\n",
			s.Name, desc, xmlEscape(s.Name)))
	}
	sb.WriteString("</pinned_skills>")
	return sb.String()
}

// resolveSkillRoots returns skill roots in priority order:
// 1) workspace/.claude/skills (project-level)
// 2) ~/.claude/skills (user default)
func resolveSkillRoots(workspaceDir string) []string {
	var roots []string
	seen := map[string]struct{}{}

	add := func(dir string) {
		if dir == "" {
			return
		}
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		roots = append(roots, dir)
	}

	if workspaceDir != "" {
		add(filepath.Join(workspaceDir, ".claude", "skills"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		add(filepath.Join(home, ".claude", "skills"))
	}
	return roots
}

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
