package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// pluginsCmd represents the plugins command
var pluginsCmd = &cobra.Command{
	Use:   "plugins",
	Short: "Plugin management",
	Long: `Manage ZimaOS-Blue plugins.

Subcommands:
  list              List installed plugins
  info <id>         Show plugin details
  enable <id>       Enable a plugin
  disable <id>      Disable a plugin
  doctor            Check plugin health`,
}

var pluginsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	Run:   runPluginsList,
}

var pluginsInfoCmd = &cobra.Command{
	Use:   "info <id>",
	Short: "Show plugin details",
	Args:  cobra.ExactArgs(1),
	Run:   runPluginsInfo,
}

var pluginsEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable a plugin",
	Args:  cobra.ExactArgs(1),
	Run:   runPluginsEnable,
}

var pluginsDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable a plugin",
	Args:  cobra.ExactArgs(1),
	Run:   runPluginsDisable,
}

var pluginsDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check plugin health",
	Run:   runPluginsDoctor,
}

func init() {
	pluginsCmd.AddCommand(pluginsListCmd)
	pluginsCmd.AddCommand(pluginsInfoCmd)
	pluginsCmd.AddCommand(pluginsEnableCmd)
	pluginsCmd.AddCommand(pluginsDisableCmd)
	pluginsCmd.AddCommand(pluginsDoctorCmd)

	rootCmd.AddCommand(pluginsCmd)
}

// PluginInfo represents plugin information
type PluginInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
}

func getPluginsBaseURL() string {
	return getServiceAPIBaseURL("/api/v1/plugins")
}

func runPluginsList(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(getPluginsBaseURL())
	if err != nil {
		printPluginsError("Failed to fetch plugins", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Plugins []PluginInfo `json:"plugins"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printPluginsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		if len(result.Plugins) == 0 {
			fmt.Println("No plugins installed")
			return
		}

		fmt.Println("Installed Plugins:")
		for _, p := range result.Plugins {
			statusIcon := "✓"
			if !p.Enabled {
				statusIcon = "-"
			} else if p.Status == "error" {
				statusIcon = "✗"
			}

			if noColor {
				fmt.Printf("  [%s] %s v%s\n", statusIcon, p.Name, p.Version)
				if p.Description != "" {
					fmt.Printf("      %s\n", p.Description)
				}
			} else {
				color := "\033[32m"
				if !p.Enabled {
					color = "\033[90m"
				} else if p.Status == "error" {
					color = "\033[31m"
				}
				fmt.Printf("  %s[%s]\033[0m %s v%s\n", color, statusIcon, p.Name, p.Version)
				if p.Description != "" {
					fmt.Printf("      \033[90m%s\033[0m\n", p.Description)
				}
			}
		}
		fmt.Printf("\nTotal: %d plugins\n", len(result.Plugins))
	}
}

func runPluginsInfo(cmd *cobra.Command, args []string) {
	pluginID := args[0]
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(getPluginsBaseURL() + "/" + pluginID)
	if err != nil {
		printPluginsError("Failed to fetch plugin", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		if jsonOutput {
			printJSON(map[string]interface{}{
				"error": "Plugin not found",
				"id":    pluginID,
			})
		} else {
			fmt.Printf("Plugin not found: %s\n", pluginID)
		}
		os.Exit(1)
	}

	var plugin PluginInfo
	if err := json.NewDecoder(resp.Body).Decode(&plugin); err != nil {
		printPluginsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(plugin)
	} else {
		fmt.Printf("Plugin: %s\n", plugin.Name)
		fmt.Printf("ID: %s\n", plugin.ID)
		fmt.Printf("Version: %s\n", plugin.Version)
		if plugin.Author != "" {
			fmt.Printf("Author: %s\n", plugin.Author)
		}
		if plugin.Description != "" {
			fmt.Printf("Description: %s\n", plugin.Description)
		}
		fmt.Printf("Enabled: %v\n", plugin.Enabled)
		fmt.Printf("Status: %s\n", plugin.Status)
		if plugin.Error != "" {
			fmt.Printf("Error: %s\n", plugin.Error)
		}
	}
}

func runPluginsEnable(cmd *cobra.Command, args []string) {
	setPluginEnabled(args[0], true)
}

func runPluginsDisable(cmd *cobra.Command, args []string) {
	setPluginEnabled(args[0], false)
}

func setPluginEnabled(pluginID string, enabled bool) {
	client := &http.Client{Timeout: 10 * time.Second}

	action := "enable"
	if !enabled {
		action = "disable"
	}

	req, err := http.NewRequest(http.MethodPost, getPluginsBaseURL()+"/"+pluginID+"/"+action, nil)
	if err != nil {
		printPluginsError("Failed to create request", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		printPluginsError("Failed to update plugin", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		printPluginsError("Plugin not found", fmt.Errorf("id: %s", pluginID))
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": true,
			"id":      pluginID,
			"enabled": enabled,
		})
	} else {
		verb := "Enabled"
		if !enabled {
			verb = "Disabled"
		}
		if noColor {
			fmt.Printf("%s plugin: %s\n", verb, pluginID)
		} else {
			fmt.Printf("\033[32m✓\033[0m %s plugin: %s\n", verb, pluginID)
		}
		fmt.Println("Note: Restart the service for changes to take effect")
	}
}

func runPluginsDoctor(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(getPluginsBaseURL() + "/doctor")
	if err != nil {
		printPluginsError("Failed to run plugin doctor", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Healthy bool `json:"healthy"`
		Plugins []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
			Error  string `json:"error,omitempty"`
		} `json:"plugins"`
		Issues []string `json:"issues,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printPluginsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		if result.Healthy {
			if noColor {
				fmt.Println("All plugins healthy")
			} else {
				fmt.Println("\033[32m✓\033[0m All plugins healthy")
			}
		} else {
			if noColor {
				fmt.Println("Plugin issues detected:")
			} else {
				fmt.Println("\033[31m✗\033[0m Plugin issues detected:")
			}
		}

		for _, p := range result.Plugins {
			if p.Status != "healthy" && p.Status != "ok" {
				if noColor {
					fmt.Printf("  [✗] %s: %s\n", p.Name, p.Status)
				} else {
					fmt.Printf("  \033[31m[✗]\033[0m %s: %s\n", p.Name, p.Status)
				}
				if p.Error != "" {
					fmt.Printf("      Error: %s\n", p.Error)
				}
			}
		}

		if len(result.Issues) > 0 {
			fmt.Println("\nIssues:")
			for _, issue := range result.Issues {
				fmt.Printf("  - %s\n", issue)
			}
		}
	}
}

func printPluginsError(msg string, err error) {
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
