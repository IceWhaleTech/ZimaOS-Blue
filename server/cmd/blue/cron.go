package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"time"
)

// cronCmd represents the cron command
var cronCmd = &cobra.Command{
	Use:   "cron",
	Short: "Cron job management",
	Long: `Manage scheduled cron jobs.

Subcommands:
  list              List all cron jobs
  status            Show cron service status
  add               Add a new cron job
  rm <id>           Remove a cron job
  enable <id>       Enable a cron job
  disable <id>      Disable a cron job
  run <id>          Run a cron job immediately
  runs <id>         Show job execution history`,
}

var cronListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all cron jobs",
	Run:   runCronList,
}

var cronStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cron service status",
	Run:   runCronStatus,
}

var cronAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new cron job",
	Run:   runCronAdd,
}

var cronRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove a cron job",
	Args:  cobra.ExactArgs(1),
	Run:   runCronRm,
}

var cronEnableCmd = &cobra.Command{
	Use:   "enable <id>",
	Short: "Enable a cron job",
	Args:  cobra.ExactArgs(1),
	Run:   runCronEnable,
}

var cronDisableCmd = &cobra.Command{
	Use:   "disable <id>",
	Short: "Disable a cron job",
	Args:  cobra.ExactArgs(1),
	Run:   runCronDisable,
}

var cronRunCmd = &cobra.Command{
	Use:   "run <id>",
	Short: "Run a cron job immediately",
	Args:  cobra.ExactArgs(1),
	Run:   runCronRun,
}

var cronRunsCmd = &cobra.Command{
	Use:   "runs <id>",
	Short: "Show job execution history",
	Args:  cobra.ExactArgs(1),
	Run:   runCronRuns,
}

// Cron add flags
var (
	cronName     string
	cronSchedule string
	cronHandler  string
	cronPayload  string
)

func init() {
	cronAddCmd.Flags().StringVar(&cronName, "name", "", "job name (required)")
	cronAddCmd.Flags().StringVar(&cronSchedule, "cron", "", "cron expression (required)")
	cronAddCmd.Flags().StringVar(&cronHandler, "handler", "http", "job handler (http, command)")
	cronAddCmd.Flags().StringVar(&cronPayload, "payload", "", "job payload as JSON")
	cronAddCmd.MarkFlagRequired("name")
	cronAddCmd.MarkFlagRequired("cron")

	cronCmd.AddCommand(cronListCmd)
	cronCmd.AddCommand(cronStatusCmd)
	cronCmd.AddCommand(cronAddCmd)
	cronCmd.AddCommand(cronRmCmd)
	cronCmd.AddCommand(cronEnableCmd)
	cronCmd.AddCommand(cronDisableCmd)
	cronCmd.AddCommand(cronRunCmd)
	cronCmd.AddCommand(cronRunsCmd)

	rootCmd.AddCommand(cronCmd)
}

// CronJob represents a cron job
type CronJob struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Schedule    string     `json:"schedule"`
	Handler     string     `json:"handler"`
	Enabled     bool       `json:"enabled"`
	Status      string     `json:"status"`
	LastRunAt   *time.Time `json:"last_run_at,omitempty"`
	NextRunAt   *time.Time `json:"next_run_at,omitempty"`
	RunCount    int64      `json:"run_count"`
	FailCount   int64      `json:"fail_count"`
}

func getCronBaseURL() string {
	return getServiceAPIBaseURL("/api/cron")
}

func runCronList(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(getCronBaseURL() + "/jobs")
	if err != nil {
		printCronError("Failed to fetch cron jobs", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Jobs []CronJob `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printCronError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		if len(result.Jobs) == 0 {
			fmt.Println("No cron jobs configured")
			return
		}

		fmt.Println("Cron Jobs:")
		for _, job := range result.Jobs {
			statusIcon := "✓"
			if !job.Enabled {
				statusIcon = "-"
			} else if job.Status == "running" {
				statusIcon = "▶"
			}

			nextRun := "N/A"
			if job.NextRunAt != nil {
				nextRun = job.NextRunAt.Format("2006-01-02 15:04")
			}

			if noColor {
				fmt.Printf("  [%s] %s (%s)\n", statusIcon, job.Name, job.Schedule)
				fmt.Printf("      ID: %s | Next: %s | Runs: %d | Fails: %d\n",
					job.ID[:8], nextRun, job.RunCount, job.FailCount)
			} else {
				color := "\033[32m" // Green
				if !job.Enabled {
					color = "\033[90m" // Gray
				} else if job.FailCount > 0 {
					color = "\033[33m" // Yellow
				}
				fmt.Printf("  %s[%s]\033[0m %s (%s)\n", color, statusIcon, job.Name, job.Schedule)
				fmt.Printf("      \033[90mID: %s | Next: %s | Runs: %d | Fails: %d\033[0m\n",
					job.ID[:8], nextRun, job.RunCount, job.FailCount)
			}
		}
		fmt.Printf("\nTotal: %d jobs\n", len(result.Jobs))
	}
}

func runCronStatus(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(getCronBaseURL() + "/status")
	if err != nil {
		printCronError("Failed to fetch cron status", err)
		return
	}
	defer resp.Body.Close()

	var status struct {
		Running    bool   `json:"running"`
		JobCount   int    `json:"job_count"`
		ActiveJobs int    `json:"active_jobs"`
		Uptime     string `json:"uptime,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		printCronError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(status)
	} else {
		statusText := "Running"
		if !status.Running {
			statusText = "Stopped"
		}

		if noColor {
			fmt.Printf("Cron Service: %s\n", statusText)
		} else {
			color := "\033[32m"
			if !status.Running {
				color = "\033[31m"
			}
			fmt.Printf("Cron Service: %s%s\033[0m\n", color, statusText)
		}
		fmt.Printf("Total Jobs: %d\n", status.JobCount)
		fmt.Printf("Active Jobs: %d\n", status.ActiveJobs)
		if status.Uptime != "" {
			fmt.Printf("Uptime: %s\n", status.Uptime)
		}
	}
}

func runCronAdd(cmd *cobra.Command, args []string) {
	client := &http.Client{Timeout: 10 * time.Second}

	payload := map[string]interface{}{
		"name":     cronName,
		"schedule": cronSchedule,
		"handler":  cronHandler,
	}

	if cronPayload != "" {
		var p map[string]interface{}
		if err := json.Unmarshal([]byte(cronPayload), &p); err != nil {
			printCronError("Invalid payload JSON", err)
			return
		}
		payload["payload"] = p
	}

	body, _ := json.Marshal(payload)
	resp, err := client.Post(getCronBaseURL()+"/jobs", "application/json", bytes.NewReader(body))
	if err != nil {
		printCronError("Failed to create cron job", err)
		return
	}
	defer resp.Body.Close()

	var result CronJob
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printCronError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": true,
			"job":     result,
		})
	} else {
		if noColor {
			fmt.Printf("Created cron job: %s (%s)\n", result.Name, result.ID)
		} else {
			fmt.Printf("\033[32m✓\033[0m Created cron job: %s (%s)\n", result.Name, result.ID)
		}
	}
}

