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
	modelsProvider string
	modelsCheck    bool
)

// modelsCmd represents the models command
var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "Model management",
	Long: `Manage LLM models and providers.

Subcommands:
  list    List available models
  status  Show model status
  set     Set default model
  scan    Scan for available models`,
}

var modelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available models",
	Run:   runModelsList,
}

var modelsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show model status",
	Run:   runModelsStatus,
}

var modelsSetCmd = &cobra.Command{
	Use:   "set <model>",
	Short: "Set default model",
	Args:  cobra.ExactArgs(1),
	Run:   runModelsSet,
}

var modelsScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for available models",
	Run:   runModelsScan,
}

func init() {
	// Models flags
	modelsListCmd.Flags().StringVar(&modelsProvider, "provider", "", "filter by provider")
	modelsListCmd.Flags().BoolVar(&modelsCheck, "check", false, "check model availability")

	modelsStatusCmd.Flags().StringVar(&modelsProvider, "provider", "", "filter by provider")

	// Add subcommands
	modelsCmd.AddCommand(modelsListCmd)
	modelsCmd.AddCommand(modelsStatusCmd)
	modelsCmd.AddCommand(modelsSetCmd)
	modelsCmd.AddCommand(modelsScanCmd)
}

// ModelInfo represents model information
type ModelInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Status   string `json:"status,omitempty"`
}

// ModelsResponse represents the models list response
type ModelsResponse struct {
	Models   []ModelInfo `json:"models"`
	Default  string      `json:"default,omitempty"`
	Provider string      `json:"provider,omitempty"`
}

func runModelsList(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	baseURL := getServiceBaseURL()

	url := baseURL + "/api/v1/models"
	if modelsProvider != "" {
		url += "?provider=" + modelsProvider
	}

	resp, err := client.Get(url)
	if err != nil {
		printModelsError("Failed to fetch models", err)
		return
	}
	defer resp.Body.Close()

	var result ModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// Try alternative format
		var models []ModelInfo
		resp.Body.Close()
		resp, _ = client.Get(url)
		if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
			printModelsError("Failed to parse response", err)
			return
		}
		result.Models = models
	}

	if jsonOutput {
		printJSON(result)
	} else {
		printModelsHuman(result)
	}
}

func runModelsStatus(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	baseURL := getServiceBaseURL()

	resp, err := client.Get(baseURL + "/api/v1/providers")
	if err != nil {
		printModelsError("Failed to fetch provider status", err)
		return
	}
	defer resp.Body.Close()

	var providers []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Status  string `json:"status"`
		Models  int    `json:"models"`
		Enabled bool   `json:"enabled"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		printModelsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(providers)
	} else {
		fmt.Println("Provider Status:")
		for _, p := range providers {
			statusIcon := "✓"
			if p.Status != "available" && p.Status != "ok" {
				statusIcon = "✗"
			}
			if !p.Enabled {
				statusIcon = "-"
			}

			if noColor {
				fmt.Printf("  [%s] %s: %s (%d models)\n", statusIcon, p.Name, p.Status, p.Models)
			} else {
				color := "\033[32m"
				if p.Status != "available" && p.Status != "ok" {
					color = "\033[31m"
				}
				if !p.Enabled {
					color = "\033[90m"
				}
				fmt.Printf("  %s[%s]\033[0m %s: %s (%d models)\n", color, statusIcon, p.Name, p.Status, p.Models)
			}
		}
	}
}

func runModelsSet(cmd *cobra.Command, args []string) {
	model := args[0]

	// Load config and set default model
	m, err := loadConfigMap()
	if err != nil {
		printModelsError("Failed to load config", err)
		return
	}

	setNestedKey(m, "llm.default_model", model)

	if err := saveConfigMap(m); err != nil {
		printModelsError("Failed to save config", err)
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": true,
			"model":   model,
		})
	} else {
		if noColor {
			fmt.Printf("Default model set to: %s\n", model)
		} else {
			fmt.Printf("\033[32m✓\033[0m Default model set to: %s\n", model)
		}
	}
}

func runModelsScan(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 30 * time.Second}

	baseURL := getServiceBaseURL()

	if !jsonOutput {
		fmt.Println("Scanning for available models...")
	}

	// Trigger model scan
	resp, err := client.Post(baseURL+"/api/v1/models/scan", "application/json", nil)
	if err != nil {
		printModelsError("Failed to scan models", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Success bool        `json:"success"`
		Models  []ModelInfo `json:"models"`
		Count   int         `json:"count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printModelsError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		if noColor {
			fmt.Printf("Found %d models\n", result.Count)
		} else {
			fmt.Printf("\033[32m✓\033[0m Found %d models\n", result.Count)
		}
	}
}

func printModelsHuman(result ModelsResponse) {
	if len(result.Models) == 0 {
		fmt.Println("No models available")
		return
	}

	fmt.Println("Available Models:")
	currentProvider := ""
	for _, m := range result.Models {
		if m.Provider != currentProvider {
			currentProvider = m.Provider
			fmt.Printf("\n%s:\n", currentProvider)
		}

		isDefault := m.ID == result.Default
		prefix := "  "
		if isDefault {
			prefix = "* "
		}

		if noColor {
			fmt.Printf("%s%s", prefix, m.Name)
		} else {
			if isDefault {
				fmt.Printf("%s\033[36m%s\033[0m", prefix, m.Name)
			} else {
				fmt.Printf("%s%s", prefix, m.Name)
			}
		}

		if m.Status != "" && m.Status != "available" {
			fmt.Printf(" (%s)", m.Status)
		}
		fmt.Println()
	}

	if result.Default != "" {
		fmt.Printf("\n* = default model\n")
	}
}

func printModelsError(msg string, err error) {
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
