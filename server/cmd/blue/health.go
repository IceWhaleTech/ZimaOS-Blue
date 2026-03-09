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
	healthTimeout int
)

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Fetch health from running service",
	Long: `Check the health status of the running ZimaOS-Blue service.
Returns detailed health information including component status.`,
	Run: runHealth,
}

func init() {
	healthCmd.Flags().IntVar(&healthTimeout, "timeout", 5, "timeout in seconds")
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status     string                     `json:"status"`
	Version    string                     `json:"version,omitempty"`
	Timestamp  string                     `json:"timestamp"`
	Components map[string]ComponentHealth `json:"components,omitempty"`
}

// ComponentHealth represents a component's health status
type ComponentHealth struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

func runHealth(cmd *cobra.Command, args []string) {
	client := &http.Client{
		Timeout: time.Duration(healthTimeout) * time.Second,
	}

	baseURL := getServiceBaseURL()

	start := time.Now()
	resp, err := client.Get(baseURL + "/health")
	latency := time.Since(start)

	if err != nil {
		printHealthError("Service not running or unreachable", err)
		return
	}
	defer resp.Body.Close()

	// Parse response
	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		// If can't parse, create basic response
		health = HealthResponse{
			Status:    "healthy",
			Timestamp: time.Now().Format(time.RFC3339),
		}
		if resp.StatusCode != http.StatusOK {
			health.Status = "unhealthy"
		}
	}

	health.Version = version

	// Add latency info if verbose
	if verbose {
		if health.Components == nil {
			health.Components = make(map[string]ComponentHealth)
		}
		health.Components["api"] = ComponentHealth{
			Status:  "healthy",
			Latency: latency.String(),
		}
	}

	// Output
	if jsonOutput {
		printJSON(health)
	} else {
		printHealthHuman(health, latency)
	}

	// Exit with error code if unhealthy
	if health.Status != "healthy" && health.Status != "ok" {
		os.Exit(1)
	}
}

func printHealthError(msg string, err error) {
	if jsonOutput {
		health := HealthResponse{
			Status:    "error",
			Timestamp: time.Now().Format(time.RFC3339),
			Components: map[string]ComponentHealth{
				"api": {
					Status:  "error",
					Message: msg,
				},
			},
		}
		printJSON(health)
	} else {
		if noColor {
			fmt.Printf("Health: ERROR\n")
			fmt.Printf("Error: %s\n", msg)
		} else {
			fmt.Printf("\033[31mHealth: ERROR\033[0m\n")
			fmt.Printf("\033[31mError:\033[0m %s\n", msg)
		}
		if err != nil && verbose {
			fmt.Printf("Details: %v\n", err)
		}
	}
	os.Exit(1)
}

func printHealthHuman(health HealthResponse, latency time.Duration) {
	// Health status
	if noColor {
		fmt.Printf("Health: %s\n", health.Status)
	} else {
		statusColor := "\033[32m" // Green
		if health.Status != "healthy" && health.Status != "ok" {
			statusColor = "\033[31m" // Red
		}
		fmt.Printf("%sHealth: %s\033[0m\n", statusColor, health.Status)
	}

	fmt.Printf("Version: %s\n", health.Version)
	fmt.Printf("Latency: %s\n", latency.Round(time.Millisecond))

	// Components
	if len(health.Components) > 0 && verbose {
		fmt.Println("\nComponents:")
		for name, comp := range health.Components {
			statusIcon := "✓"
			if comp.Status != "healthy" && comp.Status != "ok" {
				statusIcon = "✗"
			}
			if noColor {
				fmt.Printf("  [%s] %s: %s", statusIcon, name, comp.Status)
			} else {
				color := "\033[32m"
				if comp.Status != "healthy" && comp.Status != "ok" {
					color = "\033[31m"
				}
				fmt.Printf("  %s[%s]\033[0m %s: %s", color, statusIcon, name, comp.Status)
			}
			if comp.Latency != "" {
				fmt.Printf(" (%s)", comp.Latency)
			}
			if comp.Message != "" {
				fmt.Printf(" - %s", comp.Message)
			}
			fmt.Println()
		}
	}
}