func runCronRm(cmd *cobra.Command, args []string) {
	jobID := args[0]
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodDelete, getCronBaseURL()+"/jobs/"+jobID, nil)
	if err != nil {
		printCronError("Failed to create request", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		printCronError("Failed to delete cron job", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		printCronError("Cron job not found", fmt.Errorf("id: %s", jobID))
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": true,
			"id":      jobID,
		})
	} else {
		if noColor {
			fmt.Printf("Deleted cron job: %s\n", jobID)
		} else {
			fmt.Printf("\033[32m✓\033[0m Deleted cron job: %s\n", jobID)
		}
	}
}

func runCronEnable(cmd *cobra.Command, args []string) {
	setCronEnabled(args[0], true)
}

func runCronDisable(cmd *cobra.Command, args []string) {
	setCronEnabled(args[0], false)
}

func setCronEnabled(jobID string, enabled bool) {
	client := &http.Client{Timeout: 10 * time.Second}

	action := "enable"
	if !enabled {
		action = "disable"
	}

	req, err := http.NewRequest(http.MethodPost, getCronBaseURL()+"/jobs/"+jobID+"/"+action, nil)
	if err != nil {
		printCronError("Failed to create request", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		printCronError("Failed to update cron job", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		printCronError("Cron job not found", fmt.Errorf("id: %s", jobID))
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success": true,
			"id":      jobID,
			"enabled": enabled,
		})
	} else {
		verb := "Enabled"
		if !enabled {
			verb = "Disabled"
		}
		if noColor {
			fmt.Printf("%s cron job: %s\n", verb, jobID)
		} else {
			fmt.Printf("\033[32m✓\033[0m %s cron job: %s\n", verb, jobID)
		}
	}
}

func runCronRun(cmd *cobra.Command, args []string) {
	jobID := args[0]
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodPost, getCronBaseURL()+"/jobs/"+jobID+"/trigger", nil)
	if err != nil {
		printCronError("Failed to create request", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		printCronError("Failed to trigger cron job", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		printCronError("Cron job not found", fmt.Errorf("id: %s", jobID))
		return
	}

	if jsonOutput {
		printJSON(map[string]interface{}{
			"success":   true,
			"id":        jobID,
			"triggered": true,
		})
	} else {
		if noColor {
			fmt.Printf("Triggered cron job: %s\n", jobID)
		} else {
			fmt.Printf("\033[32m✓\033[0m Triggered cron job: %s\n", jobID)
		}
	}
}

func runCronRuns(cmd *cobra.Command, args []string) {
	jobID := args[0]
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(getCronBaseURL() + "/jobs/" + jobID + "/executions")
	if err != nil {
		printCronError("Failed to fetch executions", err)
		return
	}
	defer resp.Body.Close()

	var result struct {
		Executions []struct {
			ID        string     `json:"id"`
			StartedAt time.Time  `json:"started_at"`
			EndedAt   *time.Time `json:"ended_at,omitempty"`
			Status    string     `json:"status"`
			Error     string     `json:"error,omitempty"`
		} `json:"executions"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		printCronError("Failed to parse response", err)
		return
	}

	if jsonOutput {
		printJSON(result)
	} else {
		if len(result.Executions) == 0 {
			fmt.Println("No execution history")
			return
		}

		fmt.Println("Execution History:")
		for _, exec := range result.Executions {
			statusIcon := "✓"
			if exec.Status == "failed" {
				statusIcon = "✗"
			} else if exec.Status == "running" {
				statusIcon = "▶"
			}

			duration := "running"
			if exec.EndedAt != nil {
				duration = exec.EndedAt.Sub(exec.StartedAt).String()
			}

			if noColor {
				fmt.Printf("  [%s] %s - %s (%s)\n",
					statusIcon, exec.StartedAt.Format("2006-01-02 15:04:05"), exec.Status, duration)
			} else {
				color := "\033[32m"
				if exec.Status == "failed" {
					color = "\033[31m"
				} else if exec.Status == "running" {
					color = "\033[33m"
				}
				fmt.Printf("  %s[%s]\033[0m %s - %s (%s)\n",
					color, statusIcon, exec.StartedAt.Format("2006-01-02 15:04:05"), exec.Status, duration)
			}
			if exec.Error != "" && verbose {
				fmt.Printf("      Error: %s\n", exec.Error)
			}
		}
	}
}

func printCronError(msg string, err error) {
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
