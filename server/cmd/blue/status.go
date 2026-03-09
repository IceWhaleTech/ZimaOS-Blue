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
	statusAll     bool
	statusDeep    bool
	statusTimeout int
)

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show service health and recent activity",
	Long: `Display the current status of ZimaOS-Blue service including:
- Service health status
- Connected providers
- Active sessions
- Recent activity`,
	Run: runStatus,
}

func init() {
	statusCmd.Flags().BoolVar(&statusAll, "all", false, "show all details")
	statusCmd.Flags().BoolVar(&statusDeep, "deep", false, "perform deep health check")
	statusCmd.Flags().IntVar(&statusTimeout, "timeout", 5, "timeout in seconds")
}

// StatusResponse represents the status response
type StatusResponse struct {
	Status    string           `json:"status"`
	Version   string           `json:"version"`
	Uptime    string           `json:"uptime,omitempty"`
	Providers []ProviderStatus `json:"providers,omitempty"`
	Sessions  *SessionsStatus  `json:"sessions,omitempty"`
	System    *SystemStatus    `json:"system,omitempty"`
	Errors    []string         `json:"errors,omitempty"`
}

// ProviderStatus represents a provider's status
type ProviderStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Models int    `json:"models,omitempty"`
}

// SessionsStatus represents sessions status
type SessionsStatus struct {
	Active int `json:"active"`
	Total  int `json:"total"`
}

// SystemStatus represents system status
type SystemStatus struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
}

func runStatus(cmd *cobra.Command, args []string) {
	// Try to connect to the running service
	client := &http.Client{
		Timeout: time.Duration(statusTimeout) * time.Second,
	}

	baseURL := getServiceBaseURL()

	// Check health endpoint
	resp, err := client.Get(baseURL + "/health")
	if err != nil {
		printStatusError("Service not running or unreachable", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		printStatusError("Service returned unhealthy status", nil)
		return
	}

	// Build status response
	status := StatusResponse{
		Status:  "healthy",
		Version: version,
	}

	// Get additional info if --all or --deep
	if statusAll || statusDeep {
		// Get providers status
		if providers, err := getProvidersStatus(client, baseURL); err == nil {
			status.Providers = providers
		}

		// Get sessions status
		if sessions, err := getSessionsStatus(client, baseURL); err == nil {
			status.Sessions = sessions
		}

		// Get system status
		if statusDeep {
			if system, err := getSystemStatus(client, baseURL); err == nil {
				status.System = system
			}
		}
	}

	// Output
	if jsonOutput {
		printJSON(status)
	} else {
		printStatusHuman(status)
	}
}

func getProvidersStatus(client *http.Client, baseURL string) ([]ProviderStatus, error) {
	resp, err := client.Get(baseURL + "/api/v1/providers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var providers []ProviderStatus
	if err := json.NewDecoder(resp.Body).Decode(&providers); err != nil {
		return nil, err
	}
	return providers, nil
}

func getSessionsStatus(client *http.Client, baseURL string) (*SessionsStatus, error) {
	resp, err := client.Get(baseURL + "/api/v1/conversations")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Conversations []interface{} `json:"conversations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &SessionsStatus{
		Active: len(result.Conversations),
		Total:  len(result.Conversations),
	}, nil
}

func getSystemStatus(client *http.Client, baseURL string) (*SystemStatus, error) {
	resp, err := client.Get(baseURL + "/api/v1/metrics/system")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var system SystemStatus
	if err := json.NewDecoder(resp.Body).Decode(&system); err != nil {
		return nil, err
	}
	return &system, nil
}

func printStatusError(msg string, err error) {
	if jsonOutput {
		status := StatusResponse{
			Status: "error",
			Errors: []string{msg},
		}
		if err != nil {
			status.Errors = append(status.Errors, err.Error())
		}
		printJSON(status)
	} else {
		if noColor {
			fmt.Printf("Status: ERROR\n")
			fmt.Printf("Error: %s\n", msg)
		} else {
			fmt.Printf("\033[31mStatus: ERROR\033[0m\n")
			fmt.Printf("\033[31mError:\033[0m %s\n", msg)
		}
		if err != nil {
			fmt.Printf("Details: %v\n", err)
		}
	}
	os.Exit(1)
}

func printStatusHuman(status StatusResponse) {
	// Status header
	if noColor {
		fmt.Printf("Status: %s\n", status.Status)
	} else {
		statusColor := "\033[32m" // Green for healthy
		if status.Status != "healthy" {
			statusColor = "\033[31m" // Red for unhealthy
		}
		fmt.Printf("%sStatus: %s\033[0m\n", statusColor, status.Status)
	}

	fmt.Printf("Version: %s\n", status.Version)

	if status.Uptime != "" {
		fmt.Printf("Uptime: %s\n", status.Uptime)
	}

	// Providers
	if len(status.Providers) > 0 {
		fmt.Println("\nProviders:")
		for _, p := range status.Providers {
			statusIcon := "✓"
			if p.Status != "available" {
				statusIcon = "✗"
			}
			if noColor {
				fmt.Printf("  [%s] %s (%s)\n", statusIcon, p.Name, p.Status)
			} else {
				color := "\033[32m"
				if p.Status != "available" {
					color = "\033[31m"
				}
				fmt.Printf("  %s[%s]\033[0m %s (%s)\n", color, statusIcon, p.Name, p.Status)
			}
		}
	}

	// Sessions
	if status.Sessions != nil {
		fmt.Printf("\nSessions: %d active / %d total\n", status.Sessions.Active, status.Sessions.Total)
	}

	// System
	if status.System != nil {
		fmt.Println("\nSystem:")
		fmt.Printf("  CPU: %.1f%%\n", status.System.CPUUsage)
		fmt.Printf("  Memory: %.1f%%\n", status.System.MemoryUsage)
		fmt.Printf("  Disk: %.1f%%\n", status.System.DiskUsage)
	}
}

func printJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}
