package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	logsFollow bool
	logsLines  int
	logsLevel  string
)

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View service logs",
	Long: `View ZimaOS-Blue service logs.

Examples:
  echo logs                    # Show recent logs
  echo logs -f                 # Follow logs in real-time
  echo logs -n 100             # Show last 100 lines
  echo logs --level error      # Show only error logs`,
	Run: runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "follow logs in real-time")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 50, "number of lines to show")
	logsCmd.Flags().StringVar(&logsLevel, "level", "", "filter by log level (debug, info, warn, error)")

	rootCmd.AddCommand(logsCmd)
}

// LogEntry represents a parsed log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

func runLogs(cmd *cobra.Command, args []string) {
	logsDir := getLogsDir()
	logFile := filepath.Join(logsDir, "blue.log")

	// Check if log file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		// Try alternative locations
		alternatives := []string{
			"./logs/blue.log",
			"./blue.log",
			filepath.Join(getConfigDir(), "logs", "blue.log"),
		}

		found := false
		for _, alt := range alternatives {
			if _, err := os.Stat(alt); err == nil {
				logFile = alt
				found = true
				break
			}
		}

		if !found {
			if jsonOutput {
				printJSON(map[string]interface{}{
					"error":   "Log file not found",
					"checked": append([]string{logFile}, alternatives...),
				})
			} else {
				fmt.Println("Log file not found. The service may not have started yet.")
				fmt.Printf("Checked locations:\n")
				fmt.Printf("  - %s\n", logFile)
				for _, alt := range alternatives {
					fmt.Printf("  - %s\n", alt)
				}
			}
			return
		}
	}

	if logsFollow {
		followLogs(logFile)
	} else {
		showRecentLogs(logFile)
	}
}

func showRecentLogs(logFile string) {
	file, err := os.Open(logFile)
	if err != nil {
		printLogsError("Failed to open log file", err)
		return
	}
	defer file.Close()

	// Read all lines and keep last N
	var lines []string
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if logsLevel != "" && !matchesLevel(line, logsLevel) {
			continue
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		printLogsError("Error reading log file", err)
		return
	}

	// Get last N lines
	start := 0
	if len(lines) > logsLines {
		start = len(lines) - logsLines
	}

	if jsonOutput {
		entries := make([]LogEntry, 0)
		for _, line := range lines[start:] {
			entry := parseLogLine(line)
			entries = append(entries, entry)
		}
		printJSON(map[string]interface{}{
			"file":    logFile,
			"count":   len(entries),
			"entries": entries,
		})
	} else {
		for _, line := range lines[start:] {
			printLogLine(line)
		}
	}
}

func followLogs(logFile string) {
	file, err := os.Open(logFile)
	if err != nil {
		printLogsError("Failed to open log file", err)
		return
	}
	defer file.Close()

	// Seek to end
	file.Seek(0, 2)

	if !jsonOutput {
		fmt.Printf("Following %s (Ctrl+C to stop)\n\n", logFile)
	}

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			// No new data, wait and retry
			time.Sleep(100 * time.Millisecond)
			continue
		}

		line = strings.TrimSuffix(line, "\n")
		if logsLevel != "" && !matchesLevel(line, logsLevel) {
			continue
		}

		if jsonOutput {
			entry := parseLogLine(line)
			printJSON(entry)
		} else {
			printLogLine(line)
		}
	}
}

func matchesLevel(line, level string) bool {
	level = strings.ToUpper(level)
	// Check common log level patterns
	patterns := []string{
		fmt.Sprintf(`"level":"%s"`, strings.ToLower(level)),
		fmt.Sprintf(`"level": "%s"`, strings.ToLower(level)),
		fmt.Sprintf("[%s]", level),
		fmt.Sprintf(" %s ", level),
	}

	lineLower := strings.ToLower(line)
	for _, pattern := range patterns {
		if strings.Contains(lineLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

func parseLogLine(line string) LogEntry {
	entry := LogEntry{
		Message: line,
		Fields:  make(map[string]interface{}),
	}

	// Try to parse JSON log format
	if strings.HasPrefix(line, "{") {
		// JSON format - extract fields
		if idx := strings.Index(line, `"time":`); idx >= 0 {
			// Extract timestamp
			start := idx + 8
			end := strings.Index(line[start:], `"`)
			if end > 0 {
				entry.Timestamp = line[start : start+end]
			}
		}
		if idx := strings.Index(line, `"level":`); idx >= 0 {
			start := idx + 9
			end := strings.Index(line[start:], `"`)
			if end > 0 {
				entry.Level = line[start : start+end]
			}
		}
		if idx := strings.Index(line, `"message":`); idx >= 0 {
			start := idx + 11
			end := strings.Index(line[start:], `"`)
			if end > 0 {
				entry.Message = line[start : start+end]
			}
		}
	} else {
		// Plain text format - try to extract level
		for _, level := range []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"} {
			if strings.Contains(line, level) || strings.Contains(line, strings.ToLower(level)) {
				entry.Level = strings.ToLower(level)
				break
			}
		}
	}

	return entry
}

func printLogLine(line string) {
	if noColor {
		fmt.Println(line)
		return
	}

	// Colorize based on level
	lineLower := strings.ToLower(line)
	var color string

	switch {
	case strings.Contains(lineLower, "error") || strings.Contains(lineLower, "fatal"):
		color = "\033[31m" // Red
	case strings.Contains(lineLower, "warn"):
		color = "\033[33m" // Yellow
	case strings.Contains(lineLower, "debug"):
		color = "\033[90m" // Gray
	case strings.Contains(lineLower, "info"):
		color = "\033[0m" // Default
	default:
		color = "\033[0m"
	}

	fmt.Printf("%s%s\033[0m\n", color, line)
}

func printLogsError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]interface{}{
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
