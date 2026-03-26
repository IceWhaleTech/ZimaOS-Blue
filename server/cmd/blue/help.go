package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	skillEmbed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/embedded"
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
		fmt.Println("  blue help <skill>        Show full SKILL.md manual")
		fmt.Println("  blue <skill> key=value   Execute a skill")

		skills := listPinnedSkillSummaries(agentcore.PinnedSkills(), skillEmbed.SkillsFS.ReadFile)
		if len(skills) == 0 {
			return nil
		}

		fmt.Printf("\nBuilt-in skills (%d):\n", len(skills))
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

	pinnedSet := buildPinnedSkillSet(agentcore.PinnedSkills())
	for _, candidate := range candidateSkillIDs(args[0]) {
		if _, pinned := pinnedSet[candidate]; !pinned {
			continue
		}
		manual, source, ok := findSkillManual(candidate, nil, skillEmbed.SkillsFS.ReadFile)
		if ok {
			fmt.Printf("Source: %s\n\n", source)
			fmt.Print(manual)
			if !strings.HasSuffix(manual, "\n") {
				fmt.Println()
			}
			return nil
		}
	}

	if err != nil {
		return err
	}
	return fmt.Errorf("unknown help topic %q", strings.Join(args, " "))
}

func findSkillManual(
	topic string,
	roots []string,
	readEmbedded func(string) ([]byte, error),
) (manual string, source string, ok bool) {
	for _, skillID := range candidateSkillIDs(topic) {
		for _, root := range roots {
			mdPath := filepath.Join(root, skillID, "SKILL.md")
			data, err := os.ReadFile(mdPath)
			if err == nil {
				return string(data), mdPath, true
			}
		}

		if readEmbedded == nil {
			continue
		}
		embedPath := filepath.ToSlash(filepath.Join("skills", skillID, "SKILL.md"))
		data, err := readEmbedded(embedPath)
		if err == nil {
			return string(data), "embedded://" + embedPath, true
		}
	}

	return "", "", false
}

func candidateSkillIDs(topic string) []string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return nil
	}

	ids := []string{topic}
	if dot := strings.IndexByte(topic, '.'); dot > 0 {
		ids = append(ids, topic[:dot])
	}

	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

type skillSummary struct {
	Name        string
	Description string
}

func listPinnedSkillSummaries(
	pinned []string,
	readEmbeddedFile func(string) ([]byte, error),
) []skillSummary {
	if readEmbeddedFile == nil {
		return nil
	}
	out := make([]skillSummary, 0, len(pinned))
	for _, name := range pinned {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		embedPath := filepath.ToSlash(filepath.Join("skills", name, "SKILL.md"))
		data, err := readEmbeddedFile(embedPath)
		if err != nil {
			continue
		}
		out = append(out, skillSummary{
			Name:        name,
			Description: parseSkillDescription(data),
		})
	}
	return out
}

func buildPinnedSkillSet(pinned []string) map[string]struct{} {
	set := make(map[string]struct{}, len(pinned))
	for _, name := range pinned {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		set[name] = struct{}{}
	}
	return set
}

func parseSkillDescription(data []byte) string {
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
