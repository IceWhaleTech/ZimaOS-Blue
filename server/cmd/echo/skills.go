package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var (
	skillsEligible bool
)

// skillsCmd represents the skills command
var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Skill management",
	Long: `Manage ZimaOS-Echo skills.

Subcommands:
  list              List available skills
  info <id>         Show skill details
  check             Check skill availability`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available skills",
	Run:   runSkillsList,
}

var skillsInfoCmd = &cobra.Command{
	Use:   "info <id>",
	Short: "Show skill details",
	Args:  cobra.ExactArgs(1),
	Run:   runSkillsInfo,
}

var skillsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check skill availability",
	Run:   runSkillsCheck,
}

func init() {
	skillsListCmd.Flags().BoolVar(&skillsEligible, "eligible", false, "show only eligible skills")

	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsInfoCmd)
	skillsCmd.AddCommand(skillsCheckCmd)

	rootCmd.AddCommand(skillsCmd)
}

// SkillInfo represents skill information
type SkillInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Version     string   `json:"version,omitempty"`
	Category    string   `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Enabled     bool     `json:"enabled"`
	Builtin     bool     `json:"builtin"`
}

func getSkillsBaseURL() string {
	port := 8080
	if devMode {
		port = 8081
	}
	return fmt.Sprintf("http://localhost:%d/api/v1/skills", port)
}

func runSkillsList(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	url := getSkillsBaseURL()
	if skillsEligible {
		url += "?eligible=true"
	}

	resp, err := client.Get(url)
	if err != nil {
		printSkillsError("Failed to fetch skills", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Skills []SkillInfo `json:"skills"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printSkillsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		if len(result.Skills) == 0 {
			fmt.Println("No skills available")
			return
		}

		// Group by category
		categories := make(map[string][]SkillInfo)
		for _, s := range result.Skills {
			cat := s.Category
			if cat == "" {
				cat = "General"
			}
			categories[cat] = append(categories[cat], s)
		}

		fmt.Println("Available Skills:")
		for cat, skills := range categories {
			fmt.Printf("\n%s:\n", cat)
			for _, s := range skills {
				statusIcon := "✓"
				if !s.Enabled {
					statusIcon = "-"
				}

				builtinTag := ""
				if s.Builtin {
					builtinTag = " [builtin]"
				}

				if noColor {
					fmt.Printf("  [%s] %s%s\n", statusIcon, s.Name, builtinTag)
					if s.Description != "" {
						fmt.Printf("      %s\n", s.Description)
					}
				} else {
					color := "\033[32m"
					if !s.Enabled {
						color = "\033[90m"
					}
					fmt.Printf("  %s[%s]\033[0m %s%s\n", color, statusIcon, s.Name, builtinTag)
					if s.Description != "" {
						fmt.Printf("      \033[90m%s\033[0m\n", s.Description)
					}
				}
			}
		}
		fmt.Printf("\nTotal: %d skills\n", len(result.Skills))
	}
}

func runSkillsInfo(cmd *cobra.Command, args []string) {
	skillID := args[0]
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(getSkillsBaseURL() + "/" + skillID)
	if err != nil {
		printSkillsError("Failed to fetch skill", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"error": "Skill not found",
				"id":    skillID,
			})
		} else {
			fmt.Printf("Skill not found: %s\n", skillID)
		}
		os.Exit(1)
	}

	var skill SkillInfo
	if err := json.NewDecoder(resp.Body).Decode(&skill); err != nil {
		printSkillsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(skill)
	} else {
		fmt.Printf("Skill: %s\n", skill.Name)
		fmt.Printf("ID: %s\n", skill.ID)
		if skill.Version != "" {
			fmt.Printf("Version: %s\n", skill.Version)
		}
		if skill.Category != "" {
			fmt.Printf("Category: %s\n", skill.Category)
		}
		if skill.Description != "" {
			fmt.Printf("Description: %s\n", skill.Description)
		}
		fmt.Printf("Enabled: %v\n", skill.Enabled)
		fmt.Printf("Builtin: %v\n", skill.Builtin)
		if len(skill.Tags) > 0 {
			fmt.Printf("Tags: %v\n", skill.Tags)
		}
	}
}

func runSkillsCheck(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(getSkillsBaseURL() + "/check")
	if err != nil {
		printSkillsError("Failed to check skills", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Total    int `json:"total"`
		Enabled  int `json:"enabled"`
		Disabled int `json:"disabled"`
		Builtin  int `json:"builtin"`
		Custom   int `json:"custom"`
		Issues   []struct {
			SkillID string `json:"skill_id"`
			Issue   string `json:"issue"`
		} `json:"issues,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printSkillsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		fmt.Println("Skill Status:")
		fmt.Printf("  Total: %d\n", result.Total)
		fmt.Printf("  Enabled: %d\n", result.Enabled)
		fmt.Printf("  Disabled: %d\n", result.Disabled)
		fmt.Printf("  Builtin: %d\n", result.Builtin)
		fmt.Printf("  Custom: %d\n", result.Custom)

		if len(result.Issues) > 0 {
			fmt.Println("\nIssues:")
			for _, issue := range result.Issues {
				if noColor {
					fmt.Printf("  [!] %s: %s\n", issue.SkillID, issue.Issue)
				} else {
					fmt.Printf("  \033[33m[!]\033[0m %s: %s\n", issue.SkillID, issue.Issue)
				}
			}
		} else {
			if noColor {
				fmt.Println("\nAll skills OK")
			} else {
				fmt.Println("\n\033[32m✓\033[0m All skills OK")
			}
		}
	}
}

func printSkillsError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": false,
			"error":   msg,
			"details": err.Error(),
		})
	} else {
		if noColor {
			fmt.Printf("Error: %s\n", msg)
		} else {
			fmt.Printf("\033[31mError:\033[0m %s\n", msg)
		}
		if verbose && err != nil {
			fmt.Printf("Details: %v\n", err)
		}
	}
	os.Exit(1)
}
