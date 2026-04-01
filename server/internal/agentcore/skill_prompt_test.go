package agentcore

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestScanSkillsDir_Empty(t *testing.T) {
	dir := t.TempDir()
	skills := ScanSkillsDir(dir)
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestScanSkillsDir_NonExistent(t *testing.T) {
	skills := ScanSkillsDir("/nonexistent/path")
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(skills))
	}
}

func TestScanSkillsDir_WithFrontmatter(t *testing.T) {
	dir := t.TempDir()

	// Create a skill with frontmatter
	skillDir := filepath.Join(dir, "weather")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: weather
description: Get current weather for any location
---
# Weather

Detailed instructions here.
`), 0o644)

	skills := ScanSkillsDir(dir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "weather" {
		t.Errorf("expected name 'weather', got %q", skills[0].Name)
	}
	if skills[0].Description != "Get current weather for any location" {
		t.Errorf("expected description from frontmatter, got %q", skills[0].Description)
	}
}

func TestScanSkillsDir_WithoutFrontmatter(t *testing.T) {
	dir := t.TempDir()

	// Create a skill without frontmatter
	skillDir := filepath.Join(dir, "calculator")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`# Calculator

Performs basic arithmetic calculations.

## Usage
...
`), 0o644)

	skills := ScanSkillsDir(dir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "calculator" {
		t.Errorf("expected name 'calculator', got %q", skills[0].Name)
	}
	if skills[0].Description != "Performs basic arithmetic calculations." {
		t.Errorf("expected description from first paragraph, got %q", skills[0].Description)
	}
}

func TestScanSkillsDir_UsesCompatibilityEntryDocument(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "browser")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "CLAUDE.md"), []byte(`---
name: browser
description: Browser skill loaded from CLAUDE.md
---
# Browser
`), 0o644)

	skills := ScanSkillsDir(dir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Description != "Browser skill loaded from CLAUDE.md" {
		t.Fatalf("expected CLAUDE.md description, got %q", skills[0].Description)
	}
}

func TestScanSkillsDir_DisabledSkill(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "disabled_skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: disabled_skill
description: This skill is disabled
enabled: false
---
# Disabled
`), 0o644)

	skills := ScanSkillsDir(dir)
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills (disabled), got %d", len(skills))
	}
}

func TestScanSkillsDir_PlatformFilter(t *testing.T) {
	dir := t.TempDir()

	// Create a skill for a different platform
	otherOS := "windows"
	if runtime.GOOS == "windows" {
		otherOS = "linux"
	}

	skillDir := filepath.Join(dir, "platform_skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: platform_skill
description: Only for another platform
os: ["`+otherOS+`"]
---
# Platform Skill
`), 0o644)

	skills := ScanSkillsDir(dir)
	if len(skills) != 0 {
		t.Fatalf("expected 0 skills (wrong platform), got %d", len(skills))
	}
}

func TestScanSkillsDir_CurrentPlatform(t *testing.T) {
	dir := t.TempDir()

	skillDir := filepath.Join(dir, "native_skill")
	os.MkdirAll(skillDir, 0o755)
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: native_skill
description: For current platform
os: ["`+runtime.GOOS+`"]
---
# Native
`), 0o644)

	skills := ScanSkillsDir(dir)
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill (current platform), got %d", len(skills))
	}
}

func TestScanSkillsDir_SortedByName(t *testing.T) {
	dir := t.TempDir()

	for _, name := range []string{"zebra", "alpha", "middle"} {
		skillDir := filepath.Join(dir, name)
		os.MkdirAll(skillDir, 0o755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name+"\n\nDescription of "+name+"."), 0o644)
	}

	skills := ScanSkillsDir(dir)
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}
	if skills[0].Name != "alpha" || skills[1].Name != "middle" || skills[2].Name != "zebra" {
		t.Errorf("expected sorted order, got %s, %s, %s", skills[0].Name, skills[1].Name, skills[2].Name)
	}
}

func TestScanSkillsDir_PrioritySort(t *testing.T) {
	dir := t.TempDir()

	// browser and web-search should sort before alphabetical skills
	for _, name := range []string{"weather", "browser", "calculator"} {
		skillDir := filepath.Join(dir, name)
		os.MkdirAll(skillDir, 0o755)
		os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+name+"\n\nDescription of "+name+"."), 0o644)
	}

	skills := ScanSkillsDir(dir)
	if len(skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(skills))
	}
	// browser (priority 0) should be first, then calculator and weather (both priority 100, alphabetical)
	if skills[0].Name != "browser" {
		t.Errorf("expected browser first, got %q", skills[0].Name)
	}
	if skills[1].Name != "calculator" {
		t.Errorf("expected calculator second, got %q", skills[1].Name)
	}
	if skills[2].Name != "weather" {
		t.Errorf("expected weather third, got %q", skills[2].Name)
	}
}

