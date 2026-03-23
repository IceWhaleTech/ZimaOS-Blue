package tools

import (
	"regexp"
	"strings"

	sel "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
)

var structuredWorkspaceArtifactPathRegex = regexp.MustCompile("`([^`]+\\.(?:md|txt|json|csv|tsv|html|pdf|docx?|xlsx?|pptx?|png|jpe?g|webp|gif))`|\\b([A-Za-z0-9._/\\-]+\\.(?:md|txt|json|csv|tsv|html|pdf|docx?|xlsx?|pptx?|png|jpe?g|webp|gif))\\b")

// LooksLikeStructuredWorkspaceArtifactTask reports whether the user is asking
// the model to read one or more concrete local source files and produce a
// structured saved artifact such as extracted answers, a summary, or a table.
func LooksLikeStructuredWorkspaceArtifactTask(query string) bool {
	return looksLikeStructuredWorkspaceArtifactTask(query, sel.AnalyzeQuery(query))
}

// StructuredWorkspaceArtifactWorkflowToolNames returns the compact local tool
// workflow preferred for structured workspace artifact tasks.
func StructuredWorkspaceArtifactWorkflowToolNames(query string) []string {
	names := []string{
		"file_read",
		"file_write",
		"ls",
		"find",
		"convert",
	}
	lower := strings.ToLower(strings.TrimSpace(query))
	if mentionsStructuredWorkspaceArtifactExt(lower, ".pdf") {
		names = append(names, "pdf")
	}
	if mentionsStructuredWorkspaceArtifactExt(lower, ".xlsx", ".docx") {
		names = append(names, "office")
	}
	if mentionsStructuredWorkspaceArtifactExt(lower, ".png", ".jpg", ".jpeg", ".webp", ".gif") {
		names = append(names, "image")
	}
	return names
}

func looksLikeStructuredWorkspaceArtifactTask(query string, signals sel.QueryIntentSignals) bool {
	lower := strings.ToLower(strings.TrimSpace(query))
	if lower == "" || !signals.LocalWorkspace || signals.LiveWeb {
		return false
	}
	if countStructuredWorkspaceArtifactPaths(lower) < 2 {
		return false
	}
	if !hasStructuredWorkspaceArtifactWriteCue(lower) {
		return false
	}
	return hasStructuredWorkspaceArtifactExtractionCue(lower)
}

func countStructuredWorkspaceArtifactPaths(text string) int {
	matches := structuredWorkspaceArtifactPathRegex.FindAllStringSubmatch(text, -1)
	count := 0
	for _, match := range matches {
		for _, group := range match[1:] {
			if strings.TrimSpace(group) != "" {
				count++
			}
		}
	}
	return count
}

func hasStructuredWorkspaceArtifactWriteCue(lower string) bool {
	return containsAnyStructuredWorkspaceCue(lower,
		"write",
		"save",
		"saved",
		"output",
		"export",
		"create",
		"写",
		"保存",
		"输出",
		"导出",
		"生成",
	)
}

func hasStructuredWorkspaceArtifactExtractionCue(lower string) bool {
	if containsAnyStructuredWorkspaceCue(lower,
		"extract",
		"summary",
		"summarize",
		"summarise",
		"analyze",
		"analyse",
		"compare",
		"triage",
		"classify",
		"question",
		"questions",
		"answer",
		"answers",
		"table",
		"提取",
		"总结",
		"摘要",
		"分析",
		"对比",
		"归类",
		"分类",
		"问题",
		"回答",
		"答案",
		"表格",
	) {
		return true
	}
	if containsAnyStructuredWorkspaceCue(lower,
		"one answer per line",
		"one per line",
		"line by line",
		"line-by-line",
		"write the answers",
		"一行一个",
		"逐行",
	) {
		return true
	}
	return strings.Contains(lower, "1.") && strings.Contains(lower, "2.")
}

func mentionsStructuredWorkspaceArtifactExt(lower string, exts ...string) bool {
	for _, ext := range exts {
		if ext != "" && strings.Contains(lower, ext) {
			return true
		}
	}
	return false
}

func containsAnyStructuredWorkspaceCue(lower string, cues ...string) bool {
	for _, cue := range cues {
		if cue != "" && strings.Contains(lower, cue) {
			return true
		}
	}
	return false
}
