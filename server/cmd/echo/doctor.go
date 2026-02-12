package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

var (
	doctorFix bool
)

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Health checks and quick fixes",
	Long: `Run diagnostic checks on ZimaOS-Echo installation and configuration.

Checks include:
- Configuration file validity
- Service connectivity
- Required dependencies
- File permissions
- Port availability`,
	Run: runDoctor,
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "automatically fix issues where possible")
}

// CheckResult represents a single check result
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pass, warn, fail
	Message string `json:"message,omitempty"`
	Fixed   bool   `json:"fixed,omitempty"`
}

// DoctorReport represents the full doctor report
type DoctorReport struct {
	Timestamp string        `json:"timestamp"`
	Checks    []CheckResult `json:"checks"`
	Summary   struct {
		Total  int `json:"total"`
		Passed int `json:"passed"`
		Warned int `json:"warned"`
		Failed int `json:"failed"`
		Fixed  int `json:"fixed"`
	} `json:"summary"`
}

func runDoctor(cmd *cobra.Command, args []string) {
	report := DoctorReport{
		Timestamp: time.Now().Format(time.RFC3339),
		Checks:    make([]CheckResult, 0),
	}

	// Run all checks
	checks := []func() CheckResult{
		checkConfigDir,
		checkConfigFile,
		checkDataDir,
		checkLogsDir,
		checkServiceRunning,
		checkPort,
		checkDependencies,
		checkPermissions,
	}

	for _, check := range checks {
		result := check()
		report.Checks = append(report.Checks, result)

		// Update summary
		report.Summary.Total++
		switch result.Status {
		case "pass":
			report.Summary.Passed++
		case "warn":
			report.Summary.Warned++
		case "fail":
			report.Summary.Failed++
		}
		if result.Fixed {
			report.Summary.Fixed++
		}
	}

	// Output
	if jsonOutput {
		printJSON(report)
	} else {
		printDoctorHuman(report)
	}

	// Exit with error if any checks failed
	if report.Summary.Failed > 0 {
		os.Exit(1)
	}
}

func checkConfigDir() CheckResult {
	configDir := getConfigDir()
	result := CheckResult{
		Name: "Config Directory",
	}

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		if doctorFix {
			if err := os.MkdirAll(configDir, 0700); err != nil {
				result.Status = "fail"
				result.Message = fmt.Sprintf("Failed to create: %v", err)
			} else {
				result.Status = "pass"
				result.Message = fmt.Sprintf("Created %s", configDir)
				result.Fixed = true
			}
		} else {
			result.Status = "fail"
			result.Message = fmt.Sprintf("Missing: %s (run with --fix to create)", configDir)
		}
	} else {
		result.Status = "pass"
		result.Message = configDir
	}

	return result
}

func checkConfigFile() CheckResult {
	configPath := filepath.Join(getConfigDir(), "config.yaml")
	result := CheckResult{
		Name: "Config File",
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if doctorFix {
			if err := os.MkdirAll(filepath.Dir(configPath), 0700); err == nil {
				if err := os.WriteFile(configPath, []byte("# ZimaOS-Echo Configuration\n"), 0600); err != nil {
					result.Status = "fail"
					result.Message = fmt.Sprintf("Failed to create: %v", err)
				} else {
					result.Status = "pass"
					result.Message = fmt.Sprintf("Created %s", configPath)
					result.Fixed = true
				}
			} else {
				result.Status = "fail"
				result.Message = fmt.Sprintf("Failed to create directory: %v", err)
			}
		} else {
			result.Status = "warn"
			result.Message = fmt.Sprintf("Missing: %s (using defaults)", configPath)
		}
	} else {
		result.Status = "pass"
		result.Message = configPath
	}

	return result
}

func checkDataDir() CheckResult {
	dataDir := getDataDir()
	result := CheckResult{
		Name: "Data Directory",
	}

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if doctorFix {
			if err := os.MkdirAll(dataDir, 0700); err != nil {
				result.Status = "fail"
				result.Message = fmt.Sprintf("Failed to create: %v", err)
			} else {
				result.Status = "pass"
				result.Message = fmt.Sprintf("Created %s", dataDir)
				result.Fixed = true
			}
		} else {
			result.Status = "warn"
			result.Message = fmt.Sprintf("Missing: %s (will be created on first run)", dataDir)
		}
	} else {
		result.Status = "pass"
		result.Message = dataDir
	}

	return result
}

func checkLogsDir() CheckResult {
	logsDir := getLogsDir()
	result := CheckResult{
		Name: "Logs Directory",
	}

	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		if doctorFix {
			if err := os.MkdirAll(logsDir, 0700); err != nil {
				result.Status = "fail"
				result.Message = fmt.Sprintf("Failed to create: %v", err)
			} else {
				result.Status = "pass"
				result.Message = fmt.Sprintf("Created %s", logsDir)
				result.Fixed = true
			}
		} else {
			result.Status = "warn"
			result.Message = fmt.Sprintf("Missing: %s (will be created on first run)", logsDir)
		}
	} else {
		result.Status = "pass"
		result.Message = logsDir
	}

	return result
}