func TestFormatSkillsPrompt_Empty(t *testing.T) {
	result := FormatSkillsPrompt(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatSkillsPrompt_XMLOutput(t *testing.T) {
	skills := []SkillEntry{
		{Name: "weather", Description: "Get weather info", Location: "/path/to/weather/SKILL.md"},
		{Name: "calc", Description: "Basic math", Location: "/path/to/calc/SKILL.md"},
	}

	result := FormatSkillsPrompt(skills)

	// Check compact XML attribute format
	if !contains(result, "<available_skills>") {
		t.Error("missing <available_skills> tag")
	}
	if !contains(result, "</available_skills>") {
		t.Error("missing </available_skills> tag")
	}
	if !contains(result, `name="weather"`) {
		t.Error("missing weather skill name")
	}
	if !contains(result, `desc="Get weather info"`) {
		t.Error("missing weather description")
	}
	if !contains(result, `cmd="weather"`) {
		t.Error("missing weather cmd")
	}
	if !contains(result, `name="calc"`) {
		t.Error("missing calc skill name")
	}
}

func TestFormatSkillsPrompt_XMLEscape(t *testing.T) {
	skills := []SkillEntry{
		{Name: "test & <skill>", Description: `Use "quotes"`, Location: "/path/SKILL.md"},
	}

	result := FormatSkillsPrompt(skills)

	// Compact format uses %q which Go-escapes quotes, and xmlEscape handles cmd attr
	if !contains(result, "test \\u0026 \\u003cskill\\u003e") && !contains(result, `test & <skill>`) {
		// %q escapes & and < as unicode escapes
		t.Errorf("XML escaping failed for name, got: %s", result)
	}
	if !contains(result, `cmd="test &amp; &lt;skill&gt;"`) {
		t.Errorf("XML escaping failed for cmd, got: %s", result)
	}
}

func TestExtractFirstParagraph(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"heading then paragraph", "# Title\n\nFirst paragraph.", "First paragraph."},
		{"no heading", "Just text.", ""},
		{"long description", "# Title\n\n" + string(make([]byte, 250)), ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractFirstParagraph(tt.content)
			if tt.name == "long description" {
				if len(got) > 200 {
					t.Errorf("expected truncated description, got length %d", len(got))
				}
			} else if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestParseSkillOSList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{`["darwin", "linux"]`, []string{"darwin", "linux"}},
		{`["darwin"]`, []string{"darwin"}},
		{"darwin", []string{"darwin"}},
		{"", nil},
	}

	for _, tt := range tests {
		got := parseSkillOSList(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("parseSkillOSList(%q): expected %v, got %v", tt.input, tt.expected, got)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("parseSkillOSList(%q)[%d]: expected %q, got %q", tt.input, i, tt.expected[i], got[i])
			}
		}
	}
}

func TestPinnedSkills_ContainsCoreRoutedSkillSet(t *testing.T) {
	got := PinnedSkills()
	gotSet := make(map[string]struct{}, len(got))
	for _, id := range got {
		gotSet[id] = struct{}{}
	}

	required := []string{
		"reminder",
		"scheduler",
	}

	for _, id := range required {
		if _, ok := gotSet[id]; !ok {
			t.Fatalf("PinnedSkills missing %q; got=%v", id, got)
		}
	}

	planIDs := []string{"plan_create", "plan_update", "plan_append"}
	for _, id := range planIDs {
		if _, ok := gotSet[id]; ok {
			t.Fatalf("PinnedSkills should not pin %q by default; got=%v", id, got)
		}
	}
}

