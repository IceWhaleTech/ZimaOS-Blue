package skillmarket

import "testing"

func TestParseSkillMarkdownIgnoresVersionLikeCategory(t *testing.T) {
	raw := `---
id: git-expert
name: Git Expert
version: 1.2.3
category: 2.0.0
tags: [git, github]
---

# Git Expert

Review branches, inspect pull requests, and fix rebases.
`

	parsed, err := parseSkillMarkdown(raw, "git-expert")
	if err != nil {
		t.Fatalf("parseSkillMarkdown() error = %v", err)
	}
	if got := parsed.Manifest.Category; got != "development_tools" {
		t.Fatalf("category = %q, want development_tools", got)
	}
}

func TestNormalizeMarketplaceCategoryUsesBusinessTaxonomy(t *testing.T) {
	tests := map[string]string{
		"AI 智能":          "ai_intelligence",
		"开发工具":            "development_tools",
		"developer-tools": "development_tools",
		"developer_tools": "development_tools",
		"效率提升":            "productivity",
		"数据分析":            "data_analysis",
		"内容创作":            "content_creation",
		"安全合规":            "security_compliance",
		"通讯协作":            "communication_collaboration",
	}

	for raw, want := range tests {
		if got := NormalizeMarketplaceCategory(raw, "", nil); got != want {
			t.Fatalf("NormalizeMarketplaceCategory(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseSkillMarkdownRejectsHTMLContent(t *testing.T) {
	raw := `<!doctype html><html><head><title>Not a skill</title></head><body>oops</body></html>`

	if _, err := parseSkillMarkdown(raw, "html-skill"); err == nil {
		t.Fatalf("expected html content to be rejected")
	}
}
