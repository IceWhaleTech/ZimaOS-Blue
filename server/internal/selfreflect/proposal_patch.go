package selfreflect

import (
	"os"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
)

const (
	proposalTargetFile    = workspace.FileAGENTS
	proposalTargetSection = "## Research Takeaways"
)

func renderProposalPatchPreview(mgr *workspace.Manager, proposal *Proposal) string {
	if mgr == nil || proposal == nil {
		return ""
	}
	if strings.TrimSpace(proposal.TargetFile) != proposalTargetFile {
		return ""
	}
	current, err := mgr.ReadFile(proposalTargetFile)
	if err != nil && !os.IsNotExist(err) {
		return ""
	}
	current = normalizeProposalContent(current)
	updated := applyProposalToAgents(current, proposal)
	return buildProposalDiffPreview(proposalTargetFile, current, updated)
}

func applyProposalToAgents(current string, proposal *Proposal) string {
	sectionLines := buildProposalSectionLines(current, proposal)
	lines := splitLinesPreserve(current)
	start, end := findProposalSection(lines)
	if start >= 0 {
		updated := append([]string{}, lines[:start]...)
		updated = append(updated, sectionLines...)
		updated = append(updated, lines[end:]...)
		return strings.Join(updated, "\n")
	}
	base := strings.TrimRight(current, "\n")
	if base == "" {
		return strings.Join(sectionLines, "\n") + "\n"
	}
	return base + "\n\n" + strings.Join(sectionLines, "\n") + "\n"
}

func buildProposalSectionLines(current string, proposal *Proposal) []string {
	entry := renderProposalEntry(proposal)
	lines := splitLinesPreserve(current)
	start, end := findProposalSection(lines)
	if start >= 0 {
		section := append([]string{}, lines[start:end]...)
		if !sectionContainsLesson(section, proposal.Lesson) {
			if len(section) > 0 && strings.TrimSpace(section[len(section)-1]) != "" {
				section = append(section, "")
			}
			section = append(section, entry)
		}
		return trimTrailingBlankLines(section)
	}
	return []string{
		proposalTargetSection,
		"",
		entry,
	}
}

func renderProposalEntry(proposal *Proposal) string {
	parts := []string{"- " + strings.TrimSpace(proposal.Lesson)}
	if when := strings.TrimSpace(proposal.WhenToApply); when != "" {
		parts = append(parts, "Apply when: "+when)
	}
	if evidence := strings.TrimSpace(proposal.Evidence); evidence != "" {
		parts = append(parts, "Evidence: "+evidence)
	}
	return strings.Join(parts, " ")
}

func sectionContainsLesson(lines []string, lesson string) bool {
	key := normalizeDedupKey(lesson)
	if key == "" {
		return false
	}
	for _, line := range lines {
		if normalizeDedupKey(strings.TrimPrefix(strings.TrimSpace(line), "- ")) == key {
			return true
		}
	}
	return false
}

func findProposalSection(lines []string) (int, int) {
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == proposalTargetSection {
			start = i
			break
		}
	}
	if start < 0 {
		return -1, -1
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "## ") && trimmed != proposalTargetSection {
			end = i
			break
		}
	}
	return start, end
}

func buildProposalDiffPreview(fileName string, before string, after string) string {
	beforeLines := splitLinesPreserve(before)
	afterLines := splitLinesPreserve(after)
	var sb strings.Builder
	sb.WriteString("--- ")
	sb.WriteString(fileName)
	sb.WriteString("\n")
	sb.WriteString("+++ ")
	sb.WriteString(fileName)
	sb.WriteString("\n")
	sb.WriteString("@@ section: Research Takeaways @@\n")
	for _, line := range beforeLines {
		sb.WriteString("-")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	for _, line := range afterLines {
		sb.WriteString("+")
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func normalizeProposalContent(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return content
}

func splitLinesPreserve(content string) []string {
	content = normalizeProposalContent(content)
	if content == "" {
		return nil
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func trimTrailingBlankLines(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return append([]string{}, lines[:end]...)
}
