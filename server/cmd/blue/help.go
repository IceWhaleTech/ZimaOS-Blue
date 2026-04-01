package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/spf13/cobra"
)

var helpCmd = &cobra.Command{
	Use:   "help [topic]",
	Short: "Show help for a command or skill",
	Long: `Show help for built-in commands.

When the topic is not a built-in command, Blue will try to load and print the
full SKILL.md manual for that skill.`,
	Args: cobra.ArbitraryArgs,
	RunE: runHelp,
}

func init() {
	rootCmd.SetHelpCommand(helpCmd)
}

func runHelp(cmd *cobra.Command, args []string) error {
	root := cmd.Root()
	if len(args) == 0 {
		if err := root.Help(); err != nil {
			return err
		}

		fmt.Println()
		fmt.Println("Guides:")
		for _, line := range rootHelpGuideLines() {
			fmt.Println(line)
		}

		skills := listPinnedSkillSummaries(helpWorkspaceDir(), agentcore.PinnedSkills())
		if len(skills) == 0 {
			return nil
		}

		fmt.Printf("\nPinned skills (%d):\n", len(skills))
		for _, s := range skills {
			if s.Description == "" {
				fmt.Printf("  %-22s\n", s.Name)
				continue
			}
			fmt.Printf("  %-22s %s\n", s.Name, s.Description)
		}
		return nil
	}

	target, _, err := root.Find(args)
	if err == nil && target != nil && target != root {
		return target.Help()
	}

	manual, source, ok, resolveErr := resolveHelpSkillStrict(args[0], helpWorkspaceDir())
	if resolveErr != nil {
		return resolveErr
	}
	if ok {
		fmt.Printf("Source: %s\n\n", source)
		fmt.Print(manual)
		if !strings.HasSuffix(manual, "\n") {
			fmt.Println()
		}
		return nil
	}

	if err != nil {
		return err
	}
	return fmt.Errorf("unknown help topic %q", strings.Join(args, " "))
}

func rootHelpGuideLines() []string {
	return []string{
		"  blue help <skill>                 Show full SKILL.md manual",
		"  blue <skill> ...                  Execute a built-in skill or skill subcommand",
		"  blue media generate \"...\"        Run the media generation command group",
		"  blue exec command='tool ...'      Run external CLIs documented by some skills",
	}
}

func resolveHelpSkill(topic, workspaceDir string) (manual string, source string, ok bool, err error) {
	return findSkillManual(topic, skillmanifest.ResolveRoots(workspaceDir))
}

func resolveHelpSkillStrict(topic, workspaceDir string) (manual string, source string, ok bool, err error) {
	return findSkillManualStrict(topic, workspaceDir, skillmanifest.ResolveRoots(workspaceDir))
}

func helpWorkspaceDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

func findSkillManual(topic string, roots []string) (manual string, source string, ok bool, err error) {
	resolved, found, err := skillmanifest.FindAnyByCandidatesStrict(candidateSkillIDs(topic), roots, "", skillmanifest.Options{})
	if err != nil {
		return "", "", false, err
	}
	if !found {
		return "", "", false, nil
	}
	return string(resolved.Raw), resolved.Source, true, nil
}

func findSkillManualStrict(topic, workspaceDir string, roots []string) (manual string, source string, ok bool, err error) {
	resolved, found, err := skillmanifest.FindAnyByCandidatesStrict(candidateSkillIDs(topic), roots, workspaceDir, skillmanifest.Options{})
	if err != nil {
		return "", "", false, err
	}
	if !found {
		return "", "", false, nil
	}
	return string(resolved.Raw), resolved.Source, true, nil
}

func candidateSkillIDs(topic string) []string {
	return skillmanifest.CandidateIDs(topic)
}

type skillSummary struct {
	Name        string
	Description string
}

func listPinnedSkillSummaries(
	workspaceDir string,
	pinned []string,
) []skillSummary {
	roots := skillmanifest.ResolveRoots(workspaceDir)
	out := make([]skillSummary, 0, len(pinned))
	for _, name := range pinned {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		resolved, ok, err := skillmanifest.FindAnyByCandidatesStrict(skillmanifest.CandidateIDs(name), roots, workspaceDir, skillmanifest.Options{})
		if err != nil || !ok {
			continue
		}
		desc := strings.TrimSpace(resolved.Document.Description)
		if desc == "" {
			desc = parseSkillDescription(resolved.Raw)
		}
		id := strings.TrimSpace(resolved.Document.ID)
		if id == "" {
			id = name
		}
		out = append(out, skillSummary{
			Name:        id,
			Description: desc,
		})
	}
	return out
}

func parseSkillDescription(data []byte) string {
	if doc, err := skillmanifest.ParseEntry("skill", "memory:SKILL.md", data, skillmanifest.Options{}); err == nil && strings.TrimSpace(doc.Description) != "" {
		return doc.Description
	}

	content := string(data)
	body := content

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			for _, line := range strings.Split(frontmatter, "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(strings.ToLower(trimmed), "description:") {
					desc := strings.TrimSpace(trimmed[len("description:"):])
					desc = strings.Trim(desc, `"'`)
					if desc != "" {
						return desc
					}
				}
			}
			body = parts[2]
		}
	}

	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		}
		return trimmed
	}
	return ""
}