func TestFormatPinnedSkills_FallbackToHomeDefaultDir(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	homeSkillDir := filepath.Join(homeDir, ".claude", "skills", "browser")
	if err := os.MkdirAll(homeSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir home skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from home default path
---
# browser
`), 0o644); err != nil {
		t.Fatalf("write home SKILL.md: %v", err)
	}

	got := FormatPinnedSkills(workspaceDir)
	if !contains(got, `<pinned_skills>`) {
		t.Fatalf("expected pinned_skills output, got: %q", got)
	}
	if !contains(got, `name="browser"`) {
		t.Fatalf("expected browser skill from home path, got: %q", got)
	}
	if !contains(got, `desc="Browser from home default path"`) {
		t.Fatalf("expected home description, got: %q", got)
	}
}

func TestFormatPinnedSkills_WorkspaceOverridesHome(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	workspaceSkillDir := filepath.Join(workspaceDir, ".claude", "skills", "browser")
	if err := os.MkdirAll(workspaceSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceSkillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from workspace path
---
# browser
`), 0o644); err != nil {
		t.Fatalf("write workspace SKILL.md: %v", err)
	}

	homeSkillDir := filepath.Join(homeDir, ".claude", "skills", "browser")
	if err := os.MkdirAll(homeSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir home skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from home path
---
# browser
`), 0o644); err != nil {
		t.Fatalf("write home SKILL.md: %v", err)
	}

	got := FormatPinnedSkills(workspaceDir)
	if !contains(got, `desc="Browser from workspace path"`) {
		t.Fatalf("expected workspace description to win, got: %q", got)
	}
	if contains(got, `desc="Browser from home path"`) {
		t.Fatalf("did not expect home description when workspace exists, got: %q", got)
	}
}

func TestFormatPinnedSkills_AgentsRootOverridesClaudeRoot(t *testing.T) {
	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	agentsSkillDir := filepath.Join(workspaceDir, ".agents", "skills", "browser")
	if err := os.MkdirAll(agentsSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir agents skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentsSkillDir, "CLAUDE.md"), []byte(`---
name: browser
description: Browser from agents path
---
# browser
`), 0o644); err != nil {
		t.Fatalf("write agents CLAUDE.md: %v", err)
	}

	claudeSkillDir := filepath.Join(workspaceDir, ".claude", "skills", "browser")
	if err := os.MkdirAll(claudeSkillDir, 0o755); err != nil {
		t.Fatalf("mkdir claude skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeSkillDir, "SKILL.md"), []byte(`---
name: browser
description: Browser from claude path
---
# browser
`), 0o644); err != nil {
		t.Fatalf("write claude SKILL.md: %v", err)
	}

	got := FormatPinnedSkills(workspaceDir)
	if !contains(got, `desc="Browser from agents path"`) {
		t.Fatalf("expected agents description to win, got: %q", got)
	}
	if contains(got, `desc="Browser from claude path"`) {
		t.Fatalf("did not expect claude description when agents path exists, got: %q", got)
	}
}

func TestFormatPinnedSkills_FallsBackToEmbeddedBuiltins(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	got := FormatPinnedSkills("")
	if !contains(got, `<pinned_skills>`) {
		t.Fatalf("expected pinned_skills output, got: %q", got)
	}
	if !contains(got, `name="ask"`) {
		t.Fatalf("expected embedded ask skill fallback, got: %q", got)
	}
}

func TestParseSkillEntry_StructuredSections(t *testing.T) {
	md := `---
name: youtube_video_analyzer
description: "Analyze YouTube videos"
tags: ["youtube", "video", "sentiment"]
category: "media"
environment: ["python3"]
---

# YouTube Video Analyzer Skill

## Setup

No external dependencies required.

## Available Scripts

| Script | Purpose |
|--------|---------|
| scripts/fetch_transcript.py | Download transcripts |
| scripts/fetch_comments.py | Download comments |
| scripts/analyze_video.py | Analyze transcript + comments |

## Task Routing

| User Intent | Action |
|-------------|--------|
| Summarize / analyze video content | Run scripts/analyze_video.py <video> |
| Download comments file | Run scripts/fetch_comments.py <video> --save |

## Script Usage

### scripts/analyze_video.py

` + "```bash" + `
python scripts/analyze_video.py <video_id_or_url>
python scripts/analyze_video.py <video_id_or_url> --mode comments-only
` + "```" + `

## Error Handling

| Error | Resolution |
|-------|------------|
| Transcript unavailable | Inform user and continue comments analysis |
| Video unavailable | Suggest checking URL |
`

	se := parseSkillEntry("youtube_video_analyzer", "/tmp/SKILL.md", []byte(md))

	if se.Name != "youtube_video_analyzer" {
		t.Fatalf("expected parsed name, got %q", se.Name)
	}
	if se.Category != "media" {
		t.Fatalf("expected category media, got %q", se.Category)
	}
	if len(se.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %v", se.Tags)
	}
	if len(se.Environment) != 1 || se.Environment[0] != "python3" {
		t.Fatalf("expected parsed environment, got %v", se.Environment)
	}
	if !strings.Contains(se.Setup, "No external dependencies required") {
		t.Fatalf("expected setup section parsed, got %q", se.Setup)
	}
	if len(se.ScriptPaths) < 3 {
		t.Fatalf("expected script paths parsed, got %v", se.ScriptPaths)
	}
	if len(se.UsageSteps) < 2 {
		t.Fatalf("expected usage steps parsed, got %v", se.UsageSteps)
	}
	if len(se.TaskRoutes) != 2 {
		t.Fatalf("expected task routes parsed, got %v", se.TaskRoutes)
	}
	if len(se.ErrorRules) != 2 {
		t.Fatalf("expected error rules parsed, got %v", se.ErrorRules)
	}
	if se.Example == "" {
		t.Fatalf("expected example command inferred from usage")
	}
}

func TestParseFrontmatterFields_MultilineLists(t *testing.T) {
	md := `---
name: test_skill
os:
  - linux
  - darwin
tags:
  - ai
  - ir
env:
  - local
  - onnx
---

# Test Skill

## Script Usage

` + "```bash" + `
python scripts/run.py --mode test
` + "```" + `
`

	se := parseSkillEntry("test_skill", "/tmp/SKILL.md", []byte(md))
	if len(se.OS) != 2 {
		t.Fatalf("expected 2 os values, got %v", se.OS)
	}
	if len(se.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", se.Tags)
	}
	if len(se.Environment) != 2 {
		t.Fatalf("expected 2 env values, got %v", se.Environment)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