func checkServiceRunning() CheckResult {
	result := CheckResult{
		Name: "Service Status",
	}

	port := 8080
	if devMode {
		port = 8081
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		result.Status = "warn"
		result.Message = "Service not running"
	} else {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			result.Status = "pass"
			result.Message = fmt.Sprintf("Running on port %d", port)
		} else {
			result.Status = "warn"
			result.Message = fmt.Sprintf("Service unhealthy (status %d)", resp.StatusCode)
		}
	}

	return result
}

func checkPort() CheckResult {
	result := CheckResult{
		Name: "Port Availability",
	}

	port := 8080
	if devMode {
		port = 8081
	}

	// Try to connect to the port
	client := &http.Client{Timeout: 1 * time.Second}
	_, err := client.Get(fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		result.Status = "pass"
		result.Message = fmt.Sprintf("Port %d available", port)
	} else {
		// Port is in use - check if it's our service
		resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
		if err == nil {
			resp.Body.Close()
			result.Status = "pass"
			result.Message = fmt.Sprintf("Port %d in use by ZimaOS-Echo", port)
		} else {
			result.Status = "warn"
			result.Message = fmt.Sprintf("Port %d in use by another service", port)
		}
	}

	return result
}

func checkDependencies() CheckResult {
	result := CheckResult{
		Name: "Dependencies",
	}

	missing := []string{}

	// Check for optional dependencies
	if runtime.GOOS != "windows" {
		// Check for curl (used by some features)
		if _, err := exec.LookPath("curl"); err != nil {
			missing = append(missing, "curl")
		}
	}

	if len(missing) > 0 {
		result.Status = "warn"
		result.Message = fmt.Sprintf("Optional dependencies missing: %v", missing)
	} else {
		result.Status = "pass"
		result.Message = "All dependencies available"
	}

	return result
}

func checkPermissions() CheckResult {
	result := CheckResult{
		Name: "File Permissions",
	}

	configDir := getConfigDir()
	issues := []string{}

	// Check config directory permissions
	if info, err := os.Stat(configDir); err == nil {
		mode := info.Mode().Perm()
		if mode&0077 != 0 { // Check if group/other have any permissions
			if doctorFix {
				if err := os.Chmod(configDir, 0700); err != nil {
					issues = append(issues, fmt.Sprintf("Failed to fix %s permissions", configDir))
				} else {
					result.Fixed = true
				}
			} else {
				issues = append(issues, fmt.Sprintf("%s has insecure permissions (%o)", configDir, mode))
			}
		}
	}

	// Check config file permissions
	configPath := filepath.Join(configDir, "config.yaml")
	if info, err := os.Stat(configPath); err == nil {
		mode := info.Mode().Perm()
		if mode&0077 != 0 {
			if doctorFix {
				if err := os.Chmod(configPath, 0600); err != nil {
					issues = append(issues, fmt.Sprintf("Failed to fix %s permissions", configPath))
				} else {
					result.Fixed = true
				}
			} else {
				issues = append(issues, fmt.Sprintf("%s has insecure permissions (%o)", configPath, mode))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = "warn"
		result.Message = fmt.Sprintf("%v", issues)
	} else if result.Fixed {
		result.Status = "pass"
		result.Message = "Permissions fixed"
	} else {
		result.Status = "pass"
		result.Message = "Permissions OK"
	}

	return result
}

func printDoctorHuman(report DoctorReport) {
	fmt.Println("ZimaOS-Echo Doctor")
	fmt.Println("==================")
	fmt.Println()

	for _, check := range report.Checks {
		var icon, color string
		switch check.Status {
		case "pass":
			icon = "✓"
			color = "\033[32m"
		case "warn":
			icon = "!"
			color = "\033[33m"
		case "fail":
			icon = "✗"
			color = "\033[31m"
		}

		if noColor {
			fmt.Printf("[%s] %s: %s", icon, check.Name, check.Message)
		} else {
			fmt.Printf("%s[%s]\033[0m %s: %s", color, icon, check.Name, check.Message)
		}
		if check.Fixed {
			if noColor {
				fmt.Print(" (fixed)")
			} else {
				fmt.Print(" \033[32m(fixed)\033[0m")
			}
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Printf("Summary: %d passed, %d warnings, %d failed",
		report.Summary.Passed, report.Summary.Warned, report.Summary.Failed)
	if report.Summary.Fixed > 0 {
		fmt.Printf(", %d fixed", report.Summary.Fixed)
	}
	fmt.Println()
}
